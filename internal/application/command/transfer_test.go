package command_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	tfrTenant = valueobject.TenantID("t-1")
	tfrLedger = valueobject.LedgerID("l-1")
	tfrSrc    = valueobject.AccountID("a-src")
	tfrDst    = valueobject.AccountID("a-dst")
	tfrAsset  = valueobject.AssetCode("USD")
)

var tfrAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

type tfrAccounts struct {
	mu       sync.Mutex
	accounts map[valueobject.AccountID]entity.AccountData
}

func (f *tfrAccounts) Create(_ context.Context, _ entity.AccountData) error { return nil }

func (f *tfrAccounts) FindByID(_ context.Context, _ valueobject.TenantID, id valueobject.AccountID) (entity.AccountData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	account, ok := f.accounts[id]
	if !ok {
		return entity.AccountData{}, entity.NewError("ACCOUNT_NOT_FOUND", "account "+string(id)+" is unknown")
	}
	return account, nil
}

func (f *tfrAccounts) FindByTenant(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]entity.AccountData, string, error) {
	return nil, "", nil
}

func (f *tfrAccounts) UpdateMetadata(_ context.Context, _ entity.AccountData, _ int64) error {
	return nil
}

func (f *tfrAccounts) UpdateStatus(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, _ valueobject.AccountStatus, _ int64) error {
	return nil
}

func tfrTestAccounts() map[valueobject.AccountID]entity.AccountData {
	return map[valueobject.AccountID]entity.AccountData{
		tfrSrc: {ID: tfrSrc, TenantID: tfrTenant, LedgerID: tfrLedger, Number: "7000", Name: "src", Class: valueobject.ClassLiability, AssetCode: tfrAsset, Status: valueobject.StatusActive, Version: 1},
		tfrDst: {ID: tfrDst, TenantID: tfrTenant, LedgerID: tfrLedger, Number: "7001", Name: "dst", Class: valueobject.ClassLiability, AssetCode: tfrAsset, Status: valueobject.StatusActive, Version: 1},
	}
}

type tfrBalances struct {
	available map[valueobject.AccountID]int64
	err       error
}

func (s *tfrBalances) Execute(_ context.Context, query port.BalanceQuery) (port.BalanceView, error) {
	if s.err != nil {
		return port.BalanceView{}, s.err
	}
	return port.BalanceView{AccountID: query.AccountID, AssetCode: query.AssetCode, AvailableMinor: s.available[query.AccountID], Cursor: "cursor-b"}, nil
}

type tfrStore struct {
	mu        sync.Mutex
	transfers map[string]command.TransferRecord
	batches   map[string]command.BatchRecord
	items     map[string][]command.BatchItem
}

func (s *tfrStore) CreateTransfer(_ context.Context, record command.TransferRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.transfers == nil {
		s.transfers = map[string]command.TransferRecord{}
	}
	if _, dup := s.transfers[record.ID]; dup {
		return entity.NewError("TRANSFER_CONFLICT", "transfer id already exists")
	}
	s.transfers[record.ID] = record
	return nil
}

func (s *tfrStore) FindTransfer(_ context.Context, _ valueobject.TenantID, id string) (command.TransferRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.transfers[id]
	if !ok {
		return command.TransferRecord{}, entity.NewError("TRANSFER_NOT_FOUND", "transfer is unknown")
	}
	return record, nil
}

func (s *tfrStore) UpdateTransfer(_ context.Context, record command.TransferRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transfers[record.ID] = record
	return nil
}

func (s *tfrStore) ListTransfers(_ context.Context, filter command.TransferListFilter) ([]command.TransferRecord, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []command.TransferRecord
	for _, record := range s.transfers {
		if record.TenantID != filter.TenantID {
			continue
		}
		if filter.Status != "" && record.Status != filter.Status {
			continue
		}
		out = append(out, record)
	}
	return out, "", nil
}

