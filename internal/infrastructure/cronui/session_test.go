package cronui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

func testSessionKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func TestNewSessionTokens(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		key         []byte
		expectError bool
	}

	testCases := []testCase{
		{
			name:        "32 byte key accepted",
			key:         testSessionKey(),
			expectError: false,
		},
		{
			name:        "longer key accepted",
			key:         append(testSessionKey(), 'x'),
			expectError: false,
		},
		{
			name:        "short key rejected",
			key:         []byte("short"),
			expectError: true,
		},
		{
			name:        "empty key rejected",
			key:         nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			issuer, err := NewSessionTokens(tc.key)
			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, issuer)

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, issuer)
		})
	}
}

func TestSessionRoundTrip(t *testing.T) {
	t.Parallel()

	issuer, err := NewSessionTokens(testSessionKey())
	require.NoError(t, err)

	type testCase struct {
		name              string
		subject           string
		ttl               time.Duration
		expectIssueErr    bool
		expectVerifyErr   bool
		expectedSubject   string
		expectedExpiresAt time.Time
	}

	testCases := []testCase{
		{
			name:              "issue and verify",
			subject:           "op-reader",
			ttl:               time.Minute,
			expectIssueErr:    false,
			expectVerifyErr:   false,
			expectedSubject:   "op-reader",
			expectedExpiresAt: time.Now().Add(time.Minute),
		},
		{
			name:            "empty subject rejected",
			subject:         "",
			ttl:             time.Minute,
			expectIssueErr:  true,
			expectVerifyErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			token, err := issuer.Issue(tc.subject, time.Now().Add(tc.ttl))
			if tc.expectIssueErr {
				assert.Error(t, err)
				assert.Empty(t, token)

				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, token)

			session, err := issuer.Verify(context.Background(), token)
			if tc.expectVerifyErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedSubject, session.Subject)
			assert.WithinDuration(t, tc.expectedExpiresAt, session.ExpiresAt, 10*time.Second)
		})
	}
}

func TestSessionTamper(t *testing.T) {
	t.Parallel()

	issuer, err := NewSessionTokens(testSessionKey())
	require.NoError(t, err)

	token, err := issuer.Issue("op-reader", time.Now().Add(time.Hour))
	require.NoError(t, err)

	type testCase struct {
		name  string
		token func(string) string
	}

	testCases := []testCase{
		{
			name: "flipped payload character",
			token: func(valid string) string {
				return "X" + valid[1:]
			},
		},
		{
			name: "flipped signature character",
			token: func(valid string) string {
				parts := strings.Split(valid, ".")

				return parts[0] + "." + "X" + parts[1][1:]
			},
		},
		{
			name: "missing separator",
			token: func(valid string) string {
				return strings.ReplaceAll(valid, ".", "")
			},
		},
		{
			name: "extra segment",
			token: func(valid string) string {
				return valid + ".extra"
			},
		},
		{
			name: "empty token",
			token: func(string) string {
				return ""
			},
		},
		{
			name: "foreign key token",
			token: func(string) string {
				foreign, err := NewSessionTokens([]byte("fedcba9876543210fedcba9876543210"))
				require.NoError(t, err)
				forged, err := foreign.Issue("op-reader", time.Now().Add(time.Hour))
				require.NoError(t, err)

				return forged
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := issuer.Verify(context.Background(), tc.token(token))
			assert.Error(t, err)
		})
	}
}

func TestSessionExpiry(t *testing.T) {
	t.Parallel()

	issuer, err := NewSessionTokens(testSessionKey())
	require.NoError(t, err)

	type testCase struct {
		name            string
		ttl             time.Duration
		expectVerifyErr bool
	}

	testCases := []testCase{
		{
			name:            "expired token rejected",
			ttl:             -time.Minute,
			expectVerifyErr: true,
		},
		{
			name:            "live token accepted",
			ttl:             time.Hour,
			expectVerifyErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			token, err := issuer.Issue("op-reader", time.Now().Add(tc.ttl))
			require.NoError(t, err)

			_, err = issuer.Verify(context.Background(), token)
			if tc.expectVerifyErr {
				assert.Error(t, err)

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestBearerTokens(t *testing.T) {
	t.Parallel()

	t.Run("missing issuer rejected", func(t *testing.T) {
		t.Parallel()

		verifier, err := NewBearerTokens(nil)
		assert.Error(t, err)
		assert.Nil(t, verifier)
	})

	type testCase struct {
		name              string
		header            string
		scriptClaims      appport.TokenClaims
		scriptErr         bool
		expectedSubject   string
		expectedScopes    []string
		expectError       bool
		expectIssuerCalls int
	}

	testCases := []testCase{
		{
			name:              "missing scheme rejected without issuer call",
			header:            "good-jwt",
			expectError:       true,
			expectIssuerCalls: 0,
		},
		{
			name:              "empty token rejected without issuer call",
			header:            "Bearer ",
			expectError:       true,
			expectIssuerCalls: 0,
		},
		{
			name:              "empty header rejected without issuer call",
			header:            "",
			expectError:       true,
			expectIssuerCalls: 0,
		},
		{
			name:              "valid header maps claims",
			header:            "Bearer good-jwt",
			scriptClaims:      appport.TokenClaims{Subject: "op-reader", Scopes: []string{"cron:read"}, ExpiresAt: time.Now().Add(time.Hour)},
			scriptErr:         false,
			expectedSubject:   "op-reader",
			expectedScopes:    []string{"cron:read"},
			expectError:       false,
			expectIssuerCalls: 1,
		},
		{
			name:              "issuer failure rejected",
			header:            "Bearer bad-jwt",
			scriptClaims:      appport.TokenClaims{},
			scriptErr:         true,
			expectError:       true,
			expectIssuerCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			issuer := &mockapplication.MockTokenIssuer{}
			verifier, err := NewBearerTokens(issuer)
			require.NoError(t, err)

			if tc.expectIssuerCalls > 0 {
				scriptErr := error(nil)
				if tc.scriptErr {
					scriptErr = assert.AnError
				}

				issuer.On(
					"Verify",
					mock.Anything,
					mock.Anything,
				).Return(tc.scriptClaims, scriptErr)
			}

			bearer, err := verifier.VerifyBearer(context.Background(), tc.header)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedSubject, bearer.Subject)
				assert.Equal(t, tc.expectedScopes, bearer.Scopes)
			}

			assert.Len(t, issuer.Calls, tc.expectIssuerCalls)
		})
	}
}
