package tui

import (
	"context"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

func (m *model) startServerForm(edit bool) {
	m.serverEdit = edit
	if edit {
		s := m.selected
		m.fID = s.ID
		m.fName = s.DisplayName
		m.fHost = s.SSHHostOrAlias
		m.fUser = s.SSHUser
		if s.SSHPort != 0 {
			m.fPort = strconv.Itoa(s.SSHPort)
		} else {
			m.fPort = ""
		}
		m.fIdentity = s.SSHIdentityFile
	} else {
		m.fID, m.fName, m.fHost, m.fUser, m.fPort, m.fIdentity = "", "", "", "", "", ""
	}

	var groups []*huh.Group
	if !edit {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().Title("Server ID").Description("A short, unique identifier - can't be changed later.").
				Value(&m.fID).
				Validate(func(s string) error {
					if s == "" {
						return errRequired
					}
					if _, exists := m.vault.GetServer(s); exists {
						return errIDTaken
					}
					return nil
				}),
		))
	}
	groups = append(groups,
		huh.NewGroup(
			huh.NewInput().Title("Display name").Description("Defaults to the ID if left blank.").Value(&m.fName),
			huh.NewInput().Title("SSH host").Description("Hostname, IP, or ~/.ssh/config alias.").Value(&m.fHost).
				Validate(func(s string) error {
					if s == "" {
						return errRequired
					}
					return nil
				}),
			huh.NewInput().Title("SSH user").Description("Optional - defaults to ~/.ssh/config.").Value(&m.fUser),
			huh.NewInput().Title("SSH port").Description("Optional - defaults to ~/.ssh/config or 22.").Value(&m.fPort).
				Validate(validateOptionalPort),
			huh.NewInput().Title("SSH identity file").Description("Optional path to a private key.").Value(&m.fIdentity),
		),
	)

	m.serverForm = huh.NewForm(groups...).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.screen = screenServerForm
}

var (
	errRequired = fieldErr("this field is required")
	errIDTaken  = fieldErr("a server with this ID already exists")
)

type fieldErr string

func (e fieldErr) Error() string { return string(e) }

func validateOptionalPort(s string) error {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 65535 {
		return fieldErr("must be a valid port number")
	}
	return nil
}

func (m *model) updateServerForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.serverForm.Update(msg)
	m.serverForm = form.(*huh.Form)

	switch m.serverForm.State {
	case huh.StateCompleted:
		return m.submitServerForm()
	case huh.StateAborted:
		m.screen = screenServerList
		return m, nil
	}
	return m, cmd
}

// submitServerForm resolves the SSH alias locally (ssh -G, never connects
// or prompts - see sshtransport.ResolveAlias's doc comment) so a typo is
// caught before it's saved, exactly like `wgtui server add` does.
func (m *model) submitServerForm() (tea.Model, tea.Cmd) {
	name := m.fName
	if name == "" {
		name = m.fID
	}
	port, _ := strconv.Atoi(m.fPort)

	target := sshtransport.Target{HostOrAlias: m.fHost, Port: port, User: m.fUser, IdentityFile: m.fIdentity}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := sshtransport.ResolveAlias(ctx, target); err != nil {
		m.setError(err)
		return m, nil
	}

	s := domain.Server{
		ID: m.fID, DisplayName: name, SSHHostOrAlias: m.fHost,
		SSHUser: m.fUser, SSHPort: port, SSHIdentityFile: m.fIdentity,
	}
	if m.serverEdit {
		s.ID = m.selected.ID
		m.vault.UpdateServer(s)
	} else if err := m.vault.AddServer(s); err != nil {
		m.setError(err)
		return m, nil
	}
	if err := m.vault.Save(m.vaultPath); err != nil {
		m.setError(err)
		return m, nil
	}

	m.screen = screenServerList
	if m.serverEdit {
		m.setStatus("Updated " + s.DisplayName + ".")
	} else {
		m.setStatus("Added " + s.DisplayName + ".")
	}
	return m, nil
}

func (m *model) viewServerForm() string {
	title := "Add server"
	if m.serverEdit {
		title = "Edit " + m.selected.DisplayName
	}
	return titleStyle.Render("wgtui") + "  " + subtitleStyle.Render(title) + "\n\n" + m.serverForm.View()
}

func (m *model) startRemoveConfirm(s domain.Server) {
	m.selected = s
	m.confirmYes = false
	m.removeForm = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Remove " + s.DisplayName + "?").
			Description("This only forgets it locally - nothing changes on the server itself.").
			Affirmative("Remove").
			Negative("Cancel").
			Value(&m.confirmYes),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.screen = screenServerRemoveConfirm
}

func (m *model) updateRemoveConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.removeForm.Update(msg)
	m.removeForm = form.(*huh.Form)

	switch m.removeForm.State {
	case huh.StateCompleted:
		m.screen = screenServerList
		if !m.confirmYes {
			return m, nil
		}
		name := m.selected.DisplayName
		m.vault.RemoveServer(m.selected.ID)
		if err := m.vault.Save(m.vaultPath); err != nil {
			m.setError(err)
			return m, nil
		}
		if m.cursor >= len(m.vault.Registry.Servers) && m.cursor > 0 {
			m.cursor--
		}
		m.setStatus("Removed " + name + ".")
		return m, nil
	case huh.StateAborted:
		m.screen = screenServerList
		return m, nil
	}
	return m, cmd
}

func (m *model) viewRemoveConfirm() string {
	return titleStyle.Render("wgtui") + "\n\n" + m.removeForm.View()
}
