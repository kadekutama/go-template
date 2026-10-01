package crypto_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/crypto"
	mockcrypto "github.com/kadekutama/go-template/test/mock/crypto"
)

type mockKEKState struct {
	keys    sync.Map
	version atomic.Int64
	current atomic.Pointer[string]
}

func (s *mockKEKState) rotate() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	v := s.version.Add(1)
	id := fmt.Sprintf("v%d", v)
	s.keys.Store(id, key)
	s.current.Store(&id)
	return id, nil
}

func (s *mockKEKState) wrapDEK(_ context.Context, dek []byte) ([]byte, string, error) {
	cur := s.current.Load()
	if cur == nil {
		return nil, "", crypto.ErrUnknownKey
	}
	val, ok := s.keys.Load(*cur)
	if !ok {
		return nil, "", crypto.ErrUnknownKey
	}
	key := val.([]byte)
	nonce, sealed, err := sealTestDEK(key, dek)
	if err != nil {
		return nil, "", err
	}
	return append(append([]byte(nil), nonce...), sealed...), *cur, nil
}

func (s *mockKEKState) unwrapDEK(_ context.Context, keyID string, wrapped []byte) ([]byte, error) {
	val, ok := s.keys.Load(keyID)
	if !ok {
		return nil, crypto.ErrShredded
	}
	key := val.([]byte)
	if len(wrapped) < 12 {
		return nil, crypto.ErrAuthFailed
	}
	return openTestDEK(key, wrapped[:12], wrapped[12:], keyID)
}

func (s *mockKEKState) rewrapDEK(ctx context.Context, keyID string, wrapped []byte) ([]byte, error) {
	dek, err := s.unwrapDEK(ctx, keyID, wrapped)
	if err != nil {
		return nil, err
	}
	cur := s.current.Load()
	if cur == nil {
		return nil, crypto.ErrUnknownKey
	}
	curVal, ok := s.keys.Load(*cur)
	if !ok {
		return nil, crypto.ErrUnknownKey
	}
	curKey := curVal.([]byte)
	nonce, sealed, err := sealTestDEK(curKey, dek)
	if err != nil {
		return nil, err
	}
	return append(append([]byte(nil), nonce...), sealed...), nil
}

func (s *mockKEKState) destroyKey(_ context.Context, keyID string) error {
	if _, ok := s.keys.Load(keyID); !ok {
		return crypto.ErrUnknownKey
	}
	s.keys.Delete(keyID)
	return nil
}

