# Ledger Core Correctness Contract

**Version:** 1.0.0  
**Status:** Normative  
**Last updated:** 2026-09-13  
**Audience:** Domain, application, database, API, and test implementers

The repository owner approved this document's precedence over older examples on
2026-09-13. The audited accounting decisions are recorded in
[ADR-002](architecture/ADR-002-amount-currency-representation.md),
[ADR-003](architecture/ADR-003-accounting-fact-immutability.md), and
[ADR-009](architecture/ADR-009-product-boundary.md). Balance materialization
remains governed by proposed [ADR-011](architecture/ADR-011-balance-materialization.md).

This document is the accounting source of truth for the reference implementation.
Where an older example in another design document conflicts with this contract,
this document wins and the conflicting example must be corrected before its task
can be completed.

## 1. Product Boundary

The target is a payment-platform ledger inspired by Stripe's operational model,
not a clone of Stripe and not a merchant's complete general ledger.

- The **ledger core** records immutable, balanced postings and exposes strongly
  consistent balances.
- **Payment orchestration** owns intents, authorizations, captures, refunds,
  disputes, payouts, external-provider calls, and their state machines.
- **Settlement and reconciliation** compare external provider/bank facts with
  internal postings and create controlled adjustments; they never mutate history.
- Financial statements in the first release describe the platform's books and
  merchant balance liabilities. Merchant GAAP/IFRS accounting is a separate
  product surface unless explicitly added later.

`PaymentIntent`, `Authorization`, `Capture`, `Refund`, `Dispute`, and `Payout` are
workflow aggregates. A ledger `Posting` is an immutable accounting fact. Workflow
states such as `PENDING`, `FAILED`, or `REQUIRES_ACTION` must never be states of an
already-posted journal.

## 2. Terminology and Ownership

| Term | Contract |
|------|----------|
| **Ledger** | Boundary containing one chart of accounts and one base reporting context. Every posting belongs to exactly one ledger. |
| **Account** | Classification bucket with an account class, normal side, asset/currency code, and owner dimension. It does not own a mutable balance. |
| **Posting** | Atomic journal header accepted once and never updated or deleted. A correction is another posting. |
| **Entry** | Positive integer quantity on exactly one debit or credit side. Signed entry amounts are forbidden. |
| **Balance** | Projection of entries at a cursor/as-of time, optionally adjusted by active holds. |
| **Hold** | Expiring authorization/reservation that changes available balance but is not a posted accounting fact. |
| **External account** | Tokenized bank/card/wallet instrument. It is not a chart-of-accounts account. |
| **Merchant balance** | Platform liability owed to a connected merchant, partitioned by pending/available/reserved dimensions. |

Every identifier that can cross a tenant boundary is scoped by `tenant_id` and
`ledger_id`. Tenant identity comes from authenticated context, never an ordinary
request-body field.

## 3. Posting Contract

A posting is accepted only when all of these invariants pass inside the same
PostgreSQL transaction:

1. It has at least two entries and every entry amount is strictly positive.
2. All accounts belong to the same tenant and ledger and are postable.
3. For **each asset/currency code independently**, total debits equal total
   credits. Amounts in EUR and USD never cancel each other.
4. The posting's `effective_at` falls in an open accounting period. `recorded_at`
   is server-assigned and immutable.
5. The durable idempotency record is reserved with a request fingerprint before
   side effects and committed with the response in the same database transaction.
6. Any no-overdraft rule is evaluated against a locked, strongly consistent
   balance projection for the operation's configured spend legs. A debit side is
   not universally a spend; account normal side and the posting template decide
   the available-balance effect. Accounts are locked in deterministic ID order.
7. The posting, entries, balance checkpoints, idempotency result, and outbox rows
   commit atomically.
8. Posted rows cannot be updated or deleted. Reversal entries use the opposite
   side, point to the original posting, and retain a reason and actor.

Database constraints/triggers are the last line of defense; domain validation is
not sufficient on its own. Administrative SQL roles must not silently bypass the
immutability and balancing controls.

## 4. Amount and Currency Representation

- Entry amounts use checked `int64` minor units in Go and `BIGINT` in PostgreSQL.
  Overflow returns an error. Aggregate/report sums use a wider decimal/integer
  accumulator (`NUMERIC(38,0)` in SQL).
