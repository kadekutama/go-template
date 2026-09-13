# Fintech Ledger - API Contract Specifications

**Version:** 1.0.0  
**Status:** Design Phase  
**API origin:** `https://api.ledger.example.com` (current prefix: `/v1`)  
**Related:** [Ledger Core](./ledger-core.md), [User Journeys](./user-journeys.md), [Money Flow](./money-flow.md), [Data Flow](./data-flow.md)

> This file is a design inventory until generated OpenAPI, Protobuf, and GraphQL
> schemas exist. Ledger semantics and money safety follow `ledger-core.md`.

---

## 1. API Design Principles

| Principle | Implementation |
|-----------|----------------|
| **RESTful** | Resource-oriented, HTTP verbs, standard status codes |
| **Versioned** | URL path versioning (`/v1/`, `/v2/`), header fallback |
| **Idempotent** | All write operations support `Idempotency-Key` header |
| **Paginated** | Cursor-based pagination for list endpoints |
| **Filtered** | Standard query params for filtering, sorting |
| **Typed Errors** | Standardized error envelope with codes |
| **Authenticated** | Bearer JWT (RS256), API Keys for server-to-server |
| **Rate Limited** | Token bucket, headers for limits/remaining |
| **Traced** | `X-Request-ID` accepted or generated, propagated throughout |

---

## 2. Common Headers

### Request Headers
| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes | `Bearer <jwt>` or `ApiKey <key>` |
| `Idempotency-Key` | Money movement and create ops | Opaque printable ASCII, 1–100 chars; durable scoped record retained at least 24h |
| `X-Request-ID` | No | Validated if supplied; otherwise server generates it; never trusted as authorization context |
| `Accept` | Yes | `application/json` |
| `Content-Type` | Write ops | `application/json` |
| `Accept-Language` | No | `en`, `id`, etc. (i18n) |

### Response Headers
| Header | Description |
|--------|-------------|
| `X-Request-ID` | Echoed from request |
| `X-RateLimit-Limit` | Max requests in window |
| `X-RateLimit-Remaining` | Remaining in window |
| `X-RateLimit-Reset` | Unix timestamp of reset |
| `Retry-After` | Seconds until retry (on 429) |
| `X-Correlation-ID` | Internal trace ID |

---

## 3. Standard Response Envelope

### Success Response
```json
{
  "data": { ... },
  "meta": {
    "page": { "cursor": "abc123", "has_more": true, "total": 150 }
  },
  "request_id": "req_abc123"
}
```

### Error Response
```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "INSUFFICIENT_FUNDS",
    "message": "Account has insufficient available balance",
    "param": "amount_minor",
    "decline_code": null,
    "details": {
      "account_id": "acc_abc123",
      "asset_code": "USD",
      "available_amount_minor": 10000,
      "requested_amount_minor": 20000
    },
    "request_id": "req_abc123"
  },
  "request_id": "req_abc123"
}
```

**Error `type` taxonomy** (Stripe-compatible):
| Type | When |
|------|------|
| `api_error` | Server-side failure (27.5xx, retryable unless documented otherwise) |
| `card_error` | Card declined or SCA challenge failed; `decline_code` carries the network reason |
| `idempotency_error` | Same key reused with a different request |
| `invalid_request_error` | Validation, state, or business-rule failure |

`param` names the offending request field when applicable, `decline_code` the
card-network reason (e.g., `insufficient_funds`, `authentication_required`).
Card declines always return HTTP 402 with `type: card_error`.

### Validation Error Response
```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Request validation failed",
    "details": {
      "fields": [
        { "field": "amount_minor", "code": "MIN_VALUE", "message": "Amount must be positive" },
        { "field": "currency", "code": "INVALID_CURRENCY", "message": "Invalid ISO 4217 code" }
      ]
    },
    "request_id": "req_abc123"
  },
  "request_id": "req_abc123"
}
```

---

