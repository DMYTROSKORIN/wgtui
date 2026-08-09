package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) updateCommanderMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	event := tea.MouseEvent(msg)
	if event.Button != tea.MouseButtonLeft || event.Action != tea.MouseActionPress {
		return m, nil
	}
	if m.screen == screenVaultAccess || m.screen == screenCredentialPrompt || terminalTooSmall(m.width, m.height) {
		return m, nil
	}

	if event.Y == 0 && m.commanderAvailable() {
		start := 0
		for i, category := range commanderCategories {
			width := commanderCategoryWidth(category)
			if event.X >= start && event.X < start+width {
				m.menuOpen, m.menuCategory, m.menuCursor = true, i, 0
				return m, nil
			}
			start += width
		}
	}

	if !m.menuOpen && m.screen == screenServerDetail && event.Y == detailTabsRow {
		if tab, ok := tabClickTarget(event.X - detailTabsColumn); ok {
			return m, m.selectCommanderTab(tab)
		}
		return m, nil
	}

	// Clicking a server row selects it - the same thing Up/Down already do,
	// not "open"; O/Enter still open it explicitly. viewServerList uses the
	// two-panel layout (a details panel directly to the right of the row
	// list, at the same Y range) for anything other than the compact
	// single-panel layout - not just layoutWide specifically - so a click
	// must stay within the row list's own width there too, or clicking
	// server details text could be misread as picking a different row.
	if !m.menuOpen && m.screen == screenServerList {
		inRowList := true
		if layoutModeFor(m.width, m.height) != layoutCompact {
			inRowList = event.X < serverListLeftWidth(m.width)
		}
		servers := m.vault.Registry.Servers
		start, end := m.serverListRows(m.serverListVisibleRows())
		if row := start + (event.Y - serverListRowsRow); inRowList && row >= start && row < end && row < len(servers) {
			m.cursor = row
			return m, nil
		}
	}

	// Clicking a peer row selects it, mirroring Up/Down there too - exact
	// even while scrolled, since wgtui windows the peer list itself.
	if !m.menuOpen && m.screen == screenServerDetail && m.tab == tabPeers {
		if header := m.peerTableHeaderRow(); header >= 0 {
			if row, ok := m.peerTableDataRowAt(event.Y - header - 1); ok {
				m.peerCursor = row
				m.rebuildPeerTable()
				return m, nil
			}
		}
	}

	if m.menuOpen {
		items := m.commanderMenuItems(m.menuCategory)
		item := event.Y - 2
		start := commanderCategoryStart(m.menuCategory)
		menuWidth := clamp(m.width/2, 38, 72) + 2
		if event.X >= start && event.X < start+menuWidth && item >= 0 && item < len(items) {
			m.menuCursor = item
			if items[item].enabled {
				m.menuOpen = false
				return m, m.dispatchCommanderAction(items[item].action)
			}
			return m, nil
		}
		if event.Y != m.height-1 {
			m.menuOpen = false
			return m, nil
		}
	}

	if event.Y == m.height-1 && (m.screen == screenServerList || m.screen == screenServerDetail || m.menuOpen) {
		hints := m.commanderActionHints()
		if len(hints) == 0 {
			return m, nil
		}
		widths := commanderSegmentWidths(len(hints), m.width)
		start := 0
		for i, hint := range hints {
			width := widths[i]
			if event.X >= start && event.X < start+width {
				if !hint.enabled {
					return m, nil
				}
				switch hint.key {
				case "M":
					if m.menuOpen {
						m.menuOpen = false
					} else {
						m.openCommanderMenu()
					}
					return m, nil
				case "Q":
					return m, m.dispatchCommanderAction(actionQuit)
				}
				if hint.action != "" {
					return m, m.dispatchCommanderAction(hint.action)
				}
				return m, nil
			}
			start += width
		}
	}
	return m, nil
}
