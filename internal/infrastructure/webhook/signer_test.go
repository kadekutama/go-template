package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
)

func TestSignerMatchesContractVector(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		secret         string
		payload        []byte
		timestamp      time.Time
		expectedResult string
	}

	testCases := []testCase{
		{
			name:      "ascii payload",
			secret:    "whsec_test",
			payload:   []byte(`{"id":"evt_1"}`),
			timestamp: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
			expectedResult: func() string {
				mac := hmac.New(sha256.New, []byte("whsec_test"))
				stamp := strconv.FormatInt(time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC).Unix(), 10)
				mac.Write([]byte(stamp + "." + `{"id":"evt_1"}`))

				return "v1=" + hex.EncodeToString(mac.Sum(nil))
			}(),
		},
		{
			name:      "empty payload",
			secret:    "s2",
			payload:   []byte{},
			timestamp: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
			expectedResult: func() string {
				mac := hmac.New(sha256.New, []byte("s2"))
				stamp := strconv.FormatInt(time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC).Unix(), 10)
				mac.Write([]byte(stamp + "."))

				return "v1=" + hex.EncodeToString(mac.Sum(nil))
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signer, err := webhook.NewSigner(webhook.SignerParams{
				Secrets:   []string{tc.secret},
				Tolerance: 365 * 24 * time.Hour,
			})
			require.NoError(t, err)

			assert.Equal(t, tc.expectedResult, signer.Sign(tc.payload, tc.timestamp))
			require.NoError(t, signer.Verify(tc.payload, tc.timestamp, tc.expectedResult))
		})
	}
}

