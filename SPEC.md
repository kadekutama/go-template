# Go Clean Architecture Template — System Specification

**Version:** 1.1.1  
**Status:** Planning — audited, correctness contract added  
**Last Updated:** 2026-09-13  
**Go Version:** 1.27.1 (current stable patch at audit date)  

---

## 1. Project Overview

### 1.1 Purpose
An AI-friendly specification targeting a production-ready Go template that uses
Clean Architecture + DDD + Specification-Driven Delivery (SDD) with multi-binary support (REST, gRPC, GraphQL,
Cron, Consumer). The current repository is planning-only; readiness is achieved
only after the acceptance gates contain implementation and measured evidence.

### 1.2 Core Principles
- **Clean Architecture**: Dependencies point inward: Interface → Application → Domain; Infrastructure implements inner-layer ports.
- **Domain-Driven Design**: Aggregates, Entities, Value Objects, Domain Events, Specifications
- **Specification-Driven Delivery**: Approved task packets, executable scenarios,
  durable evidence, and repository-visible handoffs (`tasks/SDD.md`)
- **Domain Specification Pattern**: Pure, composable executable business invariants
- **SOLID**: Small consumer-owned ports, substitutable adapters, and explicit responsibility boundaries (see `docs/development/go-conventions.md`)
- **CQRS**: Separate application command and query contracts; one database is acceptable, and queries never perform business side effects.
- **Dependency Injection**: fx (Uber) runtime DI with lifecycle management
- **Observability-First**: OpenTelemetry, structured logging, metrics, tracing
- **Security by Default**: OWASP Top 10:2025 control coverage, secrets management, encryption
- **Testability**: 100% dockerized dependencies, Testcontainers for integration tests

### 1.3 Target Use Cases (Reference Implementation: Fintech Ledger)
- Multi-tenant SaaS platforms (Notion/Linear)
- B2B E-commerce (Shopify Plus)
- **Fintech Ledger (Stripe/Plaid core)** - Primary reference
- Logistics Orchestration (Flexport)
- IoT Device Management
- Healthcare Interoperability (FHIR)
- Real-time Collaboration (Figma/Miro)

### 1.4 Normative Sources and Precedence

The reference implementation is a payment-platform operational ledger, not a
merchant's complete accounting product. The following precedence resolves
conflicts in older examples:

1. Accepted ADRs and the owner-approved `docs/ledger-core.md` — accounting, money safety,
   idempotency, ordering, and persistence correctness.
2. The approved task packet under `tasks/specs/` — bounded implementation contract.
3. Protocol schemas under `api/` once generated and accepted.
4. `SPEC.md` — platform architecture and technology decisions.
5. Other `docs/*.md` narrative examples.
6. Harness-specific profiles and prompts.

The full 2026-09-11 assessment and outstanding risks are in
`docs/repository-audit.md`. “Production-ready,” compliance, availability, and
performance statements are targets until their gates contain measured evidence.
The repository owner approved this precedence and the audited ADR-002/003/009
decisions on 2026-09-13; ADR-011 remains proposed pending benchmark evidence.

---

## 2. Technology Stack (Target pins; verified at bootstrap)

The versions below are reproducible target pins, not a promise that every
dependency is the newest release. E00/E01 resolve them in `go.mod` and the
compose manifests, run vulnerability checks, and record any upgrade in an ADR
or dependency-update PR. Go 1.27.1 is the current stable patch release on the
audit date; keep the `go` directive and CI toolchain aligned with it.

| Category | Technology | Version | Notes |
|----------|------------|---------|-------|
| **Language** | Go | 1.27.1 | Latest stable (released 2026-09-01) |
| **DI Framework** | fx (Uber) | v1.24.0 | Runtime DI with lifecycle |
| **HTTP Router** | Echo | v5.3.1 | High performance, middleware (v4 EOL 2026-12-31) |
| **gRPC** | grpc-go | v1.66.0 | Protobuf v1.34.0+ |
| **GraphQL** | gqlgen | v0.17.94 | Schema-first, codegen |
| **Database** | PostgreSQL | 18.6 | GORM v2 (v1.31.2) |
| **Migration** | golang-migrate | v4.19.1 | Embed + CLI |
| **Cache L1** | Ristretto | v2.4.2 | In-memory, high throughput (generics) |
| **Cache L2** | Valkey | 9.0.6 | go-redis v9.22.0 (Valkey-compatible) |
| **Messaging** | NATS JetStream | 2.14.6 | nats.go v1.53.1 |
| **Auth JWT** | golang-jwt | v5.3.1 | RS256 asymmetric |
| **OAuth2** | golang.org/x/oauth2 | v0.23.0 | Google, GitHub, OIDC |
| **RBAC** | Casbin | v2.8.0 | ABAC/RBAC hybrid |
| **Feature Flags** | OpenFeature + Unleash | v1.17.2 / v6.5.1 | go-sdk + official SDK |
| **Secrets** | Bitwarden SDK | v2.1.0 | Secret Manager |
| **Circuit Breaker** | gobreaker | v0.5.0 | Sony |
| **Tracing** | OpenTelemetry | v1.26.0 | Jaeger, Prometheus, OTLP |
| **Logging** | zerolog | v1.35.1 | JSON, structured, leveled; port in `shared/kernel/log` (ADR-012) |
| **Validation** | validator | v10.22.0 | Struct tags |
| **JSON** | `pkg/jsonparser` wrapper | Sonic v1.15.4 default (ADR-012); stdlib fallback | One codec seam; benchmark recorded in E01-T07 evidence |
| **i18n** | go-i18n | v2.7.0 | YAML locale files |
| **Scheduler** | gocron | v1.5.0 | Valkey Redlock distributed |
| **Config** | koanf | v2.3.0 | Multi-source, validation |
| **Testing** | testify, testcontainers, mockery | Pinned in `go.mod` at bootstrap | Unit, integration, contract |
| **API Gateway** | Traefik | v3.2.0 | Docker labels, auto-TLS |
| **Observability Stack** | Prometheus, Grafana, Loki, Tempo | Pinned image tags at bootstrap | Full LGTM target (Prom 2.54, Grafana 11.2, Loki 3.1, Tempo 2.5) |

---

## 3. Architecture Specification

### 3.1 Layer Structure