func (s *tfrStore) CreateBatch(_ context.Context, batch command.BatchRecord, items []command.BatchItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.batches == nil {
		s.batches = map[string]command.BatchRecord{}
		s.items = map[string][]command.BatchItem{}
	}
	s.batches[batch.ID] = batch
	s.items[batch.ID] = append([]command.BatchItem(nil), items...)
	return nil
}

func (s *tfrStore) FindBatch(_ context.Context, _ valueobject.TenantID, id string) (command.BatchRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, ok := s.batches[id]
	if !ok {
		return command.BatchRecord{}, entity.NewError("BATCH_NOT_FOUND", "batch is unknown")
	}
	return batch, nil
}

func (s *tfrStore) UpdateBatchState(_ context.Context, _ valueobject.TenantID, id, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch := s.batches[id]
	batch.State = state
	s.batches[id] = batch
	return nil
}

func (s *tfrStore) UpdateBatchItem(_ context.Context, _ valueobject.TenantID, batchID string, index int, status, errorCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items[batchID] {
		if item.Index == index {
			s.items[batchID][i].Status = status
			s.items[batchID][i].ErrorCode = errorCode
		}
	}
	return nil
}

func (s *tfrStore) ListBatchItems(_ context.Context, _ valueobject.TenantID, batchID string) ([]command.BatchItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]command.BatchItem(nil), s.items[batchID]...), nil
}

type tfrIdemEntry struct {
	fingerprint string
	response    []byte
	completed   bool
}

type tfrUOW struct {
	mu     sync.Mutex
	outbox []port.OutboxFact
	idem   map[string]tfrIdemEntry
}

func (u *tfrUOW) Do(ctx context.Context, fn func(ctx context.Context, tx port.Tx) error) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	stagedOutbox := []port.OutboxFact{}
	stagedIdem := map[string]tfrIdemEntry{}
	tx := &tfrTx{uow: u, outbox: &stagedOutbox, idem: stagedIdem}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	u.outbox = append(u.outbox, stagedOutbox...)
	if u.idem == nil {
		u.idem = map[string]tfrIdemEntry{}
	}
	for key, entry := range stagedIdem {
		u.idem[key] = entry
	}
	return nil
}

type tfrTx struct {
	uow    *tfrUOW
	outbox *[]port.OutboxFact
	idem   map[string]tfrIdemEntry
	staged map[valueobject.PostingID]entity.PostingData
}

func (t *tfrTx) Postings() repository.PostingRepository { return &tfrTxPostings{tx: t} }
func (t *tfrTx) Holds() repository.HoldRepository       { return nil }
func (t *tfrTx) Idempotency() port.IdempotencyStore     { return &tfrTxIdem{tx: t} }
func (t *tfrTx) Outbox() port.EventOutbox               { return &tfrTxOutbox{tx: t} }
func (t *tfrTx) Cursor() string                         { return "cursor-5" }

type tfrTxPostings struct {
	tx *tfrTx
}

func (p *tfrTxPostings) Commit(_ context.Context, posting entity.PostingData) error {
	if p.tx.staged == nil {
		p.tx.staged = map[valueobject.PostingID]entity.PostingData{}
	}
	p.tx.staged[posting.ID] = posting
	return nil
}

func (p *tfrTxPostings) FindByID(_ context.Context, _ valueobject.TenantID, _ valueobject.PostingID) (entity.PostingData, error) {
	return entity.PostingData{}, entity.NewError("POSTING_NOT_FOUND", "posting is unknown")
}

func (p *tfrTxPostings) FindByExternalReference(_ context.Context, _ valueobject.TenantID, _ string) (entity.PostingData, error) {
	return entity.PostingData{}, entity.NewError("POSTING_NOT_FOUND", "posting is unknown")
}

func (p *tfrTxPostings) FindByAccount(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, _ string, _ int) ([]entity.PostingData, string, error) {
	return nil, "", nil
}

type tfrTxIdem struct {
	tx *tfrTx
}

