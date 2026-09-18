package command

import (
	"context"
	"strings"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// TenantStore is the consumer-owned tenant persistence boundary. The schema
// lands in E07-T10; this interface is the contract it implements. Reads used
// for provisioning decisions are strongly consistent.
type TenantStore interface {
	// Create stores a new tenant record and returns it with the database-assigned ID.
	// Strong write; fails on duplicate ID or name.
	Create(ctx context.Context, tenant entity.TenantData) (entity.TenantData, error)
	// FindByID returns one tenant by ID. Strong read.
	FindByID(ctx context.Context, id valueobject.TenantID) (entity.TenantData, error)
	// ListNames returns taken tenant names for duplicate detection. Strong read.
	ListNames(ctx context.Context) ([]string, error)
	// ListAliases returns taken tenant aliases for slug derivation. Strong read.
	ListAliases(ctx context.Context) ([]string, error)
	// UpdateSettings replaces settings guarded by the expected version.
	// Fails on version mismatch. Strong write.
	UpdateSettings(ctx context.Context, tenant entity.TenantData, expectedVersion int64) error
	// ListTenants returns all tenants (platform-scoped, edge-gated). Strong read.
	ListTenants(ctx context.Context) ([]entity.TenantData, error)
}

// TenantServiceParams carries dependencies for TenantService.
type TenantServiceParams struct {
	UoW           port.UnitOfWork
	Tenants       TenantStore
	Clock         port.Clock
	IDs           port.IDGenerator
	Authz         port.Authorizer
	Regions       []string
	DefaultAssets []valueobject.AssetCode
}

// TenantService backs tenant provisioning over the onboarding service.
// Regions and default assets are operator-supplied authority (config); the
// domain never reads global state.
type TenantService struct {
	uow           port.UnitOfWork
	tenants       TenantStore
	clock         port.Clock
	ids           port.IDGenerator
	authz         port.Authorizer
	regions       []string
	defaultAssets []valueobject.AssetCode
}

var _ port.TenantCommandUseCases = (*TenantService)(nil)

// NewTenantService constructs a TenantService with the supplied dependencies.
func NewTenantService(params TenantServiceParams) *TenantService {
	return &TenantService{
		uow:           params.UoW,
		tenants:       params.Tenants,
		clock:         params.Clock,
		ids:           params.IDs,
		authz:         params.Authz,
		regions:       params.Regions,
		defaultAssets: params.DefaultAssets,
	}
}

const platformTenantID = valueobject.TenantID("00000000-0000-0000-0000-000000000000")

// ProvisionTenant creates a tenant with its default chart atomically. Strong write.
func (s *TenantService) ProvisionTenant(ctx context.Context, req port.ProvisionTenantRequest) (port.TenantResult, error) {
	if err := validateProvisionEnvelope(req); err != nil {
		return port.TenantResult{}, err
	}
	eventID := s.ids.NewID()
	now := s.clock.Now().UTC()
	settingsJSON, err := jsonparser.Marshal(req.Settings)
	if err != nil {
		return port.TenantResult{}, err
	}
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, req.Name, req.Region, string(settingsJSON)),
		TenantID:    platformTenantID,
	}
	return s.runTenantCommand(ctx, port.Subject{ID: req.Actor}, "tenant.provision", "platform/tenants", rec,
		func(ctx context.Context, tx port.Tx) (entity.TenantData, *port.OutboxFact, error) {
			existing, err := s.tenants.ListNames(ctx)
			if err != nil {
				return entity.TenantData{}, nil, err
			}
			takenAliases, err := s.tenants.ListAliases(ctx)
			if err != nil {
				return entity.TenantData{}, nil, err
			}
			plan, err := service.ValidateOnboarding(service.OnboardingRequest{
				TenantID:        "",
				LedgerID:        "",
				Name:            req.Name,
				RequestedAlias:  req.Alias,
				Region:          req.Region,
				Settings:        req.Settings,
				Assets:          s.defaultAssets,
				ExistingNames:   existing,
				ExistingAliases: takenAliases,
				AllowedRegions:  s.regions,
				RequestedBy:     valueobject.UserID(req.Actor),
				EventID:         eventID,
				Now:             now,
			})
			if err != nil {
				return entity.TenantData{}, nil, err
			}
			stored, err := s.tenants.Create(ctx, plan.Tenant)
			if err != nil {
				return entity.TenantData{}, nil, err
			}
			eventPayload := plan.Event
			eventPayload.TenantID = stored.ID.String()
			payload, err := jsonparser.Marshal(eventPayload)
			if err != nil {
				return entity.TenantData{}, nil, err
			}
			return stored, &port.OutboxFact{
				TenantID:    stored.ID,
				EventType:   "tenant.created.v1",
				AggregateID: string(stored.ID),
				Payload:     payload,
				OccurredAt:  now,
			}, nil
		})
}

