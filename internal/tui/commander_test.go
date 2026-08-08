package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
)

func TestCommanderMenuOpensNavigatesAndClosesWithoutChangingScreen(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(key("m"))
	if !m.menuOpen || m.menuCategory != 0 || m.screen != screenServerList {
		t.Fatalf("M did not open Server menu in place: open=%v category=%d screen=%v", m.menuOpen, m.menuCategory, m.screen)
	}
	view := ansi.Strip(m.View())
	for _, want := range []string{"Server", "Open selected server", "Enter/click runs", "Esc closes"} {
		if !strings.Contains(view, want) {
			t.Errorf("open menu lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("right"))
	if m.menuCategory != 1 || m.menuCursor != 0 {
		t.Fatalf("right did not switch to Peer menu: category=%d cursor=%d", m.menuCategory, m.menuCursor)
	}
	m.Update(key("esc"))
	if m.menuOpen || m.screen != screenServerList {
		t.Fatalf("Esc did not close menu in place: open=%v screen=%v", m.menuOpen, m.screen)
	}
}

func TestCommanderDisabledActionExplainsWhyAndDoesNothing(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 26})
	m.Update(key("m"))
	view := ansi.Strip(m.View())
	for _, want := range []string{"Open selected server", "Unavailable:", "Select a registered server first"} {
		if !strings.Contains(view, want) {
			t.Errorf("disabled menu action lacks %q:\n%s", want, view)
		}
	}
	m.Update(key("enter"))
	if !m.menuOpen || m.screen != screenServerList {
		t.Fatalf("disabled action changed state: open=%v screen=%v", m.menuOpen, m.screen)
	}
}

func TestMenuLetterAcceleratorDispatchesSelectedAction(t *testing.T) {
	m := testModel(t)
	m.Update(key("m"))
	m.Update(key("n"))
	if m.menuOpen || m.screen != screenServerForm {
		t.Fatalf("N in Server menu produced open=%v screen=%v", m.menuOpen, m.screen)
	}
}

func TestCommanderFunctionKeysReachPrimaryActionsSafely(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}

	t.Run("new server", func(t *testing.T) {
		m := testModel(t, server)
		m.Update(key("f2"))
		if m.screen != screenServerForm {
			t.Fatalf("F2 screen=%v, want server form", m.screen)
		}
	})

	t.Run("open server", func(t *testing.T) {
		m := testModel(t, server)
		_, cmd := m.Update(key("f3"))
		if m.screen != screenServerDetail || m.selected.ID != server.ID || cmd == nil {
			t.Fatalf("F3 did not start server open: screen=%v selected=%q cmd=%v", m.screen, m.selected.ID, cmd)
		}
	})

	t.Run("remove requires confirmation", func(t *testing.T) {
		m := testModel(t, server)
		m.Update(key("f8"))
		if m.screen != screenServerRemoveConfirm {
			t.Fatalf("F8 screen=%v, want remove confirmation", m.screen)
		}
		if len(m.vault.Registry.Servers) != 1 {
			t.Fatal("F8 removed a server before confirmation")
		}
	})

	t.Run("diagnose", func(t *testing.T) {
		m := testModel(t, server)
		m.screen, m.selected = screenServerDetail, server
		m.Update(key("f7"))
		if m.tab != tabDiagnose || !m.checksLoaded || len(m.checks) == 0 {
			t.Fatalf("F7 did not build diagnostics: tab=%v loaded=%v checks=%d", m.tab, m.checksLoaded, len(m.checks))
		}
	})
}

func TestCommanderConfigureMovesToOverviewAndStartsPreparation(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	m := testModel(t, server)
	m.screen, m.tab, m.selected = screenServerDetail, tabDiagnose, server
	m.report.WireGuard.WgPresent = true
	m.detailSession = &session{cleanup: func() {}}
	_, cmd := m.Update(key("f6"))
	if m.tab != tabOverview || cmd == nil {
		t.Fatalf("F6 did not move to Overview and start configure prep: tab=%v cmd=%v", m.tab, cmd)
	}
}

func TestDisabledLetterAcceleratorsCannotBypassReadiness(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	m := testModel(t, server)
	m.screen, m.tab, m.selected = screenServerDetail, tabOverview, server
	m.loading = true

	for _, accelerator := range []string{"i", "c", "n"} {
		before := m.screen
		_, cmd := m.Update(key(accelerator))
		if m.screen != before || cmd != nil {
			t.Fatalf("disabled %q changed screen=%v or returned command=%v", accelerator, m.screen, cmd)
		}
	}
}

func TestF1OnHelpDoesNotLoseOriginalReturnScreen(t *testing.T) {
	m := testModel(t)
	m.Update(key("f1"))
	if m.screen != screenHelp || m.helpReturn != screenServerList {
		t.Fatalf("first F1 produced screen=%v return=%v", m.screen, m.helpReturn)
	}
	m.Update(key("f1"))
	m.Update(key("esc"))
	if m.screen != screenServerList {
		t.Fatalf("repeated F1 trapped help on screen %v", m.screen)
	}
}

