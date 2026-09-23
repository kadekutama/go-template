package webhook

import (
	"context"
	"fmt"
	"time"
)

// RetryableError asks the caller (E14 worker) to re-queue one endpoint
// delivery after NextDelay. It is a decision carrier, not a failure: the
// dispatcher stays alive and other endpoints are unaffected. Cause carries
// the send failure for observability; use errors.As to inspect.
type RetryableError struct {
	EndpointID string
	Attempt    int
	NextDelay  time.Duration
	Cause      error
}

// Error implements the error interface.
func (e *RetryableError) Error() string {
	if e == nil {
		return "webhook: retryable delivery"
	}

	return fmt.Sprintf("webhook: retry endpoint %s attempt %d after %s",
		e.EndpointID, e.Attempt, e.NextDelay)
}

// Unwrap returns the send failure.
func (e *RetryableError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Cause
}

// DLQMessage is one exhausted webhook delivery (attempts reached the policy bound).
type DLQMessage struct {
	EndpointID string
	Tenant     string
	Event      string
	Payload    []byte
	Attempts   int
}

// DLQSink records exhausted deliveries for replay operations. Production
// uses the durable Redpanda webhook DLQ sink (E08-T05 consumer adapter);
// tests use the fakes package double.
type DLQSink interface {
	Record(ctx context.Context, msg DLQMessage) error
}
