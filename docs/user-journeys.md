# Fintech Ledger - User Journey Design

**Version:** 1.0.0  
**Status:** Design Phase  
**Related:** [Ledger Core](./ledger-core.md), [Feature Spec](./fintech-ledger-features.md), [Money Flow](./money-flow.md), [Data Flow](./data-flow.md)

> Event names in the journey matrix are readable shorthand. Wire subjects and
> payload schemas use the versioned names in `docs/domain-events.md` (for
> example, `tenant.created.v1` and `transaction.posted.v1`).

---

## 1. Personas

| Persona | Description | Primary Goals |
|---------|-------------|---------------|
| **Platform Operator** | Runs the fintech platform (Stripe-like) | Manage tenants, monitor system health, compliance |
| **Merchant/Tenant** | Business using the platform to accept payments | Receive payments, manage accounts, reconcile |
| **End Customer** | Merchant's customer making payments | Pay easily, see transaction history |
| **Finance Team** | Merchant's accounting/finance | Reconcile, generate reports, period close |
| **Compliance Officer** | Regulatory/compliance monitoring | AML reviews, audit trails, reporting |
| **Developer** | Integrates with platform APIs | Build integrations, test in sandbox |

---

## 2. Core User Journeys

> Convention: endpoint paths below omit the `/v1` prefix for brevity.
> All paths map 1:1 to `docs/api-contracts.md` §7 (e.g., `POST /accounts` → `POST /v1/accounts`).

### 2.1 Journey: Merchant Onboarding & First Payment

```mermaid
sequenceDiagram
    actor Merchant
    participant Platform as Platform Operator
    participant API as REST API
    participant Ledger as Ledger Service
    participant Valkey as Valkey Cache
    participant NATS as NATS JetStream
    participant Webhook as Webhook Dispatcher

    Merchant->>Platform: Sign up for platform
    Platform->>API: POST /tenants (provision tenant)
    API->>Ledger: CreateTenantCommand
    Ledger->>Ledger: Validate specs (TenantNameUnique, ValidRegion)
    Ledger->>PostgreSQL: INSERT tenant + default accounts
    Ledger-->>API: TenantCreated {tenant_id}
    API-->>Merchant: 201 Created {tenant_id}

    Merchant->>API: POST /accounts (create operating account)
    API->>Ledger: CreateAccountCommand
    Ledger->>Ledger: Validate asset registry + chart/account purpose
    Ledger->>PostgreSQL: INSERT account (chart-selected class, USD)
    Ledger->>NATS: Outbox publisher delivers AccountCreated (at least once)
    NATS-->>Webhook: Deliver to merchant webhook
    API-->>Merchant: 201 Created {account_id, account_number}

    Merchant->>API: POST /payment-intents (create payment)
    API->>Ledger: CreatePaymentIntentCommand
    Ledger->>PostgreSQL: Reserve durable idempotency key + request hash
    Ledger->>PostgreSQL: INSERT payment_intent (PENDING)
    API-->>Merchant: 201 Created {payment_intent_id, client_token}

    Customer->>Merchant: Pays with card
    Merchant->>API: POST /payment-intents/{intent_id}/confirm
    API->>Ledger: ConfirmPaymentIntentCommand
    Ledger->>PaymentProcessor: Charge card (async)
    PaymentProcessor-->>NATS: PaymentSucceeded event
    NATS->>Ledger: HandlePaymentSucceeded
    Ledger->>Ledger: Validate specs (AccountActive, posting template)
    Ledger->>PostgreSQL: BEGIN TRANSACTION
    Ledger->>PostgreSQL: INSERT posting (DR processor receivable, CR merchant payable + fee revenue)
    Ledger->>PostgreSQL: INSERT balance checkpoints + outbox + idempotency response
    Ledger->>PostgreSQL: COMMIT
    Ledger->>NATS: Outbox publisher delivers TransactionPosted (at least once)
    NATS-->>Webhook: Deliver to merchant webhook
    API-->>Merchant: 200 OK {status: SUCCEEDED}
```

**Key Touchpoints:**
1. **Tenant Provisioning** - Creates isolated ledger with default chart of accounts
2. **Account Creation** - Chart-selected account; merchant funds use a payable
   liability account rather than an asset merely because money is received
