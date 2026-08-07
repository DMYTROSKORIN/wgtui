package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
)

// loadDiagnoseCmd reuses the exact same checklist internal/cli's `diagnose`
// command does (internal/diagnostics.BuildChecks), just rendered as a
// colored list instead of plain text.
func (m *model) loadDiagnoseCmd() tea.Cmd {
	s := m.selected
	localPeers := m.vault.ListPeers(s.ID)

	return m.releaseTerminalAndRun(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		prompts := newTerminalPrompts(os.Stdout)
		sess, err := dial(ctx, s, prompts)
		if err != nil {
			return diagnoseMsg{err: fmt.Sprintf("connect: %v", err)}
		}
		defer sess.close()

		report := discovery.Discover(ctx, sess.client, prompts)
		checks := diagnostics.BuildChecks(report, localPeers, time.Now())
		return diagnoseMsg{checks: checks}
	})
}

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
