package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

func leftClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

func TestTopMenuCategoriesAreMouseClickable(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office"})
	m.Update(leftClick(commanderCategoryStart(1)+1, 0))
	if !m.menuOpen || m.menuCategory != 1 {
		t.Fatalf("Peer menu click produced open=%v category=%d", m.menuOpen, m.menuCategory)
	}
	m.Update(leftClick(commanderCategoryStart(4)+1, 0))
	if !m.menuOpen || m.menuCategory != 4 {
		t.Fatalf("Help menu click produced open=%v category=%d", m.menuOpen, m.menuCategory)
	}
}

func TestDropdownItemsAreMouseClickableAndDisabledItemsAreSafe(t *testing.T) {
	m := testModel(t)
	m.Update(leftClick(1, 0))
	m.Update(leftClick(3, 2)) // disabled Open
	if !m.menuOpen || m.screen != screenServerList {
		t.Fatalf("disabled click changed state: open=%v screen=%v", m.menuOpen, m.screen)
	}
	m.Update(leftClick(3, 3)) // New
	if m.menuOpen || m.screen != screenServerForm {
		t.Fatalf("New click produced open=%v screen=%v", m.menuOpen, m.screen)
	}
}

func TestBottomLetterActionIsMouseClickable(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office"})
	m.width, m.height = 120, 30
	hints := m.commanderActionHints()
	base, remainder, x := m.width/len(hints), m.width%len(hints), 0
	for _, hint := range hints {
		width := base
		if remainder > 0 {
			width++
			remainder--
		}
		if hint.key == "O" {
			m.Update(leftClick(x+1, m.height-1))
			if m.screen != screenServerDetail {
				t.Fatalf("Open action click left screen=%v", m.screen)
			}
			return
		}
		x += width
	}
	t.Fatal("Open action hint not found")
}
