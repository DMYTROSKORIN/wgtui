package profilevault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVault_PeerLifecycle(t *testing.T) {
	v, _ := New("pw")
	p := PeerMetadata{ServerID: "s1", Name: "Alex", PublicKey: "pub1", CreatedAt: time.Now(), Status: PeerStatusActive}
	if err := v.AddPeer(p); err != nil {
		t.Fatalf("AddPeer: %v", err)
	}
	if err := v.AddPeer(p); err == nil {
		t.Fatal("expected duplicate error on second AddPeer with same server+key")
	}

	got, ok := v.GetPeerByName("s1", "Alex")
	if !ok || got.PublicKey != "pub1" {
		t.Fatalf("GetPeerByName = %+v, ok=%v", got, ok)
	}
	got2, ok := v.GetPeerByPublicKey("s1", "pub1")
	if !ok || got2.Name != "Alex" {
		t.Fatalf("GetPeerByPublicKey = %+v, ok=%v", got2, ok)
	}

	if len(v.ListPeers("s1")) != 1 {
		t.Fatalf("ListPeers = %v", v.ListPeers("s1"))
	}
	if len(v.ListPeers("other-server")) != 0 {
		t.Fatalf("ListPeers for unrelated server should be empty")
	}

	got.Status = PeerStatusDisabled
	got.DisabledBlockRaw = "[Peer]\nPublicKey = pub1\n"
	if !v.UpdatePeer(got) {
		t.Fatal("UpdatePeer returned false")
	}
	updated, _ := v.GetPeerByPublicKey("s1", "pub1")
	if updated.Status != PeerStatusDisabled || updated.DisabledBlockRaw == "" {
		t.Fatalf("update did not persist: %+v", updated)
	}

	if !v.RemovePeer("s1", "pub1") {
		t.Fatal("RemovePeer returned false")
	}
	if _, ok := v.GetPeerByPublicKey("s1", "pub1"); ok {
		t.Fatal("peer still present after RemovePeer")
	}
	if v.RemovePeer("s1", "pub1") {
		t.Fatal("RemovePeer on an already-removed peer should return false")
	}
}

// TestVault_RotateMetadataSequence locks in the correct sequence for a
// key rotation: PeerMetadata is keyed by (ServerID, PublicKey), so
// UpdatePeer cannot express "this row's key is changing" - it looks up
// the row by the very key that's about to change, finds nothing, and a
// caller that isn't careful (cli's runPeerRotate did this once) can leave
// the old row behind as an orphan pointing at a public key that no longer
// exists on the server. The correct sequence is RemovePeer(old) then
// AddPeer(new).
func TestVault_RotateMetadataSequence(t *testing.T) {
	v, _ := New("pw")
	orig := PeerMetadata{ServerID: "s1", Name: "Alex", PublicKey: "old-key", CreatedAt: time.Now(), Status: PeerStatusActive}
	if err := v.AddPeer(orig); err != nil {
		t.Fatalf("AddPeer: %v", err)
	}

	// The fixed rotation sequence.
	v.RemovePeer("s1", orig.PublicKey)
	rotated := orig
	rotated.PublicKey = "new-key"
	if err := v.AddPeer(rotated); err != nil {
		t.Fatalf("AddPeer after rotate: %v", err)
	}

	peers := v.ListPeers("s1")
	if len(peers) != 1 {
		t.Fatalf("ListPeers = %+v, want exactly 1 entry after rotation (found a leftover orphan)", peers)
	}
	if peers[0].PublicKey != "new-key" {
		t.Errorf("surviving entry has PublicKey %q, want new-key", peers[0].PublicKey)
	}
	if _, ok := v.GetPeerByPublicKey("s1", "old-key"); ok {
		t.Error("old-key still resolvable after rotation - orphaned row")
	}
	if got, ok := v.GetPeerByName("s1", "Alex"); !ok || got.PublicKey != "new-key" {
		t.Errorf("GetPeerByName(Alex) = %+v, ok=%v, want the rotated (new-key) entry", got, ok)
	}
}

func TestVault_ProfileStorage(t *testing.T) {
	v, _ := New("pw")
	const conf = "[Interface]\nPrivateKey = super-secret-private-key\n"
	v.SetProfile("s1", "pub1", conf)

	got, ok := v.GetProfile("s1", "pub1")
	if !ok || got != conf {
		t.Fatalf("GetProfile = %q, ok=%v, want %q", got, ok, conf)
	}
	if _, ok := v.GetProfile("s1", "does-not-exist"); ok {
		t.Fatal("GetProfile found something for a key that was never set")
	}
	// Profiles are scoped per server: same public key, different server,
	// must not collide.
	v.SetProfile("s2", "pub1", "different content")
	got1, _ := v.GetProfile("s1", "pub1")
	if got1 != conf {
		t.Fatalf("cross-server profile collision: got %q", got1)
	}

	if !v.DeleteProfile("s1", "pub1") {
		t.Fatal("DeleteProfile returned false for an existing profile")
	}
	if _, ok := v.GetProfile("s1", "pub1"); ok {
		t.Fatal("profile still retrievable after DeleteProfile")
	}
	if v.DeleteProfile("s1", "pub1") {
		t.Fatal("DeleteProfile on an already-deleted profile should return false")
	}
}

func TestVault_ProfilesEncryptedOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	v, _ := New("pw")
	const secretKey = "MHNeverShouldAppearInPlaintextOnDiskXXXX0="
	v.SetProfile("s1", "pub1", "[Interface]\nPrivateKey = "+secretKey+"\n")
	if err := v.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vault file: %v", err)
	}
	if strings.Contains(string(raw), secretKey) {
		t.Fatalf("client private key appears in plaintext in the vault file on disk:\n%s", raw)
	}

	opened, err := Open(path, "pw")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, ok := opened.GetProfile("s1", "pub1")
	if !ok || !strings.Contains(got, secretKey) {
		t.Fatalf("profile did not round-trip through Save/Open: %q", got)
	}
}
