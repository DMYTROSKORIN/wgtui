package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
)

type commanderAction string

const (
	actionOpenServer      commanderAction = "open_server"
	actionNewServer       commanderAction = "new_server"
	actionEditServer      commanderAction = "edit_server"
	actionRemoveServer    commanderAction = "remove_server"
	actionInstallWG       commanderAction = "install_wireguard"
	actionConfigureServer commanderAction = "configure_server"
	actionRefresh         commanderAction = "refresh"
	actionRefreshPeers    commanderAction = "refresh_peers"
	actionNewPeer         commanderAction = "new_peer"
	actionPeerActions     commanderAction = "peer_actions"
	actionPeerRevoke      commanderAction = "peer_revoke"
	actionShowOverview    commanderAction = "show_overview"
	actionShowPeers       commanderAction = "show_peers"
	actionShowDiagnose    commanderAction = "show_diagnose"
	actionShowServerList  commanderAction = "show_server_list"
	actionChangePassword  commanderAction = "change_password"
	actionHelp            commanderAction = "help"
	actionQuit            commanderAction = "quit"
)

type commanderMenuItem struct {
	label       string
	description string
	action      commanderAction
	enabled     bool
	reason      string
}

var commanderCategories = []string{"Server", "Peer", "View", "Tools", "Help"}

func (m *model) commanderAvailable() bool {
	return m.screen == screenServerList || m.screen == screenServerDetail || m.screen == screenPeerActionMenu || m.screen == screenPeerReveal || m.screen == screenHelp
}

func (m *model) openCommanderMenu() {
	m.menuOpen = true
	m.menuCursor = 0
	switch {
	case m.screen == screenServerDetail && m.tab == tabPeers:
		m.menuCategory = 1
	case m.screen == screenServerDetail:
		m.menuCategory = 2
	case m.screen == screenHelp:
		m.menuCategory = 4
	default:
		m.menuCategory = 0
	}
}

func (m *model) updateCommanderMenu(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.commanderMenuItems(m.menuCategory)
	switch key.String() {
	case "esc", "q", "f9":
		m.menuOpen = false
		return m, nil
	case "left", "h":
		m.menuCategory = (m.menuCategory + len(commanderCategories) - 1) % len(commanderCategories)
		m.menuCursor = 0
	case "right", "l", "tab":
		m.menuCategory = (m.menuCategory + 1) % len(commanderCategories)
		m.menuCursor = 0
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(items)-1 {
			m.menuCursor++
		}
	case "home":
		m.menuCursor = 0
	case "end":
		m.menuCursor = max(0, len(items)-1)
	case "enter":
		if m.menuCursor < 0 || m.menuCursor >= len(items) || !items[m.menuCursor].enabled {
			return m, nil
		}
		action := items[m.menuCursor].action
		m.menuOpen = false
		return m, m.dispatchCommanderAction(action)
	}
	return m, nil
}

