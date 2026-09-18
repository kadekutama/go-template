package etcd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// serveHarness runs serve in a goroutine and returns a waiter for its exit.
func serveHarness(
	t *testing.T,
	w *watcher,
	ctx context.Context,
	prefix string,
	stream clientv3.WatchChan,
	onChange WatchHandler,
) <-chan struct{} {
	t.Helper()

	done := make(chan struct{})

	go func() {
		defer close(done)
		w.serve(ctx, prefix, stream, onChange)
	}()

	return done
}

func awaitServeExit(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serve goroutine leaked after cancel")
	}
}

func TestWatcherServeErrorPaths(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		cancelBefore  bool
		stream        clientv3.WatchChan
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "clean cancellation is silent",
			cancelBefore:  true,
			stream:        make(chan clientv3.WatchResponse),
			expectedError: false,
		},
		{
			name:         "closed stream reports failure",
			cancelBefore: false,
			stream: func() clientv3.WatchChan {
				stream := make(chan clientv3.WatchResponse)
				close(stream)

				return stream
			}(),
			expectedError: true,
		},
		{
			name:         "canceled response reports failure",
			cancelBefore: false,
			stream: func() clientv3.WatchChan {
				stream := make(chan clientv3.WatchResponse, 1)
				stream <- clientv3.WatchResponse{Canceled: true, CancelReason: "test compaction"}

				return stream
			}(),
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := &watcher{cache: make(map[string][]byte), errCh: make(chan error, 4)}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tc.cancelBefore {
				cancel()
			}

			done := serveHarness(t, w, ctx, "/config/", tc.stream, func(_, _ []byte) {})

			select {
			case err := <-w.Err():
				if !tc.expectedError {
					t.Fatalf("unexpected stream error: %v", err)
				}

				assert.Error(t, err)
			case <-time.After(200 * time.Millisecond):
				if tc.expectedError {
					t.Fatal("expected stream error, got silence")
				}
			}

			if !tc.cancelBefore {
				cancel()
			}

			awaitServeExit(t, done)
		})
	}
}

func TestWatcherSpawnRecoversPanics(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		panicValue    any
		expectedError string
	}

	testCases := []testCase{
		{
			name:          "handler panic surfaces on Err",
			panicValue:    "boom",
			expectedError: "etcd: watcher panic for /config/: safe: recovered panic: boom",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := &watcher{cache: make(map[string][]byte), errCh: make(chan error, 4)}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stream := make(chan clientv3.WatchResponse)
			w.spawn(ctx, "/config/", stream, func(_, _ []byte) {
				panic(tc.panicValue)
			})

			// Drive one mutation through the panicking handler, then release.
			stream <- clientv3.WatchResponse{
				Events: []*clientv3.Event{
					{
						Type: clientv3.EventTypePut,
						Kv:   &mvccpb.KeyValue{Key: []byte("/config/x"), Value: []byte("1")},
					},
				},
			}

			select {
			case err := <-w.Err():
				require.Error(t, err)
				assert.Equal(t, tc.expectedError, err.Error())
			case <-time.After(5 * time.Second):
				t.Fatal("panic was not recovered and reported")
			}
		})
	}
}

func TestWatcherServeMutation(t *testing.T) {
	t.Parallel()

	w := &watcher{cache: make(map[string][]byte), errCh: make(chan error, 4)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := make(chan clientv3.WatchResponse, 1)
	stream <- clientv3.WatchResponse{
		Events: []*clientv3.Event{
			{
				Type: clientv3.EventTypePut,
				Kv: &mvccpb.KeyValue{
					Key:   []byte("/config/fee"),
					Value: []byte("v3"),
				},
			},
		},
	}

	handled := make(chan []byte, 1)
	done := serveHarness(t, w, ctx, "/config/", stream, func(_, value []byte) {
		handled <- value
	})

	select {
	case value := <-handled:
		assert.Equal(t, "v3", string(value))
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for mutation handler")
	}

	cancel()
	awaitServeExit(t, done)

	cached, ok := w.Cached("/config/fee")
	require.True(t, ok)
	assert.Equal(t, "v3", string(cached))

	select {
	case err := <-w.Err():
		t.Fatalf("mutation path must not report errors: %v", err)
	default:
	}
}