func newMockKEK(t *testing.T) (*mockcrypto.MockKeyEncryptionKey, func() (string, error)) {
	t.Helper()

	kek := mockcrypto.NewMockKeyEncryptionKey(t)
	state := &mockKEKState{}

	_, err := state.rotate()
	require.NoError(t, err)

	kek.EXPECT().WrapDEK(mock.Anything, mock.Anything).RunAndReturn(state.wrapDEK).Maybe()
	kek.EXPECT().UnwrapDEK(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(state.unwrapDEK).Maybe()
	kek.EXPECT().RewrapDEK(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(state.rewrapDEK).Maybe()
	kek.EXPECT().DestroyKey(mock.Anything, mock.Anything).RunAndReturn(state.destroyKey).Maybe()

	return kek, state.rotate
}

func sealTestDEK(key, dek []byte) (nonce, sealed []byte, err error) {
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

	return nonce, aead.Seal(nil, nonce, dek, []byte("kek-test")), nil
}

func openTestDEK(key, nonce, sealed []byte, _ string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plain, err := aead.Open(nil, nonce, sealed, []byte("kek-test"))
	if err != nil {
		return nil, crypto.ErrAuthFailed
	}

	return plain, nil
}

func testEnveloper(t *testing.T) *crypto.Enveloper {
	t.Helper()

	kek, _ := newMockKEK(t)
	enveloper, err := crypto.NewEnveloper(crypto.CryptoParams{KEK: kek})
	require.NoError(t, err)

	return enveloper
}

func TestEnvelopeRoundTrip(t *testing.T) {
	t.Parallel()

	tenant := valueobject.TenantID("tenant-crypto")

	type testCase struct {
		name          string
		ctx           context.Context
		tenant        valueobject.TenantID
		purpose       string
		plaintext     []byte
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "round trip",
			ctx:           context.Background(),
			tenant:        tenant,
			purpose:       "email",
			plaintext:     []byte("user@example.com"),
			expectedError: nil,
		},
		{
			name:          "empty plaintext rejected",
			ctx:           context.Background(),
			tenant:        tenant,
			purpose:       "email",
			plaintext:     nil,
			expectedError: crypto.ErrConfigRequired,
		},
		{
			name:          "empty purpose rejected",
			ctx:           context.Background(),
			tenant:        tenant,
			purpose:       "",
			plaintext:     []byte("x"),
			expectedError: crypto.ErrConfigRequired,
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			tenant:        tenant,
			purpose:       "email",
			plaintext:     []byte("x"),
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			enveloper := testEnveloper(t)

			envelope, err := enveloper.Encrypt(tc.ctx, tc.tenant, tc.purpose, tc.plaintext)
			if tc.expectedError != nil {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			plain, err := enveloper.Decrypt(context.Background(), tc.tenant, envelope)
			require.NoError(t, err)
			assert.Equal(t, tc.plaintext, plain)
		})
	}
}

func TestEnvelopeUniquenessTamper(t *testing.T) {
	t.Parallel()

	tenant := valueobject.TenantID("tenant-nonce")

	type testCase struct {
		name          string
		tamperAAD     bool
		tamperBytes   bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "identical plaintext differs",
			tamperAAD:     false,
			tamperBytes:   false,
			expectedError: nil,
		},
		{
			name:          "wrong tenant fails",
			tamperAAD:     true,
			tamperBytes:   false,
			expectedError: crypto.ErrAuthFailed,
		},
		{
			name:          "bit flip fails",
			tamperAAD:     false,
			tamperBytes:   true,
			expectedError: crypto.ErrAuthFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			enveloper := testEnveloper(t)

			first, err := enveloper.Encrypt(context.Background(), tenant, "pan", []byte("4111111111111111"))
			require.NoError(t, err)

			second, err := enveloper.Encrypt(context.Background(), tenant, "pan", []byte("4111111111111111"))
			require.NoError(t, err)
			assert.NotEqual(t, first.Ciphertext, second.Ciphertext)
			assert.NotEqual(t, first.Nonce, second.Nonce)

			tenantForDecrypt := tenant
			if tc.tamperAAD {
				tenantForDecrypt = valueobject.TenantID("other-tenant")
			}

			target := first
			if tc.tamperBytes {
				target.Ciphertext[len(target.Ciphertext)-1] ^= 0x01
			}

			_, err = enveloper.Decrypt(context.Background(), tenantForDecrypt, target)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestEnvelopeRotationShred(t *testing.T) {
	t.Parallel()

	tenant := valueobject.TenantID("tenant-rotate")

	type testCase struct {
		name          string
		shred         bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "rotation keeps old readable",
			shred:         false,
			expectedError: nil,
		},
		{
			name:          "shred makes unrecoverable",
			shred:         true,
			expectedError: crypto.ErrShredded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kek, rotate := newMockKEK(t)

			enveloper, err := crypto.NewEnveloper(crypto.CryptoParams{KEK: kek})
			require.NoError(t, err)

			old, err := enveloper.Encrypt(context.Background(), tenant, "email", []byte("a@b.co"))
			require.NoError(t, err)

			_, err = rotate()
			require.NoError(t, err)

			fresh, err := enveloper.Encrypt(context.Background(), tenant, "email", []byte("c@d.co"))
			require.NoError(t, err)

			_, err = enveloper.Decrypt(context.Background(), tenant, fresh)
			require.NoError(t, err)

			if tc.shred {
				require.NoError(t, kek.DestroyKey(context.Background(), old.KeyID))

				_, err = enveloper.Decrypt(context.Background(), tenant, old)
				assert.ErrorIs(t, err, crypto.ErrShredded)
				return
			}

			_, err = enveloper.Decrypt(context.Background(), tenant, old)
			assert.NoError(t, err)
		})
	}
}

func TestPIIFields(t *testing.T) {
	t.Parallel()

	tenant := valueobject.TenantID("tenant-pii")

	type testCase struct {
		name          string
		email         string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "field round trip",
			email:         "user@example.com",
			expectedError: nil,
		},
		{
			name:          "empty email rejected at field level",
			email:         "",
			expectedError: crypto.ErrConfigRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			enveloper := testEnveloper(t)

			token, err := enveloper.EncryptPIIField(context.Background(), tenant, "user.email", tc.email)
			if tc.email == "" {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEqual(t, tc.email, token)

			plain, err := enveloper.DecryptPIIField(context.Background(), tenant, token)
			require.NoError(t, err)
			assert.Equal(t, tc.email, plain)
		})
	}
}
