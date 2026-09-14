# Epic E01: Platform Core (fx, Config, Logging, Tracing, Shared Kernel)

**Status:** completed
**Story Points:** 17
**Phase:** 1
**Dependencies:** E00
**SDD Gate:** G1
**Design refs:** `SPEC.md §3.2` (dependency rule), `SPEC.md §7.1` (config),
`SPEC.md §7.9` (observability), `SPEC.md §9.1` (errors+i18n), `SPEC.md §9.2`
(envelope), `SPEC.md §9.5` (correlation), `SPEC.md §9.6` (shutdown),
`SPEC.md §7.10` (ports — every adapter below implements a port defined here or in domain/app)

> Why: everything else imports the kernel and wires through fx. Getting the
> ports, error model, and lifecycle right first prevents rewrites in E04–E14.

## Tasks

### E01-T01: Pin go.mod dependencies to SPEC §2 versions
**Status:** completed
**Background:** All later code compiles against these exact versions
(Go 1.27.1, fx v1.24.0, Echo v5.3.1, GORM v1.31.2, go-redis v9.22.0,
OTel v1.26.0, and the codec wrapper policy). Version drift is the top cause of
"works on my machine" failures.
**Files:**
- Modify: `go.mod`, `go.sum`
**Steps:**
1. Add every production + test dependency from `SPEC.md §2` with exact versions.
2. Run `go mod tidy && go mod verify`.
3. Run `govulncheck ./...`; upgrade or pin-away anything flagged.
**Acceptance Criteria:**
- [ ] `go mod tidy && go mod verify` pass.
- [ ] `govulncheck ./...` reports zero known vulnerabilities.
- [ ] `tasks/scripts/check-tasks.py --versions` passes (checks SPEC §2 table vs go.mod).
**Story Points:** 2
**Depends On:** E00-T01
**Related Docs:** `SPEC.md §2`, `SPEC.md §14`
**SDD Gate:** G1

---

### E01-T02: fx module registry (no global state)
**Status:** completed
**Background:** Runtime DI with lifecycle (`SPEC.md §3.2`). Every layer registers
an `fx.Option`; each `cmd/*/main.go` composes them. No globals, no init().
**Files:**
- Create: `internal/shared/di/{domain.go,application.go,infrastructure.go,rest.go,grpc.go,graphql.go,cron.go,consumer.go}`
**Steps:**
1. Define one `fx.Option` per file using only `fx.Provide`/`fx.Invoke`.
2. Wire `fx.NopLogger` for tests; real logger (E01-T04) for binaries.
3. Each `cmd/*/main.go` builds `fx.New(<needed modules>)` and calls `app.Run()`.
4. Add a test that constructs the full graph (`fx.ValidateApp`) to catch cycles early.
**Acceptance Criteria:**
- [ ] `go test ./internal/shared/di/ -run TestGraphValidates` passes (no cycles).
- [ ] No package-level `var` holding dependencies (`grep -rn "^var .*=" internal/shared/di/` empty).
**Story Points:** 2
**Depends On:** E01-T01
**Related Docs:** `SPEC.md §3.2`, `SPEC.md §8`
**SDD Gate:** G1

---

### E01-T03: Layered configuration with validation (koanf)
**Status:** completed
**Background:** Single validated `Config` struct loaded base → env → local →
env-vars → secrets (`SPEC.md §7.1`). Fails fast on missing required fields.
**Files:**
- Create: `internal/infrastructure/config/{config.go,loader.go}`, `config/config.yaml`,
  `config/config.local.yaml` (gitignored sample), `config/config.staging.yaml`,
  `config/config.production.yaml`, `config/schemas/config.json`
**Steps:**
1. Define `Config` struct matching `SPEC.md §7.1` (App, Server, Database, Cache,
   Auth, NATS, Observability, FeatureFlags, Secrets) with `koanf` + `validate` tags.
2. Implement loader priority chain; `{{ secret:... }}` references resolve via E09's
   SecretManager interface (stub that errors clearly until E09 lands).
