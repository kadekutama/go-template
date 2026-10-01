package apikey_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/apikey"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

func TestNewManagerValidation(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		params        apikey.APIKeyParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "missing clock rejected",
			params: func() apikey.APIKeyParams {
				db, err := gorm.Open(gormpostgres.Open("postgres://127.0.0.1:1/apikey_unit?sslmode=disable"), &gorm.Config{
					SkipDefaultTransaction: true,
					DisableAutomaticPing:   true,
				})
				if err != nil {
					panic(err)
				}

				return apikey.APIKeyParams{DB: db, Clock: nil}
			}(),
			expectedError: apikey.ErrClockRequired,
		},
		{
			name: "missing db rejected",
			params: func() apikey.APIKeyParams {
				return apikey.APIKeyParams{DB: nil, Clock: clk}
			}(),
			expectedError: apikey.ErrConfigRequired,
		},
		{
			name: "missing hasher rejected",
			params: func() apikey.APIKeyParams {
				db, err := gorm.Open(gormpostgres.Open("postgres://127.0.0.1:1/apikey_unit?sslmode=disable"), &gorm.Config{
					SkipDefaultTransaction: true,
					DisableAutomaticPing:   true,
				})
				if err != nil {
					panic(err)
				}

				return apikey.APIKeyParams{DB: db, Clock: clk, Hasher: apikey.Hasher{}}
			}(),
			expectedError: apikey.ErrConfigRequired,
		},
		{
			name: "zero rotation window rejected",
			params: func() apikey.APIKeyParams {
				db, err := gorm.Open(gormpostgres.Open("postgres://127.0.0.1:1/apikey_unit?sslmode=disable"), &gorm.Config{
					SkipDefaultTransaction: true,
					DisableAutomaticPing:   true,
				})
				if err != nil {
					panic(err)
				}

				hasher, err := apikey.NewHasher(testHasherParams())
				if err != nil {
					panic(err)
				}

				return apikey.APIKeyParams{DB: db, Clock: clk, Hasher: hasher, RotationWindow: 0}
			}(),
			expectedError: apikey.ErrConfigRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := apikey.NewManager(tc.params)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func testHasherParams() apikey.HasherParams {
	return apikey.HasherParams{
		Time:    3,
		Memory:  64 * 1024,
		Threads: 4,
		KeyLen:  32,
		SaltLen: 16,
	}
}

func TestHasherRoundTrip(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		secret        string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "round trip",
			secret:        "ak_abcdefgh.tenant-x." + string(make([]byte, 32)),
			expectedError: nil,
		},
		{
			name:          "empty secret rejected",
			secret:        "",
			expectedError: apikey.ErrInvalidFormat,
		},
		{
			name:          "whitespace secret hashes",
			secret:        "   ",
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasher, err := apikey.NewHasher(testHasherParams())
			require.NoError(t, err)

			sealed, err := hasher.Hash(tc.secret)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.NotContains(t, string(sealed.Hash), tc.secret)
			assert.True(t, hasher.Verify(sealed, tc.secret))
			assert.False(t, hasher.Verify(sealed, tc.secret+"x"))

			wrongVersion := sealed
			wrongVersion.Version = sealed.Version + 1
			assert.False(t, hasher.Verify(wrongVersion, tc.secret))
		})
	}
}

func TestNewHasherValidation(t *testing.T) {
	t.Parallel()

	valid := testHasherParams()

	type testCase struct {
		name          string
		params        apikey.HasherParams
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "defaults accepted",
			params:        valid,
			expectedError: nil,
		},
		{
			name: "zero time rejected",
			params: func() apikey.HasherParams {
				p := valid
				p.Time = 0
				return p
			}(),
			expectedError: apikey.ErrConfigRequired,
		},
		{
			name: "small memory rejected",
			params: func() apikey.HasherParams {
				p := valid
				p.Memory = 1024
				return p
			}(),
			expectedError: apikey.ErrConfigRequired,
		},
		{
			name: "zero threads rejected",
			params: func() apikey.HasherParams {
				p := valid
				p.Threads = 0
				return p
			}(),
			expectedError: apikey.ErrConfigRequired,
		},
		{
			name: "custom cost round-trips",
			params: func() apikey.HasherParams {
				p := valid
				p.Time = 2
				p.Memory = 32 * 1024
				return p
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasher, err := apikey.NewHasher(tc.params)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			sealed, err := hasher.Hash("probe-secret")
			require.NoError(t, err)
			assert.True(t, hasher.Verify(sealed, "probe-secret"))
			assert.False(t, hasher.Verify(sealed, "other-secret"))
		})
	}
}

func TestHasScope(t *testing.T) {
	t.Parallel()

	key := appport.APIKey{Scopes: []string{"ledger:read"}}

	type testCase struct {
		name          string
		scope         string
		expectedScope bool
	}

	testCases := []testCase{
		{
			name:          "granted scope",
			scope:         "ledger:read",
			expectedScope: true,
		},
		{
			name:          "missing scope",
			scope:         "ledger:write",
			expectedScope: false,
		},
		{
			name:          "empty scope",
			scope:         "",
			expectedScope: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedScope, apikey.HasScope(key, tc.scope))
		})
	}
}

func TestManagerRequiresDB(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	manager, err := apikey.NewManager(apikey.APIKeyParams{DB: nil, Clock: clk})
	require.ErrorIs(t, err, apikey.ErrConfigRequired)
	assert.Nil(t, manager)
}
