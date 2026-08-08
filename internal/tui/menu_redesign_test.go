package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
)

// Acceptance tests for docs/UI_MENU_REDESIGN_HANDOFF.md: the top menu is a
// calm Alt+letter navigation area and the bottom bar is a plain-letter
// context-action area, and the two must never share a promise.

func TestTopMenuBarNeverContainsPipeSeparator(t *testing.T) {
	for width := minimumTerminalWidth; width <= 200; width++ {
		m := testModel(t)
		m.width = width
		if got := ansi.Strip(m.viewCommanderMenuBar()); strings.Contains(got, "│") {
			t.Fatalf("width %d top menu bar contains │: %q", width, got)
		}
	}
}

// TestActiveCommanderCategoryOnlyWhenMenuOpen targets
// docs/UI_MENU_REDESIGN_HANDOFF.md's core visual invariant - no category has
// an active background while the menu is closed, and only the open
// category's own segment does while it's open - through the pure decision
// function viewCommanderMenuBar renders from, since the test environment's
// non-TTY lipgloss renderer strips ANSI/color codes and so cannot
// distinguish styled output by string comparison.
func TestActiveCommanderCategoryOnlyWhenMenuOpen(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if got := m.activeCommanderCategoryIndex(); got != -1 {
		t.Fatalf("closed menu reports active category %d, want none (-1)", got)
	}

	m.Update(key("m")) // opens category 0 (Server)
	if got := m.activeCommanderCategoryIndex(); got != 0 {
		t.Fatalf("opening the menu reports active category %d, want 0 (Server)", got)
	}

	m.Update(key("right")) // move to category 1 (Peer) without closing
	if got := m.activeCommanderCategoryIndex(); got != 1 {
		t.Fatalf("moving right reports active category %d, want 1 (Peer)", got)
	}

	m.Update(key("esc"))
	if got := m.activeCommanderCategoryIndex(); got != -1 {
		t.Fatalf("closing the menu reports active category %d, want none (-1)", got)
	}
}

