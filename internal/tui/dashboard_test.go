package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerops"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

func TestServerListRendersTwoPaneDashboardAndKeyBar(t *testing.T) {
	m := testModel(t, domain.Server{
		ID: "office", DisplayName: "Office", SSHHostOrAlias: "vpn.example.test", SSHUser: "admin",
	})
	m.Update(tea.WindowSizeMsg{Width: 110, Height: 32})
	view := m.View()
	for _, want := range []string{"Servers", "Office", "vpn.example.test", "? Help", "q Quit", "╭"} {
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

func TestWidePeerDashboardUsesThreePanelsAndRuntimeDetails(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office", SSHHostOrAlias: "vpn.example.test"})
	m.screen, m.tab = screenServerDetail, tabPeers
	m.selected = domain.Server{ID: "office", DisplayName: "Office", SSHHostOrAlias: "vpn.example.test"}
	m.peersLoaded = true
	m.peers = []peerview.Peer{{
		Name: "Alice", Interface: "wg0", VPNIP: "10.10.0.2/32", PublicKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH",
		Endpoint: "198.51.100.7:51820", PresharedKeyPresent: true, State: "online", Managed: true, ProfileAvailable: true,
	}}
	m.Update(tea.WindowSizeMsg{Width: 210, Height: 34})
	m.rebuildPeerTable()
	view := m.View()
	for _, want := range []string{"Servers", "Peers", "Selected peer", "Endpoint", "198.51.100.7:51820", "secret hidden", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH"} {
		if !strings.Contains(view, want) {
			t.Errorf("wide dashboard does not contain %q:\n%s", want, view)
		}
	}
	servers, peers, inspector := widePeerPanelWidths(210)
	if servers+peers+inspector+8 != 210 {
		t.Fatalf("wide panel widths consume %d cells, want 210", servers+peers+inspector+8)
	}
	if len(m.peerTable.Columns()) != 6 {
		t.Fatalf("wide peer table has %d columns, want 6", len(m.peerTable.Columns()))
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if width := lipgloss.Width(line); width > 210 {
			t.Fatalf("wide dashboard line %d is %d cells wide, terminal is 210:\n%s", i+1, width, line)
		}
	}
	if len(lines) > 34 {
		t.Fatalf("wide dashboard is %d lines tall, terminal is 34", len(lines))
	}
}

func TestPeerFormDefaultsToGoogleDNS(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office"})
	m.selected = domain.Server{ID: "office"}
	m.startPeerForm()
	if m.pfDNS != peerops.DefaultDNSCSV {
		t.Fatalf("pfDNS = %q, want %q", m.pfDNS, peerops.DefaultDNSCSV)
	}
}

func TestPeerRefreshKeepsRowsVisibleWithActivityStatus(t *testing.T) {
	m := testModel(t)
	m.peersLoaded = true
	m.peers = []peerview.Peer{{Name: "Alice", State: "idle"}}
	m.rebuildPeerTable()
	_ = m.loadPeersFromSessionCmd()
	view := m.viewPeerList()
	if !strings.Contains(view, "Refreshing peer state") || !strings.Contains(view, "Alice") {
		t.Fatalf("refresh should retain rows and show activity:\n%s", view)
	}
}

func TestPeerRuntimeValuesCannotInjectTerminalControls(t *testing.T) {
	got := terminalText("Alice\x1b[31m host\x1b]52;c;secret\a:51820")
	if strings.ContainsAny(got, "\x1b\a") {
		t.Fatalf("terminalText retained control characters: %q", got)
	}
}

func TestQuestionMarkHelpAndEscapeReturnToPreviousScreen(t *testing.T) {
	m := testModel(t)
	m.screen = screenServerList
	m.Update(key("?"))
	if m.screen != screenHelp || !strings.Contains(m.View(), "Keyboard help") {
		t.Fatal("? did not open keyboard help")
	}
	m.Update(key("esc"))
	if m.screen != screenServerList {
		t.Fatalf("Esc returned to screen %v, want server list", m.screen)
	}
}

func TestActionMenuMovesCursorWithoutMovingOptions(t *testing.T) {
	m := testModel(t)
	m.screen = screenPeerActionMenu
	m.selectedPeer = peerview.Peer{Name: "Alice", Managed: true, State: "online"}
	m.actionOptions = []peerActionOption{{"Pause access", "disable", false}, {"Rotate keys", "rotate", false}, {"Revoke access", "revoke", true}}
	before := append([]peerActionOption(nil), m.actionOptions...)
	m.Update(key("down"))
	if m.actionCursor != 1 {
		t.Fatalf("actionCursor = %d, want 1", m.actionCursor)
	}
	for i := range before {
		if before[i] != m.actionOptions[i] {
			t.Fatalf("option %d moved or changed: before=%+v after=%+v", i, before, m.actionOptions)
		}
	}
}

func TestManagedPeerActionsIncludeStoredProfileQRAndHomeExport(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.selected = domain.Server{ID: "office", DisplayName: "Office"}
	m.peers = []peerview.Peer{{Name: "Alice", PublicKey: "alice-key", Managed: true, State: "online", ProfileAvailable: true}}
	m.rebuildPeerTable()
	m.openActionMenu()
	values := make(map[string]bool)
	for _, option := range m.actionOptions {
		values[option.value] = true
	}
	for _, want := range []string{"qr", "export"} {
		if !values[want] {
			t.Errorf("stored-profile action %q is missing: %+v", want, m.actionOptions)
		}
	}
}

func TestSafeProfileFilename(t *testing.T) {
	for input, want := range map[string]string{
		"Alice Office": "Alice_Office.conf",
		"../../root":   "root.conf",
		"":             "wireguard-peer.conf",
	} {
		if got := safeProfileFilename(input); got != want {
			t.Errorf("safeProfileFilename(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExportPeerCreatesSecretFileAndNeverOverwrites(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.selected = domain.Server{ID: "office", DisplayName: "Office"}
	m.selectedPeer = peerview.Peer{Name: "Alice", PublicKey: "alice-key"}
	if err := m.vault.AddPeer(profilevault.PeerMetadata{ServerID: "office", Name: "Alice", PublicKey: "alice-key"}); err != nil {
		t.Fatal(err)
	}
	m.vault.SetProfile("office", "alice-key", "secret-profile")
	path := filepath.Join(t.TempDir(), "Alice.conf")
	m.exportPeer(path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("export mode = %o, want 600", info.Mode().Perm())
	}
	if err := os.WriteFile(path, []byte("do-not-replace"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.exportPeer(path)
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "do-not-replace" {
		t.Fatalf("existing export was changed: %q, err=%v", raw, err)
	}
}