```mermaid
graph TB
    subgraph Interface["INTERFACE LAYER (cmd/ + internal/interface/ + pkg/)"]
        REST["REST API (Echo)"]
        gRPC["gRPC API (grpc-go)"]
        GraphQL["GraphQL (gqlgen)"]
        Cron["Cron (gocron)"]
        Consumer["Consumer (NATS)"]
    end

    subgraph Application["APPLICATION LAYER (internal/application)"]
        Commands["Commands (Write)"]
        Queries["Queries (Read)"]
        DTOs["DTOs"]
        Workflows["Workflows/Sagas"]
        Ports["Port Interfaces\n(Inbound: Use Cases, Outbound: Repos)"]
    end

    subgraph Domain["DOMAIN LAYER (internal/domain)"]
        Entity["Entity"]
        VO["Value Object"]
        Aggregate["Aggregate Root"]
        Event["Event"]
        Spec["Domain Specification"]
        RepoPorts["Repository Interfaces (Ports)"]
        DomainServices["Domain Services"]
    end

    subgraph Infrastructure["INFRASTRUCTURE LAYER (internal/infrastructure)"]
        Config["Config"]
        Database["Database"]
        Cache["Cache"]
        Auth["Auth"]
        Messaging["Messaging"]
        Scheduler["Scheduler"]
        FeatureFlags["Feature Flags"]
        Secrets["Secrets Manager"]
        Resilience["Resilience"]
        Audit["Audit"]
        Crypto["Crypto"]
        Middleware["Middleware"]
    end

    Interface --> Application
    Application --> Domain
    Application -.->|Port Interfaces| Domain
    Infrastructure -.->|Implements| Domain
    Infrastructure -.->|Implements application ports| Application
```

### 3.2 Dependency Rule (compact form)

```mermaid
graph LR
    Interface["Interface Layer\n(cmd/ + internal/interface/ + pkg/)"]
    Application["Application Layer\n(internal/application)"]
    Domain["Domain Layer\n(internal/domain)"]
    Infrastructure["Infrastructure Layer\n(internal/infrastructure)"]

    Interface --> Application
    Application --> Domain
    Infrastructure -.->|Implements Interfaces| Domain
    Infrastructure -.->|Implements Interfaces| Application
```

### 3.3 Dependency Rule (text form)
```
Interface Layer → Application Layer → Domain Layer
Infrastructure Layer ──implements ports owned by Application/Domain──┘
```

---

## 4. Directory Structure Specification

```
go-template/
├── .github/workflows/              # CI/CD pipelines
├── .opencode/                      # opencode agents & skills
├── AGENTS.md                       # Agent instructions
├── SPEC.md                         # This file - master specification
├── api/                            # API contracts (source of truth)
│   ├── proto/                      # Protobuf definitions
│   ├── openapi/                    # OpenAPI 3.1 specs
│   └── graphql/                    # GraphQL schemas
├── cmd/                            # Entry points (multiple binaries)
│   ├── rest-api/main.go
│   ├── grpc-api/main.go
│   ├── graphql-api/main.go
│   ├── cron/main.go
│   └── consumer/main.go
├── config/                         # Configuration files
│   ├── config.yaml                 # Base (committed)
│   ├── config.local.yaml           # Local overrides (gitignored)
│   ├── config.staging.yaml
│   ├── config.production.yaml
│   └── schemas/config.json         # JSON Schema validation
├── deployments/
│   ├── docker/
│   │   ├── docker-compose.yml              # Core deps
│   │   ├── docker-compose.featureflags.yml # Unleash
│   │   ├── docker-compose.gateway.yml      # Traefik
│   │   ├── docker-compose.observability.yml # LGTM stack
│   │   ├── docker-compose.local.yml        # All merged
│   │   ├── docker-compose.ci.yml           # CI minimal
│   │   ├── traefik/
│   │   ├── prometheus/
│   │   ├── grafana/
│   │   ├── loki/
│   │   ├── tempo/
│   │   └── Dockerfile*                     # Multi-stage
│   └── k8s/                          # Kustomize/Helm/ArgoCD
├── docs/                             # Documentation (MD per layer)
│   ├── architecture/                 # ADRs
│   ├── domain/
│   ├── application/
│   ├── infrastructure/
│   ├── api/
│   └── development/
├── internal/                         # Private application code
│   ├── domain/                       # DDD Core (NO external deps)
│   │   ├── entity/
│   │   ├── valueobject/
│   │   ├── aggregate/
│   │   ├── event/
│   │   ├── repository/               # Port interfaces
│   │   ├── service/
│   │   ├── specification/            # Domain Specification pattern
│   │   └── error/
│   ├── application/                  # Use Cases (CQRS)
│   │   ├── command/
│   │   ├── query/
│   │   ├── dto/
│   │   ├── port/
│   │   ├── service/
│   │   └── workflow/
│   ├── interface/                    # Private protocol/job adapters
│   │   ├── rest/
│   │   ├── grpc/
│   │   ├── cron/
│   │   └── consumer/
│   ├── infrastructure/               # External adapters
│   │   ├── config/
│   │   ├── database/postgres/
│   │   ├── database/migration/
│   │   ├── cache/local/              # Ristretto
│   │   ├── cache/valkey/             # Valkey (OSS Redis fork)
│   │   ├── cache/hybrid/             # L1+L2
│   │   ├── auth/jwt/, oauth2/, rbac/, apikey/
│   │   ├── messaging/nats/
│   │   │   ├── publisher/
│   │   │   ├── consumer/
│   │   │   └── stream/
│   │   ├── scheduler/
│   │   ├── featureflag/provider/     # OpenFeature + Unleash
│   │   ├── secrets/bitwarden/
│   │   ├── resilience/circuitbreaker/
│   │   ├── audit/
│   │   ├── crypto/
│   │   ├── tracing/
│   │   ├── logging/
│   │   ├── validator/
│   │   └── middleware/
│   └── shared/                       # Shared kernel
│       ├── kernel/
│       │   ├── error/                # AppError, codes, i18n
│       │   ├── pagination/
│       │   └── shutdown/             # Graceful shutdown
│       ├── di/                       # fx modules
│       ├── locale/                   # en.yaml, id.yaml, ...
│       └── util/
├── pkg/                              # Public reusable packages
│   ├── httpserver/
│   │   ├── server.go
│   │   ├── websocket/
│   │   ├── middleware/
│   │   ├── versioning/
│   │   └── health/
│   ├── grpcserver/
│   ├── graphql/
│   ├── scheduler/
│   └── jsonparser/                   # encoding/json-compatible codec wrapper
├── scripts/                          # Automation
│   ├── build/, dev/, test/, db/, generate/, release/, security/
├── test/                             # Test organization
│   ├── unit/                         # Mirrors internal/
│   ├── integration/                  # Testcontainers
│   ├── performance/                  # k6 scripts
│   ├── chaos/                        # Litmus (K8s)
│   ├── contract/                     # Pact
│   ├── fixtures/
│   ├── mock/                         # Generated (mockery)
│   └── testcontainers/               # Shared container defs
├── go.mod
├── go.sum
├── Makefile
├── .golangci.yml
├── .air.toml
├── renovate.json
└── README.md
```

