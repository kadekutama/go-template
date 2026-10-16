# Epic E18: Docs + Developer Experience

**Status:** in_progress
**Story Points:** 32
**Phase:** 10
**Dependencies:** E01–E17 (documents what exists; each epic already wrote its ADRs inline)
**SDD Gate:** G8
**Design refs:** `SPEC.md §4` (docs skeleton from E00-T06), `SPEC.md §11` (SDK gen),
`SPEC.md §15` (ADR backlog incl. 4 pending), `docs/fintech-ledger-features.md §12`

> Note: content accrues during each epic (every epic file has an ADR step).
> This epic completes, indexes, and verifies — it does not start documentation.

## Tasks

### E18-T01: Record audit decisions, resolve the pending ADRs, and build index
**Status:** pending
**Background:** The ledger audit decisions for ADR-002/003/009 were approved by
the repository owner on 2026-09-13, and ADR-006
(reconciliation ingestion), ADR-007 (FX source/freshness), ADR-008 (archival),
and ADR-011 (balance materialization/authority) remain pending, alongside
proposed architectural decisions ADR-013 through ADR-017. ADR-002/003/009
already have permanent accepted records; this task must record the pending
decisions and finish the ADR index, with owner approval where noted.
**Files:**
- Create: `docs/architecture/ADR-00{2,3,6,7,8,9,11}-*.md` (and index the accepted
  ADR-010 logging decision); Modify: `docs/architecture/README.md`
**Steps:**
1. Each ADR: Context → Decision → Consequences → Alternatives, linked to the
   implementing tasks (e.g., ADR-002 → E02-T02).
2. Index table updated; `check-tasks.py --adrs` verifies every features §15 row resolved.
**Acceptance Criteria:**
- [ ] Zero "Pending" rows in the features §15 table (update it as part of this task).
- [ ] Each decision traceable to code (file links in ADR).
**Story Points:** 3
**Depends On:** E02-T02, E04-T01, E10-T02, E07-T05
**Related Docs:** `SPEC.md §15`, `tasks/epics/E02-ledger-domain.md`, `tasks/epics/E04-compliance.md`
**SDD Gate:** G8

---

### E18-T02: Layer documentation completion
**Status:** pending
**Background:** Fill the E00-T06 skeleton: architecture, domain, application,
infrastructure, api, development READMEs + per-topic pages.
**Files:**
- Modify: `docs/{architecture,domain,application,infrastructure,api,development}/**`
**Steps:**
1. Each layer doc: scope, diagrams (reuse design mermaid), key files table, cross-links.
2. Document the 11 detailed verification fixes (subjects, spellings, error codes) as decided outcomes.
3. Sync `AGENTS.md` + `.opencode/agents|skills|permissions` with the final epic/task structure
   (roles reference real epic IDs; skills reference real paths/versions).
4. Link-check pass over all docs.
**Acceptance Criteria:**
- [ ] `check-tasks.py --docs` (link check) passes with zero broken section links.
- [ ] Every epic's "Related Docs" links resolve (validates this whole tasks/ tree).
- [ ] A fresh agent following only `AGENTS.md` + `tasks/README.md` reaches the right epic file (review walkthrough).
**Story Points:** 4
**Depends On:** E02-T09, E06-T08, E07-T06, E11-T09
**Related Docs:** `SPEC.md §4`, `tasks/README.md`
**SDD Gate:** G8

---

### E18-T03: API documentation publishing
**Status:** pending
**Background:** OpenAPI/Swagger/ReDoc, GraphQL Voyager/Playground, buf docs, Postman.
**Files:**
- Modify: `api/openapi/openapi.yaml` (final regen), `docs/api/*`
**Steps:**
1. Regen + publish: Swagger UI `/docs/swagger`, ReDoc, Voyager/Playground (dev-only),
   `buf doc`, Postman collection artifact.
2. Every example request/response validated against live handlers (contract suite E16-T02).
**Acceptance Criteria:**
- [ ] Docs build has zero warnings; examples execute green.
**Story Points:** 3
**Depends On:** E11-T08, E12-T01, E13-T01, E16-T02
**Related Docs:** `SPEC.md §11`, `docs/api-contracts.md §1`
**SDD Gate:** G8

---

