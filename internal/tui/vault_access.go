package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

func (m *model) startVaultAccessForm() {
	m.vaultPassword, m.vaultConfirm = "", ""
	title := "Master password"
	description := "Local vault only; never sent to any server."
	fields := []huh.Field{
		huh.NewInput().Title(title).Description(description).EchoMode(huh.EchoModePassword).Value(&m.vaultPassword),
	}
	if m.vaultCreating {
		fields[0] = huh.NewInput().Title("Create master password (8+ characters)").
			Description("Local vault only; never sent to any server.").
			EchoMode(huh.EchoModePassword).Value(&m.vaultPassword).
			Validate(func(value string) error {
				if len(value) < 8 {
					return fieldErr("use at least 8 characters")
				}
				return nil
			})
		fields = append(fields, huh.NewInput().Title("Confirm master password").
			EchoMode(huh.EchoModePassword).Value(&m.vaultConfirm).
			Validate(func(value string) error {
				if value != m.vaultPassword {
					return fieldErr("passwords do not match")
				}
				return nil
			}))
	}
	m.vaultForm = huh.NewForm(huh.NewGroup(fields...)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
}

func (m *model) updateVaultAccess(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.vaultForm.Update(msg)
	m.vaultForm = form.(*huh.Form)
	switch m.vaultForm.State {
	case huh.StateCompleted:
		password := m.vaultPassword
		confirm := m.vaultConfirm
		m.vaultPassword, m.vaultConfirm = "", ""
		var vault *profilevault.Vault
		var err error
		if m.vaultCreating {
			if len(password) < 8 {
				err = fieldErr("master password must be at least 8 characters")
			} else if password != confirm {
				err = fieldErr("passwords do not match")
			} else {
				vault, err = profilevault.New(password)
			}
			if err == nil {
				err = vault.Save(m.vaultPath)
			}
		} else {
			vault, err = profilevault.Open(m.vaultPath, password)
		}
		password = ""
		if err != nil {
			if errors.Is(err, profilevault.ErrWrongPassword) {
				m.vaultFormError = "That master password did not unlock the local vault."
			} else {
				m.vaultFormError = err.Error()
			}
			m.startVaultAccessForm()
			return m, m.vaultForm.Init()
		}
		m.vault = vault
		m.vaultFormError = ""
		m.screen = screenServerList
		return m, tea.ClearScreen
	case huh.StateAborted:
		m.vaultPassword, m.vaultConfirm = "", ""
		return m, tea.Quit
	}
	return m, cmd
}

func (m *model) viewVaultAccess() string {
	mode := "Unlock local vault"
	intro := "Enter the master password created for wgtui on this computer. It unlocks local data only and is never sent to any server."
	if m.vaultCreating {
		mode = "Protect local vault"
		intro = "First start: create a master password for local server metadata and client profiles. It is never sent to any server."
	}
	content := titleStyle.Render(mode) + "\n\n"
	if layoutModeFor(m.width, m.height) != layoutCompact {
		content += wrapPlainText(intro, max(30, m.width-12), 3) + "\n\n"
	}
	content += m.vaultForm.View()
	if m.vaultFormError != "" {
		content += "\n" + dangerStyle.Render(m.vaultFormError)
	}
	// One header line, not two: this bar already carries the brand,
	// version and section title (" wgtui X.Y.Z · secure local access") -
	// appHeader("Security") used to repeat all three immediately below it,
	// which is exactly the clutter reported against the server-detail
	// header in a different spot (see docs/PROJECT_STATUS.md).
	header := chromeStyle.Width(m.width).Render(" wgtui " + strings.TrimPrefix(Version, "v") + "  ·  secure local access")
	return header + "\n" + fullPanel(m, content) + "\n" +
		keyBar(m.width, "Enter Continue", "Esc Quit", "Ctrl+C Quit")
}
