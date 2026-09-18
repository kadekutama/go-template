package etcd_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	coordination "github.com/kadekutama/go-template/internal/infrastructure/coordination/etcd"
	testcontainers "github.com/kadekutama/go-template/test/testcontainers"
)

// etcdImage pins the coordination-plane image for integration suites.
const etcdImage = "quay.io/coreos/etcd:v3.7.0"

// startEtcd boots one isolated etcd container and returns its client URL.
// It skips cleanly when no Docker daemon is reachable.
func startEtcd(t *testing.T) string {
	t.Helper()
	testcontainers.SkipIfNoDocker(t)

	ctx, cancel := testcontainers.Background()
	defer cancel()

	req := tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        etcdImage,
			ExposedPorts: []string{"2379/tcp", "2380/tcp"},
			Cmd: []string{
				"etcd",
				"--advertise-client-urls", "http://0.0.0.0:2379",
				"--listen-client-urls", "http://0.0.0.0:2379",
				"--initial-advertise-peer-urls", "http://0.0.0.0:2380",
				"--listen-peer-urls", "http://0.0.0.0:2380",
				"--initial-cluster", "default=http://0.0.0.0:2380",
			},
			WaitingFor: wait.ForLog("ready to serve client request").
				WithStartupTimeout(testcontainers.StartupTimeout()).
				WithPollInterval(500 * time.Millisecond),
		},
		Started: true,
	}

	container, err := tc.GenericContainer(ctx, req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	mapped, err := container.MappedPort(ctx, "2379")
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	return fmt.Sprintf("http://%s:%s", host, mapped.Port())
}

func TestEtcdWatcherStreamsMutations(t *testing.T) {
	endpoint := startEtcd(t)
	ctx := context.Background()

	client, err := coordination.NewClient(coordination.ClientParams{
		Config: coordination.Config{Endpoints: []string{endpoint}},
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	require.NoError(t, client.Health(ctx))

	watcher, err := coordination.NewWatcher(coordination.WatcherParams{Client: client})
	require.NoError(t, err)

	received := make(chan string, 4)

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	require.NoError(t, watcher.WatchPrefix(watchCtx, "/config/", func(_, value []byte) {
		received <- string(value)
	}))

	require.NoError(t, client.Put(ctx, "/config/fee_schedule", "v2"))

	select {
	case value := <-received:
		assert.Equal(t, "v2", value)
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for streamed watch mutation")
	}

	cached, ok := watcher.Cached("/config/fee_schedule")
	require.True(t, ok)
	assert.Equal(t, "v2", string(cached))
}

func TestEtcdElectionCampaignResign(t *testing.T) {
	endpoint := startEtcd(t)
	ctx := context.Background()

	newElector := func(prefix string) coordination.LeaderElector {
		t.Helper()

		client, err := coordination.NewClient(coordination.ClientParams{
			Config: coordination.Config{Endpoints: []string{endpoint}},
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })

		elector, err := coordination.NewElection(coordination.ElectionParams{
			Client:     client,
			KeyPrefix:  prefix,
			TTLSeconds: 5,
		})
		require.NoError(t, err)

		return elector
	}

	const prefix = "/finance/reconciliation-leader"

	leaderA := newElector(prefix)
	require.NoError(t, leaderA.Campaign(ctx, "pod-a"))

	holder, err := leaderA.Leader(ctx)
	require.NoError(t, err)
	assert.Equal(t, "pod-a", holder)

	require.Error(t, leaderA.Campaign(ctx, "pod-a"),
		"campaigning twice on one elector must fail without touching the live lease")

	standby := newElector(prefix)

	short, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	campaignErr := make(chan error, 1)
	go func() { campaignErr <- standby.Campaign(short, "pod-b") }()

	select {
	case err := <-campaignErr:
		require.Error(t, err, "standby must not preempt the active leader")
	case <-time.After(4 * time.Second):
		t.Fatal("standby campaign did not respect active leadership")
	}

	require.NoError(t, leaderA.Resign(ctx))

	require.NoError(t, standby.Campaign(ctx, "pod-b"))

	holder, err = standby.Leader(ctx)
	require.NoError(t, err)
	assert.Equal(t, "pod-b", holder)

	require.NoError(t, standby.Resign(ctx))

	// Test concurrent campaigning on a single elector: exactly one may acquire,
	// while the second must fail immediately with "already campaigning".
	concurrentElector := newElector("/finance/concurrent-test")
	ready := make(chan struct{})
	done := make(chan error, 2)
	for _, id := range []string{"pod-concurrent-a", "pod-concurrent-b"} {
		go func(id string) {
			<-ready
			done <- concurrentElector.Campaign(ctx, id)
		}(id)
	}
	close(ready)
	err1 := <-done
	err2 := <-done
	assert.True(t, (err1 == nil && err2 != nil) || (err1 != nil && err2 == nil), "exactly one campaign must succeed")
	if err1 == nil {
		assert.Contains(t, err2.Error(), "already campaigning")
	} else {
		assert.Contains(t, err1.Error(), "already campaigning")
	}
	require.NoError(t, concurrentElector.Resign(ctx))
}

func TestEtcdElectionCancelCleanup(t *testing.T) {
	endpoint := startEtcd(t)
	ctx := context.Background()

	client, err := coordination.NewClient(coordination.ClientParams{
		Config: coordination.Config{Endpoints: []string{endpoint}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	elector, err := coordination.NewElection(coordination.ElectionParams{
		Client:     client,
		KeyPrefix:  "/finance/cancel-test",
		TTLSeconds: 5,
	})
	require.NoError(t, err)

	canceled, cancel := context.WithCancel(ctx)
	cancel()

	require.Error(t, elector.Campaign(canceled, "pod-cancel"),
		"canceled campaign must fail without wedging the elector")

	require.NoError(t, elector.Campaign(ctx, "pod-cancel"),
		"elector must campaign cleanly after a canceled attempt")

	holder, err := elector.Leader(ctx)
	require.NoError(t, err)
	assert.Equal(t, "pod-cancel", holder)

	require.NoError(t, elector.Resign(ctx))
}

func TestEtcdElectionTrimsCandidate(t *testing.T) {
	endpoint := startEtcd(t)
	ctx := context.Background()

	client, err := coordination.NewClient(coordination.ClientParams{
		Config: coordination.Config{Endpoints: []string{endpoint}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	elector, err := coordination.NewElection(coordination.ElectionParams{
		Client:     client,
		KeyPrefix:  "/finance/trim-test",
		TTLSeconds: 5,
	})
	require.NoError(t, err)

	require.NoError(t, elector.Campaign(ctx, "  pod-trim  "))

	holder, err := elector.Leader(ctx)
	require.NoError(t, err)
	assert.Equal(t, "pod-trim", holder, "padded candidate IDs must normalize")

	require.NoError(t, elector.Resign(ctx))
}
