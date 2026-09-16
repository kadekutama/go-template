package command

import (
	"context"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// Subscription is one scheduled report delivery: template + cron +
// destination. The cron expression is opaque to this service (E14
// interprets it); LastRun records the most recent delivery.
type Subscription struct {
	ID          string
	TenantID    valueobject.TenantID
	Template    string
	Cron        string
	Destination string
	LastRun     time.Time
	CreatedAt   time.Time
}

// SubscriptionStore is the consumer-owned subscription persistence boundary.
// The schema lands in E07-T10; this interface is the contract it implements.
type SubscriptionStore interface {
	// CreateSubscription persists one subscription. Strong write; fails on duplicate ID.
	CreateSubscription(ctx context.Context, sub Subscription) error
	// FindSubscription returns one subscription by tenant + ID. Strong read.
	FindSubscription(ctx context.Context, tenant valueobject.TenantID, id string) (Subscription, error)
	// UpdateSubscription replaces one subscription. Strong write.
	UpdateSubscription(ctx context.Context, sub Subscription) error
	// DeleteSubscription removes one subscription. Strong write; idempotent.
	DeleteSubscription(ctx context.Context, tenant valueobject.TenantID, id string) error
	// ListDueSubscriptions returns subscriptions due at or before now. Strong read.
	ListDueSubscriptions(ctx context.Context, tenant valueobject.TenantID, now time.Time) ([]Subscription, error)
}

// ReportGenerator is the consumer-owned generation boundary used for
// scheduled delivery: the narrowest contract the subscription worker needs.
type ReportGenerator interface {
	GenerateReport(ctx context.Context, req port.ReportRequest) (port.ReportResult, error)
}

// SubscriptionServiceParams encapsulates dependencies for SubscriptionService.
type SubscriptionServiceParams struct {
	UoW           port.UnitOfWork
	Subscriptions SubscriptionStore
	Reports       ReportGenerator
	Clock         port.Clock
	IDs           port.IDGenerator
	Authz         port.Authorizer
}

// SubscriptionService manages scheduled report deliveries. Workers (E14)
// poll DueSubscriptions and run DeliverSubscription per due row with a
// per-run idempotency key; delivery generates through the report generator
// so every delivery carries a signed URL plus a generated fact.
type SubscriptionService struct {
	uow           port.UnitOfWork
	subscriptions SubscriptionStore
	reports       ReportGenerator
	clock         port.Clock
	ids           port.IDGenerator
	authz         port.Authorizer
}

// NewSubscriptionService creates an encapsulated SubscriptionService with validated dependencies.
func NewSubscriptionService(params SubscriptionServiceParams) *SubscriptionService {
	return &SubscriptionService{
		uow:           params.UoW,
		subscriptions: params.Subscriptions,
		reports:       params.Reports,
		clock:         params.Clock,
		ids:           params.IDs,
		authz:         params.Authz,
	}
}

// Subscribe persists one delivery subscription. Strong write.
func (s *SubscriptionService) Subscribe(ctx context.Context, tenant valueobject.TenantID, template, cron, destination, actor, key string) (Subscription, error) {
	if err := validateSubscriptionEnvelope(tenant, template, cron, destination, actor, key); err != nil {
		return Subscription{}, err
	}
	subject := port.Subject{ID: actor, TenantID: tenant}
	if err := RequireAuthz(ctx, s.authz, subject, "report.subscribe", "tenant/"+string(tenant)); err != nil {
		return Subscription{}, err
	}
	sub := Subscription{
		ID: s.ids.NewID(), TenantID: tenant, Template: template,
		Cron: cron, Destination: destination, CreatedAt: s.clock.Now().UTC(),
	}
	rec := port.IdempotencyRecord{
		Key:         key,
		Fingerprint: Fingerprint(key, string(tenant), template, cron, destination),
		TenantID:    tenant,
	}
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeSubscription(outcome.Response)
			if err != nil {
				return err
			}
			sub = decoded
			return nil
		}
		if err := s.subscriptions.CreateSubscription(ctx, sub); err != nil {
			return err
		}
		encoded, err := jsonparser.Marshal(sub)
		if err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return Subscription{}, err
	}
	return sub, nil
}

// Unsubscribe removes one subscription. Strong write; idempotent.
func (s *SubscriptionService) Unsubscribe(ctx context.Context, tenant valueobject.TenantID, id, actor string) error {
	subject := port.Subject{ID: actor, TenantID: tenant}
	if err := RequireAuthz(ctx, s.authz, subject, "report.subscribe", "tenant/"+string(tenant)); err != nil {
		return err
	}
	return s.subscriptions.DeleteSubscription(ctx, tenant, id)
}

// DueSubscriptions lists subscriptions due at or before now. Strong read;
// called by the report worker (E14), never by the edge directly.
func (s *SubscriptionService) DueSubscriptions(ctx context.Context, tenant valueobject.TenantID, now time.Time) ([]Subscription, error) {
	return s.subscriptions.ListDueSubscriptions(ctx, tenant, now)
}

// DeliverSubscription generates one delivery run for a due subscription with
// a per-run idempotency key and records the run. Strong write.
func (s *SubscriptionService) DeliverSubscription(ctx context.Context, tenant valueobject.TenantID, id, runKey, actor string, parameters map[string]string, now time.Time) (port.ReportResult, error) {
	sub, err := s.subscriptions.FindSubscription(ctx, tenant, id)
	if err != nil {
		return port.ReportResult{}, err
	}
	subject := port.Subject{ID: actor, TenantID: tenant}
	if err := RequireAuthz(ctx, s.authz, subject, "report.generate", "tenant/"+string(tenant)); err != nil {
		return port.ReportResult{}, err
	}
	result, err := s.reports.GenerateReport(ctx, port.ReportRequest{
		TenantID: sub.TenantID, Template: sub.Template, Parameters: parameters,
		Destination: sub.Destination, IdempotencyKey: runKey, Actor: actor,
	})
	if err != nil {
		return port.ReportResult{}, err
	}
	sub.LastRun = now.UTC()
	if err := s.subscriptions.UpdateSubscription(ctx, sub); err != nil {
		return port.ReportResult{}, err
	}
	return result, nil
}

// validateSubscriptionEnvelope checks the subscription envelope including
// template membership.
func validateSubscriptionEnvelope(tenant valueobject.TenantID, template, cron, destination, actor, key string) error {
	if strings.TrimSpace(tenant.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if !isReportTemplate(template) {
		return entity.NewError("REPORT_TEMPLATE_UNKNOWN", "report template is unknown")
	}
	if strings.TrimSpace(cron) == "" {
		return entity.NewError("SUBSCRIPTION_CRON_REQUIRED", "subscription cron is required")
	}
	if strings.TrimSpace(destination) == "" {
		return entity.NewError("SUBSCRIPTION_DESTINATION_REQUIRED", "subscription destination is required")
	}
	if strings.TrimSpace(actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "subscription actor is required")
	}
	if strings.TrimSpace(key) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "subscription requires an idempotency key")
	}
	return nil
}

// decodeSubscription restores a replayed subscription; corrupt records fail loudly.
func decodeSubscription(response []byte) (Subscription, error) {
	var sub Subscription
	if err := jsonparser.Unmarshal(response, &sub); err != nil {
		return Subscription{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if sub.ID == "" {
		return Subscription{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return sub, nil
}