### E18-T04: Runbooks (deployment, incident, migration, secrets, DR, capacity)
**Status:** pending
**Background:** Production operations per SPEC §15 phase 8.
**Files:**
- Create: `docs/development/runbooks/{deployment,incident-response,database-migration,secrets-rotation,disaster-recovery,feature-flag-rollout,capacity-planning}.md`
**Steps:**
1. Each runbook: prerequisites, steps, verification, rollback, contacts — referencing
   E07-T05 (backup scripts), E17 (pipelines), E15 (alerts) concretely.
2. DR runbook encodes RPO<5min/RTO<30min with measured timings from E19 drill.
**Acceptance Criteria:**
- [ ] Each runbook reviewed by walking it against staging (sign-off line per file).
**Story Points:** 3
**Depends On:** E07-T05, E15-T02, E17-T01
**Related Docs:** `SPEC.md §15`, `docs/fintech-ledger-features.md §11.3`
**SDD Gate:** G8

---

### E18-T05: SDKs, sandbox, onboarding, webhook testing
**Status:** pending
**Background:** Features §12 developer experience.
**Files:**
- Create: `pkg/sdk/` (Go client), generator configs for TS/Python, `docs/development/onboarding.md`,
  `docs/development/webhook-testing.md`, sandbox seed profile
**Steps:**
1. Generate TS/Python/Go SDKs from OpenAPI; smoke-test each against sandbox.
2. Onboarding: 10-minute quickstart (compose → tenant → first payment → webhook receipt).
3. Webhook testing guide: local tunnel setup + signature verification walkthrough.
4. Sandbox: pre-seeded tenant + test cards + E07-T04 seed profile.
5. Test clocks: sandbox-only virtual time per tenant (backed by the `Clock` port override)
   to time-travel trials, interest accrual, renewals, and evidence deadlines in tests/demos.
**Acceptance Criteria:**
- [ ] Fresh engineer completes onboarding in ≤10 min (timed walkthrough recorded as checklist result).
- [ ] All three SDKs perform create→confirm→refund against sandbox (tests).
- [ ] Virtual clock advance of 30 days accrues interest and ages deadlines deterministically (test).
**Story Points:** 3
**Depends On:** E11-T08, E07-T04, E10-T03
**Related Docs:** `docs/fintech-ledger-features.md §12`, `docs/api-contracts.md §11`
**SDD Gate:** G8

---

### E18-T06: Durable cross-harness code review records
**Status:** completed
**Background:** The repository-local reviewer prompt requires a durable report per
reviewed task, but the SDD artifact inventory and validator do not yet define or
check a review-record path. Add a canonical, claim-governed report artifact and
preserve compatibility with existing task packets.
**Files:**
- Modify: `tasks/SDD.md`, `tasks/SDD-INTEROP.md`, `tasks/README.md`,
  `tasks/specs/_TEMPLATE.md`, `tasks/handoffs/_TEMPLATE.md`,
  `tasks/scripts/check-tasks.py`, `tasks/EPICS.md`, `tasks/tracking/PROGRESS.md`
- Create: `tasks/reviews/_TEMPLATE.md`, `tasks/scripts/test_check_tasks.py`,
  `tasks/specs/E18-T06.md`, `tasks/claims/E18-T06.md`,
  `tasks/evidence/E18-T06.md`, `tasks/handoffs/E18-T06.md`,
  `tasks/reviews/E18-T06.md`
**Steps:**
1. Define review records as a durable SDD artifact, including write authorization,
   handoff linkage, report metadata, resolution history, and compatibility rules.
2. Extend the task-packet template with an explicit review requirement and update
   the review-report template to match the repository reviewer prompt.
3. Extend `check-tasks.py --sdd` to validate the template and required completed
   task reports, while leaving legacy packets without the new field valid.
4. Add automated validator tests for required, independent, malformed, and legacy
   review-record cases; update the task index and progress totals.
**Acceptance Criteria:**
- [ ] `tasks/reviews/<TASK-ID>.md` is a canonical durable artifact in SDD and README.
- [ ] Review reports are only written under the task claim's authorized surface;
  handoffs link the report; evidence remains the source for test-command output.
- [ ] New packets explicitly choose a review requirement; completed tasks that
  require self or independent review must have a valid report.
- [ ] `--sdd` validates required review reports and the canonical template without
  retroactively requiring reports for legacy packets that lack the field.
