package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// dispatcherGroup is this consumer's inbox receipt namespace.
const dispatcherGroup = "webhook-dispatcher"

// HTTPSender delivers one signed webhook request. Production uses an HTTP
// client with timeouts + per-endpoint breaking; tests use a fake.
type HTTPSender interface {
	Send(ctx context.Context, url string, headers map[string]string, body []byte) error
}

// EndpointLookup scopes endpoints by tenant+event (Registry implements it).
type EndpointLookup interface {
	Find(ctx context.Context, tenant, event string) []webhook.Endpoint
}

// DispatcherParams carries constructor dependencies (Parameter Object pattern).
// RetryPolicy is required: wiring passes the configured policy
// (config.WebhookConfig.RetryPolicy(), which supplies the api-contracts §11
// default when no override is set), so retry behavior is never hardcoded
// inside the dispatcher.
type DispatcherParams struct {
	Endpoints   EndpointLookup
	Sender      HTTPSender
	Receipts    ReceiptStore
	DLQ         webhook.DLQSink
	Logger      log.Logger
	RetryPolicy *webhook.RetryPolicy
}

// Dispatcher implements port.WebhookDispatcher with per-event idempotency,
// api-contracts §11 HMAC signing, per-endpoint retry accounting, and
// endpoint isolation (one failure never blocks others). Each Dispatch call
// makes one attempt per endpoint: a failed attempt returns a
// *webhook.RetryableError carrying the NextDelay for the E14 worker to
// re-queue, and when several endpoints fail in one dispatch the errors are
// aggregated with errors.Join (errors.As still finds each RetryableError).
// The policy's MaxAttempts-th consecutive failure per endpoint (initial
// delivery + configured backoffs) routes that endpoint to DLQ.
// Waiting and re-queueing belong to E14; this task decides retry-vs-DLQ per
// attempt.
type Dispatcher struct {
	endpoints EndpointLookup
	sender    HTTPSender
	receipts  ReceiptStore
	dlq       webhook.DLQSink
	logger    log.Logger
	retry     *webhook.RetryPolicy
	breakers  *breakerSet
	// attempts is write+read per dispatch (next attempt, then record), so a
	// plain Mutex beats RWMutex; no read-dominant path exists on this map.
	mu       sync.Mutex
	attempts map[string]int
}

// Compile-time port conformance.
var _ appport.WebhookDispatcher = (*Dispatcher)(nil)

// NewDispatcher builds the delivery group consumer.
func NewDispatcher(params DispatcherParams) (*Dispatcher, error) {
	if params.Endpoints == nil {
		return nil, fmt.Errorf("dispatcher: endpoint lookup is required")
	}

	if params.Sender == nil {
		return nil, fmt.Errorf("dispatcher: sender is required")
	}

	if params.Receipts == nil {
		return nil, fmt.Errorf("dispatcher: receipt store is required")
	}

	if params.DLQ == nil {
		return nil, fmt.Errorf("dispatcher: dlq sink is required")
	}

	if params.RetryPolicy == nil {
		return nil, fmt.Errorf("dispatcher: retry policy is required")
	}

	return &Dispatcher{
		endpoints: params.Endpoints,
		sender:    params.Sender,
		receipts:  params.Receipts,
		dlq:       params.DLQ,
		logger:    params.Logger,
		retry:     params.RetryPolicy,
		breakers:  newBreakerSet(),
		attempts:  make(map[string]int),
	}, nil
}

// Dispatch enqueues one webhook message for delivery. It is idempotent per
// (endpoint, event payload identity): callers pass the event ID inside the
// payload, and duplicates ack without re-sending.
//
// Each call makes one attempt per endpoint. A failed attempt releases its
// receipt so a re-queued delivery (E14 worker redelivery or process restart)
// can retry instead of being silently deduped; a successful attempt keeps the
// receipt, which is what makes the second identical Dispatch a no-op.
func (d *Dispatcher) Dispatch(ctx context.Context, message appport.WebhookMessage) error {
	tenant, event, err := validateDispatch(d, message)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("dispatcher: dispatch: %w", err)
	}

	endpoints := d.endpoints.Find(ctx, tenant, event)
	if len(endpoints) == 0 {
		return nil
	}

	return d.deliverAll(ctx, endpoints, tenant, event, message.Payload)
}

func validateDispatch(d *Dispatcher, message appport.WebhookMessage) (string, string, error) {
	if d == nil || d.endpoints == nil || d.sender == nil || d.receipts == nil {
		return "", "", fmt.Errorf("dispatcher: not initialized")
	}

	tenant := strings.TrimSpace(message.TenantID.String())
	event := strings.TrimSpace(message.EventType)

	if tenant == "" {
		return "", "", fmt.Errorf("dispatcher: tenant is required")
	}

	if event == "" {
		return "", "", fmt.Errorf("dispatcher: event type is required")
	}

	if message.Payload == nil {
		return "", "", fmt.Errorf("dispatcher: payload is required")
	}

	return tenant, event, nil
}

// delivery is one endpoint delivery attempt: the stable event identity plus
// the retry accounting needed to decide retry-versus-DLQ.
type delivery struct {
	endpoint webhook.Endpoint
	tenant   string
	event    string
	base     string
	attempt  int
	payload  []byte
}

