package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
)

func TestLayoutModeBoundaries(t *testing.T) {
	tests := []struct {
		width, height int
		want          layoutMode
	}{
		{63, 30, layoutTooSmall},
		{100, 17, layoutTooSmall},
		{64, 18, layoutCompact},
		{89, 40, layoutCompact},
		{120, 23, layoutCompact},
		{90, 24, layoutStandard},
		{134, 40, layoutStandard},
		{135, 24, layoutWide},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%dx%d", tc.width, tc.height), func(t *testing.T) {
			if got := layoutModeFor(tc.width, tc.height); got != tc.want {
				t.Fatalf("layoutModeFor(%d, %d) = %v, want %v", tc.width, tc.height, got, tc.want)
			}
		})
	}
}

func TestTooSmallScreenExplainsExactRequirementAndPreservesState(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.screen = screenServerDetail
	m.tab = tabPeers
	m.Update(tea.WindowSizeMsg{Width: 52, Height: 15})
	view := ansi.Strip(m.View())
	for _, want := range []string{"Terminal is too small", "52 × 15", "64 × 18", "90 × 24", "Resize the terminal"} {
		if !strings.Contains(view, want) {
			t.Errorf("small-terminal view does not contain %q:\n%s", want, view)
		}
	}
	m.Update(key("n"))
	if m.screen != screenServerDetail || m.tab != tabPeers {
		t.Fatalf("input changed hidden screen while terminal was too small: screen=%v tab=%v", m.screen, m.tab)
	}
	m.Update(key("?"))
	if m.screen != screenServerDetail {
		t.Fatalf("help key changed hidden screen while terminal was too small: %v", m.screen)
	}
}

func TestTooSmallScreenStillProcessesCompletedBackgroundWork(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 52, Height: 15})
	m.peersRefreshing = true
	m.Update(peerListMsg{peers: []peerview.Peer{{Name: "Completed", State: "online"}}})
	if !m.peersLoaded || m.peersRefreshing || len(m.peers) != 1 || m.peers[0].Name != "Completed" {
		t.Fatalf("small-terminal guard discarded background result: loaded=%v refreshing=%v peers=%+v", m.peersLoaded, m.peersRefreshing, m.peers)
	}
}

func TestTinyTerminalFallbackNeverExceedsAvailableCells(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {5, 2}, {20, 4}, {31, 7}} {
		m := testModel(t)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := m.View()
		if lines := strings.Split(view, "\n"); len(lines) > size[1] {
			t.Fatalf("%dx%d fallback uses %d lines", size[0], size[1], len(lines))
		}
		for _, line := range strings.Split(view, "\n") {
			if width := visibleCellWidth(line); width > size[0] {
				t.Fatalf("%dx%d fallback line is %d cells: %q", size[0], size[1], width, ansi.Strip(line))
			}
		}
	}
}

func TestResizeClearsStaleFramesOnShrinkAndBreakpointChanges(t *testing.T) {
	m := testModel(t)
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 110, Height: 30}); cmd != nil {
		t.Fatal("growing inside one layout mode unexpectedly requested a clear")
	}
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 105, Height: 30}); cmd == nil {
		t.Fatal("shrinking width must request a full screen clear")
	}
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 140, Height: 30}); cmd == nil {
		t.Fatal("crossing a layout breakpoint must request a full screen clear")
	}
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 140, Height: 26}); cmd == nil {
		t.Fatal("shrinking height must request a full screen clear")
	}
}

