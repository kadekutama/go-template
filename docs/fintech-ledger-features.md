# Fintech Ledger - Feature Specification

**Version:** 1.0.0  
**Status:** Design Phase — audited; `ledger-core.md` is normative for accounting  
**Reference:** Stripe Ledger / Plaid Core Architecture  
**Template:** Go Clean Architecture Template (this repo)

---

## 1. Overview

The Fintech Ledger is the **reference implementation** for this template. It
specifies a target production-grade, double-entry bookkeeping system suitable
for:

- Payment processors (Stripe-like)
- Banking cores (Plaid-like)
- Accounting platforms
- Multi-tenant financial SaaS
- Crypto/fiat hybrid ledgers

The first-party scope is a payment-platform operational ledger: platform cash and
receivables, merchant liabilities, fees, settlement, refunds, disputes, payouts,
and reconciliation. Full merchant bookkeeping and certified GAAP/IFRS reporting
are separate scope. See [Ledger Core](./ledger-core.md) and the
[repository audit](./repository-audit.md).

---

## 2. Core Domain Features

### 2.1 Account Management

| Feature | Description | Priority |
|---------|-------------|----------|
| **Account Types** | ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE (policy alignment required) | P0 |
| **Multi-Currency** | ISO 4217 support, per-account currency, FX rates | P0 |
| **Account Hierarchy** | Parent/child accounts, roll-up balances | P1 |
| **Account Status** | ACTIVE, FROZEN, CLOSED, PENDING_VERIFICATION | P0 |
| **Balance Types** | Posted, Available, Pending, Held, Reserved | P0 |
| **Account Metadata** | Custom fields, tags, external IDs | P1 |
| **Account Locking** | PostgreSQL row/advisory locks in deterministic order; distributed locks coordinate only | P0 |

### 2.2 Transaction Processing (Double-Entry)

| Feature | Description | Priority |
|---------|-------------|----------|
| **Immutable Postings** | Append-only Posting/Entry facts; signed checkpoints make tampering detectable | P0 |
| **Double-Entry Enforcement** | For each asset code, sum(DEBIT) == sum(CREDIT) within every committed posting | P0 |
| **Workflow States** | PENDING/FAILED/VOIDED live on payment objects; accepted postings are immutable facts | P0 |
| **Idempotency Keys** | Client-provided, prevents duplicate processing | P0 |
| **Entry-Level Detail** | Per-line: account, debit/credit, positive minor units, asset code, sequence | P0 |
| **Transaction References** | External IDs, correlation IDs, descriptors | P0 |
| **Effective Dating** | Post date vs value date vs processing date | P1 |
| **Batch Transactions** | One posting is atomic across its entries; bulk intake defines explicit item-level status/idempotency | P0 |

### 2.3 Journal & Ledger

| Feature | Description | Priority |
|---------|-------------|----------|
| **General Ledger** | Single source of truth, immutable | P0 |
| **Sub-Ledgers** | Per-tenant, per-currency, per-entity | P1 |
| **Journal Entries** | Grouped transactions with metadata | P0 |
| **Opening Balances** | Day-1 balance import per account (audited, period-open only) | P0 |
| **Period Management** | Open/close periods, prevent backdating | P1 |
| **Reversal Handling** | Automatic reversing entries, audit trail | P0 |

---

## 3. Money Movement Features

### 3.1 Transfers

| Feature | Description | Priority |
|---------|-------------|----------|
| **Internal Transfers** | Between accounts same tenant/ledger | P0 |
| **Cross-Currency Transfers** | With FX rate, gain/loss entries | P1 |
| **Scheduled Transfers** | Future-dated, recurring | P1 |
| **Bulk Transfers** | CSV/API batch, async processing | P1 |
| **Transfer Templates** | Reusable transfer definitions | P2 |

### 3.2 Payments (Inbound/Outbound)