func (s *tfrTxIdem) Reserve(_ context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error) {
	if entry, ok := s.tx.uow.idem[rec.Key]; ok {
		if entry.fingerprint != rec.Fingerprint {
			return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
		}
		s.tx.idem[rec.Key] = entry
		if entry.completed {
			return port.ReserveOutcome{Replay: true, Response: entry.response}, nil
		}
		return port.ReserveOutcome{}, nil
	}
	if entry, ok := s.tx.idem[rec.Key]; ok {
		if entry.fingerprint != rec.Fingerprint {
			return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
		}
		return port.ReserveOutcome{}, nil
	}
	s.tx.idem[rec.Key] = tfrIdemEntry{fingerprint: rec.Fingerprint}
	return port.ReserveOutcome{}, nil
}

func (s *tfrTxIdem) Complete(_ context.Context, key string, response []byte) error {
	entry, ok := s.tx.idem[key]
	if !ok {
		entry = s.tx.uow.idem[key]
	}
	entry.response = response
	entry.completed = true
	s.tx.idem[key] = entry
	return nil
}

type tfrTxOutbox struct {
	tx *tfrTx
}

func (o *tfrTxOutbox) Append(_ context.Context, facts ...port.OutboxFact) error {
	*o.tx.outbox = append(*o.tx.outbox, facts...)
	return nil
}

type tfrClock struct{}

func (tfrClock) Now() time.Time { return tfrAt }

type tfrIDs struct {
	mu   sync.Mutex
	next []string
}

func (f *tfrIDs) NewID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.next[0]
	f.next = f.next[1:]
	return id
}

func tfrTestIDs(n int) []string {
	ids := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		ids = append(ids, fmt.Sprintf("tfr-%02d", i))
	}
	return ids
}

type tfrAuthz struct {
	denied map[string]bool
	calls  int
}

func (a *tfrAuthz) Authorize(_ context.Context, subject port.Subject, action, resource string) error {
	a.calls++
	if a.denied[subject.ID+"|"+action+"|"+resource] {
		return entity.NewError("FORBIDDEN", "subject is not authorized for this action")
	}
	return nil
}

func newTransferService(uow *tfrUOW, store *tfrStore, balances *tfrBalances, authz *tfrAuthz) *command.TransferService {
	return command.NewTransferService(command.TransferServiceParams{
		UoW:       uow,
		Accounts:  &tfrAccounts{accounts: tfrTestAccounts()},
		Balances:  balances,
		Transfers: store,
		Clock:     tfrClock{},
		IDs:       &tfrIDs{next: tfrTestIDs(60)},
		Authz:     authz,
	})
}

func transferTestCommand() port.TransferRequest {
	return port.TransferRequest{
		TenantID: tfrTenant, LedgerID: tfrLedger, Source: tfrSrc, Dest: tfrDst,
		AssetCode: tfrAsset, AmountMinor: 5000, IdempotencyKey: "key-1", Actor: "u-1",
	}
}