func TestResponsiveCoreScreensAlwaysFitSupportedTerminal(t *testing.T) {
	sizes := [][2]int{{64, 18}, {72, 20}, {89, 23}, {90, 24}, {120, 30}, {134, 24}, {135, 24}, {166, 34}, {240, 60}}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			server := domain.Server{ID: "office", DisplayName: "A descriptive office server", SSHHostOrAlias: "vpn.example.test", SSHUser: "admin"}
			m := testModel(t, server)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			assertViewFits(t, m.View(), size[0], size[1])

			m.screen, m.tab, m.selected = screenServerDetail, tabPeers, server
			m.peersLoaded = true
			m.peers = []peerview.Peer{{
				Name: "A very long peer name that must never wrap into another row", Interface: "wg0", VPNIP: "10.10.0.222/32",
				PublicKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH", Endpoint: "[2001:db8:ffff:ffff::1234]:56806",
				State: "idle", Managed: false, RxBytes: 31 << 20, TxBytes: 124 << 20,
			}}
			m.rebuildPeerTable()
			assertViewFits(t, m.View(), size[0], size[1])

			m.screen, m.revealName, m.revealProfile = screenPeerReveal, "Mobile Client", "[Interface]\nPrivateKey = secret"
			m.revealNew, m.revealMode = true, ""
			m.setStatus("Added Mobile Client. Client profile saved in the encrypted vault.")
			assertViewFits(t, m.View(), size[0], size[1])
		})
	}
}

func TestCompactStaticAndFormScreensFitMinimumTerminal(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office", SSHHostOrAlias: "vpn.example.test"}
	tests := []struct {
		name  string
		setup func(*model)
	}{
		{"help", func(m *model) { m.screen = screenHelp }},
		{"overview", func(m *model) { m.screen, m.tab, m.selected = screenServerDetail, tabOverview, server }},
		{"peer action menu", func(m *model) {
			m.screen, m.selected, m.selectedPeer = screenPeerActionMenu, server, peerview.Peer{Name: "Mobile", Managed: true, State: "online"}
			m.actionOptions = []peerActionOption{{"Pause access (profile is kept)", "disable", false}, {"Rotate keys and replace client profile", "rotate", false}, {"Show QR code", "qr", false}, {"Save profile + QR bundle to home directory", "export", false}, {"Revoke access and optionally delete local data", "revoke", true}}
		}},
		{"server form", func(m *model) { m.startServerForm(false) }},
		{"peer form", func(m *model) { m.selected = server; m.startPeerForm() }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel(t, server)
			m.Update(tea.WindowSizeMsg{Width: minimumTerminalWidth, Height: minimumTerminalHeight})
			tc.setup(m)
			assertViewFits(t, m.View(), minimumTerminalWidth, minimumTerminalHeight)
		})
	}
}

func TestCompactServerListUsesSinglePanel(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office", SSHHostOrAlias: "vpn.example.test"})
	m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Selected") || !strings.Contains(view, "Press Enter") {
		t.Fatalf("compact server list lacks selected-server guidance:\n%s", view)
	}
	if strings.Count(view, "╭") != 1 {
		t.Fatalf("compact server list should render one panel, got:\n%s", view)
	}
}

func TestKeyBarAccentsKeysAndNeverExceedsWidth(t *testing.T) {
	for _, width := range []int{1, 5, 20, 40, 64, 90, 160} {
		got := keyBar(width, "Enter Actions", "n New peer", "r Refresh", "? Help", "Esc Back", "Ctrl+C Quit")
		if visible := ansi.Strip(got); len([]rune(visible)) == 0 {
			t.Fatalf("keyBar(%d) is empty", width)
		}
		if renderedWidth := visibleCellWidth(got); renderedWidth > width {
			t.Fatalf("keyBar(%d) is %d cells wide: %q", width, renderedWidth, ansi.Strip(got))
		}
	}
	if fmt.Sprint(keyStyle.GetForeground()) == fmt.Sprint(footerStyle.GetForeground()) {
		t.Fatal("key hints and descriptions use the same color")
	}
}

func visibleCellWidth(value string) int {
	return lipgloss.Width(value)
}

