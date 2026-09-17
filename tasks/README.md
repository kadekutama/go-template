# Tasks — How To Work (AI Agent Handbook)

This directory is the single source of truth for implementation work.
Design docs (`SPEC.md`, `docs/`) say **what** to build; `tasks/` says **in what order**
and **how to prove it's done**.

`tasks/SDD.md` is the normative, harness-neutral Specification-Driven Delivery
protocol. The domain Specification pattern is a separate coding technique.
`tasks/DELIVERY-SLICES.md` turns the task DAG into incremental financial proofs.

## Layout (split to avoid context flood)

```
tasks/
├── README.md              # You are here — workflow, conventions, status values
├── EPICS.md               # Epic one-liners, phases, dependency DAG, story points, gate map
├── SDD.md                 # Task packet, claim, evidence, and takeover protocol
├── SDD-INTEROP.md         # OpenSpec/Spec Kit/harness mapping (no second backlog)
├── DELIVERY-SLICES.md     # Vertical implementation/promotion order
├── specs/                 # One approved specification packet per active task
├── claims/                # Active/released ownership and lease records
├── evidence/              # Requirement-to-proof records
├── handoffs/              # Durable resume state; chat history is never required
├── epics/
│   ├── E00-foundation.md          # Bootstrap + minimal CI
│   ├── E01-platform-core.md       # fx, config, logging, tracing, kernel
│   ├── E02-ledger-domain.md       # Ledger domain core (account/txn/journal/period/specs/events/ports)
│   ├── E03-money-movement.md      # Transfers, refunds, payouts, fees, interest, FX, payments
│   ├── E04-compliance.md          # Reconciliation rules, breaks, GDPR, AML, approvals, reports defs
│   ├── E05-tenancy.md             # Tenant aggregate, hierarchies, RLS, onboarding, residency
│   ├── E06-application.md         # Commands, queries, ports, sagas
│   ├── E07-persistence.md         # Postgres/GORM, migrations, RLS, seed, outbox
│   ├── E07.1-distributed-persistence.md # Citus 14.0 sharding, Patroni/CNPG HA, multi-node replication failover
│   ├── E08-cache-messaging.md     # Otter L1, Valkey Cluster L2, Redpanda, NATS Core, webhook dispatcher
│   ├── E09-identity-security.md   # JWT, OAuth2, Casbin, API keys, OpenBao secrets/Transit, crypto, audit
│   ├── E10-integrations.md        # Unleash, FX provider, payment processor, statements, SMTP
│   ├── E11-rest-api.md            # Echo server, middleware, all §7 endpoints, OpenAPI
│   ├── E12-grpc-api.md            # Proto, server, interceptors, gateway
│   ├── E13-graphql-api.md         # Schema, resolvers, DataLoader, subscriptions
│   ├── E14-workers.md             # Cron binary (etcd election) + 8 jobs, consumer binary + 4 groups
│   ├── E15-observability.md       # OTel, Prometheus HA, Grafana HA, Loki HA, panic recovery, HTTP/3, limits
│   ├── E16-verification.md        # Shared harness, contract, k6, litmus, coverage gates
│   ├── E17-delivery.md            # CI/CD, Dockerfiles, compose, K8s, ArgoCD
│   ├── E18-docs-dx.md             # ADRs, layer docs, API docs, runbooks, SDKs, sandbox
│   └── E19-hardening-release.md   # Headers/TLS/mTLS, vuln-zero, SBOM, PGO, backup/DR drills, release
├── tracking/
│   ├── PROGRESS.md          # Progress dashboard — the tracker. Edit this.
│   └── GATES.md             # Gate definitions + check commands. Edit this.
└── scripts/
    ├── check-tasks.py       # Validator: unique IDs, SP sums, deps exist, doc links resolve.
    └── gate-check.sh        # Per-gate executable checks (runs when code exists).
```

## Agent workflow

