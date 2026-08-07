package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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

func TestServerStateBadge_CoversDiscoveryStates(t *testing.T) {
	for _, state := range []string{"active_consistent", "wireguard_missing", "tools_no_interfaces", "config_exists_stopped", "drift", "ambiguous", "insufficient_privileges", ""} {
		if got := ansi.Strip(serverStateBadge(state)); strings.TrimSpace(got) == "" {
			t.Errorf("serverStateBadge(%q) returned empty", state)
		}
	}
}

func TestBoolStatusAlwaysExplainsMeaning(t *testing.T) {
	if got := ansi.Strip(boolStatus(true, "available", "unavailable")); !strings.Contains(got, "available") || !strings.Contains(got, "✓") {
		t.Fatalf("true status is unclear: %q", got)
	}
	if got := ansi.Strip(boolStatus(false, "available", "unavailable")); !strings.Contains(got, "unavailable") || !strings.Contains(got, "○") {
		t.Fatalf("false status is unclear: %q", got)
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
