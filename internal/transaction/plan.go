package transaction

import (
	"crypto/sha256"
	"encoding/hex"
)

// Plan is a fully-formed, client-computed mutation to one interface's
// persistent config, built entirely locally before anything is sent to
// the server so it can be shown to the user as a diff before Apply runs
// (SPEC.md §20.4 - "Будет выполнено... Применить? [y/N]").
type Plan struct {
	Interface  string
	ConfigPath string
	// ExpectedFingerprint is compared against the config's current
	// sha256 on the server immediately before writing (SPEC.md §15.2
	// step 3): if the file changed since this Plan was built, Apply
	// refuses rather than silently clobbering a concurrent edit.
	ExpectedFingerprint string
	NewContent          string
	// Description is a short, secret-free human-readable summary for
	// display (e.g. "+ peer Alex, allowed_ips=10.10.0.9/32") - Apply
	// itself never prints it; that's the caller's job before it decides
	// to call Apply at all.
	Description string
}

// Fingerprint computes the same sha256 hex digest the remote transaction
// script computes with `sha256sum`, so a Plan's ExpectedFingerprint can be
// set directly from a freshly-fetched config's raw text.
func Fingerprint(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
