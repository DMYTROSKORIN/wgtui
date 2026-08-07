package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// serverLabel is a short, unambiguous identifier for a registered server,
// used in every connect banner and password prompt so it's always clear
// *which* machine a question is about.
func serverLabel(s domain.Server) string {
	host := s.SSHHostOrAlias
	if s.SSHUser != "" {
		host = s.SSHUser + "@" + host
	}
	if s.DisplayName != "" && s.DisplayName != host {
		return fmt.Sprintf("%s (%s)", s.DisplayName, host)
	}
	return host
}

// connectingBanner prints a one-line "connecting to X" notice before any
// interactive prompt appears and returns the same label the prompts below
// use - reported confusion: a user seeing an unlabeled "password:" prompt
// mid-session assumed it was re-asking their wgtui master password. By
// the time any prompt shows, the user has already seen which *remote*
// server this is about.
func connectingBanner(out io.Writer, s domain.Server) string {
	label := serverLabel(s)
	fmt.Fprintf(out, "Connecting to %s...\n", label)
	return label
}

// newTerminalPrompts mirrors internal/cli's prompts.go, including the
// same in-memory password caching (see that file's doc comment for why:
// RunSudo re-authenticates on every privileged call, so without caching a
// single server-detail refresh would prompt for the sudo password many
// times over). It is duplicated rather than imported: cli and tui are
// independent front ends over the same lower layers
// (docs/ARCHITECTURE.md) and neither should depend on the other just to
// share this much terminal I/O. Both call sites must only be used after
// the tea.Program has released the terminal (see run.go) - Bubble Tea
// otherwise owns stdin in raw mode.
//
// target (from connectingBanner/serverLabel) is woven into every prompt's
// own text - never just "SSH password:" on its own - so it's always
// unambiguous this is the *remote server's* credential, not wgtui's local
// vault master password (asked for exactly once, at startup, and nowhere
// else in the program).
func newTerminalPrompts(out io.Writer, target string) sshtransport.Prompts {
	in := bufio.NewReader(os.Stdin)
	var cachedPassword, cachedSudoPassword *string

	return sshtransport.Prompts{
		HostKey: func(prompt string) bool {
			fmt.Fprintln(out, strings.TrimSpace(prompt))
			fmt.Fprintf(out, "This is %s's host key, seen for the first time - accept and continue connecting? [y/N] ", target)
			line, _ := in.ReadString('\n')
			answer := strings.ToLower(strings.TrimSpace(line))
			return answer == "y" || answer == "yes"
		},
		Password: func() (string, error) {
			if cachedPassword != nil {
				return *cachedPassword, nil
			}
			pw, err := promptHiddenInput(out, fmt.Sprintf("SSH login password for %s (the remote server - not your wgtui master password): ", target))
			if err != nil {
				return "", err
			}
			cachedPassword = &pw
			return pw, nil
		},
		SudoPassword: func() (string, error) {
			if cachedSudoPassword != nil {
				return *cachedSudoPassword, nil
			}
			pw, err := promptHiddenInput(out, fmt.Sprintf("sudo password ON %s (the remote server - usually the same as its SSH password): ", target))
			if err != nil {
				return "", err
			}
			cachedSudoPassword = &pw
			return pw, nil
		},
	}
}

// promptHiddenInput never falls back to an unmasked read: if stdin isn't
// a real terminal (or term.ReadPassword itself fails), it returns an
// error instead of silently echoing the input - a password should never
// have a code path that prints it back.
func promptHiddenInput(out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("stdin is not a terminal")
	}
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
