package cli

import (
	"testing"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

func TestReconcilePeers_ManagedGetsRealName(t *testing.T) {
	now := time.Now()
	dump := []wgconfig.RuntimePeer{
		{Interface: "wg0", PublicKey: "pub1", AllowedIPs: []string{"10.10.0.2/32"}},
	}
	local := []profilevault.PeerMetadata{
		{PublicKey: "pub1", Name: "Alex", Status: profilevault.PeerStatusActive, Interface: "wg0"},
	}
	views := reconcilePeers(dump, local, func(string) bool { return true }, now)

	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	v := views[0]
	if v.Name != "Alex" || !v.Managed || !v.ProfileAvailable || v.Interface != "wg0" {
		t.Errorf("unexpected view: %+v", v)
	}
}

func TestReconcilePeers_UnmanagedGetsShortenedKey(t *testing.T) {
	now := time.Now()
	dump := []wgconfig.RuntimePeer{
		{Interface: "wg0", PublicKey: "cGVlcjF1bmtub3duZmFrZWZha2VmYWtlZmFrZQ=="},
	}
	views := reconcilePeers(dump, nil, func(string) bool { return false }, now)

	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	v := views[0]
	if v.Managed || v.ProfileAvailable {
		t.Errorf("unmanaged peer should have Managed=false, ProfileAvailable=false: %+v", v)
	}
	if v.Name != shortenKey("cGVlcjF1bmtub3duZmFrZWZha2VmYWtlZmFrZQ==") {
		t.Errorf("unmanaged peer Name = %q, want shortened key", v.Name)
	}
}

func TestReconcilePeers_DisabledPeerShownEvenWhenAbsentFromRuntime(t *testing.T) {
	now := time.Now()
	local := []profilevault.PeerMetadata{
		{PublicKey: "pub-disabled", Name: "Paused Alex", Status: profilevault.PeerStatusDisabled, Interface: "wg0"},
	}
	views := reconcilePeers(nil, local, func(string) bool { return true }, now)

	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	v := views[0]
	if v.Name != "Paused Alex" || v.State != "disabled" || !v.Managed {
		t.Errorf("unexpected disabled-peer view: %+v", v)
	}
}

func TestReconcilePeers_RevokedPeerNotResurrected(t *testing.T) {
	// A revoked-but-kept metadata entry (SPEC.md §17.5, delete-metadata=false)
	// must not appear in the list once it's off the server and not disabled -
	// only "disabled" status peers get the absent-from-runtime special case.
	now := time.Now()
	local := []profilevault.PeerMetadata{
		{PublicKey: "pub-revoked", Name: "Gone", Status: profilevault.PeerStatusRevoked},
	}
	views := reconcilePeers(nil, local, func(string) bool { return false }, now)
	if len(views) != 0 {
		t.Fatalf("got %d views, want 0 (revoked peer should not be resurrected): %+v", len(views), views)
	}
}

func TestReconcilePeers_ActivePeerNotDuplicatedByStaleMetadata(t *testing.T) {
	// If metadata is stale (still says "disabled" for a peer that's actually
	// back in the runtime dump - e.g. resumed by another tool/admin), the
	// runtime entry wins and there is no duplicate.
	now := time.Now()
	dump := []wgconfig.RuntimePeer{{Interface: "wg0", PublicKey: "pub1"}}
	local := []profilevault.PeerMetadata{
		{PublicKey: "pub1", Name: "Alex", Status: profilevault.PeerStatusDisabled},
	}
	views := reconcilePeers(dump, local, func(string) bool { return false }, now)
	if len(views) != 1 {
		t.Fatalf("got %d views, want exactly 1 (no duplicate), views: %+v", len(views), views)
	}
}
