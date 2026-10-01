package audit_test

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	_ "github.com/jackc/pgx/v5/stdlib"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/audit"
	"github.com/kadekutama/go-template/internal/infrastructure/database/migration"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	"github.com/kadekutama/go-template/test/testcontainers"
)

const auditTenant = "0199a5a0-2b7e-7a1e-9b0c-4d5e6f7a8b9d"

func openAuditLogger(t *testing.T) *audit.Logger {
	t.Helper()

	logger, _ := openAuditTx(t)

	return logger
}

func openAuditTx(t *testing.T) (*audit.Logger, *gorm.DB) {
	t.Helper()
	testcontainers.SkipIfNoDocker(t)

	handle, err := testcontainers.StartPostgres(t, "ledger_audit")
	require.NoError(t, err)

	sqlDB, err := sql.Open("pgx", handle.ConnectionString())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	runner, err := migration.NewRunner(migration.RunnerParams{
		DB:      sqlDB,
		Timeout: 30 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, runner.Up(context.Background()))

	gormDB, err := postgres.Open(postgres.Config{
		DSN:             handle.ConnectionString(),
		MaxOpen:         25,
		MaxIdle:         5,
		ConnMaxLifetime: 30 * time.Minute,
	})
	require.NoError(t, err)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	logger, err := audit.NewLogger(audit.AuditParams{
		DB:         gormDB,
		SigningKey: []byte("0123456789abcdef0123456789abcdef"),
		Clock:      clk,
	})
	require.NoError(t, err)

	return logger, gormDB
}

func auditEntry() appport.AuditEntry {
	return appport.AuditEntry{
		TenantID:   valueobject.TenantID(auditTenant),
		Actor:      "alice",
		Action:     "approve",
		Resource:   "break:1",
		BeforeHash: "before-1",
		AfterHash:  "after-1",
		OccurredAt: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC),
	}
}

func TestAuditDurableChain(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	type testCase struct {
		name          string
		secondAction  string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "append twice and verify",
			secondAction:  "erase",
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := openAuditLogger(t)
			ctx := context.Background()

			require.NoError(t, logger.Record(ctx, auditEntry()))

			second := auditEntry()
			second.Action = tc.secondAction
			second.Resource = "pii:9"
			require.NoError(t, logger.Record(ctx, second))

			require.NoError(t, logger.Verify(ctx, auditTenant))

			entries, err := logger.Entries(ctx, auditTenant)
			require.NoError(t, err)
			require.Len(t, entries, 2)
			assert.Equal(t, int64(1), entries[0].Seq)
			assert.Equal(t, int64(2), entries[1].Seq)
			assert.Equal(t, entries[0].EntryHash, entries[1].PrevHash)
		})
	}
}

