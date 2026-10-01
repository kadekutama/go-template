# Code Review — TASK-ID

**Task:** TASK-ID
**Epic:** EPIC-ID
**Base Commit:** full-commit-sha
**Reviewed Commit:** full-commit-sha
**Working Tree:** clean or dirty with scope note
**Reviewer:** reviewer-id (claim owner for self; packet Reviewer for independent)
**Review Date:** RFC3339 timestamp
**Review Requirement:** self or independent
**Verdict:** PASS / PASS WITH MINOR ISSUES / REVISION REQUIRED / BLOCKED
**Risk Score:** Low / Moderate / High / Critical

## Summary

Plan fidelity, finding counts by severity, and highest-risk areas. State where
this durable report is stored.

## Findings

Prioritize by severity. Use stable IDs such as `CR-001`; each finding should
include location, issue, impact, evidence, plan violation, expected behavior,
recommended fix, and verification. Write “None” when there are no findings.

## Verification Performed

List commands run and their outcomes, or inspections completed. Link full command
output in `tasks/evidence/TASK-ID.md`; never claim an unrun check passed.

## Test Coverage Gaps

List missing or inadequate tests by requirement/scenario ID. Distinguish gaps from
confirmed defects. Write “None” when there are no gaps.

## SDD / Docs Gaps

List missing or inconsistent task artifacts, evidence, handoffs, docs, ADRs, or
change-surface compliance. Write “None” when there are no gaps.

## Non-blocking Observations

Only genuinely useful maintainability/design observations. Write “None” when
there are none.

## Final Review Status

Choose exactly one: PASS, PASS WITH MINOR ISSUES, REVISION REQUIRED, BLOCKED.

## Not Verified

List what could not be checked and the specific evidence/environment needed to
close each gap. Write “None” if everything applicable was verified.

## Resolution Notes

Initial state: none. After authorized fixes, append dated notes naming finding
IDs fixed, still open, or explicitly accepted with rationale, plus verification
results. Preserve earlier findings and notes; do not replace the review history.
