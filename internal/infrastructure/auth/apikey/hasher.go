package apikey

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// HasherParams tunes the Argon2id cost (RFC 9106). Defaults are the
// low-memory option; operators raise them to hardware. Changing params after
// hashes exist invalidates stored tags — rotate keys (Rotate re-hashes)
// after any change. See ADR-020.
type HasherParams struct {
	Time    uint32 `validate:"required,min=1,max=16"`
	Memory  uint32 `validate:"required,min=8192"`
	Threads uint8  `validate:"required,min=1,max=16"`
	KeyLen  uint32 `validate:"required,min=16,max=64"`
	SaltLen int    `validate:"required,min=8,max=64"`
}

// hashVersion tags every sealed hash with the params lineage it was sealed
// under. Version 1 is the initial lineage; future lineages add versions.
const hashVersion = 1

// Hasher hashes secrets with Argon2id and verifies in constant time.
// Hash and salt persist in separate BYTEA columns with a version tag — no
// PHC string is ever parsed, so parameter upgrades are a version switch,
// not string surgery.
type Hasher struct {
	activeVersion int
	params        HasherParams
	historical    map[int]HasherParams
}

// IsZero reports whether the Hasher is uninitialized.
func (h Hasher) IsZero() bool {
	return h.activeVersion == 0
}

// validateParams checks one HasherParams block.
func validateParams(params HasherParams) error {
	if params.Time == 0 || params.Time > 16 {
		return fmt.Errorf("%w: time 1..16", ErrConfigRequired)
	}

	if params.Memory < 8192 {
		return fmt.Errorf("%w: memory >= 8192 KiB", ErrConfigRequired)
	}

	if params.Threads == 0 || params.Threads > 16 {
		return fmt.Errorf("%w: threads 1..16", ErrConfigRequired)
	}

	if params.KeyLen < 16 || params.KeyLen > 64 {
		return fmt.Errorf("%w: key length 16..64", ErrConfigRequired)
	}

	if params.SaltLen < 8 || params.SaltLen > 64 {
		return fmt.Errorf("%w: salt length 8..64", ErrConfigRequired)
	}

	return nil
}

// NewHasher builds a Hasher with explicit cost parameters under version 1.
func NewHasher(params HasherParams) (Hasher, error) {
	return NewHasherWithHistory(hashVersion, params, nil)
}

// NewHasherWithHistory builds a Hasher supporting multi-version parameters for
// zero-downtime gradual migration. New hashes use activeVersion and activeParams;
// historical versions verify existing records and signal needsUpgrade = true.
func NewHasherWithHistory(activeVersion int, activeParams HasherParams, historical map[int]HasherParams) (Hasher, error) {
	if activeVersion <= 0 {
		return Hasher{}, fmt.Errorf("%w: active version must be positive", ErrConfigRequired)
	}

	if err := validateParams(activeParams); err != nil {
		return Hasher{}, err
	}

	hist := make(map[int]HasherParams, len(historical)+1)
	for v, p := range historical {
		if v <= 0 {
			return Hasher{}, fmt.Errorf("%w: historical version must be positive", ErrConfigRequired)
		}
		if err := validateParams(p); err != nil {
			return Hasher{}, fmt.Errorf("version %d: %w", v, err)
		}
		hist[v] = p
	}
	hist[activeVersion] = activeParams

	return Hasher{
		activeVersion: activeVersion,
		params:        activeParams,
		historical:    hist,
	}, nil
}

// Sealed is one stored hash: versioned salt + tag, straight from columns.
type Sealed struct {
	Hash    []byte
	Salt    []byte
	Version int
}

// Hash seals secret with a fresh random salt under active parameters.
func (h Hasher) Hash(secret string) (Sealed, error) {
	if secret == "" {
		return Sealed{}, fmt.Errorf("%w: empty secret", ErrInvalidFormat)
	}

	salt := make([]byte, h.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return Sealed{}, err
	}

	return Sealed{
		Hash:    argon2.IDKey([]byte(secret), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen),
		Salt:    salt,
		Version: h.activeVersion,
	}, nil
}

// Verify recomputes the tag with the stored salt and compares in constant
// time. Unknown versions fail closed so a future upgrade can never silently
// verify under wrong parameters.
func (h Hasher) Verify(sealed Sealed, secret string) bool {
	valid, _ := h.VerifyWithUpgrade(sealed, secret)
	return valid
}

// VerifyWithUpgrade recomputes the tag with the stored salt under the sealed
// version's parameters and compares in constant time. Unknown versions fail
// closed. If valid, needsUpgrade is true when the sealed version differs
// from the active version, signaling callers to transparently re-hash and
// persist the upgrade.
func (h Hasher) VerifyWithUpgrade(sealed Sealed, secret string) (valid bool, needsUpgrade bool) {
	p, ok := h.historical[sealed.Version]
	if !ok {
		return false, false
	}

	if len(sealed.Salt) != p.SaltLen || len(sealed.Hash) != int(p.KeyLen) {
		return false, false
	}

	candidate := argon2.IDKey([]byte(secret), sealed.Salt, p.Time, p.Memory, p.Threads, p.KeyLen)

	if subtle.ConstantTimeCompare(candidate, sealed.Hash) != 1 {
		return false, false
	}

	return true, sealed.Version != h.activeVersion
}
