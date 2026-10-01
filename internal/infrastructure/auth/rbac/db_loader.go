package rbac

import (
	"fmt"

	"gorm.io/gorm"
)

// CasbinRuleModel maps casbin policy storage in PostgreSQL ("casbin_rules" table).
//
// ============================================================================
// WHY V0 THROUGH V5? (Casbin Column Architecture)
// ============================================================================
// In Casbin's PERM (Policy, Effect, Request, Matcher) metamodel architecture, the
// engine supports arbitrary authorization models (ACL, RBAC, ABAC, RESTful, etc.)
// with user-defined tuple shapes and variable number of arguments.
// To avoid requiring DDL schema migrations every time an authorization model is
// changed or extended, Casbin standardized on a universal persistence schema:
// a discriminator column (`ptype`) followed by six generic positional string columns:
// `v0`, `v1`, `v2`, `v3`, `v4`, and `v5`.
//
// ============================================================================
// HOW V0..V5 MAP TO OUR MULTI-TENANT RBAC MODEL (see model.conf)
// ============================================================================
// In this repository, we enforce Multi-Tenant Role-Based Access Control (RBAC).
// The semantic meaning of each column V0..V5 depends entirely on the row's `ptype`:
//
// +-------+-------+--------------------+-----------------------------------------------+
// | Ptype | Col   | Domain Meaning     | Example Value                                 |
// +-------+-------+--------------------+-----------------------------------------------+
// | "p"   | V0    | Subject / Role     | "role:finance", "role:admin", "user:usr_123"   |
// | (Rule)| V1    | Domain (Tenant ID) | "01950000-0000-7000-8000-000000000001"        |
// |       | V2    | Object (Resource)  | "ledger/posting", "payout", "reports"         |
// |       | V3    | Action             | "create", "read", "update", "approve"         |
// |       | V4    | Effect             | "allow" or "deny" (defaults to "allow")       |
// |       | V5    | Extra / Conditions | "" (reserved for future conditional ABAC)     |
// +-------+-------+--------------------+-----------------------------------------------+
// | "g"   | V0    | User ID            | "usr_01950000000070008000000000000042"        |
// | (Role)| V1    | Role Name Assigned | "role:finance"                                |
// | Map)  | V2    | Domain (Tenant ID) | "01950000-0000-7000-8000-000000000001"        |
// |       | V3..V5| Unused             | "" (Casbin `g = _, _, _` has only 3 arguments)|
// +-------+-------+--------------------+-----------------------------------------------+
//
// ============================================================================
// CONCRETE SQL EXAMPLES:
// ============================================================================
//
//  1. Policy Rule ("p"): Role 'role:finance' can 'create' on 'ledger/posting' in 'tenant-1':
//     INSERT INTO casbin_rules (ptype, v0, v1, v2, v3, v4)
//     VALUES ('p', 'role:finance', 'tenant-1', 'ledger/posting', 'create', 'allow');
//
//  2. Grouping Rule ("g"): Assign user 'usr_42' to 'role:finance' within 'tenant-1':
//     INSERT INTO casbin_rules (ptype, v0, v1, v2)
//     VALUES ('g', 'usr_42', 'role:finance', 'tenant-1');
type CasbinRuleModel struct {
	ID    uint   `gorm:"primaryKey;autoIncrement"`
	Ptype string `gorm:"size:100;index"` // "p" (policy permission rule) or "g" (user-role grouping assignment)

	// V0 carries:
	// - For "p": Subject / Role (e.g. "role:admin", "role:finance", "usr_123")
	// - For "g": User ID being granted a role (e.g. "usr_123")
	V0 string `gorm:"size:100"`

	// V1 carries:
	// - For "p": Domain / Multi-tenant scope (Tenant ID e.g. "tenant-1")
	// - For "g": Role Name assigned to the user (e.g. "role:finance")
	V1 string `gorm:"size:100"`

	// V2 carries:
	// - For "p": Object / Target resource path (e.g. "ledger/posting", "payout")
	// - For "g": Domain / Tenant ID where this role assignment applies (e.g. "tenant-1")
	V2 string `gorm:"size:100"`

	// V3 carries:
	// - For "p": Action permitted or forbidden (e.g. "create", "read", "update")
	// - For "g": Unused (3-argument grouping `g = user, role, domain`)
	V3 string `gorm:"size:100"`

	// V4 carries:
	// - For "p": Effect: "allow" or "deny" (defaults to "allow" if omitted)
	// - For "g": Unused
	V4 string `gorm:"size:100"`

	// V5 carries:
	// - For "p": Optional extra condition / ABAC constraint (unused in standard RBAC)
	// - For "g": Unused
	V5 string `gorm:"size:100"`
}

// TableName pins the model to the standard casbin table name.
func (CasbinRuleModel) TableName() string { return "casbin_rules" }

// DBPolicyLoader loads Casbin policies from PostgreSQL.
type DBPolicyLoader struct {
	db *gorm.DB
}

// Compile-time interface conformance.
var _ PolicyLoader = (*DBPolicyLoader)(nil)

// NewDBPolicyLoader builds a database policy loader.
func NewDBPolicyLoader(db *gorm.DB) (*DBPolicyLoader, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: db required", ErrPolicyInvalid)
	}

	return &DBPolicyLoader{db: db}, nil
}

// Load loads all policy rules (ptype='p') and grouping rules (ptype='g') from DB.
func (l *DBPolicyLoader) Load() ([]PolicyRule, []GroupingRule, error) {
	if l == nil || l.db == nil {
		return nil, nil, fmt.Errorf("%w: db required", ErrPolicyInvalid)
	}

	var rules []CasbinRuleModel
	if err := l.db.Find(&rules).Error; err != nil {
		return nil, nil, fmt.Errorf("%w: query policies: %s", ErrPolicyInvalid, err.Error())
	}

	var policies []PolicyRule
	var grouping []GroupingRule

	for _, r := range rules {
		switch r.Ptype {
		case "p":
			// Policy permission rule: p = sub, dom, obj, act, eft
			// V0 = Subject (role name or direct user ID)
			// V1 = Domain (tenant ID)
			// V2 = Object (resource path)
			// V3 = Action (operation)
			// V4 = Effect (allow / deny; defaults to allow)
			// V5 = Unused
			effect := r.V4
			if effect == "" {
				effect = "allow"
			}

			policies = append(policies, PolicyRule{
				Sub:    r.V0,
				Dom:    r.V1,
				Obj:    r.V2,
				Act:    r.V3,
				Effect: effect,
			})
		case "g":
			// User-role grouping rule: g = user, role, domain
			// V0 = User ID
			// V1 = Role Name assigned
			// V2 = Domain (tenant ID)
			// V3..V5 = Unused
			grouping = append(grouping, GroupingRule{
				User: r.V0,
				Role: r.V1,
				Dom:  r.V2,
			})
		}
	}

	return policies, grouping, nil
}
