package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

func runServer(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui server: expected a subcommand (add, inspect, install-wireguard, configure)")
		return 2
	}
	switch args[0] {
	case "add":
		return runServerAdd(args[1:], stdout, stderr)
	case "inspect":
		return runServerInspect(args[1:], stdout, stderr)
	case "install-wireguard":
		return runServerInstallWireguard(args[1:], stdout, stderr)
	case "configure":
		return runServerConfigure(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "wgtui server: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runServerAdd(args []string, stdout, stderr io.Writer) int {
	// The server ID is a fixed leading positional argument, SPEC.md §21's
	// own examples put it right after the subcommand and before every
	// flag ("wgtui server add office --ssh-host ..."). It has to be
	// peeled off *before* fs.Parse: the standard flag package stops
	// parsing at the first non-flag token, so if the ID were left in
	// front of fs.Parse's input it would swallow every flag after it as
	// more positional arguments instead of recognizing them.
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui server add: expected the server ID as the first argument")
		return 2
	}
	id := args[0]

	fs := flag.NewFlagSet("server add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sshHost := fs.String("ssh-host", "", "SSH hostname, IP, or ~/.ssh/config alias (required)")
	sshUser := fs.String("ssh-user", "", "SSH user (optional, defaults to ssh_config)")
	sshPort := fs.Int("ssh-port", 0, "SSH port (optional, defaults to ssh_config/22)")
	sshIdentity := fs.String("ssh-identity", "", "path to an SSH private key (optional)")
	displayName := fs.String("display-name", "", "human-readable name (defaults to the ID)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui server add: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}
	if *sshHost == "" {
		fmt.Fprintln(stderr, "wgtui server add: --ssh-host is required")
		return 2
	}
	name := *displayName
	if name == "" {
		name = id
	}

	// SPEC.md §10.1: preview the effective host/user/port via `ssh -G`
	// before anything is stored or connected to.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	resolved, err := sshtransport.ResolveAlias(ctx, sshtransport.Target{
		HostOrAlias: *sshHost, Port: *sshPort, User: *sshUser, IdentityFile: *sshIdentity,
	})
	cancel()
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server add: resolve SSH target: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Resolved (nothing connected yet): host=%s port=%d user=%s\n", resolved.Hostname, resolved.Port, resolved.User)

	v, vaultPath, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server add: %v\n", err)
		return 1
	}

	s := domain.Server{
		ID:              id,
		DisplayName:     name,
		SSHHostOrAlias:  *sshHost,
		SSHUser:         *sshUser,
		SSHPort:         *sshPort,
		SSHIdentityFile: *sshIdentity,
	}
	if err := v.AddServer(s); err != nil {
		fmt.Fprintf(stderr, "wgtui server add: %v\n", err)
		return 1
	}
	if err := v.Save(vaultPath); err != nil {
		fmt.Fprintf(stderr, "wgtui server add: save vault: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Added server %q.\n", id)
	return 0
}

type inspectionResult struct {
	State  discovery.State  `json:"state"`
	Report discovery.Report `json:"report"`
}

func runServerInspect(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui server inspect: expected the server ID as the first argument")
		return 2
	}
	id := args[0]

	fs := flag.NewFlagSet("server inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output structured JSON instead of text")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui server inspect: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}

	v, _, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server inspect: %v\n", err)
		return 1
	}
	server, ok := v.GetServer(id)
	if !ok {
		fmt.Fprintf(stderr, "wgtui server inspect: no server named %q (use 'wgtui server add' first)\n", id)
		return 1
	}

	result, err := inspectServer(stdout, server)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server inspect: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(stderr, "wgtui server inspect: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printReportText(stdout, result)
	return 0
}

func inspectServer(out io.Writer, s domain.Server) (inspectionResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	controlDir, cleanup, err := newControlDir()
	if err != nil {
		return inspectionResult{}, err
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
		return inspectionResult{}, fmt.Errorf("connect: %w", err)
	}
	defer client.Close(ctx)

	report := discovery.Discover(ctx, client, prompts)
	return inspectionResult{State: discovery.Classify(report), Report: report}, nil
}

func printReportText(out io.Writer, r inspectionResult) {
	fmt.Fprintf(out, "State: %s\n\n", r.State)
	fmt.Fprintf(out, "System:  %s\n", r.Report.System.Kernel)
	fmt.Fprintf(out, "OS:      %s %s (package manager: %s)\n", r.Report.System.OSID, r.Report.System.OSVersionID, orNone(r.Report.System.PackageManager))
	fmt.Fprintf(out, "sudo:    %v\n\n", r.Report.System.SudoAvailable)

	fmt.Fprintf(out, "WireGuard: present=%v wg-quick=%v version=%s\n", r.Report.WireGuard.WgPresent, r.Report.WireGuard.WgQuickPresent, orNone(r.Report.WireGuard.WgVersion))
	for name, d := range r.Report.WireGuard.Interfaces {
		fmt.Fprintf(out, "  %s: active=%v config=%v saveconfig=%v drift=%v runtime_peers=%d persistent_peers=%d\n",
			name, d.Active, d.ConfigExists, d.SaveConfig, d.Drift, len(d.RuntimePeerPublicKeys), len(d.PersistentPeerPublicKeys))
	}

	fmt.Fprintf(out, "\nNetwork: outbound=%s ipv4_forward=%v ipv6_forward=%v firewall=%s\n",
		orNone(r.Report.Network.DefaultOutboundInterface), r.Report.Network.IPv4Forwarding, r.Report.Network.IPv6Forwarding, orNone(r.Report.Network.FirewallBackend))
	fmt.Fprintf(out, "Listening UDP ports: %v\n", r.Report.Network.ListeningUDPPorts)

	if len(r.Report.Errors) > 0 {
		fmt.Fprintln(out, "\nNon-fatal check errors:")
		for _, e := range r.Report.Errors {
			fmt.Fprintln(out, "  - "+e)
		}
	}
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
