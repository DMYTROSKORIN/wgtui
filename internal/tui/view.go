package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	case screenApplying:
		body = m.viewApplying()
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
	case screenChangePassword:
		body = m.viewPasswordForm()
	case screenHelp:
		body = m.viewHelp()
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
	var list strings.Builder
	servers := m.vault.Registry.Servers
	if len(servers) == 0 {
		list.WriteString(dimStyle.Render("No servers yet.") + "\n\n")
		list.WriteString(textStyle.Render("Press ") + selectedStyle.Render("F2") + textStyle.Render(" to add one."))
	} else {
		for i, s := range servers {
			line := s.DisplayName
			if i == m.cursor {
				list.WriteString(selectedStyle.Render("▶ "+line) + "\n")
			} else {
				list.WriteString("  " + textStyle.Render(line) + "\n")
			}
		}
	}

	leftWidth := clamp(m.width/3, 24, 36)
	rightWidth := max(30, m.width-leftWidth-5)
	panelHeight := max(10, m.height-6)
	left := panelStyle.Width(leftWidth).Height(panelHeight).Render(titleStyle.Render("Servers") + "\n\n" + list.String())
	var details string
	if len(servers) == 0 {
		details = titleStyle.Render("Welcome") + "\n\n" +
			"wgtui manages standard WireGuard servers over SSH.\n\n" +
			dimStyle.Render("No agent or control service is installed on the server.")
	} else {
		s := servers[m.cursor]
		details = titleStyle.Render(orDefault(s.DisplayName, s.ID)) + "\n\n" +
			fmt.Sprintf("ID          %s\nSSH target  %s\nSSH user    %s\nSSH port    %s\nIdentity    %s\n\n",
				s.ID, s.SSHHostOrAlias, orDefault(s.SSHUser, "from SSH config"),
				func() string {
					if s.SSHPort == 0 {
						return "from SSH config"
					}
					return fmt.Sprint(s.SSHPort)
				}(),
				orDefault(s.SSHIdentityFile, "agent / SSH config")) +
			dimStyle.Render("Enter opens the server dashboard. Credentials are kept only in memory for that session.")
	}
	right := panelStyle.Width(rightWidth).Height(panelHeight).Render(details)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	return appHeader("Server manager") + "\n" + body + "\n" + keyBar("F1 Help", "F2 Add", "F4 Edit", "X Remove", "P Password", "Enter Open", "F10 Quit")
}

func (m *model) viewServerDetail() string {
	var content strings.Builder
	content.WriteString(tabsLine(m.tab) + "\n\n")
	switch m.tab {
	case tabPeers:
		content.WriteString(m.viewPeersTab())
	case tabDiagnose:
		content.WriteString(m.viewDiagnoseTab())
	default:
		content.WriteString(m.viewOverviewTab())
	}

	panelHeight := max(10, m.height-6)
	if m.width < 100 {
		main := panelStyle.Width(max(40, m.width-4)).Height(panelHeight).Render(content.String())
		return appHeader(m.selected.DisplayName) + "\n" + main + "\n" + footerStyle.Render(detailFooter(m.tab))
	}
	sideWidth := clamp(m.width/4, 22, 30)
	mainWidth := max(44, m.width-sideWidth-5)
	side := panelStyle.Width(sideWidth).Height(panelHeight).Render(m.viewServerSidebar())
	main := panelStyle.Width(mainWidth).Height(panelHeight).Render(content.String())
	return appHeader(m.selected.DisplayName) + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, side, " ", main) + "\n" + footerStyle.Render(detailFooter(m.tab))
}

func (m *model) viewServerSidebar() string {
	connection := badge(warningStyle, "○", "connecting")
	if m.detailErr != "" {
		connection = badge(dangerStyle, "●", "error")
	} else if m.detailSession != nil && !m.detailSession.closed {
		connection = badge(successStyle, "●", "connected")
	}
	return titleStyle.Render("Servers") + "\n\n" +
		selectedStyle.Render("▶ "+m.selected.DisplayName) + "\n" +
		dimStyle.Render(m.selected.SSHHostOrAlias) + "\n\n" +
		connection + "\n\n" +
		dimStyle.Render("Tab switches views.\nF5 refreshes explicitly.\nEsc returns to servers.")
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
		return keyBar("F1 Help", "F2 Add peer", "Enter Actions", "F5 Refresh", "Tab Next", "Esc Back", "F10 Quit")
	case tabDiagnose:
		return keyBar("F1 Help", "F5 Refresh", "Tab Next", "Esc Back", "F10 Quit")
	default:
		return keyBar("F1 Help", "I Install", "C Configure", "F5 Refresh", "Tab Next", "Esc Back", "F10 Quit")
	}
}

func appHeader(section string) string {
	return titleStyle.Render("wgtui") + "  " + subtitleStyle.Render(section)
}

func keyBar(keys ...string) string {
	return footerStyle.Render(strings.Join(keys, "   "))
}

func clamp(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func (m *model) viewHelp() string {
	text := titleStyle.Render("Keyboard help") + "\n\n" +
		"F1          Open this help\n" +
		"F2 / a      Add server or peer\n" +
		"F4 / e      Edit selected server\n" +
		"F5 / r      Refresh the current view\n" +
		"F8 / Enter  Actions for selected peer\n" +
		"Tab         Switch dashboard tabs\n" +
		"Arrows/j/k  Move selection\n" +
		"Esc         Close dialog or go back\n" +
		"F10/Ctrl+C  Quit safely\n\n" +
		dimStyle.Render("Network and destructive operations always show their own status or confirmation.")
	return appHeader("Help") + "\n" + panelStyle.Width(min(70, max(40, m.width-4))).Render(text) + "\n" + keyBar("Esc Close", "F10 Quit")
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
	b.WriteString(titleStyle.Render("Review change") + "\n\n")
	if m.confirmPlan != "" {
		b.WriteString(textStyle.Render("Nothing changes until you choose Apply.") + "\n\n")
		b.WriteString(m.confirmPlan + "\n\n")
	}
	b.WriteString(m.confirmForm.View())
	return appHeader(m.selected.DisplayName) + "\n" +
		panelStyle.Width(min(82, max(54, m.width-4))).Render(b.String()) + "\n" +
		keyBar("←/→ Choose", "Enter Confirm", "Esc Cancel", "F10 Quit")
}

// viewApplying is shown for the entire duration of an async apply step -
// the terminal is released during this window (see releaseTerminalAndRun),
// so any interactive host-key/password/sudo prompts render directly on
// top of this text via the real terminal, not through Bubble Tea.
func (m *model) viewApplying() string {
	return appHeader(m.selected.DisplayName) + "\n" +
		panelStyle.Width(min(72, max(44, m.width-4))).Render(
			titleStyle.Render("Applying change")+"\n\n"+textStyle.Render(m.confirmTitle)+"\n\n"+
				dimStyle.Render("Please wait. The operation is backed up, validated and rolled back on failure."))
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
