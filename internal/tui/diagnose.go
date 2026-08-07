package tui

import (
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
)

func (m *model) viewDiagnoseTab() string {
	switch {
	case !m.checksLoaded:
		return "Connecting and running diagnostics...\n"
	case m.checksErr != "":
		return dangerStyle.Render("Error: "+m.checksErr) + "\n"
	default:
		var b strings.Builder
		for _, c := range m.checks {
			b.WriteString(checkStatusBadge(string(c.Status)) + "  " + textStyle.Render(c.Name))
			if c.Detail != "" {
				b.WriteString("\n     " + dimStyle.Render(c.Detail))
			}
			b.WriteString("\n")
		}
		if len(diagnostics.Limitations) > 0 {
			b.WriteString("\n" + dimStyle.Render("Limitations:") + "\n")
			for _, l := range diagnostics.Limitations {
				b.WriteString(dimStyle.Render("  - "+l) + "\n")
			}
		}
		return b.String()
	}
}