func (m *model) commanderMenuItems(category int) []commanderMenuItem {
	servers := m.vault.Registry.Servers
	hasServer := m.screen == screenServerList && len(servers) > 0 && m.cursor >= 0 && m.cursor < len(servers)
	onDetail := m.screen == screenServerDetail
	detailReady := onDetail && !m.loading
	hasPeer := onDetail && m.tab == tabPeers && m.peersLoaded && m.peersErr == "" && len(m.peers) > 0
	managedPeer := false
	if hasPeer {
		row := m.peerTable.Cursor()
		managedPeer = row >= 0 && row < len(m.peers) && m.peers[row].Managed
	}

	enabled := func(label, description string, action commanderAction, ok bool, reason string) commanderMenuItem {
		return commanderMenuItem{label: label, description: description, action: action, enabled: ok, reason: reason}
	}
	switch category {
	case 0:
		return []commanderMenuItem{
			enabled("Open selected server", "Connect and open the selected server dashboard.", actionOpenServer, hasServer, "Select a registered server first."),
			enabled("New server…", "Register another SSH target in the encrypted local vault.", actionNewServer, true, ""),
			enabled("Edit selected server…", "Change the selected server's display name or SSH settings.", actionEditServer, hasServer, "Available from the server list."),
			enabled("Remove selected server…", "Remove the registration after an explicit confirmation.", actionRemoveServer, hasServer, "Available from the server list."),
			enabled("Install WireGuard…", "Build and confirm an installation plan for this server.", actionInstallWG, detailReady, "Open a connected server first."),
			enabled("Configure server…", "Build and confirm the WireGuard server configuration.", actionConfigureServer, detailReady, "Open a connected server first."),
			enabled("Refresh server", "Reconnect and refresh the current server state.", actionRefresh, onDetail, "Open a server first."),
		}
	case 1:
		return []commanderMenuItem{
			enabled("New peer…", "Create a peer and continue to QR/export setup.", actionNewPeer, detailReady, "Open a connected server first."),
			enabled("Selected peer actions…", "Open all actions available for the selected peer.", actionPeerActions, hasPeer, "Open Peers and select a peer first."),
			enabled("Revoke selected peer…", "Open the revoke action; a confirmation is still required.", actionPeerRevoke, managedPeer, "Select a peer managed by wgtui first."),
			enabled("Refresh peers", "Open Peers and reload runtime peer state from the server.", actionRefreshPeers, onDetail, "Open a server first."),
		}
	case 2:
		return []commanderMenuItem{
			enabled("Overview", "Show installation, interface and network readiness.", actionShowOverview, onDetail, "Open a server first."),
			enabled("Peers", "Show the peer table and selected-peer inspector.", actionShowPeers, onDetail, "Open a server first."),
			enabled("Diagnose", "Show checks, warnings and remediation context.", actionShowDiagnose, onDetail, "Open a server first."),
			enabled("Refresh current view", "Refresh the data used by the active view.", actionRefresh, onDetail, "Open a server first."),
			enabled("Server list", "Close the server session and return to registered servers.", actionShowServerList, onDetail, "Already at the server list."),
		}
	case 3:
		return []commanderMenuItem{
			enabled("Change vault password…", "Re-encrypt local server and peer metadata with a new master password.", actionChangePassword, m.screen == screenServerList, "Return to the server list first."),
		}
	default:
		return []commanderMenuItem{
			enabled("Keyboard help", "Show navigation, action and safety shortcuts.", actionHelp, true, ""),
			enabled("Quit wgtui", "Close SSH sessions, restore the terminal and quit safely.", actionQuit, true, ""),
		}
	}
}

func (m *model) dispatchCommanderAction(action commanderAction) tea.Cmd {
	switch action {
	case actionOpenServer:
		_, cmd := m.updateServerList(commanderKey("enter"))
		return cmd
	case actionNewServer:
		m.returnToServerList()
		_, cmd := m.updateServerList(commanderKey("n"))
		return cmd
	case actionEditServer:
		_, cmd := m.updateServerList(commanderKey("e"))
		return cmd
	case actionRemoveServer:
		_, cmd := m.updateServerList(commanderKey("d"))
		return cmd
	case actionInstallWG:
		m.tab = tabOverview
		_, cmd := m.updateServerDetail(commanderKey("i"))
		return cmd
	case actionConfigureServer:
		m.tab = tabOverview
		_, cmd := m.updateServerDetail(commanderKey("c"))
		return cmd
	case actionNewPeer:
		if cmd := m.selectCommanderTab(tabPeers); cmd != nil {
			return cmd
		}
		_, cmd := m.updateServerDetail(commanderKey("n"))
		return cmd
	case actionPeerActions:
		if m.tab != tabPeers {
			return m.selectCommanderTab(tabPeers)
		}
		_, cmd := m.openActionMenu()
		return cmd
	case actionPeerRevoke:
		if m.tab != tabPeers {
			return m.selectCommanderTab(tabPeers)
		}
		_, cmd := m.openActionMenu()
		if len(m.actionOptions) > 0 {
			m.actionCursor = len(m.actionOptions) - 1
		}
		return cmd
	case actionShowOverview:
		return m.selectCommanderTab(tabOverview)
	case actionShowPeers:
		return m.selectCommanderTab(tabPeers)
	case actionShowDiagnose:
		return m.selectCommanderTab(tabDiagnose)
	case actionShowServerList:
		m.returnToServerList()
		return nil
	case actionRefresh:
		if m.screen == screenServerDetail {
			_, cmd := m.updateServerDetail(commanderKey("r"))
			return cmd
		}
	case actionRefreshPeers:
		if m.screen != screenServerDetail {
			return nil
		}
		m.tab = tabPeers
		m.peerReconnected = false
		return m.loadPeersFromSessionCmd()
	case actionChangePassword:
		_, cmd := m.updateServerList(commanderKey("p"))
		return cmd
	case actionHelp:
		if m.screen == screenHelp {
			return nil
		}
		m.helpReturn = m.screen
		m.screen = screenHelp
		return nil
	case actionQuit:
		m.closeSessions()
		return tea.Quit
	}
	return nil
}