func TestNewSigner(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         webhook.SignerParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid single secret",
			params: webhook.SignerParams{
				Secrets: []string{"s1"},
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "valid multiple secrets with rotation",
			params: webhook.SignerParams{
				Secrets: []string{"s1", "s2"},
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "whitespace trimmed keeps signer usable",
			params: webhook.SignerParams{
				Secrets: []string{"  s1  "},
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "empty secrets rejected",
			params: webhook.SignerParams{
				Secrets: nil,
			},
			expectedResult: false,
			expectedError:  errors.New("webhook: invalid signer params (1 violation(s)): Secrets: rule \"min\" on value []"),
		},
		{
			name: "whitespace only secrets rejected",
			params: webhook.SignerParams{
				Secrets: []string{"", "  "},
			},
			expectedResult: false,
			expectedError:  errors.New("webhook: at least one secret is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signer, err := webhook.NewSigner(tc.params)
			assert.Equal(t, tc.expectedResult, signer != nil)

			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSignerSign(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		signer            func() *webhook.Signer
		payload           []byte
		timestamp         time.Time
		expectedLength    int
		expectedHasPrefix bool
	}

	testCases := []testCase{
		{
			name: "nil receiver produces empty signature",
			signer: func() *webhook.Signer {
				return nil
			},
			payload:           []byte(`{"type":"transfer.completed"}`),
			timestamp:         time.Now().UTC(),
			expectedLength:    0,
			expectedHasPrefix: false,
		},
		{
			name: "valid payload produces v1 signature",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:           []byte(`{"type":"transfer.completed"}`),
			timestamp:         time.Now().UTC(),
			expectedLength:    3 + 64,
			expectedHasPrefix: true,
		},
		{
			name: "nil payload produces empty string",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:           nil,
			timestamp:         time.Now().UTC(),
			expectedLength:    0,
			expectedHasPrefix: false,
		},
		{
			name: "empty payload signs empty body",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:           []byte(`{}`),
			timestamp:         time.Now().UTC(),
			expectedLength:    3 + 64,
			expectedHasPrefix: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.signer()
			signature := s.Sign(tc.payload, tc.timestamp)
			assert.Len(t, signature, tc.expectedLength)
			assert.Equal(t, tc.expectedHasPrefix, strings.HasPrefix(signature, "v1="))
		})
	}
}

func TestSignerVerify(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		signer          func() *webhook.Signer
		payload         []byte
		timestamp       time.Time
		signatureHeader string
		expectedError   error
	}

	testCases := []testCase{
		{
			name: "nil receiver rejected",
			signer: func() *webhook.Signer {
				return nil
			},
			payload:         []byte(`{"type":"x"}`),
			timestamp:       time.Now().UTC(),
			signatureHeader: "v1=abcd",
			expectedError:   errors.New("webhook: signer is not initialized"),
		},
		{
			name: "valid signature verifies",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:   []byte(`{"type":"x"}`),
			timestamp: time.Now().UTC(),
			signatureHeader: func() string {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s.Sign([]byte(`{"type":"x"}`), time.Now().UTC())
			}(),
			expectedError: nil,
		},
		{
			name: "tampered body fails",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:   []byte(`{"type":"y"}`),
			timestamp: time.Now().UTC(),
			signatureHeader: func() string {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s.Sign([]byte(`{"type":"x"}`), time.Now().UTC())
			}(),
			expectedError: errors.New("webhook: signature mismatch"),
		},
		{
			name: "stale timestamp fails",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:   []byte(`{"type":"x"}`),
			timestamp: time.Now().UTC().Add(-time.Hour),
			signatureHeader: func() string {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s.Sign([]byte(`{"type":"x"}`), time.Now().UTC().Add(-time.Hour))
			}(),
			expectedError: errors.New("webhook: timestamp outside tolerance"),
		},
		{
			name: "malformed signature without v1 prefix fails",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:         []byte(`{"type":"x"}`),
			timestamp:       time.Now().UTC(),
			signatureHeader: "bad-signature",
			expectedError:   errors.New("webhook: signature values must start with 'v1='"),
		},
		{
			name: "malformed signature non-hex fails",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:         []byte(`{"type":"x"}`),
			timestamp:       time.Now().UTC(),
			signatureHeader: "v1=not-a-hex-string-zz",
			expectedError:   errors.New("webhook: signature is not hex"),
		},
		{
			name: "empty signature fails",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:         []byte(`{"type":"x"}`),
			timestamp:       time.Now().UTC(),
			signatureHeader: "",
			expectedError:   errors.New("webhook: signature is required"),
		},
		{
			name: "nil payload fails",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return s
			},
			payload:         nil,
			timestamp:       time.Now().UTC(),
			signatureHeader: "v1=1234",
			expectedError:   errors.New("webhook: payload is required"),
		},
		{
			name: "rotated signature verifies against rotated secrets",
			signer: func() *webhook.Signer {
				s, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1", "s2"}})
				return s
			},
			payload:   []byte(`{"type":"x"}`),
			timestamp: time.Now().UTC(),
			signatureHeader: func() string {
				oldSigner, _ := webhook.NewSigner(webhook.SignerParams{Secrets: []string{"s1"}})
				return oldSigner.Sign([]byte(`{"type":"x"}`), time.Now().UTC())
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.signer()
			err := s.Verify(tc.payload, tc.timestamp, tc.signatureHeader)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestSignerRotation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		signerSecrets      []string
		incomingSecret     string
		expectedVerifyPass bool
	}

	testCases := []testCase{
		{
			name:               "rotation overlap verifies older secret",
			signerSecrets:      []string{"new", "old"},
			incomingSecret:     "old",
			expectedVerifyPass: true,
		},
		{
			name:               "rotation overlap verifies newer secret",
			signerSecrets:      []string{"new", "old"},
			incomingSecret:     "new",
			expectedVerifyPass: true,
		},
		{
			name:               "single new secret rejects old secret",
			signerSecrets:      []string{"new"},
			incomingSecret:     "old",
			expectedVerifyPass: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signer, err := webhook.NewSigner(webhook.SignerParams{Secrets: tc.signerSecrets})
			require.NoError(t, err)

			otherSigner, err := webhook.NewSigner(webhook.SignerParams{Secrets: []string{tc.incomingSecret}})
			require.NoError(t, err)

			payload := []byte(`{"type":"rotation-test"}`)
			stamp := time.Now().UTC()
			sig := otherSigner.Sign(payload, stamp)

			err = signer.Verify(payload, stamp, sig)
			assert.Equal(t, tc.expectedVerifyPass, err == nil)
		})
	}
}
