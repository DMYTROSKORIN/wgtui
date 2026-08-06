package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

// checkStatus is deliberately a small closed set, not a free-form string:
// SPEC.md §22 wants each diagnose check to land in one of a few plain
// buckets a script can branch on.
type checkStatus string

const (
	checkPass checkStatus = "pass"
	checkWarn checkStatus = "warn"
	checkFail checkStatus = "fail"
	checkSkip checkStatus = "skip"
)

type checkResult struct {
	Name   string      `json:"name"`
	Status checkStatus `json:"status"`
	Detail string      `json:"detail,omitempty"`
}

type diagnoseResult struct {
	State       discovery.State `json:"state"`
	Checks      []checkResult   `json:"checks"`
	Limitations []string        `json:"limitations"`
}

// diagnoseLimitations are the caveats SPEC.md §22 requires diagnose to
// state explicitly, so a pass/fail summary is never mistaken for more
// certainty than a read-only remote check can actually provide.
var diagnoseLimitations = []string{
	"a stray UDP packet does not get a normal WireGuard response - port checks here only confirm the process is listening locally, not that it is reachable from the internet",
	"a network timeout does not prove a port is closed, and \"connection refused\" on UDP does not reliably distinguish a closed port from a misrouted one",
	"the only real proof a tunnel works end to end is a fresh handshake observed from an actual client",
	"a fresh handshake proves connectivity, not that full-tunnel routing or DNS is configured correctly on the client",
	"the external IP/port must be verified from a connected client, not from the server itself",
}

