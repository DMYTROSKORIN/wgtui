package tui

import (
	"strings"
	"testing"
)

func TestPeerStateBadge_CoversAllKnownStates(t *testing.T) {
	for _, state := range []string{"online", "idle", "never_connected", "disabled", "unmanaged", "drifted", "error", "something-unexpected"} {
		out := peerStateBadge(state)
		if !strings.Contains(out, state) {
			t.Errorf("peerStateBadge(%q) = %q, doesn't contain the state name", state, out)
		}
	}
}

func TestCheckStatusBadge_CoversAllKnownStatuses(t *testing.T) {
	for _, status := range []string{"pass", "warn", "fail", "skip", "something-unexpected"} {
		out := checkStatusBadge(status)
		if out == "" {
			t.Errorf("checkStatusBadge(%q) returned empty", status)
		}
	}
}
