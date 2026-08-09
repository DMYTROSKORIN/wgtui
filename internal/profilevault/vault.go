package profilevault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

// ErrWrongPassword is returned by Open when decryption fails - AES-GCM's
// authentication tag check doubles as the password check, so there is no
// separate stored password hash to attack offline any harder than the
// ciphertext itself already allows.
var ErrWrongPassword = errors.New("profilevault: wrong master password or corrupted vault")

// ErrNoVault is returned by Open when no vault file exists yet - the
// caller should run the first-run "set a master password" flow (SPEC.md
// §30.4) instead.
var ErrNoVault = errors.New("profilevault: no vault exists yet")

const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	argonKeyLen  = 32 // AES-256
	saltLen      = 16
)

// currentVaultVersion is the format version Save writes. Version 2 binds
// the KDF parameters and salt into the AES-GCM additional authenticated
// data (see vaultAAD); version 1 files (written before this) authenticated
// only the ciphertext and are still read transparently - they upgrade to 2
// on the next Save.
const currentVaultVersion = 2

// vaultAAD builds the GCM additional authenticated data that binds the KDF
// header fields (version, Argon2 cost parameters, salt) to the ciphertext.
// These fields live in the file's plaintext header - outside the ciphertext
// - so without this an attacker with write access could alter them (e.g.
// downgrade the Argon2 cost a future re-encrypt would use) without
// invalidating the authentication tag. Feeding them as AAD makes any such
// edit fail decryption instead. Version 1 vaults were sealed with no AAD;
// Open passes nil for them so they still open, and the next Save rewrites
// them as version 2.
func vaultAAD(version int, salt []byte, argonT, argonMemKB uint32, argonLanes uint8) []byte {
	b := make([]byte, 0, 12+4+4+4+1+len(salt))
	b = append(b, []byte("wgtui-vault\x00")...)
	b = binary.BigEndian.AppendUint32(b, uint32(version))
	b = binary.BigEndian.AppendUint32(b, argonT)
	b = binary.BigEndian.AppendUint32(b, argonMemKB)
	b = append(b, argonLanes)
	b = append(b, salt...)
	return b
}

// Registry is the vault's plaintext payload once decrypted.
type Registry struct {
	Servers []domain.Server `json:"servers"`
	// Peers is the local metadata registry from SPEC.md §16.1: it links
	// server-id+public-key to a name/notes/status, since WireGuard itself
	// has no such concept.
	Peers []PeerMetadata `json:"peers"`
	// Profiles holds generated client .conf text - including each
	// profile's private key - keyed by profileKey(serverID, publicKey).
	// This is the SPEC.md §7.3 local profile vault: rather than separate
	// plaintext files under a profiles/ directory, profile content lives
	// inside this same encrypted blob, consistent with decision §30.4
	// (master-password-encrypted local storage).
	Profiles map[string]string `json:"profiles"`
}