func runDiagnose(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui diagnose: expected the server ID as the first argument")
		return 2
	}
	id := args[0]

	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output structured JSON instead of text")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui diagnose: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}

	v, _, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui diagnose: %v\n", err)
		return 1
	}
	server, ok := v.GetServer(id)
	if !ok {
		fmt.Fprintf(stderr, "wgtui diagnose: no server named %q\n", id)
		return 1
	}

	result, err := inspectServer(stdout, server)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui diagnose: %v\n", err)
		return 1
	}
	diag := diagnoseResult{
		State:       result.State,
		Checks:      buildChecks(result.Report, v.ListPeers(server.ID), time.Now()),
		Limitations: diagnoseLimitations,
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(diag); err != nil {
			fmt.Fprintf(stderr, "wgtui diagnose: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printDiagnoseText(stdout, diag)
	return 0
}

// recommendedKeyRotationAge is a soft, informational threshold only -
// SPEC.md §30.9 explicitly declined any peer-expiration *enforcement* in
// MVP. This never disables anything; it only surfaces a "consider
// rotating" hint in `diagnose`, the same way a password manager might
// flag an old credential without locking the account.
const recommendedKeyRotationAge = 180 * 24 * time.Hour

// buildChecks derives SPEC.md §22 checks from data discovery.Discover
// already collected, plus a key-age advisory (Stage 4) from the local
// peer metadata registry (localPeers is nil-safe: an empty/unavailable
// registry just means no key-age checks are produced, not an error).
// Explicitly not implemented: NAT/MASQUERADE rule presence, overlapping
// AllowedIPs, duplicate client VPN IPs, endpoint DNS resolution, the
// external-port UDP heuristic, backup health, config file permissions,
// and local profile/key mismatches - most of these need firewall rule
// parsing that doesn't exist yet. `wgtui diagnose --json` output is meant
// to grow these checks over time without changing the shape of the ones
// already here.
func buildChecks(r discovery.Report, localPeers []profilevault.PeerMetadata, now time.Time) []checkResult {
	var checks []checkResult

	checks = append(checks, checkResult{Name: "ssh_reachable", Status: checkPass, Detail: "connected"})
	checks = append(checks, checkResult{Name: "host_key_verified", Status: checkPass, Detail: "verified during connect (SPEC.md §10.2)"})

	if r.System.SudoAvailable {
		checks = append(checks, checkResult{Name: "sudo_available", Status: checkPass})
	} else {
		checks = append(checks, checkResult{Name: "sudo_available", Status: checkFail, Detail: "privileged checks below were skipped"})
	}

	if r.WireGuard.WgPresent && r.WireGuard.WgQuickPresent {
		checks = append(checks, checkResult{Name: "wireguard_tools", Status: checkPass, Detail: r.WireGuard.WgVersion})
	} else if r.WireGuard.WgPresent {
		checks = append(checks, checkResult{Name: "wireguard_tools", Status: checkWarn, Detail: "wg present but wg-quick missing"})
	} else {
		checks = append(checks, checkResult{Name: "wireguard_tools", Status: checkFail, Detail: "wg not found"})
	}

	if len(r.WireGuard.Interfaces) == 0 {
		checks = append(checks, checkResult{Name: "interface_exists", Status: checkFail})
	}
	for name, d := range r.WireGuard.Interfaces {
		checks = append(checks, interfaceChecks(name, d)...)
	}

	if r.Network.DefaultOutboundInterface != "" {
		checks = append(checks, checkResult{Name: "outbound_route", Status: checkPass, Detail: r.Network.DefaultOutboundInterface})
	} else {
		checks = append(checks, checkResult{Name: "outbound_route", Status: checkWarn, Detail: "no default route detected"})
	}

	if r.Network.IPv4Forwarding {
		checks = append(checks, checkResult{Name: "ipv4_forwarding", Status: checkPass})
	} else {
		checks = append(checks, checkResult{Name: "ipv4_forwarding", Status: checkWarn, Detail: "off - required for full-tunnel/routed peers"})
	}

	if r.Network.FirewallBackend != "" {
		checks = append(checks, checkResult{Name: "firewall_backend", Status: checkPass, Detail: r.Network.FirewallBackend})
	} else {
		checks = append(checks, checkResult{Name: "firewall_backend", Status: checkWarn, Detail: "no supported firewall backend detected"})
	}

	for _, e := range r.Errors {
		checks = append(checks, checkResult{Name: "discovery_check", Status: checkWarn, Detail: e})
	}

	for _, p := range localPeers {
		if p.Status != profilevault.PeerStatusActive || p.CreatedAt.IsZero() {
			continue
		}
		age := now.Sub(p.CreatedAt)
		if age > recommendedKeyRotationAge {
			checks = append(checks, checkResult{
				Name:   "peer_key_age:" + p.Name,
				Status: checkWarn,
				Detail: fmt.Sprintf("key is %d days old - consider `wgtui peer rotate` (informational only, not enforced)", int(age.Hours()/24)),
			})
		}
	}

	return checks
}

func interfaceChecks(name string, d discovery.InterfaceDetail) []checkResult {
	var out []checkResult

	status := checkFail
	if d.Active {
		status = checkPass
	}
	out = append(out, checkResult{Name: "interface_active:" + name, Status: status})

	if !d.ConfigExists {
		out = append(out, checkResult{Name: "interface_config:" + name, Status: checkWarn, Detail: "no persistent config file found"})
	}

	if d.Active {
		switch {
		case d.RuntimeListenPort == 0:
			out = append(out, checkResult{Name: "listen_port:" + name, Status: checkSkip, Detail: "could not determine runtime listen port"})
		case d.ConfigListenPort != 0 && d.RuntimeListenPort != d.ConfigListenPort:
			out = append(out, checkResult{Name: "listen_port:" + name, Status: checkWarn, Detail: fmt.Sprintf("runtime=%d config=%d", d.RuntimeListenPort, d.ConfigListenPort)})
		default:
			out = append(out, checkResult{Name: "listen_port:" + name, Status: checkPass, Detail: fmt.Sprintf("%d", d.RuntimeListenPort)})
		}
	}

	if d.Active && d.ConfigExists {
		if d.Drift {
			out = append(out, checkResult{Name: "runtime_persistent_drift:" + name, Status: checkWarn, Detail: fmt.Sprintf("%d runtime peers vs %d persistent peers", len(d.RuntimePeerPublicKeys), len(d.PersistentPeerPublicKeys))})
		} else {
			out = append(out, checkResult{Name: "runtime_persistent_drift:" + name, Status: checkPass})
		}
	}

	if d.SaveConfig {
		out = append(out, checkResult{Name: "saveconfig_risk:" + name, Status: checkWarn, Detail: "SaveConfig=true can silently overwrite manual edits on interface stop (SPEC.md §15.4)"})
	}

	out = append(out, duplicatePeerCheck(name, d))
	return out
}

func duplicatePeerCheck(name string, d discovery.InterfaceDetail) checkResult {
	seen := make(map[string]bool, len(d.RuntimePeerPublicKeys))
	for _, k := range d.RuntimePeerPublicKeys {
		if seen[k] {
			return checkResult{Name: "duplicate_peer_keys:" + name, Status: checkFail, Detail: "duplicate public key in runtime peer set: " + shortenKey(k)}
		}
		seen[k] = true
	}
	return checkResult{Name: "duplicate_peer_keys:" + name, Status: checkPass}
}

func printDiagnoseText(out io.Writer, d diagnoseResult) {
	fmt.Fprintf(out, "State: %s\n\n", d.State)
	for _, c := range d.Checks {
		symbol := map[checkStatus]string{checkPass: "OK  ", checkWarn: "WARN", checkFail: "FAIL", checkSkip: "SKIP"}[c.Status]
		line := fmt.Sprintf("[%s] %s", symbol, c.Name)
		if c.Detail != "" {
			line += " - " + c.Detail
		}
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out, "\nLimitations of this check:")
	for _, l := range d.Limitations {
		fmt.Fprintln(out, "  - "+l)
	}
}
