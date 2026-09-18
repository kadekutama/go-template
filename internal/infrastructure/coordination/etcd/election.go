package etcd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.etcd.io/etcd/client/v3/concurrency"
)

// LeaderElector campaigns for single-writer leadership over one key prefix
// using etcd session leases. Consumers depend on this interface, never on
// the concrete adapter.
type LeaderElector interface {
	// Campaign blocks until this candidate holds prefix leadership or ctx
	// ends. One Election holds at most one campaign at a time; call Resign
	// before campaigning again. Call Resign when the protected work ends.
	Campaign(ctx context.Context, candidateID string) error
	// Resign voluntarily releases leadership and revokes the lease so the
	// next waiter acquires it immediately.
	Resign(ctx context.Context) error
	// Leader returns the current holder recorded for the prefix.
	Leader(ctx context.Context) (string, error)
	// Prefix exposes the election key prefix (operator introspection).
	Prefix() string
}

// ElectionParams carries Election dependencies (Parameter Object pattern).
type ElectionParams struct {
	Client     *Client
	KeyPrefix  string
	TTLSeconds int
}

// election is the etcd-backed LeaderElector implementation. At most one
// holder wins per prefix; lease expiry or Resign hands leadership to the
// next waiter without manual intervention.
type election struct {
	client *Client
	prefix string
	ttl    int

	mu          sync.Mutex
	campaigning bool
	session     *concurrency.Session
	election    *concurrency.Election
}

// NewElection builds a LeaderElector. Campaign creates the lease lazily so
// construction performs no network I/O.
func NewElection(params ElectionParams) (LeaderElector, error) {
	if params.Client == nil || params.Client.inner == nil {
		return nil, fmt.Errorf("etcd: election requires an initialized client")
	}

	prefix := strings.TrimSpace(params.KeyPrefix)
	if prefix == "" {
		prefix = DefaultLeaderKeyPrefix
	}

	ttl := params.TTLSeconds
	if ttl <= 0 {
		ttl = DefaultElectionTTLSeconds
	}

	return &election{client: params.Client, prefix: prefix, ttl: ttl}, nil
}

// Campaign blocks until this candidate holds prefix leadership or ctx ends.
// Call Resign when the protected work completes.
func (e *election) Campaign(ctx context.Context, candidateID string) error {
	if e == nil || e.client == nil || e.client.inner == nil {
		return fmt.Errorf("etcd: election is not initialized")
	}

	trimmedCandidate := strings.TrimSpace(candidateID)
	if trimmedCandidate == "" {
		return fmt.Errorf("etcd: candidate id must not be blank")
	}

	e.mu.Lock()
	if e.campaigning {
		e.mu.Unlock()
		return fmt.Errorf("etcd: already campaigning for %s; resign before campaigning again", e.prefix)
	}
	e.campaigning = true
	e.mu.Unlock()

	session, err := concurrency.NewSession(e.client.inner, concurrency.WithTTL(e.ttl))
	if err != nil {
		e.mu.Lock()
		e.campaigning = false
		e.mu.Unlock()
		return fmt.Errorf("etcd: create election session: %w", err)
	}

	poll := concurrency.NewElection(session, e.prefix)

	if err := poll.Campaign(ctx, trimmedCandidate); err != nil {
		_ = session.Close()
		e.mu.Lock()
		e.campaigning = false
		e.mu.Unlock()
		return fmt.Errorf("etcd: campaign %s: %w", trimmedCandidate, err)
	}

	e.mu.Lock()
	e.session = session
	e.election = poll
	e.mu.Unlock()

	return nil
}

// Resign voluntarily releases leadership and revokes the lease so the next
// waiter acquires it immediately.
func (e *election) Resign(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("etcd: election is not initialized")
	}

	e.mu.Lock()
	poll := e.election
	session := e.session
	e.election = nil
	e.session = nil
	e.campaigning = false
	e.mu.Unlock()

	if poll == nil || session == nil {
		return fmt.Errorf("etcd: no active leadership to resign")
	}

	if err := poll.Resign(ctx); err != nil {
		_ = session.Close()
		return fmt.Errorf("etcd: resign: %w", err)
	}

	if err := session.Close(); err != nil {
		return fmt.Errorf("etcd: close election session: %w", err)
	}

	return nil
}

// Leader returns the current holder recorded for the prefix.
func (e *election) Leader(ctx context.Context) (string, error) {
	if e == nil || e.client == nil || e.client.inner == nil {
		return "", fmt.Errorf("etcd: election is not initialized")
	}

	session, err := concurrency.NewSession(e.client.inner, concurrency.WithTTL(e.ttl))
	if err != nil {
		return "", fmt.Errorf("etcd: create leader session: %w", err)
	}
	defer func() { _ = session.Close() }()

	poll := concurrency.NewElection(session, e.prefix)

	resp, err := poll.Leader(ctx)
	if err != nil {
		return "", fmt.Errorf("etcd: read leader: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("etcd: no leader elected for %s", e.prefix)
	}

	return string(resp.Kvs[0].Value), nil
}

// Prefix exposes the election key prefix (test and operator introspection).
func (e *election) Prefix() string {
	if e == nil {
		return ""
	}

	return e.prefix
}
