package repository_test

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockdomain "github.com/kadekutama/go-template/test/mock/domain"
)

const (
	testUSD                            = "USD"
	testPosting1 valueobject.PostingID = "p-1"
	testTenantID valueobject.TenantID  = "t-1"
	testLedgerID valueobject.LedgerID  = "l-1"
	testAccount1 valueobject.AccountID = "a-1"
	testAccount2 valueobject.AccountID = "a-2"
)

var (
	_ repository.AccountRepository = (*mockdomain.MockAccountRepository)(nil)
	_ repository.PostingRepository = (*mockdomain.MockPostingRepository)(nil)
)

func testAccount() entity.AccountData {
	return entity.AccountData{
		ID:        testAccount1,
		TenantID:  testTenantID,
		LedgerID:  testLedgerID,
		Number:    "1000",
		Name:      "Cash",
		Class:     valueobject.ClassAsset,
		AssetCode: testUSD,
		Status:    valueobject.StatusActive,
		Version:   1,
	}
}

func TestAccountPortRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := mockdomain.NewMockAccountRepository(t)
	acc := testAccount()

	repo.EXPECT().Create(ctx, acc).Return(acc, nil).Once()
	repo.EXPECT().Create(ctx, acc).Return(entity.AccountData{}, errors.New("duplicate Create must fail")).Once()
	repo.EXPECT().FindByID(ctx, testTenantID, testAccount1).Return(acc, nil).Once()
	repo.EXPECT().FindByID(ctx, valueobject.TenantID("t-2"), testAccount1).Return(entity.AccountData{}, errors.New("cross-tenant read must fail")).Once()
	repo.EXPECT().UpdateStatus(ctx, testTenantID, testAccount1, valueobject.StatusFrozen, int64(1)).Return(nil).Once()
	frozen := acc
	frozen.Status = valueobject.StatusFrozen
	frozen.Version = 2
	repo.EXPECT().FindByID(ctx, testTenantID, testAccount1).Return(frozen, nil).Once()
	repo.EXPECT().FindByTenant(ctx, testTenantID, "", 10).Return([]entity.AccountData{frozen}, "", nil).Once()

	created, err := repo.Create(ctx, acc)
	require.NoError(t, err)
	assert.Equal(t, acc.Name, created.Name)

	_, err = repo.Create(ctx, acc)
	assert.Error(t, err)

	got, err := repo.FindByID(ctx, testTenantID, testAccount1)
	require.NoError(t, err)
	assert.Equal(t, "Cash", got.Name)

	_, err = repo.FindByID(ctx, "t-2", testAccount1)
	assert.Error(t, err)

	err = repo.UpdateStatus(ctx, testTenantID, testAccount1, valueobject.StatusFrozen, 1)
	require.NoError(t, err)

	got, err = repo.FindByID(ctx, testTenantID, testAccount1)
	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusFrozen, got.Status)
	assert.Equal(t, int64(2), got.Version)

	page, next, err := repo.FindByTenant(ctx, testTenantID, "", 10)
	require.NoError(t, err)
	assert.Len(t, page, 1)
	assert.Empty(t, next)
}

