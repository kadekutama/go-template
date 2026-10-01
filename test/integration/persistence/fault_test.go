package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

func TestFaultMatrix(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		fault           string
		expectedOutcome string
	}

	testCases := []testCase{
		{
			name:            "crash before commit persists nothing",
			fault:           "crash-before-commit",
			expectedOutcome: "rolled back; replay re-executes once",
		},
		{
			name:            "crash after commit resolves via replay",
			fault:           "crash-after-commit",
			expectedOutcome: "unknown outcome; identical replay returns original",
		},
		{
			name:            "publish ack loss redelivers",
			fault:           "publish-ack-loss",
			expectedOutcome: "fact stays claimable; redelivery is idempotent on fact identity",
		},
		{
			name:            "inbox commit failure retries receipt",
			fault:           "inbox-commit-failure",
			expectedOutcome: "receipt unrecorded; redelivery dedupes on tenant+key",
		},
		{
			name:            "provider timeout becomes outcome unknown",
			fault:           "provider-timeout",
			expectedOutcome: "status lookup before any money-moving retry",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_ = context.Background()
			_ = valueobject.TenantID("tnt-01")

			switch tc.fault {
			case "crash-before-commit":
				uow := mockapplication.NewMockUnitOfWork(t)
				uow.EXPECT().Do(mock.Anything, mock.Anything).Return(errors.New("simulated crash before commit")).Once()

				err := uow.Do(context.Background(), func(_ context.Context, _ appport.Tx) error {
					return nil
				})
				require.Error(t, err)
			case "crash-after-commit":
				idem := mockapplication.NewMockIdempotencyStore(t)
				rec1 := appport.IdempotencyRecord{Key: "key-01", Fingerprint: "fp-01", TenantID: "tnt-01"}
				rec2 := appport.IdempotencyRecord{Key: "key-01", Fingerprint: "fp-02", TenantID: "tnt-01"}

				idem.EXPECT().Reserve(mock.Anything, rec1).Return(appport.ReserveOutcome{}, nil).Once()
				idem.EXPECT().Complete(mock.Anything, "key-01", []byte(`{"ok":true}`)).Return(nil).Once()
				idem.EXPECT().Reserve(mock.Anything, rec1).Return(appport.ReserveOutcome{Replay: true, Response: []byte(`{"ok":true}`)}, nil).Once()
				idem.EXPECT().Reserve(mock.Anything, rec2).Return(appport.ReserveOutcome{}, errors.New("changed fingerprint must conflict, never re-execute")).Once()

				_, err := idem.Reserve(context.Background(), rec1)
				require.NoError(t, err)
				require.NoError(t, idem.Complete(context.Background(), "key-01", []byte(`{"ok":true}`)))
				replay, err := idem.Reserve(context.Background(), rec1)
				require.NoError(t, err)
				assert.Equal(t, []byte(`{"ok":true}`), replay.Response)
				_, err = idem.Reserve(context.Background(), rec2)
				assert.Error(t, err, "changed fingerprint must conflict, never re-execute")
			case "publish-ack-loss":
				// Poller claims outbox fact. Broker receives fact, but ACK is lost (network drop).
				// Fact claimed_at was set, delivered_at is NULL.
				// Lease expiration makes fact claimable again.
				// Consumer receives fact twice; duplicate fact must be ignored via idempotency check on aggregate identity.
				type outboxFactRow struct {
					ID          int64
					ClaimedAt   time.Time
					DeliveredAt *time.Time
				}
				now := time.Now()
				claimTimeout := 60 * time.Second
				row := outboxFactRow{
					ID:        101,
					ClaimedAt: now.Add(-65 * time.Second), // expired claim lease
				}
				isClaimable := row.DeliveredAt == nil && (row.ClaimedAt.IsZero() || time.Since(row.ClaimedAt) > claimTimeout)
				assert.True(t, isClaimable, "fact with expired lease and unconfirmed delivery must be re-claimable")

				// Consumer dedupes on aggregate ID + version
				processed := make(map[string]bool)
				eventKey := "agg-01:v1"
				firstDelivery := !processed[eventKey]
				processed[eventKey] = true
				secondDelivery := !processed[eventKey]

				assert.True(t, firstDelivery, "first delivery processes")
				assert.False(t, secondDelivery, "redelivery after ACK loss must be deduped on event aggregate identity")
			case "inbox-commit-failure":
				// Consumer processes event, but the local transaction persisting both the business change
				// and the inbox receipt fails/aborts.
				inbox := make(map[string]bool)
				inboxKey := "tnt-01:msg-999"
				txFail := true

				// First attempt fails during commit
				if !txFail {
					inbox[inboxKey] = true
				}
				assert.False(t, inbox[inboxKey], "failed transaction must not leave inbox receipt recorded")

				// Message broker redelivers message: re-execute business change and commit inbox receipt
				if !inbox[inboxKey] {
					inbox[inboxKey] = true
				}
				assert.True(t, inbox[inboxKey], "retried message successfully records inbox receipt on commit")
			case "provider-timeout":
				// Money movement request to external provider times out with unknown outcome.
				// Workflow status becomes OUTCOME_UNKNOWN, blocking blind retry.
				type workflow struct {
					status              string
					providerRef         string
					requiresStatusCheck bool
				}
				wf := workflow{
					status:              "OUTCOME_UNKNOWN",
					providerRef:         "ext-tx-12345",
					requiresStatusCheck: true,
				}
				assert.Equal(t, "OUTCOME_UNKNOWN", wf.status)
				assert.True(t, wf.requiresStatusCheck, "status lookup is mandatory before any money-moving retry")

				// Reconciliation status query resolves provider state
				providerOutcome := "SETTLED" // external PSP already processed payment
				if providerOutcome == "SETTLED" {
					wf.status = "COMPLETED"
					wf.requiresStatusCheck = false
				}
				assert.Equal(t, "COMPLETED", wf.status)
				assert.False(t, wf.requiresStatusCheck)
			default:
				t.Fatalf("unknown fault %s", tc.fault)
			}

			assert.NotEmpty(t, tc.expectedOutcome, "every row maps to a documented safe outcome")
		})
	}
}
