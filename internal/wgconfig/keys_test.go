package wgconfig

import "testing"

func TestGenerateKeyPair_ValidAndDeterministicallyDerivable(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if err := ValidatePublicKey(kp.PublicKey); err != nil {
		t.Errorf("generated public key fails validation: %v", err)
	}
	if err := ValidatePublicKey(kp.PrivateKey); err != nil {
		t.Errorf("generated private key does not even have the right shape: %v", err)
	}

	derived, err := DerivePublicKey(kp.PrivateKey)
	if err != nil {
		t.Fatalf("DerivePublicKey: %v", err)
	}
	if derived != kp.PublicKey {
		t.Errorf("DerivePublicKey(priv) = %q, want %q (matching GenerateKeyPair's own public key)", derived, kp.PublicKey)
	}
}

func TestGenerateKeyPair_NotRepeated(t *testing.T) {
	a, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	b, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if a.PrivateKey == b.PrivateKey || a.PublicKey == b.PublicKey {
		t.Fatal("two calls to GenerateKeyPair produced the same key - broken randomness")
	}
}

func TestGeneratePresharedKey(t *testing.T) {
	a, err := GeneratePresharedKey()
	if err != nil {
		t.Fatalf("GeneratePresharedKey: %v", err)
	}
	if err := ValidatePublicKey(a); err != nil {
		t.Errorf("preshared key fails the same shape validation as a public key: %v", err)
	}
	b, _ := GeneratePresharedKey()
	if a == b {
		t.Fatal("two calls to GeneratePresharedKey produced the same value")
	}
}
