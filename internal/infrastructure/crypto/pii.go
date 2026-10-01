package crypto

import (
	"context"
	"encoding/base64"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// EncryptPIIField seals one PII string into a portable base64 envelope token.
func (e *Enveloper) EncryptPIIField(ctx context.Context, tenant valueobject.TenantID, purpose, plaintext string) (string, error) {
	if e == nil || e.kek == nil {
		return "", ErrNotInitialized
	}

	envelope, err := e.Encrypt(ctx, tenant, purpose, []byte(plaintext))
	if err != nil {
		return "", err
	}

	raw, err := jsonparser.Marshal(envelope)
	if err != nil {
		return "", err
	}

	return base64.RawStdEncoding.EncodeToString(raw), nil
}

// DecryptPIIField opens one token produced by EncryptPIIField.
func (e *Enveloper) DecryptPIIField(ctx context.Context, tenant valueobject.TenantID, token string) (string, error) {
	if e == nil || e.kek == nil {
		return "", ErrNotInitialized
	}

	raw, err := base64.RawStdEncoding.DecodeString(token)
	if err != nil {
		return "", ErrAuthFailed
	}

	var envelope appport.EncryptedEnvelope
	if err := jsonparser.Unmarshal(raw, &envelope); err != nil {
		return "", ErrAuthFailed
	}

	plain, err := e.Decrypt(ctx, tenant, envelope)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}

// Call-site pattern (deliberately explicit, no reflection): each PII field
// is sealed where the struct is built, with the purpose naming the field:
//
//	sealedEmail, err := enveloper.EncryptPIIField(ctx, tenant, "user.email", user.Email)
//	if err != nil {
//	    return err
//	}
//	profile.Email = sealedEmail
//
// Reflection was evaluated and rejected for this path: a mistyped tag, an
// unexported field, or a new field added without a tag fails silently (the
// zero value encrypts or the plaintext leaks), and the walker is invisible
// to grep and static review. Explicit per-field calls keep every PII flow
// auditable at the call site.
