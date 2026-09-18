package persistence_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/migration"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/outbox"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/posting"
	repos "github.com/kadekutama/go-template/internal/infrastructure/database/postgres/repositories"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/rls"
	"github.com/kadekutama/go-template/internal/infrastructure/database/seed"
	"github.com/kadekutama/go-template/test/testcontainers"
)

// openMigrated boots PostgreSQL, applies all migrations, and returns handles.
// Skips cleanly without a Docker daemon.
func openMigrated(t *testing.T) (*postgres.Pools, *sql.DB) {
	t.Helper()
	testcontainers.SkipIfNoDocker(t)

	ctx := context.Background()

	handle, err := testcontainers.StartPostgres(t, "ledger_g4")
	require.NoError(t, err)

	sqlDB, err := sql.Open("pgx", handle.ConnectionString())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	runner, err := migration.NewRunner(migration.RunnerParams{DB: sqlDB})
	require.NoError(t, err)
	require.NoError(t, runner.Up(ctx))

	version, err := runner.Version(ctx)
	require.NoError(t, err)
	require.Positive(t, version)

	gormDB, err := postgres.Open(postgres.DefaultConfig(handle.ConnectionString()))
	require.NoError(t, err)

	pools, err := postgres.NewPools(postgres.PoolsParams{Primary: gormDB})
	require.NoError(t, err)

	return pools, sqlDB
}

func TestMigrationsUpDown(t *testing.T) {
	t.Parallel()
	testcontainers.SkipIfNoDocker(t)

	ctx := context.Background()
	handle, err := testcontainers.StartPostgres(t, "ledger_mig")
	require.NoError(t, err)

	sqlDB, err := sql.Open("pgx", handle.ConnectionString())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	runner, err := migration.NewRunner(migration.RunnerParams{DB: sqlDB})
	require.NoError(t, err)

	type testCase struct {
		name            string
		action          string
		expectedVersion int64
	}

	testCases := []testCase{
		{
			name:            "initial up builds full schema to version 20260901000005",
			action:          "UP",
			expectedVersion: 20260901000005,
		},
		{
			name:            "down removes latest migration cleanly to version 20260901000004",
			action:          "DOWN",
			expectedVersion: 20260901000004,
		},
		{
			name:            "re-up rebuilds latest migration cleanly back to version 20260901000005",
			action:          "UP",
			expectedVersion: 20260901000005,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.action {
			case "UP":
				require.NoError(t, runner.Up(ctx))
			case "DOWN":
				require.NoError(t, runner.Down(ctx))
			}

			version, err := runner.Version(ctx)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedVersion, version)
		})
	}
}

func TestLedgerCRUD(t *testing.T) {
	t.Parallel()

	pools, _ := openMigrated(t)
	ctx := context.Background()

	ledgerRepo, err := repos.NewLedgerRepository(repos.LedgerRepositoryParams{DB: pools.Primary})
	require.NoError(t, err)

	type testCase struct {
		name           string
		tenantID       valueobject.TenantID
		otherTenantID  valueobject.TenantID
		ledgerID       valueobject.LedgerID
		ledgerName     string
		ledgerAlias    string
		baseAsset      valueobject.AssetCode
		expectConflict bool
	}

	testCases := []testCase{
		{
			name:           "create and find ledger within same tenant",
			tenantID:       "10000000-0000-4000-8000-000000000001",
			otherTenantID:  "10000000-0000-4000-8000-000000000099",
			ledgerID:       "20000000-0000-4000-8000-000000000001",
			ledgerName:     "Test Primary",
			ledgerAlias:    "test-primary",
			baseAsset:      "USD",
			expectConflict: false,
		},
		{
			name:           "second ledger in different tenant creates cleanly",
			tenantID:       "10000000-0000-4000-8000-000000000002",
			otherTenantID:  "10000000-0000-4000-8000-000000000099",
			ledgerID:       "20000000-0000-4000-8000-000000000002",
			ledgerName:     "Test Secondary",
			ledgerAlias:    "test-secondary",
			baseAsset:      "EUR",
			expectConflict: false,
		},
		{
			name:           "duplicate ledger creation returns conflict error",
			tenantID:       "10000000-0000-4000-8000-000000000001",
			otherTenantID:  "10000000-0000-4000-8000-000000000099",
			ledgerID:       "20000000-0000-4000-8000-000000000001",
			ledgerName:     "Test Duplicate",
			ledgerAlias:    "test-duplicate",
			baseAsset:      "USD",
			expectConflict: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ledger, err := entity.NewLedger(tc.ledgerID, tc.tenantID, tc.ledgerName, tc.baseAsset, "v1")
			require.NoError(t, err)
			ledger.Alias = tc.ledgerAlias

			stored, createErr := ledgerRepo.Create(ctx, ledger)
			if tc.expectConflict {
				require.Error(t, createErr)
				assert.True(t, errors.Is(createErr, repos.ErrConflict))
				return
			}
			require.NoError(t, createErr)
			assert.Equal(t, tc.ledgerID, stored.ID)
			assert.Equal(t, tc.ledgerAlias, stored.Alias)

			found, err := ledgerRepo.FindByID(ctx, tc.tenantID, tc.ledgerID)
			require.NoError(t, err)
			assert.Equal(t, tc.ledgerName, found.Name)

			_, err = ledgerRepo.FindByID(ctx, tc.otherTenantID, tc.ledgerID)
			require.Error(t, err, "cross-tenant read must not resolve")
		})
	}
}

