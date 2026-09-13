# ADR-009: Operational payment-platform ledger boundary

**Status:** Accepted  
**Date:** 2026-09-13  
**Approved by:** Repository owner

## Context

The planning documents describe Stripe-like payments, merchant balances,
settlement, reconciliation, disputes, and financial reports. Those concerns can
be mistaken for a complete merchant general ledger or for a clone of Stripe's
product. That ambiguity would expand the accounting model, compliance claims,
and task surface before the operational ledger is correct.

## Decision

The first release is a payment-platform **operational ledger**:

- The ledger core owns immutable, per-asset balanced postings; platform books;
  merchant payable balances (including pending, available, and reserved
  dimensions); holds; and strongly consistent balance reads.
- Payment orchestration owns intents, authorizations, captures, refunds,
  disputes, payouts, provider calls, and their workflow state machines. These
  aggregates reference ledger postings but do not own journal mutation.
- Settlement and reconciliation compare immutable provider/bank facts with
  internal postings and create controlled, linked adjustments. They never edit
  history.
- Reports describe the platform's operational books and merchant liabilities.
  Merchant GAAP/IFRS books, tax preparation, and a full general-ledger product
  are separate scope unless explicitly added through a new decision.
- The design is Stripe-inspired for operational behavior, not a claim of
  compatibility, parity, or access to Stripe internals.

## Consequences

- Domain and API contracts stay focused on payment operations and ledger
  correctness rather than an unbounded accounting suite.
- Integrations and workflow state can evolve independently from immutable
  postings.
- Compliance and reporting documentation must state the operational scope and
  avoid unsupported GAAP/IFRS or provider-compatibility claims.
- Adding merchant accounting or another product boundary requires a new ADR and
  explicit task/dependency updates.

## Alternatives considered

- **Build a complete merchant general ledger in the first release:** rejected;
  it materially expands accounting, tax, and compliance obligations.
- **Clone Stripe's public and private behavior:** rejected; the project needs a
  well-defined operational contract, not an unverifiable compatibility claim.
- **Keep the boundary implicit in examples:** rejected; scope must be enforced
  by the normative ledger contract and task packets.

## Traceability

- Normative contract: `docs/ledger-core.md §1`
- Feature scope: `docs/fintech-ledger-features.md §1–§2`
- Workflow and application tasks: `tasks/epics/E03-money-movement.md`,
  `tasks/epics/E04-compliance.md`, and `tasks/epics/E06-application.md`
