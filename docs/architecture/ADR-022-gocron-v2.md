# ADR-022: gocron v1.5.0 → v2.22.0 Upgrade + E14 Construction Contract

**Status:** Proposed
**Date:** 2026-10-04
**Owner:** E09.1-T01
**Related tasks:** `tasks/specs/E09.1-T01.md`, `tasks/epics/E14-workers.md#E14-T01`

## Context

`go.mod` pinned deprecated `github.com/go-co-op/gocron v1.5.0`. E14 has not yet
written cron code; the scheduler pin must move to a supported v2 API before
E14-T01 builds the `cmd/cron` binary, leader election composition, and the 8
scheduled jobs. Owner decision: no Resty is introduced; stdlib transport stays.
`robfig/cron/v3` remains an indirect-only dependency (v2 still parses cron
expressions with it).

## Decision

Pin `github.com/go-co-op/gocron/v2 v2.22.0` as the direct scheduler dependency.
`internal/shared/deps/deps.go` blank-imports the `/v2` path to retain the pin
until E14 imports the scheduler through production code. `SPEC.md §2` and `§8.4`
state the v2 pin. `tasks/epics/E14-workers.md` already references v2.22.0 and
is verified read-only here; no E14 job logic is changed.

### v1 → v2 API deltas (E14 implementer contract)

| Concern | v1 (`github.com/go-co-op/gocron`) | v2 (`github.com/go-co-op/gocron/v2`) |
|---------|-----------------------------------|--------------------------------------|
| Scheduler creation | `gocron.NewScheduler(timezone)` returns concrete `*Scheduler` (no error) | `gocron.NewScheduler(options ...SchedulerOption) (Scheduler, error)` — error MUST be handled; scheduler is unstarted until `Start()` |
| Job definition | `s.Every(d).Do(fn, args...)` / `s.Cron(expr).Do(fn)` chained builder | `s.NewJob(gocron.DurationJob(d), gocron.NewTask(fn, args...))` or `s.NewJob(gocron.CronJob(expr, withSeconds), gocron.NewTask(fn, args...))`; returns `(Job, error)` |
| Task wrapper | Raw func passed to `Do` | `gocron.NewTask(function any, parameters ...any) Task`; first `context.Context` param receives the job context (canceled on shutdown) |
| Start | `s.StartAsync()` / `s.StartBlocking()` | `s.Start()` (non-blocking); jobs added via `NewJob` run only after `Start()` |
| Stop | `s.Stop()` / `s.Clear()` | `s.Shutdown() error` — terminal; scheduler cannot be restarted after shutdown; tests MUST defer/call it on every constructed scheduler |
| Job identity | `job.GetName()` / internal tags | `j.ID() uuid.UUID` — durable run-key and idempotent step-key composition in E14-T01 MUST key off this ID plus the occurrence time, never off builder state |

### E14 construction contract (copy-paste starter)

```go
import gocron "github.com/go-co-op/gocron/v2"

// E14-T01 owns the scheduler port/factory; this snippet is the only
// construction pattern an E14 implementer needs (no v1 docs required).
s, err := gocron.NewScheduler()
if err != nil {
    return err
}
j, err := s.NewJob(
    gocron.DurationJob(time.Hour),
    gocron.NewTask(func() {}),
)
if err != nil {
    _ = s.Shutdown()
    return err
}
_ = j.ID()
s.Start()
if err := s.Shutdown(); err != nil {
    return err
}
```

E14-T01 owns: scheduler port/factory, etcd `concurrency.NewElection` primary
with Valkey Redlock fallback, durable run keys, fencing/lease checks, panic
wrapper, health endpoints, and the optional `cron_admin` UI listener
(E09.1-T02). Coordination authority is unchanged: etcd election primary,
Valkey Redlock fallback (ADR-017); the framework swap does not weaken
fencing/run-key rules.

## Consequences

- `go build ./...` green on the v2 pin; repo-wide grep proves no bare v1
  import; `robfig/cron/v3` stays indirect-only.
- E14-T01 starts from this ADR without consulting v1 docs.
- Explicit non-goals: no `gocron-ui` (E09.1-T02), no egress limiter (E09.1-T03),
  no single-flight (E09.1-T04), no Resty, no leader-election behavior change.

## Rollback

Re-pin `github.com/go-co-op/gocron v1.5.0`, revert `deps.go` blank import to
the v1 path, revert `SPEC.md §2` + `§8.4` rows, and delete the v2 contract test.
No data migration is involved.