func TestTransferCreate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		cmd                port.TransferRequest
		available          int64
		preload            func(uow *tfrUOW, svc *command.TransferService)
		expectedStatus     string
		expectedError      error
		expectedRecords    int
		expectedAuthzCalls int
	}

	testCases := []testCase{
		{
			name:               "immediate transfer posts on funds",
			cmd:                transferTestCommand(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     command.TransferCompleted,
			expectedError:      nil,
			expectedRecords:    1,
			expectedAuthzCalls: 1,
		},
		{
			name:      "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			cmd:       transferTestCommand(),
			available: 9000,
			preload: func(uow *tfrUOW, _ *command.TransferService) {
				req := transferTestCommand()
				fp := command.Fingerprint(
					req.IdempotencyKey, string(req.TenantID), string(req.LedgerID),
					string(req.Source), string(req.Dest), string(req.AssetCode),
					fmt.Sprintf("%d", req.AmountMinor),
					req.ExecuteAt.UTC().Format(time.RFC3339Nano), req.Recurrence,
				)
				uow.idem = map[string]tfrIdemEntry{
					req.IdempotencyKey: {
						fingerprint: fp,
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
			},
			expectedStatus:     "",
			expectedError:      entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
			expectedRecords:    0,
			expectedAuthzCalls: 1,
		},
		{
			name:               "immediate transfer fails without funds",
			cmd:                transferTestCommand(),
			available:          100,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("INSUFFICIENT_FUNDS", "source available balance below transfer amount"),
			expectedRecords:    0,
			expectedAuthzCalls: 1,
		},
		{
			name: "scheduled transfer persists pending without funds check",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.ExecuteAt = tfrAt.Add(24 * time.Hour)
				return c
			}(),
			available:          0,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     command.TransferPending,
			expectedError:      nil,
			expectedRecords:    1,
			expectedAuthzCalls: 1,
		},
		{
			name:      "duplicate replay returns original single record",
			cmd:       transferTestCommand(),
			available: 9000,
			preload: func(_ *tfrUOW, svc *command.TransferService) {
				_, err := svc.CreateTransfer(context.Background(), transferTestCommand())
				require.NoError(t, err)
			},
			expectedStatus:     command.TransferCompleted,
			expectedError:      nil,
			expectedRecords:    1,
			expectedAuthzCalls: 2,
		},
		{
			name: "missing tenant fails envelope validation",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.TenantID = ""
				return c
			}(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("TENANT_REQUIRED", "tenant id is required"),
			expectedRecords:    0,
			expectedAuthzCalls: 0,
		},
		{
			name: "missing ledger fails envelope validation",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.LedgerID = ""
				return c
			}(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
			expectedRecords:    0,
			expectedAuthzCalls: 0,
		},
		{
			name: "missing source account fails envelope validation",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.Source = ""
				return c
			}(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("TRANSFER_ACCOUNT_REQUIRED", "transfer requires source and destination accounts"),
			expectedRecords:    0,
			expectedAuthzCalls: 0,
		},
		{
			name: "missing asset code fails envelope validation",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.AssetCode = ""
				return c
			}(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("TRANSFER_ASSET_REQUIRED", "transfer requires an asset code"),
			expectedRecords:    0,
			expectedAuthzCalls: 0,
		},
		{
			name: "zero amount fails envelope validation",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.AmountMinor = 0
				return c
			}(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive"),
			expectedRecords:    0,
			expectedAuthzCalls: 0,
		},
		{
			name: "cross-currency transfer rejected without FX settlement lot",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.AssetCode = "EUR"
				return c
			}(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("CROSS_CURRENCY_UNSUPPORTED", "cross-currency transfers need FX settlement lots (E10 follow-up)"),
			expectedRecords:    0,
			expectedAuthzCalls: 1,
		},
		{
			name: "missing actor fails envelope validation",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.Actor = ""
				return c
			}(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("ACTOR_REQUIRED", "transfer actor is required"),
			expectedRecords:    0,
			expectedAuthzCalls: 0,
		},
		{
			name: "missing idempotency key fails envelope validation",
			cmd: func() port.TransferRequest {
				c := transferTestCommand()
				c.IdempotencyKey = ""
				return c
			}(),
			available:          9000,
			preload:            func(_ *tfrUOW, _ *command.TransferService) {},
			expectedStatus:     "",
			expectedError:      entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "transfer requires an idempotency key"),
			expectedRecords:    0,
			expectedAuthzCalls: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &tfrStore{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			balances := &tfrBalances{available: map[valueobject.AccountID]int64{tfrSrc: tc.available}}
			svc := newTransferService(uow, store, balances, authz)
			tc.preload(uow, svc)
			actualResult, err := svc.CreateTransfer(context.Background(), tc.cmd)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Status)
			}
			assert.Len(t, store.transfers, tc.expectedRecords)
			assert.Equal(t, tc.expectedAuthzCalls, authz.calls)
		})
	}
}

