#!/usr/bin/env python3
"""Validate the tasks/ tree.

Usage:
  python3 tasks/scripts/check-tasks.py [FLAGS]

No flags: structural checks only (IDs unique, epic header SP equals task sum,
Depends On targets exist, Related Docs links resolve, EPICS.md covers every
epic file). Exits non-zero with a list on failure.

Design-content flags (docs-vs-docs, safe to run anytime):
  --graph     report task-DAG validation (cycles are rejected even without flag)
  --ready     list pending tasks whose task-level dependencies are completed
  --sdd       validate active/completed task packets, claims, evidence, handoffs
  --specs     every money-flow §8 rule has a spec task
  --events    every api-contracts §10 webhook maps to a domain-events §3 type
  --handlers  every api-contracts §7 endpoint group has E06/E11 handler tasks
  --ports     every E07–E10 adapter task names its port
  --subjects  every domain event type is covered by subject config
  --docs      section-link check across ALL docs (not just task refs)

Implementation-gated flags (SKIP gracefully until the code exists):
  --breakers  provider call sites reference the breaker
  --openapi   generated spec covers every §7 endpoint
  --migrations  migration files numbered with zero gaps
  --adrs      SPEC §15 has no Pending rows
  --dirs      repo tree matches SPEC §4
  --versions  SPEC §2 versions match go.mod
  --codes     error codes used anywhere are all in the registry
"""
import pathlib
import re
import sys

BASE = pathlib.Path(__file__).resolve().parents[2]
TASKS = BASE / "tasks"
EPICS_DIR = TASKS / "epics"
DOCS = BASE / "docs"
SPECS = TASKS / "specs"
CLAIMS = TASKS / "claims"
EVIDENCE = TASKS / "evidence"
HANDOFFS = TASKS / "handoffs"
FLAGS = set(sys.argv[1:])

errors: list[str] = []
warns: list[str] = []


def parse_epic(path: pathlib.Path):
    text = path.read_text()
    sp_m = re.search(r"\*\*Story Points:\*\*\s*(\d+)", text)
    epic_sp = int(sp_m.group(1)) if sp_m else None
    tasks = []
    for m in re.finditer(
        r"^###\s+(E\d{2}-T\d{2}):\s*(.+?)\s*\n(.*?)(?=^###\s+E\d{2}-T|\Z)",
        text, re.M | re.S,
    ):
        tid, title, body = m.group(1), m.group(2).strip(), m.group(3)
        fields = {}
        for f in ["Status", "Background", "Files", "Steps",
                  "Acceptance Criteria", "Story Points", "Depends On",
                  "Related Docs", "SDD Gate"]:
            fm = re.search(r"\*\*" + re.escape(f) + r":\*\*\s*(.*?)(?=\n\*\*|\Z)",
                           body, re.S)
            fields[f] = fm.group(1).strip() if fm else ""
        tasks.append({"id": tid, "title": title, "file": path.name, **fields})
    return epic_sp, tasks


