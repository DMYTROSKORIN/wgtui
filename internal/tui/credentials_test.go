package tui

import (
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

func TestCredentialCacheReusesValuesUntilCleared(t *testing.T) {
	var c credentialCache

	if _, ok := c.sshPassword(); ok {
		t.Fatal("fresh cache already has an SSH password")
	}
	if _, ok := c.sudoPassword(); ok {
		t.Fatal("fresh cache already has a sudo password")
	}

	c.setSSHPassword("ssh-secret")
	c.setSudoPassword("sudo-secret")

	if v, ok := c.sshPassword(); !ok || v != "ssh-secret" {
		t.Fatalf("sshPassword() = %q, %v; want \"ssh-secret\", true", v, ok)
	}
	if v, ok := c.sudoPassword(); !ok || v != "sudo-secret" {
		t.Fatalf("sudoPassword() = %q, %v; want \"sudo-secret\", true", v, ok)
	}

	c.clear()
	if _, ok := c.sshPassword(); ok {
		t.Fatal("clear() left an SSH password cached")
	}
	if _, ok := c.sudoPassword(); ok {
		t.Fatal("clear() left a sudo password cached")
	}
}

func TestCredentialCacheClearSudoPasswordLeavesSSHIntact(t *testing.T) {
	var c credentialCache
	c.setSSHPassword("ssh-secret")
	c.setSudoPassword("wrong-sudo-secret")

	c.clearSudoPassword()

	if _, ok := c.sudoPassword(); ok {
		t.Fatal("clearSudoPassword() left the sudo password cached")
	}
	if v, ok := c.sshPassword(); !ok || v != "ssh-secret" {
		t.Fatalf("clearSudoPassword() disturbed the SSH password: %q, %v", v, ok)
	}
}

// TestPromptsReusePrecachedCredentialsWithoutAskingAgain pre-populates the
// cache and passes a nil *tea.Program, which makes newTUIPrompts's request
// helper return an error if it is ever actually invoked - so a nil error
// here proves Password/SudoPassword answered from the cache instead of
// asking the (nonexistent) UI, matching the "don't ask the same password a
// hundred times in one server session" requirement.
func TestPromptsReusePrecachedCredentialsWithoutAskingAgain(t *testing.T) {
	var cache credentialCache
	cache.setSSHPassword("ssh-secret")
	cache.setSudoPassword("sudo-secret")

	prompts := newTUIPrompts(nil, "test-target", &cache)

	if v, err := prompts.Password(); err != nil || v != "ssh-secret" {
		t.Fatalf("Password() = %q, %v; want \"ssh-secret\", nil", v, err)
	}
	if v, err := prompts.SudoPassword(); err != nil || v != "sudo-secret" {
		t.Fatalf("SudoPassword() = %q, %v; want \"sudo-secret\", nil", v, err)
	}
}

func TestPromptsAskOnceThenCacheForSubsequentCalls(t *testing.T) {
	var cache credentialCache
	prompts := newTUIPrompts(nil, "test-target", &cache)

	if _, err := prompts.SudoPassword(); err == nil {
		t.Fatal("SudoPassword() with an empty cache and a nil program should fail, not hang or invent a value")
	}
	// The failed request must not have cached a bogus value.
	if _, ok := cache.sudoPassword(); ok {
		t.Fatal("a failed prompt must not populate the cache")
	}
}

func TestRejectedSudoPasswordIsClearedAndReportedNotSilentlyUnprivileged(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	m := testModel(t, server)
	m.screen, m.selected = screenServerDetail, server
	m.creds.setSudoPassword("wrong-one")
	m.detailRequestID = 1

	report := discovery.Report{}
	report.System.SudoPasswordRejected = true
	m.Update(discoveryMsg{serverID: server.ID, requestID: m.detailRequestID, report: report, state: discovery.StateInsufficientPrivileges})

	if _, ok := m.creds.sudoPassword(); ok {
		t.Fatal("a rejected sudo password must be dropped from the cache, not retried again")
	}
	if !m.statusErr || m.statusMsg == "" {
		t.Fatalf("rejected sudo password did not produce a visible error: statusErr=%v statusMsg=%q", m.statusErr, m.statusMsg)
	}
}

func TestReturnToServerListClearsCachedCredentials(t *testing.T) {
	m := testModel(t)
	m.creds.setSSHPassword("ssh-secret")
	m.creds.setSudoPassword("sudo-secret")
	m.detailSession = &session{cleanup: func() {}}

	m.returnToServerList()

	if _, ok := m.creds.sshPassword(); ok {
		t.Fatal("returnToServerList left an SSH password cached across sessions")
	}
	if _, ok := m.creds.sudoPassword(); ok {
		t.Fatal("returnToServerList left a sudo password cached across sessions")
	}
}
