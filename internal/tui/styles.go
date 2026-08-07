package tui

import (
	bkey "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Colors are plain ANSI 4-bit indices (0-15), not fixed hex values: the
// terminal emulator itself remaps these to whatever palette the user has
// configured (Dracula, Nord, Gruvbox, ...), so wgtui's colors always match
// the rest of their terminal instead of clashing with it. huh.ThemeBase16
// (used for every form) works the same way - these constants mirror its
// choices so forms and hand-rolled screens read as one consistent app.
const (
	colorMuted     = "8" // bright black - secondary text, borders, hints
	colorDanger    = "9" // bright red - fail, revoke, destructive confirms
	colorSuccess   = "2" // green - active, pass, installed
	colorWarning   = "3" // yellow - disabled, warn, drift
	colorAccent    = "6" // cyan - titles, section headers
	colorHighlight = "5" // magenta - selection, focus (matches huh's button bg)
	colorText      = "7" // white - primary text
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorHighlight))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	textStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorText))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSuccess))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWarning))
	dangerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorDanger))

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorMuted)).
			Padding(0, 1)

	tabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent)).Underline(true)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))

	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
)

// formTheme is the one huh.Theme every form/confirm/select in the app uses,
// so a huh screen and a hand-rolled screen never look like two different
// programs stitched together.
func formTheme() *huh.Theme {
	return huh.ThemeBase16()
}

// formKeyMap adds "esc" as a second way to abort a form/confirm/select,
// matching the "[q/esc] back" convention every other screen in the app
// already uses - huh's own default only binds ctrl+c to Quit, which would
// otherwise be the only way out and let a stray "esc" fall through into
// whatever text field happens to be focused instead.
func formKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = bkey.NewBinding(bkey.WithKeys("ctrl+c", "esc"))
	return km
}

// badge renders a small colored status indicator + label, e.g. "● active".
// The single place that decides what color a given state gets, so the same
// state always reads the same way everywhere (peer list, diagnose
// checklist, server connection state).
func badge(style lipgloss.Style, symbol, label string) string {
	return style.Render(symbol+" ") + label
}

// peerStateBadge covers peerview.Peer.State's actual values (mirrors
// domain.PeerStatus: online, idle, never_connected, disabled, unmanaged,
// drifted, error).
func peerStateBadge(state string) string {
	switch state {
	case "online":
		return badge(successStyle, "●", state)
	case "disabled":
		return badge(warningStyle, "●", state)
	case "drifted", "error":
		return badge(dangerStyle, "●", state)
	case "idle", "never_connected", "unmanaged":
		return badge(dimStyle, "○", state)
	default:
		return badge(dimStyle, "○", state)
	}
}

// Diagnose check status (diagnostics.Status: pass/warn/fail/skip).
func checkStatusBadge(status string) string {
	switch status {
	case "pass":
		return badge(successStyle, "✓", "pass")
	case "warn":
		return badge(warningStyle, "!", "warn")
	case "fail":
		return badge(dangerStyle, "✗", "fail")
	default:
		return badge(dimStyle, "-", "skip")
	}
}
