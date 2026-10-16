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

## Review Coordination

Claims govern implementation ownership only. Reviewers may create or edit
Markdown files without an active claim, packet designation, or claim
authorization. Do not use this claim as a gate for review writes.

When `Status: released`, add `**Released At:** <RFC3339 timestamp>`. On each
reopen/release round, append timestamped round fields rather than replacing
history.

Use `active` while implementation ownership is in progress or blocked; release
when implementation ownership ends, even if a required review is still pending.
The task remains incomplete until required reviews and gates pass. Preserve prior
owners in `Previous Claim`.
