# ADR-027: Continuous Profiling (`grafana/pyroscope` v2.4.0) & Offline PGO (`google/pprof`)

**Status:** Proposed
**Date:** 2026-10-16
**Owner:** E09.2-T04, E15-T07, E19-T05
**Related tasks:** `tasks/specs/E09.2-T04.md`, `tasks/specs/E15-T07.md`, `tasks/specs/E19-T05.md`

## Context

The observability specification (`SPEC.md §2`, `§7.9`, `§12.4`) defines OpenTelemetry traces (Tempo), metrics (Prometheus), and structured logs (Loki) visualized in Grafana (the LGTM stack), and `E19-T03` requires benchmark profiling and Go Profile-Guided Optimization (`default.pgo`) to certify the 10K TPS / `p99 < 50ms` posting NFR (`docs/fintech-ledger-features.md §14`).

However, two gaps existed in the profiling architecture:
1. **Continuous Runtime Profiling & Trace-to-Profile Correlation**: Distributed traces in Tempo show *which* span was slow, but not *why* the Go runtime spent CPU cycles, allocated heap memory, or blocked on mutex/channel contention inside that span. Grafana Pyroscope (`v2.4.0`) paired with `github.com/grafana/pyroscope-go` (`v1.4.3`) and `github.com/grafana/otel-profiling-go` (`v0.7.0`) bridges OpenTelemetry spans directly to CPU, heap, goroutine, mutex, and block profiles (`LGTMP` stack).
2. **Offline Deep-Dive Analysis & Go 1.27 PGO Pipeline**: Engineers and CI jobs need `github.com/google/pprof` (`v0.0.0-20261014150452-c3b4e57cab2c`) + `graphviz` to inspect benchmark `.pprof` artifacts (`go test -bench=... -cpuprofile -memprofile -mutexprofile`) and compile production binaries with `-pgo=auto` backed by a validated `default.pgo` profile, while ensuring `net/http/pprof` is never exposed on public ingress.

## Decision

1. **Adopt Grafana Pyroscope Go SDK (`v1.4.3`) + OTel Profiling Bridge (`v0.7.0`) in `internal/infrastructure/profiling/` (`E09.2-T04`)**:
   - Pin `github.com/grafana/pyroscope-go v1.4.3` and `github.com/grafana/otel-profiling-go v0.7.0` in `go.mod`.
   - Encapsulate the continuous profiler behind the Parameter Object pattern (`ProfilerParams` + `NewProfiler`) in `internal/infrastructure/profiling/`, managed by `fx.Lifecycle` (`OnStart` starts the profiler and sets runtime mutex/block sampling fractions only when enabled; `OnStop` flushes and stops cleanly).
   - Wrap the OpenTelemetry `TracerProvider` (`internal/infrastructure/tracing/otel.go`) with `otelpyroscope.NewTracerProvider` when profiling is enabled so active root spans automatically receive `pyroscope.profile.id` attributes without leaking Pyroscope SDK types into `internal/domain/` or `internal/application/`.
   - Keep continuous profiling **disabled by default** (`observability.profiling.enabled: false`) so unit tests and local development incur zero network or sampling overhead unless explicitly enabled.
   - Enforce a strict **low-cardinality, zero-PII tag policy**: static profiler tags are limited to `service`, `env`, `region`, and `version`. Tenant IDs, account IDs, and user identifiers must never be attached as Pyroscope profiler tags.
2. **Extend the Observability Stack to LGTMP (`grafana/pyroscope:2.4.0`) and Isolate `net/http/pprof` (`E15-T07`)**:
   - Add `pyroscope` (`grafana/pyroscope:2.4.0`, port `4040`) to `deployments/docker/docker-compose.observability.yml` (`SPEC.md §12.4`) and provision the Grafana Pyroscope datasource with Tempo `tracesToProfiles` correlation.
   - Provide an optional, default-off (`observability.pprof.enabled: false`), loopback-only (`127.0.0.1:6060`) diagnostic HTTP server in `internal/infrastructure/profiling/pprof_server.go` for on-demand `runtime/pprof` / `net/http/pprof` capture. Public REST (`Echo`), gRPC, GraphQL, and Traefik ingress routers **must never** mount `/debug/pprof`.
3. **Standardize `google/pprof` (`v0.0.0-20261014150452-c3b4e57cab2c`) + `graphviz` for Benchmark Profiling & PGO (`E19-T05`)**:
   - Install `github.com/google/pprof@v0.0.0-20261014150452-c3b4e57cab2c` (`GO_TOOLS`) and `graphviz` (`PIXI_PACKAGES`) in `scripts/dev/setup.sh`.
   - Add `make bench-profile` and `make pgo-profile` targets that generate CPU, heap, and mutex profiles from ledger hot-path benchmarks and produce validated `default.pgo` profiles for Go 1.27 PGO release builds.

## Consequences

- Operators gain 1-click drill-down from slow Tempo spans in Grafana to flamegraphs in Pyroscope (`v2.4.0`).
- Zero impact on domain/application purity or default test execution (default-off configuration, clean `fx.Lifecycle` teardown).
- Benchmark profiles feed directly into Go 1.27 PGO (`-pgo=auto`) in `E19-T03`/`E19-T05`.

## Rollback

Set `observability.profiling.enabled: false` and `observability.pprof.enabled: false` in configuration, or remove `internal/infrastructure/profiling` from `internal/shared/di/infrastructure.go` and revert the `go.mod` pins.