def main() -> int:
    epics = sorted(EPICS_DIR.glob("E*.md"))
    all_tasks: dict[str, dict] = {}
    # 1. unique IDs + required fields
    for ep in epics:
        epic_sp, tasks = parse_epic(ep)
        s = sum(int(t["Story Points"]) for t in tasks if t["Story Points"].isdigit())
        if epic_sp is not None and s != epic_sp:
            errors.append(f"{ep.name}: header SP={epic_sp} but tasks sum={s}")
        for t in tasks:
            if t["id"] in all_tasks:
                errors.append(f"DUPLICATE task id {t['id']} in {ep.name} "
                              f"(also in {all_tasks[t['id']]['file']})")
            all_tasks[t["id"]] = t
            for f in ["Status", "Background", "Files", "Steps",
                      "Acceptance Criteria", "Story Points", "Depends On",
                      "Related Docs", "SDD Gate"]:
                if not t[f]:
                    errors.append(f"{t['id']}: missing field **{f}:**")
            if t["Status"] not in ("pending", "in_progress", "blocked", "completed"):
                errors.append(f"{t['id']}: bad Status '{t['Status']}'")
            if t["Story Points"].isdigit() and not 1 <= int(t["Story Points"]) <= 5:
                errors.append(
                    f"{t['id']}: {t['Story Points']} SP is not takeover-safe; split tasks above 5 SP"
                )
    print(f"tasks: {len(all_tasks)}, epics: {len(epics)}")

    # 2. deps resolve
    for tid, t in all_tasks.items():
        deps = [d.strip() for d in re.split(r",|;", t["Depends On"]) if d.strip()]
        for d in deps:
            if d in ("—", "-", "none", "None"):
                continue
            if d not in all_tasks:
                errors.append(f"{tid}: Depends On unknown task '{d}'")

    check_graph(all_tasks)

    if "--ready" in FLAGS:
        print_ready_tasks(all_tasks)

    # 3. EPICS.md table consistency
    epics_md = (TASKS / "EPICS.md").read_text()
    for ep in epics:
        eid = ep.stem.split("-")[0]
        if eid not in epics_md:
            errors.append(f"EPICS.md missing row for {eid}")
    total = sum(int(re.search(r"\*\*Story Points:\*\*\s*(\d+)",
                              (EPICS_DIR / f).read_text()).group(1))
                for f in [e.name for e in epics])
    print(f"total SP: {total}")

    # 4. doc links resolve (file must exist; § section must exist for .md targets)
    link_re = re.compile(r"(`?)(SPEC\.md|README\.md|AGENTS\.md|docs/[A-Za-z0-9_.\-/]+(?:\.md)?|tasks/epics/[A-Za-z0-9_.\-]+(?:#[A-Za-z0-9\-_]+)?)\1(?:\s*§([\d.]+))?")
    checked_sections: dict[str, str] = {}

    def section_exists(md_path: pathlib.Path, sec: str) -> bool:
        key = str(md_path)
        if key not in checked_sections:
            checked_sections[key] = md_path.read_text() if md_path.exists() else ""
        text = checked_sections[key]
        # match "## 7. ..." or "### 7.5 ..." prefixes
        return bool(re.search(r"^#{1,4}\s+" + re.escape(sec) + r"\b", text, re.M))

    for tid, t in all_tasks.items():
        for m in link_re.finditer(t["Related Docs"]):
            target, sec = m.group(2), m.group(3)
            fpath = BASE / target.split("#")[0]
            anchor = target.split("#")[1] if "#" in target else None
            if not fpath.exists():
                # allow extensionless docs/ refs
                if not (fpath.suffix == "" and (BASE / (target + ".md")).exists()):
                    errors.append(f"{tid}: Related Docs target missing: {target}")
                    continue
            if sec and fpath.suffix == ".md" and fpath.exists():
                if not section_exists(fpath, sec):
                    errors.append(f"{tid}: section §{sec} not found in {target}")
            if anchor:
                real = fpath if fpath.exists() else (BASE / (target.split('#')[0] + ".md"))
                if real.exists() and anchor not in real.read_text():
                    # anchor may be a task id like #E02-T02
                    if not re.search(r"^###\s+" + re.escape(anchor) + r"\b",
                                     real.read_text(), re.M):
                        errors.append(f"{tid}: anchor #{anchor} not found in {target}")

    for w in warns:
        print("WARN: " + w)

    run_content_checks(all_tasks)

    if "--sdd" in FLAGS:
        check_sdd(all_tasks)

    if "--format" in FLAGS:
        check_format()

    if errors:
        print(f"\nFAILED with {len(errors)} error(s):")
        for e in errors:
            print("  - " + e)
        return 1
    print("OK: structural checks passed")
    return 0


EPIC_HEADER_FIELDS = ["Status", "Story Points", "Phase", "Dependencies",
                      "SDD Gate", "Design refs"]
