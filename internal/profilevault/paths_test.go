package profilevault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDir_IsPortableDirectoryInHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".wgtui-config")
	if dir != want {
		t.Fatalf("DataDir = %q, want %q", dir, want)
	}
}

func TestVaultPath_MigratesLegacyVault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	oldPath := filepath.Join(home, ".local", "share", "wgtui", "vault.json")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("encrypted-vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	newPath, err := VaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if newPath != filepath.Join(home, ".wgtui-config", "vault.json") {
		t.Fatalf("VaultPath = %q", newPath)
	}
	if Exists(oldPath) {
		t.Fatal("legacy vault still exists after migration")
	}
	raw, err := os.ReadFile(newPath)
	if err != nil || string(raw) != "encrypted-vault" {
		t.Fatalf("migrated vault = %q, err=%v", raw, err)
	}
}

func TestVaultPath_MigratesLegacyMacOSVault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	oldPath := filepath.Join(home, "Library", "Application Support", "wgtui", "vault.json")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("encrypted-macos-vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	newPath, err := VaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if Exists(oldPath) {
		t.Fatal("legacy macOS vault still exists after migration")
	}
	raw, err := os.ReadFile(newPath)
	if err != nil || string(raw) != "encrypted-macos-vault" {
		t.Fatalf("migrated vault = %q, err=%v", raw, err)
	}
}