## 4. Error Codes Reference

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INTERNAL_ERROR` | 500 | Unexpected server error |
| `UNAUTHORIZED` | 401 | Missing/invalid authentication |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `VALIDATION_FAILED` | 400 | Request validation failed |
| `INSUFFICIENT_FUNDS` | 400 | Available balance < requested |
| `ACCOUNT_FROZEN` | 400 | Account status prevents operation |
| `ACCOUNT_CLOSED` | 400 | Account closed |
| `UNSUPPORTED_CURRENCY` | 400 | Asset/currency code is not active in the registry |
| `CURRENCY_MISMATCH` | 400 | Entries/legs use incompatible assets without an approved FX template |
| `IDEMPOTENCY_CONFLICT` | 409 | Idempotency key exists with different request |
| `IDEMPOTENCY_PROCESSING` | 409 | Request with same key still processing |
| `OUTCOME_UNKNOWN` | 409 | Provider result is unknown; resolve provider status before retrying |
| `CONCURRENT_TRANSFER` | 409 | Distributed lock held by another transfer with overlapping accounts |
| `PERIOD_CLOSED` | 400 | Accounting period closed |
| `INVALID_ENTRY_AMOUNT` | 400 | Entry is zero/negative, overflows, or is not valid minor units |
| `INVALID_POSTING_TEMPLATE` | 400 | Accounts/sides do not match the permitted operation template |
| `REFUND_WINDOW_EXPIRED` | 400 | Refund requested after allowed window |
| `REFUND_EXCEEDS_ORIGINAL` | 400 | Refund amount > original - previous refunds |
| `RATE_LIMITED` | 429 | Too many requests |
| `FEATURE_DISABLED` | 403 | Feature flag off for tenant |
| `CAPTURE_EXCEEDS_AUTHORIZED` | 400 | Capture exceeds authorized minus already-captured |
| `AUTHORIZATION_EXPIRED` | 400 | Authorization past `auth_expires_at`; re-authorize |
| `DISPUTE_ACTION_INVALID` | 400 | Action not allowed in the dispute's current state |
| `DISPUTE_WINDOW_EXPIRED` | 400 | Network dispute window elapsed |
| `PAYOUT_MINIMUM_NOT_MET` | 400 | Amount below the currency minimum; rolls to next cycle |
| `PAYOUT_BLOCKED` | 409 | Payout is ineligible under a balance, reserve, first-payout, or destination policy |
| `ACCOUNT_UNVERIFIED` | 403 | Bank account not microdeposit-verified |
| `INVALID_DESCRIPTOR` | 400 | Statement descriptor violates network rules |

---

## 5. Pagination

### Cursor-Based (Preferred)
```bash
GET /v1/accounts?cursor=eyJpZCI6ImFjY18xMjMifQ&limit=20
```

Response:
```json
{
  "data": [...],
  "meta": {
    "page": {
      "cursor": "eyJpZCI6ImFjY18xMjMifQ",
      "next_cursor": "eyJpZCI6ImFjY1MjQifQ",
      "has_more": true,
      "total": 150
    }
  }
}
```

### Offset-Based (Legacy)
```bash
GET /v1/accounts?page=2&page_size=20
```

**Limits:** `limit`/`page_size` default 20, maximum 100. Requests above the
maximum are clamped, not rejected (the effective limit echoes in `meta.page`).
Deep pagination past 10,000 rows must use cursor pagination; offset requests
beyond that return `VALIDATION_FAILED`.

---

## 6. Filtering & Sorting

### Standard Query Parameters
| Param | Format | Example |
|-------|--------|---------|
| `filter[field]` | `filter[status]=ACTIVE` | Exact match |
| `filter[field][op]` | `filter[amount][gte]=100` | Operators: `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`, `contains`, `starts_with` |
| `sort` | `sort=-created_at,account_number` | Prefix `-` for DESC |
| `include` | `include=tenant,balance` | Nested resources |

### Field Limits
| Field | Limit | On violation |
|-------|-------|--------------|
| `metadata` object | Max 50 keys; keys ≤40 chars, values ≤500 chars | `VALIDATION_FAILED` |
| `description` / `reference` | ≤255 chars | `VALIDATION_FAILED` |
| `statement_descriptor` | ≤22 chars, network charset rules | `INVALID_DESCRIPTOR` |
| Idempotency keys | ≤100 chars, stored 24h | `VALIDATION_FAILED` |

**Money representation:** request amounts are non-negative JSON integers in minor
units plus an explicit currency (for example, `amount_minor: 50000` + `USD`
means USD 500.00). New schemas should name the field `amount_minor`; a legacy
`amount` field, where shown below for compatibility, has the same minor-unit
meaning and must never be parsed as a decimal or float. The API applies an
operation-specific upper bound below JavaScript's unsafe integer limit. Decimal
strings may be returned as display-only fields, but REST, gRPC, and GraphQL use
the same minor-unit semantic. REST may call an ISO 4217 asset `currency` for
client familiarity; application code maps it to the registry-backed `asset_code`.
GraphQL exposes the same compatibility `currency` plus canonical `assetCode`.

---

## 7. REST API Endpoints

### 7.1 Tenants

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/tenants` | Provision new tenant |
| `GET` | `/v1/tenants/{tenant_id}` | Get tenant details |
| `PATCH` | `/v1/tenants/{tenant_id}` | Update tenant settings |
| `GET` | `/v1/tenants` | List tenants (platform only) |

**Create Tenant Request:**
```json
POST /v1/tenants
Idempotency-Key: tenant_prov_abc123

{
  "name": "Acme Corp",
  "region": "US-EAST-1",
  "settings": {
    "default_currency": "USD",
    "timezone": "America/New_York",
    "features": ["multi_currency", "ach", "wire"]
  }
}
```

**Create Tenant Response (201):**
```json
{
  "data": {
    "id": "ten_abc123",
    "name": "Acme Corp",
    "region": "US-EAST-1",
    "status": "ACTIVE",
    "created_at": "2026-09-10T10:00:00Z"
  },
  "request_id": "req_abc123"
}
```

