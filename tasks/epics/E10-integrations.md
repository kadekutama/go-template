# Epic E10: Platform Integrations (Flags, FX, Payments, Statements, SMTP)

**Status:** pending
**Story Points:** 17
**Phase:** 5.2 (parallel with E08, E09)
**Dependencies:** E06 (ports), E07.1 (distributed persistence)
**SDD Gate:** G4
**Design refs:** `SPEC.md §7.6`, `docs/money-flow.md §2.1, §5–§6`,
`docs/user-journeys.md §2.3`, `docs/fintech-ledger-features.md §3.2, §9, §12`

> Why: external providers are the flakiest part of a ledger. Each gets a port
> (E06), a sandbox fake for tests/dev, and a real adapter with breaker+retry —
> so E11–E14 never block on vendor access.

## Tasks

### E10-T01: Feature-flag provider (OpenFeature + Unleash, Valkey/in-memory fallback)
**Status:** pending
**Background:** Runtime flags for payment methods, currencies, HTTP/3, kill switches.
**Files:**
- Create: `internal/infrastructure/featureflag/{openfeature.go,unleash.go,redis_fallback.go}`
**Steps:**
1. OpenFeature v1.17.2 + Unleash provider v6.5.1; evaluation context
   (user/tenant/custom attrs); 10s refresh.
2. Valkey-backed or in-memory provider for local development; do not add Redis
   as a required product dependency.
3. Unleash 6.5 on shared Postgres per SPEC §12.2 (compose work itself is E17; here: client config + health check).
4. Implements: `FlagClient` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Flag flip changes behavior without restart (integration test with Unleash container).
- [ ] Unleash down → fallback provider serves last-known/defaults (test).
**Story Points:** 3
**Depends On:** E06-T12, E01-T08, E07.1-T01
**Related Docs:** `SPEC.md §7.6`, `SPEC.md §2` (OpenFeature v1.17.2, Unleash v6.5.1), `SPEC.md §12.2`
**SDD Gate:** G4

---

### E10-T02: FX rate provider + cache policy
**Status:** pending
**Background:** Feeds E03-T05 conversion rules; hourly refresh job runs in E14.
**Files:**
- Create: `internal/infrastructure/fx/{provider.go,rates.go}`,
  `internal/infrastructure/fx/fake/` (deterministic sandbox)
**Steps:**
1. Provider interface impl: fetch pair rates, honor Valkey 1h TTL policy (money-flow §7).
2. Staleness surfaced (not hidden): expired cache returns last-known + `stale:true` flag for callers to decide.
3. Fake with scripted rate series for tests/dev/sandbox.
4. Implements: `FxProvider` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Stale-rate path tested (frozen clock + expired TTL).
- [ ] Provider outage → breaker opens → last-known+stale served (test).
**Story Points:** 3
**Depends On:** E06-T12, E03-T05, E01-T08, E07.1-T01
**Related Docs:** `docs/money-flow.md §2.7, §7`, `docs/fintech-ledger-features.md §9`, `tasks/epics/E03-money-movement.md#E03-T05`
**SDD Gate:** G4

---

### E10-T03: Payment-processor adapter + sandbox fake
**Status:** pending
**Background:** Card/ACH/Wire/RTP charging behind the `PaymentProcessor` port so
E06/E11 never touch vendor SDKs directly (money-flow §5 timelines).
**Files:**
- Create: `internal/infrastructure/payments/{processor.go,sandbox.go,webhooks.go}`
**Steps:**
1. Interface impl: authorize/charge/refund/verify with per-method settlement-lag metadata.
2. Sandbox fake: scripted outcomes (approve/decline/timeout/insufficient), latency injection.
3. Inbound processor webhooks verified (signature) and mapped to internal `payment_intent.*` handling.
4. Breaker + timeout + retry per method; PAN never logged/stored (tokenization via E09-T06).
5. Implements: `PaymentProcessor` port (contract: E06-T12).
6. SCA/3DS: challenge outcomes map to `requires_action` (with `return_url`/`client_token`)
   vs final decline (with `decline_code`); exemption flags passed through per network rules.
