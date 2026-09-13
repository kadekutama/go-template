# Epic E03: Money-Movement Domain

**Status:** pending
**Story Points:** 37
**Phase:** 3 (parallel with E04, E05)
**Dependencies:** E02
**SDD Gate:** G2
**Design refs:** `docs/fintech-ledger-features.md §3`, `docs/money-flow.md §2`,
`docs/user-journeys.md §2.1–§2.2, §2.4–§2.5`, `docs/domain-events.md §3.3–§3.6, §3.9`

> Why a separate epic: transfers/refunds/payouts/fees/interest/FX are the
> product's core value and its highest-risk logic. Nemotron buried them as
> sub-bullets of generic "domain services" with no tasks for FX, settlement,
> returns, linking, templates, or payment methods.

## Tasks

### E03-T01: Transfer domain service (immediate + scheduled + recurring + bulk rules)
**Status:** pending
**Background:** Implements money-flow §2.3/§2.8/§2.9 and journeys §2.2. Covers
features §3.1 fully: internal, cross-currency, scheduled, bulk, templates.
**Files:**
- Create: `internal/domain/service/transfer_service.go`,
  `internal/domain/service/scheduled_transfer.go` (domain rules: recurrence expansion, reserve-vs-move),
  `internal/domain/service/batch_transfer.go` (item independence, idempotency-root scheme),
  `internal/domain/service/transfer_template.go`
**Steps:**
1. Immediate: validate both accounts (authoritative funds on source supplied by
   application, Active both, same tenant/ledger/asset), construct a template-based
   posting that debits source liability and credits destination liability.
2. Scheduled: validate everything *except* funds now; funds and a durable
   reservation are enforced atomically at execution. A product policy may create
   a hold at schedule time, but must lock the account and create it atomically;
   recurrence expands as `root:{occurrence}`.
3. Bulk: item independence rule (sibling failure never rolls back); batch idempotency root.
4. Templates (P2): stored parameter sets that pre-fill transfers.
5. Emit `transfer.created/completed/failed/canceled/batch.received/batch.completed` per domain-events §3.3.
**Acceptance Criteria:**
- [ ] Scheduled transfer with insufficient future funds fails at execution, not at schedule time (test).
- [ ] Bulk batch with 1 failing item completes siblings and reports PARTIAL (test).
- [ ] Cross-currency transfer without FX rate fails `CURRENCY_MISMATCH` (test).
**Story Points:** 5
**Depends On:** E02-T03, E02-T04, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §3.1`, `docs/money-flow.md §2.3, §2.8, §2.9`, `docs/user-journeys.md §2.2`, `docs/domain-events.md §3.3`
**SDD Gate:** G2

---

### E03-T02: Refund domain service (window, amount, reversal linkage)
**Status:** pending
**Background:** Journeys §2.4, money-flow §2.4. Partial/full refunds, window
expiry, reversal linkage for audit.
**Files:**
- Create: `internal/domain/service/refund_service.go`
**Steps:**
1. Enforce `OriginalExists`, `RefundAmountValid` (amount ≤ original − prior refunds),
   `RefundWindowValid` (configurable, default 90d), `AccountActive`.
2. Construct refund-acceptance and customer-settlement postings linked to the
   original capture/payment. Original entries stay immutable; fee refunds use an
   explicit processor/platform policy result, never an assumed proportion.
3. Emit `refund.created/succeeded/failed` plus posting events.
**Acceptance Criteria:**
- [ ] Refund exceeding (original − prior refunds) fails `REFUND_EXCEEDS_ORIGINAL` (test).
- [ ] Refund after window fails `REFUND_WINDOW_EXPIRED` (test, configurable window).
- [ ] Reversal posting references the original Posting ID (test).
**Story Points:** 3
**Depends On:** E02-T04, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §3.2` (returns), `docs/money-flow.md §2.4`, `docs/user-journeys.md §2.4`, `docs/domain-events.md §3.5`
**SDD Gate:** G2

---

### E03-T03: Payout domain rules (methods, states, settlement tracking)
**Status:** pending
**Background:** Money-flow §2.2 (two-stage settlement), features §3.2 + §7.1 payouts.
**Files:**
- Create: `internal/domain/service/payout_service.go`,
  `internal/domain/valueobject/{payout_method.go,payout_status.go}`
**Steps:**
1. Methods: ACH, RTP/FedNow, Wire, Check — each with expected settlement lag
   (money-flow §5 table) and state machine PENDING → IN_TRANSIT → PAID / FAILED.
