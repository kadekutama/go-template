# Specification-Driven Delivery Protocol

**Status:** Normative  
**Applies to:** Every implementation task, every AI harness, and human contributors  
**Purpose:** Make work resumable from repository state without relying on chat history

This protocol is **Specification-Driven Development (SDD)** at the delivery
level. It is distinct from domain Specifications: executable business rules
within `internal/domain/`, governed by the Domain Expertise rules in `AGENTS.md`.

The [SDD interoperability guide](SDD-INTEROP.md) maps this protocol to
OpenSpec and GitHub Spec Kit terminology. It is a mapping only: this `tasks/`
tree remains the canonical source of task state and evidence.

The repository is the coordination protocol. Harness memory, private scratchpads,
and conversation transcripts are non-authoritative and must not be required by
the next agent.

## 1. Normative Precedence

When sources conflict, use this order:

1. Accepted ADRs and the owner-approved `docs/ledger-core.md` for financial correctness.
2. The assigned task packet in `tasks/specs/<TASK-ID>.md`.
3. API/schema artifacts under `api/` once generated and accepted.
4. `SPEC.md` and the task's linked design sections.
5. Narrative examples elsewhere in `docs/`.
6. Harness-specific prompts, skills, or agent profiles.

Do not silently choose between conflicting sources. Stop, record the conflict in
the task packet, and resolve it by correcting the lower-precedence source or by
an ADR.

The task-level `Depends On` graph is the execution authority. Epic phases and
`tasks/DELIVERY-SLICES.md` are planning and promotion views: they may add a
recommended sequence or gate, but they may not relax a task dependency. If a
slice or phase conflicts with the DAG, follow the DAG and record the conflict in
the affected packet.

The repository owner approved `docs/ledger-core.md` precedence and the audited
ADR-002/003/009 decisions on 2026-09-13. Their permanent ADR records are
accepted under `docs/architecture/`; ADR-011 remains proposed pending its
benchmark and owner review.

## 2. Durable Artifacts

Each task that leaves `pending` has these repository-visible artifacts:

| Artifact | Path | Purpose |
|----------|------|---------|
| Backlog record | `tasks/epics/E*.md` | ID, dependencies, scope summary, status |
| Task packet | `tasks/specs/<TASK-ID>.md` | Atomic requirements, scenarios, interfaces, and proof plan |
| Claim | `tasks/claims/<TASK-ID>.md` | Owner, harness, branch/worktree, base commit, and lease |
| Evidence | `tasks/evidence/<TASK-ID>.md` | Commands, results, artifacts, and requirement-to-test mapping |
| Handoff | `tasks/handoffs/<TASK-ID>.md` | Current state and exact resume instructions |
| Review record | `tasks/reviews/<TASK-ID>.md` | Durable review findings, verdict, verified commits, and resolution history when required by the task packet |

Use the templates in those directories. Never put credentials, tokens, personal
data, or full production payloads in an artifact.

A review record is distinct from verification evidence: test commands and their
outputs stay in `tasks/evidence/<TASK-ID>.md`; the review record links findings
and conclusions to the exact base/reviewed commits. Link a required review record
from the task handoff. The packet's `Review Requirement` determines whether a
completion record is required (`self`, `independent`, or `none`); legacy packets
without this field are not retroactively required to create reports. Any
user-selected reviewer may create or edit Markdown files anywhere in the
repository, without packet designation, `Owns` listing, active claim, claim
reopen, or write preflight. Reviewers remain read-only for non-Markdown
implementation/source/config/test files. Reviewer and Reviewer Harness fields
are informational attribution/suggestions, not identity gates; multiple
reviewers may contribute to the same report. The report's Base Commit must equal
the claim's Base Commit, and its Reviewed Commit must be a SHA on the handoff's
Base / Head line; the checker enforces content and commit binding, not reviewer
identity or claim status. Preserve prior review history when appending findings,
replies, and adjudications.

