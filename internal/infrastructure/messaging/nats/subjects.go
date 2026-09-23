// Package nats is the NATS Core v2.14.6 real-time edge (E08-T03/T04):
// subject builders and the edge publisher feeding active WebSockets. NATS
// carries ephemeral delta notifications only (<5µs routing); the durable log
// lives in Redpanda. Subjects reuse the E05-T03 ledger.{tenant}.{event} shape.
package nats

import (
	"fmt"
	"strings"

	domainservice "github.com/kadekutama/go-template/internal/domain/service"
)

// BalanceChangedEvent is the versioned edge event for balance deltas.
const BalanceChangedEvent = "account.balance.changed.v1"

// EventSubject builds ledger.{tenant}.{eventType} via the isolation
// contract so cache keys, RLS scope, and edge routing agree.
func EventSubject(tenant, eventType string) (string, error) {
	subject, err := domainservice.BuildSubject(tenant, eventType)
	if err != nil {
		return "", err
	}

	return subject, nil
}

// ParseEventSubject inverts EventSubject.
func ParseEventSubject(subject string) (tenant, eventType string, err error) {
	return domainservice.ParseSubject(subject)
}

// BalanceSubject builds the edge subject for balance deltas.
func BalanceSubject(tenant string) (string, error) {
	return EventSubject(tenant, BalanceChangedEvent)
}

// QueueGroupFor returns the queue-group name for a consumer group so NATS
// queue subscribers drain with at-least-once semantics.
func QueueGroupFor(group string) (string, error) {
	trimmed := strings.TrimSpace(group)
	if trimmed == "" {
		return "", fmt.Errorf("nats: consumer group is required")
	}

	return "q-" + trimmed, nil
}