API keys are created through a separate privileged endpoint. A secret is shown
once in that endpoint's response, redacted from logs/traces, stored only as a
verifier/hash where possible, and cannot be retrieved later.

---

### 7.2 Accounts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/accounts` | Create account |
| `GET` | `/v1/accounts/{account_id}` | Get account |
| `GET` | `/v1/accounts` | List accounts |
| `PATCH` | `/v1/accounts/{account_id}` | Update account |
| `POST` | `/v1/accounts/{account_id}/freeze` | Freeze account |
| `POST` | `/v1/accounts/{account_id}/unfreeze` | Unfreeze account |
| `POST` | `/v1/accounts/{account_id}/close` | Close account |
| `GET` | `/v1/accounts/{account_id}/balance` | Get balance (all types) |
| `GET` | `/v1/accounts/{account_id}/statements` | Get statements |
| `GET` | `/v1/accounts/{account_id}/entries` | Get ledger entries |
| `POST` | `/v1/bank-accounts` | Create tokenized external bank instrument |
| `POST` | `/v1/bank-accounts/{bank_account_id}/verify` | Start microdeposit verification |
| `POST` | `/v1/bank-accounts/{bank_account_id}/verify/confirm` | Confirm microdeposit amounts |

External bank accounts are payment instruments, not ledger chart accounts. Raw
routing/account numbers are accepted only through a PCI/banking-token provider
boundary and are never returned by this API.

**Create Account Request:**
```json
POST /v1/accounts
Idempotency-Key: acc_create_abc123

{
  "name": "Operating Account",
  "type": "ASSET",
  "currency": "USD",
  "metadata": {
    "description": "Main operating account",
    "external_id": "ext_123"
  }
}
```

**Create Account Response (201):**
```json
{
  "data": {
    "id": "acc_abc123",
    "tenant_id": "ten_abc123",
    "account_number": "ACC-20260910-001",
    "name": "Operating Account",
    "type": "ASSET",
    "currency": "USD",
    "status": "ACTIVE",
    "balances": {
      "posted_minor": 0,
      "available_minor": 0,
      "held_minor": 0,
      "pending_minor": 0,
      "reserved_minor": 0,
      "asset_code": "USD",
      "ledger_cursor": "0",
      "as_of": "2026-09-10T10:00:00Z"
    },
    "version": 0,
    "created_at": "2026-09-10T10:00:00Z",
    "updated_at": "2026-09-10T10:00:00Z"
  },
  "request_id": "req_abc123"
}
```

**Get Balance Response (200):**
```json
{
  "data": {
    "account_id": "acc_abc123",
    "balances": {
      "posted_minor": 1000000,
      "available_minor": 950000,
      "held_minor": 50000,
      "pending_minor": 0,
      "reserved_minor": 0,
      "asset_code": "USD",
      "ledger_cursor": "1042",
      "as_of": "2026-09-10T10:30:00Z"
    },
    "asset_code": "USD"
  },
  "request_id": "req_abc123"
}
```

---

### 7.3 Transactions

`transactions` is a compatibility resource for clients that use payment
terminology. Its `POSTED` value means an immutable ledger Posting was committed;
it is not a mutable payment workflow state. Internally, the command is named
`PostLedgerPosting`, and `transaction_id` is an alias for `posting_id` in the
public projection. Payment-intent, transfer, refund, dispute, and payout
workflow states live on their own resources.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/transactions` | Restricted internal posting-template endpoint (not merchant-public) |
| `GET` | `/v1/transactions/{transaction_id}` | Get transaction |
| `GET` | `/v1/transactions` | List transactions |
| `POST` | `/v1/transactions/{transaction_id}/reverse` | Reverse transaction |

**Post Transaction Request (Internal Transfer):**
```json
POST /v1/transactions
Idempotency-Key: txn_transfer_abc123

{
  "type": "TRANSFER",
  "description": "Move funds to savings",
  "reference": "INTERNAL-TRF-001",
  "entries": [
    {
      "account_id": "acc_source",
      "direction": "DEBIT",
      "amount_minor": 50000,
      "currency": "USD"
    },
    {
      "account_id": "acc_target",
      "direction": "CREDIT",
      "amount_minor": 50000,
      "currency": "USD"
    }
  ],
  "metadata": {
    "initiated_by": "usr_456",
    "purpose": "savings_transfer"
  }
}
```

**Post Transaction Response (201):**
```json
{
  "data": {
    "id": "txn_abc123",
    "tenant_id": "ten_abc123",
    "type": "TRANSFER",
    "status": "POSTED",
    "description": "Move funds to savings",
    "reference": "INTERNAL-TRF-001",
    "entries": [
      {
        "id": "ent_1",
        "account_id": "acc_source",
        "direction": "DEBIT",
        "amount_minor": 50000,
        "currency": "USD",
        "account_sequence": 1042
      },
      {
        "id": "ent_2",
        "account_id": "acc_target",
        "direction": "CREDIT",
        "amount_minor": 50000,
        "currency": "USD",
        "account_sequence": 88
      }
    ],
    "posted_at": "2026-09-10T10:00:00Z",
    "created_at": "2026-09-10T10:00:00Z"
  },
  "request_id": "req_abc123"
}
```

---

### 7.4 Payment Intents

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/payment-intents` | Create payment intent |
| `GET` | `/v1/payment-intents/{intent_id}` | Get payment intent |
| `POST` | `/v1/payment-intents/{intent_id}/confirm` | Confirm payment (may return 402 `card_error`) |
| `POST` | `/v1/payment-intents/{intent_id}/capture` | Capture authorized amount (full/partial) |
| `POST` | `/v1/payment-intents/{intent_id}/cancel` | Cancel payment intent (voids open auth) |
| `GET` | `/v1/payment-intents` | List payment intents |