func TestPeerRevealGuidesUserThroughEveryNextStep(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.selected = domain.Server{ID: "office", DisplayName: "Office"}
	m.screen, m.revealNew, m.revealName, m.revealProfile = screenPeerReveal, true, "Phone", "secret-profile"
	view := ansi.Strip(m.View())
	for _, want := range []string{"Peer “Phone” is ready", "Next: import", "How do you want", "Show QR code", "Export profile + QR", "Finish for now"} {
		if !strings.Contains(view, want) {
			t.Errorf("post-create screen does not contain %q:\n%s", want, view)
		}
	}

	m.Update(key("down"))
	if m.revealCursor != 1 {
		t.Fatalf("down selected %d, want export choice", m.revealCursor)
	}
	m.Update(key("down"))
	m.Update(key("enter"))
	if m.revealMode != "text" {
		t.Fatalf("configuration choice selected mode %q, want text", m.revealMode)
	}
	m.Update(key("esc"))
	if m.screen != screenPeerReveal || m.revealMode != "" {
		t.Fatalf("Esc from secret view did not return to setup choices: screen=%v mode=%q", m.screen, m.revealMode)
	}
	m.revealCursor = len(clientSetupChoices) - 1
	m.Update(key("enter"))
	if m.screen != screenServerDetail || m.revealNew {
		t.Fatalf("Finish did not return to dashboard: screen=%v revealNew=%v", m.screen, m.revealNew)
	}
}

func TestPeerRevealShortQRScreenOffersSafeAlternative(t *testing.T) {
	m := testModel(t)
	m.screen, m.revealNew, m.revealName = screenPeerReveal, true, "Phone"
	m.revealMode = "qr"
	m.revealProfile = "[Interface]\nPrivateKey = 01234567890123456789012345678901234567890123\nAddress = 10.0.0.2/32"
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "not tall enough") || !strings.Contains(view, "export the profile") {
		t.Fatalf("short QR screen lacks safe recovery guidance:\n%s", view)
	}
	assertViewFits(t, m.View(), 90, 24)
}

func TestStoredProfileRevealBackReturnsToDashboard(t *testing.T) {
	m := testModel(t)
	m.screen, m.revealNew, m.revealMode = screenPeerReveal, false, "qr"
	m.Update(key("esc"))
	if m.screen != screenServerDetail {
		t.Fatalf("stored-profile reveal returned to %v, want dashboard", m.screen)
	}
}

func TestDiagnoseViewScrollsWithoutOverflowing(t *testing.T) {
	m := testModel(t)
	m.screen, m.tab, m.checksLoaded = screenServerDetail, tabDiagnose, true
	for i := range 40 {
		m.checks = append(m.checks, diagnostics.Check{Name: fmt.Sprintf("check_%02d", i), Status: diagnostics.StatusPass, Detail: strings.Repeat("long detail ", 20)})
	}
	m.Update(tea.WindowSizeMsg{Width: 64, Height: 18})
	first := ansi.Strip(m.View())
	if !strings.Contains(first, "Rows 1–") || !strings.Contains(first, "scroll") {
		t.Fatalf("overflowing diagnostics do not advertise scrolling:\n%s", first)
	}
	assertViewFits(t, m.View(), 64, 18)
	m.Update(key("down"))
	if m.diagnoseOffset != 1 {
		t.Fatalf("diagnostic scroll offset = %d, want 1", m.diagnoseOffset)
	}
	second := ansi.Strip(m.View())
	if first == second || !strings.Contains(second, "Rows 2–") {
		t.Fatalf("diagnostic view did not scroll:\n%s", second)
	}
	for range 100 {
		m.Update(key("down"))
	}
	if maxOffset := len(m.diagnoseLines()) - m.diagnoseVisibleRows(); m.diagnoseOffset != maxOffset {
		t.Fatalf("diagnostic offset escaped/clamped incorrectly: got %d want %d", m.diagnoseOffset, maxOffset)
	}
}