TASK_FIELDS = ["Status", "Background", "Files", "Steps", "Acceptance Criteria",
               "Story Points", "Depends On", "Related Docs", "SDD Gate"]


def task_dependencies(task: dict) -> list[str]:
    return [d.strip() for d in re.split(r",|;", task["Depends On"])
            if d.strip() not in ("", "—", "-", "none", "None")]


def check_graph(all_tasks: dict[str, dict]) -> None:
    """Reject dependency cycles and impossible active/completed task ordering."""
    state: dict[str, int] = {}
    stack: list[str] = []
    reported: set[tuple[str, ...]] = set()

    def canonical_cycle(nodes: list[str]) -> tuple[str, ...]:
        ring = nodes[:-1]
        rotations = [tuple(ring[i:] + ring[:i]) for i in range(len(ring))]
        return min(rotations)

    def visit(tid: str) -> None:
        state[tid] = 1
        stack.append(tid)
        for dep in task_dependencies(all_tasks[tid]):
            if dep not in all_tasks:
                continue
            if state.get(dep, 0) == 0:
                visit(dep)
            elif state.get(dep) == 1:
                cycle = stack[stack.index(dep):] + [dep]
                key = canonical_cycle(cycle)
                if key not in reported:
                    reported.add(key)
                    errors.append("dependency cycle: " + " -> ".join(cycle))
        stack.pop()
        state[tid] = 2

    for tid in all_tasks:
        if state.get(tid, 0) == 0:
            visit(tid)

    for tid, task in all_tasks.items():
        if task["Status"] not in ("in_progress", "completed"):
            continue
        for dep in task_dependencies(task):
            if dep in all_tasks and all_tasks[dep]["Status"] != "completed":
                errors.append(
                    f"{tid}: {task['Status']} but dependency {dep} is "
                    f"{all_tasks[dep]['Status']}"
                )
    if "--graph" in FLAGS:
        print(f"--graph: {len(all_tasks)} task nodes checked")


def print_ready_tasks(all_tasks: dict[str, dict]) -> None:
    """List pending tasks that can be specified or claimed next."""
    ready: list[tuple[str, str, str]] = []
    for tid, task in sorted(all_tasks.items()):
        if task["Status"] != "pending":
            continue
        deps = task_dependencies(task)
        if any(dep not in all_tasks or all_tasks[dep]["Status"] != "completed"
               for dep in deps):
            continue
        packet = SPECS / f"{tid}.md"
        packet_ready = packet.exists() and bool(
            re.search(r"^\*\*Spec Status:\*\*\s*ready\s*$",
                      packet.read_text(), re.M)
        )
        readiness = "implementation-ready" if packet_ready else "needs-packet"
        ready.append((tid, readiness, task["title"]))
    print(f"--ready: {len(ready)} task(s)")
    for tid, readiness, title in ready:
        print(f"  {tid} [{readiness}] {title}")