**Create Payment Intent Request:**
```json
POST /v1/payment-intents
Idempotency-Key: pi_create_abc123

{
  "amount_minor": 10000,
  "currency": "USD",
  "payment_method_types": ["card", "ach"],
  "customer": {
    "email": "customer@example.com",
    "name": "John Doe"
  },
  "metadata": {
    "order_id": "ord_123",
    "description": "Order #123"
  },
  "capture_method": "automatic",
  "setup_future_usage": "off_session"
}
```

**Create Payment Intent Response (201):**
```json
{
  "data": {
    "id": "pi_abc123",
    "tenant_id": "ten_abc123",
    "amount_minor": 10000,
    "currency": "USD",
    "status": "REQUIRES_PAYMENT_METHOD",
    "client_token": "pi_abc123_secret_xyz789",
    "payment_method_types": ["card", "ach"],
    "metadata": { "order_id": "ord_123" },
    "created_at": "2026-09-10T10:00:00Z"
  },
  "request_id": "req_abc123"
}
```

**Confirm Payment Intent Request:**
```json
POST /v1/payment-intents/pi_abc123/confirm
Idempotency-Key: pi_confirm_abc123

{
  "payment_method": {
    "type": "card",
    "token": "tok_visa123"
  },
  "return_url": "https://merchant.com/return"
}
```

Card confirms may return `402` with `type: card_error` (declined) or succeed
with status `requires_action` when SCA/3DS challenge is needed; the client
completes the challenge at `return_url`, then re-confirms. `capture_method`
is `automatic` (charge at confirm) or `manual` (authorize now, capture later
via the capture endpoint; authorizations expire per network rules).

---

### 7.5 Transfers

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/transfers` | Create internal transfer (immediate or scheduled) |
| `GET` | `/v1/transfers/{transfer_id}` | Get transfer |
| `GET` | `/v1/transfers` | List transfers |
| `POST` | `/v1/transfers/{transfer_id}/cancel` | Cancel pending/scheduled transfer |
| `POST` | `/v1/transfers/batch` | Create batch transfer (async, max 1000 items) |
| `GET` | `/v1/transfers/batch/{batch_id}` | Get batch status + per-item results |

**Create Transfer Request:**
```json
POST /v1/transfers
Idempotency-Key: trf_abc123

{
  "from_account_id": "acc_source",
  "to_account_id": "acc_target",
  "amount_minor": 50000,
  "currency": "USD",
  "description": "Transfer to savings",
  "reference": "TRF-001",
  "execute_at": "2026-10-01T09:00:00Z",
  "recurrence": "monthly",
  "metadata": {
    "initiated_by": "usr_456"
  }
}
```

- `execute_at` (optional, RFC 3339): future-dated execution; omit for immediate transfer.
- `recurrence` (optional): `daily` | `weekly` | `monthly`; requires `execute_at`.

**Create Batch Transfer Request:**
```json
POST /v1/transfers/batch
Idempotency-Key: batch_abc123

{
  "items": [
    {"from_account_id": "acc_a", "to_account_id": "acc_b", "amount_minor": 10000, "currency": "USD", "reference": "B-001"},
    {"from_account_id": "acc_a", "to_account_id": "acc_c", "amount_minor": 20000, "currency": "USD", "reference": "B-002"}
  ],
  "metadata": {"purpose": "payroll_run_42"}
}
```

**Batch rules:** max 1000 items; items are independent (no cross-item atomicity);
per-item idempotency key = `{batch_key}:{index}`; batch status aggregates to
`COMPLETED` / `PARTIAL` / `FAILED` with per-item results on
`GET /v1/transfers/batch/{batch_id}`.

---

### 7.6 Refunds

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/refunds` | Create refund |
| `GET` | `/v1/refunds/{refund_id}` | Get refund |
| `GET` | `/v1/refunds` | List refunds |

**Create Refund Request:**
```json
POST /v1/refunds
Idempotency-Key: ref_abc123

{
  "transaction_id": "txn_abc123",
  "amount_minor": 5000,
  "reason": "customer_request",
  "metadata": {
    "reason_detail": "Wrong item shipped"
  }
}
```

---

