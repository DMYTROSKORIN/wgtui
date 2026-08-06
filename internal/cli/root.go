// Package cli exposes wgtui's non-interactive command surface (SPEC.md
// §21). Every command here calls into internal/discovery and
// internal/sshtransport - it never runs a shell command directly.
package cli

import (
	"fmt"
	"io"
)

// Version is overridden at release-build time (Stage 5 concern; see
// docs/SPEC.md §28 Stage 5). Development builds report this placeholder.
const Version = "0.0.0-dev"

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
	case "version", "--version":
		fmt.Fprintln(stdout, "wgtui "+Version)
		return 0
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "wgtui: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
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
  wgtui version

Every command that reaches the network prompts interactively for the SSH
and/or sudo password if key/agent auth isn't enough - wgtui never accepts
either via an argument or an environment variable (SPEC.md §10.3).

See docs/SPEC.md for the full specification.
`)
}