| Feature | Description | Priority |
|---------|-------------|----------|
| **Payment Methods** | ACH, Wire, RTP, Card, Crypto, Wallet | P1 |
| **Payment Initiation** | Request-to-pay, push/pull | P1 |
| **Authorization + Capture** | Authorize now, capture later (full/partial); auth expiry and voids | P0 |
| **SCA / 3-D Secure** | Challenge flows for cards; `requires_action` status; exemption flags | P1 |
| **Statement Descriptors** | Static + dynamic descriptors (22-char network rules validated) | P1 |
| **Settlement Tracking** | Expected vs actual settlement | P1 |
| **Return/Reject Handling** | R-codes, automatic reversal | P1 |
| **Payment Linking** | Link to invoices, orders, subscriptions | P1 |
| **Zero-Decimal Currencies** | JPY-style no-minor-unit currencies; per-currency exponent table | P0 |
| **Metadata Limits** | Max 50 keys; keys ≤40 chars, values ≤500 chars (Stripe-compatible) | P0 |

### 3.3 Disputes & Chargebacks

| Feature | Description | Priority |
|---------|-------------|----------|
| **Dispute Lifecycle** | Opened → evidence due → won/lost → reversal + fee posting | P0 |
| **Evidence Submission** | Document upload, deadlines per network, completeness checks | P0 |
| **Representment** | Re-present disputed charge once with new evidence | P1 |
| **Dispute Fees** | Network fee auto-debited on open, reversed on win | P0 |
| **Fraud Early Warnings** | Pre-dispute alerts that auto-refund when cheaper than fighting | P2 |

### 3.4 Payouts & Bank Accounts

| Feature | Description | Priority |
|---------|-------------|----------|
| **Payout Schedule** | Daily/weekly/monthly per merchant, configurable cutoff | P0 |
| **Payout Minimums** | No payout below minimum (configurable per currency) | P0 |
| **Instant Payouts** | 30-minute settlement with fee, eligible methods only | P1 |
| **First-Payout Hold** | 7-day hold for new businesses; rolling reserve after | P0 |
| **Bank Verification** | Microdeposits (2 small credits + confirm amounts) | P0 |
| **Negative Balances** | Payouts blocked; auto-collection flow to cover negatives | P0 |
| **Top-ups** | Platform/merchant adds funds from bank (reverse payout) | P1 |

### 3.5 Platform Split (Connect-style)

| Feature | Description | Priority |
|---------|-------------|----------|
| **Destination Charges** | Charge on platform, settle net to connected account | P0 |
| **Application Fees** | Platform fee carved out at charge time, posted to platform revenue | P0 |
| **On-behalf-of Settlement** | Settle in the connected account's currency/region | P1 |

---

## 4. Compliance & Regulatory

### 4.1 Audit & Compliance

| Feature | Description | Priority |
|---------|-------------|----------|
| **Immutable Audit Log** | Privileged changes tracked; signed checkpoints/WORM export make tampering detectable | P0 |
| **SOX Control Readiness** | Segregation of duties, control owners, evidence, approval workflows | P1 |
| **PCI DSS Scope Reduction** | Tokenization, no raw PAN storage, documented provider boundary | P1 |
| **Privacy Controls** | Erasure/portability where legally permitted; financial retention and legal holds documented | P1 |
| **AML/KYC Integration** | Transaction monitoring hooks | P1 |
| **Regulatory Reporting** | Call reports, 1099, FATCA, CRS | P2 |

### 4.2 Reconciliation

| Feature | Description | Priority |
|---------|-------------|----------|
| **Daily Reconciliation** | Ledger vs bank statements | P0 |
| **Inter-System Reconciliation** | Core vs sub-ledgers vs external | P1 |
| **Break Detection** | Automated mismatch identification | P1 |
| **Break Resolution Workflow** | Investigation, adjustment, approval | P1 |
| **Reconciliation Reports** | Daily, weekly, monthly, on-demand | P1 |

---

## 5. Multi-Tenancy & Isolation

