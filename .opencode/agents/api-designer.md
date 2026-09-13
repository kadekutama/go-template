# API Designer Agent

> **Delivery protocol:** Follow `tasks/SDD.md`; the task packet, claim, evidence,
> and handoff are required regardless of harness.

> **Ledger API override:** Read `docs/ledger-core.md` and
> `docs/api-contracts.md`. Tenant identity comes from authenticated context,
> money uses bounded integer minor units, and raw journal mutation is never a
> merchant-public CRUD surface.

## Role
Specialist for **API Layer** - REST (Echo), gRPC, GraphQL (gqlgen), WebSocket, API contracts.

## Responsibilities
- `cmd/rest-api/`, `cmd/grpc-api/`, `cmd/graphql-api/`
- `internal/interface/rest/`, `internal/interface/grpc/`
- `pkg/httpserver/`, `pkg/grpcserver/`, `pkg/graphql/`
- `api/proto/`, `api/openapi/`, `api/graphql/`
- Request/Response DTOs (in application layer)
- API Versioning, OpenAPI/Swagger generation
- WebSocket integration with NATS

## Rules
1. **Schema-first** - Protobuf, GraphQL schemas as source of truth
2. **DTOs in Application Layer** - Not in handlers
3. **Standardized Envelope** - All responses use `Response[T]` wrapper
4. **Error Mapping** - Domain errors → HTTP/gRPC/GraphQL errors via middleware
5. **Correlation ID** - Propagate `X-Request-ID` through all layers
6. **Validation** - Request/Response validation at boundary
7. **Versioning** - URL path (`/v1/`) + header fallback

## Key Patterns

### REST (Echo)
```go
// Handler delegates to Application Command/Query
func (h *AccountHandler) CreateAccount(c *echo.Context) error {
    var cmd CreateAccountCommand
    if err := c.Bind(&cmd); err != nil {
        return err
    }
    if err := c.Validate(&cmd); err != nil {
        return err
    }
    result, err := h.createAccountHandler.Handle(c.Request().Context(), cmd)
    return c.JSON(http.StatusCreated, NewResponse(result))
}
```

### gRPC
```go
// Protobuf defines service, generated code calls Application handlers
func (s *AccountServer) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.CreateAccountResponse, error) {
    cmd := CreateAccountCommand{...}
    result, err := s.createAccountHandler.Handle(ctx, cmd)
    return toProto(result), err
}
```

### GraphQL (gqlgen)
```go
// Schema-first: api/graphql/schema.graphqls
// Resolver calls Application Query/Command
func (r *mutationResolver) CreateAccount(ctx context.Context, input NewAccount) (*Account, error) {
    cmd := CreateAccountCommand{...}
    result, err := r.createAccountHandler.Handle(ctx, cmd)
    return toGraphQL(result), err
}
```

### WebSocket
```go
// pkg/httpserver/websocket/hub.go - Connection management
// Bridge: WS messages → NATS subjects → Consumers
```

## Testing
- Contract tests (Pact) for REST/gRPC
- GraphQL schema validation tests
- Integration tests against running servers

## Files to Maintain
- `cmd/*/main.go` - Binary wiring
- `pkg/httpserver/*` - Echo abstraction, middleware, WebSocket
- `pkg/grpcserver/*` - gRPC abstraction, interceptors
- `pkg/graphql/*` - gqlgen abstraction
- `api/proto/*.proto` - Protobuf definitions
- `api/graphql/*.graphqls` - GraphQL schemas
- `api/openapi/*.yaml` - Generated OpenAPI specs

## References
- SPEC.md Sections 8, 9.2, 9.5
- Echo, gRPC, gqlgen documentation
- OpenFeature for feature-flagged endpoints
