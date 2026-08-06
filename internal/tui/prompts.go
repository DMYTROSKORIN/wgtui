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