1. Read `AGENTS.md`, `tasks/SDD.md`, and `tasks/EPICS.md`.
2. Pick any `pending` task whose task-level `Depends On` entries are completed
   and whose applicable promotion gate permits the dependency. Epics group
   ownership; they are not horizontal execution barriers.
3. Open only that epic, the exact design sections it links, and its task packet.
4. Complete/approve `tasks/specs/<TASK-ID>.md`, then create the task claim. Do not
   implement from the short epic summary alone.
5. Mark it `in_progress`, implement scenarios by stable ID, and keep the handoff current.
6. Run the task packet's proof commands and record evidence. All must pass.
7. Mark the task `**Status:** completed` in the epic file **and** tick it in
   `tracking/PROGRESS.md`. If blocked, record the reason/evidence in the handoff.
8. When every task in the epic is `completed`, run the epic's gate checks
   (`tracking/GATES.md`), then mark the epic `completed` in `PROGRESS.md`.
9. Record any architectural deviation as an ADR (`docs/architecture/ADR-xxx.md`)
   and link it from the task.

## Status values (tasks, epics, gates)

- `pending` — not started.
- `in_progress` — exactly one task per agent at a time; approved packet + claim required.
- `blocked` — waiting on something; blocker and evidence live in the handoff.
- `completed` — acceptance criteria verified by executed commands, not by intent.

`draft` and `ready` are task-packet states, not backlog status values.

## Epic file template (every epic file follows it exactly)

```markdown
# Epic EXX: <Title>

**Status:** pending
**Story Points:** N
**Phase:** N (...)
**Dependencies:** ...
**SDD Gate:** GX
**Design refs:** ...

> Why ... (required quote: why this epic exists as a unit)

## Tasks

### EXX-T01: <Title>
**Status:** pending
**Background:** ...
**Files:** ...
**Steps:**
1. ...
**Acceptance Criteria:**
- [ ] ...
**Story Points:** N
**Depends On:** ...
**Related Docs:** ...
**SDD Gate:** GX

---

### EXX-T02: ...
...

## Acceptance Criteria

- [ ] EXX-T01 … EXX-TNN all `completed` (count N SP in `tasks/tracking/PROGRESS.md`)
- [ ] <epic-specific verifiable outcomes>
- [ ] SDD gate GX checks pass — `tasks/tracking/GATES.md#GX`
```

Rules enforced by `check-tasks.py --format`:
- header has all 6 fields + the `> Why` quote;
- exactly one `## Tasks` section; tasks separated by `---` (count = tasks − 1);
- every task has the 9 fields **in order**;
- every epic ends with exactly one `## Acceptance Criteria` section containing
  checkboxes; no trailing bold `**Epic exit criteria**` line (it was absorbed
  into tasks by mistake in an earlier revision — never reintroduce it).

## Task format (every task has all fields, in order)

`ID + Title / Status / Background / Files / Steps / Acceptance Criteria /
Story Points / Depends On / Related Docs / SDD Gate`

`Related Docs` uses exact section links, e.g. `SPEC.md §7.2`,
`docs/api-contracts.md §7.5`, `docs/money-flow.md §2.8`, `tasks/epics/E02-ledger-domain.md#E02-T02`.

## Scripts

- `tasks/scripts/check-tasks.py` — validates the tree (unique IDs, SP sums,
  deps resolve, dependency graph is acyclic, doc links resolve); `--ready` lists
  the next dependency-ready tasks; opt-in design checks are also available
  (`--specs --events --handlers --ports --subjects --docs --codes --adrs`;
  `--sdd` validates packets/claims/evidence/handoffs for active or completed tasks;
  implementation-gated ones SKIP gracefully: `--breakers --openapi
  --migrations --dirs --versions`). Run it after editing any task file.
- `tasks/scripts/gate-check.sh <G1..G8>` — executable gate checks; reads gate
  names from `tracking/GATES.md`. Self-contained bash, no extra dependencies.
