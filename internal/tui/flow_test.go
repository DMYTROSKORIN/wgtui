package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// Regression test for a crash caught during manual testing: confirming a
// mutating action left m.screen on screenConfirm while the async apply
// ran. Since huh.Form.State stays StateCompleted forever once set, the
// *next* tea.Msg Bubble Tea delivers to that screen - a cursor-blink tick,
// anything - re-entered updateConfirm's StateCompleted branch and called
// onConfirmed a second time. By then handleActionDone had already closed
// and nilled m.activeSession, so the second call operated on a nil
// session and panicked, taking down the whole program - exactly what was
// reported ("работать вообще невозможно... приложение вылетает").
func TestConfirm_LeavesScreenConfirmBeforeApplying(t *testing.T) {
	m := testModel(t)
	calls := 0
	m.onCancelled = screenServerDetail
	m.onConfirmed = func(m *model) tea.Cmd {
		calls++
		// A real apply closure dereferences m.activeSession - if this
		// runs a second time after handleActionDone already cleared it,
		// the equivalent real code would nil-panic here.
		if m.activeSession == nil {
			t.Fatal("onConfirmed called with a nil activeSession - this is the crash")
		}
		return nil
	}
	m.activeSession = &session{cleanup: func() {}}
	m.confirmYes = true
	m.confirmForm = huh.NewForm(huh.NewGroup(huh.NewConfirm().Value(&m.confirmYes)))
	m.confirmForm.State = huh.StateCompleted
	m.screen = screenConfirm

	res, _ := m.updateConfirm(nil)
	rm := res.(*model)

	if rm.screen == screenConfirm {
		t.Fatal("screen is still screenConfirm after confirming - the next stray message would re-trigger onConfirmed")
	}
	if calls != 1 {
		t.Fatalf("onConfirmed called %d times, want 1", calls)
	}

	// Simulate handleActionDone having already run and cleared the
	// session, then a stray subsequent message arriving at the top-level
	// Update dispatch - it must NOT reach updateConfirm again now that
	// the screen has moved on.
	rm.activeSession = nil
	rm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if calls != 1 {
		t.Fatalf("onConfirmed called %d times after the screen moved on, want still 1 (no re-entrance)", calls)
	}
}

func TestRevokeConfirm_LeavesScreenBeforeApplying(t *testing.T) {
	m := testModel(t)
	calls := 0
	m.onCancelled = screenServerDetail
	m.onConfirmed = func(m *model) tea.Cmd {
		calls++
		return nil
	}
	m.activeSession = &session{cleanup: func() {}}
	m.revokeForm = newRevokeForm(m)
	m.revokeForm.State = huh.StateCompleted
	m.screen = screenRevokeConfirm

	res, _ := m.updateRevokeConfirm(nil)
	rm := res.(*model)

	if rm.screen == screenRevokeConfirm {
		t.Fatal("screen is still screenRevokeConfirm after confirming")
	}
	if calls != 1 {
		t.Fatalf("onConfirmed called %d times, want 1", calls)
	}
}

// Regression test: handleActionDone must not leave the user stuck on
// screenApplying forever when a flow's error path (no done closure) or a
// done closure that doesn't care about the screen completes.
func TestHandleActionDone_DefaultsBackToOnCancelled(t *testing.T) {
	m := testModel(t)
	m.onCancelled = screenServerDetail
	m.activeSession = &session{cleanup: func() {}}
	m.screen = screenApplying

	res, _ := m.handleActionDone(actionDoneMsg{err: errTest})
	rm := res.(*model)
	if rm.screen != screenServerDetail {
		t.Errorf("screen = %v, want screenServerDetail (stuck on screenApplying otherwise)", rm.screen)
	}
	if rm.activeSession != nil {
		t.Error("expected activeSession to be closed and nilled")
	}
}

type testError string

func (e testError) Error() string { return string(e) }

const errTest = testError("boom")
