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
func newTerminalPrompts(out io.Writer) sshtransport.Prompts {
	in := bufio.NewReader(os.Stdin)
	return sshtransport.Prompts{
		HostKey: func(prompt string) bool {
			fmt.Fprintln(out, strings.TrimSpace(prompt))
			fmt.Fprint(out, "Accept this host key and continue connecting? [y/N] ")
			line, _ := in.ReadString('\n')
			answer := strings.ToLower(strings.TrimSpace(line))
			return answer == "y" || answer == "yes"
		},
		Password: func() (string, error) {
			return promptHiddenInput(out, "SSH password: ")
		},
		SudoPassword: func() (string, error) {
			return promptHiddenInput(out, "sudo password: ")
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