### 7.7 Payouts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/payouts` | Create payout |
| `GET` | `/v1/payouts/{payout_id}` | Get payout |
| `GET` | `/v1/payouts` | List payouts |
| `POST` | `/v1/payouts/{payout_id}/cancel` | Cancel pending payout |
| `GET` | `/v1/payouts/schedule` | Get payout schedule + minimums |
| `PUT` | `/v1/payouts/schedule` | Update schedule (daily/weekly/monthly, cutoff) |

`instant: true` on create requests 30-minute settlement where the method allows
it (explicit fee quoted in the response); payouts below the currency minimum roll
to the next cycle; new businesses are subject to the 7-day first-payout hold;
negative available balances and required rolling reserves return
`PAYOUT_BLOCKED` with a reason in `error.details`. Recovery uses a separate
durable workflow and never mutates a posted transaction.

**Create Payout Request:**
```json
POST /v1/payouts
Idempotency-Key: payout_abc123

{
  "account_id": "acc_operating",
  "amount_minor": 100000,
  "currency": "USD",
  "method": "ACH",
  "destination": {
    "type": "bank_account",
    "bank_account_id": "ba_abc123"
  },
  "metadata": {
    "purpose": "vendor_payment"
  }
}
```

---

### 7.8 Reconciliation

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/reconciliation/runs` | Trigger reconciliation |
| `GET` | `/v1/reconciliation/runs/{run_id}` | Get reconciliation run |
| `GET` | `/v1/reconciliation/runs` | List runs |
| `GET` | `/v1/reconciliation/breaks` | List breaks |
| `POST` | `/v1/reconciliation/breaks/{break_id}/resolve` | Resolve break |
| `POST` | `/v1/reconciliation/breaks/{break_id}/acknowledge` | Acknowledge break |

---

### 7.9 Reports

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/reports` | Generate report |
| `GET` | `/v1/reports/{report_id}` | Get report status/download |
| `GET` | `/v1/reports` | List reports |
| `GET` | `/v1/reports/templates` | List report templates |

**Generate Report Request:**
```json
POST /v1/reports

{
  "template": "trial_balance",
  "parameters": {
    "period_start": "2026-09-01",
    "period_end": "2026-09-30",
    "account_types": ["ASSET", "LIABILITY", "EQUITY"],
    "format": "pdf"
  },
  "delivery": {
    "type": "download",
    "webhook_url": "https://merchant.com/reports/webhook"
  }
}
```

---

### 7.10 Periods

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/periods` | List periods |
| `GET` | `/v1/periods/{period_id}` | Get period |
| `POST` | `/v1/periods/{period_id}/close` | Close period |
| `POST` | `/v1/periods/{period_id}/reopen` | Reopen period (admin) |

---

### 7.11 Disputes

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/disputes` | Open dispute (usually via network webhook; manual for tests) |
| `GET` | `/v1/disputes/{dispute_id}` | Get dispute with deadline + fee |
| `GET` | `/v1/disputes` | List disputes (filters: status, date_range) |
| `POST` | `/v1/disputes/{dispute_id}/evidence` | Submit evidence documents |
| `POST` | `/v1/disputes/{dispute_id}/represent` | Represent once with new evidence |
| `POST` | `/v1/disputes/{dispute_id}/close` | Close as lost (accept liability) |

---

### 7.12 Top-ups

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/topups` | Add funds from a verified bank account |
| `GET` | `/v1/topups/{topup_id}` | Get top-up status |
| `GET` | `/v1/topups` | List top-ups |
| `POST` | `/v1/topups/{topup_id}/cancel` | Cancel pending top-up |

---

### 7.13 Search

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/v1/search/transactions?q=` | Search transactions (`field:value`, AND/OR, ranges) |
| `GET` | `/v1/search/accounts?q=` | Search accounts |

Query language: `status:POSTED amount_minor>100 currency:USD`, `AND`/`OR`, quoted
phrases, `*` prefix wildcard. Same result envelope + cursor pagination as list
endpoints. Backed by Postgres full-text/trigram indexes — no search cluster needed.

---

## 8. gRPC Service Definitions

