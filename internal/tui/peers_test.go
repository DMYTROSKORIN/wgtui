package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
)

// Regression test: startPeerForm built the form but initially forgot to
// set m.screen, so pressing 'a' on the Peers tab silently did nothing -
// only caught by manually driving the TUI in a real terminal, not by
// go build/vet/test.
func TestPeerFormKey_SwitchesScreen(t *testing.T) {
	m := testModel(t, domain.Server{ID: "a"})
	m.Update(key("enter"))
	m.screen = screenServerDetail
	m.tab = tabPeers

	res, cmd := m.updateServerDetail(key("a"))
	rm := res.(*model)
	if rm.screen != screenPeerForm {
		t.Errorf("screen = %v, want screenPeerForm", rm.screen)
	}
	if rm.peerForm == nil {
		t.Error("expected peerForm to be built")
	}
	if cmd == nil {
		t.Error("expected peerForm.Init() to be returned")
	}
}

func TestResolvePeerInterface_PrefersMetadata(t *testing.T) {
	// metaInterface set - must return immediately without touching sess
	// (which is nil here; a network fallback would nil-panic).
	got, err := resolvePeerInterface(context.TODO(), nil, "wg1", "wg0")
	if err != nil {
		t.Fatalf("resolvePeerInterface: %v", err)
	}
	if got != "wg1" {
		t.Errorf("got %q, want wg1 (metadata should win over the flag)", got)
	}
}

func TestValidateOptionalPort(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"22", false},
		{"65535", false},
		{"0", true},
		{"70000", true},
		{"not-a-number", true},
	}
	for _, tc := range cases {
		err := validateOptionalPort(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateOptionalPort(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
		}
	}
}

// TestPeerTableDownStaysInSyncWithCursorWhenScrolled guards a real bug:
// forwarding Up/Down key messages straight into bubbles/table's own
// Update() routes them through MoveUp/MoveDown, which - on top of the row
// window every other list in the app uses (cursor moves within a stable
// view, the view only shifts once the cursor would go off top/bottom) -
// *also* nudges the table's separate internal viewport scroll offset,
// producing a second, out-of-step scroll motion. Reported as the peer list
// visibly moving as a whole instead of just the selection, unlike every
// other list. updateServerDetail now drives Up/Down via SetCursor directly
// instead, bypassing MoveUp/MoveDown entirely. This test forces scrolling
// (more peers than the configured table height) and checks the rendered
// table always shows whichever peer is actually selected.
func TestPeerTableDownStaysInSyncWithCursorWhenScrolled(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	m := testModel(t, server)
	m.width, m.height = 100, 20
	m.screen, m.tab, m.selected = screenServerDetail, tabPeers, server
	m.peersLoaded = true
	for i := 0; i < 12; i++ {
		m.peers = append(m.peers, peerview.Peer{
			Name: fmt.Sprintf("Peer%02d", i), PublicKey: fmt.Sprintf("pk%02d", i), Managed: true, State: "online",
		})
	}
	m.rebuildPeerTable()
	if h := m.peerTable.Height(); h >= len(m.peers) {
		t.Fatalf("test setup does not actually force scrolling: table height %d >= %d peers", h, len(m.peers))
	}

	for step := 0; step < len(m.peers)-1; step++ {
		m.updateServerDetail(key("down"))
		if m.peerCursor != step+1 {
			t.Fatalf("step %d: peerCursor=%d, want %d", step, m.peerCursor, step+1)
		}
		want := m.peers[m.peerCursor].Name
		if view := m.peerTable.View(); !strings.Contains(view, want) {
			t.Fatalf("step %d: cursor is on %q but the rendered table doesn't show it:\n%s", step, want, view)
		}
	}
}
