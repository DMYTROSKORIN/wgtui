package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
)

const (
	minimumTerminalWidth  = 64
	minimumTerminalHeight = 18
	recommendedWidth      = 90
	recommendedHeight     = 24
)

// Version is replaced by GoReleaser. Keeping the version visible in the TUI
// makes screenshots actionable: an already-running process is not silently
// mistaken for the binary that was installed over it.
var Version = "0.0.0-dev"

type layoutMode uint8

const (
	layoutTooSmall layoutMode = iota
	layoutCompact
	layoutStandard
	layoutWide
)

func layoutModeFor(width, height int) layoutMode {
	switch {
	case terminalTooSmall(width, height):
		return layoutTooSmall
	case width < recommendedWidth || height < recommendedHeight:
		return layoutCompact
	case width < 135:
		return layoutStandard
	default:
		return layoutWide
	}
}

func terminalTooSmall(width, height int) bool {
	return width < minimumTerminalWidth || height < minimumTerminalHeight
}

func (m *model) View() string {
	if terminalTooSmall(m.width, m.height) {
		return m.viewTerminalTooSmall()
	}
	if m.screen == screenVaultAccess {
		return m.anchorFooter(m.viewVaultAccess())
	}
	if m.screen == screenCredentialPrompt {
		return m.anchorFooter(m.viewCredentialPrompt())
	}
	var body string
	if m.menuOpen {
		body = m.viewCommanderMenu()
		return m.anchorFooter(m.viewCommanderMenuBar() + "\n" + body)
	}
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
		status := ansi.Truncate(style.Render(m.statusMsg), max(1, m.width), "…")
		if split := strings.LastIndex(body, "\n"); split >= 0 {
			body = body[:split] + "\n\n" + status + body[split:]
		} else {
			body += "\n" + status
		}
	}
	return m.anchorFooter(m.viewCommanderMenuBar() + "\n" + body)
}

// anchorFooter keeps the final action/help row on the terminal's physical
// bottom line. Besides making the layout feel stable while resizing, this is
// what keeps rendered buttons and their mouse hit areas on the same row.
func (m *model) anchorFooter(view string) string {
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	missing := m.height - len(lines)
	if missing <= 0 || len(lines) == 0 {
		return strings.Join(lines, "\n")
	}
	footerRows := 1
	if m.statusMsg != "" && len(lines) > 1 {
		footerRows = 2
	}
	insertAt := len(lines) - footerRows
	result := make([]string, 0, m.height)
	result = append(result, lines[:insertAt]...)
	result = append(result, make([]string, missing)...)
	result = append(result, lines[insertAt:]...)
	return strings.Join(result, "\n")
}

func (m *model) viewTerminalTooSmall() string {
	if m.width < 32 || m.height < 8 {
		return ansi.Truncate(
			warningStyle.Render(fmt.Sprintf("Too small: %d×%d; need %d×%d", m.width, m.height, minimumTerminalWidth, minimumTerminalHeight)),
			max(0, m.width), "",
		)
	}
	message := titleStyle.Render("Terminal is too small") + "\n\n" +
		fmt.Sprintf("Current       %d × %d\nMinimum       %d × %d\nRecommended   %d × %d\n\n",
			m.width, m.height, minimumTerminalWidth, minimumTerminalHeight, recommendedWidth, recommendedHeight) +
		warningStyle.Render("Resize the terminal to continue.") + "\n" +
		dimStyle.Render("The current screen and its data are preserved.") + "\n\n" +
		keyHint("Ctrl+C", "Quit")
	return lipgloss.NewStyle().Padding(1, 2).Render(message)
}