- [ ] Validator tests and the full SDD structural command pass.
- [ ] Task-level G1 checks pass; E18's epic-level G8 remains a separate promotion gate.
**Story Points:** 2
**Depends On:** E00-T08
**Related Docs:** `tasks/SDD.md`, `tasks/README.md`, `tasks/SDD-INTEROP.md`
**SDD Gate:** G1

---

### E18-T07: Review-record validator and prompt alignment hardening
**Status:** completed
**Background:** The E18-T06 audit found the reviewer prompt underspecifies the
validator-required report fields/sections, the change-surface check accepts a
path listed anywhere in the section instead of under `Owns:`, report review
mode is case-sensitive while packet mode is not, and report SHAs are never
bound to the claim/handoff. Fix the prompt, validator, tests, and governing
sentences without touching released E18-T06 artifacts.
**Files:**
- Modify: `.opencode/agents/reviewer.md`, `tasks/SDD.md`, `tasks/README.md`,
  `tasks/scripts/check-tasks.py`, `tasks/scripts/test_check_tasks.py`,
  `tasks/EPICS.md`, `tasks/tracking/PROGRESS.md`
- Create: `tasks/specs/E18-T07.md`, `tasks/claims/E18-T07.md`,
  `tasks/evidence/E18-T07.md`, `tasks/handoffs/E18-T07.md`,
  `tasks/reviews/E18-T07.md`
**Steps:**
1. Align the reviewer prompt's durable-report requirements with the review
   template and validator (metadata fields, all nine sections, per-task title).
2. Enforce `Owns:`-stanza authorization, case-insensitive report mode, and
   Base/Reviewed SHA binding to claim/handoff in `check-tasks.py --sdd`, with
   governing SDD sentences.
3. Add validator unit tests for each fix plus regressions; update handbook and
   index/progress totals.
4. Run packet verification, self-review, and release.
**Acceptance Criteria:**
- [ ] A report written faithfully from the reviewer prompt passes `--sdd`.
- [ ] A report path listed only under Must Not Touch fails with an authorization error.
- [ ] Capitalized report mode passes; unknown SHAs and mismatched bases fail.
- [ ] All 16 validator tests and the full SDD structural command pass.
- [ ] Task-level G1 checks pass; released E18-T06 artifacts unchanged.
**Story Points:** 2
**Depends On:** E18-T06, E00-T08
**Related Docs:** `tasks/SDD.md`, `tasks/README.md`, `.opencode/agents/reviewer.md`
**SDD Gate:** G1

---

### E18-T08: Review validator precision and handbook coverage
**Status:** completed
**Background:** The second audit of E18-T06/T07 found the `Owns:` matcher is
substring-based (over-permissive), the handoff SHA binding scans the whole
handoff instead of the Base/Head line, the Verdict/Final-Status gate does not
block a PASS final status over unresolved metadata verdicts, and
`tasks/scripts/test_check_tasks.py` plus the definition of an "accepted" report
are undocumented in the handbook.
**Files:**
- Modify: `tasks/scripts/check-tasks.py`, `tasks/scripts/test_check_tasks.py`,
  `tasks/SDD.md`, `tasks/README.md`, `tasks/epics/E18-docs-dx.md`,
  `tasks/EPICS.md`, `tasks/tracking/PROGRESS.md`
- Create: `tasks/specs/E18-T08.md`, `tasks/claims/E18-T08.md`,
  `tasks/evidence/E18-T08.md`, `tasks/handoffs/E18-T08.md`,
  `tasks/reviews/E18-T08.md`
**Steps:**
1. Anchor `Owns:` authorization to whole-position path tokens; anchor handoff
   Reviewed Commit to the `**Base / Head:**` line; require completion verdicts
   in both Verdict and Final Review Status with resolution-note gating.
2. Add validator tests for each precision fix.
3. Define "accepted report" in SDD §3.4 and document the validator test suite
   in README Scripts.
