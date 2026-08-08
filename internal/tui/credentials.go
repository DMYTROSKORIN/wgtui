package tui

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

type credentialKind uint8

const (
	credentialHostKey credentialKind = iota
	credentialSSHPassword
	credentialSudoPassword
)

type credentialResponse struct {
	value    string
	accepted bool
	err      error
}

type credentialRequestMsg struct {
	kind     credentialKind
	target   string
	detail   string
	response chan credentialResponse
}

func newTUIPrompts(program *tea.Program, target string) sshtransport.Prompts {
	var mu sync.Mutex
	var sshPassword, sudoPassword *string
	request := func(kind credentialKind, detail string) (credentialResponse, error) {
		if program == nil {
			return credentialResponse{}, errors.New("interactive credential prompt is unavailable before the TUI starts")
		}
		response := make(chan credentialResponse, 1)
		program.Send(credentialRequestMsg{kind: kind, target: target, detail: detail, response: response})
		answer := <-response
		return answer, answer.err
	}
	return sshtransport.Prompts{
		HostKey: func(detail string) bool {
			answer, err := request(credentialHostKey, detail)
			return err == nil && answer.accepted
		},
		Password: func() (string, error) {
			mu.Lock()
			if sshPassword != nil {
				value := *sshPassword
				mu.Unlock()
				return value, nil
			}
			mu.Unlock()
			answer, err := request(credentialSSHPassword, "")
			if err != nil {
				return "", err
			}
			mu.Lock()
			sshPassword = &answer.value
			mu.Unlock()
			return answer.value, nil
		},
		SudoPassword: func() (string, error) {
			mu.Lock()
			if sudoPassword != nil {
				value := *sudoPassword
				mu.Unlock()
				return value, nil
			}
			mu.Unlock()
			answer, err := request(credentialSudoPassword, "")
			if err != nil {
				return "", err
			}
			mu.Lock()
			sudoPassword = &answer.value
			mu.Unlock()
			return answer.value, nil
		},
	}
}

func (m *model) beginCredentialPrompt(request credentialRequestMsg) (tea.Model, tea.Cmd) {
	if m.credentialRequest != nil {
		request.response <- credentialResponse{err: errors.New("another credential prompt is already active")}
		return m, nil
	}
	m.credentialReturn = m.screen
	m.credentialRequest = &request
	m.credentialValue = ""
	m.credentialAccepted = false
	var field huh.Field
	if request.kind == credentialHostKey {
		field = huh.NewConfirm().Title("Trust this SSH host key?").
			Description("Confirm only if this fingerprint belongs to the server you intended to open.").
			Affirmative("Trust and connect").Negative("Cancel").Value(&m.credentialAccepted)
	} else {
		title := "SSH login password"
		description := "Remote server credential. This is not your wgtui master password."
		if request.kind == credentialSudoPassword {
			title = "sudo password"
			description = "Privilege password on the remote server. Usually the same as its SSH password."
		}
		field = huh.NewInput().Title(title).Description(description).
			EchoMode(huh.EchoModePassword).Value(&m.credentialValue).
			Validate(func(value string) error {
				if value == "" {
					return fieldErr("password cannot be empty")
				}
				return nil
			})
	}
	m.credentialForm = huh.NewForm(huh.NewGroup(field)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.screen = screenCredentialPrompt
	return m, m.credentialForm.Init()
}

func (m *model) updateCredentialPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.credentialForm.Update(msg)
	m.credentialForm = form.(*huh.Form)
	switch m.credentialForm.State {
	case huh.StateCompleted:
		response := credentialResponse{value: m.credentialValue, accepted: true}
		if m.credentialRequest.kind == credentialHostKey {
			response.accepted = m.credentialAccepted
			if !response.accepted {
				response.err = errors.New("SSH host key was not accepted")
			}
		}
		m.finishCredentialPrompt(response)
		return m, nil
	case huh.StateAborted:
		m.finishCredentialPrompt(credentialResponse{err: errors.New("credential entry cancelled")})
		return m, nil
	}
	return m, cmd
}

func (m *model) finishCredentialPrompt(response credentialResponse) {
	if m.credentialRequest != nil {
		m.credentialRequest.response <- response
	}
	m.credentialValue = ""
	m.credentialAccepted = false
	m.credentialRequest = nil
	m.credentialForm = nil
	m.screen = m.credentialReturn
}

func (m *model) cancelCredentialPrompt(reason string) {
	if m.credentialRequest != nil {
		m.finishCredentialPrompt(credentialResponse{err: errors.New(reason)})
	}
}

func (m *model) viewCredentialPrompt() string {
	request := m.credentialRequest
	if request == nil {
		return ""
	}
	title := "Remote authentication"
	credential := "SSH password"
	if request.kind == credentialHostKey {
		title, credential = "First connection", "SSH host key"
	} else if request.kind == credentialSudoPassword {
		title, credential = "Privilege required", "sudo password"
	}
	content := accentHeadingStyle.Render(credential) + "\n\n" +
		dimStyle.Render("Server") + "  " + selectedStyle.Render(terminalText(request.target)) + "\n\n"
	if request.detail != "" {
		content += wrapPlainText(request.detail, max(30, m.width-12), 5) + "\n\n"
	}
	content += m.credentialForm.View() + "\n\n" + dangerStyle.Render("The value stays only in memory for this server session.")
	header := chromeStyle.Width(m.width).Render(fmt.Sprintf(" wgtui %s  ·  %s", strings.TrimPrefix(Version, "v"), title))
	return header + "\n" + appHeader(title) + "\n" + fullPanel(m, content) + "\n" + keyBar(m.width, "Enter Continue", "Esc Cancel", "Ctrl+C Quit")
}
