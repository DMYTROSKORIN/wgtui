package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

func TestServerLabel(t *testing.T) {
	cases := []struct {
		name string
		s    domain.Server
		want string
	}{
		{
			name: "display name and user",
			s:    domain.Server{DisplayName: "office", SSHHostOrAlias: "1.2.3.4", SSHUser: "admin"},
			want: "office (admin@1.2.3.4)",
		},
		{
			name: "no display name falls back to host",
			s:    domain.Server{SSHHostOrAlias: "nas.example"},
			want: "nas.example",
		},
		{
			name: "display name equal to host is not duplicated",
			s:    domain.Server{DisplayName: "nas.example", SSHHostOrAlias: "nas.example"},
			want: "nas.example",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serverLabel(tc.s); got != tc.want {
				t.Errorf("serverLabel(%+v) = %q, want %q", tc.s, got, tc.want)
			}
		})
	}
}

func TestConnectingBanner_PrintsAndReturnsLabel(t *testing.T) {
	var buf bytes.Buffer
	s := domain.Server{DisplayName: "office", SSHHostOrAlias: "1.2.3.4", SSHUser: "admin"}
	got := connectingBanner(&buf, s)
	if got != "office (admin@1.2.3.4)" {
		t.Errorf("connectingBanner returned %q", got)
	}
	if !strings.Contains(buf.String(), "office (admin@1.2.3.4)") {
		t.Errorf("banner output %q doesn't mention the server label", buf.String())
	}
}

// Regression guard: every remote-credential prompt must name the target
// server and explicitly disclaim it's not the wgtui master password -
// reported confusion was a user unable to tell an SSH/sudo prompt apart
// from the vault's own master password prompt.
func TestNewTerminalPrompts_TextNamesTargetAndDisclaimsMasterPassword(t *testing.T) {
	var buf bytes.Buffer
	prompts := newTerminalPrompts(&buf, "office (admin@1.2.3.4)")

	// Password/SudoPassword read from stdin, which isn't a terminal in a
	// test - they're expected to error out, but must still have printed
	// their prompt text (containing the target and, for SSH, the
	// "not your wgtui master password" disclaimer) before failing.
	_, _ = prompts.Password()
	if !strings.Contains(buf.String(), "office (admin@1.2.3.4)") {
		t.Errorf("SSH password prompt doesn't name the target server: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "not your wgtui master password") {
		t.Errorf("SSH password prompt doesn't disclaim the master password: %q", buf.String())
	}

	buf.Reset()
	_, _ = prompts.SudoPassword()
	if !strings.Contains(buf.String(), "office (admin@1.2.3.4)") {
		t.Errorf("sudo password prompt doesn't name the target server: %q", buf.String())
	}
}