// PeerMetadata is one entry in the local peer registry (SPEC.md §16.1).
type PeerMetadata struct {
	ServerID  string    `json:"server_id"`
	Name      string    `json:"name"`
	PublicKey string    `json:"public_key"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	// Interface is the WireGuard interface this peer belongs to (e.g.
	// "wg0"). Required on a multi-interface server (Stage 4) so
	// disable/enable/revoke/rotate know which interface's config to look
	// in without having to guess via peerops.ResolveInterface's
	// only-works-with-exactly-one-active-interface fallback. "" for
	// entries created before this field existed (single-interface
	// servers keep working via that same fallback).
	Interface string `json:"interface,omitempty"`
	// Status is wgtui's own bookkeeping - "active", "disabled", or
	// "revoked" - not a WireGuard concept.
	Status string `json:"status"`
	// DisabledBlockRaw holds the exact original wgconfig.Block text (as
	// "HeaderRaw+Body") of a disabled peer's persistent-config entry, so
	// a later `peer enable` (a separate process invocation) can restore
	// the identical bytes rather than reconstructing an approximation
	// (SPEC.md §17.4: "вернуть тот же peer").
	DisabledBlockRaw string `json:"disabled_block_raw,omitempty"`
	// LocalProfilePath is set only if the profile was also exported to
	// an external file (SPEC.md §7.1); "" means it only lives in this
	// vault's Profiles map.
	LocalProfilePath string `json:"local_profile_path,omitempty"`
}

const (
	PeerStatusActive   = "active"
	PeerStatusDisabled = "disabled"
	PeerStatusRevoked  = "revoked"
)

// Vault is an unlocked, in-memory view of the encrypted registry. It is
// only ever constructed via New (first run) or Open (subsequent runs) so
// that a Vault value can never exist without having been through a
// password check.
type Vault struct {
	Registry Registry

	salt   []byte
	master string // held only in memory for this process's lifetime
}

// New creates a fresh, empty, unsaved vault protected by masterPassword.
// Call Save to persist it. Used on first run (SPEC.md §30.4).
func New(masterPassword string) (*Vault, error) {
	if masterPassword == "" {
		return nil, errors.New("profilevault: master password must not be empty")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return &Vault{salt: salt, master: masterPassword}, nil
}

// Exists reports whether a vault file is already present at path.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type vaultFile struct {
	Version    int    `json:"version"`
	Salt       []byte `json:"salt"`
	ArgonTime  uint32 `json:"argon_time"`
	ArgonMemKB uint32 `json:"argon_memory_kb"`
	ArgonLanes uint8  `json:"argon_lanes"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// Open decrypts the vault at path with masterPassword. Returns
// ErrWrongPassword if the password is incorrect (or the file is
// corrupted) and ErrNoVault if path does not exist.
func Open(path, masterPassword string) (*Vault, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoVault
	}
	if err != nil {
		return nil, fmt.Errorf("read vault: %w", err)
	}

	var vf vaultFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		return nil, fmt.Errorf("parse vault: %w", err)
	}

	key := argon2.IDKey([]byte(masterPassword), vf.Salt, vf.ArgonTime, vf.ArgonMemKB, vf.ArgonLanes, argonKeyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init GCM: %w", err)
	}
	var aad []byte
	if vf.Version >= 2 {
		aad = vaultAAD(vf.Version, vf.Salt, vf.ArgonTime, vf.ArgonMemKB, vf.ArgonLanes)
	}
	plaintext, err := gcm.Open(nil, vf.Nonce, vf.Ciphertext, aad)
	if err != nil {
		return nil, ErrWrongPassword
	}

	var reg Registry
	if err := json.Unmarshal(plaintext, &reg); err != nil {
		return nil, fmt.Errorf("parse decrypted registry: %w", err)
	}

	return &Vault{Registry: reg, salt: vf.Salt, master: masterPassword}, nil
}

// Save encrypts the current in-memory Registry and atomically writes it to
// path (0600, containing directory 0700 - SPEC.md §7.3).
func (v *Vault) Save(path string) error {
	if _, err := ensureDataDir(); err != nil {
		return err
	}

	key := argon2.IDKey([]byte(v.master), v.salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("init cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("init GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	plaintext, err := json.Marshal(v.Registry)
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	aad := vaultAAD(currentVaultVersion, v.salt, argonTime, argonMemory, argonThreads)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	vf := vaultFile{
		Version:    currentVaultVersion,
		Salt:       v.salt,
		ArgonTime:  argonTime,
		ArgonMemKB: argonMemory,
		ArgonLanes: argonThreads,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}
	out, err := json.MarshalIndent(vf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal vault file: %w", err)
	}
	return atomicWriteFile(path, out, 0o600)
}

// ChangePassword verifies the current password against the vault on disk,
// then re-encrypts the in-memory registry with a fresh salt and the new
// password. Save is atomic; on any failure this Vault is restored to its
// previous key material and the existing file remains usable.
func (v *Vault) ChangePassword(path, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("profilevault: new master password must be at least 8 characters")
	}
	if _, err := Open(path, currentPassword); err != nil {
		if errors.Is(err, ErrWrongPassword) {
			return ErrWrongPassword
		}
		return fmt.Errorf("verify current password: %w", err)
	}

	newSalt := make([]byte, saltLen)
	if _, err := rand.Read(newSalt); err != nil {
		return fmt.Errorf("generate new salt: %w", err)
	}
	oldMaster, oldSalt := v.master, v.salt
	v.master, v.salt = newPassword, newSalt
	if err := v.Save(path); err != nil {
		v.master, v.salt = oldMaster, oldSalt
		return fmt.Errorf("save re-encrypted vault: %w", err)
	}
	return nil
}