func TestAuditCheckpointTamperDetection(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	type testCase struct {
		name          string
		forge         func(root audit.SignedRoot) audit.SignedRoot
		expectedError error
	}

	testCases := []testCase{
		{
			name: "checkpoint passes untouched",
			forge: func(root audit.SignedRoot) audit.SignedRoot {
				return root
			},
			expectedError: nil,
		},
		{
			name: "reordered chain detected via seq",
			forge: func(root audit.SignedRoot) audit.SignedRoot {
				root.Seq++
				return root
			},
			expectedError: audit.ErrCheckpointInvalid,
		},
		{
			name: "tampered root detected",
			forge: func(root audit.SignedRoot) audit.SignedRoot {
				root.RootHash = "tampered-root"
				return root
			},
			expectedError: audit.ErrCheckpointInvalid,
		},
		{
			name: "deleted entry detected via seq",
			forge: func(root audit.SignedRoot) audit.SignedRoot {
				root.Seq--
				return root
			},
			expectedError: audit.ErrCheckpointInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := openAuditLogger(t)
			ctx := context.Background()

			require.NoError(t, logger.Record(ctx, auditEntry()))

			second := auditEntry()
			second.Action = "erase"
			require.NoError(t, logger.Record(ctx, second))

			root, err := logger.Checkpoint(ctx, auditTenant)
			require.NoError(t, err)

			err = logger.VerifyCheckpoint(ctx, tc.forge(root))
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestRecordTxJoinsCallerTransaction(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	type testCase struct {
		name          string
		commit        bool
		nilTx         bool
		expectedCount int
	}

	testCases := []testCase{
		{
			name:          "rollback discards the fact",
			commit:        false,
			nilTx:         false,
			expectedCount: 0,
		},
		{
			name:          "commit persists the fact",
			commit:        true,
			nilTx:         false,
			expectedCount: 1,
		},
		{
			name:          "nil tx rejected",
			commit:        false,
			nilTx:         true,
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, gormDB := openAuditTx(t)
			ctx := context.Background()

			if tc.nilTx {
				assert.ErrorIs(t, logger.RecordTx(ctx, nil, auditEntry()), audit.ErrConfigRequired)
				return
			}

			tx := gormDB.WithContext(ctx).Begin()
			require.NoError(t, tx.Error)
			require.NoError(t, logger.RecordTx(ctx, tx, auditEntry()))

			if tc.commit {
				require.NoError(t, tx.Commit().Error)
			} else {
				require.NoError(t, tx.Rollback().Error)
			}

			entries, err := logger.Entries(ctx, auditTenant)
			require.NoError(t, err)
			assert.Len(t, entries, tc.expectedCount)
		})
	}
}

func TestAuditVerifySubMicrosecondTimestamps(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	type testCase struct {
		name       string
		nanosecond int
	}

	testCases := []testCase{
		{
			name:       "nanosecond timestamp verifies after microsecond round-trip",
			nanosecond: 123,
		},
		{
			name:       "whole second still verifies",
			nanosecond: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := openAuditLogger(t)
			ctx := context.Background()

			entry := auditEntry()
			entry.OccurredAt = time.Date(2026, 9, 23, 0, 0, 0, tc.nanosecond, time.UTC)
			require.NoError(t, logger.Record(ctx, entry))

			require.NoError(t, logger.Verify(ctx, auditTenant))
		})
	}
}

func TestAuditCheckpointSurvivesAppends(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	logger := openAuditLogger(t)
	ctx := context.Background()

	require.NoError(t, logger.Record(ctx, auditEntry()))

	second := auditEntry()
	second.Action = "erase"
	require.NoError(t, logger.Record(ctx, second))

	root, err := logger.Checkpoint(ctx, auditTenant)
	require.NoError(t, err)

	third := auditEntry()
	third.Action = "export"
	require.NoError(t, logger.Record(ctx, third))

	assert.NoError(t, logger.VerifyCheckpoint(ctx, root))
	assert.NoError(t, logger.Verify(ctx, auditTenant))
}

func TestAuditConcurrentAppendsSerialize(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	const writers = 4
	const perWriter = 3

	logger := openAuditLogger(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errCh := make(chan error, writers*perWriter)

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				entry := auditEntry()
				entry.Action = "append"
				entry.Resource = "w:" + strconv.Itoa(w) + "/i:" + strconv.Itoa(i)
				if err := logger.Record(ctx, entry); err != nil {
					errCh <- err
					return
				}
			}
		}(w)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		assert.NoError(t, err)
	}

	entries, err := logger.Entries(ctx, auditTenant)
	require.NoError(t, err)
	require.Len(t, entries, writers*perWriter)

	for i, entry := range entries {
		assert.Equal(t, int64(i+1), entry.Seq)
	}

	assert.NoError(t, logger.Verify(ctx, auditTenant))
}

type failingWORM struct {
	err error
}

func (f failingWORM) ArchiveCheckpoint(_ context.Context, _ audit.SignedRoot) error {
	return f.err
}

func TestAuditCheckpointWormFailureSurfaced(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	logger, db := openAuditTx(t)
	ctx := context.Background()

	require.NoError(t, logger.Record(ctx, auditEntry()))

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	boom := errors.New("worm unavailable")
	wormLogger, err := audit.NewLogger(audit.AuditParams{
		DB:         db,
		SigningKey: []byte("0123456789abcdef0123456789abcdef"),
		Clock:      clk,
		WORM:       failingWORM{err: boom},
	})
	require.NoError(t, err)

	_, err = wormLogger.Checkpoint(ctx, auditTenant)
	assert.ErrorIs(t, err, boom)

	// The durable append is unaffected by the exporter failure.
	assert.NoError(t, logger.Verify(ctx, auditTenant))
}

func TestAuditSwappedHashesDetected(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	logger, db := openAuditTx(t)
	ctx := context.Background()

	require.NoError(t, logger.Record(ctx, auditEntry()))

	second := auditEntry()
	second.Action = "erase"
	require.NoError(t, logger.Record(ctx, second))

	// Corrupt one stored entry hash directly: content-level tampering the
	// Merkle root and chain recomputation must still catch.
	require.NoError(t, db.WithContext(ctx).Exec(
		"UPDATE audit_entries SET entry_hash = 'deadbeef' WHERE seq = 2 AND tenant_id = ?",
		auditTenant,
	).Error)

	assert.ErrorIs(t, logger.Verify(ctx, auditTenant), audit.ErrChainBroken)
}
