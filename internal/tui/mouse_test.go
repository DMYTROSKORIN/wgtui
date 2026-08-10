package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

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

// TestServerListRowClickOpensTheServer matches Enter's own behavior: a
// single click on a server row opens it directly (selects it and starts
// connect+discovery), not just a select-then-a-second-action-required like
// Up/Down. Reported live: with exactly one server in the list (already
// selected by default, nothing else to visibly select), a select-only
// click looked indistinguishable from doing nothing at all.
func TestServerListRowClickOpensTheServer(t *testing.T) {
	m := testModel(t, domain.Server{ID: "a", DisplayName: "Alpha"}, domain.Server{ID: "b", DisplayName: "Bravo"})
	m.width, m.height = 100, 24
	if m.cursor != 0 {
		t.Fatalf("cursor started at %d, want 0", m.cursor)
	}
	m.Update(leftClick(3, serverListRowsRow+1))
	if m.cursor != 1 {
		t.Fatalf("clicking the second server row left cursor=%d, want 1", m.cursor)
	}
	if m.screen != screenServerDetail {
		t.Fatalf("clicking the second server row left screen=%v, want screenServerDetail (a click should open, not just select)", m.screen)
	}
	if m.selected.ID != "b" {
		t.Fatalf("clicking the second server row selected %q, want \"b\"", m.selected.ID)
	}
}

// TestServerListRowsRowMatchesTheActualRenderedRow is a real regression
// test, not a tautology against serverListRowsRow itself (unlike the other
// tests here, which build their click Y from the constant and so can never
// catch the constant drifting out of sync with the renderer). It renders
// the live view and finds the line the first server's own display name
// actually lands on - exactly the class of bug that shipped once already:
// appHeader moved from the top of the screen to the bottom in v0.5.4, and
// serverListRowsRow's hardcoded value (assuming appHeader was still at the
// top) was never updated, silently mapping every server-row mouse click to
// the row above whichever one was actually clicked.
func TestServerListRowsRowMatchesTheActualRenderedRow(t *testing.T) {
	m := testModel(t, domain.Server{ID: "a", DisplayName: "Zzzalpha9k"}, domain.Server{ID: "b", DisplayName: "Bravo"})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	lines := strings.Split(ansi.Strip(m.View()), "\n")
	row := -1
	for i, line := range lines {
		// The selection marker "▶ " (only ever drawn in front of a row-list
		// entry, never the details panel's own title on the right, which
		// shows the same display name unmarked) is what makes this
		// unambiguous.
		if strings.Contains(line, "▶ Zzzalpha9k") {
			row = i
			break
		}
	}
	if row == -1 {
		t.Fatalf("selected server row (\"▶ Zzzalpha9k\") not found in rendered view:\n%s", ansi.Strip(m.View()))
	}
	if row != serverListRowsRow {
		t.Errorf("first server row actually renders at line %d, but serverListRowsRow = %d - mouse clicks on server rows land one off", row, serverListRowsRow)
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
	if m.peerCursor != 1 {
		t.Fatalf("clicking the second peer row left peerCursor=%d, want 1", m.peerCursor)
	}
}

// TestPeerRowClickWorksEvenWhileScrolled is the direct improvement the peer
// list windowing rewrite unlocked: bubbles/table exposes no way to read its
// internal scroll offset from outside the package, so a click used to be
// deliberately left a no-op whenever there were more peers than fit on
// screen at once, to avoid ever selecting the wrong one. Now that wgtui
// tracks its own window (peerWindowStart), a click is exact regardless.
func TestPeerRowClickWorksEvenWhileScrolled(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	m := testModel(t, server)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.screen, m.tab, m.selected = screenServerDetail, tabPeers, server
	m.peersLoaded = true
	for i := 0; i < 12; i++ {
		m.peers = append(m.peers, peerview.Peer{Name: fmt.Sprintf("Peer%02d", i), PublicKey: fmt.Sprintf("pk%02d", i), Managed: true, State: "online"})
	}
	m.peerCursor = 8 // scrolls the window well past the top
	m.rebuildPeerTable()
	if m.peerWindowStart == 0 {
		t.Fatal("test setup does not actually force scrolling")
	}

	header := m.peerTableHeaderRow()
	wantIndex := m.peerWindowStart                    // the peer actually visible in that row, before the click changes anything
	m.Update(leftClick(detailTabsColumn+1, header+1)) // first visible data row
	if m.peerCursor != wantIndex {
		t.Fatalf("clicking the first visible row left peerCursor=%d, want %d (the peer that was actually there)", m.peerCursor, wantIndex)
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
