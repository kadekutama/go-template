# ADR-021: Zero Silent Defaults, Fail-Fast Validation

**Status:** Proposed
**Date:** 2026-09-27
**Note:** Proposed for repository owner review and acceptance. Doctrine for the
Round-2 revision program (plan `tasks/plans/E09-round2-revision-plan.md`).

## Context

Across epics E01–E09 the tree accumulated silent defaults: package-level fallback
constants (`DefaultTTLs()`, `DefaultRefreshRatio`, `DefaultPartitions`), zero-filling
`Normalize()`/`withDefaults()` helpers, environment-overlay constructors, and
clamping helpers that coerced invalid input into valid-looking output
(`clampPageLimit`, `DefaultConfig = Config{Level: "info"}`). Each default is a
configuration the operator never chose and cannot observe: unconfigured services
booted and ran on values nobody approved — wrong retry budgets, wrong TTLs, wrong
broker topology, info-level logging in production.

## Decision

1. **Zero silent defaults.** No package-level fallback constants, no zero-filling
   normalization, no environment-overlay constructors. Every operational parameter
   (timeouts, TTLs, topics, partitions, retry budgets, log levels, excluded routes,
   key-derivation costs) must be explicitly supplied and validated at construction;
   constructors fail fast with descriptive errors otherwise.
2. **Reject, don't coerce.** Invalid input is rejected with a stable error code
   (`INVALID_PAGE_LIMIT`, `INVALID_AUTH_EXPIRY`, `INVALID_REFUND_WINDOW`); clamping
   helpers are deleted. Apparent convenience of clamping hides caller bugs and splits
   the contract between documentation and behavior.
3. **Fail fast with clean operator diagnostics.** Missing configuration fails at
   boot/wiring time with a human-readable message and exit code 1 — never a Go
   runtime panic, never a silent degraded mode. (Panic sites in `di.ProvideLogger` /
   `FxLogger` are reworked to this shape in P0-1.)
4. **Explicit non-exemptions.** The doctrine covers serializers (`hybrid.Codec` is
   required, not defaulted to JSON), hash framing, and test doubles: no category
   gets a silent default by being "merely" polymorphic or test-only.
5. **Migrations with behavior change are documented.** Known deltas riding this
   doctrine: blank SASL mechanism + credentials now errors (was SCRAM-SHA-512);
   pagination clamp→reject (see `docs/api-contracts.md` §5); `RetryPolicy.BackoffFor`
   signature change to `(time.Duration, error)`; `etcd` `APP_COORDINATION__*` env
   overlay removed in favor of explicit wiring.

## Consequences

- Every owning packet in E01–E09 carries a dated amendment block recording its
  surface delta (see plan Appendix A); this ADR is the single doctrine reference.
- Wiring (fx modules, `cmd/*`, compose/K8s manifests, `.env` examples) must supply
  every required value; `.env.example` comments are corrected to match.
- Test suites construct explicit configs (no ambient defaults); `t.Setenv` hygiene
  applies wherever env is read.
- Residual risk: stricter boot increases misconfiguration failures at deploy time —
  accepted deliberately, mitigated by descriptive errors and documented manifests.
