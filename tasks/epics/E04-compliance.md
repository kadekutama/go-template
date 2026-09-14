# Epic E04: Compliance Domain (Reconciliation, Periods, GDPR, AML, Approvals)

**Status:** completed
**Story Points:** 16
**Phase:** 3 (parallel with E03, E05)
**Dependencies:** E02
**SDD Gate:** G2
**Design refs:** `docs/fintech-ledger-features.md §4`, `docs/money-flow.md §6`,
`docs/user-journeys.md §2.3, §2.6, §4–§5`, `docs/domain-events.md §3.7–§3.8`

> Why a separate epic: reconciliation, period close, erasure, and approvals are
> regulated behaviors with their own state machines. Mixing them into generic
> "domain services" is how audit findings happen.

## Tasks

### E04-T01: Reconciliation matching rules + break taxonomy
**Status:** completed
**Background:** 3-way match (ledger ↔ statement) and the four break types
(money-flow §6, journeys §2.3, features §4.2).
**Files:**
- Create: `internal/domain/service/reconciliation_service.go`,
  `internal/domain/valueobject/{break_type.go,match_result.go}`,
  `internal/domain/entity/reconciliation_{run,break}.go`
**Steps:**
1. Define immutable external source snapshots/lines: source account, coverage
   interval, file/object hash, ingestion time, parser/schema version, raw lineage.
2. Versioned match rules support 1:1, 1:N, N:1, partial settlement, fees, FX, and
   timing windows. Amount tolerance is rail/currency policy, never a global epsilon;
   every match group records rule version, confidence, inputs, and decision actor.
3. Break taxonomy: MISSING_IN_LEDGER, MISSING_IN_BANK, AMOUNT_MISMATCH, DATE_MISMATCH
   (+ DUPLICATE) with required evidence fields each.
4. Auto-resolve uses versioned timing/rail policy; everything else needs a human.
5. Emit `run.started/completed`, `break.found/resolved/acknowledged`.
**Acceptance Criteria:**
- [ ] Golden statement fixtures produce exactly the expected breaks (test).
- [ ] Same input twice yields identical breaks (determinism test).
- [ ] Auto-resolve never closes AMOUNT_MISMATCH (test).
**Story Points:** 5
**Depends On:** E02-T04, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §4.2`, `docs/money-flow.md §6`, `docs/user-journeys.md §2.3`, `docs/domain-events.md §3.7`
**SDD Gate:** G2

---

### E04-T02: Break resolution + approval workflow rules
**Status:** completed
**Background:** Investigation → adjustment/acknowledge/escalate with SOX
segregation of duties (features §4.1 SOX, money-flow §6 break resolution).
**Files:**
- Create: `internal/domain/service/break_resolution.go`,
  `internal/domain/valueobject/approval.go`
**Steps:**
1. Resolution actions: ADJUST_LEDGER (constructs a balanced linked adjustment posting),
   MARK_EXTERNAL_ERROR, ESCALATE_TO_COMPLIANCE, ACKNOWLEDGE (with expiry).
2. Every resolution has reason code, evidence reference, owner, actor, and audit
   link. SoD rule: maker/resolver ≠ approver for adjustments above threshold.
3. State machine: OPEN → IN_REVIEW → RESOLVED / ACKNOWLEDGED (expiry → re-OPEN) / ESCALATED.
**Acceptance Criteria:**
- [ ] Self-approval above threshold rejected (test).
- [ ] Expired acknowledgements re-open (time-machine test with fixed Clock).
- [ ] Every resolution links the adjustment entry ID (test).
**Story Points:** 3
**Depends On:** E04-T01
**Related Docs:** `docs/fintech-ledger-features.md §4.1–§4.2`, `docs/money-flow.md §6`
**SDD Gate:** G2

---

### E04-T03: Period-close validation rules
**Status:** completed
**Background:** Journeys §2.6 gate conditions as pure rules (no unresolved workflows,
breaks resolved/acknowledged, sub-ledgers balanced, FX revalued).
**Files:**
- Create: `internal/domain/service/period_close.go`
**Steps:**
1. `ValidateClose(period, unresolvedWorkflowCount, openBreaks, subledgerDeltas, fxRevalued) []error`
   returning one error per failing check (all reported, not first-only).
2. Closing entries builder: income summary → retained earnings (balanced by construction).
3. Reopen is privileged maker-checker workflow; ordinary late corrections post
   in the next open period and retain original effective-date context.
**Acceptance Criteria:**
- [ ] All four failing checks reported together (test).
- [ ] Closing entries always balance (property test).
**Story Points:** 2
**Depends On:** E02-T05, E02-T07
**Related Docs:** `docs/user-journeys.md §2.6`, `docs/fintech-ledger-features.md §2.3`, `docs/domain-events.md §3.8`
**SDD Gate:** G2

---

### E04-T04: GDPR/CCPA erasure + data-portability domain service
**Status:** completed
**Background:** Features §4.1 (right to erasure, portability) vs immutable
ledger — the classic conflict. Rule: personal data erased, ledger integrity kept.
**Files:**
- Create: `internal/domain/service/gdpr_service.go`
**Steps:**
1. Classify fields: erasable PII (names, emails, metadata) vs immutable ledger facts
   (amounts, entries, audit trail — retained under legal basis).
2. Erasure = crypto-shredding PII fields + tombstone record + audit entry;
   portability = export bundle (accounts, transactions, entries as JSON/CSV).
3. Emit the internal `pii.erased.v1` event from `docs/domain-events.md §3.15`;
   never include erased values in the payload or a public webhook projection.
**Acceptance Criteria:**
- [ ] Erasure preserves double-entry sums (invariant test on fixture ledger).
- [ ] Export bundle round-trips through import validation (test).
- [ ] Audit entry written for every erasure (test).
**Story Points:** 3
**Depends On:** E02-T03, E02-T06
**Related Docs:** `docs/fintech-ledger-features.md §4.1`, `SPEC.md §14`
**SDD Gate:** G2

---

### E04-T05: AML/KYC hooks + regulatory report definitions
**Status:** completed
**Background:** Features §4.1 (AML/KYC, call reports, 1099, FATCA, CRS).
Transaction monitoring hooks + report field definitions (rendering is E06/E18).
**Files:**
- Create: `internal/domain/service/aml_service.go` (screening rules port),
  `internal/domain/port/aml_provider.go`,
  `internal/domain/service/regulatory_reports.go` (field definitions per report type)
**Steps:**
1. Hook points: pre-post screening (amount thresholds, velocity, watchlist) returning
   ALLOW / REVIEW (hold funds) / BLOCK with reason codes.
2. Report definitions: field lists + aggregation rules for 1099, FATCA, CRS, call reports.
3. Emits hold placement via account Hold (E02-T03); review queue entry shape defined.
**Acceptance Criteria:**
- [ ] Threshold/velocity matrix table tests.
- [ ] BLOCK/HOLD decisions carry reason codes mappable to error table (test).
- [ ] Each regulatory report type has a field-definition test (no empty reports).
**Story Points:** 3
**Depends On:** E02-T03, E02-T07
**Related Docs:** `docs/fintech-ledger-features.md §4.1`, `docs/user-journeys.md §5`
**SDD Gate:** G2

## Acceptance Criteria

- [ ] E04-T01 … E04-T05 all `completed` (count 16 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Every features §4 row has rules + tests
- [ ] Break taxonomy matches money-flow §6 and api-contracts break endpoints
- [ ] SDD gate G2 checks pass — `tasks/tracking/GATES.md#G2`