---

## 5. Domain Layer

### 5.1 Core Building Blocks

The snippets in this section describe domain concepts, not a mandate for a
universal base class. Go implementations should prefer concrete domain structs
and small consumer-owned interfaces. Add a shared interface only when a real
consumer needs that polymorphism; do not force every entity or value object to
implement an `Entity`, `ValueObject`, or generic repository abstraction.

#### Entity, Value Object, and Aggregate Root

These are concrete domain types, not framework base classes. Entities expose
typed identity and encapsulate lifecycle changes; value objects validate at
construction and are immutable; aggregate roots own invariants, version/event
buffers, and the methods that can change their state. If a use case needs
polymorphism, define a small interface for that use case. Do not add a shared
`Entity`, `ValueObject`, or `AggregateRoot` interface solely to satisfy DDD
terminology.

#### Domain Event
```go
// internal/domain/event/base.go
type DomainEvent interface {
    EventID() string
    AggregateID() string
    EventType() string
    OccurredAt() time.Time
    Payload() any
}
```

The interface above is the narrow consumer contract; the compile-safe envelope
and metadata implementation live in `docs/domain-events.md §2`. Event struct
fields must not collide with method names, and every event carries an aggregate
sequence.

#### Repository Port (Interface in Domain)
```go
// internal/domain/repository/account.go
// Define the smallest interface required by the consuming use case.
type AccountRepository interface {
    Create(ctx context.Context, account *entity.Account) error
    FindByID(ctx context.Context, id AccountID) (*entity.Account, error)
    FindByTenant(ctx context.Context, tenantID TenantID, pageToken string) ([]*entity.Account, error)
    UpdateMetadata(ctx context.Context, account *entity.Account, expectedVersion int64) error
    UpdateStatus(ctx context.Context, id AccountID, status AccountStatus, expectedVersion int64) error
}
```

There is no generic `Delete` method for ledger facts. Posted postings and
entries are immutable; metadata aggregates expose only explicitly authorized
retirement/closure behavior.

### 5.2 Domain Specifications — Executable Business Rules

```go
// internal/domain/specification/specification.go
type Specification[T any] interface {
    Evaluate(ctx context.Context, candidate T) SpecResult
}

type SpecResult struct {
    Violations []Violation
}

func (r SpecResult) Passed() bool { return len(r.Violations) == 0 }

// All evaluates every child so callers receive every failed rule. Any may
// short-circuit on the first passing child. Specifications hold no evaluation
// state and are safe for concurrent reuse.
func All[T any](specs ...Specification[T]) Specification[T]
func Any[T any](specs ...Specification[T]) Specification[T]
func Not[T any](spec Specification[T], violation Violation) Specification[T]
```

**Example Specifications (Fintech Ledger):**
```go
// internal/domain/specification/account/
var (
    ValidCurrency        = NewAccountSpec("valid_currency", func(ctx context.Context, a *Account) bool { ... })
    AccountNotFrozen     = NewAccountSpec("not_frozen", func(ctx context.Context, a *Account) bool { ... })
    PostingBalancedPerCurrency = NewPostingSpec("balanced_per_currency", func(ctx context.Context, p *Posting) bool { ... })
)

func SufficientFunds(snapshot BalanceSnapshot, amount Money) Specification[PostingCandidate] { ... }
```

Repository-backed uniqueness is not a pure domain specification. Durable
idempotency is enforced by the application/persistence transaction described in
`docs/ledger-core.md §8`.

### 5.3 Domain Services (Cross-Aggregate Logic)
```go
// internal/domain/service/transfer_service.go
type TransferCandidate struct {
    FromAccountID AccountID
    ToAccountID   AccountID
    Amount        Money
}

type TransferResult struct {
    PostingID PostingID
}

type TransferService interface {
    ExecuteTransfer(ctx context.Context, candidate TransferCandidate) (TransferResult, error)
}
```

---

## 6. Application Layer Specification (CQRS)

### 6.1 Commands (Write Model)

Commands are the only application entry points allowed to change state. They
validate input, invoke domain behavior, and commit through a unit-of-work port;
they do not expose persistence models.
```go
// internal/application/command/transfer_command.go
type TransferCommand struct {
    FromAccountID   AccountID
    ToAccountID     AccountID
    Amount          Money
    IdempotencyKey  string
    Metadata        map[string]string
}

type TransferCommandHandler interface {
    Handle(ctx context.Context, cmd TransferCommand) (*TransferResult, error)
}
```

### 6.2 Queries (Read Model)

Queries are read-only from the caller's perspective. They may use a labeled
cache or replica, but must return cursor/as-of information when the read is not
strongly consistent and must not emit business side effects.
```go
// internal/application/query/account_query.go
type GetAccountQuery struct {
    AccountID AccountID
}

type GetAccountQueryHandler interface {
    Handle(ctx context.Context, query GetAccountQuery) (*AccountDTO, error)
}
```

### 6.3 Application Services (Orchestration)
```go
// internal/application/service/account_service.go
type AccountService interface {
    OpenAccount(ctx context.Context, cmd OpenAccountCommand) (*AccountDTO, error)
    CloseAccount(ctx context.Context, cmd CloseAccountCommand) error
    GetAccount(ctx context.Context, id AccountID) (*AccountDTO, error)
}
```

### 6.4 Workflows/Sagas (Long-Running Processes)
```go
// internal/application/workflow/transfer_workflow.go
type TransferWorkflow interface {
    Execute(ctx context.Context, cmd TransferCommand) (*WorkflowResult, error)
    Compensate(ctx context.Context, workflowID string) error
}
```

---

## 7. Infrastructure Layer Specification

### 7.1 Configuration
```go
// internal/infrastructure/config/config.go
type Config struct {
    App       AppConfig       `koanf:"app" validate:"required"`
    Server    ServerConfig    `koanf:"server" validate:"required"`
    Database  DatabaseConfig  `koanf:"database" validate:"required"`
    Cache     CacheConfig     `koanf:"cache" validate:"required"`
    Auth      AuthConfig      `koanf:"auth" validate:"required"`
    NATS      NATSConfig      `koanf:"nats" validate:"required"`
    Observability ObservabilityConfig `koanf:"observability"`
    FeatureFlags FeatureFlagConfig `koanf:"feature_flags"`
    Secrets   SecretsConfig   `koanf:"secrets"`
}
```