3. **Payment Intent** - Idempotent, supports retries
4. **Payment Confirmation** - Async processing, webhook notification
5. **Ledger Posting** - Atomic double-entry, immediate balance update

---

### 2.2 Journey: Internal Transfer Between Accounts

```mermaid
sequenceDiagram
    actor FinanceTeam
    participant API as REST API
    participant Ledger as Transfer Service
    participant PostgreSQL as PostgreSQL
    participant NATS as NATS JetStream

    FinanceTeam->>API: POST /transfers {from_account_id, to_account_id, amount_minor, currency, idempotency_key}
    API->>Ledger: TransferCommand
    Ledger->>PostgreSQL: Reserve scoped idempotency key + request hash
    Ledger->>PostgreSQL: SELECT FOR UPDATE accounts in deterministic ID order
    alt Durable key reserved or matching replay found
        Ledger->>Ledger: Validate specs
        Note over Ledger: SufficientFunds(from_account_id, amount_minor, currency)
        Note over Ledger: AccountActive(from), AccountActive(to)
        Note over Ledger: ValidCurrency(from, to)
        Note over Ledger: IdempotencyKeyUnique(tenant, operation, key)
        Ledger->>PostgreSQL: BEGIN
        Ledger->>PostgreSQL: INSERT immutable posting (TRANSFER)
        Ledger->>PostgreSQL: INSERT entries (DR source liability, CR destination liability)
        Ledger->>PostgreSQL: INSERT checkpoints + outbox + stored response
        Ledger->>PostgreSQL: COMMIT
        Ledger->>NATS: Outbox publisher delivers TransactionPosted
        API-->>FinanceTeam: 201 Created {transfer_id, transaction_id}
    else Same key, different request hash
        API-->>FinanceTeam: 409 Conflict {error: "IDEMPOTENCY_CONFLICT"}
    end
```

**Critical Invariants Enforced:**
- **Atomicity** - Single transaction, both entries or none
- **Idempotency** - PostgreSQL uniqueness + request fingerprint returns the original result or a conflict
- **Consistency** - Sufficient funds checked WITHIN transaction
- **Isolation** - SELECT FOR UPDATE prevents race conditions

---

### 2.3 Journey: Daily Reconciliation (Automated)

```mermaid
sequenceDiagram
    participant Cron as Cron Scheduler (gocron)
    participant Ledger as Reconciliation Workflow
    participant PostgreSQL as PostgreSQL
    participant Valkey as Valkey
    participant BankAPI as Bank API (SFTP/API)
    participant NATS as NATS JetStream
    participant Alert as Alerting (PagerDuty)

    Cron->>Ledger: Trigger DailyReconciliationWorkflow (02:00 UTC)
    Ledger->>Valkey: Acquire Distributed Lock (reconciliation:{date})
    alt Lock acquired
        Ledger->>PostgreSQL: SELECT accounts needing reconciliation
        par For each account
            Ledger->>BankAPI: Fetch statement (MT940/BAI2/CSV)
            Ledger->>Ledger: Parse statement
            Ledger->>Ledger: Match ledger entries to statement lines
            alt Match found
                Ledger->>PostgreSQL: INSERT reconciliation_match
                Note over Ledger,PostgreSQL: Entry remains immutable; match stores source ID, rule version, and decision
            else Mismatch (amount/date/reference)
                Ledger->>PostgreSQL: INSERT reconciliation_break
                Ledger->>NATS: Publish ReconciliationBreakFound
                NATS-->>Alert: Send alert to Finance Team
            end
        end
        Ledger->>PostgreSQL: INSERT reconciliation_run (status, breaks_count)
        Ledger->>Valkey: Release Lock
    else Lock failed (another instance running)
        Cron-->>Ledger: Skip (already running)
    end
```

**Reconciliation Break Types:**
| Break Type | Detection | Resolution |
|------------|-----------|------------|
| **Missing in Ledger** | Bank has entry, ledger doesn't | Investigate: failed webhook, timing diff |
| **Missing in Bank** | Ledger has entry, bank doesn't | Investigate: rejected payment, reversal |
| **Amount Mismatch** | Same ref, different amounts | Investigate: fees, FX, partial settlement |
| **Date Mismatch** | Same ref/amount, different dates | Usually timing - auto-resolve next day |

---

### 2.4 Journey: End-Customer Refund

