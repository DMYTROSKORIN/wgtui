package tui

import (
	"context"
	"errors"
	"fmt"
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
	screenVaultAccess screen = iota
	screenCredentialPrompt
	screenServerList
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
	screenChangePassword
	screenHelp
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

	activeSession         *session
	activeSessionBorrowed bool
	detailSession         *session
	width                 int
	height                int
	helpReturn            screen
	menuOpen              bool
	menuCategory          int
	menuCursor            int

	// startup vault access
	vaultForm      *huh.Form
	vaultCreating  bool
	vaultPassword  string
	vaultConfirm   string
	vaultFormError string

	// in-app SSH/host-key/sudo credential prompt
	credentialForm     *huh.Form
	credentialRequest  *credentialRequestMsg
	credentialValue    string
	credentialAccepted bool
	credentialReturn   screen

	// creds caches the SSH/sudo passwords for as long as the current server
	// stays open, so re-dialing within one working session (install,
	// post-install rediscovery, configure, ...) doesn't ask the user to
	// retype the same secret. Cleared in returnToServerList. See
	// credentialCache's doc comment for what this does and does not protect
	// against.
	creds credentialCache

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
	tab             detailTab
	loading         bool
	report          discovery.Report
	state           discovery.State
	detailErr       string
	detailRequestID uint64

	// peers tab
	peerTable       table.Model
	peers           []peerview.Peer
	peersLoaded     bool
	peersRefreshing bool
	peersErr        string
	peerRequestID   uint64
	peerReconnected bool
	selectedPeer    peerview.Peer
	actionOptions   []peerActionOption
	actionCursor    int
	exportForm      *huh.Form
	exportPath      string
	passwordForm    *huh.Form
	currentPassword string
	newPassword     string
	confirmPassword string

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
	revealCursor   int
	revealNew      bool
	exportReturn   screen

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
	checks         []diagnostics.Check
	checksLoaded   bool
	checksErr      string
	diagnoseOffset int
}

func newModel(v *profilevault.Vault, vaultPath string) *model {
	return &model{vault: v, vaultPath: vaultPath, screen: screenServerList, width: 100, height: 30}
}

func newLockedModel(vaultPath string, creating bool) *model {
	m := &model{vaultPath: vaultPath, screen: screenVaultAccess, width: 100, height: 30, vaultCreating: creating}
	m.startVaultAccessForm()
	_ = m.vaultForm.Init()
	return m
}

