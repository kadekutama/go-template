package rbac_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/rbac"
)

// parityFixtureRow is one recorded authorization decision from the
// checked-in baseline.
type parityFixtureRow struct {
	Name  string `json:"name"`
	Sub   string `json:"sub"`
	Dom   string `json:"dom"`
	Obj   string `json:"obj"`
	Act   string `json:"act"`
	Allow bool   `json:"allow"`
}

// parityFixture is the checked-in authorization-decision baseline: the
// fixed tenant/role/allow/deny corpus every enforcer build must reproduce.
// Provenance (pre-upgrade module, version, checksum, live pin) is part of
// the fixture so the parity guarantee stays tied to the captured module;
// packet S04 requires it.
type parityFixture struct {
	Role    string             `json:"role"`
	Module  string             `json:"module"`
	Version string             `json:"version"`
	GoSumH1 string             `json:"go_sum_h1"`
	LivePin string             `json:"live_pin"`
	Cases   []parityFixtureRow `json:"cases"`
}

func loadParityFixture(t *testing.T) parityFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "authorization-parity.json"))
	require.NoError(t, err)

	var fixture parityFixture
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.NotEmpty(t, fixture.Cases)

	return fixture
}

// parityPolicies is the fixed policy corpus the parity test enforces. It
// mirrors production model usage: role grants, tenant domains, an explicit
// deny, and unassigned subjects.
func parityPolicies() []rbac.PolicyRule {
	return []rbac.PolicyRule{
		{Sub: "admin", Dom: "tenant-acme", Obj: "ledger", Act: "write", Effect: "allow"},
		{Sub: "viewer", Dom: "tenant-acme", Obj: "ledger", Act: "read", Effect: "allow"},
		{Sub: "admin", Dom: "tenant-beta", Obj: "ledger", Act: "write", Effect: "allow"},
		{Sub: "blocked", Dom: "tenant-acme", Obj: "ledger", Act: "read", Effect: "deny"},
	}
}

// parityGrouping assigns users to roles per tenant domain.
func parityGrouping() []rbac.GroupingRule {
	return []rbac.GroupingRule{
		{User: "alice", Role: "admin", Dom: "tenant-acme"},
		{User: "bob", Role: "viewer", Dom: "tenant-acme"},
		{User: "carol", Role: "admin", Dom: "tenant-beta"},
		{User: "dave", Role: "blocked", Dom: "tenant-acme"},
	}
}

// parityDecision runs one corpus case against the enforcer.
func parityDecision(t *testing.T, enf *rbac.Enforcer, sub, dom, obj, act string) bool {
	t.Helper()

	err := enf.Authorize(context.Background(), appport.Subject{
		ID:       sub,
		TenantID: valueobject.TenantID(dom),
	}, act, obj)

	return err == nil
}

func newParityEnforcer(t *testing.T) *rbac.Enforcer {
	t.Helper()

	enf, err := rbac.NewEnforcer(rbac.Params{
		Policies:             parityPolicies(),
		Grouping:             parityGrouping(),
		ThresholdAmountMinor: 10000,
	})
	require.NoError(t, err)
	require.NotNil(t, enf)

	return enf
}

func TestAuthorizationDecisionsMatchBaseline(t *testing.T) {
	t.Parallel()

	fixture := loadParityFixture(t)
	assert.Equal(t, "github.com/casbin/casbin/v2", fixture.Module)
	assert.Equal(t, "v2.8.0", fixture.Version)
	assert.NotEmpty(t, fixture.GoSumH1)
	assert.Contains(t, fixture.LivePin, "github.com/casbin/casbin/v3 v3.11.0")

	enf := newParityEnforcer(t)

	type testCase struct {
		name          string
		sub           string
		dom           string
		obj           string
		act           string
		expectedAllow bool
	}

	testCases := make([]testCase, 0, len(fixture.Cases))
	for _, row := range fixture.Cases {
		testCases = append(testCases, testCase{
			name:          row.Name,
			sub:           row.Sub,
			dom:           row.Dom,
			obj:           row.Obj,
			act:           row.Act,
			expectedAllow: row.Allow,
		})
	}

	require.NotEmpty(t, testCases)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualAllow := parityDecision(t, enf, tc.sub, tc.dom, tc.obj, tc.act)
			assert.Equal(t, tc.expectedAllow, actualAllow)
		})
	}
}