func commanderKey(value string) tea.KeyMsg {
	if value == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func (m *model) returnToServerList() {
	m.detailRequestID++
	m.peerRequestID++
	m.peersRefreshing = false
	m.peerReconnected = false
	m.detailSession.close()
	m.detailSession = nil
	m.screen = screenServerList
	m.clearStatus()
}

func (m *model) selectCommanderTab(tab detailTab) tea.Cmd {
	if m.screen != screenServerDetail {
		return nil
	}
	m.tab = tab
	m.diagnoseOffset = 0
	switch tab {
	case tabPeers:
		if !m.peersLoaded && !m.loading {
			return m.loadPeersFromSessionCmd()
		}
	case tabDiagnose:
		if !m.checksLoaded {
			m.checks = diagnostics.BuildChecks(m.report, m.vault.ListPeers(m.selected.ID), time.Now())
			m.checksLoaded = true
		}
	}
	return nil
}

func (m *model) handleCommanderFunctionKey(value string) (tea.Cmd, bool) {
	if !m.commanderAvailable() {
		return nil, false
	}
	switch value {
	case "f1":
		return m.dispatchCommanderAction(actionHelp), true
	case "f2":
		if m.screen == screenServerList {
			return m.dispatchCommanderAction(actionNewServer), true
		}
		if m.screen == screenServerDetail {
			return m.dispatchCommanderAction(actionNewPeer), true
		}
	case "f3":
		if m.screen == screenServerList {
			return m.dispatchCommanderAction(actionOpenServer), true
		}
		if m.screen == screenServerDetail && m.tab == tabPeers {
			return m.dispatchCommanderAction(actionPeerActions), true
		}
	case "f4":
		if m.screen == screenServerList {
			return m.dispatchCommanderAction(actionEditServer), true
		}
	case "f5":
		if m.screen == screenServerDetail {
			return m.dispatchCommanderAction(actionRefresh), true
		}
	case "f6":
		if m.screen == screenServerDetail {
			return m.dispatchCommanderAction(actionConfigureServer), true
		}
	case "f7":
		if m.screen == screenServerDetail {
			return m.dispatchCommanderAction(actionShowDiagnose), true
		}
	case "f8":
		if m.screen == screenServerList {
			return m.dispatchCommanderAction(actionRemoveServer), true
		}
		if m.screen == screenServerDetail && m.tab == tabPeers {
			return m.dispatchCommanderAction(actionPeerRevoke), true
		}
	}
	return nil, strings.HasPrefix(value, "f")
}

func (m *model) viewCommanderMenuBar() string {
	parts := make([]string, 0, len(commanderCategories))
	for i, category := range commanderCategories {
		style := lipgloss.NewStyle().Foreground(colorText).Padding(0, 1)
		if m.menuOpen && i == m.menuCategory {
			style = style.Bold(true).Background(colorSelection)
		} else if (!m.menuOpen && i == 0 && m.screen == screenServerList) || (!m.menuOpen && i == 1 && m.screen == screenServerDetail && m.tab == tabPeers) || (!m.menuOpen && i == 2 && m.screen == screenServerDetail && m.tab != tabPeers) {
			style = style.Foreground(colorAccent).Underline(true)
		}
		parts = append(parts, style.Render(category))
	}
	return ansi.Truncate(strings.Join(parts, ""), max(0, m.width), "")
}

func (m *model) viewCommanderMenu() string {
	items := m.commanderMenuItems(m.menuCategory)
	if len(items) == 0 {
		return ""
	}
	cursor := clamp(m.menuCursor, 0, len(items)-1)
	leftWidth := clamp(m.width/3, 28, 42)
	rightWidth := max(30, m.width-leftWidth-5)
	height := m.dashboardPanelHeight()
	var list strings.Builder
	list.WriteString(titleStyle.Render(commanderCategories[m.menuCategory]) + "\n\n")
	for i, item := range items {
		label := item.label
		if !item.enabled {
			label = "○ " + label
		} else {
			label = "  " + label
		}
		label = truncateCell(label, max(8, leftWidth-4))
		switch {
		case i == cursor && item.enabled:
			list.WriteString(menuSelectedStyle.Render(label))
		case i == cursor:
			list.WriteString(lipgloss.NewStyle().Background(colorSelection).Foreground(colorMuted).Render(label))
		case !item.enabled:
			list.WriteString(dimStyle.Render(label))
		default:
			list.WriteString(textStyle.Render(label))
		}
		list.WriteByte('\n')
	}

	selected := items[cursor]
	detail := accentHeadingStyle.Render(selected.label) + "\n\n" +
		wrapPlainText(selected.description, max(20, rightWidth-4), 6) + "\n\n"
	if selected.enabled {
		detail += successStyle.Render("Available") + "\n\n" + dimStyle.Render("Enter runs this action · Esc returns")
	} else {
		detail += warningStyle.Render("Unavailable in the current context") + "\n" +
			wrapPlainText(selected.reason, max(20, rightWidth-4), 4)
	}
	left := panelStyle.Width(leftWidth).Height(height).Render(list.String())
	right := panelStyle.Width(rightWidth).Height(height).Render(detail)
	return appHeader("Command menu") + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n" + m.commanderFunctionBar()
}

func (m *model) commanderFunctionBar() string {
	labels := m.commanderFunctionLabels()
	if m.width <= 0 {
		return ""
	}
	base, remainder := m.width/10, m.width%10
	var b strings.Builder
	for i, label := range labels {
		width := base
		if i < remainder {
			width++
		}
		keyLabel := fmt.Sprintf("F%d", i+1)
		keyWidth := min(width, lipgloss.Width(keyLabel))
		keyText := ansi.Truncate(keyLabel, keyWidth, "")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorText).Background(colorAccent).Width(keyWidth).Render(keyText))
		labelWidth := max(0, width-keyWidth)
		b.WriteString(lipgloss.NewStyle().Foreground(colorText).Background(colorSelection).Width(labelWidth).Render(truncateCell(label, labelWidth)))
	}
	return b.String()
}

func (m *model) commanderFunctionLabels() [10]string {
	labels := [10]string{"Help", "New", "Open", "Edit", "Refresh", "Config", "Diagnose", "Remove", "Menu", "Quit"}
	if m.menuOpen {
		return [10]string{"Help", "", "Select", "", "", "", "", "", "Close", "Quit"}
	}
	if m.screen == screenServerDetail {
		labels[2] = ""
		labels[3] = ""
		labels[7] = ""
		if m.tab == tabPeers {
			labels[2] = "Actions"
			labels[7] = "Revoke"
		}
	} else if m.screen == screenServerList {
		labels[4] = ""
		labels[5] = ""
		labels[6] = ""
	}
	return labels
}
