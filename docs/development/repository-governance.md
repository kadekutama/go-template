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
git remote get-url origin   # must print https://github.com/kadekutama/go-template
git rev-parse --verify HEAD # full baseline commit
git rev-parse --show-toplevel
```

The origin above is the shared claim-serialization path. Pushing requires an
authenticated GitHub identity (never stored in tracked files). Note: remote
`main` (`2d7c5b7` "Initial commit") shares no history with the local planning
baseline (`6eca386`/`b51d5c9`); reconciling the two `main` lines needs an
explicit owner decision — never force-push without one.

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

### 4.1 Multi-harness enforcement and collision avoidance

1. **Shared filesystem ≠ permission:** The presence of a local clone or shared
   workspace does NOT grant an agent or harness permission to edit files. Every
   agent/harness (e.g. OpenCode, Antigravity, Claude, Codex, or human) MUST verify
   an active claim (`tasks/claims/<TASK-ID>.md` with `Status: active`) matching its
   own harness identifier before modifying any code under `internal/`, `pkg/`,
   `cmd/`, `api/`, or `test/`.
2. **Working-tree freeze on released claims:** Once all claims for a task or epic
   are released, the implementation files are frozen. If a post-release review,
   linter sweep, or polish is needed before PR merge:
   - Modifying files out-of-band without an active claim is strictly forbidden.
   - The reviewing agent must either reopen the relevant claim (flipping status to
     `active`, recording the amendment rationale and updated lease) or create a
     dedicated review/polish claim (e.g., `feat/<EPIC>-review-polish` following
     `tasks/SDD.md §4`).
   - All amended changes must be backed by updated evidence and handoff before
     re-releasing the claim.
3. **Historical reconciliation precedent (Epic E02):** In E02, a second harness
   made un-claimed edits (Hare-Niemeyer allocation fix, linter refactors, module
   rename to `github.com/kadekutama/go-template`, and kernel hardening) on
   `feat/E02-ledger-domain` while all claims were released. The repository owner
   ruled to absorb the changes because all claims were released, no active claim
   was overridden, and 100% of unit tests and gates passed. However, SDD §6 had
   no enforcement moment. This precedent formalizes the requirement: starting with
   E03, un-claimed out-of-band edits are strictly blocked.
4. **Parallel execution isolation (E03+):** Phase 3 (E03, E04, E05) contains
   multiple parallel dependency-ready tasks. When multiple harnesses operate
   concurrently:
   - Each harness MUST use an isolated git worktree:
     `git worktree add ../go-template-<TASK-ID> -b feat/<TASK-ID>-<slug>`
   - Parallel tasks MUST NOT claim overlapping files in `Allowed Change Surface`.
   - Claims must be committed to git immediately upon creation to prevent race
     conditions.

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

Replace `@kadekutama` with an additional GitHub user/team if merge authority
is ever delegated. Architectural deviations require an ADR BEFORE implementation; never
let code be the only record of a decision.

## 6. Fresh-checkout proof (E00-T00-R05/S02)

```bash
python3 tasks/scripts/check-tasks.py --format --graph --sdd
```

Must exit 0 with no conversation state. Record command, commit, environment,
and exit status in `tasks/evidence/<TASK-ID>.md`.
