# P2A-W1 Native App Host Controlled Live Canary Result Review 1

**Date**: `2026-07-28`
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `FAIL`

## Finding

The frozen runbook records the original installed `loom` and `loomd` modes as
`0755` and requires rollback to reproduce original bytes and modes.

Independent read-only verification reproduced both exact original hashes, but
found both files at mode `0700`. The live result recorded the correct hashes
without recording or reconciling this mode drift. Exact rollback evidence was
therefore incomplete and incoherent.

## Independently reproduced passing checks

- original observer loaded and `running`;
- target-process Provider-marker count `0`, without emitting values;
- SQLite hash and mode exact;
- SQLite `integrity_check=ok`;
- SQLite Event count `1`;
- Candidate native app, product run directory/socket, compatibility launcher,
  and native process absent;
- Git staging empty.

## Boundary

This Review performed no edit, stage, commit, install, bootstrap, restart,
Computer Use action, or retry. It does not authorize a Candidate repair or
another canary.

The terminal product status remains
`FAIL — ROLLED_BACK — HUMAN_REQUIRED`; P2A-W2 remains locked.