func TestTransferExecute(t *testing.T) {
	t.Parallel()

	seedPending := func(t *testing.T, svc *command.TransferService) string {
		t.Helper()
		cmd := transferTestCommand()
		cmd.ExecuteAt = tfrAt.Add(24 * time.Hour)
		res, err := svc.CreateTransfer(context.Background(), cmd)
		require.NoError(t, err)
		require.Equal(t, command.TransferPending, res.Status)
		return res.TransferID
	}

	type testCase struct {
		name           string
		tenant         valueobject.TenantID
		actor          string
		available      int64
		preload        func(t *testing.T, svc *command.TransferService, id string)
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "pending executes on arrived funds",
			tenant:         tfrTenant,
			actor:          "u-1",
			available:      9000,
			preload:        func(_ *testing.T, _ *command.TransferService, _ string) {},
			expectedStatus: command.TransferCompleted,
			expectedError:  nil,
		},
		{
			name:           "missing actor fails validation",
			tenant:         tfrTenant,
			actor:          "",
			available:      9000,
			preload:        func(_ *testing.T, _ *command.TransferService, _ string) {},
			expectedStatus: "",
			expectedError:  entity.NewError("ACTOR_REQUIRED", "transfer actor is required"),
		},
		{
			name:           "pending fails without funds and records failure",
			tenant:         tfrTenant,
			actor:          "u-1",
			available:      0,
			preload:        func(_ *testing.T, _ *command.TransferService, _ string) {},
			expectedStatus: "",
			expectedError:  entity.NewError("INSUFFICIENT_FUNDS", "source available balance below transfer amount"),
		},
		{
			name:      "completed execution replays stored outcome",
			tenant:    tfrTenant,
			actor:     "u-1",
			available: 9000,
			preload: func(t *testing.T, svc *command.TransferService, id string) {
				_, err := svc.ExecuteTransfer(context.Background(), tfrTenant, id, "u-1")
				require.NoError(t, err)
			},
			expectedStatus: command.TransferCompleted,
			expectedError:  nil,
		},
		{
			name:      "canceled transfer cannot execute",
			tenant:    tfrTenant,
			actor:     "u-1",
			available: 9000,
			preload: func(t *testing.T, svc *command.TransferService, id string) {
				_, err := svc.CancelTransfer(context.Background(), port.TransferQuery{TenantID: tfrTenant, TransferID: id, Actor: "u-1"})
				require.NoError(t, err)
			},
			expectedStatus: "",
			expectedError:  entity.NewError("TRANSFER_STATE_INVALID", "only pending transfers can execute"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &tfrStore{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			balances := &tfrBalances{available: map[valueobject.AccountID]int64{tfrSrc: 0}}
			svc := newTransferService(uow, store, balances, authz)
			id := seedPending(t, svc)
			balances.available[tfrSrc] = tc.available
			tc.preload(t, svc, id)
			actualResult, err := svc.ExecuteTransfer(context.Background(), tc.tenant, id, tc.actor)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Status)
			}
		})
	}
}

