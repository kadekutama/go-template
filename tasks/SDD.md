# Specification-Driven Delivery Protocol

**Status:** Normative  
**Applies to:** Every implementation task, every AI harness, and human contributors  
**Purpose:** Make work resumable from repository state without relying on chat history

This protocol is **Specification-Driven Development (SDD)** at the delivery
level. It is different from the domain **Specification pattern** described in
`.opencode/skills/sdd-specifications.md`.

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

Use the templates in those directories. Never put credentials, tokens, personal
data, or full production payloads in an artifact.

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
and destructive operations require a reviewer different from the spec author.
Other tasks may use self-review, but must record it explicitly.

### 3.2 Claim

Create `tasks/claims/<TASK-ID>.md` before changing implementation files. A valid
claim names the owner, harness, branch/worktree, exact base commit, start time,
and lease expiry. One agent owns one task at a time. Shared filesystem access is
not permission to edit another task's owned files.

Claim creation must be serialized by the team's shared Git/PR workflow. If two
claims race, the earliest merged claim wins; the other agent stops and selects a
different task. Lease takeover requires recording the previous claim and reason.

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
2. The task commands and all affected lower-level gate commands pass from a clean
   checkout. Record command, commit, environment, exit status, and artifact link
   in `tasks/evidence/<TASK-ID>.md`.
3. The reviewer verifies high-risk changes against the task packet rather than
   only reviewing the diff.
4. The handoff contains no remaining required work.
5. Only then mark the epic task and progress checkbox `completed` and release the
   claim. Gate completion is separate from task completion.

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

- One branch/worktree per task; branch names include the task ID.
- Avoid two active tasks owning the same files. If unavoidable, designate one
  owner and make the other consume a committed contract/stub.
- Generated files have one source-of-truth owner; consumers do not edit them.
- Database migration numbers are reserved in the claim before authoring.
- Contract changes land before or atomically with consumers.
- Merge order follows task dependencies, not completion timestamps.

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
