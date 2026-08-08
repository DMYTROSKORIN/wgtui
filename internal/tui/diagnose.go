package tui

import (
	"fmt"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m *model) viewWideDiagnose(panelHeight int) string {
	listWidth, inspectorWidth := peerSplitPanelWidths(m.width)
	if !m.checksLoaded || m.checksErr != "" || len(m.checks) == 0 {
		main := panelStyle.Width(listWidth).Height(panelHeight).Render(tabsLine(m.tab) + "\n\n" + m.viewDiagnoseTab())
		info := panelStyle.Width(inspectorWidth).Height(panelHeight).Render(accentHeadingStyle.Render("Check details") + "\n\n" + dimStyle.Render("Diagnostics context appears here."))
		return lipgloss.JoinHorizontal(lipgloss.Top, main, " ", info)
	}

	cursor := clamp(m.diagnoseOffset, 0, len(m.checks)-1)
	visible := max(2, panelHeight-4)
	start := clamp(cursor-visible+1, 0, max(0, len(m.checks)-visible))
	end := min(len(m.checks), start+visible)
	var list strings.Builder
	list.WriteString(tabsLine(m.tab) + "\n\n")
	for i := start; i < end; i++ {
		c := m.checks[i]
		if i == cursor {
			// menuSelectedStyle wraps this in one final Render() call, so
			// the text going in must be plain - never checkStatusBadge's
			// pre-styled output, which truncateCell (via terminalText)
			// would corrupt by stripping the ESC bytes out of its ANSI
			// codes while leaving the rest of each escape sequence behind
			// as visible garbage.
			plain := checkStatusPlain(string(c.Status)) + "  " + c.Name
			list.WriteString(menuSelectedStyle.Render(truncateCell(plain, max(12, listWidth-6))))
		} else {
			line := checkStatusBadge(string(c.Status)) + "  " + c.Name
			list.WriteString(ansi.Truncate(line, max(12, listWidth-4), "…"))
		}
		list.WriteByte('\n')
	}
	if len(m.checks) > visible {
		list.WriteString(dimStyle.Render(fmt.Sprintf("Check %d of %d · ↑/↓ select", cursor+1, len(m.checks))))
	}

	c := m.checks[cursor]
	detail := accentHeadingStyle.Render("Selected check") + "\n\n" +
		checkStatusBadge(string(c.Status)) + "  " + selectedStyle.Render(c.Name) + "\n\n"
	if c.Detail == "" {
		detail += dimStyle.Render("No additional detail was reported.")
	} else {
		detail += wrapPlainText(c.Detail, max(20, inspectorWidth-4), 8)
	}
	detail += "\n\n" + dimStyle.Render("R rebuilds all checks")
	left := panelStyle.Width(listWidth).Height(panelHeight).Render(list.String())
	right := panelStyle.Width(inspectorWidth).Height(panelHeight).Render(detail)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (m *model) viewDiagnoseTab() string {
	switch {
	case !m.checksLoaded:
		return "Connecting and running diagnostics...\n"
	case m.checksErr != "":
		return dangerStyle.Render("Error: "+m.checksErr) + "\n"
	default:
		lines := m.diagnoseLines()
		visible := m.diagnoseVisibleRows()
		maxOffset := max(0, len(lines)-visible)
		m.diagnoseOffset = clamp(m.diagnoseOffset, 0, maxOffset)
		end := min(len(lines), m.diagnoseOffset+visible)
		var b strings.Builder
		for _, line := range lines[m.diagnoseOffset:end] {
			b.WriteString(line + "\n")
		}
		if len(lines) > visible {
			b.WriteString(dimStyle.Render(fmt.Sprintf("Rows %d–%d of %d · ↑/↓ scroll", m.diagnoseOffset+1, end, len(lines))))
		}
		return b.String()
	}
}

func (m *model) diagnoseVisibleRows() int {
	// Two inner-panel lines are occupied by the tabs and their spacer; one
	// more is reserved for the scroll indicator when it is needed.
	return max(2, m.dashboardPanelHeight()-4)
}

func (m *model) diagnoseLines() []string {
	width := max(20, m.width-8)
	lines := make([]string, 0, len(m.checks)+len(diagnostics.Limitations)+1)
	for _, c := range m.checks {
		line := checkStatusBadge(string(c.Status)) + "  " + textStyle.Render(c.Name)
		if c.Detail != "" {
			line += dimStyle.Render(" — " + c.Detail)
		}
		lines = append(lines, ansi.Truncate(line, width, "…"))
	}
	if len(diagnostics.Limitations) > 0 {
		lines = append(lines, dimStyle.Render("Limitations:"))
		for _, limitation := range diagnostics.Limitations {
			lines = append(lines, ansi.Truncate(dimStyle.Render("• "+limitation), width, "…"))
		}
	}
	return lines
}
