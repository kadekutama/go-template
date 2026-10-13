# ADR-025: Casbin v2.8.0 → v3.11.0 Authorization Pin (Path-Only Major)

**Status:** Proposed
**Date:** 2026-10-04
**Owner:** E09.1-T06
**Related tasks:** `tasks/specs/E09.1-T06.md`

## Context

Upstream Casbin v2 is critical-fixes-only; v3 is the maintained line. Per the
upstream v2→v3 migration guide, v3 is a module-path major with identical
APIs, model/policy syntax, and adapter interfaces. This repository uses no
external Casbin adapters (the custom `db_loader.go` builds plain-string
rules), so no adapter migration exists.

## Decision

Pin `github.com/casbin/casbin/v3 v3.11.0` (latest on the Go module proxy as
of 2026-10-04, published 2026-06-23) and swap only import paths
(`casbin/v2` → `casbin/v3`, `v2/model` → `v3/model`) in
`internal/infrastructure/auth/rbac/enforcer.go` and the
`internal/shared/deps` anchor. Zero logic change.

Behavior is frozen by construction and proven by a checked-in,
table-driven parity test: the v2.8.0 authorization decisions for a fixed
tenant/role/allow/deny corpus (role grants, tenant isolation, explicit
deny, unassigned subjects) were captured before the swap into
`internal/infrastructure/auth/rbac/testdata/authorization-parity.json`
(with the exact v2.8.0 module checksum), and every v3 decision must equal
the recorded baseline. Any allow/deny delta is a migration bug, not an
accepted change. Model text, loader, `CasbinRuleModel`, migration DDL, and
stored policy rows are unchanged (verified by `git diff --exit-code` plus
the existing RBAC/loader suites); no data migration exists.

## Consequences

- `go build ./...` green on the v3 pin with no `/v2` import remaining.
- E10 provider tasks proceed on the v3 pin under the E09.1 hard gate; T06
  follows T05 to serialize the shared module and architecture-index
  surfaces.

## Rollback

Re-pin `github.com/casbin/casbin/v2 v2.8.0` and revert the two import paths.
Policy data needs no rollback (untouched).
