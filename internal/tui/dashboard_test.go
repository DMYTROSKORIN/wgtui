package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
)

func TestServerListRendersTwoPaneDashboardAndKeyBar(t *testing.T) {
	m := testModel(t, domain.Server{
		ID: "office", DisplayName: "Office", SSHHostOrAlias: "vpn.example.test", SSHUser: "admin",
	})
	m.Update(tea.WindowSizeMsg{Width: 110, Height: 32})
	view := m.View()
	for _, want := range []string{"Servers", "Office", "vpn.example.test", "F1 Help", "F10 Quit", "╭"} {
		if !strings.Contains(view, want) {
			t.Errorf("dashboard does not contain %q:\n%s", want, view)
		}
	}
}

func TestPeerDashboardShowsReadableTrafficAndSelectedCard(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.screen = screenServerDetail
	m.tab = tabPeers
	m.selected = domain.Server{ID: "office", DisplayName: "Office"}
	m.peersLoaded = true
	m.peers = []peerview.Peer{{
		Name: "Alice", Interface: "wg0", VPNIP: "10.10.0.2/32", PublicKey: "abcdefghijklmnop",
		State: "online", RxBytes: 2 * 1024 * 1024, TxBytes: 512 * 1024, Managed: true, ProfileAvailable: true,
	}}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m.rebuildPeerTable()
	view := m.View()
	for _, want := range []string{"Alice", "10.10.0.2/32", "2.0 MiB", "512.0 KiB", "Selected peer", "stored securely"} {
		if !strings.Contains(view, want) {
			t.Errorf("peer dashboard does not contain %q:\n%s", want, view)
		}
	}
}

func TestF1HelpAndEscapeReturnToPreviousScreen(t *testing.T) {
	m := testModel(t)
	m.screen = screenServerList
	m.Update(tea.KeyMsg{Type: tea.KeyF1})
	if m.screen != screenHelp || !strings.Contains(m.View(), "Keyboard help") {
		t.Fatal("F1 did not open keyboard help")
	}
	m.Update(key("esc"))
	if m.screen != screenServerList {
		t.Fatalf("Esc returned to screen %v, want server list", m.screen)
	}
}