**Loading Priority:** Base YAML → Environment YAML → Local YAML → Env Vars → Secrets

### 7.2 Database (PostgreSQL + GORM)
- **ORM**: GORM v2 with interfaces
- **Migrations**: golang-migrate (embed + CLI, Up/Down reversible)
- **Connection Pool**: Configurable (max_open, max_idle, max_lifetime)
- **Ledger write path**: explicit, reviewable SQL transaction or stored procedure;
  database-enforced per-currency balance, immutability, tenant/ledger scope,
  deterministic locking, durable idempotency, balance checkpoint, and outbox
- **Authority**: PostgreSQL entries/checkpoints are authoritative; GORM hooks,
  account balance columns, Valkey, and replicas are never money-safety boundaries

### 7.3 Cache (Multi-Level)
```go
// internal/infrastructure/cache/hybrid/cache.go
type HybridCache struct {
    l1 *ristretto.Cache  // Hot data, ~100MB
    l2 *redis.Client      // Shared, persistent (Valkey-compatible)
}

func (h *HybridCache) Get(ctx context.Context, key string, dest any) error {
    // Try L1 → Try L2 → Populate L1
}
```

Cached balances are labeled projections with a ledger cursor. They may accelerate
display reads but never authorize a spend. Idempotency cache entries are hints;
the durable record is in PostgreSQL.

### 7.4 Authentication & Authorization
- **JWT**: RS256, short-lived access (15m), rotating refresh (7d)
- **OAuth2/OIDC**: Google, GitHub, generic OIDC
- **RBAC/ABAC**: Casbin with domain-specific policies
- **API Keys**: Scoped, rate-limited, rotatable

### 7.5 Messaging (NATS JetStream)
```go
// Subject Naming Convention: ledger.{tenant}.{event_type}
// The versioned event type already carries domain/entity/action, so it is not
// duplicated in the subject. Platform consumers use bounded wildcards.
const (
    SubjectAccountCreated    = "ledger.{tenant}.account.created.v1"
    SubjectTransactionPosted = "ledger.{tenant}.transaction.posted.v1"
    SubjectBalanceChanged    = "ledger.{tenant}.account.balance.changed.v1"
)
```
- **Publisher**: Embedded in API services
- **Consumer**: Separate `cmd/consumer/` binary (scales independently)
- **Dead Letter**: Automatic DLQ with retry policy

### 7.6 Feature Flags (OpenFeature + Unleash)
```go
// Provider: github.com/open-feature/go-sdk-contrib/providers/unleash
// Evaluation context: user_id, tenant_id, custom attributes
```

### 7.7 Secrets (Bitwarden Secret Manager)
```yaml
# config.yaml references only
database:
  postgres:
    password: "{{ secret:db/postgres/password }}"
auth:
  jwt:
    private_key: "{{ secret:auth/jwt/private_key }}"
```

### 7.8 Resilience (Circuit Breaker)
```go
// Breaker is an inner-layer port; gobreaker is only one adapter.
type Breaker interface {
    Execute(ctx context.Context, operation string, idempotencyKey string,
        fn func(context.Context) error) error
}

// Apply per dependency (payment processor, FX, SMTP, OAuth, flags, etc.).
// A retry policy must classify errors and refuse blind retries of non-idempotent
// money operations without a durable/provider idempotency key.
```
- **States**: closed → open → half-open with bounded probe calls and metrics.
- **Timeouts**: every outbound call has a context deadline; cancellation is not
  retried. Retry/backoff is bounded and configured per dependency.
- **Ownership**: E01-T08 defines the port/policy; provider tasks consume it and
  E15-T06 audits every external call site.

### 7.9 Observability
- **Tracing**: OpenTelemetry → Jaeger/Tempo
- **Metrics**: Prometheus (RED + USE)
- **Logging**: zerolog JSON + correlation ID (ADR-012; port unchanged)
- **Health**: Liveness/Readiness/Startup probes

### 7.10 Third-Party Abstractions (Ports)

Every external tool sits behind a domain/application-owned interface so implementations
can be swapped without touching business logic (dependency inversion):

| Capability | Port (interface owner) | Default adapter | Swappable with |
|------------|------------------------|-----------------|----------------|
| **Logging** | `Logger` (`shared/kernel/log`) | zerolog v1.35.1 (ADR-012; was `log/slog` per ADR-010) | `log/slog`, zap |
| **Tracing** | `Tracer` (`shared/kernel/observability`) | OTel SDK + Jaeger/Tempo adapter | Datadog, Honeycomb |
| **Metrics** | `Meter` (`shared/kernel/observability`) | OTel + Prometheus adapter | StatsD, Datadog |
| **Database** | Intent-specific repository/unit-of-work ports (`domain`/`application`) | GORM + PostgreSQL adapter | sqlx, sqlc, Ent, CockroachDB |
| **Cache** | `Cache` (`application/port`) | Ristretto (L1) + Valkey (L2) | Memcached, Dragonfly, in-memory only |
| **Rate limiting** | `RateLimiter` (`application/port`) | Valkey token bucket adapter | In-memory/test implementation |
| **Messaging** | `Publisher` / `Consumer` (`application/port`) | NATS JetStream | Kafka, RabbitMQ, SQS |
| **Secrets** | `SecretManager` (`application/port`) | Bitwarden SDK adapter | Vault, AWS/GCP Secret Manager, env |
| **Feature Flags** | `FlagClient` (`application/port`) | OpenFeature + Unleash adapter | LaunchDarkly, Flipt, Redis provider |
| **Resilience** | `Breaker`/`RetryPolicy` (`shared/kernel/resilience`) | gobreaker + bounded retry adapter | Hystrix-style or provider-native adapter |
| **Clock** | `Clock` (`shared/kernel`) | System clock | Fixed clock (tests) |
| **ID Generation** | `IDGenerator` (`shared/kernel`) | UUIDv7 (`google/uuid`; ADR-012) | ULID, KSUID, Snowflake |
| **JSON** | `jsonparser` (`pkg/jsonparser`) | Sonic v1.15.4 default (ADR-012); stdlib fallback | Codec only ever behind the wrapper; benchmark recorded |

