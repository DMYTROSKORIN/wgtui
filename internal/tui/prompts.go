package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// newTerminalPrompts mirrors internal/cli's prompts.go. It is duplicated
// rather than imported: cli and tui are independent front ends over the
// same lower layers (docs/ARCHITECTURE.md) and neither should depend on
// the other just to share ~20 lines of terminal I/O. Both call sites must
// only be used after the tea.Program has released the terminal (see
// run.go) - Bubble Tea otherwise owns stdin in raw mode.
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