2. Settlement tracking: expected vs actual settlement timestamps; provider
   idempotency/reference/trace IDs; overdue detection rule.
3. Staged entries: submission DR Merchant Payable / CR Payouts Payable; settlement
   DR Payouts Payable / CR Bank Cash. Failure/return reverses submission and posts
   any fee separately.
4. Cancel allowed only while PENDING. Emit `payout.created/pending/paid/failed`.
**Acceptance Criteria:**
- [ ] Cancel after IN_TRANSIT fails (test).
- [ ] Overdue settlement (actual > expected + grace) flagged by pure rule function (test).
- [ ] Each method's settlement lag matches money-flow §5 (table test).
**Story Points:** 3
**Depends On:** E02-T04, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §3.2`, `docs/money-flow.md §2.2, §5`, `docs/domain-events.md §3.6`
**SDD Gate:** G2

---

### E03-T04: Fee calculation + interest accrual domain rules
**Status:** pending
**Background:** Cron jobs §9 (Fee monthly, Interest daily) need pure calculation
rules here; scheduling lives in E14.
**Files:**
- Create: `internal/domain/service/{fee_service.go,interest_service.go}`
**Steps:**
1. Fees: per-transaction (basis points + cap/floor), monthly tiers; produce
   template entries debiting merchant payable and crediting platform revenue.
   Track processor/network fees separately from platform/application fees.
2. Interest: daily rate = annual/365 (Actual/365), applied to configured account
   types; direction depends on asset vs liability.
3. Emit `fee.assessed/collected`, interest via `transaction.posted` (type=INTEREST).
**Acceptance Criteria:**
- [ ] Fee cap/floor clamping table tests.
- [ ] Interest for leap/non-leap day counts (table test).
- [ ] Zero/negative balance accounts accrue nothing (test).
**Story Points:** 3
**Depends On:** E02-T03, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §9` (Fee, Interest jobs), `docs/money-flow.md §2.6`, `docs/domain-events.md §3.10`
**SDD Gate:** G2

---

### E03-T05: FX value objects, rates, and gain/loss rules
**Status:** pending
**Background:** Multi-currency (features §2.1, journeys §2.5, money-flow §2.7).
**Files:**
- Create: `internal/domain/valueobject/{fx_rate.go,fx_pair.go}`,
  `internal/domain/service/fx_service.go` (pure conversion + gain/loss calc; fetching is E10)
**Steps:**
1. `FxRate{ID,pair,fixedPointRate,source,quotedAt,ttl,roundingPolicy}` with staleness rule.
2. `Convert(amount, rate) Money` + `GainLoss(authorizeRate, settleRate, amount)`.
3. Construct linked source/destination currency lots that each balance
   independently. Settlement differences post explicit FX gain/loss entries;
   mark-to-market vs realized-only is a versioned policy.
4. Emit `fx.rate.updated.v1` shape defined (published by E10 fetcher).
**Acceptance Criteria:**
- [ ] Stale rate (>TTL) rejected by rule function (test).
- [ ] Gain/loss sign correct for both directions (table test).
- [ ] Conversion never uses float; both currency lots balance independently (property tests).
**Story Points:** 3
**Depends On:** E02-T02, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §2.1, §3.1`, `docs/money-flow.md §2.7`, `docs/user-journeys.md §2.5`, `docs/domain-events.md §3.9`
**SDD Gate:** G2

---

### E03-T06: Payment methods, settlement batches, returns, linking
**Status:** pending
**Background:** Remaining features §3.2 rows Nemotron never tasked: methods
(ACH/Wire/RTP/Card/Crypto/Wallet), push/pull initiation, return/reject R-codes
with auto-reversal, payment linking to invoices/orders/subscriptions, and
provider settlement-batch identity.
**Files:**
- Create: `internal/domain/valueobject/{payment_method.go,payment_status.go,return_code.go}`,
  `internal/domain/service/payment_service.go` (domain rules),
  `internal/domain/entity/{payment_link.go,settlement_batch.go,provider_object_link.go}`
**Steps:**
1. Method registry with capabilities (push/pull, settlement lag, reversible?).
2. Return-code catalog (ACH R01–R85 subset + card decline mapping) → each maps to
   auto-reversal or manual-review disposition.
3. `PaymentLink{paymentID, linkedType, linkedID}` (invoice/order/subscription) with
   uniqueness per (payment, linked) pair.
4. Payment workflow state machine: REQUIRES_METHOD/ACTION → AUTHORIZED →
   CAPTURED → PENDING_SETTLEMENT → SETTLED / FAILED / RETURNED. These are not
   states on immutable postings.
5. Provider timeout enters `OUTCOME_UNKNOWN`; resolution queries provider state
   with the provider idempotency key before any retry that could move money.
6. Settlement batches retain an immutable provider object mapping, provider/batch
   trace IDs, gross/fee/net totals, asset code, coverage window, and per-item
   status. Partial batches reconcile each item; a duplicate provider object or
   webhook event returns the existing state without another posting.
**Acceptance Criteria:**
- [ ] Every R-code in the catalog maps to a disposition (exhaustiveness test).
- [ ] Irreversible methods reject refund-to-original-method with clear code (test).
- [ ] Duplicate payment links rejected (test).
- [ ] Settlement batch totals and provider trace IDs are retained; duplicate
  provider delivery is idempotent (test).
**Story Points:** 5
**Depends On:** E02-T04, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §3.2`, `docs/money-flow.md §5`, `docs/domain-events.md §3.4`
**SDD Gate:** G2

