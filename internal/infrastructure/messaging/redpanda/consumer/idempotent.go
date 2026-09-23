package consumer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// DefaultReceiptTTL bounds Valkey inbox hints (durable inbox lives in
// PostgreSQL; the hint only suppresses hot redelivery).
const DefaultReceiptTTL = 7 * 24 * time.Hour

// ReceiptKV is the narrow Valkey seam for inbox hints (DIP): the production
// *valkey.ValkeyClient implements it and tests use an in-memory fake.
type ReceiptKV interface {
	SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
	Delete(ctx context.Context, key string) error
}

// Compile-time seam conformance for the production client.
var _ ReceiptKV = (*valkey.ValkeyClient)(nil)

// ValkeyReceiptParams carries constructor dependencies (Parameter Object pattern).
type ValkeyReceiptParams struct {
	Client ReceiptKV     `validate:"-"`
	TTL    time.Duration `validate:"omitempty,gt=0"`
}

// ValkeyReceiptStore dedupes across instances via SET NX with TTL.
// Store errors return err (redeliver, safe direction); expiries re-allow
// delivery, so the durable postgres inbox remains the long-term boundary.
type ValkeyReceiptStore struct {
	client ReceiptKV
	ttl    time.Duration
}

// NewValkeyReceiptStore builds the hint store; Client is required.
func NewValkeyReceiptStore(params ValkeyReceiptParams) (*ValkeyReceiptStore, error) {
	if err := validate.Struct("consumer", "receipt params", params); err != nil {
		return nil, err
	}

	if params.Client == nil {
		return nil, fmt.Errorf("consumer: valkey client is required")
	}

	ttl := params.TTL
	if ttl <= 0 {
		ttl = DefaultReceiptTTL
	}

	return &ValkeyReceiptStore{client: params.Client, ttl: ttl}, nil
}

// Claim sets inbox:{consumer}:{eventID} NX; losers are duplicates.
func (s *ValkeyReceiptStore) Claim(ctx context.Context, consumer, eventID string) (bool, error) {
	if s == nil || s.client == nil {
		return false, fmt.Errorf("consumer: not initialized")
	}

	if strings.TrimSpace(consumer) == "" || strings.TrimSpace(eventID) == "" {
		return false, fmt.Errorf("consumer: consumer and event id are required")
	}

	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("consumer: claim: %w", err)
	}

	won, err := s.client.SetNX(ctx, inboxKey(consumer, eventID), []byte("1"), s.ttl)
	if err != nil {
		return false, err
	}

	return !won, nil
}

// Release deletes the inbox hint so the redelivery re-runs the handler.
// Missing keys succeed (Valkey DEL is idempotent).
func (s *ValkeyReceiptStore) Release(ctx context.Context, consumer, eventID string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("consumer: not initialized")
	}

	if strings.TrimSpace(consumer) == "" || strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("consumer: consumer and event id are required")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("consumer: release: %w", err)
	}

	return s.client.Delete(ctx, inboxKey(consumer, eventID))
}

func inboxKey(consumer, eventID string) string {
	return "inbox:" + strings.TrimSpace(consumer) + ":" + strings.TrimSpace(eventID)
}