func (m *model) Init() tea.Cmd {
	if m.screen == screenVaultAccess && m.vaultForm != nil {
		return m.vaultForm.Init()
	}
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouse, ok := msg.(tea.MouseMsg); ok {
		return m.updateCommanderMouse(mouse)
	}
	// Ctrl+C is an application-level escape hatch. Handle it before any
	// screen or huh form sees the key; huh normally treats Ctrl+C as a
	// local form abort, which made quitting depend on the active screen.
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "ctrl+c" || key.String() == "f10" {
			m.cancelCredentialPrompt("application closed")
			m.closeSessions()
			return m, tea.Quit
		}
		if terminalTooSmall(m.width, m.height) {
			// Keep the underlying screen stable while the terminal is unusably
			// small. Non-key messages (including completed network operations)
			// still flow through the state machine below.
			return m, nil
		}
		if m.menuOpen {
			return m.updateCommanderMenu(key)
		}
		if (key.String() == "m" || key.String() == "f9") && m.commanderAvailable() {
			m.openCommanderMenu()
			return m, nil
		}
		if cmd, handled := m.handleCommanderAccelerator(key.String()); handled {
			return m, cmd
		}
		if cmd, handled := m.handleCommanderFunctionKey(key.String()); handled {
			return m, cmd
		}
		switch key.String() {
		case "f1":
			if m.screen != screenHelp {
				m.helpReturn = m.screen
				m.screen = screenHelp
			}
			return m, nil
		case "?":
			if m.screen == screenServerList || m.screen == screenServerDetail || m.screen == screenPeerActionMenu || m.screen == screenPeerReveal {
				m.helpReturn = m.screen
				m.screen = screenHelp
				return m, nil
			}
		}
	}
	if request, ok := msg.(credentialRequestMsg); ok {
		return m.beginCredentialPrompt(request)
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		oldWidth, oldHeight := m.width, m.height
		oldMode := layoutModeFor(m.width, m.height)
		m.width, m.height = size.Width, size.Height
		if m.peersLoaded {
			m.rebuildPeerTable()
		}
		// Terminal emulators do not all erase rows that disappeared after a
		// resize. A full clear on breakpoint changes prevents stale borders and
		// duplicated footers without flickering during ordinary updates.
		if oldMode != layoutModeFor(m.width, m.height) || size.Width < oldWidth || size.Height < oldHeight {
			return m, tea.ClearScreen
		}
		return m, nil
	}
	switch msg := msg.(type) {
	case discoveryMsg:
		if msg.serverID != m.selected.ID || msg.requestID != m.detailRequestID {
			// A connection can finish after the user has left this server or
			// started a newer refresh. Never install that stale session.
			msg.sess.close()
			return m, nil
		}
		m.loading = false
		if msg.err != "" {
			msg.sess.close()
		} else if msg.sess != nil {
			m.detailSession.close()
			m.detailSession = msg.sess
		}
		m.report = msg.report
		m.state = msg.state
		m.detailErr = msg.err
		if msg.report.System.SudoPasswordRejected {
			// Never keep retrying a password the server has already
			// refused - the next privileged action should ask again, not
			// silently reuse it (and fail again) or proceed unprivileged.
			m.creds.clearSudoPassword()
			m.setError(errors.New("sudo password was rejected by the server - press R to reconnect and try again"))
		}
		if m.tab == tabPeers && !m.peersLoaded {
			if msg.err != "" {
				m.peersLoaded = true
				m.peersRefreshing = false
				m.peersErr = "server connection failed: " + msg.err
				return m, nil
			}
			return m, m.loadPeersFromSessionCmd()
		}
		return m, nil
	case peerListMsg:
		if msg.serverID != m.selected.ID || msg.requestID != m.peerRequestID {
			return m, nil
		}
		if msg.connectionLost && !m.peerReconnected {
			// A dead ControlMaster cannot recover by repeating a command on the
			// same socket. Reconnect once, then let discovery automatically
			// restart the peer load for the still-active Peers tab.
			m.peerReconnected = true
			m.detailSession.close()
			m.detailSession = nil
			m.loading = true
			m.detailErr = ""
			m.peersLoaded = false
			m.peersRefreshing = false
			m.peersErr = ""
			return m, m.connectAndDiscoverCmd(m.selected)
		}
		m.peersLoaded = true
		m.peersRefreshing = false
		m.peersErr = msg.err
		m.peers = msg.peers
		if msg.err == "" {
			m.peerReconnected = false
		}
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
	case screenVaultAccess:
		return m.updateVaultAccess(msg)
	case screenCredentialPrompt:
		return m.updateCredentialPrompt(msg)
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
	case screenChangePassword:
		return m.updatePasswordForm(msg)
	case screenHelp:
		if isKey && (keyMsg.String() == "esc" || keyMsg.String() == "q" || keyMsg.String() == "enter") {
			m.screen = m.helpReturn
		}
		return m, nil
	}
	return m, nil
}

// runAsync keeps Bubble Tea in control of the alternate screen while remote
// work runs. Interactive SSH questions return as model messages and are
// answered by an in-app modal instead of tearing down the TUI.
func (m *model) runAsync(fn func() tea.Msg) tea.Cmd {
	return func() (result tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				result = errMsg{err: fmt.Errorf("internal error: %v (this is a bug - please report it)", r)}
			}
		}()
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

func (m *model) closeSessions() {
	m.activeSession.close()
	m.activeSession = nil
	m.detailSession.close()
	m.detailSession = nil
}

func (m *model) releaseActiveSession() {
	if !m.activeSessionBorrowed {
		m.activeSession.close()
	}
	m.activeSession = nil
	m.activeSessionBorrowed = false
}