```mermaid
sequenceDiagram
    actor Customer
    actor Merchant
    participant API as REST API
    participant Ledger as Refund Service
    participant PP as Payment Processor
    participant PG as PostgreSQL
    participant NATS as NATS JetStream
    participant Webhook as Merchant Webhook

    Customer->>Merchant: Requests refund
    Merchant->>API: POST /refunds {transaction_id, amount_minor, currency, reason}
    API->>Ledger: CreateRefundCommand
    Ledger->>Ledger: Validate specs
    Note over Ledger: OriginalTransactionExists
    Note over Ledger: RefundAmountLTEOriginal
    Note over Ledger: RefundWindowNotExpired (90 days)
    Note over Ledger: AccountActive
    Ledger->>PP: Initiate refund (async)
    PP-->>NATS: RefundSucceeded
    NATS->>Ledger: HandleRefundSucceeded
    Ledger->>PG: BEGIN
    Ledger->>PG: INSERT immutable reversal posting (linked to original)
    Ledger->>PG: INSERT entries (DR merchant payable, CR refunds payable)
    Ledger->>PG: INSERT balance checkpoint + outbox
    Ledger->>PG: COMMIT
    Ledger->>NATS: Outbox publisher delivers TransactionPosted (type=REVERSAL)
    NATS-->>Webhook: refund.succeeded
    API-->>Merchant: 201 Created {refund_id, SUCCEEDED}
```

**Refund Variants:**
| Type | Description | Ledger Impact |
|------|-------------|---------------|
| **Full Refund** | Entire transaction reversed | Mirror entries with opposite direction |
| **Partial Refund** | Portion of amount | Proportional entries |
| **Refund to Different Method** | Card → Bank transfer | Separate settlement tracking |

---

### 2.5 Journey: Multi-Currency Payment with FX

```mermaid
sequenceDiagram
    actor Customer
    participant API as REST API
    participant Ledger as Payment Service
    participant FXService as FX Rate Service
    participant Valkey as Valkey (FX Cache)
    participant PostgreSQL as PostgreSQL

    Customer->>API: Pay 100 EUR (merchant in USD)
    API->>Ledger: CreatePaymentIntentCommand {amount_minor: 10000, currency: EUR}
    Ledger->>Valkey: GET fx_rate:EUR_USD
    alt Cache hit
        Valkey-->>Ledger: Rate = 1.0850
    else Cache miss
        Ledger->>FXService: Fetch latest EUR/USD
        FXService-->>Ledger: Rate = 1.0850
        Ledger->>Valkey: SET fx_rate:EUR_USD (TTL 1h)
    end
    Ledger->>Ledger: Calculate 10850 USD minor units from 10000 EUR at fixed-point rate 1.0850
    Ledger->>PostgreSQL: INSERT payment_intent (amount_minor: 10000 EUR, settled_amount_minor: 10850 USD, fx_rate: 1.0850)
    Note over Ledger: Settlement in merchant's account currency (USD)
    Note over Ledger: FX gain/loss tracked in separate REVENUE/EXPENSE account
```

---

### 2.6 Journey: Period Close (Month-End)

```mermaid
sequenceDiagram
    actor FinanceTeam
    participant API as REST API
    participant Ledger as Period Close Workflow
    participant PostgreSQL as PostgreSQL
    participant NATS as NATS JetStream
    participant ReportGen as Report Generator

    FinanceTeam->>API: POST /periods/{period_id}/close
    API->>Ledger: ClosePeriodCommand
    Ledger->>PostgreSQL: SELECT * FROM accounts WHERE tenant_id = ?
    Ledger->>Ledger: Validate specs
        Note over Ledger: AllTransactionsPosted (no PENDING)
        Note over Ledger: ReconciliationComplete (breaks = 0 or acknowledged)
        Note over Ledger: NoUnpostedEntries
    Ledger->>PostgreSQL: BEGIN
    Ledger->>PostgreSQL: UPDATE period SET status=CLOSED, closed_at=NOW()
    Ledger->>PostgreSQL: INSERT closing_entries (income summary → retained earnings)
    Ledger->>PostgreSQL: COMMIT
    Ledger->>NATS: Publish PeriodClosed
    NATS->>ReportGen: Generate period-end reports
    ReportGen->>PostgreSQL: STORE reports (BalanceSheet, P&L, TrialBalance)
    API-->>FinanceTeam: 200 OK {period_id, status: CLOSED, reports: [...]}
```

