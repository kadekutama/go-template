package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
)

func TestNewPoolsRequiresPrimary(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        postgres.PoolsParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil primary rejected",
			params: postgres.PoolsParams{
				Primary: nil,
			},
			expectedError: errors.New("postgres: replica pools need a primary"),
		},
		{
			name: "primary only accepted",
			params: postgres.PoolsParams{
				Primary: &gorm.DB{},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pools, err := postgres.NewPools(tc.params)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
				assert.Nil(t, pools)
			} else {
				require.NoError(t, err)
				require.NotNil(t, pools)
			}
		})
	}
}

func TestSelectorRouting(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		hasReplica    bool
		source        postgres.LagSource
		expectReplica bool
	}

	testCases := []testCase{
		{
			name:          "lag within bound reads replica",
			hasReplica:    true,
			source:        postgres.StaticLagSource{Lag: 2},
			expectReplica: true,
		},
		{
			name:          "lag beyond bound falls back to primary",
			hasReplica:    true,
			source:        postgres.StaticLagSource{Lag: 120},
			expectReplica: false,
		},
		{
			name:          "lag sample error fails toward primary",
			hasReplica:    true,
			source:        postgres.StaticLagSource{Err: errors.New("unreachable")},
			expectReplica: false,
		},
		{
			name:          "nil source always primary",
			hasReplica:    true,
			source:        nil,
			expectReplica: false,
		},
		{
			name:          "nil replica always primary",
			hasReplica:    false,
			source:        postgres.StaticLagSource{Lag: 0},
			expectReplica: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			primary := &gorm.DB{}
			var replica *gorm.DB
			if tc.hasReplica {
				replica = &gorm.DB{}
			}

			pools, err := postgres.NewPools(postgres.PoolsParams{Primary: primary, Replica: replica})
			require.NoError(t, err)

			selector, err := postgres.NewSelector(postgres.SelectorParams{
				Pools:    pools,
				LagBound: 30,
				Source:   tc.source,
			})
			require.NoError(t, err)

			assert.Same(t, primary, selector.Write(), "writes always use primary")

			if tc.expectReplica {
				assert.Same(t, replica, selector.Read())
			} else {
				assert.Same(t, primary, selector.Read())
			}
		})
	}
}

func TestNewSelectorGuards(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        postgres.SelectorParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil pools rejected",
			params: postgres.SelectorParams{
				Pools:    nil,
				LagBound: 30,
			},
			expectedError: errors.New("postgres: selector needs pools"),
		},
		{
			name: "non-positive bound rejected",
			params: func() postgres.SelectorParams {
				pools, _ := postgres.NewPools(postgres.PoolsParams{Primary: &gorm.DB{}})
				return postgres.SelectorParams{
					Pools:    pools,
					LagBound: 0,
				}
			}(),
			expectedError: errors.New("postgres: lag bound must be positive"),
		},
		{
			name: "negative bound rejected",
			params: func() postgres.SelectorParams {
				pools, _ := postgres.NewPools(postgres.PoolsParams{Primary: &gorm.DB{}})
				return postgres.SelectorParams{
					Pools:    pools,
					LagBound: -5,
				}
			}(),
			expectedError: errors.New("postgres: lag bound must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selector, err := postgres.NewSelector(tc.params)
			require.Error(t, err)
			assert.Equal(t, tc.expectedError.Error(), err.Error())
			assert.Nil(t, selector)
		})
	}
}

func TestLagAlertShape(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		expectedMetric string
		minThreshold   float64
		expectSeverity bool
	}

	testCases := []testCase{
		{
			name:           "canonical metric and threshold present",
			expectedMetric: "replication_lag_seconds",
			minThreshold:   0,
			expectSeverity: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rule := postgres.DefaultLagAlertRule()
			assert.Equal(t, tc.expectedMetric, rule.Metric)
			assert.Equal(t, postgres.LagMetricName, rule.Metric)
			assert.Greater(t, rule.Threshold, tc.minThreshold)
			if tc.expectSeverity {
				assert.NotEmpty(t, rule.Severity)
			}
		})
	}
}

func TestRouterResidency(t *testing.T) {
	t.Parallel()

	type auditEntry struct {
		tenant string
		region string
	}

	type testCase struct {
		name          string
		residency     postgres.Residency
		expectRegion  string
		expectedError error
		expectAudited bool
	}

	testCases := []testCase{
		{
			name: "eu tenant resolves eu pool",
			residency: postgres.Residency{
				TenantID: valueobject.TenantID("tnt-eu-01"),
				Region:   "eu",
			},
			expectRegion:  "eu",
			expectedError: nil,
			expectAudited: true,
		},
		{
			name: "us tenant never touches eu pool",
			residency: postgres.Residency{
				TenantID: valueobject.TenantID("tnt-us-01"),
				Region:   "us",
			},
			expectRegion:  "us",
			expectedError: nil,
			expectAudited: true,
		},
		{
			name: "unknown region fails closed",
			residency: postgres.Residency{
				TenantID: valueobject.TenantID("tnt-xx-01"),
				Region:   "xx",
			},
			expectRegion:  "",
			expectedError: errors.New("postgres: unknown region xx"),
			expectAudited: false,
		},
		{
			name: "missing region fails closed",
			residency: postgres.Residency{
				TenantID: valueobject.TenantID("tnt-xx-02"),
				Region:   "",
			},
			expectRegion:  "",
			expectedError: errors.New("postgres: residency region is required"),
			expectAudited: false,
		},
		{
			name: "missing tenant fails closed",
			residency: postgres.Residency{
				TenantID: valueobject.TenantID(""),
				Region:   "eu",
			},
			expectRegion:  "",
			expectedError: errors.New("postgres: tenant is required"),
			expectAudited: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			euPool := &gorm.DB{}
			usPool := &gorm.DB{}
			pools := map[string]*gorm.DB{"eu": euPool, "us": usPool}

			var audited []auditEntry

			router, err := postgres.NewRouter(postgres.RouterParams{
				Pools: pools,
				Audit: func(_ context.Context, tenant valueobject.TenantID, region string, _ time.Time) {
					audited = append(audited, auditEntry{tenant: tenant.String(), region: region})
				},
			})
			require.NoError(t, err)

			pool, err := router.Resolve(context.Background(), tc.residency)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
				assert.Nil(t, pool)
				assert.Empty(t, audited)
			} else {
				require.NoError(t, err)
				assert.Same(t, pools[tc.expectRegion], pool)
				require.Len(t, audited, 1)
				assert.Equal(t, tc.residency.TenantID.String(), audited[0].tenant)
				assert.Equal(t, tc.residency.Region, audited[0].region)
			}
		})
	}
}

func TestNewRouterGuards(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        postgres.RouterParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "empty registry rejected",
			params: postgres.RouterParams{
				Pools: map[string]*gorm.DB{},
			},
			expectedError: errors.New("postgres: router needs at least one region pool"),
		},
		{
			name: "blank region rejected",
			params: postgres.RouterParams{
				Pools: map[string]*gorm.DB{"": {}},
			},
			expectedError: errors.New("postgres: router needs a named pool per region"),
		},
		{
			name: "nil pool rejected",
			params: postgres.RouterParams{
				Pools: map[string]*gorm.DB{"eu": nil},
			},
			expectedError: errors.New("postgres: router needs a named pool per region"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router, err := postgres.NewRouter(tc.params)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
				assert.Nil(t, router)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
