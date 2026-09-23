// Package consumer is the idempotent Redpanda consumer framework (E08-T04):
// envelope validation → inbox receipt claim → read-side handler → ack, with
// retry/backoff then DLQ. Handlers apply read-side effects only: event
// replay never re-runs ledger commands. Trace headers propagate both ways.
//
// Lifecycle: Handle is fully synchronous and this package spawns no
// goroutines, so there is nothing to drain — graceful shutdown lives with
// the E14 subscribe loops that call Handle (drain in-flight handlers, then
// return), not here.
package consumer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// DefaultMaxDelivers bounds redelivery before DLQ routing.
const DefaultMaxDelivers = 5

// ReceiptStore dedupes on (consumer, event_id). Production backs it with
// the inbox_receipts table (E07 schema) plus the Valkey hint store; tests
// use the fakes package double and ValkeyReceiptStore (hint + TTL,
// documented in idempotent.go).
//
// The receipt follows the domain-events §5.2 inbox lifecycle: Claim is the
// claim inside the handler's transaction, and Release is its rollback. An
// implementation whose Claim already committed MUST still honour Release
// (delete the receipt) so a redelivery can re-run the handler; otherwise a
// single transient handler failure would permanently swallow the event.
type ReceiptStore interface {
	Claim(ctx context.Context, consumer, eventID string) (duplicate bool, err error)
	Release(ctx context.Context, consumer, eventID string) error
}

// FrameworkParams carries constructor dependencies (Parameter Object pattern).
// Data fields declare validate tags (checked with joined violations);
// dependency seams are nil-checked explicitly, since interfaces cannot be
// expressed as tags.
type FrameworkParams struct {
	Consumer    string       `validate:"required"`
	Receipts    ReceiptStore `validate:"-"`
	DLQ         DLQSink      `validate:"-"`
	MaxDelivers int          `validate:"omitempty,gt=0"`
	Logger      log.Logger   `validate:"-"`
}

// Framework runs one consumer-group handler with effectively-once effects.
type Framework struct {
	consumer    string
	receipts    ReceiptStore
	dlq         DLQSink
	maxDelivers int
	logger      log.Logger
}

// NewFramework builds the handler runner. Consumer + Receipts are required;
// MaxDelivers <= 0 selects DefaultMaxDelivers; DLQ may be nil (then
// exhausted messages return an error for the caller to route).
func NewFramework(params FrameworkParams) (*Framework, error) {
	params.Consumer = strings.TrimSpace(params.Consumer)

	if err := validate.Struct("consumer", "params", params); err != nil {
		return nil, err
	}

	if params.Receipts == nil {
		return nil, fmt.Errorf("consumer: receipt store is required")
	}

	maxDelivers := params.MaxDelivers
	if maxDelivers <= 0 {
		maxDelivers = DefaultMaxDelivers
	}

	return &Framework{
		consumer:    params.Consumer,
		receipts:    params.Receipts,
		dlq:         params.DLQ,
		maxDelivers: maxDelivers,
		logger:      params.Logger,
	}, nil
}

// Handle processes one message to a read-side effect. Nil return acks
// (including duplicates and DLQ-routed poison); handler errors redeliver
// until MaxDelivers, then route to DLQ. Panics are recovered as errors
// (Nak + backoff per SPEC §9.7), never as acks.
//
// The inbox receipt follows domain-events §5.2: it is released on every
// failure path — including DLQ routing — because no side effect committed.
// A replay after the operator fixes the poison must therefore re-run the
// handler instead of being swallowed as a duplicate.
func (f *Framework) Handle(ctx context.Context, msg appport.Message, handler appport.MessageHandler) (err error) {
	if err := f.validateHandle(msg, handler); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("consumer: handle: %w", err)
	}

	duplicate, err := f.receipts.Claim(ctx, f.consumer, msg.ID)
	if err != nil {
		return err
	}

	if duplicate {
		return nil
	}

	if err := f.invoke(ctx, msg, handler); err != nil {
		var rErr *receiptReleaseError
		if errors.As(err, &rErr) {
			return rErr.err
		}

		return f.release(ctx, msg.ID, err)
	}

	return nil
}

