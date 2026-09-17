package testcontainers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/test/testcontainers"
)

func TestHarnessSmoke(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string
	}

	testCases := []testCase{
		{
			name: "postgres valkey and nats boot and terminate",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testcontainers.SkipIfNoDocker(t)

			pg, err := testcontainers.StartPostgres(t, "ledger_smoke")
			require.NoError(t, err)
			assert.NotEmpty(t, pg.ConnectionString())

			vk, err := testcontainers.StartValkey(t)
			require.NoError(t, err)
			assert.NotEmpty(t, vk.Addr())

			nc, err := testcontainers.StartNATS(t)
			require.NoError(t, err)
			assert.NotEmpty(t, nc.URL())
		})
	}
}

func TestHarnessProfiles(t *testing.T) {
	type testCase struct {
		name            string
		profile         string
		expectedProfile testcontainers.Profile
	}

	testCases := []testCase{
		{
			name:            "default profile is local",
			profile:         "",
			expectedProfile: testcontainers.ProfileLocal,
		},
		{
			name:            "ci profile resolves",
			profile:         "ci",
			expectedProfile: testcontainers.ProfileCI,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TESTCONTAINERS_PROFILE", tc.profile)
			assert.Equal(t, tc.expectedProfile, testcontainers.ActiveProfile())
			assert.Positive(t, testcontainers.StartupTimeout())
		})
	}
}
