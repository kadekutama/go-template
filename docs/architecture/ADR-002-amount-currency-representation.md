# ADR-002: Amount and currency representation

**Status:** Accepted  
**Date:** 2026-09-13  
**Approved by:** Repository owner

## Context

The ledger handles multiple fiat and non-fiat assets, cross-currency trades,
fees, reversals, and aggregate reporting. Floating-point values and implicit
currency exponents can introduce rounding drift, permit unlike assets to be
netted, and make a replay produce a different result from the original write.
The original feature plan also needs a representation that is safe at API,
database, and JavaScript boundaries.

## Decision

- The domain `Money` value object stores a checked `int64` `amount_minor` and an
  explicit `AssetCode`. Amounts are positive on entries; checked arithmetic
  returns an error on overflow or an asset mismatch.
- Asset metadata is supplied by a versioned registry containing code, exponent,
  kind, and activation dates. A test fixture may seed examples, but an
  incomplete hard-coded ISO list is not the authority.
- PostgreSQL entry amounts use `BIGINT`; aggregate and report calculations use a
  wider accumulator such as `NUMERIC(38,0)`. Values outside the supported range
  are rejected rather than rounded or wrapped.
- Public JSON uses integer `amount_minor` plus `currency` and applies a bound
  below JavaScript's unsafe-integer limit. A retained legacy `amount` field has
  the same minor-unit meaning. Decimal display strings are derived only.
- FX rates use fixed-point decimal values with source, quote time, rate ID, and
  rounding policy. Rounding happens once at a declared boundary; deterministic
  largest-remainder allocation records any remainder.
- Double-entry balancing is performed independently for each asset code. An FX
  trade uses linked currency lots rather than a cross-currency net total.

## Consequences

- Arithmetic and replay are deterministic across Go, PostgreSQL, and API
  clients.
- Currency metadata changes are versioned and auditable instead of silently
  changing historical meaning.
- API clients must handle integer minor units and explicit currency/asset codes.
- Reporting code needs wider accumulators and explicit conversion/rounding
  policies; it cannot use a single mixed-currency total.

## Alternatives considered

- **`float64` or implicit decimal amounts:** rejected because precision,
  overflow, and replay behavior are not suitable for money.
- **Arbitrary-precision decimal for every stored entry:** rejected for the
  first ledger kernel; it adds storage and comparison complexity without
  improving the bounded minor-unit contract. Wider decimal accumulators remain
  available for aggregates.
- **A single global currency list in code:** rejected; asset metadata and
  activation are versioned domain data.

## Traceability

- Normative contract: `docs/ledger-core.md §4–§6.5`
- Domain implementation: `tasks/epics/E02-ledger-domain.md` (E02-T02, E02-T04)
- API semantics: `SPEC.md §13.1` and `docs/api-contracts.md`
