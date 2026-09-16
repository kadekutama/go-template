package port

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// EncryptedEnvelope carries ciphertext with the key reference needed to open
// it. Nonces are random per message; key IDs select the data key, never the
// key material itself.
type EncryptedEnvelope struct {
	// KeyID selects the data key at the adapter.
	KeyID string
	// Nonce is the per-message random nonce.
	Nonce []byte
	// Ciphertext carries the encrypted bytes with authentication tag.
	Ciphertext []byte
}

// EnvelopeCrypto is the PII field-encryption boundary (E09 implements with
// envelope DEK/KEK discipline). Plaintext PII never reaches logs, errors, or
// the outbox; key rotation re-wraps data keys without touching callers.
type EnvelopeCrypto interface {
	// Encrypt seals plaintext for tenant+purpose context. Randomized nonce.
	Encrypt(ctx context.Context, tenant valueobject.TenantID, purpose string, plaintext []byte) (EncryptedEnvelope, error)
	// Decrypt opens one envelope. Fails closed on tamper or unknown key.
	Decrypt(ctx context.Context, tenant valueobject.TenantID, envelope EncryptedEnvelope) ([]byte, error)
}
