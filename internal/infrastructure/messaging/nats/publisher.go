package nats

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// Default timeouts for the edge publisher.
const (
	DefaultConnectTimeout = 5 * time.Second
	DefaultRequestTimeout = 3 * time.Second
)

// PublisherParams carries publisher configuration (Parameter Object pattern). NKeySeed
// (nkeys) and UseTLS are validated at construction and applied on dial;
// production requires both, local/dev may run plaintext.
type PublisherParams struct {
	URL            string        `validate:"required"`
	NKeySeed       string        `validate:"-"`
	UseTLS         bool          `validate:"-"`
	ConnectTimeout time.Duration `validate:"omitempty,gt=0"`
	Logger         log.Logger    `validate:"-"`
}

// Publisher is the NATS Core edge handle. NewPublisher validates AND dials:
// NATS is a critical dependency, so a process that needs it must not start
// without it (fail fast; the orchestrator restarts until the broker is
// reachable). There is deliberately no lazy Connect and no connection mutex:
// the *nats.Conn is assigned once in the constructor and never mutated
// afterward, so concurrent Publish calls need no guard of their own —
// reconnection itself stays library-managed (infinite backoff retries below).
// It is safe for concurrent use.
type Publisher struct {
	url            string
	useTLS         bool
	connectTimeout time.Duration
	logger         log.Logger
	conn           *natsgo.Conn
}

// NewPublisher validates the URL, TLS flag, and nkey seed, then dials within
// the connect timeout. Misconfiguration and unreachable brokers both fail
// here, before the process serves anything.
func NewPublisher(params PublisherParams) (*Publisher, error) {
	if err := validate.Struct("nats", "params", params); err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(params.URL)
	if trimmed == "" {
		return nil, fmt.Errorf("nats: url is required")
	}

	connectTimeout := params.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = DefaultConnectTimeout
	}

	options := []natsgo.Option{
		natsgo.Timeout(connectTimeout),
		natsgo.MaxReconnects(-1),
	}

	if params.UseTLS {
		options = append(options, natsgo.Secure(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	if seed := strings.TrimSpace(params.NKeySeed); seed != "" {
		auth, err := nkeyAuth(seed)
		if err != nil {
			return nil, err
		}

		options = append(options, auth)
	}

	// natsgo.Timeout above already bounds the dial; no extra context plumbing.
	conn, err := natsgo.Connect(trimmed, options...)
	if err != nil {
		return nil, fmt.Errorf("nats: connect: %w", err)
	}

	return &Publisher{
		url:            trimmed,
		useTLS:         params.UseTLS,
		connectTimeout: connectTimeout,
		logger:         params.Logger,
		conn:           conn,
	}, nil
}

// nkeyAuth builds the user-nkey authenticator for an inline seed. The seed is
// supplied by the secrets store (E09) and is never logged. The keypair stays
// in memory because NATS re-signs the server nonce on every (re)connect.
func nkeyAuth(seed string) (natsgo.Option, error) {
	keyPair, err := nkeys.ParseDecoratedNKey([]byte(seed))
	if err != nil {
		return nil, fmt.Errorf("nats: invalid nkey seed: %w", err)
	}

	public, err := keyPair.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("nats: nkey public key: %w", err)
	}

	if !nkeys.IsValidPublicUserKey(public) {
		return nil, fmt.Errorf("nats: seed is not a user nkey")
	}

	sign := func(nonce []byte) ([]byte, error) {
		signed, err := keyPair.Sign(nonce)
		if err != nil {
			return nil, fmt.Errorf("nats: sign nonce: %w", err)
		}

		return signed, nil
	}

	return natsgo.Nkey(public, sign), nil
}

// PublishEvent is the contract-level real-time entry point: it derives the
// versioned ledger.{tenant}.{eventType} subject through the E05-T03
// isolation builders and publishes the payload. It is used by the outbox
// relay / live-update bridge to push state-change deltas (for example
// account.balance.changed.v1) to active WebSocket subscribers; it is not
// email/SMS notification (that port is Notifier, E10).
func (m *Publisher) PublishEvent(ctx context.Context, tenant, eventType string, payload []byte) error {
	if m == nil {
		return fmt.Errorf("nats: publisher is not initialized")
	}

	if payload == nil {
		return fmt.Errorf("nats: payload is required")
	}

	subject, err := EventSubject(tenant, eventType)
	if err != nil {
		return err
	}

	return m.Publish(ctx, subject, payload)
}

// Publish emits one ephemeral delta.
func (m *Publisher) Publish(ctx context.Context, subject string, payload []byte) error {
	if m == nil || m.conn == nil {
		return fmt.Errorf("nats: publisher is not initialized")
	}

	if strings.TrimSpace(subject) == "" {
		return fmt.Errorf("nats: subject is required")
	}

	if payload == nil {
		return fmt.Errorf("nats: payload is required")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("nats: publish: %w", err)
	}

	if err := m.conn.Publish(subject, payload); err != nil {
		return fmt.Errorf("nats: publish: %w", err)
	}

	return nil
}

// URL reports the configured server URL.
func (m *Publisher) URL() string {
	if m == nil {
		return ""
	}

	return m.url
}

// UsesTLS reports whether transport encryption is enabled.
func (m *Publisher) UsesTLS() bool {
	return m != nil && m.useTLS
}

// ConnectTimeout reports the effective dial timeout.
func (m *Publisher) ConnectTimeout() time.Duration {
	if m == nil {
		return DefaultConnectTimeout
	}

	return m.connectTimeout
}

// Close flushes buffered publishes (bounded by the request timeout) and then
// closes the connection. The underlying client call is idempotent, so Close
// is safe to call twice (notably from fx OnStop after a partial shutdown).
// Wire it to the fx OnStop hook so shutdown never drops a delta that was
// already accepted.
func (m *Publisher) Close() error {
	if m == nil || m.conn == nil {
		return nil
	}

	if err := m.conn.FlushTimeout(DefaultRequestTimeout); err != nil && m.logger != nil {
		m.logger.Warn(context.Background(), "nats.flush.failed", "cause", err)
	}

	m.conn.Close()

	return nil
}
