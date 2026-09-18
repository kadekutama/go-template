package etcd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/internal/shared/kernel/safe"
)

// WatchHandler receives one key mutation. It must be fast and non-blocking;
// slow consumers copy the payload and finish work elsewhere.
type WatchHandler func(key, value []byte)

// Watcher streams configuration mutations for key prefixes into an
// in-memory cache without polling. Implementations are safe for concurrent
// use. Consumers depend on this interface, never on the concrete adapter.
type Watcher interface {
	// WatchPrefix loads current values under prefix, anchors the stream at
	// the load revision (no gap loss), then streams mutations to onChange
	// until ctx is canceled.
	WatchPrefix(ctx context.Context, prefix string, onChange WatchHandler) error
	// Cached returns the last-seen value for key.
	Cached(key string) ([]byte, bool)
	// Err reports asynchronous stream failures. A received error means the
	// affected stream has stopped; establish a new WatchPrefix to resume.
	// Clean context cancellation reports nothing.
	Err() <-chan error
}

// WatcherParams carries Watcher dependencies (Parameter Object pattern).
// Logger is nil-tolerant: panics are still recovered and reported on Err,
// only the log line is skipped when no logger is set.
type WatcherParams struct {
	Client *Client
	Logger log.Logger
}

// watcher is the etcd-backed Watcher implementation.
type watcher struct {
	client *Client
	logger log.Logger

	mu    sync.RWMutex
	cache map[string][]byte

	errCh chan error
}

// NewWatcher builds a Watcher over an initialized client.
func NewWatcher(params WatcherParams) (Watcher, error) {
	if params.Client == nil || params.Client.inner == nil {
		return nil, fmt.Errorf("etcd: watcher requires an initialized client")
	}

	return &watcher{
		client: params.Client,
		logger: params.Logger,
		cache:  make(map[string][]byte),
		errCh:  make(chan error, 4),
	}, nil
}

// WatchPrefix loads current values under prefix, then streams mutations to
// onChange until ctx is canceled. The stream resumes at the load revision so
// mutations landing between the load and the watch are not lost. It returns
// nil once the stream is established; stream errors surface on Err.
func (w *watcher) WatchPrefix(ctx context.Context, prefix string, onChange WatchHandler) error {
	if w == nil || w.client == nil || w.client.inner == nil {
		return fmt.Errorf("etcd: watcher is not initialized")
	}

	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("etcd: watch prefix must not be blank")
	}

	if onChange == nil {
		return fmt.Errorf("etcd: watch handler is required")
	}

	resp, err := w.client.inner.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("etcd: watch prefix load %s: %w", prefix, err)
	}

	w.mu.Lock()

	for _, kv := range resp.Kvs {
		cached := make([]byte, len(kv.Value))
		copy(cached, kv.Value)
		w.cache[string(kv.Key)] = cached
	}

	w.mu.Unlock()

	stream := w.client.inner.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(resp.Header.Revision))

	w.spawn(ctx, prefix, stream, onChange)

	return nil
}

// spawn serves the stream under safe.Go: a handler panic is recovered,
// logged with stack via the watcher's logger, and reported on Err. Recovery
// is containment, never completion: owners must re-establish the watch.
func (w *watcher) spawn(ctx context.Context, prefix string, stream clientv3.WatchChan, onChange WatchHandler) {
	safe.Go(ctx, w.logger, func(p safe.Panic) {
		w.report(fmt.Errorf("etcd: watcher panic for %s: %w", prefix, p))
	}, func(ctx context.Context) {
		w.serve(ctx, prefix, stream, onChange)
	})
}

// Cached returns the last-seen value for key.
func (w *watcher) Cached(key string) ([]byte, bool) {
	if w == nil {
		return nil, false
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	value, ok := w.cache[key]
	if !ok {
		return nil, false
	}

	copied := make([]byte, len(value))
	copy(copied, value)

	return copied, true
}

// Err reports asynchronous stream failures.
func (w *watcher) Err() <-chan error {
	if w == nil {
		return nil
	}

	return w.errCh
}

// report records a stream failure without blocking the serving goroutine.
func (w *watcher) report(err error) {
	select {
	case w.errCh <- err:
	default:
	}
}

// serve relays one watch channel into the cache and handler until ctx ends
// or the stream fails. Clean cancellation is silent; failures go to Err.
func (w *watcher) serve(ctx context.Context, prefix string, stream clientv3.WatchChan, onChange WatchHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-stream:
			if !ok {
				if ctx.Err() == nil {
					w.report(fmt.Errorf("etcd: watch stream closed for %s", prefix))
				}

				return
			}

			if resp.Err() != nil {
				w.report(fmt.Errorf("etcd: watch stream failed for %s: %w", prefix, resp.Err()))
				return
			}

			for _, event := range resp.Events {
				if event.Kv == nil {
					continue
				}

				key := string(event.Kv.Key)

				w.mu.Lock()

				if event.IsCreate() || event.IsModify() {
					copied := make([]byte, len(event.Kv.Value))
					copy(copied, event.Kv.Value)
					w.cache[key] = copied
				} else {
					delete(w.cache, key)
				}

				w.mu.Unlock()

				onChange(event.Kv.Key, event.Kv.Value)
			}
		}
	}
}
