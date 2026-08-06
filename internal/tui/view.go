package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	errStyle      = lipgloss.NewStyle().Bold(true)
)

func (m *model) View() string {
	switch m.screen {
	case screenServerDetail:
		return m.viewServerDetail()
	default:
		return m.viewServerList()
	}
}

func (m *model) viewServerList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("wgtui - servers") + "\n\n")

	servers := m.vault.Registry.Servers
	if len(servers) == 0 {
		b.WriteString(dimStyle.Render("No servers yet. Add one from the command line:\n  wgtui server add <id> --ssh-host <host>\n"))
	} else {
		for i, s := range servers {
			line := fmt.Sprintf("%-20s %s@%s", s.DisplayName, orDefault(s.SSHUser, "(default user)"), s.SSHHostOrAlias)
			if i == m.cursor {
				b.WriteString(selectedStyle.Render("> "+line) + "\n")
			} else {
				b.WriteString("  " + line + "\n")
			}
		}
	}

	b.WriteString("\n" + dimStyle.Render("[enter] Open  [↑/↓] Move  [q] Quit"))
	return b.String()
}

func (m *model) viewServerDetail() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("wgtui - "+m.selected.DisplayName) + "\n\n")

	switch {
	case m.loading:
		b.WriteString("Connecting and running discovery...\n")
	case m.detailErr != "":
		b.WriteString(errStyle.Render("Error: "+m.detailErr) + "\n")
	default:
		r := m.report
		b.WriteString(fmt.Sprintf("State: %s\n\n", m.state))
		b.WriteString(fmt.Sprintf("System:    %s\n", orDefault(r.System.Kernel, "(unknown)")))
		b.WriteString(fmt.Sprintf("OS:        %s %s\n", orDefault(r.System.OSID, "?"), r.System.OSVersionID))
		b.WriteString(fmt.Sprintf("sudo:      %v\n\n", r.System.SudoAvailable))

		b.WriteString(fmt.Sprintf("WireGuard: present=%v wg-quick=%v %s\n", r.WireGuard.WgPresent, r.WireGuard.WgQuickPresent, orDefault(r.WireGuard.WgVersion, "")))
		if len(r.WireGuard.Interfaces) == 0 {
			b.WriteString(dimStyle.Render("  (no interfaces found)\n"))
		}
		for name, d := range r.WireGuard.Interfaces {
			b.WriteString(fmt.Sprintf("  %-8s active=%-5v config=%-5v drift=%-5v peers(runtime/persistent)=%d/%d\n",
				name, d.Active, d.ConfigExists, d.Drift, len(d.RuntimePeerPublicKeys), len(d.PersistentPeerPublicKeys)))
		}

		b.WriteString(fmt.Sprintf("\nNetwork:   outbound=%s ipv4_fwd=%v ipv6_fwd=%v firewall=%s\n",
			orDefault(r.Network.DefaultOutboundInterface, "?"), r.Network.IPv4Forwarding, r.Network.IPv6Forwarding, orDefault(r.Network.FirewallBackend, "none detected")))

		if len(r.Errors) > 0 {
			b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("(%d non-fatal check error(s), see `wgtui server inspect --json` for detail)\n", len(r.Errors))))
		}

		b.WriteString("\n" + dimStyle.Render("\"online\" is a heuristic based on handshake recency, not proof of an active connection (SPEC.md §19)."))
	}

	b.WriteString("\n\n" + dimStyle.Render("[r] Refresh  [q/esc] Back"))
	return b.String()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
