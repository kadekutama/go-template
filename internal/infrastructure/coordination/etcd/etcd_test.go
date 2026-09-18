package etcd_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	coordination "github.com/kadekutama/go-template/internal/infrastructure/coordination/etcd"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// stubLogger discards every line; it proves fx resolution without asserting output.
type stubLogger struct{}

func (stubLogger) Trace(context.Context, string, ...any) {}
func (stubLogger) Debug(context.Context, string, ...any) {}
func (stubLogger) Info(context.Context, string, ...any)  {}
func (stubLogger) Warn(context.Context, string, ...any)  {}
func (stubLogger) Error(context.Context, string, ...any) {}
func (stubLogger) Panic(_ context.Context, msg string, _ ...any) {
	panic(msg)
}
func (stubLogger) Fatal(context.Context, string, ...any) {}
func (stubLogger) With(...any) log.Logger                { return stubLogger{} }

func TestConfigForEndpoints(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                string
		endpoints           []string
		dialTimeout         time.Duration
		electionTTLSeconds  int
		expectedDialTimeout time.Duration
		expectedTTLSeconds  int
		expectedPrefix      string
	}

	testCases := []testCase{
		{
			name:                "explicit values kept",
			endpoints:           []string{"http://etcd-0:2379", "http://etcd-1:2379"},
			dialTimeout:         3 * time.Second,
			electionTTLSeconds:  7,
			expectedDialTimeout: 3 * time.Second,
			expectedTTLSeconds:  7,
			expectedPrefix:      coordination.DefaultLeaderKeyPrefix,
		},
		{
			name:                "non-positive values fall back to defaults",
			endpoints:           []string{"http://etcd-0:2379"},
			dialTimeout:         0,
			electionTTLSeconds:  0,
			expectedDialTimeout: coordination.DefaultDialTimeout,
			expectedTTLSeconds:  coordination.DefaultElectionTTLSeconds,
			expectedPrefix:      coordination.DefaultLeaderKeyPrefix,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := coordination.ConfigForEndpoints(tc.endpoints, tc.dialTimeout, tc.electionTTLSeconds)
			assert.Equal(t, tc.endpoints, cfg.Endpoints)
			assert.Equal(t, tc.expectedDialTimeout, cfg.DialTimeout)
			assert.Equal(t, tc.expectedTTLSeconds, cfg.ElectionTTLSeconds)
			assert.Equal(t, tc.expectedPrefix, cfg.LeaderKeyPrefix)
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
				Config: coordination.DefaultConfig(),
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
				client, err := coordination.NewClient(coordination.ClientParams{Config: coordination.DefaultConfig()})
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
				client, err := coordination.NewClient(coordination.ClientParams{Config: coordination.DefaultConfig()})
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
			name: "blank prefix falls back to default",
			params: func() coordination.ElectionParams {
				client, err := coordination.NewClient(coordination.ClientParams{Config: coordination.DefaultConfig()})
				require.NoError(t, err)
				t.Cleanup(func() { _ = client.Close() })
				return coordination.ElectionParams{
					Client:     client,
					KeyPrefix:  "",
					TTLSeconds: 0,
				}
			}(),
			expectedPrefix: coordination.DefaultLeaderKeyPrefix,
			expectedError:  false,
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
			client, err := coordination.NewClient(coordination.ClientParams{Config: coordination.DefaultConfig()})
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })

			elector, err := coordination.NewElection(coordination.ElectionParams{Client: client})
			require.NoError(t, err)

			err = elector.Campaign(context.Background(), tc.candidateID)
			require.Error(t, err)
			assert.Equal(t, tc.expectedError, err.Error())
		})
	}

	t.Run("uninitialized elector returns descriptive errors", func(t *testing.T) {
		uninit, err := coordination.NewElection(coordination.ElectionParams{
			Client: func() *coordination.Client {
				c, err := coordination.NewClient(coordination.ClientParams{Config: coordination.DefaultConfig()})
				require.NoError(t, err)
				t.Cleanup(func() { _ = c.Close() })
				return c
			}(),
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
				fx.Provide(func() log.Logger { return stubLogger{} }),
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
				fx.Provide(func() log.Logger { return stubLogger{} }),
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

func TestDefaultConfigEnv(t *testing.T) {
	type testCase struct {
		name                 string
		endpointsEnv         string
		dialTimeoutEnv       string
		electionTTLEnv       string
		leaderPrefixEnv      string
		expectedEndpoints    []string
		expectedDialTimeout  time.Duration
		expectedTTLSeconds   int
		expectedLeaderPrefix string
	}

	testCases := []testCase{
		{
			name:                 "unset environment keeps compiled defaults",
			endpointsEnv:         "",
			dialTimeoutEnv:       "",
			electionTTLEnv:       "",
			leaderPrefixEnv:      "",
			expectedEndpoints:    []string{"http://127.0.0.1:2379"},
			expectedDialTimeout:  coordination.DefaultDialTimeout,
			expectedTTLSeconds:   coordination.DefaultElectionTTLSeconds,
			expectedLeaderPrefix: coordination.DefaultLeaderKeyPrefix,
		},
		{
			name:                 "full override applies",
			endpointsEnv:         "http://etcd-a:2379, http://etcd-b:2379",
			dialTimeoutEnv:       "7",
			electionTTLEnv:       "9",
			leaderPrefixEnv:      "/finance/custom",
			expectedEndpoints:    []string{"http://etcd-a:2379", "http://etcd-b:2379"},
			expectedDialTimeout:  7 * time.Second,
			expectedTTLSeconds:   9,
			expectedLeaderPrefix: "/finance/custom",
		},
		{
			name:                 "malformed values keep defaults",
			endpointsEnv:         "   ",
			dialTimeoutEnv:       "soon",
			electionTTLEnv:       "-3",
			leaderPrefixEnv:      "  ",
			expectedEndpoints:    []string{"http://127.0.0.1:2379"},
			expectedDialTimeout:  coordination.DefaultDialTimeout,
			expectedTTLSeconds:   coordination.DefaultElectionTTLSeconds,
			expectedLeaderPrefix: coordination.DefaultLeaderKeyPrefix,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_COORDINATION__ETCD_ENDPOINTS", tc.endpointsEnv)
			t.Setenv("APP_COORDINATION__ETCD_DIAL_TIMEOUT_SEC", tc.dialTimeoutEnv)
			t.Setenv("APP_COORDINATION__ETCD_ELECTION_TTL_SEC", tc.electionTTLEnv)
			t.Setenv("APP_COORDINATION__ETCD_LEADER_PREFIX", tc.leaderPrefixEnv)

			cfg := coordination.DefaultConfig()
			assert.Equal(t, tc.expectedEndpoints, cfg.Endpoints)
			assert.Equal(t, tc.expectedDialTimeout, cfg.DialTimeout)
			assert.Equal(t, tc.expectedTTLSeconds, cfg.ElectionTTLSeconds)
			assert.Equal(t, tc.expectedLeaderPrefix, cfg.LeaderKeyPrefix)
			assert.NoError(t, cfg.Validate())
		})
	}
}
