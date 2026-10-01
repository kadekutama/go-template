package crypto_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/crypto"
)

const mockTransitBase = "https://vault.test:8200"

func mockTransitHTTPClient(configure func(transport *httpmock.MockTransport)) *http.Client {
	transport := httpmock.NewMockTransport()
	if configure != nil {
		configure(transport)
	}

	return &http.Client{Transport: transport}
}

func TestNewTransitKEKValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        crypto.TransitParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid params",
			params: crypto.TransitParams{
				Address: mockTransitBase,
				Token:   "s.validtoken",
				KeyName: "payment-key",
				HTTP:    mockTransitHTTPClient(nil),
			},
			expectedError: nil,
		},
		{
			name: "missing address",
			params: crypto.TransitParams{
				Address: "",
				Token:   "s.validtoken",
				KeyName: "payment-key",
				HTTP:    mockTransitHTTPClient(nil),
			},
			expectedError: crypto.ErrConfigRequired,
		},
		{
			name: "missing token",
			params: crypto.TransitParams{
				Address: mockTransitBase,
				Token:   "",
				KeyName: "payment-key",
				HTTP:    mockTransitHTTPClient(nil),
			},
			expectedError: crypto.ErrConfigRequired,
		},
		{
			name: "missing key name",
			params: crypto.TransitParams{
				Address: mockTransitBase,
				Token:   "s.validtoken",
				KeyName: "",
				HTTP:    mockTransitHTTPClient(nil),
			},
			expectedError: crypto.ErrConfigRequired,
		},
		{
			name: "missing http client",
			params: crypto.TransitParams{
				Address: mockTransitBase,
				Token:   "s.validtoken",
				KeyName: "payment-key",
				HTTP:    nil,
			},
			expectedError: crypto.ErrConfigRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kek, err := crypto.NewTransitKEK(tc.params)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Nil(t, kek)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, kek)
		})
	}
}

func TestTransitKEKWrapDEK(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		ctx              context.Context
		kekBuilder       func() *crypto.TransitKEK
		dek              []byte
		expectedCipher   []byte
		expectedKeyID    string
		expectedError    error
		assertErrorCheck func(t *testing.T, err error)
	}

	testCases := []testCase{
		{
			name: "successful wrap",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/encrypt/payment-key",
						httpmock.NewStringResponder(http.StatusOK, `{"data":{"ciphertext":"vault:v1:sometransitciphertext"}}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			dek:            []byte("32-byte-secret-data-encryption-k"),
			expectedCipher: []byte("vault:v1:sometransitciphertext"),
			expectedKeyID:  "payment-key",
			expectedError:  nil,
		},
		{
			name: "nil receiver",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				return nil
			},
			dek:           []byte("dek"),
			expectedError: crypto.ErrNotInitialized,
		},
		{
			name: "http 403 forbidden",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/encrypt/payment-key",
						httpmock.NewStringResponder(http.StatusForbidden, `{"errors":["permission denied"]}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.badtok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			dek:           []byte("dek"),
			expectedError: crypto.ErrAuthFailed,
		},
		{
			name: "network transport error",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/encrypt/payment-key",
						httpmock.NewErrorResponder(errors.New("dial tcp: connection refused")))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			dek: []byte("dek"),
			assertErrorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "connection refused")
			},
		},
		{
			name: "invalid json response body",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/encrypt/payment-key",
						httpmock.NewStringResponder(http.StatusOK, `not valid json`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			dek: []byte("dek"),
			assertErrorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "crypto: transit wrap decode")
			},
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/encrypt/payment-key",
						httpmock.NewStringResponder(http.StatusOK, `{"data":{"ciphertext":"vault:v1:cipher"}}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			dek: []byte("dek"),
			assertErrorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kek := tc.kekBuilder()
			wrapped, keyID, err := kek.WrapDEK(tc.ctx, tc.dek)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}
			if tc.assertErrorCheck != nil {
				require.Error(t, err)
				tc.assertErrorCheck(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCipher, wrapped)
			assert.Equal(t, tc.expectedKeyID, keyID)
		})
	}
}

func TestTransitKEKUnwrapDEK(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		ctx              context.Context
		kekBuilder       func() *crypto.TransitKEK
		keyID            string
		wrapped          []byte
		expectedDEK      []byte
		expectedError    error
		assertErrorCheck func(t *testing.T, err error)
	}

	testCases := []testCase{
		{
			name: "successful unwrap",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					plaintext := base64.StdEncoding.EncodeToString([]byte("my-decrypted-dek"))
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/decrypt/payment-key",
						httpmock.NewStringResponder(http.StatusOK, `{"data":{"plaintext":"`+plaintext+`"}}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:         "payment-key",
			wrapped:       []byte("vault:v1:sometransitciphertext"),
			expectedDEK:   []byte("my-decrypted-dek"),
			expectedError: nil,
		},
		{
			name: "nil receiver",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				return nil
			},
			keyID:         "payment-key",
			wrapped:       []byte("vault:v1:cipher"),
			expectedError: crypto.ErrNotInitialized,
		},
		{
			name: "http 400 bad request",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/decrypt/payment-key",
						httpmock.NewStringResponder(http.StatusBadRequest, `{"errors":["invalid ciphertext"]}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:         "payment-key",
			wrapped:       []byte("corrupt-cipher"),
			expectedError: crypto.ErrAuthFailed,
		},
		{
			name: "network transport error",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/decrypt/payment-key",
						httpmock.NewErrorResponder(errors.New("tls: handshake failure")))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:   "payment-key",
			wrapped: []byte("vault:v1:cipher"),
			assertErrorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "handshake failure")
			},
		},
		{
			name: "invalid base64 in response plaintext",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/decrypt/payment-key",
						httpmock.NewStringResponder(http.StatusOK, `{"data":{"plaintext":"!not-base64!"}}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:         "payment-key",
			wrapped:       []byte("vault:v1:cipher"),
			expectedError: crypto.ErrAuthFailed,
		},
		{
			name: "invalid json response",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/decrypt/payment-key",
						httpmock.NewStringResponder(http.StatusOK, `invalid json`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:   "payment-key",
			wrapped: []byte("vault:v1:cipher"),
			assertErrorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "crypto: transit unwrap decode")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kek := tc.kekBuilder()
			dek, err := kek.UnwrapDEK(tc.ctx, tc.keyID, tc.wrapped)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}
			if tc.assertErrorCheck != nil {
				require.Error(t, err)
				tc.assertErrorCheck(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedDEK, dek)
		})
	}
}

func TestTransitKEKDestroyKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		kekBuilder    func() *crypto.TransitKEK
		keyID         string
		expectedError error
	}

	testCases := []testCase{
		{
			name: "destroy fails with ErrShredded operator message",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(nil)
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:         "payment-key",
			expectedError: crypto.ErrShredded,
		},
		{
			name: "nil receiver",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				return nil
			},
			keyID:         "payment-key",
			expectedError: crypto.ErrNotInitialized,
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(nil)
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:         "payment-key",
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kek := tc.kekBuilder()
			err := kek.DestroyKey(tc.ctx, tc.keyID)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestTransitKEKRewrapDEK(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ctx            context.Context
		kekBuilder     func() *crypto.TransitKEK
		keyID          string
		wrapped        []byte
		expectedCipher []byte
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "successful rewrap under configured key",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/rewrap/payment-key",
						httpmock.NewStringResponder(http.StatusOK, `{"data":{"ciphertext":"vault:v2:rewrapped"}}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:          "payment-key",
			wrapped:        []byte("vault:v1:oldcipher"),
			expectedCipher: []byte("vault:v2:rewrapped"),
			expectedError:  nil,
		},
		{
			name: "explicit key id selects key name",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/rewrap/other-key",
						httpmock.NewStringResponder(http.StatusOK, `{"data":{"ciphertext":"vault:v2:other"}}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:          "other-key",
			wrapped:        []byte("vault:v1:oldcipher"),
			expectedCipher: []byte("vault:v2:other"),
			expectedError:  nil,
		},
		{
			name: "nil receiver",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				return nil
			},
			keyID:          "payment-key",
			wrapped:        []byte("vault:v1:cipher"),
			expectedCipher: nil,
			expectedError:  crypto.ErrNotInitialized,
		},
		{
			name: "http 400 bad request",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/rewrap/payment-key",
						httpmock.NewStringResponder(http.StatusBadRequest, `{"errors":["invalid ciphertext"]}`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:          "payment-key",
			wrapped:        []byte("corrupt-cipher"),
			expectedCipher: nil,
			expectedError:  crypto.ErrAuthFailed,
		},
		{
			name: "malformed json body",
			ctx:  context.Background(),
			kekBuilder: func() *crypto.TransitKEK {
				client := mockTransitHTTPClient(func(tr *httpmock.MockTransport) {
					tr.RegisterResponder("POST", mockTransitBase+"/v1/transit/rewrap/payment-key",
						httpmock.NewStringResponder(http.StatusOK, `not-json`))
				})
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    client,
				})
				return kek
			},
			keyID:          "payment-key",
			wrapped:        []byte("vault:v1:cipher"),
			expectedCipher: nil,
			expectedError:  errors.New("crypto: transit rewrap decode"),
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			kekBuilder: func() *crypto.TransitKEK {
				kek, _ := crypto.NewTransitKEK(crypto.TransitParams{
					Address: mockTransitBase,
					Token:   "s.tok",
					KeyName: "payment-key",
					HTTP:    mockTransitHTTPClient(nil),
				})
				return kek
			},
			keyID:          "payment-key",
			wrapped:        []byte("vault:v1:cipher"),
			expectedCipher: nil,
			expectedError:  context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kek := tc.kekBuilder()
			ciphertext, err := kek.RewrapDEK(tc.ctx, tc.keyID, tc.wrapped)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCipher, ciphertext)
		})
	}
}
