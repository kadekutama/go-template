package valkey

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

//go:embed ratelimit.lua
var rateLimitScript string

// ScriptRunner is the Lua-eval seam (DIP): the production *ValkeyClient
// implements it and tests use a canned fake, so Allow decisions are
// unit-testable without Docker.
type ScriptRunner interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
}

// Compile-time seam conformance for the production client.
var _ ScriptRunner = (*ValkeyClient)(nil)

// LimiterParams carries constructor dependencies (Parameter Object pattern).
type LimiterParams struct {
	Client  ScriptRunner
	Logger  log.Logger
	OnAllow func(ctx context.Context, key string)
	OnDeny  func(ctx context.Context, key string)
}

// RateLimiter implements port.RateLimiter over Valkey with one atomic Lua
// script. Denial is a decision, never an error: callers map it to 429.
type RateLimiter struct {
	client  ScriptRunner
	logger  log.Logger
	onAllow func(ctx context.Context, key string)
	onDeny  func(ctx context.Context, key string)
}

// Compile-time port conformance.
var _ appport.RateLimiter = (*RateLimiter)(nil)

// NewRateLimiter builds the limiter; Client must be non-nil.
func NewRateLimiter(params LimiterParams) (*RateLimiter, error) {
	if params.Client == nil {
		return nil, fmt.Errorf("ratelimit: valkey client is required")
	}

	if rateLimitScript == "" {
		return nil, fmt.Errorf("ratelimit: lua script is not embedded")
	}

	return &RateLimiter{
		client:  params.Client,
		logger:  params.Logger,
		onAllow: params.OnAllow,
		onDeny:  params.OnDeny,
	}, nil
}

// Allow checks one key against budget per window.
func (l *RateLimiter) Allow(ctx context.Context, key string, budget int64, window time.Duration) (appport.RateLimitDecision, error) {
	if l == nil || l.client == nil {
		return appport.RateLimitDecision{}, fmt.Errorf("ratelimit: not initialized")
	}

	if key == "" {
		return appport.RateLimitDecision{}, fmt.Errorf("ratelimit: key is required")
	}

	if budget <= 0 {
		return appport.RateLimitDecision{}, fmt.Errorf("ratelimit: budget must be positive")
	}

	if window <= 0 {
		return appport.RateLimitDecision{}, fmt.Errorf("ratelimit: window must be positive")
	}

	if err := ctx.Err(); err != nil {
		return appport.RateLimitDecision{}, fmt.Errorf("ratelimit: allow: %w", err)
	}

	windowMs := window.Milliseconds()
	nowMs := time.Now().UnixMilli()

	raw, err := l.client.Eval(ctx, rateLimitScript, []string{key},
		strconv.FormatInt(budget, 10),
		strconv.FormatInt(windowMs, 10),
		strconv.FormatInt(nowMs, 10),
	)
	if err != nil {
		return appport.RateLimitDecision{}, err
	}

	allowed, remaining, resetMs, err := parseDecision(raw)
	if err != nil {
		return appport.RateLimitDecision{}, err
	}

	decision := appport.RateLimitDecision{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: time.Duration(resetMs) * time.Millisecond,
	}

	if allowed {
		if l.onAllow != nil {
			l.onAllow(ctx, key)
		}
	} else {
		if l.onDeny != nil {
			l.onDeny(ctx, key)
		}
	}

	return decision, nil
}

func parseDecision(raw any) (bool, int64, int64, error) {
	parts, ok := raw.([]any)
	if !ok || len(parts) != 3 {
		return false, 0, 0, fmt.Errorf("ratelimit: unexpected script result %T", raw)
	}

	allowedNum, err := toInt64(parts[0])
	if err != nil {
		return false, 0, 0, fmt.Errorf("ratelimit: allowed: %w", err)
	}

	remaining, err := toInt64(parts[1])
	if err != nil {
		return false, 0, 0, fmt.Errorf("ratelimit: remaining: %w", err)
	}

	resetMs, err := toInt64(parts[2])
	if err != nil {
		return false, 0, 0, fmt.Errorf("ratelimit: reset: %w", err)
	}

	if resetMs < 0 {
		resetMs = 0
	}

	return allowedNum == 1, remaining, resetMs, nil
}

func toInt64(value any) (int64, error) {
	switch num := value.(type) {
	case int64:
		return num, nil
	case int:
		return int64(num), nil
	case string:
		parsed, err := strconv.ParseInt(num, 10, 64)
		if err != nil {
			return 0, err
		}

		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected number %T", value)
	}
}
