package etcd

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ClientParams carries Client dependencies (Parameter Object pattern).
type ClientParams struct {
	Config Config
}

// Client wraps an etcd v3 client with connection lifecycle management.
// It never stores a context; every method takes the caller's context.
type Client struct {
	inner *clientv3.Client
	cfg   Config
}

// NewClient builds a connected-capable etcd client. clientv3 dials lazily,
// so this returns without network I/O; Health verifies reachability.
func NewClient(params ClientParams) (*Client, error) {
	cfg := params.Config.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd: create client: %w", err)
	}

	return &Client{inner: cli, cfg: cfg}, nil
}

// Close releases client connections. Double-close returns nil.
func (c *Client) Close() error {
	if c == nil || c.inner == nil {
		return nil
	}

	if err := c.inner.Close(); err != nil {
		return fmt.Errorf("etcd: close client: %w", err)
	}

	return nil
}

// Health verifies the cluster answers a cheap metadata read within timeout.
func (c *Client) Health(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("etcd: client is not initialized")
	}

	timeout := c.cfg.DialTimeout
	if timeout <= 0 || timeout > 10*time.Second {
		timeout = 10 * time.Second
	}

	call, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if _, err := c.inner.Get(call, "health", clientv3.WithCountOnly(), clientv3.WithLimit(1)); err != nil {
		return fmt.Errorf("etcd: health check: %w", err)
	}

	return nil
}

// Put stores a key unconditionally. Callers needing transactions use Txn.
func (c *Client) Put(ctx context.Context, key, value string) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("etcd: client is not initialized")
	}

	if _, err := c.inner.Put(ctx, key, value); err != nil {
		return fmt.Errorf("etcd: put %s: %w", key, err)
	}

	return nil
}

// Get returns the value for key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if c == nil || c.inner == nil {
		return "", fmt.Errorf("etcd: client is not initialized")
	}

	resp, err := c.inner.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("etcd: get %s: %w", key, err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("etcd: key %s not found", key)
	}

	return string(resp.Kvs[0].Value), nil
}
