package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

func runPeer(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui peer: expected a subcommand (list, add, disable, enable, revoke, rotate, import, export)")
		return 2
	}
	switch args[0] {
	case "list":
		return runPeerList(args[1:], stdout, stderr)
	case "add":
		return runPeerAdd(args[1:], stdout, stderr)
	case "disable":
		return runPeerDisable(args[1:], stdout, stderr)
	case "enable":
		return runPeerEnable(args[1:], stdout, stderr)
	case "revoke":
		return runPeerRevoke(args[1:], stdout, stderr)
	case "rotate":
		return runPeerRotate(args[1:], stdout, stderr)
	case "export":
		return runPeerExport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "wgtui peer: unknown subcommand %q\n", args[0])
		return 2
	}
}

// peerView is the JSON shape from SPEC.md §21's example. Stage 1 has no
// local peer-metadata registry yet (that's Stage 2, SPEC.md §16.1), so
// Name is a shortened public key and ProfileAvailable is always false -
// this matches SPEC.md §16.1's own description of how an *unmanaged*/
// unknown peer should be displayed, which is every peer at this stage.
type peerView struct {
	Name                   string `json:"name"`
	VPNIP                  string `json:"vpn_ip,omitempty"`
	PublicKey              string `json:"public_key"`
	State                  string `json:"state"`
	LatestHandshakeSeconds *int64 `json:"latest_handshake_seconds,omitempty"`
	RxBytes                int64  `json:"rx_bytes"`
	TxBytes                int64  `json:"tx_bytes"`
	ProfileAvailable       bool   `json:"profile_available"`
}

func runPeerList(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui peer list: expected the server ID as the first argument")
		return 2
	}
	id := args[0]

	fs := flag.NewFlagSet("peer list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output structured JSON instead of text")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui peer list: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}

	v, _, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer list: %v\n", err)
		return 1
	}
	server, ok := v.GetServer(id)
	if !ok {
		fmt.Fprintf(stderr, "wgtui peer list: no server named %q\n", id)
		return 1
	}

	views, err := listPeers(stdout, server)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer list: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(views); err != nil {
			fmt.Fprintf(stderr, "wgtui peer list: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printPeersText(stdout, views)
	return 0
}

func listPeers(out io.Writer, s domain.Server) ([]peerView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	controlDir, cleanup, err := newControlDir()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	client := sshtransport.NewClient(sshtransport.Target{
		HostOrAlias:  s.SSHHostOrAlias,
		Port:         s.SSHPort,
		User:         s.SSHUser,
		IdentityFile: s.SSHIdentityFile,
	}, filepath.Join(controlDir, "cm.sock"))

	prompts := newTerminalPrompts(out)
	if err := client.Dial(ctx, prompts); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer client.Close(ctx)

	if err := client.SudoValidate(ctx, prompts); err != nil {
		return nil, fmt.Errorf("sudo: %w", err)
	}
	res, err := client.RunSudo(ctx, "wg show all dump", prompts)
	if err != nil {
		return nil, fmt.Errorf("wg show all dump: %w", err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("wg show all dump exited %d: %s", res.ExitCode, res.Stderr)
	}
	dump, err := wgconfig.ParseDump(string(res.Stdout))
	if err != nil {
		return nil, fmt.Errorf("parse wg dump: %w", err)
	}

	now := time.Now()
	views := make([]peerView, 0, len(dump.Peers))
	for _, p := range dump.Peers {
		status := domain.HandshakeStatus(now, p.LatestHandshake, domain.DefaultOnlineThreshold, false)
		pv := peerView{
			Name:      shortenKey(p.PublicKey),
			PublicKey: p.PublicKey,
			State:     string(status),
			RxBytes:   p.ReceiveBytes,
			TxBytes:   p.TransmitBytes,
		}
		if len(p.AllowedIPs) > 0 {
			pv.VPNIP = p.AllowedIPs[0]
		}
		if !p.LatestHandshake.IsZero() {
			secs := int64(now.Sub(p.LatestHandshake).Seconds())
			pv.LatestHandshakeSeconds = &secs
		}
		views = append(views, pv)
	}
	return views, nil
}

func shortenKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:8] + "…"
}

func printPeersText(out io.Writer, views []peerView) {
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVPN IP\tSTATE\tHANDSHAKE\tRX\tTX")
	for _, v := range views {
		handshake := "never"
		if v.LatestHandshakeSeconds != nil {
			handshake = fmt.Sprintf("%ds ago", *v.LatestHandshakeSeconds)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\n", v.Name, orNone(v.VPNIP), v.State, handshake, v.RxBytes, v.TxBytes)
	}
	tw.Flush()
	fmt.Fprintln(out, "\nNote: \"online\" is a heuristic based on handshake recency, not proof of an active connection (SPEC.md §19).")
}
