package rbac_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/infrastructure/auth/rbac"
)

func TestCSVLoader(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	validCSV := filepath.Join(tmpDir, "valid.csv")
	err := os.WriteFile(validCSV, []byte(`p, admin, tenant-1, ledger, write, allow
p, viewer, tenant-1, ledger, read, allow
g, alice, admin, tenant-1
g, bob, viewer, tenant-1
`), 0o600)
	require.NoError(t, err)

	invalidCSV := filepath.Join(tmpDir, "invalid.csv")
	err = os.WriteFile(invalidCSV, []byte(`"unclosed quote, p, admin`), 0o600)
	require.NoError(t, err)

	type testCase struct {
		name              string
		path              string
		expectedPolicyLen int
		expectedGroupLen  int
		expectedError     error
	}

	testCases := []testCase{
		{
			name:              "valid csv file",
			path:              validCSV,
			expectedPolicyLen: 2,
			expectedGroupLen:  2,
			expectedError:     nil,
		},
		{
			name:              "empty path",
			path:              "",
			expectedPolicyLen: 0,
			expectedGroupLen:  0,
			expectedError:     rbac.ErrPolicyInvalid,
		},
		{
			name:              "non-existent file",
			path:              filepath.Join(tmpDir, "absent.csv"),
			expectedPolicyLen: 0,
			expectedGroupLen:  0,
			expectedError:     rbac.ErrPolicyInvalid,
		},
		{
			name:              "malformed csv syntax",
			path:              invalidCSV,
			expectedPolicyLen: 0,
			expectedGroupLen:  0,
			expectedError:     rbac.ErrPolicyInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			loader := rbac.NewCSVLoader(tc.path)
			policies, grouping, err := loader.Load()
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, policies, tc.expectedPolicyLen)
			assert.Len(t, grouping, tc.expectedGroupLen)
		})
	}
}

func TestDBPolicyLoader(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		loader        *rbac.DBPolicyLoader
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil loader load fails",
			loader:        nil,
			expectedError: rbac.ErrPolicyInvalid,
		},
		{
			name: "disconnected db query fails",
			loader: func() *rbac.DBPolicyLoader {
				db, err := gorm.Open(gormpostgres.Open("postgres://127.0.0.1:1/rbac_unit?sslmode=disable"), &gorm.Config{
					SkipDefaultTransaction: true,
					DisableAutomaticPing:   true,
				})
				if err != nil {
					panic(err)
				}
				l, err := rbac.NewDBPolicyLoader(db)
				if err != nil {
					panic(err)
				}
				return l
			}(),
			expectedError: rbac.ErrPolicyInvalid,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policies, grouping, err := tc.loader.Load()
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Nil(t, policies)
				assert.Nil(t, grouping)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestNewDBPolicyLoaderValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		db            *gorm.DB
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil db returns error",
			db:            nil,
			expectedError: rbac.ErrPolicyInvalid,
		},
		{
			name: "valid db returns loader",
			db: func() *gorm.DB {
				db, err := gorm.Open(gormpostgres.Open("postgres://127.0.0.1:1/rbac_unit?sslmode=disable"), &gorm.Config{
					SkipDefaultTransaction: true,
					DisableAutomaticPing:   true,
				})
				if err != nil {
					panic(err)
				}
				return db
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			loader, err := rbac.NewDBPolicyLoader(tc.db)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Nil(t, loader)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, loader)
		})
	}
}

func TestCasbinRuleModelTableName(t *testing.T) {
	t.Parallel()

	model := rbac.CasbinRuleModel{}
	assert.Equal(t, "casbin_rules", model.TableName())
}
