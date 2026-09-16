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
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
)

var disputeAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

func disputePolicies() map[string]service.NetworkPolicy {
	return map[string]service.NetworkPolicy{
		"VISA": {
			Network: "VISA", Version: "2026.1", DisputeWindowDays: 120,
			EvidenceDays: 7, MaxRepresentments: 1, FeeMinor: 1500,
		},
	}
}

type disputeStoreFake struct {
	mu       sync.Mutex
	disputes map[string]entity.Dispute
}

func (s *disputeStoreFake) CreateDispute(_ context.Context, dispute entity.Dispute) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disputes == nil {
		s.disputes = map[string]entity.Dispute{}
	}
	s.disputes[dispute.ID] = dispute
	return nil
}

func (s *disputeStoreFake) FindDispute(_ context.Context, id string) (entity.Dispute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dispute, ok := s.disputes[id]
	if !ok {
		return entity.Dispute{}, entity.NewError("DISPUTE_NOT_FOUND", "dispute is unknown")
	}
	return dispute, nil
}

func (s *disputeStoreFake) UpdateDispute(_ context.Context, dispute entity.Dispute) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disputes[dispute.ID] = dispute
	return nil
}

func (s *disputeStoreFake) ListDisputes(_ context.Context, status string, from, to time.Time, _ string, _ int) ([]entity.Dispute, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []entity.Dispute
	for _, dispute := range s.disputes {
		if status != "" && string(dispute.Status) != status {
			continue
		}
		if !from.IsZero() && dispute.OpenedAt.Before(from) {
			continue
		}
		if !to.IsZero() && !dispute.OpenedAt.Before(to) {
			continue
		}
		out = append(out, dispute)
	}
	return out, "", nil
}

type disputeClock struct {
	now time.Time
}

func (c *disputeClock) Now() time.Time { return c.now }

func newDisputeService(uow *tfrUOW, store *disputeStoreFake, authz *tfrAuthz, clock *disputeClock) *command.DisputeService {
	return command.NewDisputeService(command.DisputeServiceParams{
		UoW:               uow,
		Disputes:          store,
		Policies:          disputePolicies(),
		SoDThresholdMinor: 10000,
		Clock:             clock,
		IDs:               &tfrIDs{next: tfrTestIDs(40)},
		Authz:             authz,
	})
}

func openDisputeCommand() port.OpenDisputeCommand {
	return port.OpenDisputeCommand{
		PaymentID:         "pay-1",
		OriginalPostingID: "p-1",
		Network:           "VISA",
		AmountMinor:       50000,
		PaymentAt:         disputeAt,
		Actor:             "u-1",
		IdempotencyKey:    "key-dispute-1",
	}
}

func TestDisputeOpen(t *testing.T) {
	t.Parallel()

	baseCmd := openDisputeCommand()

	type testCase struct {
		name           string
		cmd            port.OpenDisputeCommand
		preload        func(uow *tfrUOW)
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid open holds funds with deadline and fee",
			cmd:            baseCmd,
			preload:        func(_ *tfrUOW) {},
			denied:         false,
			expectedStatus: "OPEN",
			expectedError:  nil,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			cmd:  baseCmd,
			preload: func(uow *tfrUOW) {
				fp := command.Fingerprint(
					baseCmd.IdempotencyKey, baseCmd.PaymentID, baseCmd.OriginalPostingID,
					baseCmd.Network, fmt.Sprintf("%d", baseCmd.AmountMinor),
				)
				uow.idem = map[string]tfrIdemEntry{
					baseCmd.IdempotencyKey: {
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
			name: "missing payment ID rejected",
			cmd: func() port.OpenDisputeCommand {
				c := baseCmd
				c.PaymentID = ""
				return c
			}(),
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("DISPUTE_ID_REQUIRED", "dispute requires id, payment, and original posting"),
		},
		{
			name: "missing original posting ID rejected",
			cmd: func() port.OpenDisputeCommand {
				c := baseCmd
				c.OriginalPostingID = ""
				return c
			}(),
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("DISPUTE_ID_REQUIRED", "dispute requires id, payment, and original posting"),
		},
		{
			name: "missing network rejected",
			cmd: func() port.OpenDisputeCommand {
				c := baseCmd
				c.Network = ""
				return c
			}(),
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("DISPUTE_NETWORK_REQUIRED", "dispute requires a network"),
		},
		{
			name: "zero dispute amount rejected",
			cmd: func() port.OpenDisputeCommand {
				c := baseCmd
				c.AmountMinor = 0
				return c
			}(),
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INVALID_DISPUTE_AMOUNT", "dispute amount must be positive"),
		},
		{
			name: "missing actor rejected",
			cmd: func() port.OpenDisputeCommand {
				c := baseCmd
				c.Actor = ""
				return c
			}(),
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("ACTOR_REQUIRED", "dispute actor is required"),
		},
		{
			name: "missing idempotency key rejected",
			cmd: func() port.OpenDisputeCommand {
				c := baseCmd
				c.IdempotencyKey = ""
				return c
			}(),
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "dispute requires an idempotency key"),
		},
		{
			name: "unknown network policy rejected",
			cmd: func() port.OpenDisputeCommand {
				c := baseCmd
				c.Network = "UNKNOWN"
				return c
			}(),
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("DISPUTE_POLICY_UNKNOWN", "no network policy for UNKNOWN"),
		},
		{
			name: "dispute window expired rejected",
			cmd: func() port.OpenDisputeCommand {
				c := baseCmd
				c.PaymentAt = disputeAt.Add(-130 * 24 * time.Hour)
				return c
			}(),
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("DISPUTE_WINDOW_EXPIRED", "dispute window has expired"),
		},
		{
			name:           "denied subject returns FORBIDDEN",
			cmd:            baseCmd,
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			if tc.preload != nil {
				tc.preload(uow)
			}
			store := &disputeStoreFake{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied[tc.cmd.Actor+"|dispute.open|payment/"+tc.cmd.PaymentID] = true
			}
			clock := &disputeClock{now: disputeAt}
			svc := newDisputeService(uow, store, authz, clock)

			res, err := svc.OpenDispute(context.Background(), tc.cmd)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, string(res.Dispute.Status))
				assert.Equal(t, int64(1500), res.Dispute.FeeMinor)

				// Idempotency replay
				replayRes, replayErr := svc.OpenDispute(context.Background(), tc.cmd)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.Dispute.ID, replayRes.Dispute.ID)
			}
		})
	}
}

