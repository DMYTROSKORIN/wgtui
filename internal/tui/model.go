package tui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// screen identifies which top-level view is active. wgtui is a real
// interactive management app (server add/edit/remove, install/configure,
// full peer lifecycle), not just a read-only viewer over the CLI - every
// mutation the CLI can do has a screen here too.
type screen int

const (
	screenServerList screen = iota
	screenServerForm
	screenServerRemoveConfirm
	screenServerDetail
	screenConfirm
	screenApplying
	screenConfigureForm
	screenPeerForm
	screenPeerReveal
	screenPeerActionMenu
	screenRevokeConfirm
	screenPeerExportForm
)

type detailTab int

const (
	tabOverview detailTab = iota
	tabPeers
	tabDiagnose
)

// session holds one open SSH connection across several TUI screens (e.g.
// connect -> show a plan -> confirm -> apply), mirroring how a single CLI
// command holds one connection open from its first remote call to its
// last. Every code path that creates a session (confirm, cancel, or an
// error at any step) must eventually call close(); it's idempotent so
// that's safe to do from more than one place defensively.
type session struct {
	client  *sshtransport.Client
	prompts sshtransport.Prompts
	cleanup func()
	closed  bool
}

func (s *session) close() {
	if s == nil || s.closed {
		return
	}
	s.closed = true
	s.cleanup()
}

// model is wgtui's single Bubble Tea model. Each screen keeps its state in
// a handful of fields here rather than as separate tea.Model
// implementations - screens are simple enough, and plenty of state
// (selected server, active session, vault) is naturally shared between
// them, so one model with a screen-dispatching Update/View is less
// machinery than composing many small models.
type model struct {
	vault     *profilevault.Vault
	vaultPath string
	program   *tea.Program

	screen screen
	cursor int // server list cursor

	statusMsg string // one-line banner shown on return to a screen
	statusErr bool

	activeSession *session

	// server list / add / edit / remove
	selected   domain.Server
	serverForm *huh.Form
	serverEdit bool
	fID        string
	fName      string
	fHost      string
	fUser      string
	fPort      string
	fIdentity  string
	removeForm *huh.Form

	// server detail
	tab       detailTab
	loading   bool
	report    discovery.Report
	state     discovery.State
	detailErr string

	// peers tab
	peerTable        table.Model
	peers            []peerview.Peer
	peersLoaded      bool
	peersErr         string
	selectedPeer     peerview.Peer
	actionMenu       *huh.Form
	actionMenuChoice *string
	exportForm       *huh.Form
	exportPath       string

	// generic single-step plan/confirm (install, disable, enable, rotate)
	confirmForm  *huh.Form
	confirmTitle string
	confirmPlan  string
	confirmYes   bool
	onConfirmed  func(m *model) tea.Cmd
	onCancelled  screen

	// revoke's 3-part confirm (server access / delete profile / delete metadata)
	revokeForm           *huh.Form
	revokeYes            bool
	revokeDeleteProfile  bool
	revokeDeleteMetadata bool

	// peer add form
	peerForm       *huh.Form
	pfName         string
	pfFullTunnel   bool
	pfSplitCIDRs   string
	pfKeepalive    string
	pfDNS          string
	pfEndpointHost string
	pfPSK          bool
	revealProfile  string
	revealName     string
	revealMode     string

	// configure wizard form
	cfForm              *huh.Form
	cfInterface         string
	cfVPNCIDR           string
	cfPort              string
	cfOutboundInterface string
	cfFullTunnelDefault bool
	cfSuggestedCIDR     string
	cfDetectedOutbound  string

	// diagnose tab
	checks       []diagnostics.Check
	checksLoaded bool
	checksErr    string
}

func newModel(v *profilevault.Vault, vaultPath string) *model {
	return &model{vault: v, vaultPath: vaultPath, screen: screenServerList}
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case discoveryMsg:
		m.loading = false
		m.report = msg.report
		m.state = msg.state
		m.detailErr = msg.err
		return m, nil
	case peerListMsg:
		m.peersLoaded = true
		m.peersErr = msg.err
		m.peers = msg.peers
		m.rebuildPeerTable()
		return m, nil
	case diagnoseMsg:
		m.checksLoaded = true
		m.checksErr = msg.err
		m.checks = msg.checks
		return m, nil
	case planReadyMsg:
		return m.handlePlanReady(msg)
	case actionDoneMsg:
		return m.handleActionDone(msg)
	case configurePrepMsg:
		return m.handleConfigurePrep(msg)
	case errMsg:
		m.setError(msg.err)
		return m, nil
	}

	keyMsg, isKey := msg.(tea.KeyMsg)

	switch m.screen {
	case screenServerList:
		if isKey {
			return m.updateServerList(keyMsg)
		}
	case screenServerForm:
		return m.updateServerForm(msg)
	case screenServerRemoveConfirm:
		return m.updateRemoveConfirm(msg)
	case screenServerDetail:
		if isKey {
			return m.updateServerDetail(keyMsg)
		}
		if m.tab == tabPeers {
			var cmd tea.Cmd
			m.peerTable, cmd = m.peerTable.Update(msg)
			return m, cmd
		}
	case screenConfirm:
		return m.updateConfirm(msg)
	case screenConfigureForm:
		return m.updateConfigureForm(msg)
	case screenPeerForm:
		return m.updatePeerForm(msg)
	case screenPeerReveal:
		if isKey {
			return m.updatePeerReveal(keyMsg)
		}
	case screenPeerActionMenu:
		return m.updateActionMenu(msg)
	case screenRevokeConfirm:
		return m.updateRevokeConfirm(msg)
	case screenPeerExportForm:
		return m.updateExportForm(msg)
	}
	return m, nil
}

