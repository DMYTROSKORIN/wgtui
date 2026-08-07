package tui

import (
	"fmt"
	"strings"
)

func (m *model) View() string {
	var body string
	switch m.screen {
	case screenServerForm:
		body = m.viewServerForm()
	case screenServerRemoveConfirm:
		body = m.viewRemoveConfirm()
	case screenServerDetail:
		body = m.viewServerDetail()
	case screenConfirm:
		body = m.viewConfirm()
	case screenConfigureForm:
		body = m.viewConfigureForm()
	case screenPeerForm:
		body = m.viewPeerForm()
	case screenPeerReveal:
		body = m.viewPeerReveal()
	case screenPeerActionMenu:
		body = m.viewActionMenu()
	case screenRevokeConfirm:
		body = m.viewRevokeConfirm()
	case screenPeerExportForm:
		body = m.viewExportForm()
	default:
		body = m.viewServerList()
	}
	if m.statusMsg != "" {
		style := successStyle
		if m.statusErr {
			style = dangerStyle
		}
		body += "\n\n" + style.Render(m.statusMsg)
	}
	return body
}

func (m *model) viewServerList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("wgtui") + "  " + subtitleStyle.Render("servers") + "\n\n")

	servers := m.vault.Registry.Servers
	if len(servers) == 0 {
		b.WriteString(dimStyle.Render("No servers yet.") + "\n\n")
		b.WriteString(textStyle.Render("Press ") + selectedStyle.Render("[a]") + textStyle.Render(" to add your first one."))
	} else {
		for i, s := range servers {
			line := fmt.Sprintf("%-20s %s@%s", s.DisplayName, orDefault(s.SSHUser, "(default user)"), s.SSHHostOrAlias)
			if i == m.cursor {
				b.WriteString(selectedStyle.Render("▶ "+line) + "\n")
			} else {
				b.WriteString("  " + textStyle.Render(line) + "\n")
			}
		}
	}

	b.WriteString("\n" + footerStyle.Render("[enter] open  [a] add  [e] edit  [x] remove  [↑/↓] move  [q] quit"))
	return b.String()
}

func (m *model) viewServerDetail() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("wgtui") + "  " + subtitleStyle.Render(m.selected.DisplayName) + "\n\n")
	b.WriteString(tabsLine(m.tab) + "\n\n")

	switch m.tab {
	case tabPeers:
		b.WriteString(m.viewPeersTab())
	case tabDiagnose:
		b.WriteString(m.viewDiagnoseTab())
	default:
		b.WriteString(m.viewOverviewTab())
	}

	b.WriteString("\n\n" + footerStyle.Render(detailFooter(m.tab)))
	return b.String()
}

func tabsLine(active detailTab) string {
	tabs := []struct {
		tab   detailTab
		label string
	}{
		{tabOverview, "Overview"},
		{tabPeers, "Peers"},
		{tabDiagnose, "Diagnose"},
	}
	var parts []string
	for _, t := range tabs {
		if t.tab == active {
			parts = append(parts, tabActiveStyle.Render(t.label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(t.label))
		}
	}
	return strings.Join(parts, "   ")
}

func detailFooter(tab detailTab) string {
	switch tab {
	case tabPeers:
		return "[a] add peer  [enter] actions  [tab] switch  [q/esc] back"
	case tabDiagnose:
		return "[r] refresh  [tab] switch  [q/esc] back"
	default:
		return "[i] install  [c] configure  [r] refresh  [tab] switch  [q/esc] back"
	}
}

func (m *model) viewOverviewTab() string {
	var b strings.Builder
	switch {
	case m.loading:
		b.WriteString("Connecting and running discovery...\n")
	case m.detailErr != "":
		b.WriteString(dangerStyle.Render("Error: "+m.detailErr) + "\n")
	default:
		r := m.report
		b.WriteString(fmt.Sprintf("State: %s\n\n", m.state))
		b.WriteString(fmt.Sprintf("System:    %s\n", orDefault(r.System.Kernel, "(unknown)")))
		b.WriteString(fmt.Sprintf("OS:        %s %s\n", orDefault(r.System.OSID, "?"), r.System.OSVersionID))
		b.WriteString(fmt.Sprintf("sudo:      %v\n\n", r.System.SudoAvailable))

		if !r.WireGuard.WgPresent {
			b.WriteString(warningStyle.Render("WireGuard is not installed on this server.") + "\n")
			b.WriteString(dimStyle.Render("Press [i] to install it.") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("WireGuard: present=%v wg-quick=%v %s\n", r.WireGuard.WgPresent, r.WireGuard.WgQuickPresent, orDefault(r.WireGuard.WgVersion, "")))
			if len(r.WireGuard.Interfaces) == 0 {
				b.WriteString(dimStyle.Render("  No interfaces configured yet - press [c] to configure one.\n"))
			}
			for name, d := range r.WireGuard.Interfaces {
				b.WriteString(fmt.Sprintf("  %-8s active=%-5v config=%-5v drift=%-5v peers(runtime/persistent)=%d/%d\n",
					name, d.Active, d.ConfigExists, d.Drift, len(d.RuntimePeerPublicKeys), len(d.PersistentPeerPublicKeys)))
			}
		}

		b.WriteString(fmt.Sprintf("\nNetwork:   outbound=%s ipv4_fwd=%v ipv6_fwd=%v firewall=%s\n",
			orDefault(r.Network.DefaultOutboundInterface, "?"), r.Network.IPv4Forwarding, r.Network.IPv6Forwarding, orDefault(r.Network.FirewallBackend, "none detected")))

		if len(r.Errors) > 0 {
			b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("(%d non-fatal check error(s))\n", len(r.Errors))))
		}
		b.WriteString("\n" + dimStyle.Render("\"online\" is a heuristic based on handshake recency, not proof of an active connection."))
	}
	return b.String()
}

func (m *model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("wgtui") + "\n\n")
	if m.confirmPlan != "" {
		b.WriteString(panelStyle.Render(m.confirmPlan) + "\n\n")
	}
	b.WriteString(m.confirmForm.View())
	return b.String()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
