package redpanda

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// Broker is the transport seam shared by the relay publisher and the DLQ
// sinks. Production wires a franz-go producer (E14/E17); tests use the
// in-memory fake in the fakes package.
type Broker interface {
	Publish(ctx context.Context, topic, key string, headers map[string]string, payload []byte) error
}

// Default dial and metadata timeouts for the durable client.
const (
	DefaultDialTimeout     = 10 * time.Second
	DefaultMetadataTimeout = 10 * time.Second
)

// RedpandaParams carries client configuration (Parameter Object pattern).
// Seeds are host:port pairs. TLS + SASL/SCRAM are required in prod;
// local/dev may run plaintext against the compose fragment (E17 operates).
type RedpandaParams struct {
	Seeds            []string      `validate:"-"`
	ClientID         string        `validate:"-"`
	UseTLS           bool          `validate:"-"`
	SASLUser         string        `validate:"-"`
	SASLPass         string        `validate:"-"`
	DialTimeout      time.Duration `validate:"omitempty,gt=0"`
	MetadataTimeout  time.Duration `validate:"omitempty,gt=0"`
	MaxBufferedBytes int           `validate:"omitempty,gt=0"`
}

// Client is the thin durable-log client handle. The broker transport is
// injected by the publisher/consumer data plane (E08-T04) so topology tests
// construct without dialing;prod wires franz-go behind the Broker interface.
type Client struct {
	seeds            []string
	clientID         string
	useTLS           bool
	saslUser         string
	saslPass         string
	dialTimeout      time.Duration
	metadataTimeout  time.Duration
	maxBufferedBytes int
}

// NewClient validates endpoints without dialing. At least one non-blank
// seed is required; timeouts fall back to defaults.
func NewClient(params RedpandaParams) (*Client, error) {
	if err := validate.Struct("redpanda", "params", params); err != nil {
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

	dialTimeout := params.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = DefaultDialTimeout
	}

	metadataTimeout := params.MetadataTimeout
	if metadataTimeout <= 0 {
		metadataTimeout = DefaultMetadataTimeout
	}

	return &Client{
		seeds:            seeds,
		clientID:         strings.TrimSpace(params.ClientID),
		useTLS:           params.UseTLS,
		saslUser:         params.SASLUser,
		saslPass:         params.SASLPass,
		dialTimeout:      dialTimeout,
		metadataTimeout:  metadataTimeout,
		maxBufferedBytes: params.MaxBufferedBytes,
	}, nil
}

// Seeds reports the configured bootstrap seeds.
func (c *Client) Seeds() []string {
	if c == nil {
		return nil
	}

	out := make([]string, len(c.seeds))
	copy(out, c.seeds)

	return out
}

// UsesTLS reports whether transport encryption is enabled.
func (c *Client) UsesTLS() bool {
	return c != nil && c.useTLS
}

// HasSASL reports whether SASL/SCRAM credentials are configured.
func (c *Client) HasSASL() bool {
	return c != nil && strings.TrimSpace(c.saslUser) != ""
}
