package profilevault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"

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

// Registry is the vault's plaintext payload once decrypted. Stage 1 only
// stores the server list (no secrets beyond an identity file *path* -
// SPEC.md §6.3); the peer metadata registry and generated client profiles
// (SPEC.md §16.1, §7.3) are added here in Stage 2 without needing a format
// migration, since this whole struct is what gets encrypted as one blob.
type Registry struct {
	Servers []domain.Server `json:"servers"`
}

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
	plaintext, err := gcm.Open(nil, vf.Nonce, vf.Ciphertext, nil)
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
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	vf := vaultFile{
		Version:    1,
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
