// Package webhook is the tenant webhook delivery adapter (E08-T05):
// HMAC-signed fanout for api-contracts §10 types with retry schedule and
// per-endpoint breaking. Payloads match §10 shapes byte-for-byte; signing
// uses the endpoint secret held here, never exposed through the port.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// DefaultTolerance bounds replay (timestamp vs now).
const DefaultTolerance = 5 * time.Minute

// SignerParams carries constructor dependencies (Parameter Object pattern).
// Secrets[0] signs; the rest verify during rotation overlap.
type SignerParams struct {
	Secrets   []string      `validate:"min=1"`
	Tolerance time.Duration `validate:"omitempty,gt=0"`
}

// Validate joins every tag violation into one error (shared format).
func (p SignerParams) Validate() error {
	return validate.Struct("webhook", "signer params", p)
}

// Signer signs and verifies webhook bodies.
type Signer struct {
	secrets   []string
	tolerance time.Duration
}

// NewSigner builds the signer; at least one non-blank secret is required.
func NewSigner(params SignerParams) (*Signer, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	secrets := make([]string, 0, len(params.Secrets))
	for _, secret := range params.Secrets {
		if trimmed := strings.TrimSpace(secret); trimmed != "" {
			secrets = append(secrets, trimmed)
		}
	}

	if len(secrets) == 0 {
		return nil, fmt.Errorf("webhook: at least one secret is required")
	}

	tolerance := params.Tolerance
	if tolerance <= 0 {
		tolerance = DefaultTolerance
	}

	return &Signer{secrets: secrets, tolerance: tolerance}, nil
}

// Sign returns the v1= signature for timestamp + "." + raw_body using the
// primary secret. Timestamp is Unix seconds (UTC).
func (s *Signer) Sign(payload []byte, timestamp time.Time) string {
	if s == nil || len(s.secrets) == 0 || payload == nil {
		return ""
	}

	return "v1=" + hex.EncodeToString(signWith(payload, timestamp, s.secrets[0]))
}

// Verify checks signature against all secrets (rotation overlap) with
// constant-time comparison and timestamp tolerance. Per api-contracts §11,
// signature carries one or more comma-separated v1= values during rotation;
// any value matching any secret verifies.
func (s *Signer) Verify(payload []byte, timestamp time.Time, signature string) error {
	if s == nil || len(s.secrets) == 0 {
		return fmt.Errorf("webhook: signer is not initialized")
	}

	if payload == nil {
		return fmt.Errorf("webhook: payload is required")
	}

	values, err := splitSignatures(signature)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	delta := now.Sub(timestamp.UTC())
	if delta < 0 {
		delta = -delta
	}

	if delta > s.tolerance {
		return fmt.Errorf("webhook: timestamp outside tolerance")
	}

	for _, secret := range s.secrets {
		expected := signWith(payload, timestamp, secret)
		for _, candidate := range values {
			if subtle.ConstantTimeCompare(expected, candidate) == 1 {
				return nil
			}
		}
	}

	return fmt.Errorf("webhook: signature mismatch")
}

// splitSignatures parses one or more comma-separated v1= hex values.
func splitSignatures(signature string) ([][]byte, error) {
	trimmed := strings.TrimSpace(signature)
	if trimmed == "" {
		return nil, fmt.Errorf("webhook: signature is required")
	}

	parts := strings.Split(trimmed, ",")
	values := make([][]byte, 0, len(parts))

	for _, part := range parts {
		hexPart, ok := strings.CutPrefix(strings.TrimSpace(part), "v1=")
		if !ok || strings.TrimSpace(hexPart) == "" {
			return nil, fmt.Errorf("webhook: signature values must start with 'v1='")
		}

		sigBytes, err := hex.DecodeString(strings.TrimSpace(hexPart))
		if err != nil {
			return nil, fmt.Errorf("webhook: signature is not hex")
		}

		values = append(values, sigBytes)
	}

	return values, nil
}

// TimestampHeader formats the Unix-seconds header value.
func TimestampHeader(timestamp time.Time) string {
	return strconv.FormatInt(timestamp.UTC().Unix(), 10)
}

func signWith(payload []byte, timestamp time.Time, secret string) []byte {
	body := TimestampHeader(timestamp) + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))

	return mac.Sum(nil)
}
