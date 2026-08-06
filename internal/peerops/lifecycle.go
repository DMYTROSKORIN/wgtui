package peerops

import (
	"context"
	"fmt"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
	"github.com/DMYTROSKORIN/wgtui/internal/transaction"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

// AddResult bundles a successful add's transaction outcome with what the
// caller needs to build a client profile and persist local metadata - the
// private key in KeyPair never touched the server, only KeyPair.PublicKey
// did (SPEC.md §7.1, §6.3).
type AddResult struct {
	Outcome      transaction.Outcome
	Block        *wgconfig.Block
	KeyPair      wgconfig.KeyPair
	PresharedKey string // "" if not generated
}

// AddPeer generates a fresh client keypair (and, unless withPresharedKey
// is false, a preshared key - on by default per decision §30.8), builds a
// server-side peer block, and applies it as a new transaction.
func AddPeer(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, cc CurrentConfig, name string, allowedIPs []string, keepalive int, withPresharedKey bool) (AddResult, error) {
	kp, err := wgconfig.GenerateKeyPair()
	if err != nil {
		return AddResult{}, fmt.Errorf("generate client keypair: %w", err)
	}
	var psk string
	if withPresharedKey {
		psk, err = wgconfig.GeneratePresharedKey()
		if err != nil {
			return AddResult{}, fmt.Errorf("generate preshared key: %w", err)
		}
	}

	blk, err := wgconfig.BuildPeerBlock(wgconfig.NewPeerSpec{
		Name: name, PublicKey: kp.PublicKey, PresharedKey: psk,
		AllowedIPs: allowedIPs, PersistentKeepalive: keepalive,
	})
	if err != nil {
		return AddResult{}, err
	}

	cc.Doc.AppendPeerBlock(blk)
	plan := transaction.Plan{
		Interface:           cc.Interface,
		ConfigPath:          cc.ConfigPath,
		ExpectedFingerprint: cc.Fingerprint,
		NewContent:          cc.Doc.String(),
		Description:         fmt.Sprintf("+ peer %s, allowed_ips=%v", name, allowedIPs),
	}
	outcome, err := transaction.Apply(ctx, client, plan, prompts)
	if err != nil {
		return AddResult{}, err
	}
	return AddResult{Outcome: outcome, Block: blk, KeyPair: kp, PresharedKey: psk}, nil
}

// DisablePeer removes the peer block matching publicKey and applies the
// change (SPEC.md §17.3: gone from runtime and persistent config, local
// profile and metadata untouched). Returns the exact removed block text
// (HeaderRaw+Body) so the caller can store it for a later EnablePeer.
func DisablePeer(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, cc CurrentConfig, publicKey string) (transaction.Outcome, string, error) {
	idx := cc.Doc.FindPeerByPublicKey(publicKey)
	if idx == -1 {
		return transaction.Outcome{}, "", fmt.Errorf("no peer with public key %s in %s", publicKey, cc.ConfigPath)
	}
	removed := cc.Doc.RemoveBlockAt(idx)
	raw := removed.HeaderRaw + removed.Body

	plan := transaction.Plan{
		Interface:           cc.Interface,
		ConfigPath:          cc.ConfigPath,
		ExpectedFingerprint: cc.Fingerprint,
		NewContent:          cc.Doc.String(),
		Description:         "- peer " + shortKey(publicKey) + " (disabled, kept locally for resume)",
	}
	outcome, err := transaction.Apply(ctx, client, plan, prompts)
	return outcome, raw, err
}

// EnablePeer re-appends a block previously removed by DisablePeer,
// refusing if a peer with that public key is already present (SPEC.md
// §17.4: "проверить, что VPN-адрес и public key не заняты").
func EnablePeer(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, cc CurrentConfig, disabledBlockRaw string) (transaction.Outcome, error) {
	reparsed := wgconfig.Parse(disabledBlockRaw)
	if len(reparsed.Blocks) != 1 || reparsed.Blocks[0].Kind != "peer" {
		return transaction.Outcome{}, fmt.Errorf("stored disabled-peer data is not a single valid [Peer] block")
	}
	blk := reparsed.Blocks[0]
	pubKey := blk.PublicKey()
	if pubKey == "" {
		return transaction.Outcome{}, fmt.Errorf("stored disabled-peer block has no public key")
	}
	if cc.Doc.FindPeerByPublicKey(pubKey) != -1 {
		return transaction.Outcome{}, fmt.Errorf("a peer with public key %s is already present - not resuming", pubKey)
	}

	cc.Doc.AppendPeerBlock(blk)
	plan := transaction.Plan{
		Interface:           cc.Interface,
		ConfigPath:          cc.ConfigPath,
		ExpectedFingerprint: cc.Fingerprint,
		NewContent:          cc.Doc.String(),
		Description:         "+ peer " + shortKey(pubKey) + " (resumed)",
	}
	return transaction.Apply(ctx, client, plan, prompts)
}

// RevokePeer removes the peer block matching publicKey and applies the
// change - the server-access half of SPEC.md §17.5's three separate
// confirmations. Deleting the local profile and/or metadata is the
// caller's separate decision; RevokePeer never touches either.
func RevokePeer(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, cc CurrentConfig, publicKey string) (transaction.Outcome, error) {
	idx := cc.Doc.FindPeerByPublicKey(publicKey)
	if idx == -1 {
		return transaction.Outcome{}, fmt.Errorf("no peer with public key %s in %s", publicKey, cc.ConfigPath)
	}
	cc.Doc.RemoveBlockAt(idx)
	plan := transaction.Plan{
		Interface:           cc.Interface,
		ConfigPath:          cc.ConfigPath,
		ExpectedFingerprint: cc.Fingerprint,
		NewContent:          cc.Doc.String(),
		Description:         "- peer " + shortKey(publicKey) + " (revoked)",
	}
	return transaction.Apply(ctx, client, plan, prompts)
}

// RotateResult mirrors AddResult for a rotated peer's new identity.
type RotateResult struct {
	Outcome      transaction.Outcome
	NewBlock     *wgconfig.Block
	KeyPair      wgconfig.KeyPair
	PresharedKey string
}

// RotatePeer replaces the peer matching oldPublicKey with a freshly
// generated keypair in one transaction (SPEC.md §17.6): the old key is
// removed and the new one added atomically, not as two separate
// mutations that could leave the peer entirely absent from the server if
// the process were interrupted between them.
func RotatePeer(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, cc CurrentConfig, oldPublicKey, name string, allowedIPs []string, keepalive int, withPresharedKey bool) (RotateResult, error) {
	idx := cc.Doc.FindPeerByPublicKey(oldPublicKey)
	if idx == -1 {
		return RotateResult{}, fmt.Errorf("no peer with public key %s in %s", oldPublicKey, cc.ConfigPath)
	}

	kp, err := wgconfig.GenerateKeyPair()
	if err != nil {
		return RotateResult{}, fmt.Errorf("generate client keypair: %w", err)
	}
	var psk string
	if withPresharedKey {
		psk, err = wgconfig.GeneratePresharedKey()
		if err != nil {
			return RotateResult{}, fmt.Errorf("generate preshared key: %w", err)
		}
	}
	newBlk, err := wgconfig.BuildPeerBlock(wgconfig.NewPeerSpec{
		Name: name, PublicKey: kp.PublicKey, PresharedKey: psk,
		AllowedIPs: allowedIPs, PersistentKeepalive: keepalive,
	})
	if err != nil {
		return RotateResult{}, err
	}

	cc.Doc.RemoveBlockAt(idx)
	cc.Doc.AppendPeerBlock(newBlk)

	plan := transaction.Plan{
		Interface:           cc.Interface,
		ConfigPath:          cc.ConfigPath,
		ExpectedFingerprint: cc.Fingerprint,
		NewContent:          cc.Doc.String(),
		Description:         fmt.Sprintf("rotate peer %s: %s -> %s", name, shortKey(oldPublicKey), shortKey(kp.PublicKey)),
	}
	outcome, err := transaction.Apply(ctx, client, plan, prompts)
	if err != nil {
		return RotateResult{}, err
	}
	return RotateResult{Outcome: outcome, NewBlock: newBlk, KeyPair: kp, PresharedKey: psk}, nil
}

func shortKey(k string) string {
	if len(k) <= 12 {
		return k
	}
	return k[:8] + "…"
}
