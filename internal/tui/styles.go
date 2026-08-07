package tui

import (
	bkey "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Adaptive light/dark colors keep semantic states consistent while preserving
// contrast across terminal themes. Muted text uses a real foreground color,
// not SGR faint: faint became unreadable in several common dark palettes.
var (
	colorDanger    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FF5F6D"}
	colorSuccess   = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	colorWarning   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colorAccent    = lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#38BDF8"}
	colorHighlight = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C084FC"}
	colorText      = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}
	colorMuted     = lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	subtitleStyle = lipgloss.NewStyle().Foreground(colorMuted)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colorHighlight)
	dimStyle      = lipgloss.NewStyle().Foreground(colorMuted)
	textStyle     = lipgloss.NewStyle().Foreground(colorText)
	successStyle  = lipgloss.NewStyle().Foreground(colorSuccess)
	warningStyle  = lipgloss.NewStyle().Foreground(colorWarning)
	dangerStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorDanger)

	accentHeadingStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	menuSelectedStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(colorHighlight).Padding(0, 1)
	dangerSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(colorDanger).Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)

	tabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Underline(true)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(colorMuted)

	footerStyle = lipgloss.NewStyle().Foreground(colorMuted)
)

// formTheme is the one huh.Theme every form/confirm/select in the app
// uses, so a huh screen and a hand-rolled screen never look like two
// different programs stitched together.
func formTheme() *huh.Theme {
	t := huh.ThemeBase16()

	muted := lipgloss.NewStyle().Foreground(colorMuted)

	t.Focused.Base = t.Focused.Base.BorderForeground(colorAccent)
	t.Focused.Title = t.Focused.Title.Bold(true).Foreground(colorAccent)
	t.Focused.Description = muted
	t.Focused.TextInput.Placeholder = muted

	t.Blurred.Title = muted
	t.Blurred.NoteTitle = muted
	t.Blurred.Description = muted
	t.Blurred.TextInput.Prompt = muted
	t.Blurred.TextInput.Placeholder = muted

	return t
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
