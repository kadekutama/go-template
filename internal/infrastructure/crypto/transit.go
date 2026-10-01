package crypto

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// KeyEncryptionKey wraps data keys (KEK discipline). Master keys never leave
// the implementation: TransitKEK calls OpenBao Transit (AES-256-GCM) over
// HTTP in production, and tests supply a test-local double from *_test.go.
type KeyEncryptionKey interface {
	WrapDEK(ctx context.Context, dek []byte) (wrapped []byte, keyID string, err error)
	UnwrapDEK(ctx context.Context, keyID string, wrapped []byte) (dek []byte, err error)
	RewrapDEK(ctx context.Context, keyID string, wrapped []byte) (newWrapped []byte, err error)
	DestroyKey(ctx context.Context, keyID string) error
}

// TransitKEK calls OpenBao Transit Encryption-as-a-Service (wire-compatible
// with HashiCorp Vault Transit). Plaintext DEKs travel TLS only.
type TransitKEK struct {
	address string
	token   string
	keyName string
	http    httpclient.Doer
}

// TransitParams carries Transit KEK dependencies.
type TransitParams struct {
	Address string
	Token   string
	KeyName string
	HTTP    httpclient.Doer
}

// NewTransitKEK builds the OpenBao Transit KEK adapter.
func NewTransitKEK(params TransitParams) (*TransitKEK, error) {
	if params.Address == "" || params.Token == "" || params.KeyName == "" || params.HTTP == nil {
		return nil, ErrConfigRequired
	}

	return &TransitKEK{
		address: params.Address,
		token:   params.Token,
		keyName: params.KeyName,
		http:    params.HTTP,
	}, nil
}

// WrapDEK encrypts one DEK via Transit (base64 wire format).
func (k *TransitKEK) WrapDEK(ctx context.Context, dek []byte) ([]byte, string, error) {
	if k == nil || k.http == nil {
		return nil, "", ErrNotInitialized
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	payload, _ := jsonparser.Marshal(map[string]string{"plaintext": base64.StdEncoding.EncodeToString(dek)})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.address+"/v1/transit/encrypt/"+k.keyName, bytes.NewReader(payload))
	if err != nil {
		return nil, "", fmt.Errorf("crypto: transit wrap: %w", err)
	}

	req.Header.Set("X-Vault-Token", k.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := k.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("crypto: transit wrap: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%w: status %d", ErrAuthFailed, resp.StatusCode)
	}

	var out struct {
		Data struct {
			Ciphertext string `json:"ciphertext"`
		} `json:"data"`
	}

	if body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err != nil {
		return nil, "", fmt.Errorf("crypto: transit wrap body: %w", err)
	} else if err := jsonparser.Unmarshal(body, &out); err != nil {
		return nil, "", fmt.Errorf("crypto: transit wrap decode: %w", err)
	}

	return []byte(out.Data.Ciphertext), k.keyName, nil
}

// UnwrapDEK decrypts one Transit ciphertext back into the DEK.
// UnwrapDEK decrypts one Transit ciphertext back into the DEK. Version
// resolution is server-side: Transit ciphertexts embed their key version
// (vault:vN:…), so the engine opens the correct version regardless of the
// keyID argument. keyID selects the key NAME for multi-key deployments and
// is intentionally ignored here because WrapDEK stamps keyName as the
// envelope KeyID on single-key deployments (CR-011 mechanism record).
func (k *TransitKEK) UnwrapDEK(ctx context.Context, keyID string, wrapped []byte) ([]byte, error) {
	if k == nil || k.http == nil {
		return nil, ErrNotInitialized
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	payload, _ := jsonparser.Marshal(map[string]string{"ciphertext": string(wrapped)})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.address+"/v1/transit/decrypt/"+k.keyName, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("crypto: transit unwrap: %w", err)
	}

	req.Header.Set("X-Vault-Token", k.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := k.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crypto: transit unwrap: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrAuthFailed
	}

	var out struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}

	if body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err != nil {
		return nil, fmt.Errorf("crypto: transit unwrap body: %w", err)
	} else if err := jsonparser.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("crypto: transit unwrap decode: %w", err)
	}

	raw, err := base64.StdEncoding.DecodeString(out.Data.Plaintext)
	if err != nil {
		return nil, ErrAuthFailed
	}

	_ = keyID

	return raw, nil
}

// RewrapDEK re-encrypts one Transit ciphertext under the latest key version without returning plaintext.
// keyID selects the key name (defaults to the configured keyName); version
// selection within that key is server-side.
func (k *TransitKEK) RewrapDEK(ctx context.Context, keyID string, wrapped []byte) ([]byte, error) {
	if k == nil || k.http == nil {
		return nil, ErrNotInitialized
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	payload, err := jsonparser.Marshal(map[string]string{"ciphertext": string(wrapped)})
	if err != nil {
		return nil, fmt.Errorf("crypto: transit rewrap marshal: %w", err)
	}

	targetKey := k.keyName
	if keyID != "" {
		targetKey = keyID
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.address+"/v1/transit/rewrap/"+targetKey, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("crypto: transit rewrap: %w", err)
	}

	req.Header.Set("X-Vault-Token", k.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := k.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crypto: transit rewrap: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrAuthFailed
	}

	var out struct {
		Data struct {
			Ciphertext string `json:"ciphertext"`
		} `json:"data"`
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("crypto: transit rewrap body: %w", err)
	}
	if err := jsonparser.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("crypto: transit rewrap decode: %w", err)
	}

	return []byte(out.Data.Ciphertext), nil
}

// DestroyKey is crypto-shredding at the Transit engine (key deletion).
// The HTTP call maps onto key rotation/deletion managed by OpenBao operators;
// this stub surfaces the intent so callers fail closed until wired.
func (k *TransitKEK) DestroyKey(ctx context.Context, keyID string) error {
	if k == nil {
		return ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	_ = keyID

	return fmt.Errorf("%w: transit destroy requires operator rotation", ErrShredded)
}

// sealWithKey seals plaintext with key + random nonce under AAD.
func sealWithKey(key, plaintext, aad []byte) (nonce, sealed []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}

	return nonce, aead.Seal(nil, nonce, plaintext, aad), nil
}

// openWithKey opens sealed data with key + nonce under AAD.
func openWithKey(key, nonce, sealed, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plain, err := aead.Open(nil, nonce, sealed, aad)
	if err != nil {
		return nil, ErrAuthFailed
	}

	return plain, nil
}