| Feature | Description | Priority |
|---------|-------------|----------|
| **Tenant Isolation** | Row-level security, separate ledgers | P0 |
| **Tenant Hierarchies** | Parent/child tenants, consolidated views | P1 |
| **Data Residency** | Per-tenant region/db control | P1 |
| **White-Labeling** | Custom branding, domains | P2 |
| **Tenant Onboarding** | Self-serve provisioning, config | P1 |

---

## 6. Reporting & Analytics

### 6.1 Financial Reports

| Feature | Description | Priority |
|---------|-------------|----------|
| **Trial Balance** | Real-time, period-end | P0 |
| **Platform Balance Sheet** | Platform books only; accounting-policy review required | P0 |
| **Platform Income Statement** | Multi-period, comparative; accounting-policy review required | P0 |
| **Cash Flow Statement** | Direct/indirect method | P1 |
| **General Ledger Detail** | Filterable, exportable | P0 |
| **Account Statements** | Per-account, custom periods | P0 |

### 6.2 Operational Reports

| Feature | Description | Priority |
|---------|-------------|----------|
| **Transaction Volume** | By type, currency, tenant | P1 |
| **Settlement Reports** | Expected vs actual | P1 |
| **Fee Revenue** | By payment method, tenant | P1 |
| **Exception Reports** | Failed, reversed, held | P1 |

---

## 7. API & Integration

### 7.1 REST API (Primary)

| Endpoint Group | Operations | Priority |
|----------------|------------|----------|
| **Accounts** | Create/query/configure, balance, hierarchy, statements; no delete with history | P0 |
| **Postings** | Restricted template-based post, query, reverse, export; no update/delete | P0 |
| **Transfers** | Initiate, schedule, batch, status | P0 |
| **Payments** | Initiate, webhooks, returns | P1 |
| **Reports** | Generate, download, schedule | P1 |
| **Reconciliation** | Runs, breaks, resolutions | P1 |
| **Tenants** | Provision, configure, billing | P1 |
| **Disputes** | Open, evidence, representment, close | P0 |
| **Payout Policy** | Schedule, minimums, instant, verification, top-ups | P0 |
| **Search** | Field:value query language over transactions/accounts (no ES needed) | P1 |

### 7.2 gRPC (High-Performance Internal)

| Service | Methods | Priority |
|---------|---------|----------|
| **LedgerService** | PostTransaction (compatibility RPC → PostLedgerPosting), GetBalance, GetAccount | P0 |
| **TransferService** | CreateTransfer, GetTransfer, ListTransfers | P0 |
| **ReportingService** | StreamReport, GetReportStatus | P1 |
| **ReconciliationService** | RunReconciliation, GetBreaks | P1 |

### 7.3 GraphQL (Flexible Queries)

| Feature | Description | Priority |
|---------|-------------|----------|
| **Nested Queries** | Account → Transactions → Entries | P1 |
| **Subscriptions** | Real-time balance updates | P1 |
| **Aggregations** | Sum, count, group by in query | P1 |

### 7.4 Webhooks / Event Streaming

These are compatibility names for the public projection. Wire subjects append
`.v1` and follow the catalog in `docs/domain-events.md`; `payment.settled` is
the settlement-batch confirmation, not a mutable Posting state.

| Event Type | Payload | Priority |
|------------|---------|----------|
| `account.created` | Account details | P0 |
| `transaction.posted` | Full transaction + entries | P0 |
| `transaction.reversed` | Original + reversal ref | P0 |
| `transfer.completed` | Transfer details | P0 |
| `payment.settled` | Settlement info | P1 |
| `reconciliation.break_found` | Break details | P1 |
| `account.balance.changed.v1` | Account, per-asset balance snapshot + cursor/as-of | P1 |

---

## 8. Real-Time Features

| Feature | Technology | Priority |
|---------|------------|----------|
| **Balance Updates** | WebSocket / Server-Sent Events | P1 |
| **Transaction Notifications** | WebSocket push | P1 |
| **Reconciliation Status** | Real-time progress | P2 |
| **Dashboard Metrics** | Live charts via GraphQL subscriptions | P2 |

---