3. Validate with `validator/v10`; print all violations, exit non-zero.
**Acceptance Criteria:**
- [ ] App boots with `config/config.yaml` alone; missing required field → clear error listing every violation.
- [ ] `APP_DATABASE__HOST`-style env overrides work (documented in file header).
- [ ] `config/schemas/config.json` validates the YAML (`make validate-config`).
**Story Points:** 3
**Depends On:** E01-T01
**Related Docs:** `SPEC.md §7.1`, `SPEC.md §2` (koanf v2.3.0, validator v10.22.0)
**SDD Gate:** G1

---

### E01-T04: Structured logging port + slog adapter
**Status:** completed
**Background:** JSON logs with correlation IDs (`SPEC.md §7.9`, `§9.5`). The
`Logger` port lives in the kernel so domain/app never import a logging backend
(`SPEC.md §7.10`).
**Files:**
- Create: `internal/shared/kernel/log/{logger.go,context.go}`,
  `internal/infrastructure/logging/{slog_adapter.go,config.go}`
**Steps:**
1. Define `Logger` interface (Debug/Info/Warn/Error + `With(...)`) and
   `WithContext`/`FromContext` helpers carrying `request_id`, `trace_id`, `tenant_id`, `user_id`.
2. Implement with `log/slog`: JSON output, RFC3339Nano timestamps, level from config;
   expose an adapter seam if a benchmark later justifies another backend.
3. Provide via fx; `fx.NopLogger`-equivalent `Discard()` for unit tests.
**Acceptance Criteria:**
- [ ] Log line contains `request_id`/`trace_id` when present in context (unit test).
- [ ] Swapping `slog` for another implementation requires only a new adapter + one fx line (review check).
**Story Points:** 2
**Depends On:** E01-T02
**Related Docs:** `SPEC.md §7.9`, `SPEC.md §9.5`, `SPEC.md §7.10`, `docs/development/go-conventions.md`
**SDD Gate:** G1

---

### E01-T05: Tracing port + OpenTelemetry SDK bootstrap
**Status:** completed
**Background:** W3C TraceContext propagation end-to-end (`SPEC.md §7.9`).
Middleware in E11/E12/E13 consumes the `Tracer` port defined here.
**Files:**
- Create: `internal/shared/kernel/trace/{tracer.go,context.go}`,
  `internal/infrastructure/tracing/{otel.go,config.go}`
**Steps:**
1. Define `Tracer`/`Span` interfaces wrapping OTel concepts (start, set attributes, record error, end).
2. Bootstrap OTel SDK: service name/version/env resource attrs, OTLP exporter,
   10% probabilistic sampling (configurable), always-sample errors.
3. Helpers to inject/extract trace context into HTTP headers, gRPC metadata, NATS headers.
4. Health endpoints excluded by default.
**Acceptance Criteria:**
- [ ] Unit test: start span → inject → extract in a fresh context yields same trace ID.
- [ ] Misconfigured endpoint fails fast with a clear error (no silent no-op in prod config).
**Story Points:** 2
**Depends On:** E01-T02
**Related Docs:** `SPEC.md §7.9`, `SPEC.md §2` (OTel v1.26.0)
**SDD Gate:** G1

---

### E01-T06: Shared kernel — errors, i18n, pagination, lifecycle helpers
**Status:** completed
**Background:** The error envelope, translated messages, shutdown and goroutine
helpers every epic depends on (`SPEC.md §9.1`, `§9.2`, `§9.6`, `§9.7`).
**Files:**
- Create: `internal/shared/kernel/error/{app_error.go,codes.go,translator.go}`,
  `internal/shared/locale/{en.yaml,id.yaml}`,
  `internal/shared/kernel/pagination/{page.go,cursor.go}`,
  `internal/shared/kernel/shutdown/{shutdown.go}`,
  `internal/shared/kernel/safe/{go.go}`,
  `internal/shared/kernel/{clock.go,id.go}`
**Steps:**
1. `AppError{Code, Message, Details, Cause, HTTPStatus}` + stable uppercase
   snake-case code registry (domain-specific codes may use a
   `DOMAIN_ENTITY_ACTION` prefix); go-i18n YAML translation via context locale.
2. Cursor + offset pagination helpers. Internal handlers use ordinary Go
   `(value, error)` returns; a generic Result/monad is not required.