This global write rule applies across task-packet versions. A legacy checklist
or packet field that makes reviewer Markdown persistence conditional on reviewer
identity, packet `Owns`, active claim, claim reopen, a grant, or preflight is
superseded for that write. On a user-requested review, invoke the available
Markdown file tool immediately and verify the saved result; do not stop with
chat-only findings. Only an actual tool/platform denial may block persistence,
and the reviewer must report that concrete failure rather than inventing an SDD
authorization blocker.

A report is **accepted** for task completion only when its current Verdict and
Final Review Status are both PASS or both PASS WITH MINOR ISSUES, and every
required field and section passes `--sdd`. A new review round is recorded by
revising those fields **and** appending a dated resolution note that retains
the original findings and records what was fixed or explicitly accepted with
rationale. A resolution note alone, without revising the Verdict fields, never
unblocks completion; a current REVISION REQUIRED or BLOCKED verdict never
completes a task.

## 3. Task Lifecycle

### 3.1 Specify

Before production code is written:

1. Copy `tasks/specs/_TEMPLATE.md` to the task ID.
2. Resolve exact normative inputs and relevant accepted ADRs.
3. Give every requirement and executable scenario a stable ID.
4. Define in-scope and out-of-scope behavior, interfaces, failure semantics,
   concurrency/idempotency rules, security boundaries, and allowed file surface.
5. Map each requirement to a verification method and command.
6. Set `Spec Status` to `ready` only when no implementation-changing question is
   open and every dependency is satisfied or represented by an approved stub.

Financial postings, authorization, cryptography, migrations, public contracts,
and destructive operations require independent review. The user chooses the
reviewer; packet identity fields do not authorize or restrict Markdown writes.
Other tasks may use self-review or independent review, recorded in the packet.

Set `Review Requirement` to `self`, `independent`, or `none`. Use `independent`
for the high-risk classes above. Other tasks may use self-review or independent
review. Reviewer and Reviewer Harness packet fields are optional suggestions;
they do not designate or restrict the reviewer and may be omitted. Any
user-selected agent or harness may review, including multiple agents reviewing
the same task. `none` requires a brief rationale in packet Notes. A task
requiring self or independent review is not complete until its report is accepted
and linked from its handoff.

### 3.2 Claim

Create `tasks/claims/<TASK-ID>.md` before changing implementation files. A valid
claim names the owner, harness, branch/worktree, exact base commit, start time,
and lease expiry. One agent owns one task at a time. Shared filesystem access is
not permission to edit another task's owned files — nor to edit any implementation
files without an active, unexpired claim.

When releasing a claim, record an RFC3339 `Released At` timestamp. Each later
reopen/release round appends its timestamped history; never erase prior rounds.

The task claim's owner/harness identifies implementation ownership only.
Reviewers do not take implementation ownership, and no packet or claim
authorization is required before a reviewer creates or edits Markdown. Review
reports conventionally live at `tasks/reviews/<TASK-ID>.md`, but that path need
not appear in packet `Owns`. Reviewers may update other Markdown artifacts such
as a handoff when useful; they must not modify non-Markdown implementation files.
Multiple reviewers may contribute without claim-based serialization. For a
useful review, identify the candidate commit/snapshot in the handoff when
available; if implementation is changing, review the observed snapshot and
record that limitation rather than blocking the write or requiring an exclusive
review window.

Claim creation must be serialized by the team's shared Git/PR workflow. If two
claims race, the earliest merged claim wins; the other agent stops and selects a
different task. Lease takeover requires recording the previous claim and reason.
Once all claims on an epic/branch are released, implementation edits are frozen;
reviewer Markdown edits remain permitted without reopening a claim. Post-release
implementation changes require reopening a claim or following the takeover
protocol (§4).

A task claim may be released according to the implementation lifecycle in §3.4.
A later or additional review does not require reopening the claim; reviewers may
continue writing Markdown after release. Reopen/take over only when further
implementation changes are needed.

### 3.3 Implement

