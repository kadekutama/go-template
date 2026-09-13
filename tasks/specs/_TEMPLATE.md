# Task Specification: TASK-ID — Title

**Task:** TASK-ID  
**Spec Status:** draft  
**Author:** unassigned  
**Reviewer:** unassigned  
**Last Updated:** YYYY-MM-DD

## Objective

One observable outcome and why it matters.

## Scope

### In Scope

- Exact behavior delivered by this task.

### Out of Scope

- Adjacent behavior intentionally deferred, with owning task ID.

## Normative Inputs

- Exact file and section or accepted ADR. Do not cite a whole repository.

## Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| TASK-ID-R01 | Atomic, testable statement using MUST/MUST NOT | must |

## Executable Scenarios

| ID | Given | When | Then |
|----|-------|------|------|
| TASK-ID-S01 | Initial state | One action | Observable result and error/effect |

Include boundaries, invalid input, authorization, concurrency/idempotency, and
failure/retry scenarios when applicable.

## Interfaces and Data

Define input/output types, schema/API/event compatibility, versioning, ownership,
and migration behavior. State “none” when the task has no interface change.

## Invariants and Failure Semantics

- Domain and database invariants.
- Error codes and retryability.
- Transaction, lock, timeout, idempotency, ordering, and unknown-outcome rules.
- Security, privacy, audit, and tenant boundaries.

## Change Surface

**Owns:** files/directories this task may create or modify.  
**Coordinates:** shared/generated files requiring another owner's agreement.  
**Must Not Touch:** explicit boundaries.

## Verification Plan

| Requirement/Scenario | Proof | Command | Expected Evidence |
|----------------------|-------|---------|-------------------|
| TASK-ID-R01, TASK-ID-S01 | Unit/integration/static/manual | exact command | assertion/report path |

## Acceptance Mapping

- [ ] Every requirement and scenario appears in the verification plan.
- [ ] Backlog acceptance criteria are represented without weakening them.
- [ ] Relevant gate checks are named.

## Open Questions

None. A packet cannot be `ready` while an answer could materially change the
implementation.

## Approval

**Decision:** pending  
**Approved By:** unassigned  
**Date:** YYYY-MM-DD  
**Notes:** High-risk tasks require a reviewer different from the author.

