package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Run unlocks (or creates) the local vault in plain terminal mode, then
// starts the Bubble Tea program. SPEC.md §25: startup should stay under
// 1s without a server connection - vault unlock is local disk I/O plus a
// deliberately-slow (argon2id) key derivation, not network, so this holds.
func Run() error {
	v, vaultPath, err := openOrCreateVault(os.Stdout)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}

	m := newModel(v, vaultPath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program = p

	final, err := p.Run()
	if fm, ok := final.(*model); ok {
		fm.closeSessions()
	}
	return err
}