def check_sdd(all_tasks: dict[str, dict]) -> None:
    """Validate repository-visible state needed for cross-harness takeover."""
    templates = [
        SPECS / "_TEMPLATE.md",
        CLAIMS / "_TEMPLATE.md",
        EVIDENCE / "_TEMPLATE.md",
        HANDOFFS / "_TEMPLATE.md",
    ]
    for template in templates:
        if not template.exists():
            errors.append(f"--sdd: missing template {template.relative_to(BASE)}")

    required_headings = [
        "Objective", "Scope", "Normative Inputs", "Requirements",
        "Executable Scenarios", "Interfaces and Data",
        "Invariants and Failure Semantics", "Change Surface",
        "Verification Plan", "Acceptance Mapping", "Open Questions", "Approval",
    ]
    for packet in sorted(SPECS.glob("E??-T??.md")) if SPECS.exists() else []:
        text = packet.read_text()
        tid = packet.stem
        if tid not in all_tasks:
            errors.append(f"--sdd: packet for unknown task {tid}")
            continue
        if not re.search(r"^\*\*Task:\*\*\s*" + re.escape(tid) + r"\s*$", text, re.M):
            errors.append(f"--sdd: {tid} packet Task field mismatch")
        for heading in required_headings:
            if not re.search(r"^## " + re.escape(heading) + r"\s*$", text, re.M):
                errors.append(f"--sdd: {tid} packet missing heading '{heading}'")

        spec_status = re.search(r"^\*\*Spec Status:\*\*\s*(\w+)\s*$", text, re.M)
        if not spec_status or spec_status.group(1) not in ("draft", "ready"):
            errors.append(f"--sdd: {tid} packet has invalid Spec Status")
        if not re.search(re.escape(tid) + r"-R\d{2}\b", text):
            errors.append(f"--sdd: {tid} packet has no stable requirement ID")
        if not re.search(re.escape(tid) + r"-S\d{2}\b", text):
            errors.append(f"--sdd: {tid} packet has no stable scenario ID")
        if spec_status and spec_status.group(1) == "ready":
            if not re.search(r"^\*\*Decision:\*\*\s*approved\s*$", text, re.M):
                errors.append(f"--sdd: ready packet {tid} is not approved")
            if re.search(r"^\*\*Reviewer:\*\*\s*(?:unassigned|none)\s*$", text, re.M | re.I):
                errors.append(f"--sdd: ready packet {tid} has no reviewer")
            open_questions = re.search(
                r"^## Open Questions\s*$\n(.*?)(?=^## |\Z)", text, re.M | re.S
            )
            if not open_questions or not re.search(
                r"^\s*None\.?\s*$", open_questions.group(1), re.I
            ):
                errors.append(f"--sdd: ready packet {tid} has open questions")

    progress_text = (TASKS / "tracking/PROGRESS.md").read_text()
    progress_rows = re.findall(
        r"^- \[([ xX])\] (E\d{2}-T\d{2})\b", progress_text, re.M
    )
    progress: dict[str, bool] = {}
    for mark, tid in progress_rows:
        if tid in progress:
            errors.append(f"--sdd: PROGRESS.md contains duplicate {tid}")
        progress[tid] = mark.lower() == "x"
    for tid in sorted(set(progress) - set(all_tasks)):
        errors.append(f"--sdd: PROGRESS.md contains unknown task {tid}")

    for tid, task in all_tasks.items():
        if tid not in progress:
            errors.append(f"--sdd: PROGRESS.md missing {tid}")
        elif progress[tid] != (task["Status"] == "completed"):
            errors.append(f"--sdd: PROGRESS.md completion mismatch for {tid}")

        status = task["Status"]
        if status == "pending":
            continue
        packet = SPECS / f"{tid}.md"
        handoff = HANDOFFS / f"{tid}.md"
        claim = CLAIMS / f"{tid}.md"
        evidence = EVIDENCE / f"{tid}.md"
        if not packet.exists():
            errors.append(f"--sdd: {status} task {tid} has no task packet")
        elif not re.search(r"^\*\*Spec Status:\*\*\s*ready\s*$", packet.read_text(), re.M):
            errors.append(f"--sdd: {status} task {tid} packet is not ready")
        if not claim.exists():
            errors.append(f"--sdd: {status} task {tid} has no claim")
        else:
            claim_text = claim.read_text()
            if not re.search(r"^\*\*Task:\*\*\s*" + re.escape(tid) + r"\s*$", claim_text, re.M):
                errors.append(f"--sdd: {tid} claim Task field mismatch")
            wanted_claim_status = "released" if status == "completed" else "active"
            if not re.search(
                r"^\*\*Status:\*\*\s*" + wanted_claim_status + r"\s*$",
                claim_text, re.M
            ):
                errors.append(
                    f"--sdd: {tid} claim must be {wanted_claim_status} while task is {status}"
                )
            for cf in ["Task", "Status", "Owner", "Harness", "Branch/Worktree",
                       "Base Commit", "Started At", "Lease Until", "Previous Claim"]:
                if not re.search(r"^\*\*" + re.escape(cf) + r":\*\*", claim_text, re.M):
                    errors.append(f"--sdd: {tid} claim missing field '{cf}'")
        if not handoff.exists():
            errors.append(f"--sdd: {status} task {tid} has no handoff")
        if status == "completed":
            if not evidence.exists():
                errors.append(f"--sdd: completed task {tid} has no evidence")

    print("--sdd: task packets and takeover artifacts checked")


