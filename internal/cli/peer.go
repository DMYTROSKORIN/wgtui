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
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
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

// peerView is the JSON shape from SPEC.md §21's example, extended with
// Interface (Stage 4: multi-interface servers) and Managed. Name/Managed/
// ProfileAvailable are reconciled against the local metadata registry
// (SPEC.md §16.1) added in Stage 2 - a peer wgtui itself created shows its
// real name and profile availability; anything found on the server but
// not in local metadata is "unmanaged" and shown by shortened public key,
// exactly as §16.1 describes.
type peerView struct {
	Name                   string `json:"name"`
	Interface              string `json:"interface,omitempty"`
	VPNIP                  string `json:"vpn_ip,omitempty"`
	PublicKey              string `json:"public_key"`
	State                  string `json:"state"`
	LatestHandshakeSeconds *int64 `json:"latest_handshake_seconds,omitempty"`
	RxBytes                int64  `json:"rx_bytes"`
	TxBytes                int64  `json:"tx_bytes"`
	ProfileAvailable       bool   `json:"profile_available"`
	Managed                bool   `json:"managed_by_wgtui"`
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

	views, err := listPeers(stdout, v, server)
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

func listPeers(out io.Writer, v *profilevault.Vault, s domain.Server) ([]peerView, error) {
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

	return reconcilePeers(dump.Peers, v.ListPeers(s.ID), func(pubKey string) bool {
		_, ok := v.GetProfile(s.ID, pubKey)
		return ok
	}, time.Now()), nil
}

// reconcilePeers merges runtime peers (from `wg show all dump`) with the
// local metadata registry (SPEC.md §16.1): a peer wgtui itself created
// shows its real name and profile availability; anything found on the
// server but not in local metadata is "unmanaged" and shown by shortened
// public key. A peer wgtui disabled is deliberately absent from runtime
// *and* persistent config, but the user still asked wgtui to manage it -
// it's included too (state "disabled") rather than vanishing from the
// list just because it isn't on the wire right now. Pure function, no
// network - the SSH-dependent part is entirely in listPeers above it.
func reconcilePeers(dumpPeers []wgconfig.RuntimePeer, localPeers []profilevault.PeerMetadata, hasProfile func(publicKey string) bool, now time.Time) []peerView {
	metaByKey := make(map[string]profilevault.PeerMetadata, len(localPeers))
	for _, m := range localPeers {
		metaByKey[m.PublicKey] = m
	}

	seen := make(map[string]bool, len(dumpPeers))
	views := make([]peerView, 0, len(dumpPeers))
	for _, p := range dumpPeers {
		seen[p.PublicKey] = true
		meta, managed := metaByKey[p.PublicKey]

		status := domain.HandshakeStatus(now, p.LatestHandshake, domain.DefaultOnlineThreshold, false)
		name := shortenKey(p.PublicKey)
		if managed && meta.Name != "" {
			name = meta.Name
		}

		pv := peerView{
			Name:             name,
			Interface:        p.Interface,
			PublicKey:        p.PublicKey,
			State:            string(status),
			RxBytes:          p.ReceiveBytes,
			TxBytes:          p.TransmitBytes,
			ProfileAvailable: hasProfile(p.PublicKey),
			Managed:          managed,
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

	for _, meta := range localPeers {
		if seen[meta.PublicKey] || meta.Status != profilevault.PeerStatusDisabled {
			continue
		}
		views = append(views, peerView{
			Name:             meta.Name,
			Interface:        meta.Interface,
			PublicKey:        meta.PublicKey,
			State:            string(domain.StatusDisabled),
			ProfileAvailable: hasProfile(meta.PublicKey),
			Managed:          true,
		})
	}

	return views
}

func shortenKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:8] + "…"
}

func printPeersText(out io.Writer, views []peerView) {
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tIFACE\tVPN IP\tSTATE\tHANDSHAKE\tRX\tTX\tMANAGED")
	for _, v := range views {
		handshake := "never"
		if v.LatestHandshakeSeconds != nil {
			handshake = fmt.Sprintf("%ds ago", *v.LatestHandshakeSeconds)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%v\n", v.Name, orNone(v.Interface), orNone(v.VPNIP), v.State, handshake, v.RxBytes, v.TxBytes, v.Managed)
	}
	tw.Flush()
	fmt.Fprintln(out, "\nNote: \"online\" is a heuristic based on handshake recency, not proof of an active connection (SPEC.md §19).")
}