// TestAltCategorySwitchesWhileMenuAlreadyOpen guards against a real bug: a
// second Alt+letter while a different category's dropdown was already open
// (e.g. from an earlier M press) used to fall through every case in
// updateCommanderMenu unmatched and do nothing at all - indistinguishable
// from "the hotkey doesn't respond" to a user pressing Alt+P expecting the
// Peer menu while Server's dropdown happened to already be open.
func TestAltCategorySwitchesWhileMenuAlreadyOpen(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	m.Update(key("m")) // opens category 0 (Server)
	if !m.menuOpen || m.menuCategory != 0 {
		t.Fatalf("M did not open the Server menu: open=%v category=%d", m.menuOpen, m.menuCategory)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p"), Alt: true})
	if !m.menuOpen || m.menuCategory != 1 {
		t.Fatalf("Alt+P while the Server menu was open did not switch to Peer: open=%v category=%d", m.menuOpen, m.menuCategory)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h"), Alt: true})
	if !m.menuOpen || m.menuCategory != 4 {
		t.Fatalf("Alt+H while the Peer menu was open did not switch to Help: open=%v category=%d", m.menuOpen, m.menuCategory)
	}
}

func TestAltPeerOpensMenuIndependentlyOfPlainPasswordAccelerator(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p"), Alt: true})
	if !m.menuOpen || m.menuCategory != 1 || m.screen != screenServerList {
		t.Fatalf("Alt+P did not open the Peer category in place: open=%v category=%d screen=%v", m.menuOpen, m.menuCategory, m.screen)
	}
	m.Update(key("esc"))
	if m.menuOpen || m.screen != screenServerList {
		t.Fatalf("Esc did not close the menu without leaving the server list: open=%v screen=%v", m.menuOpen, m.screen)
	}

	// The vault password action is bound to plain W (passWord), not P -
	// deliberately, so it can never be confused with Alt+P even if a
	// terminal delivers Alt+P as a split Escape-then-letter sequence (see
	// docs/COMMANDER_UI.md).
	m.Update(key("w"))
	if m.menuOpen || m.screen != screenChangePassword {
		t.Fatalf("plain W did not open the vault password form: open=%v screen=%v", m.menuOpen, m.screen)
	}
}

// TestBottomBarNeverReusesATopCategoryLetter locks in the mitigation for the
// Peer/Password confusion reported after the first redesign: even though
// Alt+letter and a plain letter are technically different key events, a
// bottom-bar letter identical to a top category's own letter invites
// exactly the confusion a user hit in practice, especially on terminals
// where an Alt-prefixed sequence occasionally arrives split into a lone
// Escape followed by a plain letter. No bottom-bar accelerator, on any
// screen, may equal a top category's letter.
func TestBottomBarNeverReusesATopCategoryLetter(t *testing.T) {
	topLetters := map[string]bool{}
	for _, c := range commanderCategories {
		topLetters[strings.ToUpper(c.key)] = true
	}

	server := domain.Server{ID: "office", DisplayName: "Office"}
	cases := []struct {
		name  string
		setup func(*model)
	}{
		{"server list", func(m *model) {}},
		{"server overview", func(m *model) {
			m.screen, m.tab, m.selected = screenServerDetail, tabOverview, server
		}},
		{"peers tab", func(m *model) {
			m.screen, m.tab, m.selected = screenServerDetail, tabPeers, server
			m.peers = []peerview.Peer{{Name: "Phone", PublicKey: "pk", Managed: true, State: "online"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel(t, server)
			tc.setup(m)
			for _, hint := range m.commanderActionHints() {
				fold := strings.ToUpper(hint.key)
				if topLetters[fold] {
					t.Fatalf("bottom-bar accelerator %q collides with a top-menu category letter", hint.key)
				}
			}
		})
	}
}

func TestBottomBarAcceleratorsAreEmbeddedInLabelAndUniquePerPanel(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	cases := []struct {
		name  string
		setup func(*model)
	}{
		{"server list", func(m *model) {}},
		{"server overview, wireguard missing", func(m *model) {
			m.screen, m.tab, m.selected = screenServerDetail, tabOverview, server
			m.report.WireGuard.WgPresent = false
		}},
		{"server overview, wireguard installed", func(m *model) {
			m.screen, m.tab, m.selected = screenServerDetail, tabOverview, server
			m.report.WireGuard.WgPresent = true
		}},
		{"peers tab", func(m *model) {
			m.screen, m.tab, m.selected = screenServerDetail, tabPeers, server
			m.peers = []peerview.Peer{{Name: "Phone", PublicKey: "pk", Managed: true, State: "online"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel(t, server)
			tc.setup(m)
			hints := m.commanderActionHints()
			seen := map[string]string{}
			for _, h := range hints {
				fold := strings.ToUpper(h.key)
				if other, dup := seen[fold]; dup {
					t.Fatalf("accelerator %q is shared by %q and %q", h.key, other, h.label)
				}
				seen[fold] = h.label
				if h.key == "?" {
					continue // the Help symbol is not a letter of "Help" by design
				}
				if !strings.Contains(strings.ToUpper(h.label), fold) {
					t.Fatalf("accelerator %q does not appear inside its own label %q", h.key, h.label)
				}
			}
		})
	}
}

func TestBottomBarHasNoSeparateKeyCellPattern(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.width = 120
	plain := ansi.Strip(m.commanderActionBar())
	for _, bad := range []string{"OOpen", "EEdit", "WPassword", "NNew", "QQuit"} {
		if strings.Contains(plain, bad) {
			t.Fatalf("bottom bar still doubles an accelerator letter as a separate key cell: %q in %q", bad, plain)
		}
	}
}

func TestClickAndAcceleratorDispatchTheSameAction(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	viaKey := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	viaKey.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	viaKey.Update(key("o"))

	hints := m.commanderActionHints()
	widths := commanderSegmentWidths(len(hints), m.width)
	x := 0
	for i, hint := range hints {
		if hint.key == "O" {
			m.Update(leftClick(x+1, m.height-1))
			break
		}
		x += widths[i]
	}
	if m.screen != viaKey.screen || m.selected.ID != viaKey.selected.ID {
		t.Fatalf("mouse click and O accelerator diverged: click screen=%v key screen=%v", m.screen, viaKey.screen)
	}
}
