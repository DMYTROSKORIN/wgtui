package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
)

// errMsg is a catch-all async failure with nowhere more specific to go -
// shown as a status banner on whatever screen the user lands back on.
type errMsg struct{ err error }

type discoveryMsg struct {
	report discovery.Report
	state  discovery.State
	err    string
}

type peerListMsg struct {
	peers []peerview.Peer
	err   string
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
		msg.sess.close()
		m.setError(msg.err)
		m.screen = msg.cancelScreen
		return m, nil
	}
	m.activeSession = msg.sess
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
			m.activeSession.close()
			m.activeSession = nil
			m.screen = m.onCancelled
			return m, nil
		}
		return m, m.onConfirmed(m)
	case huh.StateAborted:
		m.activeSession.close()
		m.activeSession = nil
		m.screen = m.onCancelled
		return m, nil
	}
	return m, cmd
}

func (m *model) handleActionDone(msg actionDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setError(msg.err)
	}
	var cmd tea.Cmd
	if msg.done != nil {
		cmd = msg.done(m)
	}
	m.activeSession.close()
	m.activeSession = nil
	return m, cmd
}
