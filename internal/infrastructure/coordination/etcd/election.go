package etcd

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

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

const (
	electionStateIdle        uint32 = 0
	electionStateCampaigning uint32 = 1
	electionStateLeader      uint32 = 2
)

// election is the etcd-backed LeaderElector implementation. At most one
// holder wins per prefix; lease expiry or Resign hands leadership to the
// next waiter without manual intervention.
type election struct {
	client *Client
	prefix string
	ttl    int

	state    atomic.Uint32
	session  atomic.Pointer[concurrency.Session]
	election atomic.Pointer[concurrency.Election]
}

// NewElection builds a LeaderElector. Client, KeyPrefix, and positive TTLSeconds are required.
// Campaign creates the lease lazily so construction performs no network I/O.
func NewElection(params ElectionParams) (LeaderElector, error) {
	if params.Client == nil || params.Client.inner == nil {
		return nil, fmt.Errorf("etcd: election requires an initialized client")
	}

	prefix := strings.TrimSpace(params.KeyPrefix)
	if prefix == "" {
		return nil, fmt.Errorf("etcd: election requires a key prefix")
	}

	if params.TTLSeconds <= 0 {
		return nil, fmt.Errorf("etcd: election requires a positive TTL")
	}

	return &election{client: params.Client, prefix: prefix, ttl: params.TTLSeconds}, nil
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

	if !e.state.CompareAndSwap(electionStateIdle, electionStateCampaigning) {
		return fmt.Errorf("etcd: already campaigning for %s; resign before campaigning again", e.prefix)
	}

	session, err := concurrency.NewSession(e.client.inner, concurrency.WithTTL(e.ttl))
	if err != nil {
		e.state.Store(electionStateIdle)
		return fmt.Errorf("etcd: create election session: %w", err)
	}

	poll := concurrency.NewElection(session, e.prefix)

	if err := poll.Campaign(ctx, trimmedCandidate); err != nil {
		_ = session.Close()
		e.state.Store(electionStateIdle)
		return fmt.Errorf("etcd: campaign %s: %w", trimmedCandidate, err)
	}

	e.session.Store(session)
	e.election.Store(poll)
	e.state.Store(electionStateLeader)

	return nil
}

// Resign voluntarily releases leadership and revokes the lease so the next
// waiter acquires it immediately.
func (e *election) Resign(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("etcd: election is not initialized")
	}

	if !e.state.CompareAndSwap(electionStateLeader, electionStateIdle) {
		return fmt.Errorf("etcd: no active leadership to resign")
	}

	poll := e.election.Swap(nil)
	session := e.session.Swap(nil)

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
