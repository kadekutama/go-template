package port

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// NotifyChannel names one delivery channel. Channel handling (templates,
// retries, provider failover) lives with the adapter; callers only choose.
type NotifyChannel string

// Supported notification channels.
const (
	NotifyChannelEmail   NotifyChannel = "email"
	NotifyChannelSMS     NotifyChannel = "sms"
	NotifyChannelWebhook NotifyChannel = "webhook"
)

// Notification is one human- or system-facing message. Bodies are
// fully-rendered strings: template selection happens before this boundary.
// Bodies MUST NOT carry secrets or unrestricted PII.
type Notification struct {
	TenantID valueobject.TenantID
	Channel  NotifyChannel
	To       string
	Subject  string
	Body     string
}

// Notifier is the human-notification boundary (SMTP/maildev in E10).
// Delivery is at-least-once with adapter-side dedupe on the caller-supplied
// idempotency key; money movement never waits on notification success.
type Notifier interface {
	// Send delivers one notification. Idempotent per key.
	Send(ctx context.Context, notification Notification, idempotencyKey string) error
}
