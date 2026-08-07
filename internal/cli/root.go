// Package cli exposes wgtui's non-interactive command surface (SPEC.md
// §21). Every command here calls into internal/discovery and
// internal/sshtransport - it never runs a shell command directly.
package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/update"
)

// Version is overridden at release-build time via -ldflags -X (see
// .goreleaser.yaml and docs/SPEC.md §28 Stage 5) - a var, not a const,
// because -X only rewrites package-level string variables. Development
// builds (plain `go build`) report this placeholder.
var Version = "0.0.0-dev"

// Run is the CLI entry point. It returns a process exit code: 0 on
// success, 1 on a runtime failure (connection error, remote command
// failure, ...), 2 on a usage error.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "server":
		return runServer(args[1:], stdout, stderr)
	case "peer":
		return runPeer(args[1:], stdout, stderr)
	case "diagnose":
		return runDiagnose(args[1:], stdout, stderr)
	case "completion":
		return runCompletion(args[1:], stdout, stderr)
	case "version", "--version":
		return runVersion(stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "wgtui: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

// runVersion prints the running version and, on a best-effort basis,
// whether a newer release exists. This is the "update-available
// notification, no auto-update" requirement (spec §28, Stage 5): purely
// informational, never downloads or installs anything, and the check is
// silently skipped on any network failure - `wgtui version` must never
// fail or hang just because GitHub is unreachable.
func runVersion(stdout io.Writer) int {
	fmt.Fprintln(stdout, "wgtui "+Version)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	info, err := (update.Checker{}).Check(ctx, Version)
	if err == nil && info.Available {
		fmt.Fprintf(stdout, "A newer version is available: %s (you have %s)\n%s\n", info.Latest, Version, info.URL)
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `wgtui - manage remote WireGuard servers over SSH

Usage:
  wgtui server add <id> --ssh-host <host> [--ssh-user <user>] [--ssh-port <port>] [--ssh-identity <path>] [--display-name <name>]
  wgtui server inspect <id> [--json]
  wgtui server install-wireguard <id> [--yes]
  wgtui server configure <id> [--interface wg0] [--vpn-cidr <cidr>] [--port 51820]
                          [--outbound-interface <if>] [--full-tunnel-default=false] [--yes]
  wgtui peer list <id> [--json]
  wgtui peer add <id> --name <name> [--full-tunnel | --split-tunnel <cidrs>] [--keepalive N] [--dns <ips>]
                 [--endpoint-host <host>] [--no-psk] [--output <path>] [--qr] [--show] [--yes] [--json]
  wgtui peer disable <id> <peer-name> [--yes]
  wgtui peer enable <id> <peer-name> [--yes]
  wgtui peer revoke <id> <peer-name> [--delete-profile] [--delete-metadata] [--yes]
  wgtui peer rotate <id> <peer-name> [--yes]
  wgtui peer export <id> <peer-name> --output <path> [--force]
  wgtui peer import <id> --file <path> --name <name> [--interface <if>]
  wgtui diagnose <id> [--json]
  wgtui mcp                    run as an MCP server over stdio, for AI agent front ends (see docs/ARCHITECTURE.md)
  wgtui completion <bash|zsh|fish>
  wgtui version

Every command that reaches the network prompts interactively for the SSH
and/or sudo password if key/agent auth isn't enough - wgtui never accepts
either via an argument or an environment variable (SPEC.md §10.3).

See docs/SPEC.md for the full specification.
`)
}
