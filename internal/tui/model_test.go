package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

func testModel(t *testing.T, servers ...domain.Server) *model {
	t.Helper()
	v, err := profilevault.New("test-password-not-used")
	if err != nil {
		t.Fatalf("New vault: %v", err)
	}
	for _, s := range servers {
		if err := v.AddServer(s); err != nil {
			t.Fatalf("AddServer: %v", err)
		}
	}
	return newModel(v, "")
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestCtrlCQuitsFromEveryScreenAndClosesSessions(t *testing.T) {
	for _, screen := range []screen{screenServerList, screenServerForm, screenServerDetail, screenConfirm, screenPeerForm, screenChangePassword} {
		m := testModel(t)
		m.screen = screen
		activeClosed, detailClosed := false, false
		m.activeSession = &session{cleanup: func() { activeClosed = true }}
		m.detailSession = &session{cleanup: func() { detailClosed = true }}
		_, cmd := m.Update(key("ctrl+c"))
		if cmd == nil {
			t.Fatalf("screen %v: Ctrl+C returned no quit command", screen)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("screen %v: Ctrl+C did not return tea.QuitMsg", screen)
		}
		if !activeClosed || !detailClosed {
			t.Fatalf("screen %v: sessions not closed (active=%v detail=%v)", screen, activeClosed, detailClosed)
		}
	}
}

func TestTabUsesExistingDetailSessionWithoutTerminalRelease(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office"})
	m.screen = screenServerDetail
	m.tab = tabOverview
	m.selected = domain.Server{ID: "office"}
	m.detailSession = nil

	res, cmd := m.Update(key("tab"))
	rm := res.(*model)
	if rm.tab != tabPeers {
		t.Fatalf("tab = %v, want tabPeers", rm.tab)
	}
	if cmd == nil {
		t.Fatal("expected an in-TUI peer load command")
	}
	msg := cmd()
	peerMsg, ok := msg.(peerListMsg)
	if !ok || peerMsg.err == "" {
		t.Fatalf("command result = %#v, want a controlled disconnected-session error", msg)
	}
}

func TestServerListCursor_StaysWithinBounds(t *testing.T) {
	m := testModel(t, domain.Server{ID: "a"}, domain.Server{ID: "b"}, domain.Server{ID: "c"})

	if got, _ := m.Update(key("up")); got.(*model).cursor != 0 {
		t.Errorf("cursor moved above 0: %d", got.(*model).cursor)
	}
	m.Update(key("down"))
	m.Update(key("down"))
	res, _ := m.Update(key("down")) // one past the end
	if got := res.(*model).cursor; got != 2 {
		t.Errorf("cursor = %d, want 2 (clamped to last server)", got)
	}
}

func TestServerListCursor_EmptyRegistryIsSafe(t *testing.T) {
	m := testModel(t)
	res, cmd := m.Update(key("enter"))
	if cmd != nil {
		t.Error("enter on an empty server list should not trigger a connect command")
	}
	if res.(*model).screen != screenServerList {
		t.Error("enter on an empty server list should not change screen")
	}
}

func TestEnterOpensServerDetail(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office308", DisplayName: "Office 308"})
	res, cmd := m.Update(key("enter"))
	rm := res.(*model)
	if rm.screen != screenServerDetail {
		t.Errorf("screen = %v, want screenServerDetail", rm.screen)
	}
	if rm.selected.ID != "office308" {
		t.Errorf("selected.ID = %q, want office308", rm.selected.ID)
	}
	if !rm.loading {
		t.Error("expected loading=true right after opening a server")
	}
	if cmd == nil {
		t.Error("expected a connect-and-discover command to be returned")
	}
}

func TestEscFromDetailReturnsToList(t *testing.T) {
	m := testModel(t, domain.Server{ID: "a"})
	m.Update(key("enter"))
	res, _ := m.Update(key("esc"))
	if res.(*model).screen != screenServerList {
		t.Error("esc from detail should return to the server list")
	}
}

func TestQuitFromServerList(t *testing.T) {
	m := testModel(t)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("expected a command from 'q'")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}