func TestTransferCancel(t *testing.T) {
	t.Parallel()

	seedPending := func(t *testing.T, svc *command.TransferService) string {
		t.Helper()
		cmd := transferTestCommand()
		cmd.ExecuteAt = tfrAt.Add(24 * time.Hour)
		res, err := svc.CreateTransfer(context.Background(), cmd)
		require.NoError(t, err)
		return res.TransferID
	}

	type testCase struct {
		name           string
		query          func(id string) port.TransferQuery
		preload        func(t *testing.T, svc *command.TransferService, id string)
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "pending transfer is canceled",
			query: func(id string) port.TransferQuery {
				return port.TransferQuery{TenantID: tfrTenant, TransferID: id, Actor: "u-1"}
			},
			preload:        func(_ *testing.T, _ *command.TransferService, _ string) {},
			denied:         false,
			expectedStatus: command.TransferCanceled,
			expectedError:  nil,
		},
		{
			name: "missing actor fails",
			query: func(id string) port.TransferQuery {
				return port.TransferQuery{TenantID: tfrTenant, TransferID: id, Actor: ""}
			},
			preload:        func(_ *testing.T, _ *command.TransferService, _ string) {},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("ACTOR_REQUIRED", "transfer actor is required"),
		},
		{
			name: "replay cancel returns stored cancellation",
			query: func(id string) port.TransferQuery {
				return port.TransferQuery{TenantID: tfrTenant, TransferID: id, Actor: "u-1"}
			},
			preload: func(t *testing.T, svc *command.TransferService, id string) {
				_, err := svc.CancelTransfer(context.Background(), port.TransferQuery{TenantID: tfrTenant, TransferID: id, Actor: "u-1"})
				require.NoError(t, err)
			},
			denied:         false,
			expectedStatus: command.TransferCanceled,
			expectedError:  nil,
		},
		{
			name: "non-pending transfer cannot be canceled",
			query: func(id string) port.TransferQuery {
				return port.TransferQuery{TenantID: tfrTenant, TransferID: id, Actor: "u-1"}
			},
			preload: func(t *testing.T, svc *command.TransferService, id string) {
				_, err := svc.ExecuteTransfer(context.Background(), tfrTenant, id, "u-1")
				require.NoError(t, err)
			},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("TRANSFER_STATE_INVALID", "only pending transfers can be canceled"),
		},
		{
			name: "denied subject returns FORBIDDEN",
			query: func(id string) port.TransferQuery {
				return port.TransferQuery{TenantID: tfrTenant, TransferID: id, Actor: "u-1"}
			},
			preload:        func(_ *testing.T, _ *command.TransferService, _ string) {},
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &tfrStore{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			balances := &tfrBalances{available: map[valueobject.AccountID]int64{tfrSrc: 9000}}
			svc := newTransferService(uow, store, balances, authz)
			id := seedPending(t, svc)
			query := tc.query(id)
			if tc.denied {
				authz.denied["u-1|transfer.cancel|ledger/"+id] = true
			}
			tc.preload(t, svc, id)
			actualResult, err := svc.CancelTransfer(context.Background(), query)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Status)
			}
		})
	}
}