3. Shutdown helper (signal → drain → close) used by all binaries; `safe.Go()`
   goroutine wrapper that records panics and cancels/fails the owning component
   instead of silently continuing (feeds SPEC §9.7). It must never acknowledge
   a financial mutation as successful after recovery.
4. `Clock` (system/fixed) and `IDGenerator` (ULID) ports for testability.
**Acceptance Criteria:**
- [ ] Unknown error code fails `check-tasks.py --codes` (registry enforced).
- [ ] Translation test: same code renders EN and ID messages via context locale.
- [ ] `safe.Go()` recovers a panicking func in test, records it, and signals the
  owning component failure without acknowledging work.
**Story Points:** 3
**Depends On:** E01-T01
**Related Docs:** `SPEC.md §9.1`, `SPEC.md §9.2`, `SPEC.md §9.5`, `SPEC.md §9.6`, `SPEC.md §9.7`, `SPEC.md §7.10`
**SDD Gate:** G1

---

### E01-T07: JSON codec wrapper package
**Status:** completed
**Background:** Single choke point for JSON so the codec stays swappable. The
standard library is the default; Sonic is optional after compatibility and
benchmark evidence (`SPEC.md §2`).
**Files:**
- Create: `pkg/jsonparser/{json.go,json_test.go}`
**Steps:**
1. Wrap `Marshal/Unmarshal/Get` with an `encoding/json`-compatible API and a
   standard-library implementation; add a Sonic implementation only if the
   benchmark justifies it.
2. Document: all code imports `pkg/jsonparser`, never a concrete codec directly
   (lint rule `depguard` if available).
**Acceptance Criteria:**
- [ ] A repository-wide import check allows codec imports only in
  `pkg/jsonparser`.
- [ ] Round-trip test for integer-minor Money and large payloads passes; a
  benchmark record explains any non-stdlib adapter.
**Story Points:** 1
**Depends On:** E01-T01
**Related Docs:** `SPEC.md §2` (JSON codec policy), `SPEC.md §7.10`,
`docs/development/go-conventions.md`
**SDD Gate:** G1

---

### E01-T08: Resilience primitives — breaker, timeout, and retry policy
**Status:** completed
**Background:** Every outbound provider/client needs one reviewable resilience
policy. E10 adapters must not each invent retry semantics or retry a possibly
committed money operation blindly (`SPEC.md §7.8`, `docs/ledger-core.md §8`).
**Files:**
- Create: `internal/shared/kernel/resilience/{breaker.go,policy.go}`,
  `internal/infrastructure/resilience/{gobreaker.go,retry.go}`
**Steps:**
1. Define a small `Breaker` port and `RetryPolicy`/error-classification
   contract owned by the inner layer; propagate context deadlines and never
   hide provider-specific error meanings.
2. Implement one gobreaker adapter per dependency with closed/open/half-open
   transitions, metrics hooks, bounded backoff, and an explicit rule that
   non-idempotent operations require a durable/provider idempotency key before
   retry.
3. Add deterministic fake/clock tests for open/half-open recovery, timeout,
   cancellation, retry exhaustion, and no-blind-retry behavior.
**Acceptance Criteria:**
- [ ] Breaker state transitions and retry classification pass unit tests with a fixed clock.
- [ ] A non-idempotent call without an idempotency key is never retried (test).
- [ ] E10 provider adapters can swap the breaker through a port without importing gobreaker.
**Story Points:** 2
**Depends On:** E01-T02, E01-T06
**Related Docs:** `SPEC.md §7.8`, `SPEC.md §7.10`, `docs/ledger-core.md §8`, `docs/development/go-conventions.md`
**SDD Gate:** G1

## Acceptance Criteria

- [x] E01-T01 … E01-T08 all `completed` (count 17 SP in `tasks/tracking/PROGRESS.md`; T04/T06/T07 reworked 2026-09-14 per ADR-012, re-verified)
- [x] `make build-all` + `make lint` green; `fx.ValidateApp` passes (no cycles)
- [x] Invalid config fails with every violation listed; no global state in `di/`
- [x] SDD gate G1 checks pass — `tasks/tracking/GATES.md#G1` (`tasks/scripts/gate-check.sh G1`)
