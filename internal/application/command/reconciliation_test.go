package command_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

var opsAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

type reconStoreFake struct {
	mu      sync.Mutex
	runs    map[string]command.ReconRunRecord
	breaks  map[string]command.BreakRecord
	openN   int
	openErr error
}

func (s *reconStoreFake) CreateRun(_ context.Context, run command.ReconRunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runs == nil {
		s.runs = map[string]command.ReconRunRecord{}
	}
	s.runs[run.ID] = run
	return nil
}

func (s *reconStoreFake) FindRun(_ context.Context, _ valueobject.TenantID, id string) (command.ReconRunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		return command.ReconRunRecord{}, entity.NewError("RUN_NOT_FOUND", "reconciliation run is unknown")
	}
	return run, nil
}

func (s *reconStoreFake) UpdateRun(_ context.Context, run command.ReconRunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *reconStoreFake) CreateBreaks(_ context.Context, breaks []command.BreakRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.breaks == nil {
		s.breaks = map[string]command.BreakRecord{}
	}
	for _, br := range breaks {
		s.breaks[br.Break.BreakID] = br
	}
	return nil
}

func (s *reconStoreFake) FindBreak(_ context.Context, _ valueobject.TenantID, id string) (command.BreakRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.breaks[id]
	if !ok {
		return command.BreakRecord{}, entity.NewError("BREAK_NOT_FOUND", "break is unknown")
	}
	return record, nil
}

func (s *reconStoreFake) UpdateBreak(_ context.Context, record command.BreakRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.breaks[record.Break.BreakID] = record
	return nil
}

func (s *reconStoreFake) ListBreaksByRun(_ context.Context, _ valueobject.TenantID, runID string) ([]command.BreakRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []command.BreakRecord
	for _, record := range s.breaks {
		if record.Break.RunID == runID {
			out = append(out, record)
		}
	}
	return out, nil
}

func (s *reconStoreFake) CountOpenBreaks(_ context.Context, _ valueobject.TenantID, _ valueobject.LedgerID) (int, error) {
	return s.openN, s.openErr
}

func (s *reconStoreFake) ListRuns(_ context.Context, _ valueobject.TenantID, limit int) ([]command.ReconRunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []command.ReconRunRecord
	for _, run := range s.runs {
		out = append(out, run)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *reconStoreFake) ListBreaks(_ context.Context, _ valueobject.TenantID, status string, limit int) ([]command.BreakRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []command.BreakRecord
	for _, record := range s.breaks {
		if status != "" && string(record.Status) != status {
			continue
		}
		out = append(out, record)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func newReconService(uow *tfrUOW, store *reconStoreFake, authz *tfrAuthz) *command.ReconService {
	return command.NewReconService(command.ReconServiceParams{
		UoW:               uow,
		Runs:              store,
		Clock:             tfrClock{},
		IDs:               &tfrIDs{next: tfrTestIDs(40)},
		Authz:             authz,
		SoDThresholdMinor: 10000,
	})
}

func reconLine(id, ref string, amount int64) entity.ExternalStatementLine {
	return entity.ExternalStatementLine{
		LineID: id, SourceAccount: "bank-acct", Reference: ref, AmountMinor: amount,
		AssetCode: "USD", EffectiveAt: opsAt, Hash: "hash-" + id, ParserVersion: "csv-v1",
		RawLineage: "line:" + id, CoverageStart: opsAt.Add(-24 * time.Hour), CoverageEnd: opsAt,
	}
}

func TestReconTrigger(t *testing.T) {
	t.Parallel()

	baseReq := port.ReconRunRequest{
		TenantID:       tfrTenant,
		LedgerID:       tfrLedger,
		Source:         "bank-csv",
		WindowStart:    opsAt.Add(-24 * time.Hour),
		WindowEnd:      opsAt,
		IdempotencyKey: "key-run-1",
		Actor:          "u-1",
	}

	type testCase struct {
		name           string
		req            port.ReconRunRequest
		preload        func(uow *tfrUOW)
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid trigger starts pending run with outbox fact",
			req:            baseReq,
			preload:        func(_ *tfrUOW) {},
			denied:         false,
			expectedStatus: command.ReconRunPending,
			expectedError:  nil,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  baseReq,
			preload: func(uow *tfrUOW) {
				fp := command.Fingerprint(baseReq.IdempotencyKey, string(baseReq.TenantID), baseReq.Source, baseReq.WindowStart.UTC().Format(time.RFC3339Nano), baseReq.WindowEnd.UTC().Format(time.RFC3339Nano))
				uow.idem = map[string]tfrIdemEntry{
					baseReq.IdempotencyKey: {
						fingerprint: fp,
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
			},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
		},
		{
			name: "missing tenant returns TENANT_REQUIRED",
			req: func() port.ReconRunRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			preload:        func(_ *tfrUOW) {},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			req: func() port.ReconRunRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			preload:        func(_ *tfrUOW) {},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("ACTOR_REQUIRED", "reconciliation actor is required"),
		},
		{
			name: "missing idempotency key returns IDEMPOTENCY_KEY_REQUIRED",
			req: func() port.ReconRunRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			preload:        func(_ *tfrUOW) {},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "reconciliation requires an idempotency key"),
		},
		{
			name:           "denied subject returns FORBIDDEN",
			req:            baseReq,
			preload:        func(_ *tfrUOW) {},
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &reconStoreFake{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied[tc.req.Actor+"|recon.trigger|ledger/"+string(tc.req.LedgerID)] = true
			}
			svc := newReconService(uow, store, authz)
			tc.preload(uow)

			res, err := svc.TriggerReconRun(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, res.Status)
				assert.Contains(t, outboxTypes(uow), command.EventReconRunStarted)

				// Idempotency replay
				replayRes, replayErr := svc.TriggerReconRun(context.Background(), tc.req)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.RunID, replayRes.RunID)
			}
		})
	}
}

