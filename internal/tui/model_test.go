package tui

import (
	"strings"
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

func TestPeersTabWaitsForDiscoveryThenStartsCurrentServerLoad(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office"})
	m.screen, m.tab, m.loading = screenServerDetail, tabOverview, true
	m.selected = domain.Server{ID: "office"}
	m.detailRequestID = 7

	_, cmd := m.Update(key("tab"))
	if cmd != nil || m.tab != tabPeers || m.peersRefreshing {
		t.Fatalf("tab during discovery started premature load: tab=%v refreshing=%v cmd=%v", m.tab, m.peersRefreshing, cmd)
	}

	sess := &session{cleanup: func() {}}
	_, cmd = m.Update(discoveryMsg{serverID: "office", requestID: 7, sess: sess})
	if cmd == nil || !m.peersRefreshing || m.peersLoaded {
		t.Fatalf("completed discovery did not start peer load: loaded=%v refreshing=%v cmd=%v", m.peersLoaded, m.peersRefreshing, cmd)
	}
}

func TestStaleAsyncServerResultsAreIgnored(t *testing.T) {
	m := testModel(t, domain.Server{ID: "new"})
	m.screen, m.tab = screenServerDetail, tabPeers
	m.selected = domain.Server{ID: "new"}
	m.detailRequestID, m.peerRequestID = 4, 9
	m.peersRefreshing = true

	closed := false
	staleSession := &session{cleanup: func() { closed = true }}
	m.Update(discoveryMsg{serverID: "old", requestID: 3, sess: staleSession, err: "old error"})
	if !closed || m.detailErr != "" {
		t.Fatalf("stale discovery applied or leaked its session: closed=%v detailErr=%q", closed, m.detailErr)
	}

	m.Update(peerListMsg{serverID: "new", requestID: 8, err: "old peer error"})
	if m.peersLoaded || !m.peersRefreshing || m.peersErr != "" {
		t.Fatalf("stale peer result changed current request: loaded=%v refreshing=%v err=%q", m.peersLoaded, m.peersRefreshing, m.peersErr)
	}
}

func TestCurrentPeerFailureAlwaysEndsLoading(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office"})
	m.screen, m.tab = screenServerDetail, tabPeers
	m.selected = domain.Server{ID: "office"}
	m.peerRequestID, m.peersRefreshing = 3, true
	m.Update(peerListMsg{serverID: "office", requestID: 3, err: "SSH connection lost"})
	if !m.peersLoaded || m.peersRefreshing || m.peersErr == "" {
		t.Fatalf("current failure left loading state: loaded=%v refreshing=%v err=%q", m.peersLoaded, m.peersRefreshing, m.peersErr)
	}
	if got := m.viewPeerList(); !strings.Contains(got, "SSH connection lost") {
		t.Fatalf("peer failure is not visible: %s", got)
	}
}

func TestLostPeerConnectionReconnectsOnceThenShowsError(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office"})
	m.screen, m.tab = screenServerDetail, tabPeers
	m.selected = domain.Server{ID: "office"}
	m.peerRequestID, m.peersRefreshing = 3, true
	closed := false
	m.detailSession = &session{cleanup: func() { closed = true }}

	_, cmd := m.Update(peerListMsg{
		serverID: "office", requestID: 3, connectionLost: true, err: "SSH connection was lost",
	})
	if cmd == nil || !closed || !m.loading || !m.peerReconnected || m.peersLoaded {
		t.Fatalf("lost connection did not start one recovery: cmd=%v closed=%v loading=%v reconnected=%v loaded=%v", cmd, closed, m.loading, m.peerReconnected, m.peersLoaded)
	}

	// Simulate the peer command issued after that reconnect. A second loss
	// must settle into an actionable error instead of reconnecting forever.
	m.loading = false
	m.peerRequestID = 4
	m.peersRefreshing = true
	_, cmd = m.Update(peerListMsg{
		serverID: "office", requestID: 4, connectionLost: true, err: "SSH connection was lost again",
	})
	if cmd != nil || !m.peersLoaded || m.peersRefreshing || m.peersErr == "" {
		t.Fatalf("second loss did not settle: cmd=%v loaded=%v refreshing=%v err=%q", cmd, m.peersLoaded, m.peersRefreshing, m.peersErr)
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