func (m *model) updateServerList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	servers := m.vault.Registry.Servers
	switch msg.String() {
	case "q":
		m.closeSessions()
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(servers)-1 {
			m.cursor++
		}
	case "a", "n", "f2":
		m.clearStatus()
		m.startServerForm(false)
		return m, m.serverForm.Init()
	case "e", "f4":
		if len(servers) == 0 {
			return m, nil
		}
		m.clearStatus()
		m.selected = servers[m.cursor]
		m.startServerForm(true)
		return m, m.serverForm.Init()
	case "x", "d":
		if len(servers) == 0 {
			return m, nil
		}
		m.clearStatus()
		m.startRemoveConfirm(servers[m.cursor])
		return m, m.removeForm.Init()
	case "p":
		m.clearStatus()
		m.startPasswordForm()
		return m, m.passwordForm.Init()
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
		m.peerRequestID++
		m.peers = nil
		m.peersLoaded = false
		m.peersRefreshing = false
		m.peersErr = ""
		m.peerReconnected = false
		m.checksLoaded = false
		return m, m.connectAndDiscoverCmd(m.selected)
	}
	return m, nil
}

func (m *model) updateServerDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.detailRequestID++
		m.peerRequestID++
		m.peersRefreshing = false
		m.peerReconnected = false
		m.detailSession.close()
		m.detailSession = nil
		m.screen = screenServerList
		m.clearStatus()
		return m, nil
	case "tab":
		m.tab = (m.tab + 1) % 3
		m.diagnoseOffset = 0
		if m.tab == tabPeers && !m.peersLoaded {
			if m.loading {
				return m, nil
			}
			return m, m.loadPeersFromSessionCmd()
		}
		if m.tab == tabDiagnose && !m.checksLoaded {
			m.checks = diagnostics.BuildChecks(m.report, m.vault.ListPeers(m.selected.ID), time.Now())
			m.checksLoaded = true
		}
		return m, nil
	case "r", "f5":
		switch m.tab {
		case tabPeers:
			m.peerReconnected = false
			return m, m.loadPeersFromSessionCmd()
		case tabDiagnose:
			m.checks = diagnostics.BuildChecks(m.report, m.vault.ListPeers(m.selected.ID), time.Now())
			m.checksLoaded = true
			m.diagnoseOffset = 0
			return m, nil
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
	case "a", "n", "f2":
		if m.tab == tabPeers {
			m.startPeerForm()
			return m, m.peerForm.Init()
		}
	case "enter", "f8":
		if m.tab == tabPeers {
			return m.openActionMenu()
		}
	}
	if m.tab == tabDiagnose {
		maxOffset := max(0, len(m.diagnoseLines())-m.diagnoseVisibleRows())
		if layoutModeFor(m.width, m.height) == layoutWide {
			maxOffset = max(0, len(m.checks)-1)
		}
		switch msg.String() {
		case "up", "k":
			if m.diagnoseOffset > 0 {
				m.diagnoseOffset--
			}
		case "down", "j":
			if m.diagnoseOffset < maxOffset {
				m.diagnoseOffset++
			}
		}
		return m, nil
	}
	if m.tab == tabPeers {
		var cmd tea.Cmd
		m.peerTable, cmd = m.peerTable.Update(msg)
		return m, cmd
	}
	return m, nil
}

// connectAndDiscoverCmd dials the server and runs full discovery. Interactive
// host-key, SSH-password and sudo questions are brokered back into Bubble Tea
// as credentialRequestMsg values while this command waits in the background.
func (m *model) connectAndDiscoverCmd(s domain.Server) tea.Cmd {
	m.detailRequestID++
	requestID := m.detailRequestID
	return m.runAsync(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		prompts := newTUIPrompts(m.program, serverLabel(s), &m.creds)
		sess, err := dial(ctx, s, prompts)
		if err != nil {
			return discoveryMsg{serverID: s.ID, requestID: requestID, err: fmt.Sprintf("connect: %v", err)}
		}

		report := discovery.Discover(ctx, sess.client, prompts)
		return discoveryMsg{serverID: s.ID, requestID: requestID, report: report, state: discovery.Classify(report), sess: sess}
	})
}
