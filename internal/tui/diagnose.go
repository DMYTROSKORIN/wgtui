package tui

import (
	"fmt"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/charmbracelet/x/ansi"
)

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
