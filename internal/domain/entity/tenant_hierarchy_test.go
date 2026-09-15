package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestTenantLinkValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		link          entity.TenantLink
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid link",
			link:          entity.TenantLink{ParentID: "t-parent", ChildID: "t-child"},
			expectedError: nil,
		},
		{
			name:          "missing parent",
			link:          entity.TenantLink{ParentID: "", ChildID: "t-child"},
			expectedError: entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy link requires parent and child ids"),
		},
		{
			name:          "whitespace parent",
			link:          entity.TenantLink{ParentID: "   ", ChildID: "t-child"},
			expectedError: entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy link requires parent and child ids"),
		},
		{
			name:          "missing child",
			link:          entity.TenantLink{ParentID: "t-parent", ChildID: ""},
			expectedError: entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy link requires parent and child ids"),
		},
		{
			name:          "whitespace child",
			link:          entity.TenantLink{ParentID: "t-parent", ChildID: "  "},
			expectedError: entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy link requires parent and child ids"),
		},
		{
			name:          "self parent rejected",
			link:          entity.TenantLink{ParentID: "t-1", ChildID: "t-1"},
			expectedError: entity.NewError("HIERARCHY_SELF_PARENT", "tenant cannot parent to itself"),
		},
		{
			name:          "self parent with whitespace padding rejected",
			link:          entity.TenantLink{ParentID: " t-1 ", ChildID: "t-1"},
			expectedError: entity.NewError("HIERARCHY_SELF_PARENT", "tenant cannot parent to itself"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.link.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateHierarchy(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		links         []entity.TenantLink
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "empty hierarchy passes",
			links:         nil,
			expectedError: nil,
		},
		{
			name: "acyclic chain passes",
			links: []entity.TenantLink{
				{ParentID: "t-a", ChildID: "t-b"},
				{ParentID: "t-b", ChildID: "t-c"},
			},
			expectedError: nil,
		},
		{
			name: "duplicate links handled cleanly",
			links: []entity.TenantLink{
				{ParentID: "t-a", ChildID: "t-b"},
				{ParentID: "t-a", ChildID: "t-b"},
			},
			expectedError: nil,
		},
		{
			name: "disjoint acyclic trees pass",
			links: []entity.TenantLink{
				{ParentID: "t-a", ChildID: "t-b"},
				{ParentID: "t-x", ChildID: "t-y"},
			},
			expectedError: nil,
		},
		{
			name: "branching hierarchy passes",
			links: []entity.TenantLink{
				{ParentID: "t-root", ChildID: "t-child1"},
				{ParentID: "t-root", ChildID: "t-child2"},
				{ParentID: "t-child1", ChildID: "t-sub1"},
				{ParentID: "t-child2", ChildID: "t-sub2"},
			},
			expectedError: nil,
		},
		{
			name: "diamond DAG passes within limit",
			links: []entity.TenantLink{
				{ParentID: "t-root", ChildID: "t-mid1"},
				{ParentID: "t-root", ChildID: "t-mid2"},
				{ParentID: "t-mid1", ChildID: "t-leaf"},
				{ParentID: "t-mid2", ChildID: "t-leaf"},
			},
			expectedError: nil,
		},
		{
			name: "cycle rejected",
			links: []entity.TenantLink{
				{ParentID: "t-a", ChildID: "t-b"},
				{ParentID: "t-b", ChildID: "t-c"},
				{ParentID: "t-c", ChildID: "t-a"},
			},
			expectedError: entity.NewError("HIERARCHY_CYCLE", "tenant hierarchy must be acyclic"),
		},
		{
			name: "cycle in subcomponent rejected",
			links: []entity.TenantLink{
				{ParentID: "t-a", ChildID: "t-b"},
				{ParentID: "t-b", ChildID: "t-c"},
				{ParentID: "t-c", ChildID: "t-b"},
			},
			expectedError: entity.NewError("HIERARCHY_CYCLE", "tenant hierarchy must be acyclic"),
		},
		{
			name: "self parent rejected",
			links: []entity.TenantLink{
				{ParentID: "t-a", ChildID: "t-a"},
			},
			expectedError: entity.NewError("HIERARCHY_SELF_PARENT", "tenant cannot parent to itself"),
		},
		{
			name: "chain at depth limit passes",
			links: []entity.TenantLink{
				{ParentID: "t-1", ChildID: "t-2"},
				{ParentID: "t-2", ChildID: "t-3"},
				{ParentID: "t-3", ChildID: "t-4"},
				{ParentID: "t-4", ChildID: "t-5"},
			},
			expectedError: nil,
		},
		{
			name: "chain one past depth limit rejected",
			links: []entity.TenantLink{
				{ParentID: "t-1", ChildID: "t-2"},
				{ParentID: "t-2", ChildID: "t-3"},
				{ParentID: "t-3", ChildID: "t-4"},
				{ParentID: "t-4", ChildID: "t-5"},
				{ParentID: "t-5", ChildID: "t-6"},
			},
			expectedError: entity.NewError("HIERARCHY_DEPTH_EXCEEDED", "tenant hierarchy exceeds depth limit"),
		},
		{
			name: "deep chain beyond limit rejected",
			links: []entity.TenantLink{
				{ParentID: "t-1", ChildID: "t-2"},
				{ParentID: "t-2", ChildID: "t-3"},
				{ParentID: "t-3", ChildID: "t-4"},
				{ParentID: "t-4", ChildID: "t-5"},
				{ParentID: "t-5", ChildID: "t-6"},
				{ParentID: "t-6", ChildID: "t-7"},
			},
			expectedError: entity.NewError("HIERARCHY_DEPTH_EXCEEDED", "tenant hierarchy exceeds depth limit"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := entity.ValidateHierarchy(tc.links)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
