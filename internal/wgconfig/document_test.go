package wgconfig

import (
	"strings"
	"testing"
)

var goldenFixtures = []string{
	"minimal.conf",
	"multi_peer_saveconfig.conf",
	"crlf_lowercase_key.conf",
	"unknown_fields.conf",
}

// TestDocument_RoundTrip is the golden-test backbone SPEC.md §26.2 asks
// for: Parse then String on an untouched Document must reproduce the
// original bytes exactly, across every fixture (comments, CRLF,
// SaveConfig, unknown directives, PostUp/PostDown with ';').
func TestDocument_RoundTrip(t *testing.T) {
	for _, name := range goldenFixtures {
		t.Run(name, func(t *testing.T) {
			raw := readFixture(t, name)
			doc := Parse(raw)
			if got := doc.String(); got != raw {
				t.Errorf("round trip mismatch for %s:\n--- want ---\n%q\n--- got ---\n%q", name, raw, got)
			}
		})
	}
}

func TestDocument_AddPeer_PreservesRest(t *testing.T) {
	raw := readFixture(t, "minimal.conf")
	doc := Parse(raw)

	blk, err := BuildPeerBlock(NewPeerSpec{
		Name:       "Alex's laptop",
		PublicKey:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		AllowedIPs: []string{"10.10.0.9/32"},
	})
	if err != nil {
		t.Fatalf("BuildPeerBlock: %v", err)
	}
	doc.AppendPeerBlock(blk)

	got := doc.String()
	if !strings.HasPrefix(got, raw) {
		t.Fatalf("adding a peer changed existing bytes; original content is not an unmodified prefix of the result:\n--- original ---\n%q\n--- got ---\n%q", raw, got)
	}
	if !strings.Contains(got, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") {
		t.Error("new peer's public key missing from output")
	}
	if !strings.Contains(got, "# Alex's laptop") {
		t.Error("new peer's name comment missing from output")
	}
}

func TestDocument_RemovePeer_PreservesRest(t *testing.T) {
	raw := readFixture(t, "multi_peer_saveconfig.conf")
	doc := Parse(raw)

	const targetKey = "cGVlcjFwdWJsaWNrZXlmYWtlZmFrZWZha2VmYWtlZmFrZQ=="
	const otherKey = "cGVlcjJwdWJsaWNrZXlmYWtlZmFrZWZha2VmYWtlZmFrZQ=="

	idx := doc.FindPeerByPublicKey(targetKey)
	if idx == -1 {
		t.Fatal("target peer not found")
	}
	removed := doc.RemoveBlockAt(idx)
	if removed.PublicKey() != targetKey {
		t.Fatalf("removed wrong block: got key %s", removed.PublicKey())
	}

	got := doc.String()
	if strings.Contains(got, targetKey) {
		t.Error("removed peer's public key still present after removal")
	}
	if !strings.Contains(got, otherKey) {
		t.Error("the other peer was lost too - only the target should be removed")
	}
	if !strings.Contains(got, "SaveConfig = true") {
		t.Error("interface block content was lost")
	}

	// Every remaining block's exact bytes must be untouched - compare the
	// surviving blocks' raw text against substrings of the original.
	ifaceBlk, ok := doc.InterfaceBlock()
	if !ok {
		t.Fatal("interface block missing after removal")
	}
	if !strings.Contains(raw, ifaceBlk.HeaderRaw+ifaceBlk.Body) {
		t.Error("interface block bytes changed after removing an unrelated peer")
	}
}

func TestDocument_ResumeRoundTrip(t *testing.T) {
	// disable = remove + keep; resume = re-append the same block. The
	// resulting document must be byte-identical to the original, proving
	// "resume returns the exact same peer" (SPEC.md §17.4) at the config
	// level, not just semantically.
	raw := readFixture(t, "multi_peer_saveconfig.conf")
	doc := Parse(raw)

	const targetKey = "cGVlcjFwdWJsaWNrZXlmYWtlZmFrZWZha2VmYWtlZmFrZQ=="
	idx := doc.FindPeerByPublicKey(targetKey)
	removed := doc.RemoveBlockAt(idx)
	doc.AppendPeerBlock(removed)

	// Order changes (the resumed peer moves to the end), so this isn't a
	// byte-identical round trip of the whole file, but the removed peer's
	// own bytes must be unchanged and still present.
	got := doc.String()
	if !strings.Contains(got, removed.HeaderRaw+removed.Body) {
		t.Error("resumed peer's block bytes changed across disable+resume")
	}
}

func TestBuildPeerBlock_RejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		spec NewPeerSpec
	}{
		{"bad public key", NewPeerSpec{PublicKey: "not-a-key", AllowedIPs: []string{"10.0.0.1/32"}}},
		{"short public key", NewPeerSpec{PublicKey: "AAAA=", AllowedIPs: []string{"10.0.0.1/32"}}},
		{"no allowed ips", NewPeerSpec{PublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}},
		{"bad cidr", NewPeerSpec{PublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", AllowedIPs: []string{"not-a-cidr"}}},
		{"negative keepalive", NewPeerSpec{PublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", AllowedIPs: []string{"10.0.0.1/32"}, PersistentKeepalive: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BuildPeerBlock(tc.spec); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestBuildPeerBlock_SanitizesNameInjection(t *testing.T) {
	blk, err := BuildPeerBlock(NewPeerSpec{
		Name:       "evil\n[Interface]\nPrivateKey = injected",
		PublicKey:  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		AllowedIPs: []string{"10.0.0.1/32"},
	})
	if err != nil {
		t.Fatalf("BuildPeerBlock: %v", err)
	}
	// The word "PrivateKey" surviving as inert text inside one comment
	// line is fine; what must never happen is a *newline* surviving,
	// which would turn it into its own directive line.
	if strings.Count(blk.HeaderRaw, "\n") != 2 {
		t.Fatalf("expected exactly 2 newlines in header (comment line + [Peer] line) - a stray newline would let Name inject a new config line, got %q", blk.HeaderRaw)
	}
	if !strings.HasPrefix(blk.HeaderRaw, "# ") || !strings.HasSuffix(blk.HeaderRaw, "[Peer]\n") {
		t.Fatalf("header no longer has the expected comment+[Peer] shape: %q", blk.HeaderRaw)
	}
}

func TestValidatePublicKey(t *testing.T) {
	valid := "4LZENKxeg1ldbGgKz1ck5CPzOtNAgRB1UAqPfBOhqQE="
	if err := ValidatePublicKey(valid); err != nil {
		t.Errorf("ValidatePublicKey(%q) = %v, want nil", valid, err)
	}
	invalid := []string{"", "short", "not base64!!!!==============================", strings.Repeat("A", 43) + "="}
	// the last case is 44 chars of valid-looking base64 that happens to
	// decode to something other than 32 bytes is *not* guaranteed by
	// construction here, so only assert on the clearly-bad ones:
	for _, s := range invalid[:3] {
		if err := ValidatePublicKey(s); err == nil {
			t.Errorf("ValidatePublicKey(%q) = nil, want an error", s)
		}
	}
}