**Period Close Validations:**
- No unresolved payment/settlement workflows affecting the period
- All reconciliation breaks resolved or acknowledged
- All sub-ledgers balanced
- FX revaluation posted (if multi-currency)

---

### 2.7 Journey: Dispute Lifecycle (Chargeback)

```mermaid
sequenceDiagram
    actor Network as Card Network
    participant API as REST API
    participant Ledger as Dispute Service
    participant PG as PostgreSQL
    participant NATS as NATS JetStream
    participant Merchant as Merchant

    Network->>API: dispute.opened notification
    API->>Ledger: OpenDisputeCommand
    Ledger->>PG: Create durable hold (no entry) + post dispute fee template
    Ledger->>NATS: Publish dispute.opened
    NATS-->>Merchant: Alert + evidence deadline
    Merchant->>API: POST /disputes/{id}/evidence
    API->>Ledger: SubmitEvidenceCommand
    alt Evidence wins
        Ledger->>PG: Release hold + reverse fee
        Ledger->>NATS: Publish dispute.closed (WON)
    else Evidence loses or deadline passes
        Ledger->>PG: Convert hold to reversal entries
        Ledger->>NATS: Publish dispute.closed (LOST)
    end
    API-->>Merchant: 200 OK {dispute_id, outcome}
```

**Dispute Rules Recap:**
- The hold and fee posting are created atomically at open; the hold itself is
  not a journal entry. Evidence has a network deadline.
- Representment stages and evidence rules come from versioned network policy.
- Fraud early warnings auto-refund when cheaper than the dispute fee.

---

## 3. Journey Summary Matrix

| Journey | Primary Actor | API Endpoints | Key Domain Services | Events Published |
|---------|---------------|---------------|---------------------|------------------|
| Merchant Onboarding | Platform Operator | POST /tenants, /accounts | TenantService, AccountService | TenantCreated, AccountCreated |
| First Payment | Merchant/Customer | POST /payment-intents, /confirm | PaymentService, TransferService | PaymentIntentCreated, TransactionPosted |
| Internal Transfer | Finance Team | POST /transfers | TransferService | TransactionPosted |
| Daily Reconciliation | System (Cron) | N/A (internal) | ReconciliationWorkflow | ReconciliationBreakFound, ReconciliationCompleted |
| Refund | Merchant | POST /refunds | RefundService | TransactionPosted (REVERSAL) |
| Multi-Currency Pay | Customer | POST /payment-intents | PaymentService, FXService | TransactionPosted |
| Period Close | Finance Team | POST /periods/{period_id}/close | PeriodCloseWorkflow | PeriodClosed |
| Dispute Lifecycle | Card Network / Merchant | POST /disputes, /evidence, /represent, /close | DisputeService | dispute.opened, dispute.closed |

---

## 4. Error Handling & Compensation

| Failure Point | Detection | Compensation |
|---------------|-----------|--------------|
| Payment processor timeout | Circuit breaker opens | PaymentIntent stays PENDING/OUTCOME_UNKNOWN; resolve provider status by idempotency key before retry |
| Database deadlock | PostgreSQL error 40P01 | Retry with exponential backoff (max 3) |
| Insufficient funds | Specification failure | Return 400 with INSUFFICIENT_FUNDS code |
| Idempotency conflict | PostgreSQL durable key/fingerprint differs | Return 409 with existing posting/workflow ID |
| Reconciliation break | Mismatch detected | Create break record, alert, manual resolution workflow |
| Period close validation fails | Specification failure | Return 400 with list of failing checks |

---

## 5. Security & Compliance Touchpoints

| Journey Step | Security Control | Compliance Artifact |
|--------------|------------------|---------------------|
| Authentication | JWT validation (RS256) | Auth log entry |
| Authorization | Casbin policy check | Authorization decision log |
| Payment creation | Idempotency key validation | Idempotency record |
| Ledger posting | Double-entry validation | Transaction + entries (immutable) |
| Webhook delivery | Signature verification | Delivery receipt |
| Period close | Multi-person approval (configurable) | Approval audit trail |

---

*This document should be reviewed by all implementation agents before Phase 2 (Domain Layer) begins.*