## 9. Scheduled Jobs (Cron)

| Job | Schedule | Description | Priority |
|-----|----------|-------------|----------|
| **Daily Reconciliation** | 02:00 UTC | Match ledger to bank statements | P0 |
| **Interest Accrual** | Daily | Calculate/post interest | P1 |
| **Fee Calculation** | Monthly | Compute and post fees | P1 |
| **Period Close** | Month-end | Close accounting period | P1 |
| **Report Generation** | Daily/Monthly | Pre-generate standard reports | P1 |
| **Data Archival** | Quarterly | Move cold data to archive | P2 |
| **FX Rate Update** | Hourly | Fetch and store rates | P1 |
| **Compliance Scans** | Daily | AML/KYC transaction review | P1 |

---

## 10. Security Features

| Feature | Implementation | Priority |
|---------|---------------|----------|
| **Authentication** | JWT RS256 + OAuth2/OIDC | P0 |
| **Authorization** | Casbin RBAC/ABAC (tenant, role, resource) | P0 |
| **API Keys** | Scoped, rotatable, rate-limited | P0 |
| **Encryption at Rest** | Envelope encryption (DEK/KEK), KEK in HSM/Bitwarden | P0 |
| **Encryption in Transit** | TLS 1.3, mTLS for service-to-service | P0 |
| **PII Handling** | Field-level encryption, tokenization | P0 |
| **Secrets Management** | Bitwarden Secret Manager | P0 |
| **Rate Limiting** | Token bucket (Valkey), per-tenant/user/IP | P0 |
| **Audit Signing** | Cryptographic log signing | P1 |

---

## 11. Operational Excellence

### 11.1 Observability

| Feature | Implementation | Priority |
|---------|---------------|----------|
| **Distributed Tracing** | OpenTelemetry → Tempo | P0 |
| **Metrics** | Prometheus (RED + USE + business) | P0 |
| **Logging** | Structured JSON (`log/slog`) + correlation IDs | P0 |
| **Alerting** | PrometheusAlertmanager → PagerDuty/Slack | P0 |
| **Dashboards** | Grafana (pre-built for ledger ops) | P0 |
| **Log Aggregation** | Loki with tenant labels | P0 |

### 11.2 Reliability

| Feature | Implementation | Priority |
|---------|---------------|----------|
| **Circuit Breakers** | gobreaker on all external calls | P0 |
| **Retry Policies** | Exponential backoff, jitter | P0 |
| **Dead Letter Queues** | NATS DLQ for failed events | P0 |
| **Graceful Degradation** | Feature flags for non-critical paths | P1 |
| **Chaos Engineering** | Litmus scenarios (monthly) | P1 |

### 11.3 Disaster Recovery

| Feature | Implementation | Priority |
|---------|---------------|----------|
| **Point-in-Time Recovery** | PITR (PostgreSQL WAL) | P0 |
| **Cross-Region Replication** | Async replica for DR | P1 |
| **Backup Validation** | Automated restore tests (weekly) | P1 |
| **RPO/RTO Targets** | RPO < 5min, RTO < 30min, proven by recurring restore/failover drills | P0 |

---

## 12. Developer Experience

| Feature | Implementation | Priority |
|---------|---------------|----------|
| **OpenAPI/Swagger** | Auto-generated from code | P0 |
| **SDK Generation** | TypeScript, Python, Go | P1 |
| **Sandbox Environment** | Pre-seeded tenant for testing | P0 |
| **Test Clocks** | Virtual time per sandbox tenant (time-travel trials, interest, renewals) | P1 |
| **API Explorer** | Interactive docs with auth | P1 |
| **Webhook Testing** | ngrok-style local tunnel | P1 |

---

## 13. Implementation Priority Matrix

