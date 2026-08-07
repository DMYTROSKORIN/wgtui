package profilevault

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir returns the directory wgtui stores its encrypted vault in,
// under the home directory of the user who ran
// wgtui (SPEC.md §7.3, decision §30.3/§30.5 - no server-side metadata
// file, no OS keychain, everything local to this user's profile).
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".wgtui-config"), nil
}

// VaultPath returns the path to the encrypted vault file within DataDir.
func VaultPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "vault.json")
	if Exists(path) {
		return path, nil
	}
	if err := migrateLegacyVault(path); err != nil {
		return "", err
	}
	return path, nil
}

func migrateLegacyVault(newPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	candidates := []string{
		filepath.Join(home, ".local", "share", "wgtui", "vault.json"),
		filepath.Join(home, "Library", "Application Support", "wgtui", "vault.json"),
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		candidates = append([]string{filepath.Join(xdg, "wgtui", "vault.json")}, candidates...)
	}
	for _, oldPath := range candidates {
		if oldPath == newPath || !Exists(oldPath) {
			continue
		}
		raw, err := os.ReadFile(oldPath)
		if err != nil {
			return fmt.Errorf("read legacy vault %s: %w", oldPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
			return fmt.Errorf("create portable data directory: %w", err)
		}
		if err := atomicWriteFile(newPath, raw, 0o600); err != nil {
			return fmt.Errorf("migrate vault to %s: %w", newPath, err)
		}
		if err := os.Remove(oldPath); err != nil {
			_ = os.Remove(newPath)
			return fmt.Errorf("remove legacy vault after migration: %w", err)
		}
		return nil
	}
	return nil
}

// ensureDataDir creates DataDir with 0700 permissions if it does not
// already exist, and tightens the mode if it does but is looser than 0700.
func ensureDataDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create data directory %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("set permissions on %s: %w", dir, err)
	}
	return dir, nil
}
