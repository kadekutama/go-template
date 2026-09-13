# Repository Audit: Stripe-Style Ledger Readiness

**Audit date:** 2026-09-11  
**Scope:** All design documents, hidden `.opencode` agent/skill/permission assets,
task definitions, validation scripts, and current repository contents  
**Verdict:** Planning-ready after the correctness revisions in this audit, but
not implementation-ready; the original accounting model would have permitted
materially incorrect postings and the runtime has not been bootstrapped.

## 1. Executive Assessment

The repository has unusually broad planning coverage: stack selection, API
surfaces, flows, events, journeys, 20 epics, 140 tasks, 431 story points, and eight
gates. The task validator passes structural, spec, event, handler, port, subject,
link, code, and format checks; ADR validation correctly fails because four
decisions are still pending. There is no implementation, Go module, migration,
OpenAPI file, or generated schema yet, so build/runtime/security claims remain
unproven.

The largest risk is conceptual rather than mechanical. The original documents
model a merchant balance as an asset, use debit/credit as send/receive labels,
allow workflow states on immutable transactions, balance unlike currencies
together, and make Valkey part of the money-safety boundary. Those choices are
not safe foundations for a payment ledger. `docs/ledger-core.md` now defines the
normative replacement.

### Readiness scorecard

| Area | Before audit | Assessment |
|------|-------------:|------------|
| Repository/bootstrap | 1/5 | Documentation-only; README quick start cannot run. |
| Accounting model | 1/5 | Critical debit/credit, ownership, lifecycle, and FX errors. |
| API product model | 2/5 | Broad endpoints, but inconsistent money types and unsafe tenant/key behavior. |
| Persistence/concurrency | 2/5 | RLS/outbox/locking planned; database-enforced ledger invariants were absent. |
| Events/webhooks | 3/5 | Good catalog/outbox intent; invalid Go sample, timestamp ordering, and signature/replay gaps. |
| Reconciliation/operations | 3/5 | Strong breadth; missing immutable sources, many-to-many matching, and adjustment controls. |
| Security/compliance | 2/5 | Useful checklist, but compliance is claimed without scope/evidence/control ownership. |
| Testing/delivery | 3/5 | Good gates and fault categories; financial model/concurrency proofs needed strengthening. |

## 2. Release-Blocking Findings

| ID | Finding | Risk | Required disposition |
|----|---------|------|----------------------|
| C-01 | Inbound payment credits a merchant `ASSET`; source/destination transfer directions conflict with normal-side rules. | Balances can move in the opposite economic direction. | Use merchant payable liabilities and canonical journals from `ledger-core.md §6`. |
| C-02 | `ValidAccountType` forbids debits to liability/revenue and credits to assets/expenses. | Legitimate payout, refund, settlement, reversal, and correction entries become impossible. | Remove as a universal invariant; validate approved posting templates instead. |
| C-03 | A `Transaction` is both immutable journal and mutable payment/auth state machine. | History can be mutated or failed intents can masquerade as accounting facts. | Separate immutable Posting/Entry from workflow aggregates. |
| C-04 | Cross-currency example compares EUR debit with USD credit and an unspecified gain/loss leg. | Journal is not balanced in any meaningful unit. | Balance each currency independently and link lots with an FX trade/rate ID. |
| C-05 | Mutable account balance fields and cache TTL reads are treated as authoritative. | Lost updates and stale spend decisions can create money. | Entries/checkpoints in PostgreSQL are authoritative; caches are labeled projections only. |
| C-06 | Valkey `SETNX`/Redlock is the primary idempotency/concurrency mechanism. | Eviction, expiry, partition, or failover can duplicate money movement. | Durable DB idempotency with request fingerprint; DB uniqueness/locking for funds. |
| C-07 | The schema has no database-level balanced-posting or immutability enforcement. | A bug/admin/ad-hoc writer can insert one-sided or mutate posted data. | Add staged posting procedure/constraint triggers and deny UPDATE/DELETE. |
| C-08 | Outbox and cron gates promise “exactly once.” | This cannot be guaranteed across DB, broker, provider, and process failures. | Specify at-least-once delivery plus durable inbox/effect dedupe; jobs idempotent by run key. |
| C-09 | The event base Go sample uses field and method names that collide (`EventID`, etc.). | It will not compile. | Use distinct fields/accessors and add compile tests. |
| C-10 | Webhook HMAC signs only the body while timestamp is a separate unsigned header. | Captured payloads can be replayed. | Sign timestamp plus raw body; tolerance, key ID, and dual-secret rotation. |
| C-11 | Tenant provisioning response returns a live secret API key. | Secrets may leak through response logs, telemetry, or replay. | Create keys through a separate endpoint; show secret once, redact everywhere, audit access. |
| C-12 | README says production-ready and exposes runnable commands, but no code or those files exist. | Misleads implementers and reviewers. | Mark planning-only and make commands explicitly future-state until G1. |
| C-13 | README declared MIT but no `LICENSE` file exists. | Distribution rights are ambiguous. | Remove the claim; select/add a license before distribution. |
| C-14 | `IsSatisfiedBy()` followed by state-free `Errors()` cannot report only the failed composed rules; a stateful implementation would race when singleton specs are reused. | Wrong errors or data races under concurrent requests. | Replace with stateless `Evaluate(candidate) SpecResult`; define all-errors versus short-circuit combinators explicitly. |
| C-15 | “SDD” meant both the domain Specification pattern and test gates, with no task packet, claim, evidence, or takeover contract. | Different harnesses can implement different behavior or lose state at handoff. | Make `tasks/SDD.md` normative and keep harness profiles as thin adapters. |
| C-16 | The task graph contained four E02 cycles, and E07 tests depended on a shared harness scheduled later in E16. | No agent could legally reach some tasks; integration setup would be duplicated or invented. | Enforce acyclic dependencies, move the harness before consumers, and schedule vertical slices. |
| C-17 | `go-template/` has no resolvable baseline commit or origin. | Claims, diffs, reviews, and cross-harness takeovers cannot identify shared state safely. | The owner selected a standalone repository with `main` as its default branch; E00-T00 must establish writable Git metadata, the shared baseline, and claim-serialization policy before implementation claims. |

