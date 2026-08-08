package tui

import (
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
	key         string
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
	for i, item := range items {
		if strings.EqualFold(key.String(), item.key) {
			m.menuCursor = i
			if !item.enabled {
				return m, nil
			}
			m.menuOpen = false
			return m, m.dispatchCommanderAction(item.action)
		}
	}
	switch key.String() {
	case "esc", "m", "f9":
		m.menuOpen = false
		return m, nil
	case "q":
		m.menuOpen = false
		return m, m.dispatchCommanderAction(actionQuit)
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
	detailReady := onDetail && !m.loading && m.detailErr == ""
	wgPresent := detailReady && m.report.WireGuard.WgPresent
	interfaceReady := wgPresent && len(m.report.WireGuard.Interfaces) > 0
	hasPeer := onDetail && m.tab == tabPeers && m.peersLoaded && m.peersErr == "" && len(m.peers) > 0
	managedPeer := false
	if hasPeer {
		row := m.peerTable.Cursor()
		managedPeer = row >= 0 && row < len(m.peers) && m.peers[row].Managed
	}

	enabled := func(key, label, description string, action commanderAction, ok bool, reason string) commanderMenuItem {
		return commanderMenuItem{key: key, label: label, description: description, action: action, enabled: ok, reason: reason}
	}
	switch category {
	case 0:
		return []commanderMenuItem{
			enabled("O", "Open selected server", "Connect and open the selected server dashboard.", actionOpenServer, hasServer, "Select a registered server first."),
			enabled("N", "New server…", "Register another SSH target in the encrypted local vault.", actionNewServer, true, ""),
			enabled("E", "Edit selected server…", "Change the selected server's display name or SSH settings.", actionEditServer, hasServer, "Available from the server list."),
			enabled("X", "Remove selected server…", "Remove the registration after an explicit confirmation.", actionRemoveServer, hasServer, "Available from the server list."),
			enabled("I", "Install WireGuard…", "Build and confirm an installation plan for this server.", actionInstallWG, detailReady && !wgPresent, "Available when the connected server does not have WireGuard."),
			enabled("C", "Configure server…", "Build and confirm the WireGuard server configuration.", actionConfigureServer, wgPresent, "Connect to a server with WireGuard installed first."),
			enabled("R", "Refresh server", "Reconnect and refresh the current server state.", actionRefresh, onDetail, "Open a server first."),
		}
	case 1:
		return []commanderMenuItem{
			enabled("N", "New peer…", "Create a peer and continue to QR/export setup.", actionNewPeer, interfaceReady, "Configure a WireGuard interface first."),
			enabled("A", "Selected peer actions…", "Open all actions available for the selected peer.", actionPeerActions, hasPeer, "Open Peers and select a peer first."),
			enabled("X", "Revoke selected peer…", "Open the revoke action; a confirmation is still required.", actionPeerRevoke, managedPeer, "Select a peer managed by wgtui first."),
			enabled("R", "Refresh peers", "Open Peers and reload runtime peer state from the server.", actionRefreshPeers, onDetail, "Open a server first."),
		}
	case 2:
		return []commanderMenuItem{
			enabled("O", "Overview", "Show installation, interface and network readiness.", actionShowOverview, onDetail, "Open a server first."),
			enabled("P", "Peers", "Show the peer table and selected-peer inspector.", actionShowPeers, onDetail, "Open a server first."),
			enabled("D", "Diagnose", "Show checks, warnings and remediation context.", actionShowDiagnose, onDetail, "Open a server first."),
			enabled("R", "Refresh current view", "Refresh the data used by the active view.", actionRefresh, onDetail, "Open a server first."),
			enabled("B", "Server list", "Close the server session and return to registered servers.", actionShowServerList, onDetail, "Already at the server list."),
		}
	case 3:
		return []commanderMenuItem{
			enabled("P", "Change vault password…", "Re-encrypt local server and peer metadata with a new master password.", actionChangePassword, m.screen == screenServerList, "Return to the server list first."),
		}
	default:
		return []commanderMenuItem{
			enabled("H", "Keyboard help", "Show navigation, action and safety shortcuts.", actionHelp, true, ""),
			enabled("Q", "Quit wgtui", "Close SSH sessions, restore the terminal and quit safely.", actionQuit, true, ""),
		}
	}
}

func (m *model) dispatchCommanderAction(action commanderAction) tea.Cmd {
	if action != actionQuit && action != actionHelp && !m.commanderActionEnabled(action) {
		return nil
	}
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

func (m *model) commanderActionEnabled(action commanderAction) bool {
	for category := range commanderCategories {
		for _, item := range m.commanderMenuItems(category) {
			if item.action == action && item.enabled {
				return true
			}
		}
	}
	return false
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

func (m *model) handleCommanderAccelerator(value string) (tea.Cmd, bool) {
	if !m.commanderAvailable() {
		return nil, false
	}
	for i, key := range []string{"alt+s", "alt+p", "alt+v", "alt+t", "alt+h"} {
		if value == key {
			m.menuOpen, m.menuCategory, m.menuCursor = true, i, 0
			return nil, true
		}
	}
	switch value {
	case "q":
		return m.dispatchCommanderAction(actionQuit), true
	case "?":
		return m.dispatchCommanderAction(actionHelp), true
	}
	if m.screen == screenServerList {
		switch value {
		case "n":
			return m.dispatchCommanderAction(actionNewServer), true
		case "o":
			return m.dispatchCommanderAction(actionOpenServer), true
		case "e":
			return m.dispatchCommanderAction(actionEditServer), true
		case "x":
			return m.dispatchCommanderAction(actionRemoveServer), true
		case "p":
			return m.dispatchCommanderAction(actionChangePassword), true
		}
	}
	if m.screen == screenServerDetail {
		switch value {
		case "i":
			return m.dispatchCommanderAction(actionInstallWG), true
		case "n":
			return m.dispatchCommanderAction(actionNewPeer), true
		case "r":
			return m.dispatchCommanderAction(actionRefresh), true
		case "c":
			return m.dispatchCommanderAction(actionConfigureServer), true
		case "d":
			return m.dispatchCommanderAction(actionShowDiagnose), true
		case "b":
			return m.dispatchCommanderAction(actionShowServerList), true
		case "a":
			if m.tab == tabPeers {
				return m.dispatchCommanderAction(actionPeerActions), true
			}
		case "x":
			if m.tab == tabPeers {
				return m.dispatchCommanderAction(actionPeerRevoke), true
			}
		}
	}
	return nil, false
}

func (m *model) viewCommanderMenuBar() string {
	parts := make([]string, 0, len(commanderCategories)*2)
	for i, category := range commanderCategories {
		active := (m.menuOpen && i == m.menuCategory) ||
			(!m.menuOpen && i == 0 && m.screen == screenServerList) ||
			(!m.menuOpen && i == 1 && m.screen == screenServerDetail && m.tab == tabPeers) ||
			(!m.menuOpen && i == 2 && m.screen == screenServerDetail && m.tab != tabPeers)
		parts = append(parts, renderChromeAccelerator(strings.ToUpper(category[:1]), category, active))
		if i < len(commanderCategories)-1 {
			parts = append(parts, chromeStyle.Render("│"))
		}
	}
	bar := strings.Join(parts, "")
	return chromeStyle.Width(max(0, m.width)).Render(ansi.Truncate(bar, max(0, m.width), ""))
}

func (m *model) viewCommanderMenu() string {
	items := m.commanderMenuItems(m.menuCategory)
	if len(items) == 0 {
		return ""
	}
	cursor := clamp(m.menuCursor, 0, len(items)-1)
	menuWidth := clamp(m.width/2, 38, 72)
	var list strings.Builder
	for i, item := range items {
		list.WriteString(renderCommanderMenuItem(item, i == cursor, menuWidth-4))
		list.WriteByte('\n')
	}

	selected := items[cursor]
	detail := "\n" + dimStyle.Render(strings.Repeat("─", max(8, menuWidth-4))) + "\n" +
		wrapPlainText(selected.description, max(20, menuWidth-4), 4) + "\n"
	if selected.enabled {
		detail += dimStyle.Render("Enter/click runs · Esc closes")
	} else {
		detail += warningStyle.Render("Unavailable: ") + wrapPlainText(selected.reason, max(20, menuWidth-17), 3)
	}
	panel := panelStyle.Width(menuWidth).Render(strings.TrimSuffix(list.String(), "\n") + detail)
	offset := commanderCategoryStart(m.menuCategory)
	panel = strings.Repeat(" ", offset) + panel
	panelLines := strings.Count(panel, "\n") + 1
	blank := max(0, m.height-2-panelLines)
	return panel + strings.Repeat("\n", blank+1) + m.commanderActionBar()
}

func renderCommanderMenuItem(item commanderMenuItem, selected bool, width int) string {
	background := colorChromeKey
	foreground := colorText
	keyForeground := colorWarning
	if selected {
		background = colorChromeAlt
		foreground = colorChromeInk
	} else if item.enabled {
		keyForeground = colorChromeInk
	}
	if !item.enabled {
		foreground, keyForeground = colorMuted, colorMuted
		if selected {
			background = colorChromeKey
		}
	}
	base := lipgloss.NewStyle().Foreground(foreground).Background(background)
	keyStyle := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(keyForeground).Background(background)
	labelWidth := max(0, width-2)
	return base.Render(" ") + keyStyle.Render(item.key) +
		base.Width(labelWidth).Render(truncateCell("  "+item.label, labelWidth))
}

type commanderHint struct {
	key     string
	label   string
	action  commanderAction
	enabled bool
}

func (m *model) commanderActionBar() string {
	hints := m.commanderActionHints()
	if m.width <= 0 {
		return ""
	}
	base, remainder := m.width/len(hints), m.width%len(hints)
	var b strings.Builder
	for i, hint := range hints {
		width := base
		if i < remainder {
			width++
		}
		keyWidth := min(width, lipgloss.Width(hint.key)+1)
		keyStyle, labelStyle := chromeKeyStyle, chromeLabelStyle
		if !hint.enabled {
			keyStyle = keyStyle.Foreground(colorMuted)
			labelStyle = labelStyle.Foreground(colorMuted)
		}
		b.WriteString(keyStyle.Width(keyWidth).Render(" " + hint.key))
		labelWidth := max(0, width-keyWidth-1)
		b.WriteString(labelStyle.Width(labelWidth).Render(truncateCell(hint.label, labelWidth)))
		b.WriteString(lipgloss.NewStyle().Background(colorChromeKey).Width(min(1, width-keyWidth-labelWidth)).Render(" "))
	}
	return b.String()
}

func (m *model) commanderActionHints() []commanderHint {
	if m.menuOpen {
		return []commanderHint{{"↑↓", "Select", "", true}, {"↵", "Run", "", true}, {"M", "Close", "", true}, {"Q", "Quit", actionQuit, true}}
	}
	hints := []commanderHint{{"?", "Help", actionHelp, true}}
	if m.screen == screenServerDetail {
		peerReady := !m.loading && m.detailErr == "" && m.report.WireGuard.WgPresent && len(m.report.WireGuard.Interfaces) > 0
		hints = append(hints, commanderHint{"N", "New peer", actionNewPeer, peerReady})
		if m.tab == tabPeers {
			hints = append(hints, commanderHint{"A", "Actions", actionPeerActions, len(m.peers) > 0}, commanderHint{"X", "Revoke", actionPeerRevoke, len(m.peers) > 0})
		}
		hints = append(hints, commanderHint{"R", "Refresh", actionRefresh, true})
		if m.tab == tabOverview && !m.report.WireGuard.WgPresent {
			hints = append(hints, commanderHint{"I", "Install", actionInstallWG, !m.loading && m.detailErr == ""})
		}
		hints = append(hints, commanderHint{"C", "Configure", actionConfigureServer, !m.loading && m.detailErr == "" && m.report.WireGuard.WgPresent}, commanderHint{"D", "Diagnose", actionShowDiagnose, true}, commanderHint{"B", "Back", actionShowServerList, true})
	} else if m.screen == screenServerList {
		hasServer := len(m.vault.Registry.Servers) > 0
		hints = append(hints, commanderHint{"N", "New", actionNewServer, true}, commanderHint{"O", "Open", actionOpenServer, hasServer}, commanderHint{"E", "Edit", actionEditServer, hasServer}, commanderHint{"X", "Remove", actionRemoveServer, hasServer}, commanderHint{"P", "Password", actionChangePassword, true})
	}
	return append(hints, commanderHint{"M", "Menu", "", true}, commanderHint{"Q", "Quit", actionQuit, true})
}

func renderChromeAccelerator(key, label string, active bool) string {
	background := colorChrome
	if active {
		background = colorChromeKey
	}
	keyStyle := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(colorWarning).Background(background)
	labelStyle := lipgloss.NewStyle().Foreground(colorChromeInk).Background(background)
	rest := strings.TrimPrefix(label, strings.ToUpper(key))
	if rest == label && len(label) > 0 {
		rest = label[1:]
	}
	return labelStyle.Render(" ") + keyStyle.Render(key) + labelStyle.Render(rest+" ")
}

func commanderCategoryStart(category int) int {
	start := 0
	for i := 0; i < category && i < len(commanderCategories); i++ {
		start += len(commanderCategories[i]) + 3
	}
	return start
}
