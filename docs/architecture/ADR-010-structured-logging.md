# ADR-010: Structured logging API

**Status:** Accepted  
**Date:** 2026-09-11

## Context

The template needs structured JSON logs with request, trace, and tenant
correlation while keeping the domain independent of a logging backend. Echo v5
uses the standard-library `log/slog` API, and the project targets Go 1.27.1.
The previous design named zerolog as the default even though the kernel already
requires a swappable logging port.

## Decision

Define a small `Logger` port in `internal/shared/kernel/log`. The default
adapter is `log/slog` with JSON output and UTC/RFC3339Nano timestamps. Other
backends may be added only behind the same port and only with compatibility and
benchmark evidence. Domain code does not import `log/slog` or any third-party
logger. Context attributes are allow-listed and secrets/PII are redacted before
they reach the handler.

## Consequences

- Echo, workers, and adapters share one structured logging contract.
- Unit tests can use a discard or in-memory adapter without global state.
- A logger migration is one adapter plus composition-root binding, not a domain
  rewrite.
- `slog` becomes a target dependency only through the Go toolchain; no external
  logger is required for the first vertical slice.

## Alternatives considered

- **zerolog as default:** rejected; it creates an unnecessary external default
  and conflicts with Echo v5's native logging API.
- **Direct logger calls in every package:** rejected; it violates dependency
  inversion and makes redaction/correlation inconsistent.
