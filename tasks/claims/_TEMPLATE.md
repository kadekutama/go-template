# Task Claim: TASK-ID

**Task:** TASK-ID  
**Status:** active  
**Owner:** agent-or-human-id  
**Harness:** tool and version  
**Branch/Worktree:** branch-or-path  
**Base Commit:** full-commit-sha  
**Started At:** RFC3339 timestamp  
**Lease Until:** RFC3339 timestamp  
**Previous Claim:** none

## Coordination Notes

- Reserved migration/event/schema identifiers.
- Shared files and owners contacted.
- Takeover reason when applicable.

Use `active` while work is in progress or blocked and `released` only after the
task evidence/handoff is complete. Preserve prior owners in `Previous Claim`.