| Phase | Features | Timeline |
|-------|----------|----------|
| **MVP (Phase 1-2)** | Accounts, Transactions, Double-entry, Basic API, Auth, Audit | Weeks 1-4 |
| **Core (Phase 3-4)** | Transfers, Multi-currency, Reconciliation, Reports, Webhooks | Weeks 5-8 |
| **Production (Phase 5-6)** | Multi-tenancy, Payments, Compliance, Observability, DR | Weeks 9-12 |
| **Scale (Phase 7+)** | GraphQL, Real-time, Advanced reporting, ML/AML | Weeks 13+ |

---

## 14. Non-Functional Requirements

| Requirement | Target and evidence rule |
|-------------|--------------------------|
| **Throughput** | 10,000 postings/sec only after workload/topology/durability profile and report are committed |
| **Latency (p99)** | < 50ms for posting under that declared workload; not a planning-stage guarantee |
| **Availability** | 99.99% service SLO after dependency budgets and multi-AZ failover tests exist |
| **Consistency** | Strong (ACID) for ledger, Eventual for reporting |
| **Data Retention** | 7 years (configurable per jurisdiction) |
| **Audit Log Retention** | Signed roots retained indefinitely; detailed records follow jurisdictional retention/legal-hold policy |
| **Multi-Region** | Active-active reads and fenced active-passive writes only after residency/failover evidence |

---

## 15. Technical Decisions (ADRs to Create or Resolve)

| ADR | Topic | Status |
|-----|-------|--------|
| ADR-001 | Double-entry vs single-entry | Decided: Double-entry |
| ADR-002 | Amount/currency representation | Accepted by owner 2026-09-13: checked `int64` minor units, versioned exponent registry, wide aggregates |
| ADR-003 | Accounting fact immutability strategy | Accepted by owner 2026-09-13: immutable Posting/Entry; workflow states separate; corrections via linked postings |
| ADR-004 | Multi-tenancy: shared DB vs separate DB | Decided: Shared DB + RLS |
| ADR-005 | Event sourcing vs state-based | Decided: State + Domain Events |
| ADR-006 | Reconciliation: pull vs push model | Pending |
| ADR-007 | FX rate source and freshness | Pending |
| ADR-008 | Archival strategy for cold data | Pending |
| ADR-009 | Product boundary | Accepted by owner 2026-09-13: operational payment-platform ledger; full merchant books are separate scope |
| ADR-010 | Structured logging API | Decided by audit: kernel Logger port with stdlib `log/slog` adapter |
| ADR-011 | Balance materialization and authority | Pending — benchmark and owner approval required before E07 |

---

## 16. Dependencies on Template Features

| Template Feature | Used By |
|------------------|---------|
| **Clean Architecture** | All domain logic in `internal/domain/` |
| **DDD** | Ledger, Account, Posting, Hold, and payment-workflow aggregates |
| **Domain Specification pattern** | Executable invariants (sufficient funds, valid currency) |
| **CQRS** | Commands (PostLedgerPosting; compatibility PostTransaction) / Queries (GetBalance) |
| **fx DI** | Wiring all 5 binaries |
| **Valkey (L2 Cache)** | Non-authoritative balance/idempotency cache, rate limiting |
| **Ristretto (L1 Cache)** | Hot account/tenant config |
| **NATS JetStream** | Event streaming, async processing, webhooks |
| **gocron + Valkey Redlock** | Daily reconciliation, interest accrual |
| **OpenFeature + Unleash** | Feature flags for payment methods, new currencies |
| **Bitwarden Secrets** | DB passwords, API keys, encryption keys |
| **OpenTelemetry** | Full tracing across all services |
| **Traefik** | API Gateway with mTLS, rate limiting |
| **Testcontainers** | Integration tests with real Postgres/Valkey/NATS |

---

## 17. Next Steps

Execute the vertical proofs in `tasks/DELIVERY-SLICES.md`: establish the SDD
control plane, prove the ledger kernel through PostgreSQL, add tenant/public-edge
controls, then deliver capture/settlement/refund/payout and reconciliation. Tests,
evidence, and documentation ship with each task rather than as final phases.

---

*This document describes feature scope. Normative precedence and task-level
authority are defined by `SPEC.md §1.4` and `tasks/SDD.md`.*
