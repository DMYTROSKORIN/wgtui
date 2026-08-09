package profilevault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

// argonIDKeyForTest derives a key the same way the vault does, for building
// a synthetic legacy (version-1) vault in tests.
func argonIDKeyForTest(t *testing.T, password string, salt []byte) []byte {
	t.Helper()
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
}

// sealForTest AES-GCM-seals plaintext with key and the given AAD (nil for a
// legacy version-1 vault), returning a fresh nonce and the ciphertext.
func sealForTest(t *testing.T, key, plaintext, aad []byte) (nonce, ciphertext []byte) {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, aad)
}

func TestVault_CreateSaveOpenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")

	v, err := New("correct horse battery staple")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := v.AddServer(domain.Server{ID: "office308", DisplayName: "Office 308"}); err != nil {
		t.Fatalf("AddServer: %v", err)
	}
	if err := v.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	opened, err := Open(path, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Open with correct password: %v", err)
	}
	if len(opened.Registry.Servers) != 1 || opened.Registry.Servers[0].ID != "office308" {
		t.Fatalf("unexpected registry after round trip: %+v", opened.Registry)
	}
}

func TestVault_WrongPassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")

	v, _ := New("right-password")
	if err := v.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := Open(path, "wrong-password")
	if err == nil {
		t.Fatal("expected an error opening with the wrong password, got nil")
	}
	if err != ErrWrongPassword {
		t.Errorf("got error %v, want ErrWrongPassword", err)
	}
}

func TestVault_OpenMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Open(filepath.Join(dir, "does-not-exist.json"), "anything")
	if err != ErrNoVault {
		t.Errorf("got error %v, want ErrNoVault", err)
	}
}

func TestVault_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	v, _ := New("pw")
	if err := v.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("vault file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestVault_PlaintextNeverOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	v, _ := New("pw")
	const secretMarker = "SERVER_ID_THAT_MUST_BE_ENCRYPTED"
	if err := v.AddServer(domain.Server{ID: secretMarker}); err != nil {
		t.Fatalf("AddServer: %v", err)
	}
	if err := v.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(rawBytes), secretMarker) {
		t.Fatalf("server ID appears in plaintext in the vault file on disk:\n%s", rawBytes)
	}
}

func TestVault_ChangePasswordReencryptsAndPreservesData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	v, _ := New("old-password")
	_ = v.AddServer(domain.Server{ID: "office"})
	if err := v.Save(path); err != nil {
		t.Fatal(err)
	}

	if err := v.ChangePassword(path, "old-password", "new-password"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := Open(path, "old-password"); err != ErrWrongPassword {
		t.Fatalf("old password error = %v, want ErrWrongPassword", err)
	}
	opened, err := Open(path, "new-password")
	if err != nil {
		t.Fatalf("open with new password: %v", err)
	}
	if _, ok := opened.GetServer("office"); !ok {
		t.Fatal("server data was lost during password change")
	}
}

func TestVault_ChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := New("old-password")
	if err := v.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePassword(path, "wrong-password", "new-password"); err != ErrWrongPassword {
		t.Fatalf("ChangePassword error = %v, want ErrWrongPassword", err)
	}
	if _, err := Open(path, "old-password"); err != nil {
		t.Fatalf("original vault was damaged: %v", err)
	}
}

func TestVault_AddServer_DuplicateID(t *testing.T) {
	v, _ := New("pw")
	if err := v.AddServer(domain.Server{ID: "a"}); err != nil {
		t.Fatalf("AddServer: %v", err)
	}
	err := v.AddServer(domain.Server{ID: "a"})
	if err == nil {
		t.Fatal("expected duplicate ID error, got nil")
	}
}

func TestVault_RemoveServer(t *testing.T) {
	v, _ := New("pw")
	_ = v.AddServer(domain.Server{ID: "a"})
	_ = v.AddServer(domain.Server{ID: "b"})
	if !v.RemoveServer("a") {
		t.Fatal("RemoveServer(a) = false, want true")
	}
	if _, ok := v.GetServer("a"); ok {
		t.Fatal("server a still present after RemoveServer")
	}
	if _, ok := v.GetServer("b"); !ok {
		t.Fatal("server b should still be present")
	}
	if v.RemoveServer("does-not-exist") {
		t.Fatal("RemoveServer on unknown ID = true, want false")
	}
}

// TestVault_WritesVersion2WithBoundParams guards F4: Save must write the
// version-2 format that binds the Argon2 parameters into the GCM AAD.
func TestVault_WritesVersion2WithBoundParams(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := New("a-master-password")
	if err := v.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, _ := os.ReadFile(path)
	var vf vaultFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if vf.Version != 2 {
		t.Fatalf("vault version = %d, want 2", vf.Version)
	}
}

// TestVault_TamperedArgonParamsFailToOpen guards F4: editing the KDF
// parameters in the (plaintext) header of a version-2 vault must break
// authentication rather than silently changing how a future re-encrypt
// derives its key.
func TestVault_TamperedArgonParamsFailToOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := New("a-master-password")
	if err := v.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, _ := os.ReadFile(path)
	var vf vaultFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	vf.ArgonMemKB = 8 // downgrade the cost parameter in the header
	tampered, _ := json.Marshal(vf)
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, "a-master-password"); err == nil {
		t.Fatal("Open accepted a vault whose Argon2 parameters were tampered with")
	}
}

// TestVault_LegacyVersion1StillOpens guards F4 backward compatibility: a
// version-1 vault (sealed with no AAD) must still open with the right
// password, and re-saving must upgrade it to version 2.
func TestVault_LegacyVersion1StillOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")

	// Hand-build a version-1 vault the old way: seal with nil AAD.
	salt := make([]byte, saltLen)
	key := argonIDKeyForTest(t, "legacy-pass", salt)
	nonce, ciphertext := sealForTest(t, key, []byte(`{"servers":[{"id":"old"}]}`), nil)
	vf := vaultFile{Version: 1, Salt: salt, ArgonTime: argonTime, ArgonMemKB: argonMemory, ArgonLanes: argonThreads, Nonce: nonce, Ciphertext: ciphertext}
	out, _ := json.MarshalIndent(vf, "", "  ")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}

	opened, err := Open(path, "legacy-pass")
	if err != nil {
		t.Fatalf("Open legacy v1 vault: %v", err)
	}
	if len(opened.Registry.Servers) != 1 || opened.Registry.Servers[0].ID != "old" {
		t.Fatalf("legacy registry not read back: %+v", opened.Registry)
	}
	if err := opened.Save(path); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	raw, _ := os.ReadFile(path)
	var reread vaultFile
	_ = json.Unmarshal(raw, &reread)
	if reread.Version != 2 {
		t.Fatalf("re-saved legacy vault stayed at version %d, want upgrade to 2", reread.Version)
	}
	if _, err := Open(path, "legacy-pass"); err != nil {
		t.Fatalf("Open after upgrade: %v", err)
	}
}