**Acceptance Criteria:**
- [ ] Timeout path leaves intent PENDING and is safely retryable (test).
- [ ] No PAN in logs/VCR cassettes (grep test on fixtures).
- [ ] Settlement-lag table matches money-flow §5 per method (test).
- [ ] Challenge-required outcome yields `requires_action` (not success/failure); decline yields `decline_code` (tests).
**Story Points:** 4
**Depends On:** E06-T12, E03-T06, E01-T08, E07.1-T01
**Related Docs:** `docs/money-flow.md §2.1, §5`, `docs/fintech-ledger-features.md §3.2`, `SPEC.md §14`
**SDD Gate:** G4

---

### E10-T04: Bank statement parsers (MT940/BAI2/CSV) + normalizer
**Status:** pending
**Background:** Feeds E04-T01 matching; journeys §2.3 fetches statements in exactly these formats.
**Files:**
- Create: `internal/infrastructure/statements/{mt940.go,bai2.go,csv.go,normalize.go}` + golden fixtures
**Steps:**
1. Parsers → normalized `StatementLine{ref, amountMinor, assetCode, date, type, raw}`;
   decimal display values are never used for matching.
2. Reference normalization shared with E04-T01 fuzzy matching.
3. Golden files per format incl. edge cases (multi-page MT940, BAI2 continuation, CSV quoting/BOM).
4. Implements: `StatementParser` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Golden-file tests for all three formats incl. edge cases.
- [ ] Malformed lines collected as errors, never silently dropped (test).
**Story Points:** 3
**Depends On:** E04-T01, E06-T12, E07.1-T01
**Related Docs:** `docs/user-journeys.md §2.3`, `docs/money-flow.md §6`, `tasks/epics/E04-compliance.md#E04-T01`
**SDD Gate:** G4

---

### E10-T05: SMTP/maildev integration for notifications
**Status:** pending
**Background:** Dev/test mail delivery (maildev 3.0.0-rc.3 per SPEC §12.1); prod uses provider SMTP.
**Files:**
- Create: `internal/infrastructure/notify/{smtp.go,templates/}`
**Steps:**
1. SMTP sender with templates (payout settled, break alerts, report ready).
2. maildev in local compose (E17); messages assertable in integration tests via maildev API.
3. Implements: `SmtpSender` port (contract: E06-T12).
**Acceptance Criteria:**
- [ ] Notification content test via maildev API in integration suite.
- [ ] No real sends in dev/test (guard test).
**Story Points:** 2
**Depends On:** E06-T12, E01-T08, E07.1-T01
**Related Docs:** `SPEC.md §12.1`, `docs/user-journeys.md §2.3` (alerts)
**SDD Gate:** G4

---

### E10-T06: Integration tests for providers (G4 slice)
**Status:** pending
**Background:** G4 evidence with Testcontainers + fakes.
**Files:**
- Create: `test/integration/providers/...`
**Steps:**
1. Unleash eval, FX fetch+stale, processor sandbox outcomes, statement goldens end-to-end into E04 matching, maildev assertions.
**Acceptance Criteria:**
- [ ] All green with `-race -count=3`; no vendor credentials required (fakes only).
**Story Points:** 2
**Depends On:** E10-T01, E10-T02, E10-T03, E10-T04, E10-T05, E07-T09
**Related Docs:** `SPEC.md §10.3`
**SDD Gate:** G4

## Acceptance Criteria

- [ ] E10-T01 … E10-T06 all `completed` (count 17 SP in `tasks/tracking/PROGRESS.md`)
- [ ] E11–E14 can build against fakes with zero vendor access
- [ ] Every provider has breaker+timeout+retry wired
- [ ] SDD gate G4 checks pass — `tasks/tracking/GATES.md#G4`