1. Mark the backlog task `in_progress` and update the progress dashboard.
2. Work only inside the task packet's allowed change surface. Coordinate any
   overlapping file before editing it.
3. Write tests from scenario IDs before or with implementation.
4. Change the packet first if behavior changes. Architectural deviation requires
   an ADR; never make the code the only record of a decision.
5. Update the handoff after every meaningful checkpoint and before stopping.

An agent may use any harness or tools. The required outputs and verification
commands remain identical.

### 3.4 Verify and Complete

Completion requires all of the following:

1. Every requirement maps to at least one passing test, static check, inspection,
   or explicitly justified manual proof.
2. The task commands, static analysis checks, and all affected lower-level gate commands pass from a clean
   checkout:
   - Code formatting: `gofmt -s -w .` and `goimports -l -w .`.
   - Strict linting: `make lint` (`golangci-lint run ./...` with 0 issues).
   - Race-detector tests: `CGO_ENABLED=1 CC=clang go test -v -race ./...` (zero races).
   - Target gate check: `./tasks/scripts/gate-check.sh <GATE>` (e.g. `G2`).
   - Repository structural check: `python3 tasks/scripts/check-tasks.py --format --graph --sdd --specs --events --codes`.
   Record command, commit, environment (including Pixi/CGO toolchain), exit status, and terminal output
   in `tasks/evidence/<TASK-ID>.md`.
3. The reviewer verifies high-risk changes against the task packet rather than
   only reviewing the diff. When the packet requires self or independent review,
   record the outcome in `tasks/reviews/<TASK-ID>.md`, using its template and
   linking it from the handoff. A report with `REVISION REQUIRED` or `BLOCKED`
   does not satisfy completion; findings must be fixed or explicitly accepted
   with rationale and recorded in a dated resolution note.
    When the implementer disagrees with a finding, both sides converse in the
    same review file while preserving section ownership: the implementer appends a
   dated `### Implementer reply` entry stating the disposition (`fixed` with
   the verifying commit/test, or `contested` with reason and counter-evidence)
    and never edits reviewer findings or adjudication; new implementer packets
    may coordinate the report path for replies. The reviewer
   adjudicates each contested finding in `## Resolution Notes` (withdrawn with
   reason, or held with rebuttal grounded in the packet or normative sources),
   quotes reply text rather than editing it, and may reject an implementer
   contest that does not stand. Only the
   repository owner may overrule a held finding, by explicit recorded decision
   with rationale. Replies recorded in handoffs before this rule remain valid
   history. Every finding must reach a terminal state — FIXED,
   WITHDRAWN, ACCEPTED (with named acceptor), or OPEN — and any OPEN finding
   above Low blocks completion.
4. The handoff contains no remaining implementation work and links the final
   review record when one is required.
5. Release the implementation claim when implementation ownership ends; review
   Markdown writes do not require reopening it. Mark the epic task and progress
   checkbox `completed` only after all verification passes and any required review
   is persisted, accepted, and linked. Gate completion is separate from task
   completion.

“Implemented,” screenshots, code review approval, or an agent's narrative are not
proof unless the specified checks also pass.

## 4. Takeover Protocol

A replacement agent starts without trusting previous chat context:

1. Read `AGENTS.md`, this file, the backlog record, and the task packet.
2. Verify the claim and inspect the handoff, evidence, `git status`, branch, base,
   and diff.
3. Rerun the packet's baseline/smoke command before editing.
4. Compare implemented behavior with requirement and scenario IDs.
5. Record takeover in the claim and handoff; preserve unfinished work unless the
   packet proves it should be replaced.
6. Continue from the first unverified requirement.
7. If review is required, read the review record and resolution history before
   treating findings as closed; verify its reviewed commit matches the candidate
   being resumed.
8. For a requested review, use packet, claim, handoff, evidence, and
   implementation artifacts as review context. Reviewer identity, packet `Owns`,
   and claim status are not prerequisites to write Markdown findings; do not
   modify non-Markdown implementation files.

