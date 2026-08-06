package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// confirm shows plan (a secret-free diff/description, SPEC.md §20.4) and
// asks the user to type y to proceed. yesFlag lets --yes skip the prompt
// for automation (SPEC.md §21).
func confirm(out io.Writer, yesFlag bool, plan string) bool {
	fmt.Fprintln(out, "Will do:")
	fmt.Fprintln(out, plan)
	if yesFlag {
		return true
	}
	fmt.Fprint(out, "Apply? [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// confirmOptIn asks a secondary yes/no question that must default to "no"
// even under --yes (SPEC.md §17.5: revoking server access must never
// silently imply deleting the local profile too - each is its own
// explicit choice).
func confirmOptIn(out io.Writer, yesFlag bool, prompt string) bool {
	if yesFlag {
		return false
	}
	fmt.Fprint(out, prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