4. Update index/progress totals; verify, self-review, and release.
**Acceptance Criteria:**
- [ ] Substring or prose-embedded path/SHA matches no longer authorize a report.
- [ ] A Final Review Status of PASS cannot complete a task whose Verdict is REVISION REQUIRED/BLOCKED, with or without a resolution note.
- [ ] README documents `test_check_tasks.py`; SDD defines when a report is accepted.
- [ ] All validator tests and the full SDD structural command pass.
- [ ] Task-level G1 checks pass; released E18-T06/T07 artifacts unchanged.
**Story Points:** 2
**Depends On:** E18-T07, E00-T08
**Related Docs:** `tasks/SDD.md`, `tasks/README.md`, `tasks/reviews/_TEMPLATE.md`
**SDD Gate:** G1

---

### E18-T09: Reviewer agent frontmatter and least-privilege permissions
**Status:** completed
**Background:** The reviewer agent's review-only posture is currently enforced
only by prompt instruction. OpenCode V2 frontmatter can enforce it with tool
permissions while guaranteeing the write access the SDD requires for review
records.
**Files:**
- Modify: `.opencode/agents/reviewer.md` (frontmatter only; body unchanged),
  `tasks/epics/E18-docs-dx.md`, `tasks/EPICS.md`, `tasks/tracking/PROGRESS.md`
- Create: `tasks/specs/E18-T09.md`, `tasks/claims/E18-T09.md`,
  `tasks/evidence/E18-T09.md`, `tasks/handoffs/E18-T09.md`,
  `tasks/reviews/E18-T09.md`
**Steps:**
1. Prepend V2 frontmatter: description, `mode: all`, deny-default edit with
   `tasks/reviews/**` allow, deny-default shell with narrow `ask` families,
   subagent denial.
2. Verify the prompt body is byte-identical below the block.
3. Update index/progress totals; verify, self-review, and release.
**Acceptance Criteria:**
- [ ] Frontmatter is present, shape-valid, and the body is unchanged.
- [ ] The agent can write only `tasks/reviews/**`; all other writes are denied.
- [ ] Shell is denied by default with narrow `ask` families; subagents denied.
- [ ] Full SDD structural command and task gate G1 pass.
**Story Points:** 1
**Depends On:** E18-T08, E00-T08
**Related Docs:** `tasks/SDD.md`, `.opencode/agents/reviewer.md`
**SDD Gate:** G1

---

### E18-T10: Reviewer persistence checklist and handoff-link handoff
**Status:** completed
**Background:** A live session showed the reviewer staying in chat instead of
writing the authorized report: persistence is phrased as a conditional with an
easy chat exit, and the handoff-link step orders an edit the agent's
permissions forbid.
**Files:**
- Modify: `.opencode/agents/reviewer.md` (persistence paragraphs only),
  `tasks/epics/E18-docs-dx.md`, `tasks/EPICS.md`, `tasks/tracking/PROGRESS.md`
- Create: `tasks/specs/E18-T10.md`, `tasks/claims/E18-T10.md`,
  `tasks/evidence/E18-T10.md`, `tasks/handoffs/E18-T10.md`,
  `tasks/reviews/E18-T10.md`
**Steps:**
1. Declare persistence the default outcome in Mode; ending in chat without the
   checklist is incomplete.
2. Restructure persistence as a numbered checklist in validator order with a
   re-read confirmation, an owner-executed handoff step, and a burden-of-proof
   fallback quoting the failed check.
3. Update index/progress totals; verify, self-review, and release.
**Acceptance Criteria:**
- [ ] Checklist order matches the validator's enforcement order.
- [ ] The handoff step is executable under denied handoff edits.
- [ ] Fallback requires file-and-line evidence of the failed check.
- [ ] Full SDD structural command and task gate G1 pass.
**Story Points:** 1
**Depends On:** E18-T09, E00-T08
**Related Docs:** `tasks/SDD.md`, `.opencode/agents/reviewer.md`
**SDD Gate:** G1

---

### E18-T11: Review feedback loop with owner override
**Status:** completed
**Background:** Review findings can be wrong or misunderstood, and the reviewer
cannot edit handoffs while the implementer should not edit reports. Define the
two-channel reply/adjudication loop with reviewer rejection rights and a final
owner override.
**Files:**
- Modify: `tasks/SDD.md` (§3.4 block only), `tasks/handoffs/_TEMPLATE.md`
  (`## Code Review` section only), `.opencode/agents/reviewer.md`
  (adjudication paragraph only), `tasks/epics/E18-docs-dx.md`,
  `tasks/EPICS.md`, `tasks/tracking/PROGRESS.md`