**Rules:**
- Domain and application layers import **only** the port, never the adapter package.
- Each adapter lives in `internal/infrastructure/<capability>/<impl>/` and is wired via fx.
- Swapping = new adapter package + one-line fx binding change, zero business-logic edits.

---

## 8. Interface Layer Specification (Multi-Binary)

### 8.1 REST API (Echo)
- **Router**: Echo v5 with middleware chain
- **Validation**: Request/Response DTO validation
- **Error Handling**: Unified AppError → HTTP mapping
- **Versioning**: URL path (`/v1/`) + header fallback
- **WebSocket**: Native Echo + NATS bridge for real-time

### 8.2 gRPC API
- **Codegen**: Protobuf → Go + gRPC Gateway (REST proxy)
- **Interceptors**: Auth, logging, tracing, circuit breaker
- **Reflection**: Enabled for debugging

### 8.3 GraphQL API (gqlgen)
- **Schema-First**: `.graphqls` files as source of truth
- **Codegen**: gqlgen → resolvers
- **Subscriptions**: WebSocket transport
- **DataLoader**: N+1 prevention

### 8.4 Cron (Distributed Scheduler)
- **Framework**: gocron v1.5.0
- **Distributed Lock**: Valkey Redlock (go-redis v9.22.0)
- **Jobs**: leader coordination plus a durable unique run key; at-least-once
  invocation, idempotent effects, fencing/lease checks, and automatic failover

### 8.5 Consumer (NATS)
- **Separate Binary**: Deploys to different cluster
- **Pull-based**: JetStream consumer groups
- **Scaling**: Horizontal, independent of API tier

---

## 9. Cross-Cutting Concerns Specification

### 9.1 Error Handling
```go
// internal/shared/kernel/error/app_error.go
type AppError struct {
    Code       string         // Machine: "USER_NOT_FOUND"
    Message    string         // Human: "User not found"
    Details    map[string]any // Context
    Cause      error          // Wrapped
    HTTPStatus int            // Mapping
}
```
- **i18n**: go-i18n with YAML locale files (en.yaml, id.yaml, ...)
- **Codes**: Stable uppercase snake-case registry; domain-specific codes may use
  a `DOMAIN_ENTITY_ACTION`-style prefix, while transport/common codes such as
  `VALIDATION_FAILED` and `PAYOUT_BLOCKED` remain explicit stable identifiers.

### 9.2 Request/Response Standardization
```go
// Standard envelope
type Response[T any] struct {
    Data       *T        `json:"data,omitempty"`
    Error      *AppError `json:"error,omitempty"`
    Meta       *Meta     `json:"meta,omitempty"`
    RequestID  string    `json:"request_id"`
}
```

### 9.3 Validation
- **Struct Tags**: `validate:"required,email,max=255"`
- **Custom Validators**: Domain-specific rules
- **Response**: Standardized validation error format

### 9.4 Rate Limiting
- **Algorithm**: Token bucket (Valkey-backed for distributed)
- **Dimensions**: IP, User, API Key, Endpoint
- **Headers**: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`

### 9.5 Correlation ID
- **Header**: `X-Request-ID` (generate if missing)
- **Propagation**: Context → HTTP/gRPC/NATS headers
- **Logging**: Included in all log entries through the kernel logger port

### 9.6 Graceful Shutdown
- **fx Lifecycle**: `OnStart`/`OnStop` hooks
- **Signal Handling**: SIGTERM, SIGINT
- **Drain**: Stop accepting → finish in-flight → close connections

### 9.7 Panic Handling & Recovery

Panics must never crash a server, leak a stack trace to clients, or lose a message.
Recovery is enforced at every execution boundary:

| Boundary | Mechanism | On panic |
|----------|-----------|----------|
| **REST (Echo)** | `RecoveryMiddleware` (first in chain, after RequestID) | Log ERROR with stack + trace/correlation ID → emit metric `http.panics.total` → alert → return `500 INTERNAL_ERROR` envelope (no stack in body) |
| **gRPC** | `recovery.UnaryServerInterceptor` + `StreamServerInterceptor` | Same as REST, mapped to `codes.Internal` with `request_id` in trailing metadata |
| **GraphQL** | gqlgen `RecoverFunc` | Same as REST, returned as GraphQL error extension `{code: INTERNAL_ERROR, request_id}` |
| **NATS Consumer** | `RecoverHandler` wrapper around each message handler | Log + metric `consumer.panics.total` → **Nak** (redeliver) with backoff; after max delivers → DLQ + alert |
| **Cron Job** | `RecoverJob` wrapper in scheduler | Log + metric `cron.panics.total` → mark run FAILED → release distributed lock → alert |
| **Background goroutine** | `shared/kernel/safe.Go()` helper | Log + metric, cancel/restart the owning component; never ACK or hide a mutation |

```go
// pkg/httpserver/middleware/recovery.go
func RecoveryMiddleware(log Logger, meter Meter) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c *echo.Context) (err error) {
            defer func() {
                if r := recover(); r != nil {
                    stack := debug.Stack()
                    reqID := RequestIDFrom(c)
                    log.Error(c.Request().Context(), "panic recovered",
                        "panic", r, "stack", string(stack),
                        "request_id", reqID, "path", c.Path())
                    meter.Counter("http.panics.total").Add(c.Request().Context(), 1)
                    err = NewAppError("INTERNAL_ERROR", "Internal server error", http.StatusInternalServerError)
                }
            }()
            return next(c)
        }
    }
}
```

**Panic budget:** any panic in production pages on-call (postmortem required).
Panics in domain logic are treated as bugs — domain code must return errors,
never panic. Recovery is a containment boundary, not success: a panic during a
financial mutation must cancel or fail its owner, leave the operation
retryable/unknown, and never acknowledge work as committed.

### 9.8 HTTP/3 (QUIC)

HTTP/3 is supported but **off by default** (feature-flagged), since it requires UDP,
TLS, and load-balancer support:

- **Echo**: `pkg/httpserver` exposes `NewHTTP3Server()` using `quic-go`; same router,
  middleware chain, and handlers as HTTP/1.1+2 (no handler changes).
- **Requirements**: TLS 1.3 certificate (no plaintext QUIC); UDP port open (default 443/udp).
- **Traefik**: enable `entryPoints.websecure.http3: {}` and expose `443/udp` alongside `443/tcp`.
- **Fallback**: clients without QUIC automatically fall back to HTTP/2 → HTTP/1.1
  (Alt-Svc header advertisement).
- **Config**: `server.rest.http3.enabled: false` (default); enable per environment.
- **Testing**: k6 with `--http3` flag; CI runs HTTP/1.1 suite by default, HTTP/3 as nightly job.

---

## 10. Testing Specification

### 10.1 Test Pyramid
```
           ┌─────────────┐
           │   E2E       │  ← Few, critical paths
           ├─────────────┤
           │  Contract   │  ← Pact consumer-driven
           ├─────────────┤
           │ Integration │  ← Testcontainers (real deps)
           ├─────────────┤
           │    Unit     │  ← Many, fast, isolated
           └─────────────┘
