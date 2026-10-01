package jwt_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/jwt"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

func newMockClock(t *testing.T, now time.Time) *mockapplication.MockClock {
	t.Helper()
	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(now).Maybe()
	return clk
}

func mustKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return key
}

func testProvider(t *testing.T, clk appport.Clock, kid string) *jwt.SnapshotProvider {
	t.Helper()

	provider, err := jwt.NewSnapshotProvider(jwt.SnapshotParams{
		Clock:  clk,
		Keys:   map[string]jwt.KeyEntry{kid: {Private: mustKey(t), NotAfter: clk.Now().Add(time.Hour)}},
		Active: kid,
	})
	require.NoError(t, err)

	return provider
}

func testParams(clk appport.Clock, keys jwt.KeyProvider) jwt.JWTParams {
	return jwt.JWTParams{
		Issuer:        "ledger-test",
		Audience:      "ledger-api",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
		RotationGrace: time.Hour,
		Keys:          keys,
		Clock:         clk,
	}
}

func TestNewIssuer(t *testing.T) {
	t.Parallel()

	clk := newMockClock(t, time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC))
	keys := testProvider(t, clk, "k1")

	type testCase struct {
		name          string
		params        jwt.JWTParams
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid params",
			params:        testParams(clk, keys),
			expectedError: nil,
		},
		{
			name: "missing keys",
			params: func() jwt.JWTParams {
				p := testParams(clk, keys)
				p.Keys = nil
				return p
			}(),
			expectedError: jwt.ErrKeyRequired,
		},
		{
			name: "missing clock",
			params: func() jwt.JWTParams {
				p := testParams(clk, keys)
				p.Clock = nil
				return p
			}(),
			expectedError: jwt.ErrClockRequired,
		},
		{
			name: "blank issuer",
			params: func() jwt.JWTParams {
				p := testParams(clk, keys)
				p.Issuer = ""
				return p
			}(),
			expectedError: nil,
		},
		{
			name: "zero access ttl",
			params: func() jwt.JWTParams {
				p := testParams(clk, keys)
				p.AccessTTL = 0
				return p
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := jwt.NewIssuer(tc.params)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			if tc.name == "blank issuer" || tc.name == "zero access ttl" {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestIssuerIssueVerify(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(t, now)

	baseClaims := appport.TokenClaims{
		Subject:  "user-123",
		TenantID: valueobject.TenantID("tenant-abc"),
		Scopes:   []string{"ledger:read"},
	}

	type testCase struct {
		name           string
		ctx            context.Context
		claims         appport.TokenClaims
		mutateToken    func(string) string
		expectedError  error
		expectVerified bool
	}

	testCases := []testCase{
		{
			name:           "round trip",
			ctx:            context.Background(),
			claims:         baseClaims,
			mutateToken:    nil,
			expectedError:  nil,
			expectVerified: true,
		},
		{
			name:   "tampered token",
			ctx:    context.Background(),
			claims: baseClaims,
			mutateToken: func(token string) string {
				return token + "tamper"
			},
			expectedError:  jwt.ErrInvalidSignature,
			expectVerified: false,
		},
		{
			name: "empty subject",
			ctx:  context.Background(),
			claims: func() appport.TokenClaims {
				c := baseClaims
				c.Subject = ""
				return c
			}(),
			mutateToken:    nil,
			expectedError:  jwt.ErrInvalidClaims,
			expectVerified: false,
		},
		{
			name: "empty tenant",
			ctx:  context.Background(),
			claims: func() appport.TokenClaims {
				c := baseClaims
				c.TenantID = ""
				return c
			}(),
			mutateToken:    nil,
			expectedError:  jwt.ErrInvalidClaims,
			expectVerified: false,
		},
		{
			name: "canceled context on issue",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			claims:         baseClaims,
			mutateToken:    nil,
			expectedError:  context.Canceled,
			expectVerified: false,
		},
		{
			name:           "empty token verify",
			ctx:            context.Background(),
			claims:         baseClaims,
			mutateToken:    func(string) string { return "" },
			expectedError:  jwt.ErrInvalidToken,
			expectVerified: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := testProvider(t, clk, "k1")
			issuer, err := jwt.NewIssuer(testParams(clk, provider))
			require.NoError(t, err)

			token, err := issuer.Issue(tc.ctx, tc.claims)
			if tc.name == "empty subject" || tc.name == "empty tenant" || tc.name == "canceled context on issue" {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tc.mutateToken != nil {
				token = tc.mutateToken(token)
			}

			got, err := issuer.Verify(context.Background(), token)
			if !tc.expectVerified {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.claims.Subject, got.Subject)
			assert.Equal(t, tc.claims.TenantID, got.TenantID)
			assert.Equal(t, tc.claims.Scopes, got.Scopes)
		})
	}
}

func TestIssuerRotationOverlap(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(t, now)

	type testCase struct {
		name           string
		grace          time.Duration
		verifyAt       time.Time
		expectedError  error
		expectVerified bool
	}

	testCases := []testCase{
		{
			name:           "old key within grace",
			grace:          time.Hour,
			verifyAt:       now.Add(30 * time.Minute),
			expectedError:  nil,
			expectVerified: true,
		},
		{
			name:           "old key past grace excluded from jwks",
			grace:          time.Hour,
			verifyAt:       now.Add(3 * time.Hour),
			expectedError:  jwt.ErrUnknownKeyID,
			expectVerified: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			k1 := mustKey(t)
			provider, err := jwt.NewSnapshotProvider(jwt.SnapshotParams{
				Clock:  clk,
				Keys:   map[string]jwt.KeyEntry{"k1": {Private: k1, NotAfter: now.Add(time.Hour)}},
				Active: "k1",
			})
			require.NoError(t, err)

			// Long access TTL isolates the kid/grace dimension: temporal
			// expiry must never be the reason these cases pass or fail.
			issuerParams := func(c appport.Clock) jwt.JWTParams {
				return jwt.JWTParams{
					Issuer:        "ledger-test",
					Audience:      "ledger-api",
					AccessTTL:     4 * time.Hour,
					RefreshTTL:    7 * 24 * time.Hour,
					RotationGrace: tc.grace,
					Keys:          provider,
					Clock:         c,
				}
			}

			issuer, err := jwt.NewIssuer(issuerParams(clk))
			require.NoError(t, err)

			oldToken, err := issuer.Issue(context.Background(), appport.TokenClaims{
				Subject:  "user-1",
				TenantID: valueobject.TenantID("t-1"),
			})
			require.NoError(t, err)

			k2 := mustKey(t)
			provider.Swap(map[string]jwt.KeyEntry{"k2": {Private: k2, NotAfter: now.Add(2 * time.Hour)}}, "k2")

			laterClk := newMockClock(t, tc.verifyAt)
			laterIssuer, err := jwt.NewIssuer(issuerParams(laterClk))
			require.NoError(t, err)

			got, err := laterIssuer.Verify(context.Background(), oldToken)
			if tc.expectVerified {
				require.NoError(t, err)
				assert.Equal(t, "user-1", got.Subject)
				assert.Equal(t, valueobject.TenantID("t-1"), got.TenantID)

				set := laterIssuer.JWKS()
				found := false
				for _, k := range set.Keys {
					if k.Kid == "k1" {
						found = true
					}
				}
				assert.True(t, found)
				return
			}

			assert.ErrorIs(t, err, tc.expectedError)
			set := laterIssuer.JWKS()
			for _, k := range set.Keys {
				assert.NotEqual(t, "k1", k.Kid)
			}
		})
	}
}

func TestSnapshotValidation(t *testing.T) {
	t.Parallel()

	clk := newMockClock(t, time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC))

	type testCase struct {
		name          string
		params        jwt.SnapshotParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "empty keys rejected",
			params: func() jwt.SnapshotParams {
				return jwt.SnapshotParams{Clock: clk, Keys: map[string]jwt.KeyEntry{}, Active: "k1"}
			}(),
			expectedError: jwt.ErrKeyRequired,
		},
		{
			name: "unknown active rejected",
			params: func() jwt.SnapshotParams {
				return jwt.SnapshotParams{
					Clock:  clk,
					Keys:   map[string]jwt.KeyEntry{"k1": {Private: mustKey(t)}},
					Active: "k9",
				}
			}(),
			expectedError: jwt.ErrNoActiveKey,
		},
		{
			name: "unknown kid lookup",
			params: func() jwt.SnapshotParams {
				return jwt.SnapshotParams{
					Clock:  clk,
					Keys:   map[string]jwt.KeyEntry{"k1": {Private: mustKey(t)}},
					Active: "k1",
				}
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := jwt.NewSnapshotProvider(tc.params)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			if tc.name == "unknown kid lookup" {
				_, err = provider.ByID("nope")
				assert.ErrorIs(t, err, jwt.ErrUnknownKeyID)
			}
		})
	}
}