func TestEntrySequencesMonotonic(t *testing.T) {
	t.Parallel()

	pools, sqlDB := openMigrated(t)
	ctx := context.Background()

	_, err := sqlDB.ExecContext(ctx, `INSERT INTO assets (code) VALUES ('USD') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO ledgers (id, tenant_id, name, alias, base_asset, chart_version)
		VALUES ('90000000-0000-4000-8000-000000000011', '90000000-0000-4000-8000-000000000001', 'Seq', 'seq', 'USD', 'v1') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
		VALUES ('77777777-7777-4777-8777-777777777777', '90000000-0000-4000-8000-000000000001', '90000000-0000-4000-8000-000000000011', '1000', 'Cash', 'ASSET', 'USD', 'ACTIVE'),
		       ('77777777-7777-4777-8777-777777777778', '90000000-0000-4000-8000-000000000001', '90000000-0000-4000-8000-000000000011', '2000', 'Payable', 'LIABILITY', 'USD', 'ACTIVE'),
		       ('77777777-7777-4777-8777-777777777779', '90000000-0000-4000-8000-000000000001', '90000000-0000-4000-8000-000000000011', '3000', 'Reserve', 'ASSET', 'USD', 'ACTIVE')
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	postingRepo, err := repos.NewPostingRepository(repos.PostingRepositoryParams{DB: pools.Primary})
	require.NoError(t, err)

	entryStore, err := repos.NewEntryStore(repos.EntryStoreParams{DB: pools.Primary})
	require.NoError(t, err)

	commitPosting := func(postingID, entryA, entryB, acctA, acctB string) {
		now := time.Now()
		_, err := postingRepo.Commit(ctx, entity.PostingData{
			ID:          valueobject.PostingID(postingID),
			TenantID:    "90000000-0000-4000-8000-000000000001",
			LedgerID:    "90000000-0000-4000-8000-000000000011",
			Operation:   "TRANSFER",
			Description: "seq probe",
			Entries: []entity.Entry{
				{
					ID: valueobject.EntryID(entryA), PostingID: valueobject.PostingID(postingID),
					AccountID: valueobject.AccountID(acctA), Side: valueobject.DirectionDebit, AmountMinor: 10, AssetCode: "USD",
				},
				{
					ID: valueobject.EntryID(entryB), PostingID: valueobject.PostingID(postingID),
					AccountID: valueobject.AccountID(acctB), Side: valueobject.DirectionCredit, AmountMinor: 10, AssetCode: "USD",
				},
			},
			EffectiveAt: now,
			RecordedAt:  now,
		})
		require.NoError(t, err)
	}

	commitPosting("44444444-4444-4444-8444-444444444444", "55555555-5555-4555-8555-555555555555", "66666666-6666-4666-8666-666666666666", "77777777-7777-4777-8777-777777777777", "77777777-7777-4777-8777-777777777778")
	commitPosting("77777777-7777-4777-8777-777777777777", "88888888-8888-4888-8888-888888888888", "99999999-9999-4999-8999-999999999999", "77777777-7777-4777-8777-777777777777", "77777777-7777-4777-8777-777777777778")
	commitPosting("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "cccccccc-cccc-4ccc-8ccc-cccccccccccc", "77777777-7777-4777-8777-777777777779", "77777777-7777-4777-8777-777777777778")

	type testCase struct {
		name             string
		accountID        string
		expectedCount    int
		expectedFirstSeq int64
		expectedFinalSeq int64
	}

	testCases := []testCase{
		{
			name:             "account s-1 has 2 entries with monotonic sequences 1 and 2",
			accountID:        "77777777-7777-4777-8777-777777777777",
			expectedCount:    2,
			expectedFirstSeq: 1,
			expectedFinalSeq: 2,
		},
		{
			name:             "account s-2 participated in all 3 postings with monotonic sequences 1, 2, 3",
			accountID:        "77777777-7777-4777-8777-777777777778",
			expectedCount:    3,
			expectedFirstSeq: 1,
			expectedFinalSeq: 3,
		},
		{
			name:             "independent account s-3 starts at sequence 1 despite prior ledger postings",
			accountID:        "77777777-7777-4777-8777-777777777779",
			expectedCount:    1,
			expectedFirstSeq: 1,
			expectedFinalSeq: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entries, next, err := entryStore.FindByAccount(ctx, "90000000-0000-4000-8000-000000000001", valueobject.AccountID(tc.accountID), "", 10)
			require.NoError(t, err)
			assert.Empty(t, next)
			require.Len(t, entries, tc.expectedCount)
			assert.Equal(t, tc.expectedFirstSeq, entries[0].AccountSeq)
			assert.Equal(t, tc.expectedFinalSeq, entries[len(entries)-1].AccountSeq)

			for i := 0; i < len(entries); i++ {
				assert.Equal(t, int64(i+1), entries[i].AccountSeq, "sequence must strictly equal 1-based index")
			}
		})
	}
}

func TestPostingAtomicity(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		creditMinor   int64
		expectError   bool
		expectPersist bool
	}

	testCases := []testCase{
		{
			name:          "balanced posting commits",
			creditMinor:   100,
			expectError:   false,
			expectPersist: true,
		},
		{
			name:          "unbalanced posting rejected with nothing persisted",
			creditMinor:   99,
			expectError:   true,
			expectPersist: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pools, sqlDB := openMigrated(t)
			ctx := context.Background()

			_, err := sqlDB.ExecContext(ctx, `INSERT INTO assets (code) VALUES ('USD') ON CONFLICT DO NOTHING`)
			require.NoError(t, err)
			_, err = sqlDB.ExecContext(ctx, `INSERT INTO ledgers (id, tenant_id, name, alias, base_asset, chart_version)
				VALUES ('90000000-0000-4000-8000-000000000012','90000000-0000-4000-8000-000000000002','Atomic','atomic','USD','v1') ON CONFLICT DO NOTHING`)
			require.NoError(t, err)
			_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
				VALUES ('90000000-0000-4000-8000-000000000021','90000000-0000-4000-8000-000000000002','90000000-0000-4000-8000-000000000012','1000','Cash','ASSET','USD','ACTIVE') ON CONFLICT DO NOTHING`)
			require.NoError(t, err)
			_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
				VALUES ('90000000-0000-4000-8000-000000000022','90000000-0000-4000-8000-000000000002','90000000-0000-4000-8000-000000000012','2000','Payable','LIABILITY','USD','ACTIVE') ON CONFLICT DO NOTHING`)
			require.NoError(t, err)

			postingRepo, err := repos.NewPostingRepository(repos.PostingRepositoryParams{DB: pools.Primary})
			require.NoError(t, err)

			now := time.Now()

			posting := entity.PostingData{
				ID:          valueobject.PostingID("11111111-1111-4111-8111-111111111111"),
				TenantID:    "90000000-0000-4000-8000-000000000002",
				LedgerID:    "90000000-0000-4000-8000-000000000012",
				Operation:   "TRANSFER",
				Description: "atomicity probe",
				Entries: []entity.Entry{
					{
						ID: valueobject.EntryID("22222222-2222-4222-8222-222222222222"), PostingID: "11111111-1111-4111-8111-111111111111",
						AccountID: "90000000-0000-4000-8000-000000000021", Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: "USD", AccountSeq: 1,
					},
					{
						ID: valueobject.EntryID("33333333-3333-4333-8333-333333333333"), PostingID: "11111111-1111-4111-8111-111111111111",
						AccountID: "90000000-0000-4000-8000-000000000022", Side: valueobject.DirectionCredit, AmountMinor: tc.creditMinor, AssetCode: "USD", AccountSeq: 1,
					},
				},
				EffectiveAt: now,
				RecordedAt:  now,
			}

			_, err = postingRepo.Commit(ctx, posting)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			_, findErr := postingRepo.FindByID(ctx, "90000000-0000-4000-8000-000000000002", posting.ID)
			if tc.expectPersist {
				require.NoError(t, findErr)
			} else {
				require.Error(t, findErr, "failed posting must persist nothing")
			}
		})
	}
}

func TestCommitScopeEnforcement(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		setup       string
		accountID   string
		expectError string
	}

	testCases := []testCase{
		{
			name:        "unknown account rejected",
			setup:       "base",
			accountID:   "90000000-0000-4000-8000-000000000099",
			expectError: "unknown account",
		},
		{
			name:        "frozen account rejected",
			setup:       "frozen",
			accountID:   "90000000-0000-4000-8000-000000000024",
			expectError: "not ACTIVE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pools, sqlDB := openMigrated(t)
			ctx := context.Background()

			_, err := sqlDB.ExecContext(ctx, `INSERT INTO assets (code) VALUES ('USD') ON CONFLICT DO NOTHING`)
			require.NoError(t, err)
			_, err = sqlDB.ExecContext(ctx, `INSERT INTO ledgers (id, tenant_id, name, alias, base_asset, chart_version)
				VALUES ('90000000-0000-4000-8000-000000000013','90000000-0000-4000-8000-000000000003','Scope','scope','USD','v1') ON CONFLICT DO NOTHING`)
			require.NoError(t, err)
			_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
				VALUES ('90000000-0000-4000-8000-000000000023','90000000-0000-4000-8000-000000000003','90000000-0000-4000-8000-000000000013','1000','Cash','ASSET','USD','ACTIVE') ON CONFLICT DO NOTHING`)
			require.NoError(t, err)

			if tc.setup == "frozen" {
				_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
					VALUES ('90000000-0000-4000-8000-000000000024','90000000-0000-4000-8000-000000000003','90000000-0000-4000-8000-000000000013','1001','Frozen','ASSET','USD','FROZEN') ON CONFLICT DO NOTHING`)
				require.NoError(t, err)
			}

			postingRepo, err := repos.NewPostingRepository(repos.PostingRepositoryParams{DB: pools.Primary})
			require.NoError(t, err)

			now := time.Now()
			_, err = postingRepo.Commit(ctx, entity.PostingData{
				ID:          valueobject.PostingID("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"),
				TenantID:    "90000000-0000-4000-8000-000000000003",
				LedgerID:    "90000000-0000-4000-8000-000000000013",
				Operation:   "TRANSFER",
				Description: "scope probe",
				Entries: []entity.Entry{
					{
						ID: valueobject.EntryID("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"), PostingID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
						AccountID: "90000000-0000-4000-8000-000000000023", Side: valueobject.DirectionDebit, AmountMinor: 10, AssetCode: "USD",
					},
					{
						ID: valueobject.EntryID("cccccccc-cccc-4ccc-8ccc-cccccccccccc"), PostingID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
						AccountID: valueobject.AccountID(tc.accountID), Side: valueobject.DirectionCredit, AmountMinor: 10, AssetCode: "USD",
					},
				},
				EffectiveAt: now,
				RecordedAt:  now,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectError)
		})
	}
}

func TestPostingSearchEndToEnd(t *testing.T) {
	t.Parallel()

	pools, sqlDB := openMigrated(t)
	ctx := context.Background()

	_, err := sqlDB.ExecContext(ctx, `INSERT INTO assets (code) VALUES ('USD') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO ledgers (id, tenant_id, name, alias, base_asset, chart_version)
		VALUES ('90000000-0000-4000-8000-000000000014', '90000000-0000-4000-8000-000000000004', 'Search', 'search', 'USD', 'v1') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
		VALUES ('90000000-0000-4000-8000-000000000025', '90000000-0000-4000-8000-000000000004', '90000000-0000-4000-8000-000000000014', '1000', 'Cash', 'ASSET', 'USD', 'ACTIVE'),
		       ('90000000-0000-4000-8000-000000000026', '90000000-0000-4000-8000-000000000004', '90000000-0000-4000-8000-000000000014', '2000', 'Payable', 'LIABILITY', 'USD', 'ACTIVE'),
		       ('90000000-0000-4000-8000-000000000027', '90000000-0000-4000-8000-000000000004', '90000000-0000-4000-8000-000000000014', '3000', 'Reserve', 'ASSET', 'USD', 'ACTIVE')
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	postingRepo, err := repos.NewPostingRepository(repos.PostingRepositoryParams{DB: pools.Primary})
	require.NoError(t, err)

	now := time.Now()
	_, err = postingRepo.Commit(ctx, entity.PostingData{
		ID:          valueobject.PostingID("dddddddd-dddd-4ddd-8ddd-dddddddddddd"),
		TenantID:    "90000000-0000-4000-8000-000000000004",
		LedgerID:    "90000000-0000-4000-8000-000000000014",
		Operation:   "FUNDING",
		Description: "search probe funding",
		Entries: []entity.Entry{
			{
				ID: valueobject.EntryID("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"), PostingID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
				AccountID: "90000000-0000-4000-8000-000000000025", Side: valueobject.DirectionDebit, AmountMinor: 50, AssetCode: "USD",
			},
			{
				ID: valueobject.EntryID("ffffffff-ffff-4fff-8fff-ffffffffffff"), PostingID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
				AccountID: "90000000-0000-4000-8000-000000000026", Side: valueobject.DirectionCredit, AmountMinor: 50, AssetCode: "USD",
			},
		},
		EffectiveAt: now,
		RecordedAt:  now,
	})
	require.NoError(t, err)

	_, err = postingRepo.Commit(ctx, entity.PostingData{
		ID:          valueobject.PostingID("12121212-1212-4212-8212-121212121212"),
		TenantID:    "90000000-0000-4000-8000-000000000004",
		LedgerID:    "90000000-0000-4000-8000-000000000014",
		Operation:   "TRANSFER",
		Description: "search probe transfer",
		Entries: []entity.Entry{
			{
				ID: valueobject.EntryID("23232323-2323-4323-8323-232323232323"), PostingID: "12121212-1212-4212-8212-121212121212",
				AccountID: "90000000-0000-4000-8000-000000000026", Side: valueobject.DirectionDebit, AmountMinor: 75, AssetCode: "USD",
			},
			{
				ID: valueobject.EntryID("34343434-3434-4343-8343-343434343434"), PostingID: "12121212-1212-4212-8212-121212121212",
				AccountID: "90000000-0000-4000-8000-000000000027", Side: valueobject.DirectionCredit, AmountMinor: 75, AssetCode: "USD",
			},
		},
		EffectiveAt: now.Add(time.Minute),
		RecordedAt:  now.Add(time.Minute),
	})
	require.NoError(t, err)

	search, err := repos.NewPostingSearch(repos.PostingSearchParams{DB: pools.Primary})
	require.NoError(t, err)

	type testCase struct {
		name          string
		ctx           context.Context
		filter        port.PostingFilter
		expectedCount int
		expectedError string
	}

	testCases := []testCase{
		{
			name: "operation filter FUNDING returns 1 matching posting",
			ctx:  ctx,
			filter: port.PostingFilter{
				TenantID:  "90000000-0000-4000-8000-000000000004",
				Operation: "FUNDING",
				Limit:     10,
			},
			expectedCount: 1,
			expectedError: "",
		},
		{
			name: "operation filter TRANSFER returns 1 matching posting",
			ctx:  ctx,
			filter: port.PostingFilter{
				TenantID:  "90000000-0000-4000-8000-000000000004",
				Operation: "TRANSFER",
				Limit:     10,
			},
			expectedCount: 1,
			expectedError: "",
		},
		{
			name: "unmatched operation REFUND returns 0 postings",
			ctx:  ctx,
			filter: port.PostingFilter{
				TenantID:  "90000000-0000-4000-8000-000000000004",
				Operation: "REFUND",
				Limit:     10,
			},
			expectedCount: 0,
			expectedError: "",
		},
		{
			name: "account filter q-1 returns only its 1 participating posting",
			ctx:  ctx,
			filter: port.PostingFilter{
				TenantID:  "90000000-0000-4000-8000-000000000004",
				AccountID: "90000000-0000-4000-8000-000000000025",
				Limit:     10,
			},
			expectedCount: 1,
			expectedError: "",
		},
		{
			name: "account filter q-2 participating in both postings returns 2 postings",
			ctx:  ctx,
			filter: port.PostingFilter{
				TenantID:  "90000000-0000-4000-8000-000000000004",
				AccountID: "90000000-0000-4000-8000-000000000026",
				Limit:     10,
			},
			expectedCount: 2,
			expectedError: "",
		},
		{
			name: "wildcard search without operation or account returns all 2 postings",
			ctx:  ctx,
			filter: port.PostingFilter{
				TenantID: "90000000-0000-4000-8000-000000000004",
				Limit:    10,
			},
			expectedCount: 2,
			expectedError: "",
		},
		{
			name: "other tenant search returns 0 postings enforcing tenant isolation",
			ctx:  ctx,
			filter: port.PostingFilter{
				TenantID: "90000000-0000-4000-8000-000000000094",
				Limit:    10,
			},
			expectedCount: 0,
			expectedError: "",
		},
		{
			name: "missing tenant ID returns validation error",
			ctx:  ctx,
			filter: port.PostingFilter{
				Limit: 10,
			},
			expectedCount: 0,
			expectedError: "postgres: tenant is required",
		},
		{
			name: "amount filter returns unsupported error",
			ctx:  ctx,
			filter: port.PostingFilter{
				TenantID:  "90000000-0000-4000-8000-000000000004",
				MinAmount: 10,
				Limit:     10,
			},
			expectedCount: 0,
			expectedError: "postgres: amount bounds need the reporting read model",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := search.Search(tc.ctx, tc.filter)
			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Len(t, page.Postings, tc.expectedCount)
			if tc.filter.Operation != "" && len(page.Postings) > 0 {
				assert.Equal(t, tc.filter.Operation, page.Postings[0].Operation)
			}
		})
	}
}

