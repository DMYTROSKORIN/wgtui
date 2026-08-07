package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

func (m *model) startPasswordForm() {
	m.currentPassword, m.newPassword, m.confirmPassword = "", "", ""
	m.passwordForm = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Current master password").
			Description("Unlocks only the local encrypted vault; it is never sent to a server.").
			EchoMode(huh.EchoModePassword).Value(&m.currentPassword),
		huh.NewInput().Title("New master password").
			Description("At least 8 characters. Use a unique passphrase.").
			EchoMode(huh.EchoModePassword).Value(&m.newPassword).
			Validate(func(s string) error {
				if len(s) < 8 {
					return fieldErr("must be at least 8 characters")
				}
				return nil
			}),
		huh.NewInput().Title("Confirm new master password").
			EchoMode(huh.EchoModePassword).Value(&m.confirmPassword).
			Validate(func(s string) error {
				if s != m.newPassword {
					return fieldErr("passwords do not match")
				}
				return nil
			}),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.screen = screenChangePassword
}

func (m *model) updatePasswordForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.passwordForm.Update(msg)
	m.passwordForm = form.(*huh.Form)
	switch m.passwordForm.State {
	case huh.StateCompleted:
		if err := m.vault.ChangePassword(m.vaultPath, m.currentPassword, m.newPassword); err != nil {
			if errors.Is(err, profilevault.ErrWrongPassword) {
				m.setError(fieldErr("current master password is incorrect"))
			} else {
				m.setError(err)
			}
			m.startPasswordForm()
			return m, m.passwordForm.Init()
		}
		m.currentPassword, m.newPassword, m.confirmPassword = "", "", ""
		m.screen = screenServerList
		m.setStatus("Master password changed. The vault was re-encrypted successfully.")
		return m, nil
	case huh.StateAborted:
		m.currentPassword, m.newPassword, m.confirmPassword = "", "", ""
		m.screen = screenServerList
		return m, nil
	}
	return m, cmd
}

func (m *model) viewPasswordForm() string {
	return titleStyle.Render("wgtui") + "  " + subtitleStyle.Render("Security / change master password") + "\n\n" +
		m.passwordForm.View() + "\n\n" + footerStyle.Render("[esc] cancel  [ctrl+c] quit")
}
