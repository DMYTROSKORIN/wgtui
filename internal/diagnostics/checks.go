// Package diagnostics builds the SPEC.md §22 diagnose checklist (plus the
// Stage 4 key-age advisory) from a discovery.Report and the local peer
// metadata registry. Shared, network-free logic used by both cli and
// mcpserver so it's derived once, not duplicated per front end - unlike
// the small, stable per-front-end helpers (terminal prompts, vault
// bootstrap) that *are* deliberately duplicated elsewhere in this project,
// this checklist is expected to keep growing new checks over time, which
// makes keeping one copy worth the import.
package diagnostics

import (
	"fmt"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

// Status is deliberately a small closed set, not a free-form string:
// SPEC.md §22 wants each check to land in one of a few plain buckets a
// caller can branch on.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Limitations are the caveats SPEC.md §22 requires diagnose to state
// explicitly, so a pass/fail summary is never mistaken for more certainty
// than a read-only remote check can actually provide.
var Limitations = []string{
	"a stray UDP packet does not get a normal WireGuard response - port checks here only confirm the process is listening locally, not that it is reachable from the internet",
	"a network timeout does not prove a port is closed, and \"connection refused\" on UDP does not reliably distinguish a closed port from a misrouted one",
	"the only real proof a tunnel works end to end is a fresh handshake observed from an actual client",
	"a fresh handshake proves connectivity, not that full-tunnel routing or DNS is configured correctly on the client",
	"the external IP/port must be verified from a connected client, not from the server itself",
}

// RecommendedKeyRotationAge is a soft, informational threshold only -
// SPEC.md §30.9 explicitly declined any peer-expiration *enforcement* in
// MVP. This never disables anything; it only surfaces a "consider
// rotating" hint, the same way a password manager might flag an old
// credential without locking the account.
const RecommendedKeyRotationAge = 180 * 24 * time.Hour

// BuildChecks derives the checklist from data discovery.Discover already
// collected, plus the key-age advisory from the local peer metadata
// registry (localPeers is nil-safe: an empty/unavailable registry just
// means no key-age checks are produced, not an error). Explicitly not
// implemented: NAT/MASQUERADE rule presence, overlapping AllowedIPs,
// duplicate client VPN IPs, endpoint DNS resolution, the external-port UDP
// heuristic, backup health, config file permissions, and local
// profile/key mismatches - most of these need firewall rule parsing that
// doesn't exist yet. Meant to grow more checks over time without changing
// the shape of the ones already here.
func BuildChecks(r discovery.Report, localPeers []profilevault.PeerMetadata, now time.Time) []Check {
	var checks []Check

	checks = append(checks, Check{Name: "ssh_reachable", Status: StatusPass, Detail: "connected"})
	checks = append(checks, Check{Name: "host_key_verified", Status: StatusPass, Detail: "verified during connect (SPEC.md §10.2)"})

	if r.System.SudoAvailable {
		checks = append(checks, Check{Name: "sudo_available", Status: StatusPass})
	} else {
		checks = append(checks, Check{Name: "sudo_available", Status: StatusFail, Detail: "privileged checks below were skipped"})
	}

	if r.WireGuard.WgPresent && r.WireGuard.WgQuickPresent {
		checks = append(checks, Check{Name: "wireguard_tools", Status: StatusPass, Detail: r.WireGuard.WgVersion})
	} else if r.WireGuard.WgPresent {
		checks = append(checks, Check{Name: "wireguard_tools", Status: StatusWarn, Detail: "wg present but wg-quick missing"})
	} else {
		checks = append(checks, Check{Name: "wireguard_tools", Status: StatusFail, Detail: "wg not found"})
	}

	if len(r.WireGuard.Interfaces) == 0 {
		checks = append(checks, Check{Name: "interface_exists", Status: StatusFail})
	}
	for name, d := range r.WireGuard.Interfaces {
		checks = append(checks, interfaceChecks(name, d)...)
	}

	if r.Network.DefaultOutboundInterface != "" {
		checks = append(checks, Check{Name: "outbound_route", Status: StatusPass, Detail: r.Network.DefaultOutboundInterface})
	} else {
		checks = append(checks, Check{Name: "outbound_route", Status: StatusWarn, Detail: "no default route detected"})
	}

	if r.Network.IPv4Forwarding {
		checks = append(checks, Check{Name: "ipv4_forwarding", Status: StatusPass})
	} else {
		checks = append(checks, Check{Name: "ipv4_forwarding", Status: StatusWarn, Detail: "off - required for full-tunnel/routed peers"})
	}

	if r.Network.FirewallBackend != "" {
		checks = append(checks, Check{Name: "firewall_backend", Status: StatusPass, Detail: r.Network.FirewallBackend})
	} else {
		checks = append(checks, Check{Name: "firewall_backend", Status: StatusWarn, Detail: "no supported firewall backend detected"})
	}

	for _, e := range r.Errors {
		checks = append(checks, Check{Name: "discovery_check", Status: StatusWarn, Detail: e})
	}

	for _, p := range localPeers {
		if p.Status != profilevault.PeerStatusActive || p.CreatedAt.IsZero() {
			continue
		}
		age := now.Sub(p.CreatedAt)
		if age > RecommendedKeyRotationAge {
			checks = append(checks, Check{
				Name:   "peer_key_age:" + p.Name,
				Status: StatusWarn,
				Detail: fmt.Sprintf("key is %d days old - consider rotating it (informational only, not enforced)", int(age.Hours()/24)),
			})
		}
	}

	return checks
}

func interfaceChecks(name string, d discovery.InterfaceDetail) []Check {
	var out []Check

	status := StatusFail
	if d.Active {
		status = StatusPass
	}
	out = append(out, Check{Name: "interface_active:" + name, Status: status})

	if !d.ConfigExists {
		out = append(out, Check{Name: "interface_config:" + name, Status: StatusWarn, Detail: "no persistent config file found"})
	}

	if d.Active {
		switch {
		case d.RuntimeListenPort == 0:
			out = append(out, Check{Name: "listen_port:" + name, Status: StatusSkip, Detail: "could not determine runtime listen port"})
		case d.ConfigListenPort != 0 && d.RuntimeListenPort != d.ConfigListenPort:
			out = append(out, Check{Name: "listen_port:" + name, Status: StatusWarn, Detail: fmt.Sprintf("runtime=%d config=%d", d.RuntimeListenPort, d.ConfigListenPort)})
		default:
			out = append(out, Check{Name: "listen_port:" + name, Status: StatusPass, Detail: fmt.Sprintf("%d", d.RuntimeListenPort)})
		}
	}

	if d.Active && d.ConfigExists {
		if d.Drift {
			out = append(out, Check{Name: "runtime_persistent_drift:" + name, Status: StatusWarn, Detail: fmt.Sprintf("%d runtime peers vs %d persistent peers", len(d.RuntimePeerPublicKeys), len(d.PersistentPeerPublicKeys))})
		} else {
			out = append(out, Check{Name: "runtime_persistent_drift:" + name, Status: StatusPass})
		}
	}

	if d.SaveConfig {
		out = append(out, Check{Name: "saveconfig_risk:" + name, Status: StatusWarn, Detail: "SaveConfig=true can silently overwrite manual edits on interface stop (SPEC.md §15.4)"})
	}

	out = append(out, duplicatePeerCheck(name, d))
	return out
}

func duplicatePeerCheck(name string, d discovery.InterfaceDetail) Check {
	seen := make(map[string]bool, len(d.RuntimePeerPublicKeys))
	for _, k := range d.RuntimePeerPublicKeys {
		if seen[k] {
			return Check{Name: "duplicate_peer_keys:" + name, Status: StatusFail, Detail: "duplicate public key in runtime peer set: " + shortenKey(k)}
		}
		seen[k] = true
	}
	return Check{Name: "duplicate_peer_keys:" + name, Status: StatusPass}
}

func shortenKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:8] + "…"
}
