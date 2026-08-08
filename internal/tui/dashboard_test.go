package tui

import (
	"fmt"
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
	for _, want := range []string{"Alice", "10.10.0.2/32", "2.0M", "512.0K", "Selected peer", "stored securely"} {
		if !strings.Contains(view, want) {
			t.Errorf("peer dashboard does not contain %q:\n%s", want, view)
		}
	}
}

func TestWidePeerDashboardUsesListAndInspectorWithoutServerSidebar(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office", SSHHostOrAlias: "vpn.example.test"})
	m.screen, m.tab = screenServerDetail, tabPeers
	m.selected = domain.Server{ID: "office", DisplayName: "Office", SSHHostOrAlias: "vpn.example.test"}
	m.detailSession = &session{cleanup: func() {}}
	m.peersLoaded = true
	m.peers = []peerview.Peer{{
		Name: "Alice Workstation With A Useful Name", Interface: "wg0", VPNIP: "10.10.0.2/32", PublicKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH",
		Endpoint: "198.51.100.7:51820", PresharedKeyPresent: true, State: "online", Managed: true, ProfileAvailable: true,
	}}
	m.Update(tea.WindowSizeMsg{Width: 166, Height: 34})
	m.rebuildPeerTable()
	view := m.View()
	for _, want := range []string{"Peers", "Selected peer", "Alice Workstation With A Useful Name", "Endpoint", "198.51.100.7:51820", "secret hidden", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH", "connected"} {
		if !strings.Contains(view, want) {
			t.Errorf("wide dashboard does not contain %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "│ Servers") {
		t.Fatalf("internal dashboard still renders the redundant Servers sidebar:\n%s", view)
	}
	peers, inspector := peerSplitPanelWidths(166)
	if peers+inspector+5 != 166 {
		t.Fatalf("split panel widths consume %d cells, want 166", peers+inspector+5)
	}
	if got := m.peerTable.Columns()[0].Width; got < 45 {
		t.Fatalf("Peer column width = %d, want at least 45 at screenshot-like width", got)
	}
	assertViewFits(t, view, 166, 34)
}

func TestPeerDashboardAdaptsWithoutWrappedCells(t *testing.T) {
	wantColumns := map[int]int{64: 3, 78: 3, 120: 6, 166: 4, 204: 6, 240: 7}
	for width, columnCount := range wantColumns {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
			m.screen, m.tab = screenServerDetail, tabPeers
			m.selected = domain.Server{ID: "office", DisplayName: "Office"}
			m.peersLoaded = true
			m.peers = []peerview.Peer{{
				Name: "A very long descriptive workstation peer name used to verify that rows can never wrap", Interface: "wg0", VPNIP: "10.10.0.222/32",
				PublicKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH", Endpoint: "198.51.100.200:56806",
				State: "never_connected", RxBytes: 31 * 1024 * 1024, TxBytes: 124 * 1024 * 1024, Managed: true,
			}}
			m.Update(tea.WindowSizeMsg{Width: width, Height: 34})
			m.rebuildPeerTable()
			view := m.View()
			if len(m.peerTable.Columns()) != columnCount {
				t.Fatalf("columns = %d, want %d", len(m.peerTable.Columns()), columnCount)
			}
			for rowIndex, row := range m.peerTable.Rows() {
				for columnIndex, cell := range row {
					if got, limit := lipgloss.Width(cell), m.peerTable.Columns()[columnIndex].Width; got > limit {
						t.Fatalf("row %d column %d is %d cells, limit %d: %q", rowIndex, columnIndex, got, limit, cell)
					}
				}
			}
			assertViewFits(t, view, width, 34)
			assertPeerTableFits(t, m)
			if strings.Contains(view, "│ Servers") {
				t.Fatal("internal dashboard rendered Servers sidebar")
			}
		})
	}
}

func assertPeerTableFits(t *testing.T, m *model) {
	t.Helper()
	limit := m.peerTableAvailableWidth()
	for i, line := range strings.Split(m.peerTable.View(), "\n") {
		if got := lipgloss.Width(line); got > limit {
			t.Fatalf("peer table line %d is %d cells wide, panel content is %d:\n%s", i+1, got, limit, line)
		}
	}
}

