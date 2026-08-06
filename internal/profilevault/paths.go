package profilevault

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DataDir returns the directory wgtui stores its encrypted vault and
// generated profiles in, under the home directory of the user who ran
// wgtui (SPEC.md §7.3, decision §30.3/§30.5 - no server-side metadata
// file, no OS keychain, everything local to this user's profile).
func DataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "wgtui"), nil
	default: // linux and other unix-likes
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "wgtui"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".local", "share", "wgtui"), nil
	}
}

// VaultPath returns the path to the encrypted vault file within DataDir.
func VaultPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vault.json"), nil
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