func TestDirectSQLInvariantAttacks(t *testing.T) {
	t.Parallel()

	pools, sqlDB := openMigrated(t)
	ctx := context.Background()

	_, err := sqlDB.ExecContext(ctx, `INSERT INTO assets (code) VALUES ('USD') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO ledgers (id, tenant_id, name, alias, base_asset, chart_version)
		VALUES ('90000000-0000-4000-8000-000000000015','90000000-0000-4000-8000-000000000005','Attack','attack','USD','v1') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
		VALUES ('90000000-0000-4000-8000-000000000028','90000000-0000-4000-8000-000000000005','90000000-0000-4000-8000-000000000015','1000','Cash','ASSET','USD','ACTIVE') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
		VALUES ('90000000-0000-4000-8000-000000000029','90000000-0000-4000-8000-000000000005','90000000-0000-4000-8000-000000000015','2000','Payable','LIABILITY','USD','ACTIVE') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	postingRepo, err := repos.NewPostingRepository(repos.PostingRepositoryParams{DB: pools.Primary})
	require.NoError(t, err)

	now := time.Now()
	_, err = postingRepo.Commit(ctx, entity.PostingData{
		ID:          valueobject.PostingID("11111111-2222-4333-8444-555555555555"),
		TenantID:    "90000000-0000-4000-8000-000000000005",
		LedgerID:    "90000000-0000-4000-8000-000000000015",
		Operation:   "TRANSFER",
		Description: "immutable target",
		Entries: []entity.Entry{
			{
				ID: valueobject.EntryID("aaaa1111-2222-4333-8444-555555555555"), PostingID: "11111111-2222-4333-8444-555555555555",
				AccountID: "90000000-0000-4000-8000-000000000028", Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: "USD",
			},
			{
				ID: valueobject.EntryID("bbbb1111-2222-4333-8444-555555555555"), PostingID: "11111111-2222-4333-8444-555555555555",
				AccountID: "90000000-0000-4000-8000-000000000029", Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: "USD",
			},
		},
		EffectiveAt: now,
		RecordedAt:  now,
	})
	require.NoError(t, err)

	type testCase struct {
		name          string
		attackSQL     string
		args          []any
		inTransaction bool
		expectedError string
	}

	testCases := []testCase{
		{
			name:          "direct update on postings rejected by trigger",
			attackSQL:     `UPDATE postings SET operation = 'CORRUPTED' WHERE id = $1`,
			args:          []any{"11111111-2222-4333-8444-555555555555"},
			inTransaction: false,
			expectedError: "posted facts are immutable",
		},
		{
			name:          "direct delete on postings rejected by trigger",
			attackSQL:     `DELETE FROM postings WHERE id = $1`,
			args:          []any{"11111111-2222-4333-8444-555555555555"},
			inTransaction: false,
			expectedError: "posted facts are immutable",
		},
		{
			name:          "direct update on entries rejected by trigger",
			attackSQL:     `UPDATE entries SET amount_minor = 999999 WHERE id = $1`,
			args:          []any{"aaaa1111-2222-4333-8444-555555555555"},
			inTransaction: false,
			expectedError: "posted facts are immutable",
		},
		{
			name:          "direct delete on entries rejected by trigger",
			attackSQL:     `DELETE FROM entries WHERE id = $1`,
			args:          []any{"aaaa1111-2222-4333-8444-555555555555"},
			inTransaction: false,
			expectedError: "posted facts are immutable",
		},
		{
			name: "unbalanced entries rejected on transaction commit by constraint trigger",
			attackSQL: `INSERT INTO entries (id, posting_id, tenant_id, ledger_id, account_id, side, amount_minor, asset_code, account_seq)
				VALUES ('90000000-0000-4000-8000-000000000035', '22222222-3333-4444-8555-666666666666', '90000000-0000-4000-8000-000000000005', '90000000-0000-4000-8000-000000000015', '90000000-0000-4000-8000-000000000028', 'DEBIT', 500, 'USD', 2)`,
			args:          nil,
			inTransaction: true,
			expectedError: "debits must equal credits per asset",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.inTransaction {
				tx, err := sqlDB.BeginTx(ctx, nil)
				require.NoError(t, err)
				defer func() { _ = tx.Rollback() }()

				_, err = tx.ExecContext(ctx, `INSERT INTO postings (id, tenant_id, ledger_id, operation, effective_at)
					VALUES ('22222222-3333-4444-8555-666666666666', '90000000-0000-4000-8000-000000000005', '90000000-0000-4000-8000-000000000015', 'TRANSFER', now()) ON CONFLICT DO NOTHING`)
				require.NoError(t, err)

				_, err = tx.ExecContext(ctx, tc.attackSQL, tc.args...)
				require.NoError(t, err)

				commitErr := tx.Commit()
				require.Error(t, commitErr)
				assert.Contains(t, commitErr.Error(), tc.expectedError)
			} else {
				_, err := sqlDB.ExecContext(ctx, tc.attackSQL, tc.args...)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			}
		})
	}
}

func TestDurableIdempotencyPostgres(t *testing.T) {
	t.Parallel()

	pools, _ := openMigrated(t)
	ctx := context.Background()

	uow, err := posting.NewUnitOfWork(posting.UnitOfWorkParams{DB: pools.Primary})
	require.NoError(t, err)

	type testCase struct {
		name            string
		tenantID        valueobject.TenantID
		key             string
		fingerprint     string
		completePayload []byte
		secondTenantID  valueobject.TenantID
		secondKey       string
		secondFP        string
		expectConflict  bool
		expectReplay    bool
	}

	testCases := []testCase{
		{
			name:            "identical fingerprint replays completed response",
			tenantID:        "90000000-0000-4000-8000-000000000051",
			key:             "idem-key-1",
			fingerprint:     "fp-orig-1",
			completePayload: []byte(`{"status":"SUCCESS","id":"pst-1"}`),
			secondTenantID:  "90000000-0000-4000-8000-000000000051",
			secondKey:       "idem-key-1",
			secondFP:        "fp-orig-1",
			expectConflict:  false,
			expectReplay:    true,
		},
		{
			name:            "altered fingerprint returns conflict",
			tenantID:        "90000000-0000-4000-8000-000000000052",
			key:             "idem-key-2",
			fingerprint:     "fp-orig-2",
			completePayload: []byte(`{"status":"SUCCESS"}`),
			secondTenantID:  "90000000-0000-4000-8000-000000000052",
			secondKey:       "idem-key-2",
			secondFP:        "fp-tampered-2",
			expectConflict:  true,
			expectReplay:    false,
		},
		{
			name:            "cross-tenant key is completely isolated",
			tenantID:        "90000000-0000-4000-8000-000000000053",
			key:             "shared-key",
			fingerprint:     "fp-a",
			completePayload: []byte(`{"tenant":"a"}`),
			secondTenantID:  "90000000-0000-4000-8000-000000000054",
			secondKey:       "shared-key",
			secondFP:        "fp-b",
			expectConflict:  false,
			expectReplay:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
				outcome, resErr := tx.Idempotency().Reserve(ctx, port.IdempotencyRecord{
					TenantID:    tc.tenantID,
					Key:         tc.key,
					Fingerprint: tc.fingerprint,
				})
				require.NoError(t, resErr)
				assert.False(t, outcome.Replay)
				assert.Nil(t, outcome.Response)

				return tx.Idempotency().Complete(ctx, tc.key, tc.completePayload)
			})
			require.NoError(t, err)

			err = uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
				outcome, resErr := tx.Idempotency().Reserve(ctx, port.IdempotencyRecord{
					TenantID:    tc.secondTenantID,
					Key:         tc.secondKey,
					Fingerprint: tc.secondFP,
				})
				if tc.expectConflict {
					var domainErr *entity.Error
					require.ErrorAs(t, resErr, &domainErr)
					assert.Equal(t, "IDEMPOTENCY_CONFLICT", domainErr.Code)
					return nil
				}

				require.NoError(t, resErr)
				assert.Equal(t, tc.expectReplay, outcome.Replay)
				if tc.expectReplay {
					assert.Equal(t, tc.completePayload, outcome.Response)
				} else {
					assert.Nil(t, outcome.Response)
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
}

type testSubscriber struct {
	mu        sync.Mutex
	published []port.OutboxFact
}

func (s *testSubscriber) Publish(_ context.Context, facts ...port.OutboxFact) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.published = append(s.published, facts...)
	return nil
}

func TestOutboxRelayPostgres(t *testing.T) {
	t.Parallel()

	pools, sqlDB := openMigrated(t)
	ctx := context.Background()

	sub := &testSubscriber{}

	poller, err := outbox.NewPoller(outbox.PollerParams{
		Publisher: sub,
		MaxBatch:  50,
	})
	require.NoError(t, err)

	writer, err := outbox.NewWriter(outbox.WriterParams{DB: pools.Primary})
	require.NoError(t, err)

	now := time.Now()
	facts := []port.OutboxFact{
		{
			TenantID:    "90000000-0000-4000-8000-000000000055",
			LedgerID:    "90000000-0000-4000-8000-000000000065",
			EventType:   "transfer.created.v1",
			AggregateID: "agg-ob-1",
			Payload:     []byte(`{"amount":100}`),
			OccurredAt:  now,
		},
		{
			TenantID:    "90000000-0000-4000-8000-000000000055",
			LedgerID:    "90000000-0000-4000-8000-000000000065",
			EventType:   "transfer.completed.v1",
			AggregateID: "agg-ob-1",
			Payload:     []byte(`{"amount":100}`),
			OccurredAt:  now,
		},
	}

	err = pools.Primary.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return writer.AppendTx(ctx, tx, facts...)
	})
	require.NoError(t, err)

	delivered, err := poller.RunOnce(ctx, pools.Primary)
	require.NoError(t, err)
	assert.Equal(t, 2, delivered)

	sub.mu.Lock()
	require.Len(t, sub.published, 2)
	assert.Equal(t, "transfer.created.v1", sub.published[0].EventType)
	assert.Equal(t, "transfer.completed.v1", sub.published[1].EventType)
	sub.mu.Unlock()

	var undeliveredCount int64
	err = sqlDB.QueryRowContext(ctx, "SELECT count(*) FROM outbox_events WHERE delivered_at IS NULL").Scan(&undeliveredCount)
	require.NoError(t, err)
	assert.Equal(t, int64(0), undeliveredCount)

	secondDelivered, err := poller.RunOnce(ctx, pools.Primary)
	require.NoError(t, err)
	assert.Equal(t, 0, secondDelivered)
}

func TestAdversarialRLS(t *testing.T) {
	t.Parallel()

	_, sqlDB := openMigrated(t)
	ctx := context.Background()

	// 1. Seed base assets, ledgers, and accounts for two distinct tenants
	_, err := sqlDB.ExecContext(ctx, `INSERT INTO assets (code) VALUES ('USD') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	_, err = sqlDB.ExecContext(ctx, `INSERT INTO ledgers (id, tenant_id, name, alias, base_asset, chart_version)
		VALUES ('90000000-0000-4000-8000-000000000016', '90000000-0000-4000-8000-000000000006', 'Ledger A', 'ledger-a', 'USD', 'v1'),
		       ('90000000-0000-4000-8000-000000000017', '90000000-0000-4000-8000-000000000007', 'Ledger B', 'ledger-b', 'USD', 'v1')
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
		VALUES ('90000000-0000-4000-8000-000000000046', '90000000-0000-4000-8000-000000000006', '90000000-0000-4000-8000-000000000016', '1000', 'Account A', 'ASSET', 'USD', 'ACTIVE'),
		       ('90000000-0000-4000-8000-000000000047', '90000000-0000-4000-8000-000000000007', '90000000-0000-4000-8000-000000000017', '2000', 'Account B', 'ASSET', 'USD', 'ACTIVE')
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	// Create standard application role without superuser or bypassrls privileges
	_, err = sqlDB.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'app_user') THEN
				CREATE ROLE app_user NOSUPERUSER NOBYPASSRLS;
			END IF;
		END
		$$;
		GRANT ALL ON ALL TABLES IN SCHEMA public TO app_user;
		GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO app_user;
	`)
	require.NoError(t, err)

	type testCase struct {
		name          string
		sessionTenant string
		bypassRole    bool
		querySQL      string
		args          []any
		expectedCount int64
		attemptInsert bool
		insertSQL     string
		expectError   string
	}

	testCases := []testCase{
		{
			name:          "tenant A cannot see tenant B account",
			sessionTenant: "90000000-0000-4000-8000-000000000006",
			bypassRole:    false,
			querySQL:      `SELECT count(*) FROM accounts WHERE id = '90000000-0000-4000-8000-000000000047'`,
			expectedCount: 0,
			attemptInsert: false,
		},
		{
			name:          "tenant A query count only sees own account",
			sessionTenant: "90000000-0000-4000-8000-000000000006",
			bypassRole:    false,
			querySQL:      `SELECT count(*) FROM accounts`,
			expectedCount: 1,
			attemptInsert: false,
		},
		{
			name:          "tenant B query count only sees own account",
			sessionTenant: "90000000-0000-4000-8000-000000000007",
			bypassRole:    false,
			querySQL:      `SELECT count(*) FROM accounts`,
			expectedCount: 1,
			attemptInsert: false,
		},
		{
			name:          "service role without app_user bypasses RLS and sees all accounts",
			sessionTenant: "90000000-0000-4000-8000-000000000006",
			bypassRole:    true,
			querySQL:      `SELECT count(*) FROM accounts`,
			expectedCount: 2,
			attemptInsert: false,
		},
		{
			name:          "tenant A cannot insert account for tenant B",
			sessionTenant: "90000000-0000-4000-8000-000000000006",
			bypassRole:    false,
			attemptInsert: true,
			insertSQL: `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
				VALUES ('90000000-0000-4000-8000-000000000048', '90000000-0000-4000-8000-000000000007', '90000000-0000-4000-8000-000000000017', '3000', 'Hacked', 'ASSET', 'USD', 'ACTIVE')`,
			expectError: "violates row-level security policy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := sqlDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()

			if !tc.bypassRole {
				_, err = tx.ExecContext(ctx, "SET LOCAL ROLE app_user")
				require.NoError(t, err)
			}

			// Scope transaction to session tenant
			_, err = tx.ExecContext(ctx, fmt.Sprintf("SELECT set_config('%s', $1, true)", rls.SettingName), tc.sessionTenant)
			require.NoError(t, err)

			if tc.attemptInsert {
				_, err = tx.ExecContext(ctx, tc.insertSQL)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			} else {
				var count int64
				err = tx.QueryRowContext(ctx, tc.querySQL, tc.args...).Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedCount, count)
			}
		})
	}
}

