package rbac

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

//go:embed model.conf
var defaultModel string

// PolicyRule is one grant line: role/user, tenant domain, object, action.
type PolicyRule struct {
	Sub    string `validate:"required,max=256"`
	Dom    string `validate:"required,max=256"`
	Obj    string `validate:"required,max=256"`
	Act    string `validate:"required,max=256"`
	Effect string `validate:"required,oneof=allow deny"`
}

// GroupingRule assigns a user to a role within a tenant domain.
type GroupingRule struct {
	User string `validate:"required,max=256"`
	Role string `validate:"required,max=256"`
	Dom  string `validate:"required,max=256"`
}

// Decision is one audited authorization outcome.
type Decision struct {
	Subject  string
	Tenant   string
	Action   string
	Resource string
	Allow    bool
	Reason   string
}

// Attributes carries ABAC inputs for threshold approvals.
type Attributes struct {
	AmountMinor int64
	Approver    string
	Resolver    string
}

// RBACParams carries constructor dependencies (Parameter Object pattern).
type RBACParams struct {
	ModelText            string
	Loader               PolicyLoader
	Policies             []PolicyRule
	Grouping             []GroupingRule
	ThresholdAmountMinor int64
	AuditHook            func(Decision)
}

// Enforcer wraps a synchronized Casbin enforcer with tenant scoping,
// threshold ABAC, and audit hooks. Concurrency control belongs to
// casbin.SyncedEnforcer (the library's own synchronized type): this wrapper
// holds no mutex. Reload swaps policy content through the synchronized API;
// readers observing a mid-reload policy fail closed (default deny).
type Enforcer struct {
	enforcer  *casbin.SyncedEnforcer
	threshold int64
	audit     func(Decision)
}

// Compile-time port conformance.
var _ appport.Authorizer = (*Enforcer)(nil)

// NewEnforcer builds the RBAC adapter with embedded model fallback.
// ThresholdAmountMinor must be positive: a zero threshold would silently
// disable the approver-distinct-from-resolver rule (CR-005), so
// misconfiguration fails here instead of running unguarded.
func NewEnforcer(params RBACParams) (*Enforcer, error) {
	if params.ThresholdAmountMinor <= 0 {
		return nil, fmt.Errorf("%w: threshold amount must be positive", ErrPolicyInvalid)
	}

	modelText := params.ModelText
	if modelText == "" {
		modelText = defaultModel
	}

	casbinModel, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, fmt.Errorf("%w: model: %s", ErrPolicyInvalid, err.Error())
	}

	enf, err := casbin.NewSyncedEnforcer(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("%w: enforcer: %s", ErrPolicyInvalid, err.Error())
	}

	policies := params.Policies
	grouping := params.Grouping
	if len(policies) == 0 && len(grouping) == 0 && params.Loader != nil {
		var loadErr error
		policies, grouping, loadErr = params.Loader.Load()
		if loadErr != nil {
			return nil, fmt.Errorf("%w: loader: %s", ErrPolicyInvalid, loadErr.Error())
		}
	}

	if err := addPolicies(enf, policies, grouping); err != nil {
		return nil, err
	}

	return &Enforcer{
		enforcer:  enf,
		threshold: params.ThresholdAmountMinor,
		audit:     params.AuditHook,
	}, nil
}

// addPolicies installs policy + grouping rules onto the enforcer after
// validating every rule against its struct tags (CR-006 follow-up: malformed
// rules fail here instead of sneaking past partial manual checks).
func addPolicies(enf *casbin.SyncedEnforcer, policies []PolicyRule, grouping []GroupingRule) error {
	if err := validateRules(policies, grouping); err != nil {
		return err
	}

	for _, rule := range policies {
		if _, err := enf.AddPolicy(rule.Sub, rule.Dom, rule.Obj, rule.Act, effectOf(rule)); err != nil {
			return fmt.Errorf("%w: add policy", ErrPolicyInvalid)
		}
	}

	for _, grp := range grouping {
		if _, err := enf.AddGroupingPolicy(grp.User, grp.Role, grp.Dom); err != nil {
			return fmt.Errorf("%w: add grouping", ErrPolicyInvalid)
		}
	}

	return nil
}

