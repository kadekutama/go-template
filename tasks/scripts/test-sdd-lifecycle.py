#!/usr/bin/env python3
"""SDD lifecycle fixture tests (E00-T08).

Proves tasks/scripts/check-tasks.py accepts the valid task cycle and rejects
lifecycle violations, using a synthetic task tree in a temporary directory.
NEVER touches the real tasks/ tree (asserted at the end via git status).

Cases: valid cycle accepted; dependency cycle, active task without a ready
packet, wrong/missing claim, PROGRESS mismatch, and completion without
evidence are all rejected.
"""
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

REAL_ROOT = pathlib.Path(__file__).resolve().parents[2]
REAL_TASKS = REAL_ROOT / "tasks"
CHECK = ["--format", "--graph", "--sdd"]

PACKET = """# Task Specification: {tid} — Fixture

**Task:** {tid}  
**Spec Status:** ready  
**Author:** fixture  
**Reviewer:** fixture-reviewer  
**Last Updated:** 2026-09-13

## Objective

Fixture.

## Scope

### In Scope

- Fixture.

### Out of Scope

- Fixture.

## Normative Inputs

- `tasks/SDD.md` for the lifecycle.

## Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| {tid}-R01 | Fixture MUST pass. | must |

## Executable Scenarios

| ID | Given | When | Then |
|----|-------|------|------|
| {tid}-S01 | Fixture | It runs | It passes. |

## Interfaces and Data

None.

## Invariants and Failure Semantics

None.

## Change Surface

**Owns:** fixture.  
**Coordinates:** none.  
**Must Not Touch:** fixture.

## Verification Plan

| Requirement/Scenario | Proof | Command | Expected Evidence |
|----------------------|-------|---------|-------------------|
| {tid}-R01, {tid}-S01 | inspection | true | pass |

## Acceptance Mapping

- [x] Fixture.
- [x] Fixture.
- [x] Fixture.

## Open Questions

None.

## Approval

**Decision:** approved  
**Approved By:** fixture  
**Date:** 2026-09-13  
**Notes:** Fixture.
"""

EPIC = """# Epic E99: Fixture epic

**Status:** pending
**Story Points:** 3
**Phase:** 0
**Dependencies:** none
**SDD Gate:** G1
**Design refs:** tasks/SDD.md

> Why this epic exists: validator fixture only.

## Tasks

### E99-T00: First fixture task
**Status:** {s0}
**Background:** bg
**Files:** none
**Steps:**
1. do
**Acceptance Criteria:**
- [ ] done
**Story Points:** 1
**Depends On:** {d0}
**Related Docs:** tasks/SDD.md
**SDD Gate:** G1

---

### E99-T01: Second fixture task
**Status:** {s1}
**Background:** bg
**Files:** none
**Steps:**
1. do
**Acceptance Criteria:**
- [ ] done
**Story Points:** 2
**Depends On:** {d1}
**Related Docs:** tasks/SDD.md
**SDD Gate:** G1

## Acceptance Criteria

- [ ] E99-T00 and E99-T01 all `completed`
- [ ] fixture only
- [ ] SDD gate G1 checks pass
"""

CLAIM = """# Task Claim: {tid}

**Task:** {tid}  
**Status:** {status}  
**Owner:** fixture  
**Harness:** fixture  
**Branch/Worktree:** fixture  
**Base Commit:** abc123  
**Started At:** 2026-09-13T00:00:00Z  
**Lease Until:** 2026-09-20T00:00:00Z  
**Previous Claim:** none

## Coordination Notes

- Fixture.
"""