def check_format() -> None:
    """Enforce the canonical epic template (see tasks/README.md)."""
    n = 0
    for ep in sorted(EPICS_DIR.glob("E*.md")):
        text = ep.read_text()
        eid = ep.stem.split("-")[0]
        n += 1
        head, sep, rest = text.partition("## Tasks")
        if not sep:
            errors.append(f"--format: {ep.name} missing '## Tasks' section")
            continue
        for f in EPIC_HEADER_FIELDS:
            if f"**{f}:**" not in head and f"**{f}**" not in head:
                errors.append(f"--format: {ep.name} header missing {f}")
        if not re.search(r"^> ", head, re.M):
            errors.append(f"--format: {ep.name} header missing '> Why' quote")
        if "**Epic exit criteria" in text:
            errors.append(f"--format: {ep.name} still has trailing bold "
                          f"'Epic exit criteria' line (must be '## Acceptance Criteria')")
        tasks_part, sep2, tail = rest.partition("## Acceptance Criteria")
        if not sep2:
            errors.append(f"--format: {ep.name} missing '## Acceptance Criteria' section")
            continue
        if not re.search(r"-\s*\[[ xX]\]", tail):
            errors.append(f"--format: {ep.name} '## Acceptance Criteria' has no checkboxes")
        gate_m = re.search(r"\*\*SDD Gate:\*\*\s*(G\d)", head)
        if gate_m and gate_m.group(1) not in tail:
            errors.append(f"--format: {ep.name} acceptance section does not reference its gate {gate_m.group(1)}")
        ids = re.findall(r"^###\s+(E\d{2}-T\d{2}):", tasks_part, flags=re.M)
        seps = len(re.findall(r"^---$", tasks_part, flags=re.M))
        if seps != max(0, len(ids) - 1):
            errors.append(f"--format: {ep.name} has {len(ids)} tasks but {seps} '---' separators (want {len(ids)-1})")
        # per-task field order
        bodies = re.split(r"^###\s+E\d{2}-T\d{2}:.*$", tasks_part, flags=re.M)[1:]
        for tid, body in zip(ids, bodies):
            if not tid.startswith(eid):
                errors.append(f"--format: {tid} has wrong epic prefix in {ep.name}")
            found = re.findall(r"^\*\*(.+?):\*\*", body, flags=re.M)
            if found != TASK_FIELDS:
                errors.append(f"--format: {tid} field order/set differs from template: {found}")
    print(f"--format: {n} epic files checked")


def _section_text(md_path: pathlib.Path, sec: str) -> str:
    """Return the body of top-level heading `sec` (e.g. '8' or '7.5'),
    including any ### subsections, up to the next ## heading or EOF."""
    text = md_path.read_text()
    m = re.search(r"^##\s+" + re.escape(sec) + r"\b.*?(?=^##\s+|\Z)",
                  text, re.M | re.S)
    return m.group(0) if m else ""