// release rolls the inbox claim back so the redelivery can run the handler
// again. A release failure is joined with the handler error: the message is
// still redelivered, and the operator can see that the receipt may be stuck
// (which would turn the next delivery into a no-op ack).
func (f *Framework) release(ctx context.Context, eventID string, cause error) error {
	if err := f.receipts.Release(ctx, f.consumer, eventID); err != nil {
		return errors.Join(cause, fmt.Errorf("consumer: release receipt: %w", err))
	}

	return cause
}

func (f *Framework) validateHandle(msg appport.Message, handler appport.MessageHandler) error {
	if f == nil || f.receipts == nil {
		return fmt.Errorf("consumer: not initialized")
	}

	if strings.TrimSpace(msg.ID) == "" {
		return fmt.Errorf("consumer: message id is required")
	}

	if handler == nil {
		return fmt.Errorf("consumer: handler is required")
	}

	return nil
}

func (f *Framework) invoke(ctx context.Context, msg appport.Message, handler appport.MessageHandler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicErr := fmt.Errorf("consumer: handler panic: %v", recovered)
			err = f.routeFailure(ctx, msg, panicErr)
		}
	}()

	ctx = withEnvelopeCorrelation(ctx, msg.Payload)

	if err := handler(ctx, msg); err != nil {
		return f.routeFailure(ctx, msg, err)
	}

	return nil
}

// withEnvelopeCorrelation restores trace/request IDs the publisher embedded
// in the envelope body onto the handler context. Non-envelope payloads
// pass through untouched (best-effort: extraction never fails the message).
func withEnvelopeCorrelation(ctx context.Context, payload []byte) context.Context {
	traceID, requestID := envelopeCorrelation(payload)

	fields := make(map[string]string, 2)
	if traceID != "" {
		fields[log.FieldTraceID] = traceID
	}

	if requestID != "" {
		fields[log.FieldRequestID] = requestID
	}

	if len(fields) == 0 {
		return ctx
	}

	return log.WithContext(ctx, fields)
}

// envelopeCorrelation extracts trace/request IDs from an envelope body.
// Malformed or non-envelope payloads yield empty IDs.
func envelopeCorrelation(payload []byte) (traceID, requestID string) {
	if len(payload) == 0 {
		return "", ""
	}

	if raw, err := jsonparser.Get(payload, "trace_id"); err == nil {
		var id string
		if err := jsonparser.Unmarshal(raw, &id); err == nil {
			traceID = id
		}
	}

	if raw, err := jsonparser.Get(payload, "request_id"); err == nil {
		var id string
		if err := jsonparser.Unmarshal(raw, &id); err == nil {
			requestID = id
		}
	}

	return traceID, requestID
}

// routeFailure decides retry versus DLQ. Below MaxDelivers it returns the
// handler error so the broker redelivers. On exhaustion it records the poison
// message to the DLQ and releases the inbox receipt (no effect committed, per
// domain-events §5.2), returning nil to acknowledge the poison; a later DLQ
// replay can therefore re-run the handler. A release failure is joined with
// the handler error and returns to the caller (redeliver, safe direction).
func (f *Framework) routeFailure(ctx context.Context, msg appport.Message, handlerErr error) error {
	if msg.Redelivered+1 < f.maxDelivers {
		return handlerErr
	}

	if f.dlq == nil {
		return handlerErr
	}

	if err := f.dlq.Record(ctx, msg); err != nil {
		return err
	}

	if err := f.receipts.Release(ctx, f.consumer, msg.ID); err != nil {
		return &receiptReleaseError{
			err: errors.Join(handlerErr, fmt.Errorf("consumer: release receipt after dlq: %w", err)),
		}
	}

	return nil
}

// receiptReleaseError marks an error where receipt release was already attempted
// and failed during DLQ routing, preventing Handle from calling f.release again.
type receiptReleaseError struct {
	err error
}

func (e *receiptReleaseError) Error() string {
	return e.err.Error()
}

func (e *receiptReleaseError) Unwrap() error {
	return e.err
}