## 3. High-Priority Missing Capabilities

### Ledger primitives and controls

- Explicit `ledger_id`, legal entity/owner, chart version, account purpose, and
  currency/asset registry.
- Posting templates for capture, settlement, fee, refund, dispute, reserve,
  payout, return, top-up, FX, and manual adjustment.
- `effective_at`, `recorded_at`, monotonic ledger cursor, aggregate version,
  reversal link, and reason/actor fields.
- Durable holds/reservations with expiry and release idempotency.
- Trial-balance/checkpoint recomputation and a suspense-account policy with SLO.
- Maker-checker control for manual journals, period reopen, write-off, and break
  resolution. No direct CRUD over posted ledger facts.

### Payment operations

- Provider object mapping and webhook dedupe keyed by provider + account + event.
- Explicit unknown outcome state after provider timeout; never blindly retry a
  possibly successful charge/refund/payout without provider idempotency/status lookup.
- Rail-specific returns, dispute stages, fee-refund policy, settlement batches,
  reserves, negative-balance recovery, and payout trace IDs are now explicitly
  owned by E03-T06/E03-T11; they still require implementation and proof.
- Separate platform fees from processor/network fees and taxes.

### Reconciliation and treasury

- Immutable statement/provider-report snapshots with checksums and parser versions.
- One-to-many and many-to-one match groups, partial settlement, fee/FX matching,
  confidence, rule version, and reversible match decisions.
- Bank cash/processor receivable/payable control-account reconciliation, not only
  matching arbitrary ledger entries.
- Intraday reconciliation, aging, break ownership, evidence, and suspense aging.

### Security and compliance

- Threat model and trust boundaries; service identity and authorization for each
  command, not only HTTP middleware.
- PCI scope decision, token-provider boundary, data classification, retention
  legal holds, key rotation/crypto-shredding limitations, and access reviews.
- Signed audit checkpoints/WORM export. “Hash chained” alone does not prevent a
  privileged writer from rewriting the chain.
- Compliance claims expressed as control objectives with owner, evidence, cadence,
  and applicable jurisdiction—not “compliant” feature labels.

## 4. Cross-Document Consistency Findings

- `SPEC.md` originally duplicated heading `3.2` and displayed a dependency arrow
  from Domain to Infrastructure, contradicting Clean Architecture; both are fixed.
- Hidden OpenCode agent/skill examples repeated the mutable-balance,
  repository-aware idempotency, GORM-only write-path, and invalid account-side
  rules even after top-level docs changed; their banners/examples/permissions
  are now aligned with the normative ledger contract.
- The five agent roles did not cover several epics, and their permission file did
  not allow the status edits required by `tasks/README.md`. `AGENTS.md` now maps
  every epic to a role/coordinator and the role permissions allow the assigned
  epic plus progress updates.
