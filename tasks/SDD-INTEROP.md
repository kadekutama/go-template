# SDD Interoperability Guide

**Status:** Normative repository guidance  
**Purpose:** Allow OpenSpec, GitHub Spec Kit, OpenCode, Codex, and other
harnesses to work on the same repository without creating competing sources of
truth.

## Decision

Keep this repository's `tasks/` protocol as the canonical delivery format. Do
not add an `openspec/` or `.specify/` tree merely because a harness supports
one of those tools. Tool-specific files are adapters; they must point to the
canonical task packet and may not redefine accounting rules, task status,
acceptance criteria, or evidence requirements.

The repository deliberately combines two useful ideas from the other systems:

- OpenSpec's one-change package, delta thinking, and archive-after-merge model.
- Spec Kit's constitution, Specify → Plan → Tasks → Implement flow, analysis
  checks, and convergence loop.

Those ideas are implemented in a form that also supports a long-lived ledger
backlog, vertical delivery slices, financial proof gates, ownership leases, and
cross-harness takeover.

## Structure comparison

| Concern | OpenSpec | GitHub Spec Kit | This repository |
|---|---|---|---|
| Stable truth | `openspec/specs/` | feature artifacts plus project constitution | `SPEC.md`, `docs/ledger-core.md`, accepted ADRs |
| Unit of change | one folder in `openspec/changes/` | one feature directory with `spec.md`, `plan.md`, `tasks.md` | one task ID in `tasks/epics/` plus `tasks/specs/<ID>.md` |
| Requirements | SHALL requirements and scenarios | user stories and acceptance criteria | stable `Rxx`/`Sxx` requirements and scenarios |
| Technical design | `design.md` | `plan.md` | packet interfaces/data/failure sections plus linked design docs |
| Implementation list | change `tasks.md` | feature `tasks.md` | dependency-ordered epic tasks and packet proof plan |
| Review/analysis | human agreement before apply | clarify/checklist/analyze | packet approval, validator, reviewer, and promotion gates |
| Execution | `/opsx:apply` | `/speckit.implement` | claim → implement → evidence → handoff |
| Completion | archive deltas into specs | converge until no gaps remain | close task, retain evidence, update canonical docs/ADR, release claim |
| Coordination | tool-managed change state | feature state/branch conventions | Git claim lease, exact base commit, handoff, and acyclic DAG |

## Artifact mapping

When translating a request from another harness, use this mapping rather than
copying its directory tree:

| External artifact/step | Canonical repository action |
|---|---|
| Constitution | Read/update `AGENTS.md`, `SPEC.md`, `docs/ledger-core.md`, and `tasks/SDD.md`; record architectural changes in an ADR. |
| Explore / clarify | Write the uncertainty in the task packet's **Open Questions**; a packet is not `ready` until behavior-changing questions are resolved. |
| Specify / proposal | Fill **Objective**, **Scope**, requirements, and scenarios in `tasks/specs/<TASK-ID>.md`. |
| Plan / design | Fill interfaces, data, invariants, failure semantics, change surface, and verification plan; link exact sections, not whole files. |
| Tasks | Use the epic task as the backlog record and keep the packet's stable IDs aligned with it. |
| Implement / apply | Create a claim, mark the task `in_progress`, modify only the allowed surface, and update the handoff at checkpoints. |
| Analyze / checklist | Run `python3 tasks/scripts/check-tasks.py --format --graph --sdd` plus the relevant content checks and packet commands. |
| Converge | Re-run the gate, compare every requirement with evidence, and append a new task for every remaining gap. Never silently widen a task. |
| Archive | Mark the task completed only after evidence/review; keep the packet, evidence, and final handoff, and update the canonical design/ADR. |
| Delta spec | Describe the exact before/after in **Change Surface** and the affected canonical document sections. Do not maintain a parallel delta-spec tree. |

## Cross-harness handoff contract

A replacement agent needs only repository state. The minimum resume sequence is:

```text
AGENTS.md → tasks/SDD.md → tasks/EPICS.md → assigned epic
→ task packet → claim → handoff → evidence → git status/diff
```

If an adapter emits a `spec.md`, `plan.md`, or `tasks.md`, it must either be
ignored as generated scratch material or include a pointer to the canonical
task ID. It must not be used to mark work complete. A tool can add a command
alias, but the required repository checks and artifact paths remain unchanged.

## Recommended adapter behavior

1. Run `python3 tasks/scripts/check-tasks.py --ready` and select one
   dependency-ready task; do not infer readiness from an epic phase alone.
2. Refuse implementation when the packet is not approved, the Git base is not
   identifiable, or a claim already owns the task.
3. Treat `docs/ledger-core.md` as the accounting contract and use the packet for
   the task-specific delta.
4. Record command output and artifact paths in evidence; prose from a chat is
   never evidence.
5. On interruption, leave a handoff that names the first unverified requirement
   and exact commands to resume.

## References

- [OpenSpec concepts](https://github.com/Fission-AI/OpenSpec/blob/main/docs/overview.md)
- [GitHub Spec Kit workflow](https://github.com/github/spec-kit/blob/main/docs/index.md)
- [This repository's delivery protocol](SDD.md)