// releaseTerminalAndRun runs fn with the real terminal (Bubble Tea's raw
// mode released), restoring it again once fn returns - every flow that
// needs an interactive host-key/password/sudo prompt goes through this,
// since Bubble Tea otherwise owns stdin in raw mode. This is also the
// single highest-risk spot in the whole program for a genuine crash: it's
// the only place doing raw terminal-mode surgery concurrently with
// Bubble Tea's own event loop, and a reported "the app just dies at the
// sudo password prompt" crash couldn't be reproduced against a real SSH
// target despite trying - so on top of fixing every specific bug found,
// recover() here converts *any* panic in this window into an error
// message instead of taking the whole program down. The recover defer is
// registered before ReleaseTerminal specifically so it still runs (LIFO)
// after RestoreTerminal's defer below, if that's what panics - terminal
// state gets put back first either way.
func (m *model) releaseTerminalAndRun(fn func() tea.Msg) tea.Cmd {
	return func() (result tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				result = errMsg{err: fmt.Errorf("internal error: %v (this is a bug - please report it)", r)}
			}
		}()
		if err := m.program.ReleaseTerminal(); err != nil {
			return errMsg{err: err}
		}
		defer m.program.RestoreTerminal()
		return fn()
	}
}

// clearStatus drops any banner from a previous screen before entering a
// new one, so a stale success/error message can't linger where it no
// longer applies.
func (m *model) clearStatus() {
	m.statusMsg = ""
	m.statusErr = false
}

func (m *model) setError(err error) {
	m.statusMsg = err.Error()
	m.statusErr = true
}

func (m *model) setStatus(msg string) {
	m.statusMsg = msg
	m.statusErr = false
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
	case "a":
		m.clearStatus()
		m.startServerForm(false)
		return m, m.serverForm.Init()
	case "e":
		if len(servers) == 0 {
			return m, nil
		}
		m.clearStatus()
		m.selected = servers[m.cursor]
		m.startServerForm(true)
		return m, m.serverForm.Init()
	case "x":
		if len(servers) == 0 {
			return m, nil
		}
		m.clearStatus()
		m.startRemoveConfirm(servers[m.cursor])
		return m, m.removeForm.Init()
	case "enter":
		if len(servers) == 0 {
			return m, nil
		}
		m.clearStatus()
		m.selected = servers[m.cursor]
		m.screen = screenServerDetail
		m.tab = tabOverview
		m.loading = true
		m.detailErr = ""
		m.report = discovery.Report{}
		m.peersLoaded = false
		m.checksLoaded = false
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
		m.clearStatus()
		return m, nil
	case "tab":
		m.tab = (m.tab + 1) % 3
		if m.tab == tabPeers && !m.peersLoaded {
			return m, m.loadPeersCmd()
		}
		if m.tab == tabDiagnose && !m.checksLoaded {
			return m, m.loadDiagnoseCmd()
		}
		return m, nil
	case "r":
		switch m.tab {
		case tabPeers:
			m.peersLoaded = false
			return m, m.loadPeersCmd()
		case tabDiagnose:
			m.checksLoaded = false
			return m, m.loadDiagnoseCmd()
		default:
			if m.loading {
				return m, nil
			}
			m.loading = true
			m.detailErr = ""
			return m, m.connectAndDiscoverCmd(m.selected)
		}
	case "i":
		if m.tab == tabOverview && !m.loading {
			return m, m.startInstallWizard()
		}
	case "c":
		if m.tab == tabOverview && !m.loading {
			return m, m.startConfigurePrep()
		}
	case "a":
		if m.tab == tabPeers {
			m.startPeerForm()
			return m, m.peerForm.Init()
		}
	case "enter":
		if m.tab == tabPeers {
			return m.openActionMenu()
		}
	}
	return m, nil
}

// connectAndDiscoverCmd dials the server and runs full discovery. It must
// release the terminal first: Bubble Tea owns stdin in raw mode, but
// Dial/discovery need to interactively prompt for a host-key confirmation
// or a password on the same terminal.
func (m *model) connectAndDiscoverCmd(s domain.Server) tea.Cmd {
	return m.releaseTerminalAndRun(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		target := connectingBanner(os.Stdout, s)
		prompts := newTerminalPrompts(os.Stdout, target)
		sess, err := dial(ctx, s, prompts)
		if err != nil {
			return discoveryMsg{err: fmt.Sprintf("connect: %v", err)}
		}
		defer sess.close()

		report := discovery.Discover(ctx, sess.client, prompts)
		return discoveryMsg{report: report, state: discovery.Classify(report)}
	})
}