func TestConcurrentPostingSpendSerialization(t *testing.T) {
	t.Parallel()

	pools, sqlDB := openMigrated(t)
	ctx := context.Background()

	_, err := sqlDB.ExecContext(ctx, `INSERT INTO assets (code) VALUES ('USD') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO ledgers (id, tenant_id, name, alias, base_asset, chart_version)
		VALUES ('90000000-0000-4000-8000-000000000018','90000000-0000-4000-8000-000000000008','Conc','conc','USD','v1') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO accounts (id, tenant_id, ledger_id, number, name, class, asset_code, status)
		VALUES ('90000000-0000-4000-8000-000000000031','90000000-0000-4000-8000-000000000008','90000000-0000-4000-8000-000000000018','1000','Cash','ASSET','USD','ACTIVE'),
		       ('90000000-0000-4000-8000-000000000032','90000000-0000-4000-8000-000000000008','90000000-0000-4000-8000-000000000018','2000','Payable','LIABILITY','USD','ACTIVE')
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	postingRepo, err := repos.NewPostingRepository(repos.PostingRepositoryParams{DB: pools.Primary})
	require.NoError(t, err)

	const numWorkers = 8
	var wg sync.WaitGroup
	errCh := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		workerID := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			now := time.Now()
			pID := fmt.Sprintf("c0000000-0000-4000-8000-%012d", workerID+1)
			eA := fmt.Sprintf("ca000000-0000-4000-8000-%012d", workerID+1)
			eB := fmt.Sprintf("cb000000-0000-4000-8000-%012d", workerID+1)

			_, err := postingRepo.Commit(ctx, entity.PostingData{
				ID:          valueobject.PostingID(pID),
				TenantID:    "90000000-0000-4000-8000-000000000008",
				LedgerID:    "90000000-0000-4000-8000-000000000018",
				Operation:   "TRANSFER",
				Description: fmt.Sprintf("concurrent worker %d", workerID),
				Entries: []entity.Entry{
					{
						ID: valueobject.EntryID(eA), PostingID: valueobject.PostingID(pID),
						AccountID: "90000000-0000-4000-8000-000000000031", Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: "USD",
					},
					{
						ID: valueobject.EntryID(eB), PostingID: valueobject.PostingID(pID),
						AccountID: "90000000-0000-4000-8000-000000000032", Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: "USD",
					},
				},
				EffectiveAt: now,
				RecordedAt:  now,
			})
			if err != nil {
				errCh <- fmt.Errorf("worker %d: %w", workerID, err)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err, "concurrent postings with deterministic lock order must not deadlock or fail")
	}

	// Verify all account sequences are continuous and monotonic
	entryStore, err := repos.NewEntryStore(repos.EntryStoreParams{DB: pools.Primary})
	require.NoError(t, err)

	entries, _, err := entryStore.FindByAccount(ctx, "90000000-0000-4000-8000-000000000008", "90000000-0000-4000-8000-000000000031", "", 20)
	require.NoError(t, err)
	require.Len(t, entries, numWorkers)

	for i, entry := range entries {
		assert.Equal(t, int64(i+1), entry.AccountSeq, "sequences must be strictly monotonic without gaps")
	}
}

func TestPostgresSeedEndToEnd(t *testing.T) {
	t.Parallel()

	pools, sqlDB := openMigrated(t)
	ctx := context.Background()

	type testCase struct {
		name             string
		ctx              context.Context
		db               *gorm.DB
		plan             seed.Plan
		expectedTenants  int64
		expectedLedgers  int64
		expectedAccounts int64
		expectedPostings int64
		expectedEntries  int64
		expectedError    error
	}

	testCases := []testCase{
		{
			name:             "initial seed applies tenant, ledger, accounts, and postings cleanly",
			ctx:              ctx,
			db:               pools.Primary,
			plan:             seed.DevPlan(),
			expectedTenants:  1,
			expectedLedgers:  1,
			expectedAccounts: 9,
			expectedPostings: 4,
			expectedEntries:  8,
			expectedError:    nil,
		},
		{
			name:             "re-applying seed is strictly idempotent without duplicate key conflicts",
			ctx:              ctx,
			db:               pools.Primary,
			plan:             seed.DevPlan(),
			expectedTenants:  1,
			expectedLedgers:  1,
			expectedAccounts: 9,
			expectedPostings: 4,
			expectedEntries:  8,
			expectedError:    nil,
		},
		{
			name: "nil database returns error",
			ctx:  ctx,
			db:   nil,
			plan: seed.DevPlan(),
			expectedError: func() error {
				return errors.New("postgres seed: DB is required")
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := seed.ApplyPostgres(tc.ctx, tc.db, tc.plan)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
				return
			}
			require.NoError(t, err)

			var tenantCount, ledgerCount, accountCount, postingCount, entryCount int64
			require.NoError(t, sqlDB.QueryRowContext(tc.ctx, `SELECT count(*) FROM tenants WHERE id = $1`, tc.plan.Tenant.TenantID).Scan(&tenantCount))
			require.NoError(t, sqlDB.QueryRowContext(tc.ctx, `SELECT count(*) FROM ledgers WHERE tenant_id = $1`, tc.plan.Tenant.TenantID).Scan(&ledgerCount))
			require.NoError(t, sqlDB.QueryRowContext(tc.ctx, `SELECT count(*) FROM accounts WHERE tenant_id = $1`, tc.plan.Tenant.TenantID).Scan(&accountCount))
			require.NoError(t, sqlDB.QueryRowContext(tc.ctx, `SELECT count(*) FROM postings WHERE tenant_id = $1`, tc.plan.Tenant.TenantID).Scan(&postingCount))
			require.NoError(t, sqlDB.QueryRowContext(tc.ctx, `SELECT count(*) FROM entries WHERE tenant_id = $1`, tc.plan.Tenant.TenantID).Scan(&entryCount))

			assert.Equal(t, tc.expectedTenants, tenantCount)
			assert.Equal(t, tc.expectedLedgers, ledgerCount)
			assert.Equal(t, tc.expectedAccounts, accountCount)
			assert.Equal(t, tc.expectedPostings, postingCount)
			assert.Equal(t, tc.expectedEntries, entryCount)

			// Verify tenant JSONB settings is valid and non-empty
			var settingsJSON string
			require.NoError(t, sqlDB.QueryRowContext(tc.ctx, `SELECT settings::text FROM tenants WHERE id = $1`, tc.plan.Tenant.TenantID).Scan(&settingsJSON))
			assert.Equal(t, "{}", settingsJSON)
		})
	}
}

func TestUUIDRoundTrip(t *testing.T) {
	t.Parallel()

	pools, sqlDB := openMigrated(t)
	ctx := context.Background()

	_, err := sqlDB.ExecContext(ctx, `INSERT INTO assets (code) VALUES ('USD') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	ledgerRepo, err := repos.NewLedgerRepository(repos.LedgerRepositoryParams{DB: pools.Primary})
	require.NoError(t, err)

	tenantID := valueobject.TenantID("90000000-0000-4000-8000-000000000071")

	ledger, err := entity.NewLedger(
		valueobject.LedgerID("90000000-0000-4000-8000-000000000072"),
		tenantID, "Roundtrip", "USD", "v1",
	)
	require.NoError(t, err)
	ledger.Alias = "roundtrip"

	storedLedger, err := ledgerRepo.Create(ctx, ledger)
	require.NoError(t, err)
	assert.Equal(t, ledger.ID, storedLedger.ID, "explicit IDs must round-trip unchanged")

	type testCase struct {
		name          string
		accountNumber string
	}

	testCases := []testCase{
		{
			name:          "omitted ID returns database-assigned canonical UUID",
			accountNumber: "009001",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var model models.AccountModel
			model.TenantID = tenantID.String()
			model.LedgerID = storedLedger.ID.String()
			model.Number = tc.accountNumber
			model.Name = "Roundtrip Account"
			model.Class = "ASSET"
			model.AssetCode = "USD"
			model.Status = "ACTIVE"
			model.Version = 1

			require.NoError(t, pools.Primary.WithContext(ctx).Create(&model).Error)
			require.NotEmpty(t, model.ID, "postgres must fill the default uuidv7() identity")

			parsed, parseErr := valueobject.ParseAccountID(model.ID)
			require.NoError(t, parseErr, "assigned ID must be a canonical UUID")
			assert.NotEqual(t, valueobject.AccountID(""), parsed)

			var reread models.AccountModel
			require.NoError(t, pools.Primary.WithContext(ctx).
				Where("tenant_id = ? AND id = ?", tenantID.String(), model.ID).
				First(&reread).Error)
			assert.Equal(t, model.ID, reread.ID)
			assert.Equal(t, tc.accountNumber, reread.Number)
		})
	}
}
