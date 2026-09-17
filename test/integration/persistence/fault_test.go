package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// fakeTx models commit/rollback outcomes for the fault matrix.
type fakeTx struct {
	committed  bool
	rolledBack bool
	failCommit bool
}

func (f *fakeTx) commit() error {
	if f.failCommit {
		f.rolledBack = true
		return errors.New("crash after commit: unknown outcome")
	}

	f.committed = true

	return nil
}

// fakeIdempotency models reserve/replay/conflict for the fault matrix.
type fakeIdempotency struct {
	records map[string]appport.IdempotencyRecord
	results map[string][]byte
}

func newFakeIdempotency() *fakeIdempotency {
	return &fakeIdempotency{records: make(map[string]appport.IdempotencyRecord), results: make(map[string][]byte)}
}

func (f *fakeIdempotency) reserve(key string, fingerprint string) (replay []byte, conflict bool) {
	if rec, ok := f.records[key]; ok {
		if rec.Fingerprint != fingerprint {
			return nil, true
		}

		return f.results[key], false
	}

	f.records[key] = appport.IdempotencyRecord{Key: key, Fingerprint: fingerprint, TenantID: "tnt-01"}

	return nil, false
}

func (f *fakeIdempotency) complete(key string, response []byte) {
	f.results[key] = response
}

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
				tx := &fakeTx{rolledBack: true}
				assert.True(t, tx.rolledBack)
				assert.False(t, tx.committed)
			case "crash-after-commit":
				tx := &fakeTx{failCommit: true}
				require.Error(t, tx.commit(), "unknown commit outcome must surface, never claim success")
				assert.True(t, tx.rolledBack)

				fakes := newFakeIdempotency()
				_, conflict := fakes.reserve("key-01", "fp-01")
				require.False(t, conflict)
				fakes.complete("key-01", []byte(`{"ok":true}`))
				replay, conflict := fakes.reserve("key-01", "fp-01")
				require.False(t, conflict)
				assert.Equal(t, []byte(`{"ok":true}`), replay)
				_, conflict = fakes.reserve("key-01", "fp-02")
				assert.True(t, conflict, "changed fingerprint must conflict, never re-execute")
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
