package osadapter

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// Distro is the detected remote OS, just enough to pick an install/service
// adapter. Mirrors discovery.SystemInfo's OS fields rather than importing
// them: osadapter sits below discovery in the layer diagram
// (docs/ARCHITECTURE.md), and this parse is ~15 lines, not worth an
// import the wrong direction to save.
type Distro struct {
	ID             string // /etc/os-release ID, e.g. "ubuntu", "debian", "fedora"
	VersionID      string
	PackageManager string // "apt", "dnf", "" if unsupported
}

// Supported reports whether wgtui has an install adapter for d - MVP is
// Ubuntu/Debian/Fedora only (decision SPEC.md §30.6); everything else must
// get a clear read-only report, never a guessed mutation (§29.6).
func (d Distro) Supported() bool {
	return d.PackageManager != ""
}

// DetectDistro reads /etc/os-release over an already-Dialed client. No
// sudo needed - the file is world-readable.
func DetectDistro(ctx context.Context, client *sshtransport.Client) (Distro, error) {
	res, err := client.RunCommand(ctx, "cat /etc/os-release")
	if err != nil {
		return Distro{}, fmt.Errorf("read /etc/os-release: %w", err)
	}
	if res.ExitCode != 0 {
		return Distro{}, fmt.Errorf("read /etc/os-release exited %d: %s", res.ExitCode, res.Stderr)
	}
	id, versionID := parseOSRelease(string(res.Stdout))
	return Distro{ID: id, VersionID: versionID, PackageManager: packageManagerFor(id)}, nil
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