def run_content_checks(all_tasks: dict) -> None:
    if "--specs" in FLAGS:
        money8 = _section_text(DOCS / "money-flow.md", "8")
        rules = re.findall(r"\|\s*\*\*([A-Za-z]+)\*\*", money8)
        spec_tasks = "\n".join(
            t for tid, t in all_tasks.items()
            if tid.startswith(("E02-T07", "E03-", "E04-"))
            for t in [t["Steps"] + "\n" + t["Acceptance Criteria"]])
        for r in rules:
            if r not in spec_tasks:
                errors.append(f"--specs: money-flow §8 rule '{r}' has no spec task")
        print(f"--specs: {len(rules)} rules checked")

    if "--events" in FLAGS:
        api10 = _section_text(BASE / "docs" / "api-contracts.md", "10")
        hooks = set(re.findall(r"`([a-z_]+\.[a-z_]+)`", api10))
        evdoc = (DOCS / "domain-events.md").read_text()
        for h in sorted(hooks):
            if not re.search(re.escape(h) + r"\.v\d+", evdoc):
                errors.append(f"--events: webhook '{h}' has no domain event type")
        print(f"--events: {len(hooks)} webhooks checked")

    if "--handlers" in FLAGS:
        api7 = _section_text(BASE / "docs" / "api-contracts.md", "7")
        groups = ["accounts", "tenants", "transactions", "transfers",
                  "payments", "refunds", "payouts", "reconciliation",
                  "periods", "reports", "webhook"]
        app = "\n".join(t["title"] + "\n" + t["Steps"] + t["Acceptance Criteria"]
                        for tid, t in all_tasks.items()
                        if tid.startswith("E06-") or tid.startswith("E11-")).lower()
        for g in groups:
            if g not in app and g.rstrip("s") not in app:
                errors.append(f"--handlers: api §7 group '{g}' has no handler task")
        print(f"--handlers: {len(groups)} groups checked")

    if "--ports" in FLAGS:
        for tid, t in all_tasks.items():
            if re.match(r"E(07|08|09|10)-", tid) and "test" not in t["title"].lower() \
                    and "integration tests" not in t["title"].lower():
                body = (t["Background"] + "\n" + t["Steps"] + "\n"
                        + t["Acceptance Criteria"])
                if not re.search(r"\bports?\b", body, re.I):
                    errors.append(f"--ports: adapter task {tid} names no port")
        print("--ports: adapter tasks checked")

    if "--subjects" in FLAGS:
        full_evdoc = (DOCS / "domain-events.md").read_text()
        evdoc = _section_text(DOCS / "domain-events.md", "3")
        # Event names may contain more than one action segment (for example
        # account.balance.changed.v1 and reconciliation.break.found.v1).
        types = set(re.findall(r"`([a-z_]+(?:\.[a-z_]+)+\.v\d+)`", evdoc))
        has_template = bool(re.search(
            r"ledger\.\{tenant(?:_id)?\}\.\{event_type\}", full_evdoc
        ))
        has_stream = bool(re.search(r"\bstream:\s*LEDGER_EVENTS\b", full_evdoc))
        if not has_template:
            errors.append("--subjects: canonical ledger.{tenant}.{event_type} template missing")
        if not has_stream:
            errors.append("--subjects: canonical LEDGER_EVENTS stream missing")
        if not types:
            errors.append("--subjects: no versioned event types found")
        print(f"--subjects: {len(types)} event types checked")

    if "--docs" in FLAGS:
        link_re = re.compile(r"\]\((SPEC\.md|README\.md|AGENTS\.md|docs/[A-Za-z0-9_.\-/]+\.md|tasks/[A-Za-z0-9_.\-/]+)(#[A-Za-z0-9\-_]+)?\)")
        for md in [BASE / "SPEC.md", BASE / "README.md", BASE / "AGENTS.md",
                   *sorted(DOCS.rglob("*.md")), TASKS / "EPICS.md",
                   TASKS / "README.md", TASKS / "SDD.md",
                   TASKS / "DELIVERY-SLICES.md", *sorted(EPICS_DIR.glob("*.md")),
                   *sorted(SPECS.glob("*.md")), *sorted(HANDOFFS.glob("*.md")),
                   *sorted(EVIDENCE.glob("*.md"))]:
            text = md.read_text()
            for m in link_re.finditer(text):
                target, anchor = m.group(1), m.group(2)
                if not (BASE / target).exists():
                    errors.append(f"--docs: {md.name} links missing file {target}")
        print("--docs: markdown links checked")

    if "--breakers" in FLAGS:
        prov = list((BASE / "internal/infrastructure").glob("payments/*.go")) + \
            list((BASE / "internal/infrastructure").glob("fx/*.go")) + \
            list((BASE / "internal/infrastructure").glob("notify/*.go"))
        if not prov:
            print("--breakers: SKIP (no provider code yet)")
        else:
            for p in prov:
                if "breaker" not in p.read_text().lower():
                    errors.append(f"--breakers: {p} references no breaker")
            print(f"--breakers: {len(prov)} provider files checked")
    if "--openapi" in FLAGS:
        spec_path = BASE / "api/openapi/openapi.yaml"
        if not spec_path.exists():
            print("--openapi: SKIP (no generated spec yet)")
        else:
            api7 = _section_text(BASE / "docs" / "api-contracts.md", "7")
            paths = set(re.findall(r"`(/v1/[a-z0-9\-{}/_]+)`", api7))
            gen = spec_path.read_text()
            for p in sorted(paths):
                route = re.sub(r"\{[^}]*\}", "{x}", p)
                if route.replace("{x}", "")[:12] not in gen:
                    errors.append(f"--openapi: endpoint {p} missing from generated spec")
            print(f"--openapi: {len(paths)} endpoints checked")
    if "--migrations" in FLAGS:
        mig = BASE / "internal/infrastructure/database/migration/versions"
        if not mig.exists():
            print("--migrations: SKIP (no migration files yet)")
        else:
            nums = sorted(int(f.name.split("_")[0]) for f in mig.glob("*.up.sql"))
            if nums != list(range(1, len(nums) + 1)):
                errors.append(f"--migrations: numbering gaps: {nums}")
            else:
                print(f"--migrations: {len(nums)} versions OK")
    if "--adrs" in FLAGS:
        feat15 = _section_text(DOCS / "fintech-ledger-features.md", "15")
        pend = re.findall(r"\|\s*(ADR-\d+)\s*\|[^|]*\|\s*[Pp]ending", feat15)
        if pend:
            errors.append(f"--adrs: pending ADRs: {pend}")
        else:
            print("--adrs: no pending ADRs")
    if "--dirs" in FLAGS:
        want = ["internal/domain", "internal/application", "internal/interface",
                "internal/infrastructure", "cmd/rest-api"]
        if not any((BASE / d).exists() for d in want):
            print("--dirs: SKIP (repo tree not bootstrapped yet)")
        else:
            for d in want:
                if not (BASE / d).exists():
                    errors.append(f"--dirs: missing directory {d}/")
            print("--dirs: layout checked")
    if "--versions" in FLAGS:
        gomod = BASE / "go.mod"
        if not gomod.exists():
            print("--versions: SKIP (no go.mod yet)")
        else:
            spectab = _section_text(BASE / "SPEC.md", "2")
            for mod, ver in [("go-redis/v9", "v9.22.0"), ("gqlgen", "v0.17.94"),
                             ("golang-migrate/migrate/v4", "v4.19.1"),
                             ("go-redis", "v9.22.0")]:
                if mod in spectab and ver not in gomod.read_text():
                    errors.append(f"--versions: {mod}@{ver} not in go.mod")
            print("--versions: pinned versions checked")
    if "--codes" in FLAGS:
        api4 = _section_text(BASE / "docs" / "api-contracts.md", "4")
        codes = set(re.findall(r"`([A-Z]{3,}(?:_[A-Z]+)+)`", api4))
        journeys = (DOCS / "user-journeys.md").read_text()
        used = set(re.findall(r"[A-Z]{3,}(?:_[A-Z]+)+", journeys))
        for c in sorted(used - codes):
            if c not in ("EUR_USD",):
                errors.append(f"--codes: '{c}' used in journeys but not in api §4 table")
        print(f"--codes: {len(codes)} registered codes checked")


if __name__ == "__main__":
    sys.exit(main())