func TestOverviewSortsAndSummarizesManyInterfaces(t *testing.T) {
	m := testModel(t)
	m.screen, m.tab, m.state = screenServerDetail, tabOverview, discovery.StateActiveConsistent
	m.report.WireGuard.WgPresent = true
	m.report.WireGuard.Interfaces = make(map[string]discovery.InterfaceDetail)
	for i := 19; i >= 0; i-- {
		m.report.WireGuard.Interfaces[fmt.Sprintf("wg%02d", i)] = discovery.InterfaceDetail{Active: true}
	}
	m.Update(tea.WindowSizeMsg{Width: 64, Height: 18})
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "more interface(s)") {
		t.Fatalf("many-interface overview lacks summary:\n%s", view)
	}
	if strings.Index(view, "wg00") > strings.Index(view, "wg01") {
		t.Fatalf("interfaces are not deterministically sorted:\n%s", view)
	}
	assertViewFits(t, m.View(), 64, 18)
}

func TestExportStartedFromOnboardingReturnsToOnboarding(t *testing.T) {
	m := testModel(t)
	m.screen, m.revealNew, m.revealName = screenPeerReveal, true, "Phone"
	_, _ = m.chooseClientSetup("export")
	if m.screen != screenPeerExportForm || m.exportReturn != screenPeerReveal {
		t.Fatalf("export flow lost onboarding return target: screen=%v return=%v", m.screen, m.exportReturn)
	}
	m.Update(key("esc"))
	if m.screen != screenPeerReveal {
		t.Fatalf("cancelling onboarding export returned to %v", m.screen)
	}
}

func TestPeerAndOnboardingViewsFitAcrossAllResponsiveBreakpoints(t *testing.T) {
	heights := []int{18, 19, 20, 23, 24, 25, 30, 40}
	for width := minimumTerminalWidth; width <= 180; width++ {
		for _, height := range heights {
			t.Run(fmt.Sprintf("%dx%d", width, height), func(t *testing.T) {
				m := testModel(t)
				m.Update(tea.WindowSizeMsg{Width: width, Height: height})
				m.screen, m.tab, m.peersLoaded = screenServerDetail, tabPeers, true
				m.peers = []peerview.Peer{{Name: strings.Repeat("Long peer ", 20), PublicKey: strings.Repeat("a", 44), Endpoint: "[2001:db8::1]:51820", State: "never_connected", Managed: true}}
				m.rebuildPeerTable()
				assertViewFits(t, m.View(), width, height)

				m.screen, m.revealNew, m.revealName, m.revealProfile = screenPeerReveal, true, "Phone", "secret"
				m.revealMode = ""
				assertViewFits(t, m.View(), width, height)
			})
		}
	}
}

func FuzzTerminalTextAndTruncationStayTerminalSafe(f *testing.F) {
	for _, seed := range []string{"Alice", "\x1b[31mred", "line\nbreak", "emoji 👩🏽‍💻", "日本語", "\x1b]52;c;secret\a"} {
		f.Add(seed, uint8(20))
	}
	f.Fuzz(func(t *testing.T, value string, rawWidth uint8) {
		width := int(rawWidth%80) + 1
		clean := terminalText(value)
		for _, r := range clean {
			if unicode.IsControl(r) {
				t.Fatalf("terminalText retained control %U in %q", r, clean)
			}
		}
		if got := truncateCell(clean, width); lipgloss.Width(got) > width {
			t.Fatalf("truncateCell width = %d, limit %d, value %q", lipgloss.Width(got), width, got)
		}
	})
}

func FuzzSafeProfileBaseNeverCreatesAPath(f *testing.F) {
	for _, seed := range []string{"Alice Office", "../../root", "..", "", "\x00secret", "Телефон Димы", "a/b\\c"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		base := safeProfileBase(value)
		if base == "" || base == "." || base == ".." {
			t.Fatalf("unsafe empty/dot base %q from %q", base, value)
		}
		if filepath.Base(base) != base || strings.ContainsAny(base, `/\`) {
			t.Fatalf("safeProfileBase created a path %q from %q", base, value)
		}
	})
}