---

### E03-T07: Dispute domain service (evidence, representment, fees)
**Status:** pending
**Background:** Features §3.3 + money-flow §2.11 + journeys §2.7. Holds, network
deadlines, network-configured representment stages, and explicit fee policy.
**Files:**
- Create: `internal/domain/service/dispute_service.go`,
  `internal/domain/entity/dispute.go`,
  `internal/domain/valueobject/dispute_status.go`
**Steps:**
1. `OpenDispute`: validate network window (config per network, e.g. Visa 120d);
   create a durable hold (not an entry) and post the dispute fee with an
   approved template; emit `dispute.opened.v1`.
2. Evidence window: track `evidence_due_at`; late submissions rejected.
3. `CloseDispute(outcome)`: release/consume the hold and construct policy-approved
   postings; network/provider result determines whether a fee is refunded.
4. Representment stages/count/deadlines come from versioned network policy; do
   not hard-code “exactly once.” New evidence and transition legality are enforced.
5. Fraud early warnings: auto-refund recommendation rule when refund cost < expected dispute cost + fee.
**Acceptance Criteria:**
- [ ] Late dispute and late evidence both rejected with distinct codes (tests).
- [ ] Representment and fee behavior follow versioned network fixtures (table tests).
- [ ] Lost dispute produces balanced reversal entries linked to the original (test).
**Story Points:** 4
**Depends On:** E02-T04, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §3.3`, `docs/money-flow.md §2.11`, `docs/user-journeys.md §2.7`, `docs/domain-events.md §3.12`
**SDD Gate:** G2

---

### E03-T08: Authorization lifecycle + descriptors + SCA states
**Status:** pending
**Background:** Money-flow §2.12: authorize now, capture later (full/partial),
expiry auto-void; SCA/3DS challenge states; network descriptor rules.
**Files:**
- Create: `internal/domain/service/authorization_service.go`,
  `internal/domain/valueobject/{auth_status.go,descriptor.go}`
**Steps:**
1. Authorize: place hold for the full amount with `auth_expires_at` (+7d default,
   network-configurable); status AUTHORIZED.
2. Capture: enforce `CaptureAmountValid` (captured_total + capture ≤ authorized);
   single partial capture unless the network allows multiples; release remainder.
3. Expiry sweep rule: expired auths auto-void (called by E14 compliance scans).
4. Descriptors: ≤22 chars + network charset validation (`INVALID_DESCRIPTOR`).
5. SCA: `requires_action` state entered on challenge; resolves to succeeded/failed
   via E10-T03 challenge completion (maps to `payment_intent.requires_action.v1`).
**Acceptance Criteria:**
- [ ] Over-capture rejected with remaining capturable amount in details (test).
- [ ] Repeated expiry handling produces one hold-release effect (idempotent test).
- [ ] Invalid descriptors rejected per network rules (table test).
**Story Points:** 3
**Depends On:** E02-T04, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §3.2`, `docs/money-flow.md §2.12`, `docs/domain-events.md §3.4`
**SDD Gate:** G2

---