func TestReconExecuteRun(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		runID         string
		seedRun       bool
		runStatus     string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "execute pending run matches and completes",
			runID:         "run-exec-1",
			seedRun:       true,
			runStatus:     command.ReconRunPending,
			expectedError: nil,
		},
		{
			name:          "run not found returns RUN_NOT_FOUND",
			runID:         "run-missing",
			seedRun:       false,
			runStatus:     "",
			expectedError: entity.NewError("RUN_NOT_FOUND", "reconciliation run is unknown"),
		},
		{
			name:          "already completed run rejected",
			runID:         "run-completed-1",
			seedRun:       true,
			runStatus:     command.ReconRunCompleted,
			expectedError: entity.NewError("RUN_STATE_INVALID", "only pending runs can execute"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &reconStoreFake{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			svc := newReconService(uow, store, authz)

			if tc.seedRun {
				_ = store.CreateRun(context.Background(), command.ReconRunRecord{
					ID:       tc.runID,
					TenantID: tfrTenant,
					LedgerID: tfrLedger,
					Source:   "bank-csv",
					Status:   tc.runStatus,
				})
			}

			res, err := svc.ExecuteRun(context.Background(), tfrTenant, tc.runID, command.MatchInput{
				Ledger: []service.LedgerFact{
					{PostingID: "p-1", Reference: "ref-1", AmountMinor: 5000, AssetCode: "USD", EffectiveAt: opsAt},
				},
				External: []entity.ExternalStatementLine{
					reconLine("l-1", "ref-1", 5000),
					reconLine("l-2", "ref-2", 100),
				},
				Config: service.MatchConfig{
					RuleVersion: "v1", TimingWindow: time.Hour, Actor: "u-1",
					Rail: "ACH", RunID: tc.runID, TenantID: string(tfrTenant),
					Tolerances: []service.TolerancePolicy{{Rail: "ACH", AssetCode: "USD", ToleranceMinor: 0}},
				},
			})
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, command.ReconRunCompleted, res.Status)
				assert.Len(t, store.breaks, 1)
				assert.Contains(t, outboxTypes(uow), command.EventReconRunCompleted)
				assert.Contains(t, outboxTypes(uow), command.EventReconBreakFound)
			}
		})
	}
}