- Create: `tasks/specs/E18-T11.md`, `tasks/claims/E18-T11.md`,
  `tasks/evidence/E18-T11.md`, `tasks/handoffs/E18-T11.md`,
  `tasks/reviews/E18-T11.md`
**Steps:**
1. SDD §3.4: implementer reply location/format, reviewer adjudication with
   contest-rejection rights, terminal states, owner-only override.
2. Handoff template: per-finding reply format (state, reason, evidence).
3. Reviewer prompt: adjudication paragraph (read dispositions first, withdraw
   or hold, update only notes + verdict fields).
4. Update index/progress totals; verify, self-review, and release.
**Acceptance Criteria:**
- [ ] Implementer dispositions carry state, reason, and evidence; reports untouched.
- [ ] Reviewer may reject contests with rebuttal; never edits Findings/handoff.
- [ ] Only the owner may overrule a held finding, recorded and final.
- [ ] Full SDD structural command and task gate G1 pass.
**Story Points:** 1
**Depends On:** E18-T10, E00-T08
**Related Docs:** `tasks/SDD.md`, `.opencode/agents/reviewer.md`
**SDD Gate:** G1

---

### E18-T12: AGENTS.md review-response duty
**Status:** completed
**Background:** Implementer agents auto-load `AGENTS.md` but it states no
review-reply duty, so every feedback loop needs a manual prompt. A standing
paragraph there makes SDD §3.4 replies default behavior.
**Files:**
- Modify: `AGENTS.md`, `tasks/epics/E18-docs-dx.md`, `tasks/EPICS.md`,
  `tasks/tracking/PROGRESS.md`
- Create: `tasks/specs/E18-T12.md`, `tasks/claims/E18-T12.md`,
  `tasks/evidence/E18-T12.md`, `tasks/handoffs/E18-T12.md`,
  `tasks/reviews/E18-T12.md`
**Steps:**
1. Add the review-response pointer paragraph to the Mandatory Delivery
   Protocol section.
2. Update index/progress totals; verify, self-review, and release.
**Acceptance Criteria:**
- [ ] Paragraph points at SDD §3.4 and the handoff template; no semantic copy.
- [ ] Full SDD structural command and task gate G1 pass.
**Story Points:** 1
**Depends On:** E18-T11, E00-T08
**Related Docs:** `tasks/SDD.md`
**SDD Gate:** G1

---

### E18-T13: Shared review file with section ownership
**Status:** completed
**Background:** Reviewer and implementer converse better in one shared file
than across two. Legitimize section-owned replies inside the review report
instead of forcing handoff placement.
**Files:**
- Modify: `tasks/SDD.md` (§3.4 block only), `tasks/handoffs/_TEMPLATE.md`
  (`## Code Review` section only), `.opencode/agents/reviewer.md`
  (adjudication paragraph only), `AGENTS.md` (review-response paragraph only),
  `tasks/epics/E18-docs-dx.md`, `tasks/EPICS.md`, `tasks/tracking/PROGRESS.md`
- Create: `tasks/specs/E18-T13.md`, `tasks/claims/E18-T13.md`,
  `tasks/evidence/E18-T13.md`, `tasks/handoffs/E18-T13.md`,
  `tasks/reviews/E18-T13.md`
**Steps:**
1. SDD §3.4: shared-file section ownership, packet coordination rule,
   E09 grandfathering.
2. Handoff template: link-plus-pointer section.
3. Reviewer prompt: adjudication reads in-file replies, quote-don't-edit.
4. AGENTS.md: reply location updated to the shared file.
5. Update index/progress totals; verify, self-review, and release.
**Acceptance Criteria:**
- [ ] Implementer appends only under `### Implementer reply`; nothing else touched.
- [ ] Reviewer quotes replies, never edits them; verdict sync and override unchanged.
- [ ] E09 handoff replies grandfathered; no second move.
- [ ] Full SDD structural command and task gate G1 pass.
**Story Points:** 1
**Depends On:** E18-T12, E00-T08
**Related Docs:** `tasks/SDD.md`, `.opencode/agents/reviewer.md`
**SDD Gate:** G1

---

