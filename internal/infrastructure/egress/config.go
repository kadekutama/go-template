package egress

import (
	"errors"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/kadekutama/go-template/internal/domain/entity"
	domainservice "github.com/kadekutama/go-template/internal/domain/service"
)

// Policy is one provider-operation outbound quota. Every field is
// required; there is no default or unlimited quota. Budget counts token
// bucket admissions per Window; MaxWait bounds one Wait call.
type Policy struct {
	Provider  string
	Operation string
	Budget    int64
	Window    time.Duration
	MaxWait   time.Duration
}

// Validate rejects missing, malformed, or non-positive quotas before any
// provider call can run unmetered.
func (p Policy) Validate() error {
	if p.Provider == "" || p.Operation == "" {
		return entity.NewError(CodeInvalidPolicy, "egress: provider and operation are required")
	}

	if _, err := domainservice.BuildRateLimitKey(p.Provider, p.Operation); err != nil {
		return errors.Join(entity.NewError(CodeInvalidPolicy, "egress: provider/operation are not safe key components"), err)
	}

	if p.Budget <= 0 {
		return entity.NewError(CodeInvalidPolicy, "egress: budget must be positive")
	}

	if p.Window <= 0 {
		return entity.NewError(CodeInvalidPolicy, "egress: window must be positive")
	}

	if p.MaxWait <= 0 {
		return entity.NewError(CodeInvalidPolicy, "egress: max wait must be positive")
	}

	return nil
}

// rawPolicy mirrors the on-disk shape with duration strings.
type rawPolicy struct {
	Provider  string `yaml:"provider"`
	Operation string `yaml:"operation"`
	Budget    int64  `yaml:"budget"`
	Window    string `yaml:"window"`
	MaxWait   string `yaml:"max_wait"`
}

// rawFile is the top-level policies document. Per-provider files live under
// config/egress/ (owned by E10); this task ships only the format.
type rawFile struct {
	Policies []rawPolicy `yaml:"policies"`
}

// ParsePolicies loads and validates a policies document. Every configured
// provider operation needs a positive budget/window/max-wait; a missing or
// invalid policy is a typed error, never an implicit unlimited quota.
func ParsePolicies(data []byte) ([]Policy, error) {
	var file rawFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, errors.Join(entity.NewError(CodeInvalidPolicy, "egress: policies do not parse"), err)
	}

	if len(file.Policies) == 0 {
		return nil, entity.NewError(CodeInvalidPolicy, "egress: at least one policy is required")
	}

	policies := make([]Policy, 0, len(file.Policies))
	for i, raw := range file.Policies {
		window, err := time.ParseDuration(raw.Window)
		if err != nil {
			return nil, errors.Join(entity.Errorf(CodeInvalidPolicy, "egress: policy %d has an invalid window", i), err)
		}

		maxWait, err := time.ParseDuration(raw.MaxWait)
		if err != nil {
			return nil, errors.Join(entity.Errorf(CodeInvalidPolicy, "egress: policy %d has an invalid max wait", i), err)
		}

		policy := Policy{
			Provider:  raw.Provider,
			Operation: raw.Operation,
			Budget:    raw.Budget,
			Window:    window,
			MaxWait:   maxWait,
		}
		if err := policy.Validate(); err != nil {
			return nil, err
		}

		policies = append(policies, policy)
	}

	return policies, nil
}