func TestDisputeSubmitEvidence(t *testing.T) {
	t.Parallel()

	baseEvidence := port.EvidenceCommand{
		DisputeID:      "disp-1",
		Actor:          "u-1",
		IdempotencyKey: "key-evidence-1",
	}

	type testCase struct {
		name           string
		cmd            port.EvidenceCommand
		advanceTime    time.Duration
		seedDispute    bool
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "evidence on time moves dispute to UNDER_REVIEW",
			cmd:            baseEvidence,
			advanceTime:    0,
			seedDispute:    true,
			denied:         false,
			expectedStatus: "UNDER_REVIEW",
			expectedError:  nil,
		},
		{
			name: "missing dispute ID rejected",
			cmd: func() port.EvidenceCommand {
				c := baseEvidence
				c.DisputeID = ""
				return c
			}(),
			advanceTime:    0,
			seedDispute:    false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("DISPUTE_ID_REQUIRED", "dispute id is required"),
		},
		{
			name: "missing actor rejected",
			cmd: func() port.EvidenceCommand {
				c := baseEvidence
				c.Actor = ""
				return c
			}(),
			advanceTime:    0,
			seedDispute:    false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("ACTOR_REQUIRED", "dispute actor is required"),
		},
		{
			name: "missing idempotency key rejected",
			cmd: func() port.EvidenceCommand {
				c := baseEvidence
				c.IdempotencyKey = ""
				return c
			}(),
			advanceTime:    0,
			seedDispute:    false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "dispute requires an idempotency key"),
		},
		{
			name:           "dispute not found returns DISPUTE_NOT_FOUND",
			cmd:            baseEvidence,
			advanceTime:    0,
			seedDispute:    false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("DISPUTE_NOT_FOUND", "dispute is unknown"),
		},
		{
			name:           "late evidence rejected with window code",
			cmd:            baseEvidence,
			advanceTime:    30 * 24 * time.Hour,
			seedDispute:    true,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("EVIDENCE_WINDOW_EXPIRED", "dispute evidence window has expired"),
		},
		{
			name:           "denied subject returns FORBIDDEN",
			cmd:            baseEvidence,
			advanceTime:    0,
			seedDispute:    true,
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &disputeStoreFake{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			clock := &disputeClock{now: disputeAt}
			svc := newDisputeService(uow, store, authz, clock)
			if tc.seedDispute {
				opened, err := svc.OpenDispute(context.Background(), openDisputeCommand())
				require.NoError(t, err)
				if tc.cmd.DisputeID == "disp-1" {
					tc.cmd.DisputeID = opened.Dispute.ID
				}
			}

			if tc.denied {
				authz.denied[tc.cmd.Actor+"|dispute.evidence|dispute/"+tc.cmd.DisputeID] = true
			}

			if tc.advanceTime > 0 {
				clock.now = disputeAt.Add(tc.advanceTime)
			}

			res, err := svc.SubmitEvidence(context.Background(), tc.cmd)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, string(res.Dispute.Status))

				// Idempotency replay
				replayRes, replayErr := svc.SubmitEvidence(context.Background(), tc.cmd)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.Dispute.ID, replayRes.Dispute.ID)
			}
		})
	}
}

