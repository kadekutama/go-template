# ADR-023: gocron-ui Read-Only Security Boundary

**Status:** Proposed
**Date:** 2026-10-04
**Owner:** E09.1-T02
**Related tasks:** `tasks/specs/E09.1-T02.md`

## Context

`github.com/go-co-op/gocron-ui v0.3.0` ships a useful scheduler dashboard, but
its upstream defaults are unsafe for us: the router is unauthenticated,
`CheckOrigin` allows all origins, CORS allows `*` with credentials, the
README omits the registered `POST /api/jobs` mutation route, and the router
exposes direct scheduler controls (`POST /api/jobs/{id}/run`,
`DELETE /api/jobs/{id}`, `POST /api/scheduler/{start,stop}`). Raw `RunNow`,
job deletion/creation, and scheduler start/stop can bypass durable run keys,
leader coordination, and fencing rules. Constructing the upstream server also
starts a broadcaster goroutine. Upstream `WithAPIDisabled()` /
`WithWebSocketDisabled()` cannot be used: they would also remove the
`GET /api/config`, `GET /api/jobs`, `GET /api/jobs/{id}` reads and `/ws`
updates the read-only dashboard needs.

## Decision

Mount the pinned upstream handler (`server.NewServer(scheduler, 0).Router`,
built once per enabled process in `internal/infrastructure/cronui`) only
behind the narrow wrapper in `internal/interface/cron/admin`, which is the
security boundary — never the upstream options:

1. **Session boundary.** Same-origin `POST /admin/session` validates an
   existing E09 bearer JWT, authorizes `cron:read`, and mints only a
   down-scoped `cron:read` token (TTL ≤ 5 minutes and ≤ source-token expiry)
   in a `__Host-cron_ui_session` cookie (`Secure`, `HttpOnly`,
   `SameSite=Strict`, `Path=/`, no `Domain`). The source token is never
   stored, logged, or returned. `DELETE /admin/session` clears the cookie.
   Bootstrap/logout require the exact configured origin (CSRF).
2. **Read-only allow-list.** Reachable: session bootstrap/logout, embedded
   UI assets, `GET /api/config`, `GET /api/jobs`, `GET /api/jobs/{id}`, and
   `/ws`. Denied before upstream invocation: `POST /api/jobs`,
   `POST /api/jobs/{id}/run`, `DELETE /api/jobs/{id}`,
   `POST /api/scheduler/{start,stop}`, and every unknown `/api/*` path.
   Denied scheduler-control attempts record exactly one audit event without
   credentials or payloads; audit failure fails closed. Unauthenticated read
   denials (routine 401s) are not audit events.
3. **Same-origin defense.** Exact Host/Origin checks run before the upstream
   handler; the wrapper strips `Origin` on a request clone and suppresses all
   upstream `Access-Control-*` response headers, so wildcard CORS cannot
   authorize cross-origin access. Upstream WebSocket `CheckOrigin` is never
   relied upon; `/ws` requires the exact origin.
4. **Internal listener.** Disabled by default: no upstream instance,
   listener, or broadcaster goroutine exists. When enabled, loopback bind is
   the default; non-loopback requires explicit private-listener mode with
   verified mTLS client certificates (plus the E17 private-network policy).
   E11 must not mount this handler and public ingress must not route to it.
5. **Rate limiting.** After Host/Origin validation every request consults the
   E08-T07 limiter with positive configured budget/window. Keys use E05-T03
   `BuildRateLimitKey("cron-ui", opaqueSubjectID)` for canonical opaque
   non-PII actor IDs (1–128 bytes, no `:`, no surrounding whitespace, no
   control characters); anything else uses the fixed `anonymous` bucket and
   is denied. Denial returns 429 + `Retry-After` without invoking the
   authorizer or upstream; store failure returns 503. Denied mutations are
   audited even when rate-limited. Forwarded IP headers never form keys.
6. **Layering.** The interface package depends only on narrow consumer-owned
   seams plus `port.RateLimiter` and the E05 key builder; no Casbin/JWT/audit
   concretes and no gocron-ui types cross into it.

## Consequences

- Operators with `cron:read` can view schedules over a same-origin session;
  all mutations fail closed and are audited; cross-origin access is rejected
  with no wildcard CORS emitted.
- Any future manual-run capability needs a separate safe E14 control contract
  and task; this surface stays read-only.
- E14-T01 owns starting/shutting down the optional listener in `cmd/cron`
  with `cron_admin.enabled: false` by default.

## Rollback

Set `cron_admin.enabled: false` (default): the wrapper and upstream instance
are not constructed and no listener exists. No data migration is involved.

## E14 Wiring (Remaining Bindings)

The UI-side pieces are built, tested, and exemplified
(`internal/infrastructure/cronui/`, `compose_example_test.go`, `cron_admin`
config shape). Binding them in `cmd/cron` still needs decisions owned
elsewhere — deliberately not invented here:

1. **Scheduler**: `gocron.NewScheduler()` plus E14's election, jobs, and
   lifecycle (E14-T01).
2. **Bearer trust**: which key material the cron binary verifies E09 JWTs
   against (shared secrets backend vs JWKS sync). File-based trust would
   need E09 support for public-only key entries or shipping private keys
   (rejected); the port-level `BearerTokens` adapter is ready.
3. **Operator policy content**: the allowlist rows (subjects) live in an
   operator-owned file; the fixed internal domain/object mapping is
   documented here but the tenant/policy model stays E09-T03-owned.
4. **Audit store**: the admin seam accepts any `AuditLogger`; until E14
   binds the chain logger, a structured-log adapter suffices for an
   internal read-only surface.
5. **Start/stop lifecycle** with the cron process (E14-T01).
