package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

// openOrCreateVault implements the SPEC.md §30.4 first-run flow: if no
// vault exists yet, prompt to set a master password (with confirmation);
// otherwise prompt to unlock the existing one, retrying a wrong password
// up to a few times rather than giving up after one typo.
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
	fmt.Fprintln(out, "This protects your local server list and, from Stage 2 on, generated client profiles. It is never sent anywhere and cannot be recovered if lost.")
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

// newControlDir creates a private temporary directory for one SSH
// ControlMaster socket. os.MkdirTemp uses mode 0700 on unix, matching the
// "only this user can reach the socket" requirement in
// sshtransport.NewClient's doc comment.
func newControlDir() (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "wgtui-ssh-*")
	if err != nil {
		return "", nil, fmt.Errorf("create control socket directory: %w", err)
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}
