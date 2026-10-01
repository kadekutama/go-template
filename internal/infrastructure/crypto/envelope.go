package crypto

import (
	"context"
	"crypto/rand"
	"fmt"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// CryptoParams carries enveloper dependencies (Parameter Object pattern).
// The KEK is the only key handle: production wires OpenBao Transit
// (durable, server-side); tests supply a test-local double from *_test.go.
type CryptoParams struct {
	KEK KeyEncryptionKey
}

// Enveloper seals PII fields with per-envelope DEKs wrapped by the KEK.
type Enveloper struct {
	kek KeyEncryptionKey
}

// Compile-time port conformance.
var _ appport.EnvelopeCrypto = (*Enveloper)(nil)

// NewEnveloper builds the envelope adapter; KEK must be non-nil.
func NewEnveloper(params CryptoParams) (*Enveloper, error) {
	if params.KEK == nil {
		return nil, ErrConfigRequired
	}

	return &Enveloper{kek: params.KEK}, nil
}

// Encrypt seals plaintext for tenant+purpose with a fresh DEK + nonce.
func (e *Enveloper) Encrypt(ctx context.Context, tenant valueobject.TenantID, purpose string, plaintext []byte) (appport.EncryptedEnvelope, error) {
	if e == nil || e.kek == nil {
		return appport.EncryptedEnvelope{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return appport.EncryptedEnvelope{}, err
	}

	if tenant == "" || purpose == "" || len(plaintext) == 0 {
		return appport.EncryptedEnvelope{}, fmt.Errorf("%w: tenant, purpose and plaintext required", ErrConfigRequired)
	}

	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return appport.EncryptedEnvelope{}, err
	}

	wrapped, keyID, err := e.kek.WrapDEK(ctx, dek)
	if err != nil {
		return appport.EncryptedEnvelope{}, err
	}

	aad := envelopeAAD(string(tenant), purpose, keyID)

	nonce, sealed, err := sealWithKey(dek, plaintext, aad)
	if err != nil {
		return appport.EncryptedEnvelope{}, err
	}

	packed := packCiphertext(wrapped, purpose, sealed)

	return appport.EncryptedEnvelope{KeyID: keyID, Nonce: nonce, Ciphertext: packed}, nil
}

// Decrypt opens one envelope and fails closed on tamper/unknown/shred.
func (e *Enveloper) Decrypt(ctx context.Context, tenant valueobject.TenantID, envelope appport.EncryptedEnvelope) ([]byte, error) {
	if e == nil || e.kek == nil {
		return nil, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if envelope.KeyID == "" || len(envelope.Nonce) == 0 || len(envelope.Ciphertext) == 0 {
		return nil, ErrAuthFailed
	}

	wrapped, purpose, sealed, err := unpackCiphertext(envelope.Ciphertext)
	if err != nil {
		return nil, err
	}

	dek, err := e.kek.UnwrapDEK(ctx, envelope.KeyID, wrapped)
	if err != nil {
		return nil, err
	}

	strictAAD := envelopeAAD(string(tenant), purpose, envelope.KeyID)

	return openWithKey(dek, envelope.Nonce, sealed, strictAAD)
}

// Rewrap rotates the envelope's wrapped DEK under the latest KEK version without decrypting payload data.
func (e *Enveloper) Rewrap(ctx context.Context, envelope appport.EncryptedEnvelope) (appport.EncryptedEnvelope, error) {
	if e == nil || e.kek == nil {
		return appport.EncryptedEnvelope{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return appport.EncryptedEnvelope{}, err
	}

	if envelope.KeyID == "" || len(envelope.Nonce) == 0 || len(envelope.Ciphertext) == 0 {
		return appport.EncryptedEnvelope{}, ErrAuthFailed
	}

	wrapped, purpose, sealed, err := unpackCiphertext(envelope.Ciphertext)
	if err != nil {
		return appport.EncryptedEnvelope{}, err
	}

	newWrapped, err := e.kek.RewrapDEK(ctx, envelope.KeyID, wrapped)
	if err != nil {
		return appport.EncryptedEnvelope{}, err
	}

	newPacked := packCiphertext(newWrapped, purpose, sealed)

	return appport.EncryptedEnvelope{
		KeyID:      envelope.KeyID,
		Nonce:      envelope.Nonce,
		Ciphertext: newPacked,
	}, nil
}

// envelopeAAD binds tenant + purpose + key version.
func envelopeAAD(tenant, purpose, keyID string) []byte {
	return []byte(tenant + "\x00" + purpose + "\x00" + keyID)
}

// packedEnvelope is the JSON framing for wrapped DEK + purpose + data.
// JSON framing avoids manual length-prefix arithmetic entirely.
type packedEnvelope struct {
	Wrapped []byte `json:"wrapped"`
	Purpose string `json:"purpose"`
	Data    []byte `json:"data"`
}

// packCiphertext frames wrapped DEK + purpose + sealed data.
func packCiphertext(wrapped []byte, purpose string, sealed []byte) []byte {
	raw, err := jsonparser.Marshal(packedEnvelope{Wrapped: wrapped, Purpose: purpose, Data: sealed})
	if err != nil {
		return nil
	}

	return raw
}

// unpackCiphertext parses the packed envelope framing.
func unpackCiphertext(packed []byte) (wrapped []byte, purpose string, sealed []byte, err error) {
	var frame packedEnvelope
	if uerr := jsonparser.Unmarshal(packed, &frame); uerr != nil {
		return nil, "", nil, ErrAuthFailed
	}

	if len(frame.Wrapped) == 0 || frame.Purpose == "" || len(frame.Data) == 0 {
		return nil, "", nil, ErrAuthFailed
	}

	return frame.Wrapped, frame.Purpose, frame.Data, nil
}
