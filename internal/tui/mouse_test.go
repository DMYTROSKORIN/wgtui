package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
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

func TestServerListRowIsMouseClickable(t *testing.T) {
	m := testModel(t, domain.Server{ID: "a", DisplayName: "Alpha"}, domain.Server{ID: "b", DisplayName: "Bravo"})
	m.width, m.height = 100, 24
	if m.cursor != 0 {
		t.Fatalf("cursor started at %d, want 0", m.cursor)
	}
	m.Update(leftClick(3, serverListRowsRow+1))
	if m.cursor != 1 {
		t.Fatalf("clicking the second server row left cursor=%d, want 1", m.cursor)
	}
}

func TestServerListRowClickIgnoresTheDetailsPanelInWideLayout(t *testing.T) {
	m := testModel(t, domain.Server{ID: "a", DisplayName: "Alpha"}, domain.Server{ID: "b", DisplayName: "Bravo"})
	m.width, m.height = 100, 24
	// Same Y as the second row, but well past the row list's own width -
	// inside the details panel, not a second click target for row 1.
	m.Update(leftClick(serverListLeftWidth(m.width)+10, serverListRowsRow+1))
	if m.cursor != 0 {
		t.Fatalf("clicking inside the details panel changed cursor to %d, want unchanged 0", m.cursor)
	}
}

func TestPeerTableRowIsMouseClickable(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	m := testModel(t, server)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.screen, m.tab, m.selected = screenServerDetail, tabPeers, server
	m.peersLoaded = true
	m.peers = []peerview.Peer{
		{Name: "Phone", PublicKey: "pk1", Managed: true, State: "online"},
		{Name: "Laptop", PublicKey: "pk2", Managed: true, State: "online"},
	}
	m.rebuildPeerTable()

	header := m.peerTableHeaderRow()
	if header < 0 {
		t.Fatal("peerTableHeaderRow() = -1 with two loaded peers, want a real row")
	}
	m.Update(leftClick(detailTabsColumn+1, header+2)) // second data row
	if got := m.peerTable.Cursor(); got != 1 {
		t.Fatalf("clicking the second peer row left cursor=%d, want 1", got)
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
