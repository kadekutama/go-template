// Package valkey is the Valkey L2 client adapter (E08-T01):
// go-redis v9.22.0 against Valkey 9.1.2 (Sentinel and Cluster compatible).
// It backs the hybrid port.Cache L2, the Redlock coordinator (E08-T02),
// and the token-bucket limiter (E08-T07). Cached values are hints only.
package valkey

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/store"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// ErrMiss aliases the shared seam miss sentinel so direct consumers (locks,
// receipts) keep matching one error while the cache path uses store.ErrMiss.
var ErrMiss = store.ErrMiss

// Default timeouts and pool sizing for the L2 client.
const (
	DefaultPoolSize    = 10
	DefaultDialTimeout = 5 * time.Second
	DefaultIOTimeout   = 3 * time.Second
)

// ValkeyParams carries constructor dependencies (Parameter Object pattern).
// Field rules are declared with validator tags and enforced by
// validateParams; nil-dependency rules stay explicit at the call site.
type ValkeyParams struct {
	Addr         string        `validate:"required"`
	Password     string        `validate:"-"`
	DB           int           `validate:"gte=0,lte=15"`
	PoolSize     int           `validate:"omitempty,gt=0"`
	DialTimeout  time.Duration `validate:"omitempty,gt=0"`
	ReadTimeout  time.Duration `validate:"omitempty,gt=0"`
	WriteTimeout time.Duration `validate:"omitempty,gt=0"`
	UseTLS       bool          `validate:"-"`
}

// ValkeyClient is the L2 handle. All methods are safe for concurrent use.
type ValkeyClient struct {
	client *redis.Client
}

// Compile-time seam conformance: the client plugs into the shared engine
// beside the Otter L1 with identical Get/Set/Delete shapes.
var (
	_ store.Store         = (*ValkeyClient)(nil)
	_ store.ExpiringStore = (*ValkeyClient)(nil)
	_ ScriptRunner        = (*ValkeyClient)(nil)
)

// NewValkeyClient builds the client. Addr is required (blank or whitespace
// only is rejected); dial is lazy so topology tests construct without I/O.
// Call Ping to verify connectivity.
func NewValkeyClient(params ValkeyParams) (*ValkeyClient, error) {
	if err := validate.Struct("valkey", "params", params); err != nil {
		return nil, err
	}

	addr := strings.TrimSpace(params.Addr)
	if addr == "" {
		return nil, fmt.Errorf("valkey: addr is required")
	}

	poolSize := params.PoolSize
	if poolSize <= 0 {
		poolSize = DefaultPoolSize
	}

	dialTimeout := params.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = DefaultDialTimeout
	}

	readTimeout := params.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = DefaultIOTimeout
	}

	writeTimeout := params.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = DefaultIOTimeout
	}

	opts := &redis.Options{
		Addr:         addr,
		Password:     params.Password,
		DB:           params.DB,
		PoolSize:     poolSize,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	if params.UseTLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return &ValkeyClient{client: redis.NewClient(opts)}, nil
}

// Ping verifies connectivity within ctx.
func (c *ValkeyClient) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("valkey: client is not initialized")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("valkey: ping: %w", err)
	}

	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("valkey: ping: %w", err)
	}

	return nil
}

// Get returns the value or store.ErrMiss. Context cancellation aborts the
// read. It implements the shared store.Store contract with the same shape
// as the Otter L1.
func (c *ValkeyClient) Get(ctx context.Context, key string) ([]byte, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("valkey: client is not initialized")
	}

	if key == "" {
		return nil, fmt.Errorf("valkey: key is required")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("valkey: get: %w", err)
	}

	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, store.ErrMiss
		}

		return nil, fmt.Errorf("valkey: get: %w", err)
	}

	return raw, nil
}

// TTL reports a record's remaining TTL for bounding L1 repopulation: a
// copied entry must never outlive its source. It returns store.NoExpiry for
// a persisted key and store.ErrMiss for an absent one.
func (c *ValkeyClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	if c == nil || c.client == nil {
		return 0, fmt.Errorf("valkey: client is not initialized")
	}

	if key == "" {
		return 0, fmt.Errorf("valkey: key is required")
	}

	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("valkey: ttl: %w", err)
	}

	remaining, err := c.client.PTTL(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, store.ErrMiss
		}

		return 0, fmt.Errorf("valkey: ttl: %w", err)
	}

	switch remaining {
	case -1:
		return store.NoExpiry, nil
	case -2:
		// The key vanished between calls; treat as expiring now.
		return 0, nil
	default:
		return remaining, nil
	}
}

// Set stores value with mandatory positive TTL. A nil payload is legal and
// stored as an empty value: callers use it as a negative-cache marker, and
// Get cannot distinguish nil from empty on the wire.
func (c *ValkeyClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("valkey: client is not initialized")
	}

	if key == "" {
		return fmt.Errorf("valkey: key is required")
	}

	if ttl <= 0 {
		return fmt.Errorf("valkey: ttl must be positive")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("valkey: set: %w", err)
	}

	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("valkey: set: %w", err)
	}

	return nil
}

// Delete invalidates one key. Missing keys succeed.
func (c *ValkeyClient) Delete(ctx context.Context, key string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("valkey: client is not initialized")
	}

	if key == "" {
		return fmt.Errorf("valkey: key is required")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("valkey: delete: %w", err)
	}

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("valkey: delete: %w", err)
	}

	return nil
}

// Eval executes a Lua script on the server with the given keys and arguments.
func (c *ValkeyClient) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("valkey: client is not initialized")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("valkey: eval: %w", err)
	}

	res, err := c.client.Eval(ctx, script, keys, args...).Result()
	if err != nil {
		return nil, fmt.Errorf("valkey: eval: %w", err)
	}

	return res, nil
}

// SetNX sets key only when absent with TTL; it reports whether the set won.
func (c *ValkeyClient) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if c == nil || c.client == nil {
		return false, fmt.Errorf("valkey: client is not initialized")
	}

	if key == "" {
		return false, fmt.Errorf("valkey: key is required")
	}

	if ttl <= 0 {
		return false, fmt.Errorf("valkey: ttl must be positive")
	}

	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("valkey: setnx: %w", err)
	}

	won, err := c.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("valkey: setnx: %w", err)
	}

	return won, nil
}

// Close drains the pool.
func (c *ValkeyClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}

	if err := c.client.Close(); err != nil {
		return fmt.Errorf("valkey: close: %w", err)
	}

	return nil
}