// UpdateTenantSettings mutates non-identity settings under optimistic locking. Strong write.
func (s *TenantService) UpdateTenantSettings(ctx context.Context, req port.UpdateTenantSettingsRequest) (port.TenantResult, error) {
	if err := validateSettingsEnvelope(req); err != nil {
		return port.TenantResult{}, err
	}
	settingsJSON, err := jsonparser.Marshal(req.Settings)
	if err != nil {
		return port.TenantResult{}, err
	}
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, string(req.TenantID), int64ToString(req.ExpectedVersion), string(settingsJSON)),
		TenantID:    req.TenantID,
	}
	return s.runTenantCommand(ctx, port.Subject{ID: req.Actor, TenantID: req.TenantID}, "tenant.update", "tenant/"+string(req.TenantID), rec,
		func(ctx context.Context, _ port.Tx) (entity.TenantData, *port.OutboxFact, error) {
			stored, err := s.tenants.FindByID(ctx, req.TenantID)
			if err != nil {
				return entity.TenantData{}, nil, err
			}
			updated := stored
			updated.Settings = req.Settings
			if err := updated.Validate(); err != nil {
				return entity.TenantData{}, nil, err
			}
			if err := s.tenants.UpdateSettings(ctx, updated, req.ExpectedVersion); err != nil {
				return entity.TenantData{}, nil, err
			}
			// No webhook: the catalog carries tenant.created only; the
			// update is queryable state.
			return updated, nil, nil
		})
}

// runTenantCommand authorizes, reserves idempotency inside one UnitOfWork,
// and either replays the stored response or builds the tenant change exactly
// once, staging the optional webhook fact with the writes. A nil fact
// (settings updates: no cataloged webhook) stages nothing.
func (s *TenantService) runTenantCommand(ctx context.Context, subject port.Subject, action, resource string, rec port.IdempotencyRecord,
	build func(ctx context.Context, tx port.Tx) (entity.TenantData, *port.OutboxFact, error),
) (port.TenantResult, error) {
	if err := RequireAuthz(ctx, s.authz, subject, action, resource); err != nil {
		return port.TenantResult{}, err
	}
	var result port.TenantResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeTenantResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		tenant, fact, err := build(ctx, tx)
		if err != nil {
			return err
		}
		result = port.TenantResult{Tenant: tenant, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if fact != nil {
			if err := tx.Outbox().Append(ctx, *fact); err != nil {
				return err
			}
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.TenantResult{}, err
	}
	return result, nil
}

// validateProvisionEnvelope checks the provision request envelope.
func validateProvisionEnvelope(req port.ProvisionTenantRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return entity.NewError("TENANT_NAME_REQUIRED", "tenant name is required")
	}
	if strings.TrimSpace(req.Alias) != "" {
		if err := entity.ValidateTenantAlias(strings.TrimSpace(req.Alias)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "tenant actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "tenant command requires an idempotency key")
	}
	return nil
}

// validateSettingsEnvelope checks the settings-update envelope.
func validateSettingsEnvelope(req port.UpdateTenantSettingsRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_ID_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "tenant actor is required")
	}
	if req.ExpectedVersion < 1 {
		return entity.NewError("TENANT_VERSION_INVALID", "expected version must be at least 1")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "tenant command requires an idempotency key")
	}
	return nil
}

// decodeTenantResult restores a replayed response; corrupt records fail loudly.
func decodeTenantResult(response []byte) (port.TenantResult, error) {
	var result port.TenantResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.TenantResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.Tenant.ID.String() == "" {
		return port.TenantResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