### E18-T14: Harden cross-harness agent adapters
**Status:** in_progress
**Background:** Preserve the hardened OpenCode implementer/reviewer, add native
Antigravity 2 custom-agent adapters with 1:1 role-prompt parity and explicit
approval-gated execution; remove unused specialist profiles and obsolete
permission metadata, discard redundant project-specific skills, and synchronize
current docs. Recurring
implementer failures also require actionable test/config/reuse/architecture
checks. Make changed-test audits explicit and enforce the complete
modernization diagnostics from golangci-lint and gopls without disabling
analyzers. Preserve existing OpenCode shell and `.env` permission fixes.
**Files:**
- Modify: `.opencode/agents/implementer.md`, `.opencode/agents/reviewer.md`,
  `.agents/agents/implementer.md`, `.agents/agents/reviewer.md`, `AGENTS.md`,
  `docs/development/go-conventions.md`, `.golangci.yml`, `Makefile`,
  `.zed/settings.json`, `README.md`, `tasks/SDD.md`,
  `tasks/README.md`,
  `tasks/specs/_TEMPLATE.md`, `tasks/claims/_TEMPLATE.md`,
  `tasks/scripts/check-tasks.py`, `tasks/scripts/test_check_tasks.py`,
  `tasks/epics/E18-docs-dx.md`, `tasks/EPICS.md`,
  `tasks/tracking/PROGRESS.md`
- Delete: `.opencode/agents/domain-expert.md`, `.opencode/agents/api-designer.md`,
  `.opencode/agents/db-engineer.md`, `.opencode/agents/devops-engineer.md`,
  `.opencode/agents/test-engineer.md`, `.opencode/permissions/default.json`,
  `.opencode/skills/ddd-modeling.md`, `.opencode/skills/sdd-specifications.md`,
  `.opencode/skills/go-best-practices.md`, `.opencode/skills/clean-architecture.md`
- Create: `tasks/specs/E18-T14.md`, `tasks/claims/E18-T14.md`,
  `tasks/evidence/E18-T14.md`, `tasks/handoffs/E18-T14.md`,
  `tasks/reviews/E18-T14.md`
**Steps:**
1. Add implementer guidance that operationalizes `AGENTS.md`'s exact table-driven
   test policy and requires a test-by-test diff audit before handoff: identify
   each SUT signature and assert each return value exactly once inside its table
   subtest. Mirror that audit in reviewer guidance and `go-conventions.md`.
2. Require schema-backed configuration for runtime values across the typed
   `Config`, YAML defaults, JSON Schema, and validation/test alignment; distinguish
   those from constants for true invariants.
3. Require a repository search and reuse of existing layer-appropriate clock and
   error primitives before adding new implementations; record why any new seam is
   semantically distinct.
4. Add concrete architecture/SOLID and Go receiver-selection implementation
   checks plus mirrored reviewer checks, while preserving reviewer-only behavior
   and permission limits.
5. Preserve shell approval and append `.env` read rules in both profiles; retain
   all main/commit/push/destructive-operation safeguards.
6. Remove the five unused specialist profiles and unreferenced role permission
   map. Keep specialty ownership guidance in `AGENTS.md` as harness-neutral roles.
7. Remove all remaining `.opencode/skills` summaries because they duplicate or
   weaken the normative repository guidance; update the SDD distinction text.
8. Add Antigravity 2 native implementer/reviewer custom-agent files in its
   documented workspace location. Preserve every corresponding OpenCode prompt
   directive and section 1:1; differences are limited to documented Antigravity
   metadata and permission-model translation. Disable command auto-execution and
   document project-level Ask rules for implementer commands/writes and reviewer
   commands. Explicitly allowlist each role's documented read/search tools and
    required edit/command tools; the reviewer needs create/replace tools to persist
    Markdown review artifacts. Any user-selected agent may write Markdown without
    packet/Owns/active-claim checks or task-specific authorization. It must invoke
    the appropriate file tool immediately and must not stop at chat-only output.
    Explain that tool availability is separate from platform file permissions.
    Preserve OpenCode's shared review-file
   communication behavior in Antigravity: the reviewer creates a full report
   when absent; the implementer does not create one on the reviewer's behalf.
   When a report exists, append dated implementer replies and reviewer
   adjudications under `## Resolution Notes` without overwriting prior role-owned
   text; the reviewer also appends new finding cards with unique IDs. Verify each
   append by reading the report back. Reuse root `AGENTS.md`/`tasks/SDD.md` as
   authority; the platform, not prompt text, enforces permissions.
