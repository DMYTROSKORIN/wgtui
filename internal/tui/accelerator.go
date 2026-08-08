package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// renderAcceleratedLabel renders label at exactly width terminal cells,
// highlighting the single rune matching key (case-insensitively) with
// accent while the rest of the text uses base. base and accent must share
// the same background so the segment reads as one continuous surface -
// docs/UI_MENU_REDESIGN_HANDOFF.md requires a single renderer for both the
// top menu (Alt+letter category accelerators) and the bottom action bar
// (plain-letter context accelerators) so neither prints the shortcut as a
// separate key cell next to its own label.
//
// If key does not occur in label (the "?" help symbol is not a letter of
// "Help"), key is rendered as its own leading accent character instead of
// highlighting mid-word - the one case where a distinct key glyph is
// unavoidable because there is no matching letter to highlight.
func renderAcceleratedLabel(label, key string, width int, base, accent lipgloss.Style) string {
	runes := []rune(label)
	idx := runeIndexFold(runes, key)

	var b strings.Builder
	used := 0
	write := func(s string, style lipgloss.Style) {
		if s == "" {
			return
		}
		b.WriteString(style.Render(s))
		used += lipgloss.Width(s)
	}

	if idx >= 0 {
		write(string(runes[:idx]), base)
		write(string(runes[idx:idx+1]), accent)
		write(string(runes[idx+1:]), base)
	} else {
		write(key, accent)
		write(label, base)
	}

	if used < width {
		b.WriteString(base.Render(strings.Repeat(" ", width-used)))
		used = width
	}
	out := b.String()
	if used > width {
		out = ansi.Truncate(out, width, "")
	}
	return out
}

// runeIndexFold returns the index within runes of the first rune matching
// key case-insensitively, or -1. key must be exactly one rune - any other
// length (e.g. the "?" help symbol) never matches, which is the intended
// fallback trigger in renderAcceleratedLabel.
func runeIndexFold(runes []rune, key string) int {
	keyRunes := []rune(key)
	if len(keyRunes) != 1 {
		return -1
	}
	target := unicode.ToUpper(keyRunes[0])
	for i, r := range runes {
		if unicode.ToUpper(r) == target {
			return i
		}
	}
	return -1
}

// commanderSegmentWidths splits total cells evenly across count segments,
// handing the remainder to the leftmost segments one column at a time - the
// same distribution the render path and the mouse hit-test path must both
// use so a click always lands on the segment it visually appears over
// (docs/UI_MENU_REDESIGN_HANDOFF.md requirement 4).
func commanderSegmentWidths(count, total int) []int {
	if count <= 0 {
		return nil
	}
	base, remainder := total/count, total%count
	widths := make([]int, count)
	for i := range widths {
		widths[i] = base
		if i < remainder {
			widths[i]++
		}
	}
	return widths
}
