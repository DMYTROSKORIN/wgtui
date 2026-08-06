package sshtransport

import (
	"strings"
	"testing"
)

// sampleSSHGOutput is a trimmed but realistic `ssh -G <alias>` transcript.
const sampleSSHGOutput = `user skorin
hostname nas.paynet.one
port 2626
addressfamily any
batchmode no
canonicalizehostname false
identityfile ~/.ssh/id_ed25519
identityfile ~/.ssh/id_rsa
`

func TestParseResolved(t *testing.T) {
	r, err := parseResolved(strings.NewReader(sampleSSHGOutput))
	if err != nil {
		t.Fatalf("parseResolved: %v", err)
	}
	if r.Hostname != "nas.paynet.one" {
		t.Errorf("Hostname = %q, want nas.paynet.one", r.Hostname)
	}
	if r.Port != 2626 {
		t.Errorf("Port = %d, want 2626", r.Port)
	}
	if r.User != "skorin" {
		t.Errorf("User = %q, want skorin", r.User)
	}
	if r.IdentityFile != "~/.ssh/id_ed25519" {
		t.Errorf("IdentityFile = %q, want the first identityfile line", r.IdentityFile)
	}
}

func TestParseResolved_EmptyInput(t *testing.T) {
	r, err := parseResolved(strings.NewReader(""))
	if err != nil {
		t.Fatalf("parseResolved(\"\"): %v", err)
	}
	if r != (Resolved{}) {
		t.Errorf("expected zero Resolved, got %+v", r)
	}
}

func TestResolveArgs_NeverIncludesSecretLookingContent(t *testing.T) {
	// Target has no field capable of carrying a password or private key
	// (only a path via IdentityFile), so this is mostly a guard against a
	// future field being added carelessly: if resolveArgs ever grows a
	// branch that stringifies something secret-shaped, this test's
	// intent is to be the place that catches it.
	tgt := Target{HostOrAlias: "nas.paynet.one", Port: 2626, User: "skorin"}
	args := resolveArgs(tgt)
	joined := strings.Join(args, " ")
	for _, forbidden := range []string{"password", "Password", "PRIVATE KEY"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("resolveArgs output unexpectedly contains %q: %v", forbidden, args)
		}
	}
	if got, want := args[len(args)-1], "nas.paynet.one"; got != want {
		t.Errorf("last arg = %q, want %q", got, want)
	}
}
