package discovery

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

func checkErr(check string, res sshtransport.Result, err error) string {
	if err != nil {
		return fmt.Sprintf("%s: %v", check, err)
	}
	return fmt.Sprintf("%s: exited %d: %s", check, res.ExitCode, strings.TrimSpace(string(res.Stderr)))
}

func parseOSRelease(text string) (id, versionID string) {
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch strings.TrimSpace(key) {
		case "ID":
			id = val
		case "VERSION_ID":
			versionID = val
		}
	}
	return
}

func packageManagerFor(osID string) string {
	switch osID {
	case "ubuntu", "debian":
		return "apt"
	case "fedora":
		return "dnf"
	default:
		return ""
	}
}

// interfaceNamePattern matches a plausible Linux netdev name: IFNAMSIZ
// allows at most 15 usable characters, and real names never contain shell
// metacharacters. Anything remote-reported that fails this check is
// dropped rather than used to build a command string (SPEC.md §23.1: never
// build a remote command by concatenating unvalidated input, including
// input sourced from the remote server itself - see THREAT_MODEL.md T8).
var interfaceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,15}$`)

func isValidInterfaceName(name string) bool {
	return interfaceNamePattern.MatchString(name)
}
