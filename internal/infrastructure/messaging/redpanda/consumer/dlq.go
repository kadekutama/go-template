package consumer

import (
	"context"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// DLQSink records poison messages after max delivers. Production uses the
// durable Redpanda sink (redpanda_dlq.go); tests use the fakes package.
type DLQSink interface {
	Record(ctx context.Context, msg appport.Message) error
}