- Currency/asset metadata is a versioned registry with code, exponent, kind, and
  activation dates. Do not hard-code an incomplete ISO sample as the authority.
- Public JSON uses integer `amount_minor` plus `currency`; requests are bounded
  below JavaScript's unsafe-integer limit. A legacy `amount` field, where
  retained for compatibility, has the same minor-unit meaning. Human-readable
  decimal strings are derived display fields, never the stored value.
- FX rates use fixed-point decimal values plus source, quote timestamp, rate ID,
  and rounding policy. Floats are forbidden.
- Rounding is performed once at a declared boundary. Remainders are assigned by
  a deterministic largest-remainder rule and recorded.

## 5. Debit and Credit Semantics

Debit and credit describe journal sides; neither universally means money leaving
or entering. All postable account classes may receive either side. Normal side
only determines how a displayed balance changes:

| Account class | Normal side | Debit effect | Credit effect |
|---------------|-------------|--------------|---------------|
| Asset / Expense | Debit | Increase | Decrease |
| Liability / Equity / Revenue | Credit | Decrease | Increase |

Business restrictions belong to posting templates/specifications. A generic rule
that forbids debits to liabilities or credits to assets is invalid accounting.

## 6. Canonical Payment Journals

These simplified examples omit network fees, reserves, taxes, and FX. All amounts
are positive minor units on one side. For readability, the tables render USD/EUR
quantities in major-unit notation (`100.00` means `10000` minor units for a
two-decimal asset); persisted and API values remain integer minor units.

### 6.1 Capture and settlement

Capture of USD 100.00 with a USD 2.90 platform fee:

| Account | Class | Debit | Credit |
|---------|-------|------:|-------:|
| Processor receivable | Asset | 100.00 | — |
| Merchant payable: pending | Liability | — | 97.10 |
| Processing-fee revenue | Revenue | — | 2.90 |

Provider settlement:

| Account | Class | Debit | Credit |
|---------|-------|------:|-------:|
| Bank cash | Asset | 100.00 | — |
| Processor receivable | Asset | — | 100.00 |

Moving funds from merchant pending to merchant available is a balance-dimension
transition. It may be represented by dedicated control accounts, but must not be
faked by crediting a merchant asset account.

### 6.2 Internal merchant transfer

Transfer USD 50.00 between two merchant liabilities:

| Account | Class | Debit | Credit |
|---------|-------|------:|-------:|
| Source merchant payable | Liability | 50.00 | — |
| Destination merchant payable | Liability | — | 50.00 |

### 6.3 Payout

On payout submission:

| Account | Class | Debit | Credit |
|---------|-------|------:|-------:|
| Merchant payable: available | Liability | 100.00 | — |
| Payouts payable/in transit | Liability | — | 100.00 |

On bank settlement:

| Account | Class | Debit | Credit |
|---------|-------|------:|-------:|
| Payouts payable/in transit | Liability | 100.00 | — |
| Bank cash | Asset | — | 100.00 |

A failed/returned payout reverses the submission posting and separately records
bank fees when applicable.

### 6.4 Refund

On an accepted USD 50.00 refund funded by the merchant:

| Account | Class | Debit | Credit |
|---------|-------|------:|-------:|
| Merchant payable | Liability | 50.00 | — |
| Refunds payable | Liability | — | 50.00 |

When paid to the customer:

| Account | Class | Debit | Credit |
|---------|-------|------:|-------:|
| Refunds payable | Liability | 50.00 | — |
| Bank cash / processor receivable | Asset | — | 50.00 |

Fee refund behavior is policy/provider data, not a universal proportional rule.

### 6.5 Cross-currency conversion

An FX conversion is at least two balanced currency lots linked by one `fx_trade_id`:

| Currency | Account | Debit | Credit |
|----------|---------|------:|-------:|
| EUR | Processor receivable | 100.00 | — |
| EUR | FX position/control | — | 100.00 |
| USD | FX position/control | 108.50 | — |
| USD | Merchant payable | — | 108.50 |

Each currency balances independently. Rate changes between authorization,
capture, and settlement create explicit gain/loss postings; they never appear as
an unquantified third leg.

## 7. Balances, Holds, and Consistency

