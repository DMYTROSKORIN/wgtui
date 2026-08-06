package wgconfig

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// KeyPair is a WireGuard keypair in the same base64 text form used in
// config files. GenerateKeyPair is the only place in this project that
// creates a *client* private key - SPEC.md §7.1/§6.3: it is generated
// locally and never sent to the server, only PublicKey is.
type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateKeyPair creates a new random Curve25519 keypair, clamped the
// same way `wg genkey` clamps it, so the stored private key bytes match
// what the reference implementation would produce.
func GenerateKeyPair() (KeyPair, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return KeyPair{}, fmt.Errorf("generate private key: %w", err)
	}
	// RFC 7748 / WireGuard clamping.
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return KeyPair{}, fmt.Errorf("derive public key: %w", err)
	}
	return KeyPair{
		PrivateKey: base64.StdEncoding.EncodeToString(priv[:]),
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
	}, nil
}

// DerivePublicKey computes the public key for an existing base64 private
// key - used by peer import (SPEC.md §17.7) to match an existing local
// .conf against a server-side peer without ever sending the private key
// anywhere.
func DerivePublicKey(privateKeyB64 string) (string, error) {
	priv, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	if len(priv) != 32 {
		return "", fmt.Errorf("private key decodes to %d bytes, want 32", len(priv))
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("derive public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}

// GeneratePresharedKey creates a new random preshared key (SPEC.md §30.8:
// on by default for new peers). Unlike a keypair this is symmetric - just
// 32 random bytes, no derivation.
func GeneratePresharedKey() (string, error) {
	var psk [32]byte
	if _, err := rand.Read(psk[:]); err != nil {
		return "", fmt.Errorf("generate preshared key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(psk[:]), nil
}
