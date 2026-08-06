package sshtransport

import (
	"context"
	"strings"
	"testing"
)

// TestNewSSHCmd_ForcesNoAskpass locks in the fix for a real hang found
// against a disposable test VM: with an inherited SSH_ASKPASS (set by
// most desktop Linux sessions), ssh can divert a prompt - including the
// first-time host-key confirmation - to an external GUI helper instead of
// the PTY we're reading from, hanging until our own timeout kills it.
func TestNewSSHCmd_ForcesNoAskpass(t *testing.T) {
	cmd := newSSHCmd(context.Background(), "-V")
	found := false
	for _, kv := range cmd.Env {
		if kv == "SSH_ASKPASS_REQUIRE=never" {
			found = true
		}
		if strings.HasPrefix(kv, "SSH_ASKPASS_REQUIRE=") && kv != "SSH_ASKPASS_REQUIRE=never" {
			t.Errorf("conflicting SSH_ASKPASS_REQUIRE entry: %s", kv)
		}
	}
	if !found {
		t.Error("newSSHCmd's Env does not contain SSH_ASKPASS_REQUIRE=never")
	}
}