### 8.1 Ledger Service (proto)
```protobuf
syntax = "proto3";

package ledger.v1;

import "google/protobuf/timestamp.proto";

option go_package = "example.com/go-template/api/proto/ledger/v1;ledgerv1";

service LedgerService {
  // Account operations
  rpc CreateAccount(CreateAccountRequest) returns (CreateAccountResponse);
  rpc GetAccount(GetAccountRequest) returns (GetAccountResponse);
  rpc ListAccounts(ListAccountsRequest) returns (ListAccountsResponse);
  rpc UpdateAccount(UpdateAccountRequest) returns (UpdateAccountResponse);
  rpc FreezeAccount(FreezeAccountRequest) returns (FreezeAccountResponse);
  rpc GetAccountBalance(GetAccountBalanceRequest) returns (GetAccountBalanceResponse);
  rpc GetAccountEntries(GetAccountEntriesRequest) returns (GetAccountEntriesResponse);

  // Transaction operations
  rpc PostTransaction(PostTransactionRequest) returns (PostTransactionResponse);
  rpc GetTransaction(GetTransactionRequest) returns (GetTransactionResponse);
  rpc ListTransactions(ListTransactionsRequest) returns (ListTransactionsResponse);
  rpc ReverseTransaction(ReverseTransactionRequest) returns (ReverseTransactionResponse);

  // Transfer operations
  rpc CreateTransfer(CreateTransferRequest) returns (CreateTransferResponse);
  rpc GetTransfer(GetTransferRequest) returns (GetTransferResponse);
  rpc ListTransfers(ListTransfersRequest) returns (ListTransfersResponse);
}

// Messages (illustrative excerpt; generated proto must add all imports and
// request/response messages before it is treated as a compilable contract.)
message Money {
  int64 amount_minor = 1; // Positive minor units; operation bounds apply
  string asset_code = 2;  // ISO 4217 or a registered non-fiat asset code
}

message Account {
  string id = 1;
  string tenant_id = 2;
  string account_number = 3;
  string name = 4;
  AccountType type = 5;
  string asset_code = 6;
  AccountStatus status = 7;
  Balances balances = 8;
  int64 version = 9;
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
}

enum AccountType {
  ACCOUNT_TYPE_UNSPECIFIED = 0;
  ACCOUNT_TYPE_ASSET = 1;
  ACCOUNT_TYPE_LIABILITY = 2;
  ACCOUNT_TYPE_EQUITY = 3;
  ACCOUNT_TYPE_REVENUE = 4;
  ACCOUNT_TYPE_EXPENSE = 5;
}

enum AccountStatus {
  ACCOUNT_STATUS_UNSPECIFIED = 0;
  ACCOUNT_STATUS_ACTIVE = 1;
  ACCOUNT_STATUS_FROZEN = 2;
  ACCOUNT_STATUS_CLOSED = 3;
  ACCOUNT_STATUS_PENDING_VERIFICATION = 4;
}

message MoneyBalance {
  int64 amount_minor = 1;
  string asset_code = 2;
}

message Balances {
  MoneyBalance posted = 1;
  MoneyBalance available = 2;
  MoneyBalance held = 3;
  MoneyBalance pending = 4;
  MoneyBalance reserved = 5;
  int64 ledger_cursor = 6;
  google.protobuf.Timestamp as_of = 7;
}
```

**Protocol parity rule:** every capability in §7 (including scheduled-transfer
fields `execute_at`/`recurrence` and the batch endpoints in §7.5) must be
reflected in the gRPC messages and GraphQL input types when they are authored
in implementation. REST is the contract of record; gRPC/GraphQL must not lag it.

---

## 9. GraphQL Schema