// validateRules enforces the validate tags on every policy and grouping rule.
func validateRules(policies []PolicyRule, grouping []GroupingRule) error {
	for i := range policies {
		if err := validate.Struct("rbac", "policy", policies[i]); err != nil {
			return fmt.Errorf("%w: policy %d: %s", ErrPolicyInvalid, i, err.Error())
		}
	}

	for i := range grouping {
		if err := validate.Struct("rbac", "grouping", grouping[i]); err != nil {
			return fmt.Errorf("%w: grouping %d: %s", ErrPolicyInvalid, i, err.Error())
		}
	}

	return nil
}

// effectOf defaults empty effects to allow.
func effectOf(rule PolicyRule) string {
	if rule.Effect == "" {
		return "allow"
	}

	return rule.Effect
}

// Authorize permits subject to perform action on resource or returns an error.
func (e *Enforcer) Authorize(ctx context.Context, subject appport.Subject, action, resource string) error {
	return e.AuthorizeWithAttrs(ctx, subject, action, resource, Attributes{})
}

// AuthorizeWithAttrs enforces the threshold approver rule on top of Casbin.
func (e *Enforcer) AuthorizeWithAttrs(ctx context.Context, subject appport.Subject, action, resource string, attrs Attributes) error {
	if e == nil || e.enforcer == nil {
		return ErrNotInit
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rbac: authorize: %w", err)
	}

	allow, reason := e.evaluate(subject, action, resource, attrs)
	e.emit(Decision{
		Subject:  subject.ID,
		Tenant:   string(subject.TenantID),
		Action:   action,
		Resource: resource,
		Allow:    allow,
		Reason:   reason,
	})

	if !allow {
		return fmt.Errorf("%w: %s cannot %s %s", ErrForbidden, subject.ID, action, resource)
	}

	return nil
}

// evaluate runs Casbin plus the threshold ABAC rule. Enforce is internally
// synchronized by SyncedEnforcer; no wrapper lock is taken.
func (e *Enforcer) evaluate(subject appport.Subject, action, resource string, attrs Attributes) (bool, string) {
	if subject.ID == "" || subject.TenantID == "" || action == "" || resource == "" {
		return false, "missing fields"
	}

	if attrs.AmountMinor > e.threshold && e.threshold > 0 {
		if attrs.Approver == "" || attrs.Approver == attrs.Resolver {
			return false, "approver must differ from resolver above threshold"
		}
	}

	allowed, err := e.enforcer.Enforce(subject.ID, string(subject.TenantID), resource, action)
	if err != nil {
		return false, "enforce error"
	}

	if !allowed {
		return false, "policy deny"
	}

	return true, "policy allow"
}

// emit calls the audit hook.
func (e *Enforcer) emit(decision Decision) {
	if e == nil || e.audit == nil {
		return
	}

	e.audit(decision)
}

// Reload replaces all policies without restart through the synchronized API.
// A reader racing Reload observes a partially replaced policy and fails
// closed (default deny) rather than tearing. The incoming set is validated
// before anything is cleared, so a malformed reload leaves current policy
// intact instead of half-applied.
func (e *Enforcer) Reload(policies []PolicyRule, grouping []GroupingRule) error {
	if e == nil || e.enforcer == nil {
		return ErrNotInit
	}

	if err := validateRules(policies, grouping); err != nil {
		return err
	}

	if _, err := e.enforcer.RemoveFilteredPolicy(0, "", "", "", "", ""); err != nil {
		return fmt.Errorf("%w: clear", ErrPolicyInvalid)
	}

	if _, err := e.enforcer.RemoveFilteredGroupingPolicy(0, "", "", ""); err != nil {
		return fmt.Errorf("%w: clear grouping", ErrPolicyInvalid)
	}

	for _, rule := range policies {
		if _, err := e.enforcer.AddPolicy(rule.Sub, rule.Dom, rule.Obj, rule.Act, effectOf(rule)); err != nil {
			return fmt.Errorf("%w: add policy", ErrPolicyInvalid)
		}
	}

	for _, grp := range grouping {
		if _, err := e.enforcer.AddGroupingPolicy(grp.User, grp.Role, grp.Dom); err != nil {
			return fmt.Errorf("%w: add grouping", ErrPolicyInvalid)
		}
	}

	return nil
}

// ReloadFrom loads policies and groupings from loader and swaps them.
func (e *Enforcer) ReloadFrom(loader PolicyLoader) error {
	if e == nil || e.enforcer == nil {
		return ErrNotInit
	}

	if loader == nil {
		return fmt.Errorf("%w: loader required", ErrPolicyInvalid)
	}

	policies, grouping, err := loader.Load()
	if err != nil {
		return err
	}

	return e.Reload(policies, grouping)
}
