package tui

import (
	"strings"
	"testing"
)

func TestPeerStateBadge_CoversAllKnownStates(t *testing.T) {
	wantLabel := map[string]string{"never_connected": "never"}
	for _, state := range []string{"online", "idle", "never_connected", "disabled", "unmanaged", "drifted", "error", "something-unexpected"} {
		out := peerStateBadge(state)
		want := state
		if label := wantLabel[state]; label != "" {
			want = label
		}
		if !strings.Contains(out, want) {
			t.Errorf("peerStateBadge(%q) = %q, doesn't contain %q", state, out, want)
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