```graphql
# Illustrative excerpt. E13 owns the complete generated schema, including the
# connection, input, enum, and payload types referenced below.
scalar BigInt
scalar AssetCode
scalar DateTime
scalar Currency
scalar JSON

# `currency` is the ISO-4217 compatibility alias. `assetCode` is the
# registry-backed canonical identifier and also supports registered non-fiat
# assets.

type Account {
  id: ID!
  tenantId: ID!
  accountNumber: String!
  name: String!
  type: AccountType!
  currency: Currency!
  assetCode: AssetCode!
  status: AccountStatus!
  balances: Balances!
  version: Int!
  createdAt: DateTime!
  updatedAt: DateTime!
  entries(first: Int, after: String): EntryConnection!
  statements(first: Int, after: String): StatementConnection!
}

type Balances {
  assetCode: AssetCode!
  postedMinor: BigInt!
  availableMinor: BigInt!
  heldMinor: BigInt!
  pendingMinor: BigInt!
  reservedMinor: BigInt!
  ledgerCursor: String!
  asOf: DateTime!
}

# Compatibility projection of an immutable Posting. `status` is `POSTED` for a
# committed fact; pending/failed states belong to payment/transfer workflows.
type Transaction {
  id: ID!
  tenantId: ID!
  type: TransactionType!
  status: TransactionStatus!
  description: String
  reference: String
  entries: [TransactionEntry!]!
  postedAt: DateTime
  createdAt: DateTime!
}

type TransactionEntry {
  id: ID!
  account: Account!
  direction: EntryDirection!
  amountMinor: BigInt!
  assetCode: AssetCode!
  accountSequence: String!
}

type PaymentIntent {
  id: ID!
  tenantId: ID!
  amountMinor: BigInt!
  currency: Currency!
  status: PaymentIntentStatus!
  clientToken: String
  paymentMethodTypes: [PaymentMethodType!]!
  metadata: JSON
  createdAt: DateTime!
}

type Transfer {
  id: ID!
  fromAccount: Account!
  toAccount: Account!
  amountMinor: BigInt!
  currency: Currency!
  status: TransferStatus!
  description: String
  reference: String
  createdAt: DateTime!
  completedAt: DateTime
}

# Queries
type Query {
  account(id: ID!): Account
  accounts(filter: AccountFilter, first: Int, after: String): AccountConnection!
  accountBalance(accountId: ID!): Balances!
  transaction(id: ID!): Transaction
  transactions(filter: TransactionFilter, first: Int, after: String): TransactionConnection!
  paymentIntent(id: ID!): PaymentIntent
  paymentIntents(filter: PaymentIntentFilter, first: Int, after: String): PaymentIntentConnection!
  transfer(id: ID!): Transfer
  transfers(filter: TransferFilter, first: Int, after: String): TransferConnection!
  report(id: ID!): Report
  reports(filter: ReportFilter, first: Int, after: String): ReportConnection!
  reconciliationRun(id: ID!): ReconciliationRun
  reconciliationRuns(filter: ReconciliationRunFilter, first: Int, after: String): ReconciliationRunConnection!
  breaks: BreakConnection!
}

# Mutations
type Mutation {
  createAccount(input: CreateAccountInput!): CreateAccountPayload!
  updateAccount(id: ID!, input: UpdateAccountInput!): UpdateAccountPayload!
  freezeAccount(id: ID!): FreezeAccountPayload!
  unfreezeAccount(id: ID!): UnfreezeAccountPayload!
  closeAccount(id: ID!): CloseAccountPayload!
  
  createPaymentIntent(input: CreatePaymentIntentInput!): CreatePaymentIntentPayload!
  confirmPaymentIntent(id: ID!, input: ConfirmPaymentIntentInput!): ConfirmPaymentIntentPayload!
  cancelPaymentIntent(id: ID!): CancelPaymentIntentPayload!
  
  createTransfer(input: CreateTransferInput!): CreateTransferPayload!
  cancelTransfer(id: ID!): CancelTransferPayload!
  
  createRefund(input: CreateRefundInput!): CreateRefundPayload!
  
  createPayout(input: CreatePayoutInput!): CreatePayoutPayload!
  cancelPayout(id: ID!): CancelPayoutPayload!
  
  postTransaction(input: PostTransactionInput!): PostTransactionPayload!
  reverseTransaction(id: ID!): ReverseTransactionPayload!
  
  triggerReconciliation(input: TriggerReconciliationInput!): TriggerReconciliationPayload!
  resolveBreak(id: ID!, input: ResolveBreakInput!): ResolveBreakPayload!
  acknowledgeBreak(id: ID!): AcknowledgeBreakPayload!
  
  generateReport(input: GenerateReportInput!): GenerateReportPayload!
  closePeriod(id: ID!): ClosePeriodPayload!
}

# Subscriptions
type Subscription {
  accountBalanceChanged(accountId: ID!): BalanceChangedEvent!
  transactionPosted(tenantId: ID!): TransactionPostedEvent!
  paymentSucceeded(tenantId: ID!): PaymentSucceededEvent!
  reconciliationBreakFound(tenantId: ID!): ReconciliationBreakFoundEvent!
  periodClosed(tenantId: ID!): PeriodClosedEvent!
}

# Events
type BalanceChangedEvent {
  accountId: ID!
  assetCode: AssetCode!
  postedMinor: BigInt!
  availableMinor: BigInt!
  heldMinor: BigInt!
  pendingMinor: BigInt!
  reservedMinor: BigInt!
  ledgerCursor: String!
  asOf: DateTime!
  postingId: ID!
}

type TransactionPostedEvent {
  transaction: Transaction!
  timestamp: DateTime!
}

type PaymentSucceededEvent {
  paymentIntent: PaymentIntent!
  transaction: Transaction!
  timestamp: DateTime!
}

type ReconciliationBreakFoundEvent {
  break: ReconciliationBreak!
  timestamp: DateTime!
}

type PeriodClosedEvent {
  period: Period!
  reports: [Report!]!
  timestamp: DateTime!
}
```

---

## 10. Webhook Events

Webhook objects are versioned, allow-listed projections of internal events—not a
serialization of domain structs. Idempotency keys, unrestricted metadata/PII,
internal account identifiers, actor IDs, and trace context are excluded unless a
specific public schema explicitly permits them.

### 10.1 Event Format
```json
{
  "id": "evt_abc123",
  "type": "transaction.posted",
  "api_version": "2026-09-01",
  "created": 1725962400,
  "data": {
    "object": { ... }
  },
  "livemode": true,
  "pending_webhooks": 2,
  "request_id": "req_abc123"
}
```

### 10.2 Event Types

The public `type` values below are compatibility names. Internal subjects and
domain events append the schema version (for example,
`ledger.<tenant>.payment.settled.v1`); `api_version` governs the public
allow-listed projection.