func (m *model) viewServerList() string {
	var list strings.Builder
	servers := m.vault.Registry.Servers
	if len(servers) == 0 {
		list.WriteString(dimStyle.Render("No servers yet.") + "\n\n")
		list.WriteString(textStyle.Render("Press ") + selectedStyle.Render("n") + textStyle.Render(" to add one."))
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

	if layoutModeFor(m.width, m.height) == layoutCompact {
		var details string
		if len(servers) > 0 {
			s := servers[m.cursor]
			details = "\n" + dimStyle.Render("Selected") + "  " + textStyle.Render(orDefault(s.DisplayName, s.ID)) +
				"\n" + dimStyle.Render("SSH target") + "  " + textStyle.Render(s.SSHHostOrAlias) +
				"\n\n" + dimStyle.Render("Press Enter to open this server.")
		}
		content := titleStyle.Render("Servers") + "\n\n" + list.String() + details
		return appHeader("Server manager") + "\n" + fullPanel(m, content) + "\n" +
			m.commanderActionBar()
	}

	leftWidth := serverListLeftWidth(m.width)
	rightWidth := max(30, m.width-leftWidth-5)
	panelHeight := m.dashboardPanelHeight()
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
	return appHeader("Server manager") + "\n" + body + "\n" + m.commanderActionBar()
}

func (m *model) viewServerDetail() string {
	panelHeight := m.dashboardPanelHeight()
	header := m.viewServerHeader()
	if layoutModeFor(m.width, m.height) == layoutWide {
		var body string
		switch m.tab {
		case tabPeers:
			peersWidth, inspectorWidth := peerSplitPanelWidths(m.width)
			peers := panelStyle.Width(peersWidth).Height(panelHeight).Render(tabsLine(m.tab) + "\n" + m.viewPeerSummary() + "\n" + m.viewPeerList())
			inspector := panelStyle.Width(inspectorWidth).Height(panelHeight).Render(m.viewSelectedPeerInspector())
			body = lipgloss.JoinHorizontal(lipgloss.Top, peers, " ", inspector)
		case tabDiagnose:
			body = m.viewWideDiagnose(panelHeight)
		default:
			body = m.viewWideOverview(panelHeight)
		}
		return header + "\n" + body + "\n" + m.commanderActionBar()
	}

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

	main := panelStyle.Width(max(40, m.width-4)).Height(panelHeight).Render(content.String())
	return header + "\n" + main + "\n" + m.commanderActionBar()
}

func (m *model) viewServerHeader() string {
	connection := badge(warningStyle, "○", "connecting")
	if m.detailErr != "" {
		connection = badge(dangerStyle, "●", "error")
	} else if m.detailSession != nil && !m.detailSession.closed {
		connection = badge(successStyle, "●", "connected")
	}
	nameWidth := max(12, m.width-lipgloss.Width(appBrand())-lipgloss.Width(connection)-4)
	return appHeader(truncateCell(m.selected.DisplayName, nameWidth)) + "  " + connection
}

// detailTabs lists the server-detail tabs in display order. Shared by
// tabsLine (rendering) and tabClickTarget (mouse hit-testing) so a click
// always lands on the tab it visually appears over.
var detailTabs = []struct {
	tab   detailTab
	label string
}{
	{tabOverview, "Overview"},
	{tabPeers, "Peers"},
	{tabDiagnose, "Diagnose"},
}

const detailTabGap = "   "

func tabsLine(active detailTab) string {
	var parts []string
	for _, t := range detailTabs {
		if t.tab == active {
			parts = append(parts, tabActiveStyle.Render(t.label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(t.label))
		}
	}
	return strings.Join(parts, detailTabGap)
}

// detailTabsRow is the fixed absolute row the tabs line renders on for
// every server-detail layout (compact or wide): row 0 is the top commander
// menu bar, row 1 is the server header, row 2 is the panel's top border,
// row 3 is the panel's first content line - which is always tabsLine.
const detailTabsRow = 3

// detailTabsColumn is the fixed absolute column the tabs line starts at:
// the left panel's own border (1 cell) plus its horizontal padding (1 cell).
const detailTabsColumn = 2

// tabClickTarget returns the tab whose rendered label contains column x
// (measured from the tabs line's own left edge, i.e. subtract
// detailTabsColumn from a mouse event's absolute X first), or ok=false if x
// falls in a gap between tabs or past the last one.
func tabClickTarget(x int) (tab detailTab, ok bool) {
	pos := 0
	for i, t := range detailTabs {
		w := lipgloss.Width(t.label)
		if x >= pos && x < pos+w {
			return t.tab, true
		}
		pos += w
		if i < len(detailTabs)-1 {
			pos += len(detailTabGap)
		}
	}
	return 0, false
}

// serverListRowsRow is the fixed absolute row server rows start at in
// viewServerList, for both the compact and wide layout (both build the
// exact same "Servers\n\n" + one line per server content, just inside a
// differently-sized left panel starting at the same X=0): row 0 top menu
// bar, row 1 appHeader, row 2 panel top border, row 3 "Servers" title,
// row 4 blank line, row 5 the first server.
const serverListRowsRow = 5

// serverListLeftWidth is the wide-layout server list's left (server rows)
// panel width - shared between the renderer and the mouse hit-test so a
// click landing in the details panel to its right can never be mistaken
// for a row click.
func serverListLeftWidth(width int) int {
	return clamp(width/3, 24, 36)
}

// peerTableHeaderRow returns the absolute row bubbles/table's own header
// line renders on, or -1 if the peer table isn't actually being shown
// (still loading, errored, or genuinely empty - viewPeerList shows a plain
// message instead in all of those cases, nothing to click). Derived from
// the same line counts viewServerDetail/viewPeersTab/viewPeerList build,
// not duplicated rendering logic - just how many lines each conditional
// piece contributes.
func (m *model) peerTableHeaderRow() int {
	if !m.peersLoaded || m.peersErr != "" || len(m.peers) == 0 {
		return -1
	}
	// tabsLine -> summary is one "\n" in wide mode's single content
	// string, but two ("\n\n") in compact/standard's tabsLine-then-blank
	// pattern before viewPeersTab's own content starts.
	summaryRow := detailTabsRow + 1
	if layoutModeFor(m.width, m.height) != layoutWide {
		summaryRow++
	}
	row := summaryRow + 1
	if m.peersRefreshing {
		row += 2 // "↻ Refreshing peer state…" + its trailing blank line
	}
	return row
}

// peerTableDataRowAt returns the peers index the table's own header-relative
// visible row y (0 = the row directly under the header) is showing, or
// ok=false. Only safe when every peer fits in the table's own height at
// once - bubbles/table@v1.0.0 exposes no public way to read its internal
// scroll offset from outside the package, so if it might be scrolled
// (len(m.peers) > table height) a click cannot be reliably mapped back to a
// data row and is intentionally left a no-op rather than risk selecting the
// wrong peer.
func (m *model) peerTableDataRowAt(y int) (int, bool) {
	if y < 0 || len(m.peers) > m.peerTable.Height() || y >= len(m.peers) {
		return 0, false
	}
	return y, true
}

func appHeader(section string) string {
	return appBrand() + "  " + subtitleStyle.Render(section)
}

func appBrand() string {
	version := strings.TrimPrefix(Version, "v")
	return titleStyle.Render("wgtui") + " " + dimStyle.Render("v"+version)
}

func keyBar(width int, keys ...string) string {
	parts := make([]string, 0, len(keys))
	for _, hint := range keys {
		key, label, ok := strings.Cut(hint, " ")
		if !ok {
			parts = append(parts, keyStyle.Render(hint))
			continue
		}
		parts = append(parts, keyHint(key, label))
	}
	for len(parts) > 1 && lipgloss.Width(strings.Join(parts, "   ")) > width {
		parts = parts[:len(parts)-1]
	}
	return ansi.Truncate(strings.Join(parts, "   "), max(0, width), "")
}

func keyHint(key, label string) string {
	return keyStyle.Render(key) + " " + footerStyle.Render(label)
}

func formScreen(m *model, title, content string, keys ...string) string {
	return appHeader(title) + "\n" + fullPanel(m, content) + "\n" + keyBar(m.width, keys...)
}

func fullPanel(m *model, content string) string {
	return panelStyle.
		Width(max(40, m.width-4)).
		Height(m.fullPanelHeight()).
		Render(content)
}

func (m *model) fullPanelHeight() int {
	reserved := 6
	if m.statusMsg != "" {
		reserved = 9
	}
	return max(10, m.height-reserved)
}

func (m *model) dashboardPanelHeight() int {
	reserved := 7
	if m.statusMsg != "" {
		reserved = 10
	}
	return max(10, m.height-reserved)
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
	if layoutModeFor(m.width, m.height) == layoutCompact {
		text := titleStyle.Render("Keyboard help") + "\n\n" +
			dimStyle.Render("Alt+letter opens a top menu. Plain letter runs a bottom action.") + "\n\n" +
			keyHint("Alt+S/P/V/T/H", "Top menu") + "\n" +
			keyHint("?", "Help") + "   " + keyHint("N", "New") + "\n" +
			keyHint("O", "Open") + "   " + keyHint("A", "Peer actions") + "\n" +
			keyHint("R", "Refresh") + "   " + keyHint("D", "Diagnose") + "\n" +
			keyHint("M", "Command menu") + "   " + keyHint("Q", "Quit safely") + "\n" +
			keyHint("↑/↓", "Move selection") + "   " + keyHint("Esc", "Back / cancel")
		return appHeader("Help") + "\n" + fullPanel(m, text) + "\n" + keyBar(m.width, "Esc Close", "Ctrl+C Quit")
	}
	text := titleStyle.Render("Keyboard help") + "\n\n" +
		textStyle.Render("Two separate accelerator systems, never the same key:") + "\n" +
		"  Alt+letter   top menu — Server(S) Peer(P) View(V) Tools(T) Help(H)\n" +
		"  letter       bottom action — the highlighted letter inside its label\n\n" +
		textStyle.Render("Server list bottom bar") + "\n" +
		"  N New    O Open    E Edit    R Remove    W Password\n\n" +
		textStyle.Render("Server detail bottom bar") + "\n" +
		"  N New peer   A Actions   O Revoke (Peers tab)   R Refresh\n" +
		"  I Install (if missing)   C Configure   D Diagnose   B Back\n\n" +
		"Arrows/j/k  Move selection       Tab Switch dashboard tab\n" +
		"Enter       Open / continue      Esc Close dialog or go back\n" +
		"M           Command menu         Q Quit safely\n\n" +
		dimStyle.Render("The same letter can mean different things on different screens - e.g. plain O opens the selected server on the server list, while plain O revokes the selected peer on the Peers tab.") + "\n" +
		dimStyle.Render("Mouse clicks on either bar work the same as their keys.") + "\n" +
		dimStyle.Render("Network and destructive operations always show their own status or confirmation.")
	return appHeader("Help") + "\n" + fullPanel(m, text) + "\n" + keyBar(m.width, "Esc Close", "Ctrl+C Quit")
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
		if layoutModeFor(m.width, m.height) == layoutCompact {
			return m.viewCompactOverview()
		}
		b.WriteString(dimStyle.Render("State") + "       " + serverStateBadge(string(m.state)) + "\n\n")
		b.WriteString(dimStyle.Render("System") + "      " + textStyle.Render(orDefault(r.System.Kernel, "(unknown)")) + "\n")
		b.WriteString(dimStyle.Render("OS") + "          " + textStyle.Render(orDefault(r.System.OSID, "?")+" "+r.System.OSVersionID) + "\n")
		b.WriteString(dimStyle.Render("sudo") + "        " + boolStatus(r.System.SudoAvailable, "available", "unavailable") + "\n")
		if r.System.SudoPasswordRejected {
			b.WriteString(dangerStyle.Render("  ✗ That sudo password was rejected by the server.") + "  " + keyHint("r", "Retry") + "\n")
		}
		b.WriteString("\n")

		if !r.WireGuard.WgPresent {
			b.WriteString(warningStyle.Render("WireGuard is not installed on this server.") + "\n")
			b.WriteString(dimStyle.Render("Press i to install it.") + "\n")
		} else {
			b.WriteString(dimStyle.Render("WireGuard") + "   " + successStyle.Render("✓ installed") + "  " + textStyle.Render(orDefault(r.WireGuard.WgVersion, "")) + "\n")
			if len(r.WireGuard.Interfaces) == 0 {
				b.WriteString(warningStyle.Render("  ○ No interfaces configured yet") + "  " + keyHint("c", "Configure") + "\n")
			}
			interfaceNames := sortedInterfaceNames(r)
			limit := max(1, m.dashboardPanelHeight()-14)
			for _, name := range interfaceNames[:min(len(interfaceNames), limit)] {
				d := r.WireGuard.Interfaces[name]
				state := boolStatus(d.Active, "active", "stopped")
				if d.Drift {
					state = dangerStyle.Render("! drift detected")
				}
				b.WriteString("  " + selectedStyle.Render(name) + "  " + state +
					fmt.Sprintf("  peers %d/%d", len(d.RuntimePeerPublicKeys), len(d.PersistentPeerPublicKeys)) + "\n")
			}
			if len(interfaceNames) > limit {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  … %d more interface(s); enlarge the terminal to show them\n", len(interfaceNames)-limit)))
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

func (m *model) viewWideOverview(panelHeight int) string {
	mainWidth, inspectorWidth := peerSplitPanelWidths(m.width)
	main := panelStyle.Width(mainWidth).Height(panelHeight).Render(tabsLine(m.tab) + "\n\n" + m.viewOverviewTab())
	actions := panelStyle.Width(inspectorWidth).Height(panelHeight).Render(m.viewOverviewActions())
	return lipgloss.JoinHorizontal(lipgloss.Top, main, " ", actions)
}

func (m *model) viewOverviewActions() string {
	r := m.report
	var actions []string
	if m.loading {
		actions = append(actions, warningStyle.Render("○ Discovery is running"))
	} else if m.detailErr != "" {
		actions = append(actions, dangerStyle.Render("✗ Reconnect with R"))
	} else {
		if !r.WireGuard.WgPresent {
			actions = append(actions, warningStyle.Render("1. Install WireGuard")+"  "+keyHint("i", "start"))
		} else if len(r.WireGuard.Interfaces) == 0 {
			actions = append(actions, warningStyle.Render("1. Configure an interface")+"  "+keyHint("c", "start"))
		} else {
			actions = append(actions, successStyle.Render("✓ WireGuard is ready"))
		}
		if !r.System.SudoAvailable {
			actions = append(actions, dangerStyle.Render("2. Restore sudo access"))
		}
		if !r.Network.IPv4Forwarding {
			actions = append(actions, warningStyle.Render("3. Enable IPv4 forwarding"))
		}
		if len(r.Errors) > 0 {
			actions = append(actions, warningStyle.Render(fmt.Sprintf("4. Review %d discovery warning(s)", len(r.Errors))))
		}
	}
	if len(actions) == 0 {
		actions = append(actions, successStyle.Render("✓ No immediate action required"))
	}
	return accentHeadingStyle.Render("Recommended actions") + "\n\n" +
		strings.Join(actions, "\n\n") + "\n\n" +
		dimStyle.Render("R refreshes · D opens diagnostics")
}

func (m *model) viewCompactOverview() string {
	r := m.report
	var b strings.Builder
	b.WriteString(dimStyle.Render("State") + "      " + serverStateBadge(string(m.state)) + "\n")
	b.WriteString(dimStyle.Render("OS") + "         " + textStyle.Render(orDefault(r.System.OSID, "?")+" "+r.System.OSVersionID) + "\n")
	b.WriteString(dimStyle.Render("sudo") + "       " + boolStatus(r.System.SudoAvailable, "available", "unavailable") + "\n")
	if r.System.SudoPasswordRejected {
		b.WriteString(dangerStyle.Render("✗ Password rejected") + "  " + keyHint("r", "Retry") + "\n")
	}
	b.WriteString("\n")
	if !r.WireGuard.WgPresent {
		b.WriteString(warningStyle.Render("○ WireGuard is not installed") + "  " + keyHint("i", "Install") + "\n")
	} else if len(r.WireGuard.Interfaces) == 0 {
		b.WriteString(warningStyle.Render("○ No WireGuard interfaces") + "  " + keyHint("c", "Configure") + "\n")
	} else {
		interfaceNames := sortedInterfaceNames(r)
		limit := max(1, m.dashboardPanelHeight()-9)
		for _, name := range interfaceNames[:min(len(interfaceNames), limit)] {
			d := r.WireGuard.Interfaces[name]
			state := boolStatus(d.Active, "active", "stopped")
			if d.Drift {
				state = dangerStyle.Render("! drift")
			}
			b.WriteString(selectedStyle.Render(name) + "  " + state + fmt.Sprintf("  peers %d/%d\n", len(d.RuntimePeerPublicKeys), len(d.PersistentPeerPublicKeys)))
		}
		if len(interfaceNames) > limit {
			b.WriteString(dimStyle.Render(fmt.Sprintf("… %d more interface(s)\n", len(interfaceNames)-limit)))
		}
	}
	b.WriteString("\n" + dimStyle.Render("Outbound") + "   " + textStyle.Render(orDefault(r.Network.DefaultOutboundInterface, "unknown")))
	return b.String()
}

func sortedInterfaceNames(report discovery.Report) []string {
	names := make([]string, 0, len(report.WireGuard.Interfaces))
	for name := range report.WireGuard.Interfaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
		fullPanel(m, b.String()) + "\n" +
		keyBar(m.width, "←/→ Choose", "Enter Confirm", "Esc Cancel", "Ctrl+C Quit")
}

// viewApplying is shown for the entire duration of an async apply step -
// remote work runs asynchronously while SSH/sudo questions appear as
// credential modals inside Bubble Tea's alternate screen.
func (m *model) viewApplying() string {
	return appHeader(m.selected.DisplayName) + "\n" +
		fullPanel(m,
			titleStyle.Render("Applying change")+"\n\n"+textStyle.Render(m.confirmTitle)+"\n\n"+
				dimStyle.Render("Please wait. The operation is validated and rolled back automatically on failure."))
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
