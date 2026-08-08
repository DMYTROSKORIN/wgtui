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
		for i := range commanderCategories {
			start := commanderCategoryStart(i)
			end := start + len(commanderCategories[i]) + 2
			if event.X >= start && event.X < end {
				m.menuOpen, m.menuCategory, m.menuCursor = true, i, 0
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
		base, remainder, start := m.width/len(hints), m.width%len(hints), 0
		for _, hint := range hints {
			width := base
			if remainder > 0 {
				width++
				remainder--
			}
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