func TestPeerTableFitsEveryWideTerminalWidth(t *testing.T) {
	for width := 135; width <= 260; width++ {
		m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
		m.screen, m.tab = screenServerDetail, tabPeers
		m.selected = domain.Server{ID: "office", DisplayName: "Office"}
		m.peersLoaded = true
		m.peers = []peerview.Peer{{
			Name: "unusually long peer alias", VPNIP: "10.10.0.254/32", State: "never_connected",
			Endpoint: "198.51.100.200:56806", RxBytes: 53 * 1024 * 1024, TxBytes: 124 * 1024 * 1024,
		}}
		m.Update(tea.WindowSizeMsg{Width: width, Height: 34})
		m.rebuildPeerTable()
		assertPeerTableFits(t, m)
		assertViewFits(t, m.View(), width, 34)
	}
}

func TestLongPeerErrorCannotOverwritePanelBorder(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.screen, m.tab = screenServerDetail, tabPeers
	m.selected = domain.Server{ID: "office", DisplayName: "Office"}
	m.peersLoaded = true
	m.peersErr = strings.Repeat("ssh connection diagnostic with unsafe width ", 12)
	for _, width := range []int{64, 90, 135, 204, 240} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 34})
		for i, line := range strings.Split(m.viewPeerList(), "\n") {
			if got, limit := lipgloss.Width(line), m.peerTableAvailableWidth(); got > limit {
				t.Fatalf("width %d error line %d is %d cells, panel allows %d", width, i+1, got, limit)
			}
		}
		assertViewFits(t, m.View(), width, 34)
	}
}

func assertViewFits(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if renderedWidth := lipgloss.Width(line); renderedWidth > width {
			t.Fatalf("dashboard line %d is %d cells wide, terminal is %d:\n%s", i+1, renderedWidth, width, line)
		}
	}
	if len(lines) > height {
		t.Fatalf("dashboard is %d lines tall, terminal is %d", len(lines), height)
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

func TestExportFormDefaultsToNamedWireGuardBundle(t *testing.T) {
	m := testModel(t)
	m.selectedPeer = peerview.Peer{Name: "Dima Office"}
	m.startExportForm()
	if m.exportPath != "Dima_Office_WireGuard" {
		t.Fatalf("exportPath = %q, want Dima_Office_WireGuard", m.exportPath)
	}
}

func TestExportPeerCreatesSecretBundleAndNeverOverwrites(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.selected = domain.Server{ID: "office", DisplayName: "Office"}
	m.selectedPeer = peerview.Peer{Name: "Alice", PublicKey: "alice-key"}
	if err := m.vault.AddPeer(profilevault.PeerMetadata{ServerID: "office", Name: "Alice", PublicKey: "alice-key"}); err != nil {
		t.Fatal(err)
	}
	m.vault.SetProfile("office", "alice-key", "secret-profile")
	dirPath := filepath.Join(t.TempDir(), "Alice_WireGuard")
	m.exportPeerBundle(dirPath)
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("bundle directory mode = %o, want 700", dirInfo.Mode().Perm())
	}
	configPath := filepath.Join(dirPath, "Alice.conf")
	qrPath := filepath.Join(dirPath, "Alice_QR.png")
	for _, path := range []string{configPath, qrPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o, want 600", path, info.Mode().Perm())
		}
	}
	config, err := os.ReadFile(configPath)
	if err != nil || string(config) != "secret-profile" {
		t.Fatalf("config = %q, err=%v", config, err)
	}
	png, err := os.ReadFile(qrPath)
	if err != nil || len(png) < 8 || string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("QR export is not a PNG, len=%d, err=%v", len(png), err)
	}
	m.clearStatus()
	m.exportPeerBundle(dirPath)
	if !m.statusErr || !strings.Contains(m.statusMsg, "already exists") {
		t.Fatalf("second export should refuse existing bundle, status=%q", m.statusMsg)
	}
}