- Several documents linked to `../fintech-ledger-features.md`, although the file
  is in the same `docs/` directory; those links are fixed.
- REST previously mixed decimal strings (`"500.00"`) and integer minor units
  (`10000`), while gRPC used a decimal string. The revised contracts use integer
  minor units/`int64` consistently; generated schemas must enforce this.
- Idempotency is said to require UUIDv4, while examples use semantic keys such as
  `pi_create_abc123`.
- `X-Request-ID` is both required and “generated if missing.” It should be optional
  from untrusted clients, validated if supplied, and always returned.
- Domain events say minimal payload, then include full entries and sensitive
  idempotency/metadata fields. Public webhooks need a separately versioned,
  allow-listed projection.
- Outbox ordering by `created_at` does not prove commit or aggregate order;
  webhook ordering by event time is equally unsafe.
- Report events were exempted from the outbox because reports are reproducible;
  reproducibility does not make notification delivery atomic.
- The proto excerpt is not a compilable contract (missing imports/messages and
  incomplete parity), while the document calls it source of truth.
- Generic ledger accounts and external bank accounts are conflated by the
  microdeposit `/accounts/{id}/verify` routes.
- “General ledger, GAAP/IFRS statements” and “Stripe-like operational ledger” are
  mixed without declaring which entity's books are produced.

## 5. Task, SDD, and Gate Audit

A second pass focused on cross-harness takeover found that the backlog was
detailed but not yet executable as a dependency graph: E02 contained four
aggregate/specification/port cycles, and the shared Testcontainers harness was
scheduled after the persistence suite that required it. “SDD” also ambiguously
meant both the domain Specification pattern and phase-level test gates; there was
no durable task packet, claim, evidence record, or takeover protocol.

Those delivery defects are now corrected:

- `tasks/SDD.md` defines a harness-neutral packet/claim/evidence/handoff lifecycle,
  normative precedence, high-risk review rule, and takeover procedure.
- `check-tasks.py --graph --sdd` rejects dependency cycles, impossible active
  ordering, progress drift, and missing lifecycle artifacts.
- E02's graph is acyclic; the shared integration harness moved from late E16 to
  E07-T09 before adapter suites.
- The 8-point all-schema migration task is split into ledger-core and mutable
  workflow/control-plane migrations.
- Core ports, core posting/balance use cases, the HTTP server core, and a
  restricted ledger pilot are separated from broad provider/public middleware.
- `tasks/DELIVERY-SLICES.md` replaces horizontal phase execution with incremental
  financial proofs while retaining epics for ownership/reporting.

Accounting and reliability revisions remain:

- E02 must implement Posting/Entry, holds, account normal-side display, per-currency
  balancing, checked minor units, and posting-template specs.
- E07 must implement durable idempotency, balance checkpoints, immutable source
  records, database enforcement, and at-least-once outbox semantics.
- E08 Redlock is coordination-only; webhook signing and durable consumer inboxes
  are strengthened.
- E14 jobs are idempotent by durable run key, not described as exactly-once.
- G2/G4/G5 add adversarial accounting, concurrency, inbox, and replay checks.

## 6. Architecture, CQRS, Go, and AI-readiness audit

### SOLID and Clean Architecture

The revised design is directionally sound, with one important boundary: SOLID
is a set of tests for coupling and substitutability, not a requirement to add
interfaces everywhere. Domain and application packages own small ports;
infrastructure adapters implement them; protocol handlers translate DTOs and do
not make accounting decisions. The old generic `Save`/`Delete` repository and
`*Impl` examples have been replaced with intent-specific account metadata ports
and an explicit immutable-posting commit path. This protects Interface
Segregation and Liskov substitution while keeping the ledger write boundary
reviewable. The rules are centralized in
[`docs/development/go-conventions.md`](development/go-conventions.md).

### CQRS

CQRS is now defined as separate application command and query contracts. A
command may mutate state, publish through an outbox, and must honor durable
idempotency. A query is side-effect-free from the caller's perspective and
returns a projection with cursor/as-of semantics when it reads a replica or
cache. CQRS does not require a second database, event sourcing, or a workflow
state on an immutable Posting. The compatibility `transactions` API maps to
`PostLedgerPosting`; payment, transfer, refund, dispute, and payout lifecycles
remain separate aggregates.

### Current Go practice