```

### 10.2 Unit Tests
- **Domain**: Pure functions, no mocks needed
- **Application**: Mock repository ports (mockery)
- **Infrastructure**: Integration-style with testcontainers

### 10.3 Integration Tests
- **Testcontainers**: Postgres, Valkey, NATS, Unleash, MinIO
- **API Contracts**: Test against running services
- **Database**: Migration up/down, seed, verify

### 10.4 Performance Tests
- **Tool**: k6 scripts in `test/performance/`
- **Scenarios**: Baseline, load, stress, soak

### 10.5 Chaos Engineering (K8s only)
- **Tool**: Litmus/Chaos Mesh
- **Scenarios**: Pod kill, network latency, DB failure

### 10.6 Contract Tests
- **Tool**: Pact (consumer-driven)
- **Broker**: Dockerized in CI

---

## 11. CI/CD Specification

### 11.1 Pipeline Stages
```yaml
# .github/workflows/ci.yml
stages:
  - lint:           golangci-lint (strict)
  - test-unit:      go test ./internal/... ./pkg/...
  - test-integration: testcontainers (docker-compose.ci.yml)
  - test-contract:  pact verification
  - build:          Multi-binary, multi-arch
  - security:       govulncheck, gosec, trivy (container)
  - sbom:           syft → spdx.json
  - license:        go-licenses check
```

### 11.2 Deployment
- **Staging**: Auto on merge to main
- **Production**: Manual approval, tag-based
- **GitOps**: ArgoCD/Flux with Kustomize overlays
- **Rollback**: Helm/Kustomize rollback, DB migration down

---

## 12. Docker Compose Specifications (All Dependencies)

### 12.1 Core (`docker-compose.yml`)
```yaml
services:
  postgres:
    image: postgres:18-alpine
    environment: 
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_MULTIPLE_DATABASES: app,unleash
    ports: ["5432:5432"]
    healthcheck: pg_isready -U postgres
    volumes: [postgres_data:/var/lib/postgresql/data]

  valkey:
    image: valkey/valkey:9-alpine
    ports: ["6379:6379"]
    healthcheck: valkey-cli ping
    volumes: [valkey_data:/data]

  nats:
    image: nats:2.14-alpine
    ports: ["4222:4222", "8222:8222"]
    command: ["-js", "-m", "8222"]

  jaeger:
    image: jaegertracing/all-in-one:1.58
    ports: ["16686:16686", "14268:14268"]

  maildev:
    image: maildev/maildev:3.0.0-rc.3
    ports: ["1025:1025", "1080:1080"]
    command: ["--smtp-port=1025", "--web-port=1080"]

volumes:
  postgres_data:
  valkey_data:
```

### 12.2 Feature Flags (`docker-compose.featureflags.yml`)
```yaml
# Extends docker-compose.yml - adds Unleash using shared PostgreSQL
services:
  unleash:
    image: unleashorg/unleash-server:6.5
    environment:
      DATABASE_URL: postgres://postgres:postgres@postgres:5432/unleash
      ADMIN_API_KEY: "dev-admin-key"
    ports: ["4242:4242"]
    depends_on:
      postgres:
        condition: service_healthy
```

### 12.3 Gateway (`docker-compose.gateway.yml`)
```yaml
# Extends docker-compose.yml - adds Traefik
services:
  traefik:
    image: traefik:v3.2
    command:
      - "--api.dashboard=true"
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
    ports: ["80:80", "443:443", "8080:8080"]
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "letsencrypt:/letsencrypt"
    labels:
      - "traefik.enable=true"

volumes:
  letsencrypt:
```

### 12.4 Observability (`docker-compose.observability.yml`)
```yaml
# Extends docker-compose.yml - adds LGTM Stack
services:
  prometheus:
    image: prom/prometheus:v2.54
    ports: ["9090:9090"]
    volumes: [./prometheus:/etc/prometheus, prom_data:/prometheus]

  grafana:
    image: grafana/grafana:11.2
    ports: ["3000:3000"]
    environment: {GF_SECURITY_ADMIN_PASSWORD: admin}
    volumes: [grafana_data:/var/lib/grafana, ./grafana/dashboards:/etc/grafana/provisioning/dashboards]

  loki:
    image: grafana/loki:3.1
    ports: ["3100:3100"]
    volumes: [./loki:/etc/loki, loki_data:/loki]

  tempo:
    image: grafana/tempo:2.5
    ports: ["3200:3200", "4317:4317", "9411:9411"]
    volumes: [./tempo:/etc/tempo, tempo_data:/tempo]

volumes:
  prom_data:
  grafana_data:
  loki_data:
  tempo_data:
```

### 12.5 CI Minimal (`docker-compose.ci.yml`)
```yaml
# Minimal dependencies for CI - no UI services
services:
  postgres:
    image: postgres:18-alpine
    environment: 
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: app
    ports: ["5432:5432"]
    healthcheck: pg_isready -U postgres
    tmpfs: /var/lib/postgresql/data

  valkey:
    image: valkey/valkey:9-alpine
    ports: ["6379:6379"]
    healthcheck: valkey-cli ping
    tmpfs: /data

  nats:
    image: nats:2.14-alpine
    ports: ["4222:4222"]
    command: ["-js", "-m", "8222"]
```

### 12.6 Local Dev (`docker-compose.local.yml`)
```yaml
# Complete stack for local development - combines all above
# Usage: docker compose -f docker-compose.yml -f docker-compose.featureflags.yml -f docker-compose.gateway.yml -f docker-compose.observability.yml up
# Or: make dev-up