### E03-T09: Platform split (destination charges + application fees)
**Status:** pending
**Background:** Features §3.5 + money-flow §2.10: charge on platform, carve the
fee at charge time, settle net to the connected account (with FX if needed).
**Files:**
- Create: `internal/domain/service/platform_split.go`
**Steps:**
1. Validate: platform + connected accounts active, same tenant hierarchy grant (E05-T02).
2. Capture posting: DR Processor Receivable gross; CR Connected Merchant Payable
   net; CR Platform Fee Revenue fee. Settlement and payout are later templates.
3. Refund allocation and fee refund follow explicit contract/provider policy;
   never assume all fees reverse proportionally.
**Acceptance Criteria:**
- [ ] Gross == fee + net in every posting (property test).
- [ ] Cross-currency split applies §2.7 FX with gain/loss legs (test).
- [ ] Refund of a split charge reverses all three legs (test).
**Story Points:** 3
**Depends On:** E02-T04, E02-T07, E03-T01, E05-T02
**Related Docs:** `docs/fintech-ledger-features.md §3.5`, `docs/money-flow.md §2.10`
**SDD Gate:** G2

---

### E03-T10: Top-ups (platform funding from bank)
**Status:** pending
**Background:** Features §3.4: reverse-payout flow crediting platform/merchant
balance from a verified bank account.
**Files:**
- Create: `internal/domain/service/topup_service.go`
**Steps:**
1. Validate: verified external bank-account instrument (never a ledger-account
   `Verify` method), amount positive, and idempotency key.
2. States PENDING → SUCCEEDED / FAILED; settlement tracked like payouts in reverse.
3. Emit `topup.succeeded/failed.v1`; cancel allowed while PENDING.
**Acceptance Criteria:**
- [ ] Top-up from unverified account rejected with `ACCOUNT_UNVERIFIED` (test).
- [ ] Failed top-up moves nothing (test).
**Story Points:** 2
**Depends On:** E02-T04, E02-T07, E03-T06
**Related Docs:** `docs/fintech-ledger-features.md §3.4`, `docs/domain-events.md §3.13`
**SDD Gate:** G2

---

### E03-T11: Payout eligibility, reserves, and negative-balance recovery
**Status:** pending
**Background:** P0 payout controls in features §3.4 and money-flow §5 need an
explicit policy owner. A ledger can become negative after a return, dispute, or
fee; blocking payouts and recovering the amount must be durable workflows, not
cache or ad-hoc handler logic.
**Files:**
- Create: `internal/domain/service/payout_policy.go`,
  `internal/domain/service/negative_balance_recovery.go`,
  `internal/domain/valueobject/payout_policy.go`
**Steps:**
1. Define `PayoutEligibility` per-tenant/asset policy: minimums, seven-day first-payout
   hold, rolling reserve, instant-payout eligibility, and the available-balance
   calculation at a supplied ledger cursor.
2. Reject an ineligible payout with a stable error and reason details (negative
   available balance, first-payout hold, reserve, minimum, or unverified
   destination). Eligibility always reads the strong PostgreSQL projection, never
   Valkey or a replica that is outside its staleness contract.
3. Model negative-balance recovery as a durable, idempotent workflow tied to a
   verified external bank instrument. Provider timeouts become `OUTCOME_UNKNOWN`;
   status lookup is required before retry. A successful collection posts an
   approved funding/offset template; failures escalate for review.
4. Record policy decision, recovery attempt/run key, provider trace ID, actor,
   and evidence; do not mutate Posting/Entry rows or an account balance column.
**Acceptance Criteria:**
- [ ] A first payout before the configured seven-day hold or below the
  configured reserve is rejected with an explainable reason (test).
- [ ] Negative available balance blocks payout and starts at most one durable
  recovery attempt per idempotency key; replay returns the original result (test).
- [ ] Unknown provider outcome is not blindly retried; a confirmed recovery
  produces a balanced posting and audit/outbox facts (fault-injection test).
**Story Points:** 3
**Depends On:** E02-T04, E02-T07, E03-T03, E03-T10
**Related Docs:** `docs/fintech-ledger-features.md §3.4`, `docs/money-flow.md §5`, `docs/ledger-core.md §7–§8`, `docs/api-contracts.md §4`
**SDD Gate:** G2

## Acceptance Criteria

- [ ] E03-T01 … E03-T11 all `completed` (count 37 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Every features §3 row has domain rules + specs + tests
- [ ] Every money-flow §2 pattern has a corresponding service
- [ ] SDD gate G2 checks pass — `tasks/tracking/GATES.md#G2`