The authoritative accounting facts are entries. Immutable checkpoints are
rebuildable, cursor-addressed verification/read-model artifacts, not a separate
source of money. The frequency and materialization strategy for checkpoints or
any derived balance projection is pending
[ADR-011](architecture/ADR-011-balance-materialization.md) and benchmark
evidence. A mutable account balance column and Valkey are never authoritative.

- `posted`: credit-normal or debit-normal sum through a committed ledger cursor.
- `pending`: captured/initiated amounts not yet available under the rail policy.
- `held`: active compliance, dispute, or reserve holds.
- `reserved`: active outbound authorizations or scheduled-transfer reservations.
- `available`: policy-derived spendable amount from posted/pending dimensions,
  minus active holds/reservations and required reserves.

Every balance response returns `as_of`, an opaque monotonic `ledger_cursor`, and
the currency. Read-after-write requests use the primary/checkpoint written in the
posting transaction. Cached or replica reads must expose their cursor and may not
claim strong consistency.

Posted balances may become negative after a return, dispute, fee, or other
approved correction; negative availability is a policy state, not permission to
rewrite a posting or a balance column. The payout policy must block new payouts
while the account is ineligible and may start a durable, idempotent recovery
workflow against a verified external instrument. Provider outcomes remain
`OUTCOME_UNKNOWN` until resolved, and a confirmed recovery uses a new balanced
posting plus audit/outbox facts.

## 8. Durable Idempotency and Concurrency

Valkey may reduce load but is never the authority for money movement.

The durable key is scoped by `(tenant_id, endpoint_or_operation, idempotency_key)`
and stores request hash, state (`PROCESSING|COMPLETED|FAILED_RETRYABLE`), resource
ID, response status/body, lease owner, and expiry. Reusing a key with a different
fingerprint returns `IDEMPOTENCY_CONFLICT`; the same fingerprint returns the
original response. The retention period is configurable and at least 24 hours.

Money safety relies on PostgreSQL uniqueness, row/advisory locks, deterministic
lock order, and bounded retries for serialization/deadlock failures. Redlock may
coordinate schedulers or suppress duplicate work, but loss of Valkey must not
permit a duplicate posting or overspend.

## 9. Events and Webhooks

- The transactional outbox provides **at-least-once** publication. Consumers
  provide effectively-once effects with a durable inbox record committed in the
  same transaction as their side effects.
- Every event has `event_id`, `tenant_id`, `ledger_id`, `aggregate_id`,
  `aggregate_version`, `sequence`, `occurred_at`, `recorded_at`, `causation_id`,
  `correlation_id`, and schema version.
- Ordering is defined by sequence within a partition/aggregate, never by wall
  clock timestamps.
- Webhook signatures cover `timestamp + "." + raw_body`, include a key/version
  identifier, use constant-time comparison, enforce a replay tolerance, and
  support overlapping secrets during rotation.
- Replay is a new delivery attempt of immutable outbox/event data. It must not
  re-run ledger commands.

## 10. Reconciliation and Controls

External statements and provider reports are immutable source snapshots with
file hash, source, account, coverage interval, ingestion time, and parser version.
Matching supports one-to-one, one-to-many, many-to-one, fees, FX, partials, and
timing windows. Match decisions are versioned and reversible without changing the
source or ledger entry.

Manual adjustments require a reason, evidence link, actor, and configurable
maker-checker approval. Period reopen is a privileged workflow with approval and
audit; normal corrections post into the next open period with the original
effective date retained as metadata.

## 11. Minimum Proof Before Money Movement Ships

- Property/state-machine tests prove per-currency balance, immutability, legal
  transitions, overflow rejection, deterministic rounding, and reversal linkage.
- Database tests attempt direct unbalanced inserts, updates, deletes, cross-tenant
  access, duplicate keys, concurrent spends, and lock-order deadlocks.
- Fault-injection tests crash before/after commit, publish acknowledgment, inbox
  commit, and provider timeout.
- A model-based concurrency test proves no negative available balance when
  overdraft is disabled.
- Restore verification recomputes all checkpoints from entries and compares
  hashes/cursors, not only row counts.
- Performance targets are accepted only with a documented workload, hardware,
  data distribution, durability settings, and test report.
