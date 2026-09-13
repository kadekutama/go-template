# Repository Governance — go-template

**Owner decision (2026-09-13):** `go-template/` is a standalone repository.
**Default branch:** `main`.
**Planning baseline:** `6eca386` (initial planning baseline) + `b51d5c9`
(review-round refinements). Both commits are immutable; later work branches
from them.
**Task protocol:** `tasks/SDD.md` (packets, claims, evidence, handoffs).

## 1. Boundary

- Git toplevel MUST be `go-template/` (`git rev-parse --show-toplevel`).
- No nested `.git` inside the tree; no sibling project files in commits
  (`E00-T00-S04`: parent dir must contain only `go-template/` content).
- Credentials, tokens, and remote URLs with secrets MUST NOT appear in tracked
  files.

## 2. Remote and default branch

```bash
git remote add origin <owner-remote-url>
git push -u origin main
git remote get-url origin   # must print the owner-approved URL
git rev-parse --verify HEAD # full baseline commit
git rev-parse --show-toplevel
```

Until the owner supplies `<owner-remote-url>`, the serialized local `main`
branch is the approved coordination stub. Push/PR workflow and multi-harness
claim racing activate once origin exists.

## 3. Branch and worktree naming

- One branch/worktree per task; the name MUST include the task ID:
  `feat/<TASK-ID>-short-name` (e.g. `feat/E00-T00-git-baseline`).
- Feature branches fork from `main`. Never commit directly to `main` except the
  initial baseline; land via the merge path below.

## 4. Merge and claim serialization

1. Create `tasks/claims/<TASK-ID>.md` (owner, harness, branch, exact base
   commit, start, lease) BEFORE changing implementation files.
2. One agent owns one task at a time; do not edit another task's owned files
   without coordinating with its claim owner.
3. If two claims race for the same task, the earliest merged claim wins; the
   loser stops and selects another task (`tasks/SDD.md §3.2`).
4. Lease takeover MUST record the previous claim and reason in the claim file.
5. Release a claim (`active` → `released`) only after evidence and handoff are
   complete.

## 5. Merge authority (CODEOWNERS)

`/.github/CODEOWNERS` assigns review/merge authority:

| Artifact class | Path | Owner |
|---|---|---|
| Task coordination | `tasks/claims/`, `tasks/specs/`, `tasks/evidence/`, `tasks/handoffs/`, `tasks/epics/`, `tasks/tracking/` | repository owner |
| Migrations | `internal/infrastructure/database/migration/` | repository owner |
| Generated contracts | `api/` | repository owner |
| ADRs + ledger correctness | `docs/architecture/`, `docs/ledger-core.md`, `SPEC.md` | repository owner |
| Delivery | `.github/workflows/`, `deployments/` | repository owner |
| Everything else (default) | `*` | repository owner |

Replace `@repository-owner` with the real GitHub user/team when origin is
created. Architectural deviations require an ADR BEFORE implementation; never
let code be the only record of a decision.

## 6. Fresh-checkout proof (E00-T00-R05/S02)

```bash
python3 tasks/scripts/check-tasks.py --format --graph --sdd
```

Must exit 0 with no conversation state. Record command, commit, environment,
and exit status in `tasks/evidence/<TASK-ID>.md`.
