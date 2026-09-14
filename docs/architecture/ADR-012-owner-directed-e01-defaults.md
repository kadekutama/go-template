# ADR-012: Owner-directed E01 defaults (Sonic codec, UUIDv7 IDs, zerolog backend)

**Status:** Accepted  
**Date:** 2026-09-14  
**Approved By:** repository owner (directed 2026-09-14, supersedes ADR-010 default-backend choice)

## Context

During review of the E01 platform-core branch (2026-09-14), the repository
owner issued three explicit directives that override prior spec defaults:

1. Use Sonic for JSON (owner missed the stdlib-first row in spec review and
   strictly wants Sonic).
2. Use `google/uuid` instead of `oklog/ulid` for IDs.
3. Use zerolog as the logging backend (owner's standing requirement).

Directive 3 conflicts with accepted ADR-010, which rejected zerolog-as-default
in favor of `log/slog`. Per the ADR index policy, an accepted ADR is immutable:
this record supersedes ADR-010's default-backend choice (ADR-010 keeps its port
design, which is unchanged — only the default adapter changes).

## Decision

1. **JSON codec:** `pkg/jsonparser` keeps its API; the backend becomes
   `sonic.ConfigStd` (stdlib-compatible profile) at Sonic v1.15.4 — v1.12.0
   (the SPEC-named candidate) does not link against Go 1.27's stdlib
   (`encoding/json.unquoteBytes` relocation removed upstream), so the pin
   moved to the newest verified-compatible release. The benchmark is still
   recorded in E01-T07 evidence (owner waives the must-beat gate, not the
   measurement). Owning packet: `tasks/specs/E01-T07.md`.
2. **IDs:** `kernel.IDGenerator` is implemented with UUIDv7
   (`github.com/google/uuid` v1.6.0), preserving the time-ordering intent as
   closely as UUID allows. Same-millisecond monotonicity (previously proven via
   ULID's shared entropy source) is knowingly lost: UUIDv7 same-ms ordering is
   random. Uniqueness + version bits are tested instead. Owning packet:
   `tasks/specs/E01-T06.md`.
3. **Logging backend:** the slog adapter is replaced with a zerolog adapter
   behind the UNCHANGED `log.Logger` port. There is deliberately no slog hop:
   our port is a custom interface (not the slog API), and no `zerolog/slog`
   bridge ships in zerolog v1.35.1 — routing port → slog API → bridge →
   zerolog would be ceremony without behavioral gain. zerolog's shape globals
   (`MessageFieldName="msg"`, RFC3339Nano, UTC) are set once under `sync.Once`
   in the adapter constructor (same sanctioned-global treatment as OTel).
   Echo-v5/slog interop, if E11 needs it, uses stdlib slog directly for Echo
   internals. Owning packet: `tasks/specs/E01-T04.md`.

## Consequences

- `SPEC.md §2` (JSON, Logging rows), `§7.9`, `§7.10` (Logging/ID/JSON rows),
  and `§17` (JSON decision) are updated to these defaults.
- ADR-010 status becomes `Superseded` for the default-backend choice; its port
  design stands and is implemented unchanged.
- `go.mod`: `+ bytedance/sonic v1.15.4`, `+ rs/zerolog v1.35.1`,
  `google/uuid` indirect → direct v1.6.0, `- oklog/ulid/v2`.
- E11 (Echo middleware) and E09 (crypto) are unaffected structurally; E11 must
  read the adapter's JSON shape note (zerolog field conventions) when parsing logs.