func TestBreakResolveAndAcknowledge(t *testing.T) {
	t.Parallel()

	baseReq := port.BreakResolutionRequest{
		TenantID:            tfrTenant,
		BreakID:             "br-1",
		Approver:            "u-2",
		Decision:            "adjust",
		Note:                "ev-1",
		Actor:               "u-1",
		AdjustmentPostingID: "p-9",
		IdempotencyKey:      "key-resolve-1",
	}

	type testCase struct {
		name          string
		req           port.BreakResolutionRequest
		acknowledge   bool
		seedBreak     bool
		breakAmount   int64
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid resolve decides with fact",
			req:           baseReq,
			acknowledge:   false,
			seedBreak:     true,
			breakAmount:   50000,
			denied:        false,
			expectedError: nil,
		},
		{
			name:          "acknowledge marks reviewed without adjustment",
			req:           baseReq,
			acknowledge:   true,
			seedBreak:     true,
			breakAmount:   5000,
			denied:        false,
			expectedError: nil,
		},
		{
			name: "missing tenant returns TENANT_REQUIRED",
			req: func() port.BreakResolutionRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			acknowledge:   false,
			seedBreak:     false,
			breakAmount:   0,
			denied:        false,
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing break ID returns BREAK_ID_REQUIRED",
			req: func() port.BreakResolutionRequest {
				r := baseReq
				r.BreakID = ""
				return r
			}(),
			acknowledge:   false,
			seedBreak:     false,
			breakAmount:   0,
			denied:        false,
			expectedError: entity.NewError("BREAK_ID_REQUIRED", "break id is required"),
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			req: func() port.BreakResolutionRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			acknowledge:   false,
			seedBreak:     false,
			breakAmount:   0,
			denied:        false,
			expectedError: entity.NewError("ACTOR_REQUIRED", "reconciliation actor is required"),
		},
		{
			name: "missing idempotency key returns IDEMPOTENCY_KEY_REQUIRED",
			req: func() port.BreakResolutionRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			acknowledge:   false,
			seedBreak:     false,
			breakAmount:   0,
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "reconciliation requires an idempotency key"),
		},
		{
			name:          "break not found returns BREAK_NOT_FOUND",
			req:           baseReq,
			acknowledge:   false,
			seedBreak:     false,
			breakAmount:   0,
			denied:        false,
			expectedError: entity.NewError("BREAK_NOT_FOUND", "break is unknown"),
		},
		{
			name: "self approval above threshold forbidden by SoD",
			req: func() port.BreakResolutionRequest {
				r := baseReq
				r.Approver = r.Actor
				return r
			}(),
			acknowledge:   false,
			seedBreak:     true,
			breakAmount:   50000,
			denied:        false,
			expectedError: entity.NewError("SELF_APPROVAL_FORBIDDEN", "adjustment approver must differ from maker"),
		},
		{
			name:          "denied subject returns FORBIDDEN",
			req:           baseReq,
			acknowledge:   false,
			seedBreak:     true,
			breakAmount:   50000,
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &reconStoreFake{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied[tc.req.Actor+"|recon.resolve|recon/"+tc.req.BreakID] = true
			}
			svc := newReconService(uow, store, authz)

			if tc.seedBreak {
				_ = store.CreateBreaks(context.Background(), []command.BreakRecord{
					{
						Break: entity.ReconciliationBreak{
							BreakID:       tc.req.BreakID,
							RunID:         "run-1",
							TenantID:      tc.req.TenantID.String(),
							Type:          valueobject.BreakAmountMismatch,
							LedgerRef:     "tx-1",
							ExternalRef:   "ext-1",
							ExpectedMinor: tc.breakAmount,
							ActualMinor:   0,
							RuleVersion:   "v1",
						},
						Status: valueobject.BreakOpen,
					},
				})
			}

			var err error
			if tc.acknowledge {
				err = svc.AcknowledgeBreak(context.Background(), tc.req)
			} else {
				err = svc.ResolveBreak(context.Background(), tc.req)
			}

			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				stored, findErr := store.FindBreak(context.Background(), tc.req.TenantID, tc.req.BreakID)
				require.NoError(t, findErr)
				if tc.acknowledge {
					assert.Equal(t, valueobject.BreakAcknowledged, stored.Status)
					assert.Contains(t, outboxTypes(uow), command.EventReconBreakAcknowledged)
				} else {
					assert.Equal(t, valueobject.BreakResolved, stored.Status)
					assert.Contains(t, outboxTypes(uow), command.EventReconBreakResolved)
				}

				// Idempotency replay
				if tc.acknowledge {
					assert.NoError(t, svc.AcknowledgeBreak(context.Background(), tc.req))
				} else {
					assert.NoError(t, svc.ResolveBreak(context.Background(), tc.req))
				}
			}
		})
	}
}
