package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

type screen int

const (
	screenServerList screen = iota
	screenServerDetail
)

// model is Stage 1's read-only TUI (SPEC.md §20.1, §20.2): browse the
// local server registry and drill into one server's discovery report and
// peer list. Mutating actions (add/edit/forget servers, peer
// add/disable/rotate/revoke) are Stage 2 work - `wgtui server add` on the
// CLI covers registration for now.
type model struct {
	vault     *profilevault.Vault
	vaultPath string
	program   *tea.Program

	screen screen
	cursor int

	loading   bool
	report    discovery.Report
	state     discovery.State
	detailErr string
	selected  domain.Server
}

func newModel(v *profilevault.Vault, vaultPath string) *model {
	return &model{vault: v, vaultPath: vaultPath, screen: screenServerList}
}

func (m *model) Init() tea.Cmd { return nil }

type discoveryMsg struct {
	report discovery.Report
	state  discovery.State
	err    string
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)

	if dm, ok := msg.(discoveryMsg); ok {
		m.loading = false
		m.report = dm.report
		m.state = dm.state
		m.detailErr = dm.err
		return m, nil
	}

	if !isKey {
		return m, nil
	}

	switch m.screen {
	case screenServerList:
		return m.updateServerList(keyMsg)
	case screenServerDetail:
		return m.updateServerDetail(keyMsg)
	}
	return m, nil
}

func (m *model) updateServerList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	servers := m.vault.Registry.Servers
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(servers)-1 {
			m.cursor++
		}
	case "enter":
		if len(servers) == 0 {
			return m, nil
		}
		m.selected = servers[m.cursor]
		m.screen = screenServerDetail
		m.loading = true
		m.detailErr = ""
		m.report = discovery.Report{}
		return m, m.connectAndDiscoverCmd(m.selected)
	}
	return m, nil
}

func (m *model) updateServerDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		m.screen = screenServerList
		return m, nil
	case "r":
		if m.loading {
			return m, nil
		}
		m.loading = true
		m.detailErr = ""
		return m, m.connectAndDiscoverCmd(m.selected)
	}
	return m, nil
}

// connectAndDiscoverCmd dials the server and runs full discovery. It must
// release the terminal first: Bubble Tea owns stdin in raw mode, but
// Dial/discovery need to interactively prompt for a host-key confirmation
// or a password on the same terminal (SPEC.md §10.2, §10.3).
func (m *model) connectAndDiscoverCmd(s domain.Server) tea.Cmd {
	return func() tea.Msg {
		if err := m.program.ReleaseTerminal(); err != nil {
			return discoveryMsg{err: fmt.Sprintf("release terminal: %v", err)}
		}
		defer m.program.RestoreTerminal()

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		controlDir, err := os.MkdirTemp("", "wgtui-ssh-*")
		if err != nil {
			return discoveryMsg{err: err.Error()}
		}
		defer os.RemoveAll(controlDir)

		client := sshtransport.NewClient(sshtransport.Target{
			HostOrAlias: s.SSHHostOrAlias, Port: s.SSHPort, User: s.SSHUser, IdentityFile: s.SSHIdentityFile,
		}, filepath.Join(controlDir, "cm.sock"))

		prompts := newTerminalPrompts(os.Stdout)
		if err := client.Dial(ctx, prompts); err != nil {
			return discoveryMsg{err: fmt.Sprintf("connect: %v", err)}
		}
		defer client.Close(ctx)

		report := discovery.Discover(ctx, client, prompts)
		return discoveryMsg{report: report, state: discovery.Classify(report)}
	}
}