9. Scan current documentation references, update README inventory, preserve
   historical task records, and preserve unrelated E09.1 README changes.
10. Require reviewer Makefile commands to be approval-gated and limited to
     targets whose recipes/prerequisites are safe, inspection/verification-only;
     block mutating targets in review-only mode. Add narrow Ask-gated exceptions
     for standalone read-only reviewer shell inspections while keeping arbitrary
     shell commands denied. Update E18 indexes, progress totals, evidence/handoff/
     review; run profile metadata/body assertions, reference checks, structural/
     docs checks, and required verification.
 11. Enable the complete golangci-lint `modernize` suite with no analyzer
     disables or exclusions. Explicitly enable gopls modernize in the repository's
     Zed settings, surface Hint-level results from `gopls-check`, synchronize
     modernization guidance across all four role adapters, and record out-of-scope
     findings without changing E09.1 source/tests.
  12. Apply the owner-approved all-linter policy without narrowing any other
     non-deprecated analyzer; configure the targeted testifylint, test-file
     forcetypeassert, tagliatelle, paralleltest, and Clean Architecture depguard
     rules. Rerun the full configured scan and rewrite `docs/LINTERS.md` with the
     current census, policy rationale, and direct answers about parallel subtests
     and interface return types.
  13. Keep shared and both-harness unit-test guidance aligned with the upstream
      paralleltest pattern for independent/safe table subtests, preserving
      sequential execution only where shared resources/order require it. Expand
      `docs/LINTERS.md` with repository-backed interface-return code and the
      concrete-return tradeoffs, without changing Go tests or signatures.
    14. Retain `ireturn` with anchored, type-specific exceptions for the audited
        intentional interfaces; preserve its built-in defaults and generic returns.
        Remove `paralleltest.ignore-missing-subtests: true` so the refreshed census
        includes subtest misses for a later safety-reviewed sweep. Do not blindly
        edit Go tests in this config amendment.
   15. Remove reviewer packet/claim authorization and preflight gates. Allow
        user-selected reviewers to create/edit Markdown at any repository path,
        regardless of packet metadata, `Owns`, claim status, or harness; retain a
        Markdown-only write boundary for reviewer code safety. Allow multiple
        reviewers to contribute attributed, append-only findings without
        claim-based serialization. Keep `--sdd` focused on review content,
        verdict, commit binding, and handoff linkage.
     16. Remove the OpenCode-to-Antigravity default reviewer designation from
        new packets. Reviewer and harness fields are optional suggestions only;
        any user-selected agent may review without packet/claim changes first.
