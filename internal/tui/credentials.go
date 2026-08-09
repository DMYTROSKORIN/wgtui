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

// credentialCache holds the SSH login and sudo passwords for as long as one
// server stays open in the detail screen, so reconnecting for install,
// post-install rediscovery, configure and so on within that same working
// session doesn't ask the user to retype a secret they already gave. It is
// held on *model (see the creds field) and cleared by clearCredentials -
// called wherever a server session actually ends (returnToServerList) or a
// cached sudo password turns out to be wrong (see clearSudoPassword) - so a
// secret never outlives the session it was collected for and a rejected
// password is never silently retried forever.
//
// Values are kept as []byte rather than string specifically so clear can
// overwrite them in place: a Go string is immutable and can only be
// dropped for the garbage collector to reclaim on its own schedule, never
// actively erased. This narrows the exposure window but does not, and in
// pure Go without cgo/mlock cannot, fully prevent another process with
// ptrace or core-dump access to this same already-running, same-user wgtui
// process from recovering a live value - the same residual risk
// docs/THREAT_MODEL.md already documents for the sudo password held during
// a single dial. Nothing here is ever written to disk, logs, argv or env
// (SPEC.md §23.1), and sshtransport's Debug hook never receives typed
// secrets.
type credentialCache struct {
	mu   sync.Mutex
	ssh  []byte
	sudo []byte
}

func (c *credentialCache) sshPassword() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ssh == nil {
		return "", false
	}
	return string(c.ssh), true
}

func (c *credentialCache) setSSHPassword(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ssh = []byte(value)
}

func (c *credentialCache) sudoPassword() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sudo == nil {
		return "", false
	}
	return string(c.sudo), true
}

func (c *credentialCache) setSudoPassword(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sudo = []byte(value)
}

// clearSudoPassword drops a cached sudo password without touching the SSH
// one - used specifically when the server told us that password was wrong
// (discovery.Report.System.SudoPasswordRejected), so the next privileged
// action prompts the human again instead of silently retrying a value the
// server has already refused.
func (c *credentialCache) clearSudoPassword() {
	c.mu.Lock()
	defer c.mu.Unlock()
	zeroBytes(c.sudo)
	c.sudo = nil
}

// clear erases both cached secrets - call when a server session ends.
func (c *credentialCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	zeroBytes(c.ssh)
	zeroBytes(c.sudo)
	c.ssh, c.sudo = nil, nil
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func newTUIPrompts(program *tea.Program, target string, cache *credentialCache) sshtransport.Prompts {
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
			if value, ok := cache.sshPassword(); ok {
				return value, nil
			}
			answer, err := request(credentialSSHPassword, "")
			if err != nil {
				return "", err
			}
			cache.setSSHPassword(answer.value)
			return answer.value, nil
		},
		SudoPassword: func() (string, error) {
			if value, ok := cache.sudoPassword(); ok {
				return value, nil
			}
			answer, err := request(credentialSudoPassword, "")
			if err != nil {
				return "", err
			}
			cache.setSudoPassword(answer.value)
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
	// One header line, not two - see viewVaultAccess's comment for why.
	header := chromeStyle.Width(m.width).Render(fmt.Sprintf(" wgtui %s  ·  %s", strings.TrimPrefix(Version, "v"), title))
	return header + "\n" + fullPanel(m, content) + "\n" + keyBar(m.width, "Enter Continue", "Esc Cancel", "Ctrl+C Quit")
}
