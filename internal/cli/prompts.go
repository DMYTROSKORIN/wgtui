package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// newTerminalPrompts implements sshtransport.Prompts by reading from the
// controlling terminal. Passwords are read with local echo disabled
// (SPEC.md §10.3); nothing here ever writes a secret to stdout, a log, or
// a command argument.
//
// SudoPassword and Password each cache the value in memory after the
// first successful read and return it directly on later calls instead of
// prompting again. This matters because sshtransport.Client.RunSudo
// re-authenticates over its own fresh -tt channel on *every* privileged
// call rather than relying on sudo's own cross-process credential cache
// (see sshtransport/client.go's SudoValidate doc comment for why that
// cache can't be trusted) - without this, a single `server inspect` or
// `diagnose` run, which makes several privileged calls, would ask the
// user to retype their sudo password most of a dozen times. The cache is
// scoped to one Prompts value, i.e. one command invocation / one server
// connection - it does not outlive the process (SPEC.md §10.3: "held only
// in process memory for the session lifetime").
func newTerminalPrompts(out io.Writer) sshtransport.Prompts {
	in := bufio.NewReader(os.Stdin)
	var cachedPassword, cachedSudoPassword *string

	return sshtransport.Prompts{
		HostKey: func(prompt string) bool {
			fmt.Fprintln(out, strings.TrimSpace(prompt))
			fmt.Fprint(out, "Accept this host key and continue connecting? [y/N] ")
			line, _ := in.ReadString('\n')
			answer := strings.ToLower(strings.TrimSpace(line))
			return answer == "y" || answer == "yes"
		},
		Password: func() (string, error) {
			if cachedPassword != nil {
				return *cachedPassword, nil
			}
			pw, err := promptHiddenInput(out, "SSH password: ")
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
			pw, err := promptHiddenInput(out, "sudo password: ")
			if err != nil {
				return "", err
			}
			cachedSudoPassword = &pw
			return pw, nil
		},
	}
}

// promptHiddenInput prints prompt, reads one line from the controlling
// terminal with echo disabled, and returns it. Refuses to fall back to a
// plain read if stdin is not a terminal, since SPEC.md §10.3 forbids
// accepting a password any other way (argument, environment variable, or
// unprompted stdin that could be logged by a wrapping script).
func promptHiddenInput(out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("stdin is not a terminal; wgtui never accepts a password via argument, environment variable, or a non-interactive pipe")
	}
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