| Event Type | Description | Payload |
|------------|-------------|---------|
| `account.created` | New account created | Account |
| `account.updated` | Account modified | Account |
| `account.frozen` | Account frozen | Account |
| `account.unfrozen` | Account unfrozen | Account |
| `account.closed` | Account closed | Account |
| `account.balance.changed` | Per-asset balance snapshot updated | BalanceChangedEvent |
| `transaction.posted` | Transaction committed | Transaction |
| `transaction.reversed` | Transaction reversed | Transaction |
| `transaction.failed` | Transaction failed | Transaction + error |
| `payment_intent.created` | Payment intent created | PaymentIntent |
| `payment_intent.succeeded` | Payment completed | PaymentIntent + Transaction |
| `payment_intent.failed` | Payment failed | PaymentIntent + error |
| `payment_intent.canceled` | Payment canceled | PaymentIntent |
| `payment_intent.requires_action` | SCA/3DS challenge needed | PaymentIntent + next-action |
| `payment.settled` | Provider/bank settlement confirmed | Payment + settlement trace |
| `transfer.created` | Transfer initiated | Transfer |
| `transfer.completed` | Transfer completed | Transfer |
| `transfer.failed` | Transfer failed | Transfer + error |
| `transfer.batch.received` | Batch accepted for processing | TransferBatch |
| `transfer.batch.completed` | Batch settled (COMPLETED/PARTIAL/FAILED) | TransferBatch + items |
| `refund.created` | Refund initiated | Refund |
| `refund.succeeded` | Refund completed | Refund + Transaction |
| `refund.failed` | Refund failed | Refund + error |
| `payout.created` | Payout initiated | Payout |
| `payout.pending` | Payout submitted to network | Payout |
| `payout.paid` | Payout settled | Payout |
| `payout.failed` | Payout failed | Payout + error |
| `reconciliation.run.completed` | Reconciliation finished | ReconciliationRun |
| `reconciliation.break.found` | Break detected | ReconciliationBreak |
| `reconciliation.break.resolved` | Break resolved | ReconciliationBreak |
| `period.closed` | Period closed | Period + Reports |
| `report.generated` | Report ready | Report |
| `dispute.opened` | Dispute opened (funds held + fee debited) | Dispute |
| `dispute.closed` | Dispute decided (`outcome`: won/lost) | Dispute + reversal ref |
| `topup.succeeded` | Top-up settled | TopUp |
| `topup.failed` | Top-up failed | TopUp + error |
| `account.verified` | Bank account microdeposit-verified | Account |
| `tenant.created` | Tenant provisioning committed | Tenant |

---

## 11. Webhook Security

### Signature Verification
```python
# Python example
import hmac
import hashlib
import time

def verify_webhook(raw_body, timestamp, signature, secret, tolerance=300):
    if abs(int(time.time()) - int(timestamp)) > tolerance:
        return False
    signed = f"{timestamp}.".encode() + raw_body
    expected = hmac.new(
        secret.encode(),
        signed,
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(f"v1={expected}", signature)
```

**Headers:**
| Header | Description |
|--------|-------------|
| `X-Ledger-Signature` | `v1=<hmac(timestamp + "." + raw_body)>`; multiple `v1` values allowed during rotation |
| `X-Ledger-Timestamp` | Unix timestamp |
| `X-Ledger-Event-ID` | Unique event ID |
| `X-Ledger-Key-ID` | Signing key version; endpoint accepts old/new secrets during a bounded rotation window |

### Delivery Guarantees & Versioning

- **At-least-once:** every event carries a stable `id`; receivers must dedupe
  (same contract as our own consumer framework, domain-events §5).
- **No global ordering:** events may arrive out of order across aggregates.
  Order by explicit aggregate/partition sequence, never by `created` timestamps.
- **API version pinning:** each webhook endpoint pins an `api_version`
  (see `api_version` in §10.1); payload shapes never change under a pinned version.
- **Retry horizon:** exponential backoff over ~72 hours
  (1m → 5m → 15m → 1h → 6h → 24h → 48h, 7 attempts), then DLQ + alert.
  Matches `E08-T05`; Stripe-parity horizon, not our old ~7h window.

---

## 12. Rate Limits

| Tier | Requests/Minute | Burst | Use Case |
|------|-----------------|-------|----------|
| **Free** | 60 | 10 | Development |
| **Standard** | 600 | 100 | Production |
| **Premium** | 6000 | 1000 | High-volume |
| **Custom** | Configurable | Configurable | Enterprise |

---

## 13. API Versioning Strategy

| Version | Release Date | Status | Deprecation |
|---------|--------------|--------|-------------|
| `v1` | 2026-09-10 | Current | TBD |
| `v2` | Planned | Design | N/A |

**Migration Path:**
1. New version in URL: `/v2/accounts`
2. Header fallback: `Accept-Version: 2`
3. 12-month overlap period
4. Deprecation notice 6 months before sunset

---

## 14. SDKs & Client Libraries

| Language | Package | Status |
|----------|---------|--------|
| **TypeScript/JavaScript** | `@ledger/sdk` | Planned |
| **Python** | `ledger-python` | Planned |
| **Go** | `example.com/ledger-go` (placeholder module path) | Planned |
| **Ruby** | `ledger-ruby` | Planned |
| **Java** | `com.ledger:sdk` | Planned |

---

*These API contracts are the source of truth for all implementations. Generated OpenAPI/Protobuf/GraphQL schemas must match exactly.*
