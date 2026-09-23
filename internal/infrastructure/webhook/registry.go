package webhook

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// Endpoint is one tenant webhook subscription. Secret is held by the
// adapter and never exposed through port shapes or logs. Field rules are
// declared with validator tags and enforced by Validate (joined violations);
// Normalized trims user input first so blank-only values fail required.
type Endpoint struct {
	ID     string   `validate:"required"`
	Tenant string   `validate:"required"`
	URL    string   `validate:"required,http_url"`
	Events []string `validate:"required,min=1,dive,required"`
	Secret string   `validate:"required"`
	Active bool     `validate:"-"`
}

// Normalized returns a copy with string fields and event entries trimmed, so
// whitespace-only input cannot sneak past the required rules.
func (e Endpoint) Normalized() Endpoint {
	events := make([]string, 0, len(e.Events))
	for _, event := range e.Events {
		events = append(events, strings.TrimSpace(event))
	}

	e.ID = strings.TrimSpace(e.ID)
	e.Tenant = strings.TrimSpace(e.Tenant)
	e.URL = strings.TrimSpace(e.URL)
	e.Secret = strings.TrimSpace(e.Secret)
	e.Events = events

	return e
}

// Validate joins every tag violation into one error (shared format).
func (e Endpoint) Validate() error {
	return validate.Struct("webhook", "endpoint", e)
}

// RegistryParams carries constructor dependencies (Parameter Object pattern).
type RegistryParams struct {
}

// Registry is the endpoint store backing the webhook-mgmt API. It is an
// in-memory reference adapter: state is lost on restart, so production MUST
// back it with durable storage (webhook-subscription table owned by E07 and
// wired by E11's management endpoints). It remains here because endpoint
// configuration is low-churn and the DB repository does not exist yet; the
// doc comment is the explicit deferral marker.
type Registry struct {
	mu        sync.RWMutex
	endpoints map[string]Endpoint
}

// NewRegistry builds the store.
func NewRegistry(_ RegistryParams) *Registry {
	return &Registry{endpoints: make(map[string]Endpoint)}
}

// Upsert creates or replaces an endpoint: input is normalized, then every
// required/format rule is checked before the write.
func (r *Registry) Upsert(_ context.Context, endpoint Endpoint) error {
	if r == nil {
		return fmt.Errorf("webhook: registry is not initialized")
	}

	endpoint = endpoint.Normalized()

	if err := endpoint.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	endpoint.Active = true
	r.endpoints[endpoint.ID] = endpoint

	return nil
}

// Remove deactivates one endpoint. Missing IDs succeed.
func (r *Registry) Remove(_ context.Context, id string) error {
	if r == nil {
		return fmt.Errorf("webhook: registry is not initialized")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.endpoints, strings.TrimSpace(id))

	return nil
}

// Find returns active endpoints for tenant+event (isolation: tenant scope
// is always applied; cross-tenant reads never match).
func (r *Registry) Find(_ context.Context, tenant, event string) []Endpoint {
	if r == nil {
		return nil
	}

	trimmedTenant := strings.TrimSpace(tenant)
	trimmedEvent := strings.TrimSpace(event)

	if trimmedTenant == "" || trimmedEvent == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []Endpoint
	for _, endpoint := range r.endpoints {
		if !endpoint.Active {
			continue
		}

		if strings.TrimSpace(endpoint.Tenant) != trimmedTenant {
			continue
		}

		for _, candidate := range endpoint.Events {
			if strings.TrimSpace(candidate) == trimmedEvent {
				out = append(out, endpoint)
				break
			}
		}
	}

	return out
}