func TestAccountPortVersionConflict(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		ctx             context.Context
		tenant          valueobject.TenantID
		id              valueobject.AccountID
		status          valueobject.AccountStatus
		expectedVersion int64
		setupMock       func(repo *mockdomain.MockAccountRepository)
		expectedError   error
	}

	testCases := []testCase{
		{
			name:            "stale version conflicts",
			ctx:             context.Background(),
			tenant:          testTenantID,
			id:              testAccount1,
			status:          valueobject.StatusFrozen,
			expectedVersion: 99,
			setupMock: func(repo *mockdomain.MockAccountRepository) {
				repo.EXPECT().UpdateStatus(mock.Anything, testTenantID, testAccount1, valueobject.StatusFrozen, int64(99)).
					Return(errors.New("version conflict")).Once()
			},
			expectedError: errors.New("version conflict"),
		},
		{
			name:            "current version succeeds",
			ctx:             context.Background(),
			tenant:          testTenantID,
			id:              testAccount1,
			status:          valueobject.StatusFrozen,
			expectedVersion: 1,
			setupMock: func(repo *mockdomain.MockAccountRepository) {
				repo.EXPECT().UpdateStatus(mock.Anything, testTenantID, testAccount1, valueobject.StatusFrozen, int64(1)).
					Return(nil).Once()
				frozen := testAccount()
				frozen.Status = valueobject.StatusFrozen
				frozen.Version = 2
				repo.EXPECT().FindByID(mock.Anything, testTenantID, testAccount1).
					Return(frozen, nil).Once()
			},
			expectedError: nil,
		},
		{
			name:            "unknown account fails",
			ctx:             context.Background(),
			tenant:          testTenantID,
			id:              "unknown",
			status:          valueobject.StatusFrozen,
			expectedVersion: 1,
			setupMock: func(repo *mockdomain.MockAccountRepository) {
				repo.EXPECT().UpdateStatus(mock.Anything, testTenantID, valueobject.AccountID("unknown"), valueobject.StatusFrozen, int64(1)).
					Return(errors.New("account not found")).Once()
			},
			expectedError: errors.New("account not found"),
		},
		{
			name:            "tenant mismatch fails",
			ctx:             context.Background(),
			tenant:          "other-tenant",
			id:              testAccount1,
			status:          valueobject.StatusFrozen,
			expectedVersion: 1,
			setupMock: func(repo *mockdomain.MockAccountRepository) {
				repo.EXPECT().UpdateStatus(mock.Anything, valueobject.TenantID("other-tenant"), testAccount1, valueobject.StatusFrozen, int64(1)).
					Return(errors.New("account not found")).Once()
			},
			expectedError: errors.New("account not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := mockdomain.NewMockAccountRepository(t)
			tc.setupMock(repo)
			err := repo.UpdateStatus(tc.ctx, tc.tenant, tc.id, tc.status, tc.expectedVersion)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
				got, err := repo.FindByID(tc.ctx, tc.tenant, tc.id)
				assert.NoError(t, err)
				assert.Equal(t, tc.status, got.Status)
				assert.Equal(t, int64(2), got.Version)
			}
		})
	}
}

func TestPostingPortRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := mockdomain.NewMockPostingRepository(t)
	p := entity.PostingData{
		ID:        testPosting1,
		TenantID:  testTenantID,
		LedgerID:  testLedgerID,
		Operation: "transfer.v1",
		Entries: []entity.Entry{
			{ID: "e-1", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
			{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
		},
	}

	repo.EXPECT().Commit(ctx, p).Return(p, nil).Once()
	repo.EXPECT().FindByID(ctx, testTenantID, testPosting1).Return(p, nil).Once()
	repo.EXPECT().FindByID(ctx, valueobject.TenantID("t-9"), testPosting1).Return(entity.PostingData{}, errors.New("cross-tenant read must fail")).Once()
	repo.EXPECT().FindByAccount(ctx, testTenantID, testAccount1, "", 10).Return([]entity.PostingData{p}, "", nil).Once()

	committed, err := repo.Commit(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, p.ID, committed.ID)

	got, err := repo.FindByID(ctx, testTenantID, testPosting1)
	require.NoError(t, err)
	assert.Len(t, got.Entries, 2)

	_, err = repo.FindByID(ctx, "t-9", testPosting1)
	assert.Error(t, err)

	list, _, err := repo.FindByAccount(ctx, testTenantID, testAccount1, "", 10)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func inspectPortFile(t *testing.T, filename string) (int, int) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(filename)) // #nosec G304 -- test inspects package source files
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	src := string(raw)
	for _, banned := range []string{"\n\tSave(", "\n\tDelete(", "WriteEntry(", "SaveEntry("} {
		if strings.Contains(src, banned) {
			t.Errorf("%s exposes banned port method %q", filename, strings.TrimSpace(banned))
		}
	}
	verifyPortImports(t, filename, raw)
	methods := strings.Count(src, "(ctx context.Context")
	documented := strings.Count(src, "Strong read") + strings.Count(src, "Strong write") + strings.Count(src, "Point-in-time")
	return methods, documented
}

func verifyPortImports(t *testing.T, filename string, raw []byte) {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filename, raw, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, imp := range parsed.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == "context" || path == "time" || path == "sync" {
			continue
		}
		if !strings.HasPrefix(path, "github.com/kadekutama/go-template/internal/domain/") {
			t.Errorf("%s imports %s (ports stay inside domain + stdlib)", filename, path)
		}
	}
}

func TestPortSurfaceInspection(t *testing.T) {
	t.Parallel()
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var documented, methods int
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		m, d := inspectPortFile(t, f.Name())
		methods += m
		documented += d
	}
	if methods == 0 {
		t.Fatal("no port methods found")
	}
	if documented < methods {
		t.Errorf("only %d of %d methods document consistency", documented, methods)
	}
}
