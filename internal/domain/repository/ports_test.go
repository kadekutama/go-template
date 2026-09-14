package repository_test

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testUSD      = "USD"
	testPosting1 = "p-1"
	testTenantID = "t-1"
	testLedgerID = "l-1"
	testAccount1 = "a-1"
	testAccount2 = "a-2"
)

// fakeAccounts is an in-memory AccountRepository proving the port is
// implementable without infrastructure imports.
type fakeAccounts struct {
	mu   sync.Mutex
	rows map[valueobject.AccountID]entity.AccountData
}

var _ repository.AccountRepository = (*fakeAccounts)(nil)

func (f *fakeAccounts) Create(_ context.Context, a entity.AccountData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.rows[a.ID]; exists {
		return errors.New("duplicate account")
	}
	f.rows[a.ID] = a
	return nil
}

func (f *fakeAccounts) FindByID(_ context.Context, tenant valueobject.TenantID, id valueobject.AccountID) (entity.AccountData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.rows[id]
	if !ok || a.TenantID != tenant {
		return entity.AccountData{}, errors.New("account not found")
	}
	return a, nil
}

func (f *fakeAccounts) FindByTenant(_ context.Context, tenant valueobject.TenantID, cursor string, limit int) ([]entity.AccountData, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for id, a := range f.rows {
		if a.TenantID == tenant && string(id) > cursor {
			ids = append(ids, string(id))
		}
	}
	sort.Strings(ids)
	if limit <= 0 || limit > len(ids) {
		limit = len(ids)
	}
	out := make([]entity.AccountData, 0, limit)
	for _, id := range ids[:limit] {
		out = append(out, f.rows[valueobject.AccountID(id)])
	}
	next := ""
	if len(ids) > limit {
		next = ids[limit-1]
	}
	return out, next, nil
}

func (f *fakeAccounts) UpdateMetadata(_ context.Context, a entity.AccountData, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.rows[a.ID]
	if !ok {
		return errors.New("account not found")
	}
	if cur.Version != expectedVersion {
		return errors.New("version conflict")
	}
	a.Version = cur.Version + 1
	f.rows[a.ID] = a
	return nil
}

func (f *fakeAccounts) UpdateStatus(_ context.Context, tenant valueobject.TenantID, id valueobject.AccountID, status valueobject.AccountStatus, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.rows[id]
	if !ok || cur.TenantID != tenant {
		return errors.New("account not found")
	}
	if cur.Version != expectedVersion {
		return errors.New("version conflict")
	}
	cur.Status = status
	cur.Version++
	f.rows[id] = cur
	return nil
}

// fakePostings is an in-memory PostingRepository.
type fakePostings struct {
	mu   sync.Mutex
	rows map[valueobject.PostingID]entity.PostingData
}

var _ repository.PostingRepository = (*fakePostings)(nil)

func (f *fakePostings) Commit(_ context.Context, p entity.PostingData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.rows[p.ID]; exists {
		return errors.New("duplicate posting")
	}
	entries := make([]entity.Entry, len(p.Entries))
	copy(entries, p.Entries)
	p.Entries = entries
	f.rows[p.ID] = p
	return nil
}

func (f *fakePostings) FindByID(_ context.Context, tenant valueobject.TenantID, id valueobject.PostingID) (entity.PostingData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.rows[id]
	if !ok || p.TenantID != tenant {
		return entity.PostingData{}, errors.New("posting not found")
	}
	return p, nil
}

func (f *fakePostings) FindByExternalReference(_ context.Context, tenant valueobject.TenantID, reference string) (entity.PostingData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.rows {
		if p.TenantID == tenant && p.ExternalReference == reference {
			return p, nil
		}
	}
	return entity.PostingData{}, errors.New("posting not found")
}

func (f *fakePostings) FindByAccount(_ context.Context, tenant valueobject.TenantID, account valueobject.AccountID, _ string, limit int) ([]entity.PostingData, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []entity.PostingData
	for _, p := range f.rows {
		if p.TenantID != tenant {
			continue
		}
		for _, e := range p.Entries {
			if e.AccountID == account {
				out = append(out, p)
				break
			}
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, "", nil
}

func testAccount() entity.AccountData {
	return entity.AccountData{ID: testAccount1, TenantID: testTenantID, LedgerID: testLedgerID, Number: "1000", Name: "Cash",
		Class: valueobject.ClassAsset, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1}
}

func TestAccountPortRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeAccounts{rows: map[valueobject.AccountID]entity.AccountData{}}
	if err := repo.Create(ctx, testAccount()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, testAccount()); err == nil {
		t.Fatal("duplicate Create must fail")
	}
	got, err := repo.FindByID(ctx, testTenantID, testAccount1)
	if err != nil || got.Name != "Cash" {
		t.Fatalf("FindByID = %+v, %v", got, err)
	}
	if _, err := repo.FindByID(ctx, "t-2", testAccount1); err == nil {
		t.Fatal("cross-tenant read must fail")
	}
	if err := repo.UpdateStatus(ctx, testTenantID, testAccount1, valueobject.StatusFrozen, 1); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ = repo.FindByID(ctx, testTenantID, testAccount1)
	if got.Status != valueobject.StatusFrozen || got.Version != 2 {
		t.Fatalf("after update = %+v", got)
	}
	page, next, err := repo.FindByTenant(ctx, testTenantID, "", 10)
	if err != nil || len(page) != 1 || next != "" {
		t.Fatalf("FindByTenant = %+v, %q, %v", page, next, err)
	}
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
			expectedError:   errors.New("version conflict"),
		},
		{
			name:            "current version succeeds",
			ctx:             context.Background(),
			tenant:          testTenantID,
			id:              testAccount1,
			status:          valueobject.StatusFrozen,
			expectedVersion: 1,
			expectedError:   nil,
		},
		{
			name:            "unknown account fails",
			ctx:             context.Background(),
			tenant:          testTenantID,
			id:              "unknown",
			status:          valueobject.StatusFrozen,
			expectedVersion: 1,
			expectedError:   errors.New("account not found"),
		},
		{
			name:            "tenant mismatch fails",
			ctx:             context.Background(),
			tenant:          "other-tenant",
			id:              testAccount1,
			status:          valueobject.StatusFrozen,
			expectedVersion: 1,
			expectedError:   errors.New("account not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAccounts{rows: map[valueobject.AccountID]entity.AccountData{
				testAccount1: testAccount(),
			}}
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
	repo := &fakePostings{rows: map[valueobject.PostingID]entity.PostingData{}}
	p := entity.PostingData{ID: testPosting1, TenantID: testTenantID, LedgerID: testLedgerID, Operation: "transfer.v1",
		Entries: []entity.Entry{
			{ID: "e-1", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
			{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
		}}
	if err := repo.Commit(ctx, p); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	got, err := repo.FindByID(ctx, testTenantID, testPosting1)
	if err != nil || len(got.Entries) != 2 {
		t.Fatalf("FindByID = %+v, %v", got, err)
	}
	if _, err := repo.FindByID(ctx, "t-9", testPosting1); err == nil {
		t.Fatal("cross-tenant read must fail")
	}
	list, _, err := repo.FindByAccount(ctx, testTenantID, testAccount1, "", 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("FindByAccount = %+v, %v", list, err)
	}
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