func TestDisputeRepresentAndClose(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		action         string
		outcome        string
		actor          string
		approver       string
		key            string
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid representment increments stage",
			action:         "represent",
			outcome:        "",
			actor:          "u-1",
			approver:       "",
			key:            "key-rep-1",
			denied:         false,
			expectedStatus: "UNDER_REVIEW",
			expectedError:  nil,
		},
		{
			name:           "above-threshold close requires distinct approver by SoD",
			action:         "close",
			outcome:        "LOST",
			actor:          "u-1",
			approver:       "u-1",
			key:            "key-close-sod",
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("SOD_VIOLATION", "above-threshold close requires a different approver"),
		},
		{
			name:           "above-threshold close with distinct approver succeeds as LOST",
			action:         "close",
			outcome:        "LOST",
			actor:          "u-1",
			approver:       "u-2",
			key:            "key-close-lost",
			denied:         false,
			expectedStatus: "LOST",
			expectedError:  nil,
		},
		{
			name:           "above-threshold close with distinct approver succeeds as WON",
			action:         "close",
			outcome:        "WON",
			actor:          "u-1",
			approver:       "u-2",
			key:            "key-close-won",
			denied:         false,
			expectedStatus: "WON",
			expectedError:  nil,
		},
		{
			name:           "invalid outcome rejected",
			action:         "close",
			outcome:        "MAYBE",
			actor:          "u-1",
			approver:       "u-2",
			key:            "key-close-maybe",
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("DISPUTE_OUTCOME_INVALID", "dispute outcome must be WON or LOST"),
		},
		{
			name:           "denied subject on close returns FORBIDDEN",
			action:         "close",
			outcome:        "WON",
			actor:          "u-1",
			approver:       "u-2",
			key:            "key-close-denied",
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &disputeStoreFake{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			clock := &disputeClock{now: disputeAt}
			svc := newDisputeService(uow, store, authz, clock)

			// Setup an under_review dispute
			opened, err := svc.OpenDispute(context.Background(), openDisputeCommand())
			require.NoError(t, err)
			_, err = svc.SubmitEvidence(context.Background(), port.EvidenceCommand{
				DisputeID: opened.Dispute.ID, Actor: "u-1", IdempotencyKey: "key-ev-seed",
			})
			require.NoError(t, err)

			disputeID := opened.Dispute.ID
			if tc.denied {
				authz.denied[tc.actor+"|dispute."+tc.action+"|dispute/"+disputeID] = true
			}

			switch tc.action {
			case "represent":
				res, err := svc.RepresentDispute(context.Background(), port.RepresentCommand{
					DisputeID: disputeID, Actor: tc.actor, IdempotencyKey: tc.key,
				})
				assert.Equal(t, tc.expectedError, err)
				if tc.expectedError == nil {
					assert.Equal(t, 1, res.Dispute.RepresentStage)

					// Exhausted check
					_, secondErr := svc.RepresentDispute(context.Background(), port.RepresentCommand{
						DisputeID: disputeID, Actor: tc.actor, IdempotencyKey: tc.key + "-2",
					})
					assert.Equal(t, entity.NewError("REPRESENTMENT_EXHAUSTED", "representment allowance is exhausted"), secondErr)
				}
			case "close":
				res, err := svc.CloseDispute(context.Background(), port.CloseDisputeCommand{
					DisputeID: disputeID, Outcome: tc.outcome, Actor: tc.actor, Approver: tc.approver, IdempotencyKey: tc.key,
				})
				assert.Equal(t, tc.expectedError, err)
				if tc.expectedError == nil {
					assert.Equal(t, tc.expectedStatus, string(res.Dispute.Status))

					// Idempotency replay
					replayRes, replayErr := svc.CloseDispute(context.Background(), port.CloseDisputeCommand{
						DisputeID: disputeID, Outcome: tc.outcome, Actor: tc.actor, Approver: tc.approver, IdempotencyKey: tc.key,
					})
					assert.NoError(t, replayErr)
					assert.Equal(t, res.Dispute.ID, replayRes.Dispute.ID)
				}
			}
		})
	}
}