func (d *Dispatcher) deliverAll(ctx context.Context, endpoints []webhook.Endpoint, tenant, event string, payload []byte) error {
	var errs []error

	for _, endpoint := range endpoints {
		if err := d.deliverOne(ctx, endpoint, tenant, event, payload); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (d *Dispatcher) deliverOne(ctx context.Context, endpoint webhook.Endpoint, tenant, event string, payload []byte) error {
	if d.breakers.open(endpoint.ID) {
		return nil
	}

	eventID := eventIdentity(payload)
	job := delivery{
		endpoint: endpoint,
		tenant:   tenant,
		event:    event,
		base:     endpoint.ID + ":" + tenant + ":" + event + ":" + eventID,
		payload:  payload,
	}
	job.attempt = d.nextAttempt(job.base)

	// The receipt key is the stable delivery identity (not the attempt): a
	// re-queued delivery must reach the endpoint again, so it only stays
	// claimed once an attempt has actually succeeded.
	duplicate, err := d.receipts.Claim(ctx, dispatcherGroup, job.base)
	if err != nil {
		return err
	}

	if duplicate {
		return nil
	}

	if err := d.sendOne(ctx, endpoint, eventID, payload); err != nil {
		return d.recordFailure(ctx, job, err)
	}

	d.clearAttempt(job.base)
	d.breakers.succeed(endpoint.ID)

	return nil
}

// recordFailure accounts one failed attempt: attempts below the policy's
// MaxAttempts return a RetryableError for the E14 worker to re-queue after
// NextDelay; the MaxAttempts-th consecutive failure routes to DLQ and parks
// the endpoint until operator reset.
//
// The delivery receipt is released so the re-queue is not mistaken for a
// duplicate of the failed attempt. A failed release is joined with the retry
// decision (errors.As still finds RetryableError) and needs operator
// reconciliation, because the next delivery would otherwise ack as duplicate.
func (d *Dispatcher) recordFailure(ctx context.Context, job delivery, sendErr error) error {
	d.mu.Lock()
	d.attempts[job.base] = job.attempt
	d.mu.Unlock()

	outcome := d.failureOutcome(ctx, job, sendErr)

	if err := d.receipts.Release(ctx, dispatcherGroup, job.base); err != nil {
		return errors.Join(outcome, fmt.Errorf("dispatcher: release receipt: %w", err))
	}

	return outcome
}

// failureOutcome returns the RetryableError for a further attempt, or records
// the exhausted delivery to DLQ and returns nil (ack).
func (d *Dispatcher) failureOutcome(ctx context.Context, job delivery, sendErr error) error {
	if job.attempt < d.retry.MaxAttempts() {
		delay, _ := d.retry.NextDelay(job.attempt - 1)

		return fmt.Errorf("dispatcher: send %s: %w", job.endpoint.ID,
			&webhook.RetryableError{EndpointID: job.endpoint.ID, Attempt: job.attempt, NextDelay: delay, Cause: sendErr})
	}

	if err := d.dlq.Record(ctx, webhook.DLQMessage{
		EndpointID: job.endpoint.ID,
		Tenant:     job.tenant,
		Event:      job.event,
		Payload:    job.payload,
		Attempts:   job.attempt,
	}); err != nil {
		return err
	}

	d.clearAttempt(job.base)
	d.breakers.forceOpen(job.endpoint.ID)

	if d.logger != nil {
		d.logger.Error(ctx, "dispatcher.endpoint.parked",
			"endpoint_id", job.endpoint.ID, "attempts", job.attempt, "cause", sendErr)
	}

	return nil
}

func (d *Dispatcher) nextAttempt(base string) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.attempts[base] + 1
}

func (d *Dispatcher) clearAttempt(base string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.attempts, base)
}

func (d *Dispatcher) sendOne(ctx context.Context, endpoint webhook.Endpoint, eventID string, payload []byte) error {
	signer, err := webhook.NewSigner(webhook.SignerParams{Secrets: []string{endpoint.Secret}})
	if err != nil {
		return err
	}

	stamp := time.Now().UTC()
	signature := signer.Sign(payload, stamp)

	headers := map[string]string{
		"Content-Type":       "application/json",
		"X-Ledger-Signature": signature,
		"X-Ledger-Timestamp": webhook.TimestampHeader(stamp),
		"X-Ledger-Event-ID":  eventID,
		"X-Ledger-Key-ID":    "k1",
	}

	if err := d.sender.Send(ctx, strings.TrimSpace(endpoint.URL), headers, payload); err != nil {
		return fmt.Errorf("dispatcher: send %s: %w", endpoint.ID, err)
	}

	return nil
}

// breakerSet opens poison endpoints after DLQ routing (see recordFailure)
// so one dead receiver never blocks the group. Success clears the flag.
// Operator reset for a recovered endpoint is E11 webhook-mgmt follow-up.
// RWMutex: open() is the hot read path (every endpoint of every dispatch);
// forceOpen/succeed are rare writes (DLQ routing, success).
type breakerSet struct {
	mu     sync.RWMutex
	opened map[string]bool
}

func newBreakerSet() *breakerSet {
	return &breakerSet{opened: make(map[string]bool)}
}

func (b *breakerSet) open(id string) bool {
	if b == nil {
		return false
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.opened[id]
}

// forceOpen parks an endpoint after DLQ routing until operator reset.
func (b *breakerSet) forceOpen(id string) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.opened[id] = true
}

func (b *breakerSet) succeed(id string) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.opened[id] = false
}

// eventIdentity is the stable delivery identity: SHA-256 over the exact
// payload bytes, doubling as X-Ledger-Event-ID (port carries no event ID,
// and identical bytes are the same event).
func eventIdentity(payload []byte) string {
	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:])
}
