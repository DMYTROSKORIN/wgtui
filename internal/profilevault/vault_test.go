package profilevault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

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
