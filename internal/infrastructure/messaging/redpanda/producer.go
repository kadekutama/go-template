package redpanda

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// SASL mechanisms accepted for broker authentication.
const (
	SASLMechanismPlain       = "plain"
	SASLMechanismScramSHA256 = "scram-sha-256"
	SASLMechanismScramSHA512 = "scram-sha-512"

	defaultSASLMechanism = SASLMechanismScramSHA512
)

// ProducerParams carries producer configuration (Parameter Object pattern).
// Seeds are host:port pairs; UseTLS enables TLS 1.2+; SASL credentials are
// optional (plaintext dev clusters). Topic names come from the configured
// TopicRegistry at the call site, never from this package.
type ProducerParams struct {
	Seeds            []string      `validate:"-"`
	ClientID         string        `validate:"-"`
	UseTLS           bool          `validate:"-"`
	SASLUser         string        `validate:"-"`
	SASLPass         string        `validate:"-"`
	SASLMechanism    string        `validate:"-"`
	DialTimeout      time.Duration `validate:"omitempty,gt=0"`
	MaxBufferedBytes int           `validate:"omitempty,gt=0"`
	Logger           log.Logger    `validate:"-"`
}

// Producer is the franz-go client implementing the Broker seam: the single
// production transport shared by the relay publisher and both durable DLQ
// sinks. franz-go was chosen for protocol completeness (KIP-848-ready
// consumer path for E14, native contexts, bounded buffers, rich admin),
// with the Broker seam keeping any future client swap to one file.
//
// The underlying client is initialized directly in NewProducer (matching
// nats.Publisher) and is safe for concurrent use without locking;
// Publish is synchronous (ProduceSync) so an unknown outcome stays an error
// for the outbox poller; Close flushes in-flight records before shutting
// down when SIGTERM is received.
type Producer struct {
	client *kgo.Client
	logger log.Logger
}

// Compile-time seam conformance.
var _ Broker = (*Producer)(nil)

// NewProducer validates configuration and initializes the shared producer. At
// least one non-blank seed is required. SASL credentials without a mechanism
// select SCRAM-SHA-512 (the Redpanda default); an unknown mechanism fails.
func NewProducer(params ProducerParams) (*Producer, error) {
	if err := validate.Struct("redpanda", "producer params", params); err != nil {
		return nil, err
	}

	seeds := make([]string, 0, len(params.Seeds))
	for _, seed := range params.Seeds {
		if trimmed := strings.TrimSpace(seed); trimmed != "" {
			seeds = append(seeds, trimmed)
		}
	}

	if len(seeds) == 0 {
		return nil, fmt.Errorf("redpanda: at least one seed is required")
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(seeds...),
		kgo.ClientID(strings.TrimSpace(params.ClientID)),
		kgo.DialTimeout(defaultOr(params.DialTimeout, DefaultDialTimeout)),
		kgo.ProduceRequestTimeout(30 * time.Second),
	}

	if maxBytes := params.MaxBufferedBytes; maxBytes > 0 {
		opts = append(opts, kgo.MaxBufferedBytes(maxBytes))
	}

	if params.UseTLS {
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	if user := strings.TrimSpace(params.SASLUser); user != "" {
		mechanism, err := saslMechanism(strings.TrimSpace(params.SASLMechanism), user, params.SASLPass)
		if err != nil {
			return nil, err
		}

		opts = append(opts, kgo.SASL(mechanism))
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("redpanda: dial: %w", err)
	}

	return &Producer{client: client, logger: params.Logger}, nil
}

// saslMechanism selects the authenticator: an explicit mechanism wins, blank
// defaults to SCRAM-SHA-512 (the Redpanda default) when credentials are
// present. Unknown mechanisms fail at construction, never at publish time.
func saslMechanism(mechanism, user, pass string) (sasl.Mechanism, error) {
	if mechanism == "" {
		mechanism = defaultSASLMechanism
	}

	switch mechanism {
	case SASLMechanismScramSHA512:
		return scram.Auth{User: user, Pass: pass}.AsSha512Mechanism(), nil
	case SASLMechanismScramSHA256:
		return scram.Auth{User: user, Pass: pass}.AsSha256Mechanism(), nil
	case SASLMechanismPlain:
		return plain.Auth{User: user, Pass: pass}.AsMechanism(), nil
	default:
		return nil, fmt.Errorf("redpanda: unknown sasl mechanism %q", mechanism)
	}
}

// Publish writes one record synchronously: success means the broker
// acknowledged it, anything else stays an error (at-least-once relay).
func (p *Producer) Publish(ctx context.Context, topic, key string, headers map[string]string, payload []byte) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("redpanda: producer is not initialized")
	}

	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("redpanda: topic is required")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("redpanda: publish: %w", err)
	}

	record := &kgo.Record{
		Topic: strings.TrimSpace(topic),
		Key:   []byte(key),
		Value: payload,
	}

	for name, value := range headers {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: name, Value: []byte(value)})
	}

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("redpanda: publish %s: %w", topic, err)
	}

	return nil
}

// Close flushes in-flight records and shuts the client down; double-close
// succeeds. Wire it to the fx OnStop hook so shutdown never drops buffered
// publishes.
func (p *Producer) Close() error {
	if p == nil || p.client == nil {
		return nil
	}

	p.client.Close()

	return nil
}

func defaultOr(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}

	return value
}
