# Clean Architecture Skill

> Apply this skill inside the task packet and takeover protocol in `tasks/SDD.md`.

## Core Principle
**Dependencies point inward** - Inner layers know nothing of outer layers.

```
Interface Layer (cmd/, pkg/)
       │
       ▼
Application Layer (internal/application/)
       │
       ▼
Domain Layer (internal/domain/) ◄──┐
       ▲                           │
       │                           │
Infrastructure Layer ──────────────┘
(internal/infrastructure/)
```

## Layer Responsibilities

### Domain Layer (Center)
- **Pure business logic** - Zero external dependencies
- **Types**: Entities, Value Objects, Aggregates, Domain Events
- **Interfaces**: Repository ports, Domain Service interfaces
- **Domain Specifications**: Executable business rules (Specification pattern)
- **No**: Database, HTTP, Config, ambient logging, or ambient/system time. Use
  injected ports for clocks and logging; domain value objects may use `time.Time`
  when the timestamp is supplied by the caller.

### Application Layer
- **Use Cases / Orchestration** - CQRS Commands & Queries
- **DTOs** - Data Transfer Objects for boundaries
- **Ports** - Inbound (Use Cases), Outbound (Repository interfaces)
- **Services** - Application services (orchestrate domain objects)
- **Workflows** - Sagas, long-running processes
- **No**: HTTP, Database specifics, UI logic

### Infrastructure Layer
- **Adapters** - Implement domain/application interfaces
- **Database**: GORM repositories, Migrations
- **Cache**: Ristretto, Valkey, Hybrid
- **Messaging**: NATS Publisher/Consumer
- **Auth**: JWT, OAuth2, RBAC, API Keys
- **Config**: koanf loading, validation
- **Observability**: OTel, `log/slog`, Prometheus
- **Resilience**: Circuit Breaker, Retry
- **Secrets**: Bitwarden SDK
- **Feature Flags**: OpenFeature + Unleash

### Interface Layer (Delivery)
- **Entry Points** - Multiple binaries in `cmd/`
  - `rest-api` - Echo HTTP server
  - `grpc-api` - gRPC server
  - `graphql-api` - gqlgen server
  - `cron` - Distributed scheduler
  - `consumer` - NATS consumers (separate cluster)
- **Protocol Adapters** - Translate protocol → Application commands/queries
- Private handlers live in `internal/interface/{rest,grpc,cron,consumer}`;
  reusable transport/server primitives live in `pkg/`.
- **Middleware** - Auth, Rate Limit, Correlation ID, Logging, Tracing

## Dependency Injection (fx)
```go
// Each layer registers its own module
var DomainModule = fx.Options(
    // Aggregates are constructed per command, not registered as singletons.
)

var ApplicationModule = fx.Options(
    fx.Provide(NewOpenAccountHandler),
    fx.Provide(NewAccountService),
)

var InfrastructureModule = fx.Options(
    fx.Provide(NewPostgresDB),
    fx.Provide(NewPostgresAccountRepository), // Implements domain.AccountRepository
    fx.Provide(NewValkeyClient),
    fx.Provide(NewJWTManager),
)

// Main wires them together
func main() {
    app := fx.New(
        DomainModule,
        ApplicationModule,
        InfrastructureModule,
        fx.Invoke(RegisterRoutes), // Side effects
    )
    app.Run()
}
```

## Key Rules

1. **Domain imports nothing external** - Only stdlib
2. **Application imports Domain only** - No infrastructure
3. **Infrastructure imports Domain + Application** - Implements interfaces
4. **Interface imports all** - Wires via DI
5. **No circular imports** - Enforced by `go mod verify` and linting
6. **Interfaces in Domain/Application** - Define contracts
7. **Implementations in Infrastructure** - Fulfill contracts

## Testing Strategy by Layer
| Layer | Test Type | Dependencies |
|-------|-----------|--------------|
| Domain | Unit (pure) | None |
| Application | Unit (mock ports) | Mockery-generated mocks |
| Infrastructure | Integration | Testcontainers (real DB, Valkey, NATS) |
| Interface | Contract/E2E | Running services |

## References
- Clean Architecture (Robert C. Martin)
- SPEC.md Sections 3, 4, 15
