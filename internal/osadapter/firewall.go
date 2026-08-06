package osadapter

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// DetectFirewallBackend identifies the active firewall front-end, in
// priority order (an active service beats a bare CLI tool being merely
// installed). "" means none detected - the caller should fall back to a
// bare nftables/iptables INPUT rule.
func DetectFirewallBackend(ctx context.Context, client *sshtransport.Client) string {
	if isActiveService(ctx, client, "ufw") {
		return "ufw"
	}
	if isActiveService(ctx, client, "firewalld") {
		return "firewalld"
	}
	if commandExists(ctx, client, "nft") {
		return "nftables"
	}
	if commandExists(ctx, client, "iptables") {
		return "iptables"
	}
	return ""
}

func isActiveService(ctx context.Context, client *sshtransport.Client, name string) bool {
	res, err := client.RunCommand(ctx, "systemctl is-active "+name)
	return err == nil && strings.TrimSpace(string(res.Stdout)) == "active"
}

func commandExists(ctx context.Context, client *sshtransport.Client, name string) bool {
	res, err := client.RunCommand(ctx, "command -v "+name)
	return err == nil && res.ExitCode == 0
}

// FirewallPlan is the exact command(s) that would open the WireGuard UDP
// port - shown to the user before ApplyFirewallPlan runs anything
// (SPEC.md §13.5 step "показать план до применения"). Every command only
// *adds* a rule; none of them ever flushes an existing table or chain.
type FirewallPlan struct {
	Backend  string
	Commands []string
}

// PlanOpenPort builds the plan for opening udpPort on backend. NAT/
// forwarding is handled separately via the interface's own PostUp/
// PostDown hooks (osadapter.NewInterfaceSpec.NAT) rather than here - see
// that type's doc comment for why.
func PlanOpenPort(backend string, udpPort int) (FirewallPlan, error) {
	port := strconv.Itoa(udpPort)
	switch backend {
	case "ufw":
		return FirewallPlan{Backend: backend, Commands: []string{
			"ufw allow " + port + "/udp",
		}}, nil
	case "firewalld":
		return FirewallPlan{Backend: backend, Commands: []string{
			"firewall-cmd --permanent --add-port=" + port + "/udp",
			"firewall-cmd --reload",
		}}, nil
	case "nftables":
		return FirewallPlan{Backend: backend, Commands: []string{
			"nft add table inet wgtui",
			"nft add chain inet wgtui input { type filter hook input priority 0 \\; }",
			"nft add rule inet wgtui input udp dport " + port + " accept",
		}}, nil
	case "iptables":
		return FirewallPlan{Backend: backend, Commands: []string{
			"iptables -A INPUT -p udp --dport " + port + " -j ACCEPT",
		}}, nil
	case "":
		return FirewallPlan{}, fmt.Errorf("no firewall backend detected - if one is active but not one of ufw/firewalld/nftables/iptables, open UDP port %s manually", port)
	default:
		return FirewallPlan{}, fmt.Errorf("unsupported firewall backend %q", backend)
	}
}

// IsPortOpen reads the current firewall state for backend and reports
// whether udpPort already looks allowed (SPEC.md §13.5's "прочитать
// существующие правила" step). Callers should skip ApplyFirewallPlan when
// this is true - `ufw allow`/`firewall-cmd --add-port` are themselves
// idempotent, but plain `nft add rule`/`iptables -A` are not: reapplying
// unconditionally would pile up duplicate rules on every re-run of
// `server configure` instead of being safe to repeat.
func IsPortOpen(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, backend string, udpPort int) (bool, error) {
	cmd, ok := statusCommandFor(backend)
	if !ok {
		return false, fmt.Errorf("unsupported firewall backend %q", backend)
	}
	res, err := client.RunSudo(ctx, cmd, prompts)
	if err != nil {
		return false, fmt.Errorf("%s: %w", cmd, err)
	}
	if res.ExitCode != 0 {
		return false, fmt.Errorf("%s exited %d: %s", cmd, res.ExitCode, res.Stderr)
	}
	return portLooksOpen(backend, string(res.Stdout), strconv.Itoa(udpPort)), nil
}

func statusCommandFor(backend string) (string, bool) {
	switch backend {
	case "ufw":
		return "ufw status", true
	case "firewalld":
		return "firewall-cmd --list-ports", true
	case "nftables":
		return "nft list ruleset", true
	case "iptables":
		return "iptables -S INPUT", true
	default:
		return "", false
	}
}

// portLooksOpen is a best-effort text match against each backend's status
// output - split out from IsPortOpen so it can be unit tested with
// captured sample output instead of a live server.
func portLooksOpen(backend, statusOutput, port string) bool {
	switch backend {
	case "ufw":
		return strings.Contains(statusOutput, port+"/udp")
	case "firewalld":
		for _, tok := range strings.Fields(statusOutput) {
			if tok == port+"/udp" {
				return true
			}
		}
		return false
	case "nftables", "iptables":
		return strings.Contains(statusOutput, "dport "+port)
	default:
		return false
	}
}

// ApplyFirewallPlan runs plan's commands in order, stopping at the first
// failure.
func ApplyFirewallPlan(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, plan FirewallPlan) error {
	for _, cmd := range plan.Commands {
		res, err := client.RunSudo(ctx, cmd, prompts)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("%s exited %d: %s", cmd, res.ExitCode, res.Stderr)
		}
	}
	return nil
}
