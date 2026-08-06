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