If the handoff is missing or stale, reconstruct state from the diff and tests and
record that fact. Never infer completion from a prior agent's prose.

## 5. Task Sizing and Boundaries

A task is takeover-safe when one agent can understand its packet and produce a
reviewable change without loading an entire epic. Prefer tasks of 1–3 story
points. A 4–5 point task must have independently verifiable checkpoints; an
8-point implementation task must be split.

Each task should produce one primary behavior or contract. Separate:

- schema/contract from handler implementation;
- ledger-core persistence from workflow/reporting persistence;
- happy-path behavior from migration/backfill operations when rollout differs;
- shared test harnesses from the suites consuming them;
- provider interfaces/fakes from live provider adapters.

Task dependencies form the execution schedule. Epics are ownership/reporting
groups, not a reason to wait for an entire horizontal layer. Promotion gates
control when a capability may be depended on or exposed.

## 6. Parallel Work Rules

- **Implementation claims versus review writing:** Shared filesystem access does
  not authorize implementation/code changes without an active claim owning that
  task. User-selected reviewers may write Markdown files anywhere without a
  packet/claim authorization; they must not modify non-Markdown implementation
  files.
- **Physical worktree isolation:** When multiple agents or harnesses implement
  parallel tasks (e.g. Phase 3 across E03, E04, E05), each harness MUST operate
  in an isolated Git worktree:
  `git worktree add ../go-template-<TASK-ID> -b feat/<TASK-ID>-short-name`.
  Two implementation harnesses must NEVER share an uncommitted working
  directory. Multiple review-only agents may review the same task without a
  claim or worktree reservation; they should append attributed findings with
  unique IDs and reconcile any concurrent Markdown edits without dropping text.
- **Branch and claim naming:** One branch/worktree per task; branch names include
  the task ID.
- **Strict Change Surface isolation:** Avoid two active tasks declaring overlapping
  files in `Allowed Change Surface`. If unavoidable, designate one owner and make
  the other consume a committed contract/stub.
- **Review artifact access:** Reviewers may create/edit Markdown files at any
  repository path, regardless of packet `Owns` or claim state. Their permissions
  do not include non-Markdown implementation changes.
- **Generated files:** Generated files have one source-of-truth owner; consumers
  do not edit them.
- **Migrations:** Database migration numbers are reserved in the claim before authoring.
- **Contracts:** Contract changes land before or atomically with consumers.
- **Merge order:** Merge order follows task dependencies, not completion timestamps.
- **Working-tree freeze on release:** When all claims for a branch/epic are
  released, the branch files are frozen. Out-of-band edits without an active claim
  are prohibited; post-release refinements require reopening the claim or a formal
  takeover (§4).

## 7. Required Checks

Run before claiming work and after changing task metadata:

```bash
python3 tasks/scripts/check-tasks.py --format --graph --sdd
```

Run the assigned task's verification commands continuously. Run its gate subset
before handoff; `make test-all` is a release check, not a substitute for focused
feedback during implementation.

To select work without relying on harness memory, run:

```bash
python3 tasks/scripts/check-tasks.py --ready
```

`needs-packet` means the task is dependency-ready but still needs an approved
packet; `implementation-ready` means its packet is ready and it can be claimed.

## 8. Harness Adapters

`AGENTS.md`, `.opencode/agents/`, IDE rules, and future harness configuration are
thin adapters. They may explain how a tool operates, but must point here and may
not redefine task status, financial semantics, acceptance criteria, or evidence.

An adapter may expose aliases such as `specify`, `plan`, `tasks`, `implement`,
`analyze`, or `converge`, but each alias must resolve to the packet/claim/
evidence/handoff lifecycle above. Generated tool artifacts are pointers or
scratch material; they are not a second backlog.

For the exact OpenSpec/Spec Kit mapping and archive/convergence policy, see
`tasks/SDD-INTEROP.md`.
