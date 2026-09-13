# ADR-003: Accounting fact immutability

**Status:** Accepted  
**Date:** 2026-09-13  
**Approved by:** Repository owner

## Context

Payment workflows have retries and mutable states such as pending, failed,
requires-action, and returned. A journal posting is a financial fact, however,
and changing or deleting it after acceptance would make balances, audit trails,
reconciliation, and event replay diverge. Treating the workflow transaction and
the accounting journal as one mutable aggregate also makes provider retries
unsafe.

## Decision

- A committed `Posting` header and its positive `Entry` rows are append-only
  accounting facts. They have no pending, failed, voided, or mutable lifecycle
  state after acceptance.
- Payment intents, authorizations, captures, refunds, disputes, payouts, and
  other process state are separate workflow aggregates. They may reference
  postings but cannot mutate them.
- Corrections are new balanced postings with opposite-side entries, a link to
  the original posting, a reason, and an actor. No generic update or delete
  repository method is exposed for accounting facts.
- PostgreSQL constraints/triggers enforce append-only behavior and balancing as
  the final defense; domain validation and application authorization remain
  required. Posting, entries, idempotency outcome, checkpoints, and outbox rows
  commit atomically in one unit of work.
- Balances and checkpoints are rebuildable projections of immutable entries.
  They never become an alternate source of money.

## Consequences

- Replays and reconciliation can recompute history from durable facts.
- Corrections consume additional journal rows and require explicit lineage,
  review, and audit evidence.
- Workflow persistence and journal persistence have different schemas and
  retention/state policies, increasing the number of ports but keeping each
  model honest.
- Database migrations must preserve immutability triggers and test them with
  privileged and ordinary roles.

## Alternatives considered

- **Mutable transaction row with a status and editable amount:** rejected;
  retries or administrative edits can rewrite financial history.
- **Delete and reinsert to correct a posting:** rejected; it breaks lineage and
  external reconciliation.
- **Event sourcing as the only persistence model:** rejected for the first
  release; state-based workflow aggregates plus immutable postings and domain
  events provide the required auditability without making every read a replay.

## Traceability

- Normative contract: `docs/ledger-core.md §1`, `§3`, `§7–§9`
- Domain implementation: `tasks/epics/E02-ledger-domain.md` (E02-T04)
- Application/persistence boundaries: `tasks/epics/E06-application.md` (E06-T06,
  E06-T13) and `tasks/epics/E07-persistence.md` (E07-T01)
