// Package update implements the "update-available notification, no
// auto-update" requirement (spec §28, Stage 5): a best-effort check
// against GitHub Releases that only ever prints a notice. It never
// downloads or installs anything, and never runs unless the caller
// explicitly invokes Check (wired into `wgtui version` - see
// internal/cli/root.go - not into every command or the TUI's startup
// path, so wgtui never makes a network call the user didn't ask for).
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// DefaultReleasesURL is the GitHub API endpoint for this project's latest
// non-draft, non-prerelease release.
const DefaultReleasesURL = "https://api.github.com/repos/DMYTROSKORIN/wgtui/releases/latest"

// Info describes the result of a Check.
type Info struct {
	Current   string
	Latest    string
	URL       string
	Available bool
}

type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Checker looks up the latest release. The zero value uses
// DefaultReleasesURL and http.DefaultClient; tests substitute both.
type Checker struct {
	URL        string
	HTTPClient *http.Client
}

// Check compares currentVersion against the latest published release and
// reports whether a newer one exists. Any network or parse failure is
// returned as an error for the caller to silently ignore - a failed
// update check must never block or fail the command that triggered it.
func (c Checker) Check(ctx context.Context, currentVersion string) (Info, error) {
	url := c.URL
	if url == "" {
		url = DefaultReleasesURL
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Info{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "wgtui/"+currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("update check: unexpected status %s", resp.Status)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Info{}, fmt.Errorf("update check: decode response: %w", err)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	return Info{
		Current:   currentVersion,
		Latest:    rel.TagName,
		URL:       rel.HTMLURL,
		Available: compareVersions(current, latest) < 0,
	}, nil
}

// compareVersions compares two dotted major.minor.patch versions
// (an optional "-suffix" like "-dev" is ignored for the comparison).
// Returns -1 if a<b, 0 if equal, 1 if a>b. Not a full semver
// implementation - deliberately just enough to compare this project's
// own release tags, which are always plain vMAJOR.MINOR.PATCH.
func compareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionParts(v string) [3]int {
	v, _, _ = strings.Cut(v, "-")
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		if i >= 3 {
			break
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		out[i] = n
	}
	return out
}