func TestTransferBatch(t *testing.T) {
	t.Parallel()

	batchCommand := func(n int, tenant valueobject.TenantID) port.BatchTransferRequest {
		items := make([]port.TransferRequest, 0, n)
		for i := 0; i < n; i++ {
			item := transferTestCommand()
			item.TenantID = tenant
			item.IdempotencyKey = fmt.Sprintf("item-%d", i)
			items = append(items, item)
		}
		return port.BatchTransferRequest{
			TenantID: tfrTenant, LedgerID: tfrLedger, Items: items,
			IdempotencyKey: "batch-1", Actor: "u-1",
		}
	}

	type testCase struct {
		name              string
		req               port.BatchTransferRequest
		available         int64
		preload           func(uow *tfrUOW)
		expectedState     string
		expectedError     error
		expectedBatches   int
		expectedSucceeded int
		expectedFailed    int
	}

	testCases := []testCase{
		{
			name:              "all items complete the batch",
			req:               batchCommand(3, tfrTenant),
			available:         90000,
			expectedState:     command.BatchCompleted,
			expectedError:     nil,
			expectedBatches:   1,
			expectedSucceeded: 3,
			expectedFailed:    0,
		},
		{
			name:      "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:       batchCommand(2, tfrTenant),
			available: 90000,
			preload: func(uow *tfrUOW) {
				req := batchCommand(2, tfrTenant)
				parts := []string{req.IdempotencyKey, string(req.TenantID), string(req.LedgerID), fmt.Sprintf("%d", len(req.Items))}
				for _, it := range req.Items {
					parts = append(parts, string(it.Source), string(it.Dest), string(it.AssetCode), fmt.Sprintf("%d", it.AmountMinor))
				}
				uow.idem = map[string]tfrIdemEntry{
					req.IdempotencyKey: {
						fingerprint: command.Fingerprint(parts...),
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
			},
			expectedState:   "",
			expectedError:   entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
			expectedBatches: 0,
		},
		{
			name: "one failing item completes partial with siblings intact",
			req: func() port.BatchTransferRequest {
				req := batchCommand(3, tfrTenant)
				req.Items[1].AmountMinor = 90000
				return req
			}(),
			available:         9000,
			expectedState:     command.BatchPartial,
			expectedError:     nil,
			expectedBatches:   1,
			expectedSucceeded: 2,
			expectedFailed:    1,
		},
		{
			name: "over-limit batch rejected pre-intake",
			req: func() port.BatchTransferRequest {
				items := make([]port.TransferRequest, 0, 1001)
				for i := 0; i < 1001; i++ {
					items = append(items, transferTestCommand())
				}
				return port.BatchTransferRequest{
					TenantID: tfrTenant, LedgerID: tfrLedger, Items: items,
					IdempotencyKey: "batch-1", Actor: "u-1",
				}
			}(),
			available:       90000,
			expectedState:   "",
			expectedError:   entity.NewError("BATCH_TOO_LARGE", "batch exceeds 1000 items"),
			expectedBatches: 0,
		},
		{
			name:            "mixed-tenant batch rejected pre-intake",
			req:             batchCommand(2, "t-2"),
			available:       90000,
			expectedState:   "",
			expectedError:   entity.NewError("BATCH_TENANT_MISMATCH", "batch items must share the batch tenant"),
			expectedBatches: 0,
		},
		{
			name: "missing tenant fails batch envelope validation",
			req: func() port.BatchTransferRequest {
				b := batchCommand(2, tfrTenant)
				b.TenantID = ""
				return b
			}(),
			available:       90000,
			expectedState:   "",
			expectedError:   entity.NewError("TENANT_REQUIRED", "tenant id is required"),
			expectedBatches: 0,
		},
		{
			name: "missing ledger fails batch envelope validation",
			req: func() port.BatchTransferRequest {
				b := batchCommand(2, tfrTenant)
				b.LedgerID = ""
				return b
			}(),
			available:       90000,
			expectedState:   "",
			expectedError:   entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
			expectedBatches: 0,
		},
		{
			name: "empty batch items fails envelope validation",
			req: func() port.BatchTransferRequest {
				b := batchCommand(0, tfrTenant)
				return b
			}(),
			available:       90000,
			expectedState:   "",
			expectedError:   entity.NewError("BATCH_EMPTY", "batch requires at least one item"),
			expectedBatches: 0,
		},
		{
			name: "missing actor fails batch envelope validation",
			req: func() port.BatchTransferRequest {
				b := batchCommand(2, tfrTenant)
				b.Actor = ""
				return b
			}(),
			available:       90000,
			expectedState:   "",
			expectedError:   entity.NewError("ACTOR_REQUIRED", "transfer actor is required"),
			expectedBatches: 0,
		},
		{
			name: "missing idempotency key fails batch envelope validation",
			req: func() port.BatchTransferRequest {
				b := batchCommand(2, tfrTenant)
				b.IdempotencyKey = ""
				return b
			}(),
			available:       90000,
			expectedState:   "",
			expectedError:   entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "transfer requires an idempotency key"),
			expectedBatches: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			if tc.preload != nil {
				tc.preload(uow)
			}
			store := &tfrStore{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			balances := &tfrBalances{available: map[valueobject.AccountID]int64{tfrSrc: tc.available}}
			svc := newTransferService(uow, store, balances, authz)
			actualResult, err := svc.CreateBatchTransfer(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			assert.Len(t, store.batches, tc.expectedBatches)
			if tc.expectedError != nil {
				return
			}
			assert.Equal(t, len(tc.req.Items), actualResult.TotalItems)
			querySvc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: store})
			status, statusErr := querySvc.GetBatchStatus(context.Background(), port.BatchStatusQuery{TenantID: tfrTenant, BatchID: actualResult.BatchID})
			require.NoError(t, statusErr)
			assert.Equal(t, tc.expectedState, status.State)
			assert.Len(t, status.Items, len(tc.req.Items))
			assert.Equal(t, tc.expectedSucceeded, status.Succeeded)
			assert.Equal(t, tc.expectedFailed, status.Failed)
			for _, item := range status.Items {
				if item.Status == command.TransferFailed {
					assert.Equal(t, "INSUFFICIENT_FUNDS", item.ErrorCode)
				}
			}
		})
	}
}
