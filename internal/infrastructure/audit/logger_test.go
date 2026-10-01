package audit_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/audit"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockaudit "github.com/kadekutama/go-template/test/mock/audit"
)

// lazyDB opens a GORM handle without connecting: DisableAutomaticPing keeps
// Open from dialing, so input-validation paths (which return before any SQL)
// run without Docker.
func lazyDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(gormpostgres.Open("postgres://127.0.0.1:1/audit_unit?sslmode=disable"), &gorm.Config{
		SkipDefaultTransaction: true,
		DisableAutomaticPing:   true,
	})
	require.NoError(t, err)

	return db
}

func testLogger(t *testing.T) *audit.Logger {
	t.Helper()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	logger, err := audit.NewLogger(audit.AuditParams{
		DB:         lazyDB(t),
		SigningKey: []byte("0123456789abcdef0123456789abcdef"),
		Clock:      clk,
	})
	require.NoError(t, err)

	return logger
}

func TestNewLoggerValidation(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		params        audit.AuditParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "missing db rejected",
			params: func() audit.AuditParams {
				return audit.AuditParams{
					SigningKey: []byte("0123456789abcdef0123456789abcdef"),
					Clock:      clk,
				}
			}(),
			expectedError: audit.ErrConfigRequired,
		},
		{
			name: "short signing key rejected",
			params: func() audit.AuditParams {
				return audit.AuditParams{
					DB:         lazyDB(t),
					SigningKey: []byte("short"),
					Clock:      clk,
				}
			}(),
			expectedError: audit.ErrConfigRequired,
		},
		{
			name: "missing clock rejected",
			params: func() audit.AuditParams {
				return audit.AuditParams{
					DB:         lazyDB(t),
					SigningKey: []byte("0123456789abcdef0123456789abcdef"),
					Clock:      nil,
				}
			}(),
			expectedError: audit.ErrClockRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := audit.NewLogger(tc.params)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestRecordValidation(t *testing.T) {
	t.Parallel()

	const tenant = "0199a5a0-2b7e-7a1e-9b0c-4d5e6f7a8b9c"

	type testCase struct {
		name          string
		ctx           context.Context
		entry         appport.AuditEntry
		expectedError error
	}

	testCases := []testCase{
		{
			name: "secret material rejected before storage",
			ctx:  context.Background(),
			entry: func() appport.AuditEntry {
				return appport.AuditEntry{
					TenantID: tenant,
					Actor:    "alice",
					Action:   "login password=hunter2",
					Resource: "session:1",
				}
			}(),
			expectedError: audit.ErrSecretInEntry,
		},
		{
			name: "bearer token rejected",
			ctx:  context.Background(),
			entry: func() appport.AuditEntry {
				return appport.AuditEntry{
					TenantID: tenant,
					Actor:    "alice",
					Action:   "call",
					Resource: "api bearer abc123",
				}
			}(),
			expectedError: audit.ErrSecretInEntry,
		},
		{
			name: "empty actor rejected",
			ctx:  context.Background(),
			entry: func() appport.AuditEntry {
				return appport.AuditEntry{
					TenantID: tenant,
					Action:   "approve",
					Resource: "break:1",
				}
			}(),
			expectedError: audit.ErrConfigRequired,
		},
		{
			name: "non-uuid tenant rejected",
			ctx:  context.Background(),
			entry: func() appport.AuditEntry {
				return appport.AuditEntry{
					TenantID: "not-a-uuid",
					Actor:    "alice",
					Action:   "approve",
					Resource: "break:1",
				}
			}(),
			expectedError: audit.ErrConfigRequired,
		},
		{
			name: "canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			entry: func() appport.AuditEntry {
				return appport.AuditEntry{
					TenantID: tenant,
					Actor:    "alice",
					Action:   "approve",
					Resource: "break:1",
				}
			}(),
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testLogger(t)

			err := logger.Record(tc.ctx, tc.entry)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestRecordTxValidation(t *testing.T) {
	t.Parallel()

	const tenant = "0199a5a0-2b7e-7a1e-9b0c-4d5e6f7a8b9c"

	type testCase struct {
		name          string
		ctx           context.Context
		tx            *gorm.DB
		entry         appport.AuditEntry
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil tx rejected",
			ctx:  context.Background(),
			tx:   nil,
			entry: func() appport.AuditEntry {
				return appport.AuditEntry{
					TenantID: tenant,
					Actor:    "alice",
					Action:   "approve",
					Resource: "break:1",
				}
			}(),
			expectedError: audit.ErrConfigRequired,
		},
		{
			name: "secret in entry rejected on tx",
			ctx:  context.Background(),
			tx:   lazyDB(t),
			entry: func() appport.AuditEntry {
				return appport.AuditEntry{
					TenantID: tenant,
					Actor:    "alice",
					Action:   "update private_key=abc",
					Resource: "key:1",
				}
			}(),
			expectedError: audit.ErrSecretInEntry,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testLogger(t)

			err := logger.RecordTx(tc.ctx, tc.tx, tc.entry)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestMerkleTreeRoot(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		nodes        []string
		expectedRoot string
	}

	testCases := []testCase{
		{
			name:  "empty list produces nil hash",
			nodes: nil,
			expectedRoot: func() string {
				sum := sha256.Sum256(nil)
				return hex.EncodeToString(sum[:])
			}(),
		},
		{
			name:         "single leaf returns leaf unchanged",
			nodes:        []string{"leaf-hash-1"},
			expectedRoot: "leaf-hash-1",
		},
		{
			name:  "two leaves paired directly",
			nodes: []string{"a", "b"},
			expectedRoot: func() string {
				sum := sha256.Sum256([]byte("ab"))
				return hex.EncodeToString(sum[:])
			}(),
		},
		{
			name:  "three leaves with odd paired with itself",
			nodes: []string{"a", "b", "c"},
			expectedRoot: func() string {
				ab := sha256.Sum256([]byte("ab"))
				cc := sha256.Sum256([]byte("cc"))
				root := sha256.Sum256([]byte(hex.EncodeToString(ab[:]) + hex.EncodeToString(cc[:])))
				return hex.EncodeToString(root[:])
			}(),
		},
		{
			name:  "four leaves binary balance",
			nodes: []string{"a", "b", "c", "d"},
			expectedRoot: func() string {
				ab := sha256.Sum256([]byte("ab"))
				cd := sha256.Sum256([]byte("cd"))
				root := sha256.Sum256([]byte(hex.EncodeToString(ab[:]) + hex.EncodeToString(cd[:])))
				return hex.EncodeToString(root[:])
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := audit.MerkleTreeRootForTest(tc.nodes)
			assert.Equal(t, tc.expectedRoot, actual)
		})
	}
}

func TestSignAndVerifyHash(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")

	type testCase struct {
		name          string
		key           []byte
		entryHash     string
		tamperedSig   bool
		expectedValid bool
	}

	testCases := []testCase{
		{
			name:          "valid signature passes",
			key:           key,
			entryHash:     "sample-entry-hash-12345",
			tamperedSig:   false,
			expectedValid: true,
		},
		{
			name:          "tampered signature fails",
			key:           key,
			entryHash:     "sample-entry-hash-12345",
			tamperedSig:   true,
			expectedValid: false,
		},
		{
			name:          "wrong key fails",
			key:           []byte("different-key-0123456789abcdef01"),
			entryHash:     "sample-entry-hash-12345",
			tamperedSig:   false,
			expectedValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sig := audit.SignHashForTest(key, tc.entryHash)
			if tc.tamperedSig {
				sig = "00" + sig[2:]
			}

			valid := audit.VerifySignatureForTest(tc.key, tc.entryHash, sig)
			assert.Equal(t, tc.expectedValid, valid)
		})
	}
}

func TestCheckpointValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		tenant        string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "invalid tenant format rejected",
			tenant:        "not-valid",
			expectedError: audit.ErrConfigRequired,
		},
		{
			name:          "empty tenant rejected",
			tenant:        "",
			expectedError: audit.ErrConfigRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clk := mockapplication.NewMockClock(t)
			clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

			worm := mockaudit.NewMockWORMExporter(t)
			worm.EXPECT().ArchiveCheckpoint(mock.Anything, mock.Anything).Return(errors.New("worm error")).Maybe()

			logger, err := audit.NewLogger(audit.AuditParams{
				DB:         lazyDB(t),
				SigningKey: []byte("0123456789abcdef0123456789abcdef"),
				Clock:      clk,
				WORM:       worm,
			})
			require.NoError(t, err)

			_, chkErr := logger.Checkpoint(context.Background(), tc.tenant)
			assert.ErrorIs(t, chkErr, tc.expectedError)
		})
	}
}