# This file documents the compose command, actual composition is done via multiple -f flags
# Core: postgres, valkey, nats, jaeger, maildev
# + FeatureFlags: unleash
# + Gateway: traefik
# + Observability: prometheus, grafana, loki, tempo
```

### 12.7 Production (`docker-compose.prod.yml`)
```yaml
# Production-ready configuration with resource limits and secrets
services:
  postgres:
    image: postgres:18-alpine
    environment: 
      POSTGRES_USER_FILE: /run/secrets/postgres_user
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password
      POSTGRES_MULTIPLE_DATABASES: app,unleash
    ports: ["5432:5432"]
    healthcheck: pg_isready -U postgres
    volumes: [postgres_data:/var/lib/postgresql/data]
    deploy:
      resources:
        limits:
          memory: 2G
        reservations:
          memory: 1G

  valkey:
    image: valkey/valkey:9-alpine
    ports: ["6379:6379"]
    healthcheck: valkey-cli ping
    volumes: [valkey_data:/data]
    command: ["--requirepass-file", "/run/secrets/valkey_password"]
    deploy:
      resources:
        limits:
          memory: 1G
        reservations:
          memory: 512M

  nats:
    image: nats:2.14-alpine
    ports: ["4222:4222", "8222:8222"]
    command: ["-js", "-m", "8222", "--config", "/etc/nats/nats.conf"]
    volumes: [./nats/nats.conf:/etc/nats/nats.conf:ro]

  traefik:
    image: traefik:v3.2
    command:
      - "--api.dashboard=false"
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.web.http.redirections.entrypoint.to=websecure"
      - "--entrypoints.web.http.redirections.entrypoint.scheme=https"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
    ports: ["80:80", "443:443"]
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "letsencrypt:/letsencrypt"
    deploy:
      labels:
        - "traefik.enable=true"

  app:
    # Supply an immutable release tag or digest; never deploy a floating tag.
    image: ${LEDGER_IMAGE:?set LEDGER_IMAGE to an immutable tag or digest}
    environment:
      - CONFIG_ENV=production
    deploy:
      replicas: 3
      resources:
        limits:
          memory: 512M
      restart_policy:
        condition: on-failure

volumes:
  postgres_data:
  valkey_data:
  letsencrypt:
secrets:
  postgres_user:
    external: true
  postgres_password:
    external: true
  valkey_password:
    external: true