**Acceptance Criteria:**
- [ ] Implementer directs tests to follow every applicable `AGENTS.md` table-driven rule and records every changed test function's SUT signature, return arity, assertion count, and assertion placement; reviewer independently audits every changed test function and reports concrete violations without sampling.
- [ ] Runtime config changes update the typed config, YAML, JSON Schema, and validation/tests; constants are not used as deploy-time settings.
- [ ] Implementer directs search/reuse of existing clock/error helpers and correct layer-specific error types before adding alternatives.
- [ ] Implementer/reviewer contain concrete, repository-grounded architecture and receiver-selection checks; reviewer flags only evidenced violations and does not demand interfaces mechanically.
- [ ] The implementer retains its existing shell/edit/subagent protections; the reviewer can edit Markdown anywhere but cannot edit non-Markdown implementation files. `.env` reads ask and `.env.example` remains allowed where supported.
- [ ] Only implementer/reviewer OpenCode profiles remain; stale specialist files and `.opencode/permissions/default.json` are deleted, with no live references left.
- [ ] No `.opencode/skills` files remain; the SDD protocol explanation has no dangling skill reference; README does not advertise bundled skills.
- [ ] Antigravity adapters exist at `.agents/agents/implementer.md` and `reviewer.md`, use documented Antigravity frontmatter, defer to `AGENTS.md`/`tasks/SDD.md`, and accurately state permission-enforcement limits.
- [ ] A section-by-section parity review confirms every OpenCode role directive, workflow stage, safeguard, and review checklist is represented in the corresponding Antigravity body; only harness metadata/permission translation differs.
- [ ] Antigravity roles do not auto-execute commands: implementer actions and reviewer commands are configured to request owner approval; reviewer writes may target Markdown files anywhere but not non-Markdown files. Makefile targets with disallowed side effects are blocked in review-only mode.
- [ ] Antigravity custom-agent tool allowlists expose role-required file/search/edit and terminal tools; the reviewer can create/update Markdown review artifacts without a task-specific report-path authorization or write approval requirement. A permission rule is not treated as a substitute for a missing tool.
- [ ] When a user requests a review, the Antigravity reviewer invokes its file tool and verifies the saved report without packet/claim/preflight checks; it does not substitute chat output for persistence. Its first response begins with the exact workspace-reviewer profile-load canary, which is diagnostic only and never a persistence gate. Genuine tool/platform failures are reported accurately.
- [ ] Existing review reports remain append-only shared communication: implementers append per-finding replies, any number of reviewers append attributed findings/adjudications, and all preserve prior text and verify their changes.
- [ ] Antigravity ordinary workspace reads match OpenCode's non-prompting read behavior; it does not recommend `read_file(*) Ask`, and protected-file reads fail closed if the platform cannot provide the required approval boundary. ACL enforcement differences are stated accurately.
- [ ] OpenCode `webfetch` and Antigravity `read_url_content` have equivalent read-only access without extra prompts; Antigravity guidance documents `read_url(*)` Allow and does not expose URL/browser actuation.
- [ ] Reviewer shell access allows only approval-gated Git/task-validator commands, safe Makefile targets, and the narrow standalone read-only inspection list; arbitrary commands, command composition, protected secret reads, and non-Markdown writes remain disallowed.
- [ ] `.golangci.yml` enables the full `modernize` suite without analyzer disables/exclusions; Zed explicitly enables gopls modernize and `make gopls-check` includes Hint-level diagnostics.
- [ ] All four role profiles carry matching full-modernization instructions in their OpenCode/Antigravity role pairs; in-scope findings are addressed and out-of-scope findings are recorded without disabling analyzers or editing unowned files.
- [ ] `.golangci.yml` matches the owner-approved disable set and targeted analyzer rules; depguard enforces the documented production import direction with composition-root exceptions, JSON tags use snake_case for API/database compatibility, and `docs/LINTERS.md` reflects a fresh full scan with direct policy answers.
- [ ] `AGENTS.md`, Go conventions, and both OpenCode/Antigravity role pairs teach safe parent and table-subtest parallelism from the upstream README, `t.Cleanup` lifetime guidance, and current Go loop-variable semantics; sequential exceptions are reasoned. The linter report includes source-backed interface-return examples and risks of blanket concrete returns without source refactors.
- [ ] `ireturn` remains enabled with exact package-qualified exceptions for reviewed interface contracts and the refreshed full scan has no baseline ireturn findings; `paralleltest` does not ignore missing subtests and its expanded findings are recorded for a safety-reviewed sweep, without blanket test edits.
- [ ] Any user-selected reviewer on any harness can contribute Markdown without reviewer/packet identity matching, `Owns` membership, active claims, or authorization preflight. The validator accepts arbitrary reviewer/harness metadata and released/absent claim state when required report content, verdict, commit binding, and handoff linkage are valid.
- [ ] New task packets do not default to a specific reviewer identity or harness; optional suggestions never block review. Multiple reviewers can append attributed contributions with unique IDs and preserve prior report content.
- [ ] `AGENTS.md` describes specializations as harness-neutral ownership areas and README lists both harnesses accurately; the existing Casbin version change is preserved.
- [ ] G1 and full SDD structural validation pass; evidence records the prompt probe and repository verification.
**Story Points:** 5
**Depends On:** E18-T13
**Related Docs:** `tasks/SDD.md`, `AGENTS.md`, `README.md`, `docs/development/go-conventions.md`, `docs/architecture/clean-architecture-guide.md`
**SDD Gate:** G1

## Acceptance Criteria

- [ ] E18-T01 … E18-T14 all `completed` (count 32 SP in `tasks/tracking/PROGRESS.md`)
- [ ] Zero pending ADRs; docs link-check clean; onboarding timed ≤10 min
- [ ] SDD gate G8 checks pass — `tasks/tracking/GATES.md#G8`