def build_tree(mutate):
    tmp = pathlib.Path(tempfile.mkdtemp(prefix="sdd-fixture-"))
    tasks = tmp / "tasks"
    (tasks / "epics").mkdir(parents=True)
    (tasks / "specs").mkdir()
    (tasks / "claims").mkdir()
    (tasks / "evidence").mkdir()
    (tasks / "handoffs").mkdir()
    (tasks / "tracking").mkdir()
    (tasks / "scripts").mkdir()
    for name in ["specs", "claims", "evidence", "handoffs"]:
        shutil.copy(REAL_TASKS / name / "_TEMPLATE.md", tasks / name / "_TEMPLATE.md")
    shutil.copy(REAL_TASKS / "scripts" / "check-tasks.py", tasks / "scripts" / "check-tasks.py")
    shutil.copy(REAL_TASKS / "SDD.md", tasks / "SDD.md")
    (tasks / "EPICS.md").write_text("# Fixture\n\n| E99 | Fixture epic |\n")
    state = {
        "s0": "pending", "d0": "—", "s1": "pending", "d1": "E99-T00",
        "progress": "- [ ] E99-T00\n- [ ] E99-T01\n",
        "packet": False, "claim": None, "handoff": False, "evidence": False,
    }
    mutate(state)
    (tasks / "epics" / "E99-foundation.md").write_text(
        EPIC.format(s0=state["s0"], d0=state["d0"], s1=state["s1"], d1=state["d1"]))
    (tasks / "tracking" / "PROGRESS.md").write_text(
        "# Progress\n\n" + state["progress"])
    if state["packet"]:
        (tasks / "specs" / "E99-T00.md").write_text(PACKET.format(tid="E99-T00"))
    if state["claim"]:
        (tasks / "claims" / "E99-T00.md").write_text(
            CLAIM.format(tid="E99-T00", status=state["claim"]))
    if state["handoff"]:
        (tasks / "handoffs" / "E99-T00.md").write_text("# Handoff: E99-T00\n")
    if state["evidence"]:
        (tasks / "evidence" / "E99-T00.md").write_text("# Evidence: E99-T00\n")
    return tmp


def run_validator(tmp):
    proc = subprocess.run(
        [sys.executable, str(tmp / "tasks" / "scripts" / "check-tasks.py")] + CHECK,
        capture_output=True, text=True)
    return proc.returncode, proc.stdout + proc.stderr


def case(name, mutate, expect_ok, expect_fragment=None):
    tmp = build_tree(mutate)
    try:
        code, out = run_validator(tmp)
        if expect_ok:
            assert code == 0, f"{name}: expected exit 0, got {code}\n{out}"
        else:
            assert code != 0, f"{name}: expected rejection, got exit 0"
            assert expect_fragment in out, (
                f"{name}: missing {expect_fragment!r}\n{out}")
        print(f"ok: {name}")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def complete(state):
    state.update(s0="completed", packet=True, claim="released",
                 handoff=True, evidence=True,
                 progress="- [x] E99-T00\n- [ ] E99-T01\n")


def tracked_changes():
    proc = subprocess.run(["git", "-C", str(REAL_ROOT), "diff", "--name-only"],
                          capture_output=True, text=True)
    assert proc.returncode == 0, proc.stderr
    # Plus staged changes: anything the fixture could only have caused via
    # tracked-file writes shows up here; own untracked new files are ignored
    # because the fixture writes exclusively under its temp directory.
    staged = subprocess.run(
        ["git", "-C", str(REAL_ROOT), "diff", "--cached", "--name-only"],
        capture_output=True, text=True)
    return proc.stdout + staged.stdout


def main():
    before = tracked_changes()
    case("valid full cycle accepted", complete, True)
    case("dependency cycle rejected",
         lambda s: s.update(d0="E99-T01", d1="E99-T00"),
         False, "dependency cycle")
    case("active task without packet rejected",
         lambda s: s.update(s0="in_progress"),
         False, "has no task packet")
    case("active task with wrong claim status rejected",
         lambda s: s.update(s0="in_progress", packet=True, handoff=True,
                            claim="released"),
         False, "must be active")
    case("active task without claim rejected",
         lambda s: s.update(s0="in_progress", packet=True, handoff=True),
         False, "has no claim")
    case("progress mismatch rejected",
         lambda s: complete(s) or s.update(
             progress="- [ ] E99-T00\n- [ ] E99-T01\n"),
         False, "completion mismatch")
    case("completion without evidence rejected",
         lambda s: complete(s) or s.update(evidence=False),
         False, "has no evidence")
    # The real tree must be untouched by every case above: no tracked file
    # may change (the fixture writes exclusively under its temp directory;
    # this script and packet are the agent's own untracked work, not fixture
    # output).
    assert tracked_changes() == before, "fixture modified the real tree"
    print("ok: real tree untouched")
    print("7/7 fixture cases passed")


if __name__ == "__main__":
    main()
