package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// errMsg is a catch-all async failure with nowhere more specific to go -
// shown as a status banner on whatever screen the user lands back on.
type errMsg struct{ err error }

type discoveryMsg struct {
	serverID  string
	requestID uint64
	report    discovery.Report
	state     discovery.State
	err       string
	sess      *session
}

type peerListMsg struct {
	serverID       string
	requestID      uint64
	peers          []peerview.Peer
	err            string
	connectionLost bool
}

type diagnoseMsg struct {
	checks []diagnostics.Check
	err    string
}

// planReadyMsg is posted once a mutating flow's "connect and gather what's
// needed to show a plan" phase finishes. Every one of them - install,
// configure, peer add/disable/enable/revoke/rotate - funnels through the
// same confirm screen this way: apply is already closed over whatever
// that specific flow needs (the fetched CurrentConfig, computed addresses,
// form values, ...), so the generic handler here doesn't need to know
// which flow it's driving.
type planReadyMsg struct {
	err          error
	sess         *session
	title        string
	plan         string
	apply        func(m *model) tea.Cmd
	cancelScreen screen
	// revoke's plan needs its own 3-question form instead of a single
	// yes/no - set only for that flow.
	multiStep bool
	borrowed  bool
}

// actionDoneMsg is posted once a confirmed action's apply phase finishes.
// done decides what the user sees next (a result banner, the peer reveal
// screen, a refreshed peer list, ...) and is called before the session is
// closed, so it can still be certain the connection was valid up to this
// point (it never needs to touch it directly). It may return a follow-up
// tea.Cmd (e.g. re-running discovery after install/configure); nil if
// there's nothing more to do.
type actionDoneMsg struct {
	err  error
	done func(m *model) tea.Cmd
}

func (m *model) handlePlanReady(msg planReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if !msg.borrowed {
			msg.sess.close()
		}
		m.setError(msg.err)
		m.screen = msg.cancelScreen
		return m, nil
	}
	m.activeSession = msg.sess
	m.activeSessionBorrowed = msg.borrowed
	m.confirmTitle = msg.title
	m.confirmPlan = msg.plan
	m.onCancelled = msg.cancelScreen

	if msg.multiStep {
		m.onConfirmed = msg.apply
		m.revokeYes, m.revokeDeleteProfile, m.revokeDeleteMetadata = false, false, false
		m.revokeForm = newRevokeForm(m)
		m.screen = screenRevokeConfirm
		return m, m.revokeForm.Init()
	}

	m.onConfirmed = msg.apply
	m.confirmYes = false
	m.confirmForm = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(msg.title).
			Affirmative("Apply").
			Negative("Cancel").
			Value(&m.confirmYes),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.screen = screenConfirm
	return m, m.confirmForm.Init()
}

func (m *model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.confirmForm.Update(msg)
	m.confirmForm = form.(*huh.Form)

	switch m.confirmForm.State {
	case huh.StateCompleted:
		if !m.confirmYes {
			m.releaseActiveSession()
			m.screen = m.onCancelled
			return m, nil
		}
		// Must leave screenConfirm *before* running onConfirmed: the form's
		// State stays StateCompleted forever once set, so if m.screen were
		// still screenConfirm, literally the next tea.Msg Bubble Tea
		// delivers (a cursor-blink tick, anything) would route back into
		// this same case and call onConfirmed a second time - by then
		// m.activeSession has already been closed and nilled out by
		// handleActionDone, so the second call panics on a nil session.
		// Caught for real during manual testing: this crashed the whole
		// program, not just this screen.
		m.screen = screenApplying
		return m, m.onConfirmed(m)
	case huh.StateAborted:
		m.releaseActiveSession()
		m.screen = m.onCancelled
		return m, nil
	}
	return m, cmd
}

func (m *model) handleActionDone(msg actionDoneMsg) (tea.Model, tea.Cmd) {
	// Default back to the screen this flow started from - the same one
	// onCancelled already points at - so a plain error (no done closure)
	// or a done closure that doesn't care never leaves the user stuck on
	// screenApplying. A closure that needs somewhere else (peer add's
	// reveal screen) sets m.screen itself, after this line runs.
	m.screen = m.onCancelled
	if msg.err != nil {
		if errors.Is(msg.err, sshtransport.ErrSudoPasswordRejected) {
			// A privileged step inside this action (install, configure,
			// peer add/enable/disable/revoke/rotate, ...) hit a sudo
			// password the server has already refused - never keep
			// retrying it silently, and say what actually happened instead
			// of leaving the user staring at a bare exit code.
			m.creds.clearSudoPassword()
		}
		m.setError(msg.err)
	}
	var cmd tea.Cmd
	if msg.done != nil {
		cmd = msg.done(m)
	}
	m.releaseActiveSession()
	return m, cmd
}