Go 1.27.1 is the toolchain target and was the current stable patch on the audit
date ([Go release history](https://go.dev/doc/devel/release)), but the repository
has no `go.mod`, so the version and dependency pins are not yet machine-proven.
New code is required to use `gofmt`, context-aware I/O,
wrapped/inspectable errors, `log/slog`, bounded goroutines, race/vet/staticcheck/
govulncheck checks, and a codec wrapper rather than direct serializer imports.
Echo v5 handlers use the concrete `*echo.Context` signature. These are target
conventions until E00/E01 create and verify the module.

### AI-readiness verdict

This is **planning-ready but not implementation-ready**:

- **Ready:** normative precedence, ledger contract, task DAG, vertical slices,
  packet/claim/evidence/handoff templates, structural validator, ownership map,
  and OpenSpec/Spec Kit interoperability guidance are repository-visible.
- **Not ready:** no Git baseline commit, Go module, executable code, generated
  schemas, migrations, or passing runtime tests. E00-T00 must establish the
  repository boundary and baseline; E00-T08 must make the SDD control plane
  executable before parallel implementation claims are trusted.

No additional epic is needed. The existing 140 tasks (20 epics, 431 SP) are
acyclic and each is at most five story points. The first implementation order is
the vertical sequence in `tasks/DELIVERY-SLICES.md`; activate a task only after
its packet is ready and claim it before editing. Add a new task only when
convergence finds a genuinely new behavior or an out-of-scope gap—never widen a
claimed task silently.

ADR-002/003/009 are owner-approved (2026-09-13) and now have permanent ADR
records. The four unresolved choices remain deliberate blockers: ADR-006 before reconciliation,
ADR-007 before FX integration, ADR-008 before archival, and ADR-011 before E07
chooses a production balance materialization strategy.

## 7. Claims That Must Be Proven, Not Presumed

- `10,000 TPS`, `p99 < 50 ms`, `99.99%`, `RPO < 5 min`, and `RTO < 30 min`
  are targets. They require a workload model, topology, durability settings,
  fault domain, and measured evidence.
- “Production-ready,” “GAAP/IFRS compliant,” “SOX compliant,” “PCI DSS,” and
  “OWASP compliant” are not valid repository statuses at planning phase.
- Active/passive writes conflict with a vague per-tenant multi-region routing
  design until failover authority, fencing, data residency, and split-brain
  behavior are specified.
- GORM is acceptable for metadata CRUD, but the ledger posting path should use an
  explicit transaction/query layer or stored procedure whose SQL and lock order
  are reviewable. ORM hooks are not the financial integrity boundary.

## 8. Recommended Build Slice

Do not start with five binaries and three public APIs. The safest first vertical
slice is:

1. One ledger, one tenant, one currency registry, chart/accounts, immutable
   posting/entry, durable idempotency, outbox, and balance checkpoints.
2. PostgreSQL constraints plus unit/property/concurrency/fault tests.
3. One internal posting API and one read API; no GraphQL mutations and no direct
   merchant raw-journal API.
4. Capture → settlement → refund → payout templates with a fake processor and
   bank report reconciliation.
5. Only after correctness gates pass: multi-currency/FX, disputes, tenancy/RLS,
public webhooks, scale tests, and additional protocol surfaces.

The exact task-level sequence is maintained in `tasks/DELIVERY-SLICES.md`.

## 9. Audit Evidence

Commands executed during the audit:

```text
python3 tasks/scripts/check-tasks.py
# 140 tasks, 20 epics, 431 SP; structural checks passed

python3 tasks/scripts/check-tasks.py --format --graph --sdd --specs --events --handlers \
  --ports --subjects --docs --codes --adrs --breakers --openapi \
  --migrations --dirs --versions
# all documentation-era checks passed except the expected four pending ADRs;
# implementation-dependent checks skipped because no code exists
```

Version spot checks against official release pages confirmed Go 1.27.1,
PostgreSQL 18.6, Echo v5.3.1, and NATS 2.14.6 as of the audit date. Other pinned
dependencies still require machine-verifiable module/container resolution in G1.

## 10. Primary Version/API Sources

- [Go downloads](https://go.dev/dl/)
- [PostgreSQL versioning policy](https://www.postgresql.org/support/versioning/)
- [Echo releases](https://github.com/labstack/echo/releases)
- [NATS Server releases](https://github.com/nats-io/nats-server/releases)
- [Current nats.go JetStream API](https://github.com/nats-io/nats.go/blob/main/jetstream/README.md)
- [OWASP Top 10:2025](https://top10.owasp.org/2025/)
- [PCI DSS overview and scope](https://www.pcisecuritystandards.org/standards/pci-dss/)
