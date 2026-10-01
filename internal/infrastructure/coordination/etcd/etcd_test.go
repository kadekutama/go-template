package etcd_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	coordination "github.com/kadekutama/go-template/internal/infrastructure/coordination/etcd"
	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

func testEtcdConfig() coordination.Config {
	return coordination.Config{
		Endpoints:          []string{"http://127.0.0.1:2379"},
		DialTimeout:        5 * time.Second,
		ElectionTTLSeconds: 5,
		LeaderKeyPrefix:    "/finance/worker-leader",
	}
}

func TestConfigForEndpoints(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		endpoints          []string
		dialTimeout        time.Duration
		electionTTLSeconds int
		leaderKeyPrefix    string
		expectedError      bool
	}

	testCases := []testCase{
		{
			name:               "explicit valid values kept",
			endpoints:          []string{"http://etcd-0:2379", "http://etcd-1:2379"},
			dialTimeout:        3 * time.Second,
			electionTTLSeconds: 7,
			leaderKeyPrefix:    "/finance/custom-leader",
			expectedError:      false,
		},
		{
			name:               "invalid zero values rejected",
			endpoints:          []string{"http://etcd-0:2379"},
			dialTimeout:        0,
			electionTTLSeconds: 0,
			leaderKeyPrefix:    "",
			expectedError:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := coordination.ConfigForEndpoints(tc.endpoints, tc.dialTimeout, tc.electionTTLSeconds, tc.leaderKeyPrefix)
			if tc.expectedError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.endpoints, cfg.Endpoints)
			assert.Equal(t, tc.dialTimeout, cfg.DialTimeout)
			assert.Equal(t, tc.electionTTLSeconds, cfg.ElectionTTLSeconds)
			assert.Equal(t, tc.leaderKeyPrefix, cfg.LeaderKeyPrefix)
			assert.NoError(t, cfg.Validate())
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		config        coordination.Config
		expectedError string
	}

	testCases := []testCase{
		{
			name: "valid endpoints",
			config: coordination.Config{
				Endpoints:          []string{"http://127.0.0.1:2379"},
				DialTimeout:        5 * time.Second,
				ElectionTTLSeconds: 5,
				LeaderKeyPrefix:    "/finance/worker-leader",
			},
			expectedError: "",
		},
		{
			name: "missing endpoints",
			config: coordination.Config{
				Endpoints:          nil,
				DialTimeout:        5 * time.Second,
				ElectionTTLSeconds: 5,
				LeaderKeyPrefix:    "/finance/worker-leader",
			},
			expectedError: "etcd: at least one endpoint is required",
		},
		{
			name: "blank endpoint",
			config: coordination.Config{
				Endpoints:          []string{"   "},
				DialTimeout:        5 * time.Second,
				ElectionTTLSeconds: 5,
				LeaderKeyPrefix:    "/finance/worker-leader",
			},
			expectedError: "etcd: endpoint must not be blank",
		},
		{
			name: "non-positive dial timeout",
			config: coordination.Config{
				Endpoints:          []string{"http://127.0.0.1:2379"},
				DialTimeout:        0,
				ElectionTTLSeconds: 5,
				LeaderKeyPrefix:    "/finance/worker-leader",
			},
			expectedError: "etcd: dial timeout must be positive",
		},
		{
			name: "non-positive election ttl",
			config: coordination.Config{
				Endpoints:          []string{"http://127.0.0.1:2379"},
				DialTimeout:        5 * time.Second,
				ElectionTTLSeconds: 0,
				LeaderKeyPrefix:    "/finance/worker-leader",
			},
			expectedError: "etcd: election TTL must be positive",
		},
		{
			name: "blank leader prefix",
			config: coordination.Config{
				Endpoints:          []string{"http://127.0.0.1:2379"},
				DialTimeout:        5 * time.Second,
				ElectionTTLSeconds: 5,
				LeaderKeyPrefix:    "  ",
			},
			expectedError: "etcd: leader key prefix must not be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedError == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError, err.Error())
			}
		})
	}
}

func TestNewClientValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        coordination.ClientParams
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "missing endpoints fails",
			params:        coordination.ClientParams{},
			expectedError: true,
		},
		{
			name: "lazy client builds without a live server",
			params: coordination.ClientParams{
				Config: testEtcdConfig(),
			},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := coordination.NewClient(tc.params)
			if tc.expectedError {
				require.Error(t, err)
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			assert.NoError(t, client.Close())
		})
	}
}

func TestNewWatcherValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        coordination.WatcherParams
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "nil client fails",
			params:        coordination.WatcherParams{Client: nil},
			expectedError: true,
		},
		{
			name: "initialized client succeeds",
			params: func() coordination.WatcherParams {
				client, err := coordination.NewClient(coordination.ClientParams{Config: testEtcdConfig()})
				require.NoError(t, err)
				t.Cleanup(func() { _ = client.Close() })
				return coordination.WatcherParams{Client: client}
			}(),
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			watcher, err := coordination.NewWatcher(tc.params)
			if tc.expectedError {
				require.Error(t, err)
				assert.Nil(t, watcher)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, watcher)
		})
	}
}

func TestNewElectionValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         coordination.ElectionParams
		expectedPrefix string
		expectedError  bool
	}

	testCases := []testCase{
		{
			name:           "nil client fails",
			params:         coordination.ElectionParams{Client: nil, KeyPrefix: "/finance/a", TTLSeconds: 5},
			expectedPrefix: "",
			expectedError:  true,
		},
		{
			name: "explicit prefix kept",
			params: func() coordination.ElectionParams {
				client, err := coordination.NewClient(coordination.ClientParams{Config: testEtcdConfig()})
				require.NoError(t, err)
				t.Cleanup(func() { _ = client.Close() })
				return coordination.ElectionParams{
					Client:     client,
					KeyPrefix:  "/finance/reconciliation-leader",
					TTLSeconds: 5,
				}
			}(),
			expectedPrefix: "/finance/reconciliation-leader",
			expectedError:  false,
		},
		{
			name: "blank prefix rejected",
			params: func() coordination.ElectionParams {
				client, err := coordination.NewClient(coordination.ClientParams{Config: testEtcdConfig()})
				require.NoError(t, err)
				t.Cleanup(func() { _ = client.Close() })
				return coordination.ElectionParams{
					Client:     client,
					KeyPrefix:  "",
					TTLSeconds: 5,
				}
			}(),
			expectedPrefix: "",
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			election, err := coordination.NewElection(tc.params)
			if tc.expectedError {
				require.Error(t, err)
				assert.Nil(t, election)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, election)
			assert.Equal(t, tc.expectedPrefix, election.Prefix())
		})
	}
}

func TestElectionUnitMethods(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		candidateID   string
		expectedError string
	}

	testCases := []testCase{
		{
			name:          "blank candidate id rejected",
			candidateID:   "",
			expectedError: "etcd: candidate id must not be blank",
		},
		{
			name:          "whitespace candidate id rejected",
			candidateID:   "   ",
			expectedError: "etcd: candidate id must not be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := coordination.NewClient(coordination.ClientParams{Config: testEtcdConfig()})
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })

			elector, err := coordination.NewElection(coordination.ElectionParams{
				Client:     client,
				KeyPrefix:  "/finance/worker-leader",
				TTLSeconds: 5,
			})
			require.NoError(t, err)

			err = elector.Campaign(context.Background(), tc.candidateID)
			require.Error(t, err)
			assert.Equal(t, tc.expectedError, err.Error())
		})
	}

	t.Run("uninitialized elector returns descriptive errors", func(t *testing.T) {
		uninit, err := coordination.NewElection(coordination.ElectionParams{
			Client: func() *coordination.Client {
				c, err := coordination.NewClient(coordination.ClientParams{Config: testEtcdConfig()})
				require.NoError(t, err)
				t.Cleanup(func() { _ = c.Close() })
				return c
			}(),
			KeyPrefix:  "/finance/worker-leader",
			TTLSeconds: 5,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, uninit.Prefix())
		assert.Error(t, uninit.Resign(context.Background()))
	})
}

func TestEtcdModule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "fx resolves client watcher and election",
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := fx.New(
				coordination.Module(),
				fx.Provide(testEtcdConfig),
				fx.Provide(func() log.Logger { return logging.New(io.Discard, logging.Config{Level: "disabled"}) }),
				fx.Invoke(func(_ *coordination.Client, _ coordination.Watcher, _ coordination.LeaderElector) {
				}),
			)
			if tc.expectedError {
				require.Error(t, app.Err())
				return
			}

			require.NoError(t, app.Err())
		})
	}
}

func TestEtcdModuleDrainsClient(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "start then stop closes the client without a live server",
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := fx.New(
				coordination.Module(),
				fx.Provide(testEtcdConfig),
				fx.Provide(func() log.Logger { return logging.New(io.Discard, logging.Config{Level: "disabled"}) }),
				fx.Invoke(func(_ *coordination.Client) {}),
			)
			require.NoError(t, app.Err())

			ctx := context.Background()
			require.NoError(t, app.Start(ctx))

			err := app.Stop(ctx)
			if tc.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}