func TestStatusBannerKeepsActionBarOnLastClickableRow(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.width, m.height = 120, 30
	m.setStatus("Connected successfully")
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != m.height {
		t.Fatalf("view with status has %d rows, want terminal height %d", len(lines), m.height)
	}
	last := ansi.Strip(lines[len(lines)-1])
	if !strings.Contains(last, "Open") || !strings.Contains(last, "Quit") {
		t.Fatalf("last row is not the action bar: %q", last)
	}
	if !strings.Contains(ansi.Strip(lines[len(lines)-2]), "Connected successfully") {
		t.Fatalf("status is not immediately above the action bar: %q", ansi.Strip(lines[len(lines)-2]))
	}
}

func TestCommanderPeerRevokeOpensActionMenuWithoutApplying(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	m := testModel(t, server)
	m.screen, m.tab, m.selected = screenServerDetail, tabPeers, server
	m.peersLoaded = true
	m.peers = []peerview.Peer{{Name: "Phone", PublicKey: "public-key", Managed: true, State: "online"}}
	m.rebuildPeerTable()
	m.Update(key("f8"))
	if m.screen != screenPeerActionMenu {
		t.Fatalf("F8 screen=%v, want peer action menu", m.screen)
	}
	if len(m.actionOptions) == 0 || m.actionOptions[m.actionCursor].value != "revoke" {
		t.Fatalf("F8 did not select explicit revoke flow: cursor=%d options=%+v", m.actionCursor, m.actionOptions)
	}
}

func TestCommanderActionBarAlwaysFitsAndShowsLetterAccelerators(t *testing.T) {
	for width := minimumTerminalWidth; width <= 300; width++ {
		m := testModel(t)
		m.width = width
		bar := m.commanderActionBar()
		if got := visibleCellWidth(bar); got != width {
			t.Fatalf("width %d rendered %d cells", width, got)
		}
		plain := ansi.Strip(bar)
		if strings.Contains(plain, "F1") || strings.Contains(plain, "F10") {
			t.Fatalf("width %d still exposes function keys: %q", width, plain)
		}
		for _, key := range []string{"?", "N", "O", "E", "R", "P", "M", "Q"} {
			if !strings.Contains(plain, key) {
				t.Fatalf("width %d omits %s: %q", width, key, plain)
			}
		}
	}
}

func TestMissingWireGuardOverviewExposesInstallAccelerator(t *testing.T) {
	m := testModel(t)
	m.screen, m.tab = screenServerDetail, tabOverview
	m.report.WireGuard.WgPresent = false
	hints := m.commanderActionHints()
	for _, hint := range hints {
		if hint.key == "I" && hint.action == actionInstallWG && hint.enabled {
			return
		}
	}
	t.Fatalf("missing-WireGuard action bar lacks enabled Install: %+v", hints)
}

func TestCommanderMenuAndWideDetailPanesFitSupportedSizes(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	for _, size := range [][2]int{{64, 18}, {90, 24}, {134, 24}, {135, 24}, {166, 34}, {240, 60}} {
		t.Run(fmt.Sprintf("menu_%dx%d", size[0], size[1]), func(t *testing.T) {
			m := testModel(t, server)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.Update(key("m"))
			assertViewFits(t, m.View(), size[0], size[1])
		})
	}

	for _, tab := range []detailTab{tabOverview, tabDiagnose} {
		t.Run(fmt.Sprintf("wide_tab_%d", tab), func(t *testing.T) {
			m := testModel(t, server)
			m.Update(tea.WindowSizeMsg{Width: 166, Height: 34})
			m.screen, m.tab, m.selected = screenServerDetail, tab, server
			m.report.WireGuard.WgPresent = true
			m.report.WireGuard.Interfaces = map[string]discovery.InterfaceDetail{"wg0": {Active: true}}
			m.checksLoaded = true
			m.checks = []diagnostics.Check{{Name: "wireguard_tools", Status: diagnostics.StatusPass, Detail: "installed"}}
			view := ansi.Strip(m.View())
			assertViewFits(t, m.View(), 166, 34)
			if strings.Count(view, "╭") != 2 {
				t.Fatalf("wide tab %d is not a stable two-pane layout:\n%s", tab, view)
			}
		})
	}
}

func TestReturningFromModalCommanderMenuClosesDetailSession(t *testing.T) {
	m := testModel(t)
	m.screen = screenPeerActionMenu
	closed := false
	m.detailSession = &session{cleanup: func() { closed = true }}
	m.returnToServerList()
	if !closed || m.detailSession != nil || m.screen != screenServerList {
		t.Fatalf("return leaked detail session: closed=%v session=%v screen=%v", closed, m.detailSession, m.screen)
	}
}