func TestSnapshotProviderConcurrentSwap(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	clk := newMockClock(t, now)

	provider, err := jwt.NewSnapshotProvider(jwt.SnapshotParams{
		Clock:  clk,
		Keys:   map[string]jwt.KeyEntry{"k1": {Private: mustKey(t), NotAfter: now.Add(time.Hour)}},
		Active: "k1",
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	prekeys := make([]*rsa.PrivateKey, 8)
	for i := range prekeys {
		prekeys[i] = mustKey(t)
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			kid := "kx-" + strconv.Itoa(i)
			provider.Swap(map[string]jwt.KeyEntry{kid: {Private: prekeys[i], NotAfter: now.Add(time.Hour)}}, kid)
		}(i)
	}
	wg.Wait()

	set := provider.ActiveSet()
	kids := make(map[string]bool, len(set))
	for _, entry := range set {
		kids[entry.KID] = true
	}

	assert.True(t, kids["k1"])
	for i := 0; i < 8; i++ {
		assert.True(t, kids["kx-"+strconv.Itoa(i)], "concurrent swap dropped a key")
	}
}

func TestIssuerVerifyRejects(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name             string
		issueTTL         time.Duration
		verifyAt         time.Time
		verifyIssuer     string
		verifyAudience   string
		cancelCtx        bool
		expectedError    error
		expectedContains string
	}

	testCases := []testCase{
		{
			name:             "expired token rejected",
			issueTTL:         time.Second,
			verifyAt:         now.Add(time.Hour),
			verifyIssuer:     "ledger-test",
			verifyAudience:   "ledger-api",
			cancelCtx:        false,
			expectedError:    jwt.ErrExpired,
			expectedContains: "",
		},
		{
			name:             "wrong audience rejected",
			issueTTL:         time.Hour,
			verifyAt:         now,
			verifyIssuer:     "ledger-test",
			verifyAudience:   "other-api",
			cancelCtx:        false,
			expectedError:    jwt.ErrInvalidClaims,
			expectedContains: "audience mismatch",
		},
		{
			name:             "wrong issuer rejected",
			issueTTL:         time.Hour,
			verifyAt:         now,
			verifyIssuer:     "other-issuer",
			verifyAudience:   "ledger-api",
			cancelCtx:        false,
			expectedError:    jwt.ErrInvalidClaims,
			expectedContains: "issuer mismatch",
		},
		{
			name:             "canceled context rejected",
			issueTTL:         time.Hour,
			verifyAt:         now,
			verifyIssuer:     "ledger-test",
			verifyAudience:   "ledger-api",
			cancelCtx:        true,
			expectedError:    context.Canceled,
			expectedContains: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clk := newMockClock(t, now)
			provider := testProvider(t, clk, "k1")

			issuer, err := jwt.NewIssuer(jwt.JWTParams{
				Issuer:        "ledger-test",
				Audience:      "ledger-api",
				AccessTTL:     tc.issueTTL,
				RefreshTTL:    7 * 24 * time.Hour,
				RotationGrace: time.Hour,
				Keys:          provider,
				Clock:         clk,
			})
			require.NoError(t, err)

			token, err := issuer.Issue(context.Background(), appport.TokenClaims{
				Subject:  "user-1",
				TenantID: valueobject.TenantID("t-1"),
			})
			require.NoError(t, err)

			verifier, err := jwt.NewIssuer(jwt.JWTParams{
				Issuer:        tc.verifyIssuer,
				Audience:      tc.verifyAudience,
				AccessTTL:     tc.issueTTL,
				RefreshTTL:    7 * 24 * time.Hour,
				RotationGrace: time.Hour,
				Keys:          provider,
				Clock:         newMockClock(t, tc.verifyAt),
			})
			require.NoError(t, err)

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}

			_, err = verifier.Verify(ctx, token)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.expectedError)
			if tc.expectedContains != "" {
				assert.Contains(t, err.Error(), tc.expectedContains)
			}
		})
	}
}