```

---

## 13. Fintech Ledger - Reference Domain Specification

### 13.1 Core Entities
```go
// Account - classification metadata; balances are projections, not aggregate fields
type Account struct {
    ID            AccountID
    TenantID      TenantID
    LedgerID      LedgerID
    Name          string
    Type          AccountType     // ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE
    NormalSide    EntryDirection  // derived from account class
    AssetCode     AssetCode       // registry-backed currency/asset code
    Status        AccountStatus   // ACTIVE, FROZEN, CLOSED
    Version       int             // Optimistic locking
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// Posting - immutable accounting fact; it has no pending/failed state
type Posting struct {
    ID                PostingID
    TenantID          TenantID
    LedgerID          LedgerID
    Operation         string        // posting-template name/version
    ExternalReference string
    Description       string
    Entries           []Entry       // balances per asset code
    EffectiveAt       time.Time
    RecordedAt        time.Time     // server assigned
    ReversalOf        *PostingID
    Metadata          map[string]string
}

// Entry - Single side of double-entry
type Entry struct {
    ID            EntryID
    PostingID     PostingID
    AccountID     AccountID
    Direction     EntryDirection    // DEBIT, CREDIT
    AmountMinor   int64             // strictly positive, checked arithmetic
    AssetCode     AssetCode
    AccountSeq    int64             // deterministic per-account ordering
}

// Dispute - Chargeback lifecycle
type Dispute struct {
    ID               DisputeID
    TenantID         TenantID
    TransactionID    TransactionID // compatibility alias for the immutable capture PostingID; workflow code also retains its payment/provider link
    Network          string          // VISA, MASTERCARD, ACH, ...
    Amount           Money
    FeeAmount        Money           // Network fee (reversed on win)
    Status           DisputeStatus   // OPENED, EVIDENCE_DUE, WON, LOST, CLOSED
    EvidenceDueAt    time.Time
    RepresentmentCount int           // Max 1 with new evidence
    CreatedAt        time.Time
    ClosedAt         *time.Time
}
```

Payment intents, authorizations, captures, refunds, disputes, payouts, and their
states are separate workflow aggregates. Holds/reservations affect available
balance but are not mutable states or entries on an immutable Posting. See
`docs/ledger-core.md §§1–8`.

### 13.2 Domain Invariants (Specifications)
| Specification | Description |
|---------------|-------------|
| `PostingBalancesPerCurrency` | For each asset code, sum of debits == sum of credits |
| `ValidCurrency` | Every entry asset code matches its account; different currencies use separate balanced lots |
| `SufficientFunds` | Configured spend legs do not exceed available balance; debit alone is not a universal spend signal |
| `AccountActive` | Account not FROZEN/CLOSED |
| `EntryAmountPositive` | Entry amount is a positive integer minor-unit quantity; no signed entries |
| `PostingTemplateAllowed` | Accounts/sides match the versioned operation template; both sides are valid for every account class |
| `CaptureAmountValid` | Total captured never exceeds authorized (money-flow §2.12) |
| `AllocationExact` | Split shares sum exactly to source after largest-remainder allocation (money-flow §2.13) |
| `PayoutEligibility` | Strong available balance satisfies configured minimum, reserve, first-payout, method, and destination policy |
| `RefundWindowValid` / `RefundAmountValid` | Refund within window and within unrefunded remainder |
| `PeriodOpen` | No posting into CLOSED periods |

Idempotency uniqueness, request fingerprint equality, tenant/ledger scope,
immutability, and concurrent-spend serialization are enforced transactionally in
PostgreSQL in addition to domain checks.

### 13.3 Domain Events
- `account.created`, `account.closed`, `account.frozen`, `account.unfrozen`, `account.verified`
- `transaction.posted`, `transaction.reversed`, `transaction.failed`, `payment.captured`
- `payment_intent.requires_action` (SCA/3DS challenge)
- `dispute.opened`, `dispute.closed` (outcome won/lost)
- `topup.succeeded`, `topup.failed`
- `tenant.created`
- `account.balance.changed` (per account, per posting)
- Full catalog: `docs/domain-events.md §3` (source of truth for names + payloads)

### 13.4 Workflows
- **Transfer**: Apply the versioned transfer template's source/destination
  sides → post the immutable Posting
- **Reversal**: Create reversing transaction → Post → Notify
- **Reconciliation**: Scheduled job → Compare computed vs stored → Alert
- **Destination charge**: Charge platform → carve application fee → transfer net to connected account (money-flow §2.10)
- **Dispute**: Hold + fee → evidence → won (release) / lost (reversal) (money-flow §2.11, journeys §2.7)
- **Auth-capture**: Authorize (hold) → capture full/partial → expiry auto-void (money-flow §2.12)

---

## 14. Security & Compliance Specification

### 14.1 OWASP Top 10:2025 Coverage
| Risk | Mitigation |
|------|------------|
| A01: Broken Access Control | Casbin RBAC/ABAC, object- and tenant-scope checks, default deny |
| A02: Security Misconfiguration | Secure defaults, config validation, no secrets in code |
| A03: Software Supply Chain Failures | govulncheck, dependency review, SBOM, signed provenance and containers |
| A04: Cryptographic Failures | TLS 1.3, Argon2id passwords, envelope encryption, rotation |
| A05: Injection | Parameterized queries, input validation, command and template hardening |
| A06: Insecure Design | Threat modeling, abuse cases, executable security requirements |
| A07: Authentication Failures | Asymmetric access tokens, short expiry, refresh rotation, MFA-ready controls |
| A08: Software or Data Integrity Failures | Signed artifacts, authenticated events, webhook signatures, integrity checks |
| A09: Security Logging and Alerting Failures | Structured audit logs, correlation IDs, signed checkpoints and alerts |
| A10: Mishandling of Exceptional Conditions | Fail closed, bounded retries, explicit unknown outcomes, chaos and fault-injection tests |

SSRF remains a required threat-model control even though it is no longer a
standalone 2025 Top 10 category: validate destinations, use allowlists, block
link-local/private metadata targets, and constrain egress.

### 14.2 Data Protection
- **Encryption at Rest**: Envelope encryption (DEK + KEK), KEK in Bitwarden/HSM
- **Encryption in Transit**: TLS 1.3 everywhere (mTLS for service-to-service)
- **PII Handling**: Field-level encryption, GDPR erasure workflow
- **Audit Logging**: Append-only, signed, tamper-evident

PCI DSS controls apply only after the cardholder-data environment and service
provider responsibilities are explicitly scoped. This specification is a
control plan, not evidence of assessment or compliance; prefer processor tokens
so PAN and sensitive authentication data never enter the ledger service.

---

## 15. Implementation Phases (Trackable)

| Phase | Scope | Deliverables | Status |
|-------|-------|--------------|--------|
| **1. Foundation** | go.mod, Makefile, config, kernel, DI, scripts, docker-compose | Buildable skeleton, dev environment | 📋 Planned |
| **2. Domain Layer** | Entities, VOs, Aggregates, Events, Repositories, Specifications | Pure domain, executable specs | 📋 Planned |
| **3. Infrastructure** | Postgres+GORM+Migrations, Cache (Ristretto+Valkey), Auth, NATS, Scheduler, FeatureFlags, Secrets, Resilience, Audit, Crypto, Observability | All adapters implemented | 📋 Planned |
| **4. Application Layer** | Commands, Queries, DTOs, Ports, Services, Workflows | CQRS use cases, orchestration | 📋 Planned |
| **5. Interface Layer** | REST (Echo), gRPC, GraphQL, Cron, Consumer | 5 binaries, WebSocket, HTTP/3 ready | 📋 Planned |
| **6. Cross-Cutting** | Middleware, Error handling, Validation, Rate limit, Correlation ID, Health, Graceful shutdown | Production-ready concerns | 📋 Planned |
| **7. Testing** | Unit, Integration (testcontainers), Performance (k6), Contract (Pact), Chaos | Test pyramid complete | 📋 Planned |
| **8. Documentation** | ADRs, Layer docs, API docs, Development guides | Complete docs | 📋 Planned |
| **9. CI/CD & Deploy** | GitHub Actions, Docker multi-stage, K8s/Helm, ArgoCD | Automated pipeline | 📋 Planned |
| **10. Polish** | Linting, security scan, SBOM, licenses, benchmark, examples | Release candidate | 📋 Planned |

---

## 16. Acceptance Criteria (Definition of Done)

### Per Phase
- [ ] All code compiles (`go build ./...`)
- [ ] Linting passes (`golangci-lint run`)
- [ ] Unit tests pass (`go test ./internal/... ./pkg/...`)
- [ ] Integration tests pass (testcontainers)
- [ ] Documentation updated
- [ ] ADR recorded for architectural decisions

### Template Readiness
- [ ] `make dev-up` starts all dependencies
- [ ] `make build-all` produces 5 binaries
- [ ] `make test-all` runs full test suite
- [ ] `make migrate-up` applies migrations
- [ ] New project can be scaffolded in < 10 min
- [ ] Fintech Ledger reference implementation compiles

---

## 17. Open Decisions (To Resolve During Implementation)

| Decision | Options | Recommendation |
|----------|---------|----------------|
| HTTP/3 enable default | On/Off | Off (feature flag) |
| Migration strategy | Up/Down vs Up-only | Up/Down (reversible) |
| Custom JSON parser | stdlib / Sonic | Sonic default per owner (ADR-012); benchmark recorded in E01-T07 evidence |
| WebSocket protocol | JSON / Protobuf | JSON (simpler), Protobuf for high-perf |
| Distributed trace sampling | Always / Probabilistic | Probabilistic (10%) |
| Log sampling in prod | None / Tail / Adaptive | Tail (ERROR always, WARN sampled) |
| Balance materialization | On-demand / immutable checkpoints / derived projection | Benchmark and owner approval required (ADR-011) |

---

## 18. References & Resources

- **Clean Architecture**: Robert C. Martin
- **DDD**: Eric Evans, Vaughn Vernon
- **Delivery SDD**: `tasks/SDD.md` task packets, scenarios, evidence, and takeover protocol
- **Domain Specification pattern**: Specification by Example (Gojko Adzic)
- **CQRS/ES**: Greg Young, Martin Fowler
- **Go Best Practices**: Uber Go Style, Google Go Style
- **OpenFeature**: https://openfeature.dev
- **Unleash**: https://unleash.io
- **Bitwarden Secrets**: https://bitwarden.com/products/secrets-manager
- **Testcontainers**: https://golang.testcontainers.org

---

## 19. Version History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-09-10 | Planning Agent | Initial specification |
| 1.1.0 | 2026-09-11 | Codex audit | Added normative ledger correctness contract; corrected boundaries, persistence authority, posting model, and delivery semantics |
| 1.1.1 | 2026-09-13 | Repository owner | Approved ledger-core precedence and ADR-002/003/009 decisions; recorded standalone `main` repository choice |

---

**Next Step**: Resolve blocking ADRs, then implement the vertical slice in
`docs/repository-audit.md §8`. All implementation must follow the normative
precedence in §1.4.
