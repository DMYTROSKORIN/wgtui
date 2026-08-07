package tui

import (
	"errors"
	"fmt"
	"io"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

// openOrCreateVault mirrors internal/cli's vault.go (see prompts.go for
// why this is duplicated rather than shared). It runs before the Bubble
// Tea program starts, in plain terminal mode, so it can use the same
// blocking hidden-input prompt as the CLI.
func openOrCreateVault(out io.Writer) (*profilevault.Vault, string, error) {
	path, err := profilevault.VaultPath()
	if err != nil {
		return nil, "", err
	}
	if !profilevault.Exists(path) {
		return createVault(out, path)
	}
	return unlockVault(out, path)
}

func createVault(out io.Writer, path string) (*profilevault.Vault, string, error) {
	fmt.Fprintln(out, "No local vault found - set a master password to protect it.")
	fmt.Fprintln(out, "This protects your locally stored server list and any generated client private keys. It's never sent anywhere, and can't be recovered if lost - keep it somewhere safe.")
	for {
		pw1, err := promptHiddenInput(out, "New master password: ")
		if err != nil {
			return nil, "", err
		}
		if len(pw1) < 8 {
			fmt.Fprintln(out, "Master password must be at least 8 characters, try again.")
			continue
		}
		pw2, err := promptHiddenInput(out, "Confirm master password: ")
		if err != nil {
			return nil, "", err
		}
		if pw1 != pw2 {
			fmt.Fprintln(out, "Passwords did not match, try again.")
			continue
		}
		v, err := profilevault.New(pw1)
		if err != nil {
			return nil, "", err
		}
		if err := v.Save(path); err != nil {
			return nil, "", fmt.Errorf("save new vault: %w", err)
		}
		return v, path, nil
	}
}

func unlockVault(out io.Writer, path string) (*profilevault.Vault, string, error) {
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pw, err := promptHiddenInput(out, "Master password: ")
		if err != nil {
			return nil, "", err
		}
		v, err := profilevault.Open(path, pw)
		if err == nil {
			return v, path, nil
		}
		if !errors.Is(err, profilevault.ErrWrongPassword) {
			return nil, "", err
		}
		if attempt < maxAttempts {
			fmt.Fprintln(out, "Wrong password, try again.")
		}
	}
	return nil, "", fmt.Errorf("too many failed unlock attempts")
}
