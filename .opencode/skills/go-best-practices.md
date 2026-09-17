# Go Best Practices Skill

> Apply this skill inside the task packet and takeover protocol in `tasks/SDD.md`.

## Code Style
- **Formatting**: `gofmt` / `goimports` (enforced by CI)
- **Linting**: `golangci-lint` with strict config (`.golangci.yml`)
- **Naming**: 
  - Packages: short, lowercase, no underscores (`userrepo`, not `user_repo`)
  - Interfaces: define small consumer-owned interfaces; implementations use
    descriptive names such as `postgresAccountRepository`, not an `Impl` suffix
  - Errors: `Err` prefix for sentinel errors (`ErrNotFound`), `Error` suffix for types (`ValidationError`)

## Error Handling
- **Never ignore errors** - Always handle or wrap
- **Sentinel errors** for expected failures: `ErrNotFound`, `ErrConflict`
- **Error wrapping**: `fmt.Errorf("context: %w", err)` (Go 1.13+)
- **Custom error types** for programmatic handling: `AppError` with codes

## Concurrency
- **Context everywhere** - Pass `context.Context` as first param
- **Worker pools** - Use `errgroup` or bounded channels
- **Graceful shutdown** - `fx` lifecycle hooks + signal handling
- **No global state** - Use DI (fx) for all dependencies

## Dependencies
- **Minimal external deps** - Prefer stdlib
- **Version pinning** - Exact versions in `go.mod`
- **Vendor** - Not used (Go modules)
- **Private modules** - `GOPRIVATE` for internal packages
- **Toolchain & Dependency Bootstrap** - Run `./scripts/dev/setup.sh` (or `make setup`) to automatically install Pixi, toolchains (Go, Clang, Docker, Make), and Go developer tools.

## Testing
- **Table-driven tests** - Standard pattern
- **Testify** for assertions (`require`, `assert`, `mock`)
- **Mockery** for interface mocks (generate: `make generate-mocks`)
- **Testcontainers** for integration tests
- **Parallel tests** - `t.Parallel()` where safe

## Performance
- **Profiling** - `pprof` endpoints in debug mode
- **Benchmarks** - `testing.B` for hot paths
- **Allocation tracking** - `go test -bench=. -benchmem`
- **JSON**: Prefer the standard library at boundaries; if performance testing
  justifies Sonic, hide it behind a small application/infrastructure codec port
  so domain code remains independent of the serializer.

## Security
- **No secrets in code** - Use OpenBao (Dynamic DB credentials & Transit encryption)
- **Input validation** - All boundaries (validator.v10)
- **SQL injection** - GORM or explicitly parameterized SQL only; never build
  query text from untrusted values.
- **Dependencies** - `govulncheck`, `gosec` in CI

## Project Structure
```
internal/          # Private - not importable by other modules
pkg/               # Public - reusable by other modules
cmd/               # Main packages (binaries)
api/               # Contracts (proto, openapi, graphql)
config/            # Configuration files
deployments/       # Docker, K8s, Helm
scripts/           # Automation
test/              # Test organization
docs/              # Documentation
```

## FX Dependency Injection
```go
// Module per layer
var DomainModule = fx.Options(
    // Aggregates are constructed per command, not registered as singletons.
)

var ApplicationModule = fx.Options(
    fx.Provide(NewOpenAccountHandler),
    fx.Invoke(RegisterAccountRoutes), // For side effects (route registration)
)

// App construction
app := fx.New(
    DomainModule,
    ApplicationModule,
    InfrastructureModule,
    fx.NopLogger, // Or custom logger
)
```

## Configuration (koanf)
```go
var k = koanf.New(".")
k.Load(file.Provider("config.yaml"), yaml.Parser())
k.Load(env.Provider("APP_", ".", func(s string) string {
    return strings.ToLower(strings.ReplaceAll(s, "_", "."))
}), nil)
// Struct binding with validation
var cfg Config
k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"})
validate.Struct(cfg)
```

## References
- [`docs/development/go-conventions.md`](../../docs/development/go-conventions.md)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Go 1.27 Release Notes](https://go.dev/doc/go1.27)
- [Effective Go](https://go.dev/doc/effective_go)
