package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

// Run starts Bubble Tea immediately; the first in-app screen unlocks or creates
// the local vault. Vault access is local disk I/O plus deliberately-slow
// Argon2id derivation and never opens a server connection.
func Run() error {
	vaultPath, err := profilevault.VaultPath()
	if err != nil {
		return fmt.Errorf("resolve vault path: %w", err)
	}

	m := newLockedModel(vaultPath, !profilevault.Exists(vaultPath))
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p

	final, err := p.Run()
	if fm, ok := final.(*model); ok {
		fm.closeSessions()
	}
	return err
}
