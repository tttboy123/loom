# P2A-W1 Native App Final Exit Closure Contract Review

**Date**: 2026-07-28  
**Role**: fresh independent read-only Contract Reviewer  
**Verdict**: `PASS`  
**Findings**: none

## Assessment

The contract validly freezes one final vertical P2A-W1 exit closure without
granting live authority. It does not reinterpret the consumed replacement
canary: current allowance remains `0`, P2A-W1 remains failed and uncommitted,
and P2A-W2 remains locked. The future live path still requires RED, GREEN,
complete verification, fresh independent Implementation Review, a post-Review
audit, and the exact new activation phrase.

Restricting implementation to one governed evidence harness, its behavioral
fixture, and `docs/CURRENT.md` is consistent and safer than modifying product
source. The failed canary was an execution transaction omission, while the
already-frozen native runbook explicitly required run-root creation.

The contract completely and testably binds:

- run-root creation and validation before install, bootout, and bootstrap;
- consumption assignment immediately after successful bootstrap;
- no backedge, retry, or readiness-based accounting;
- the distinction between initial bootstrap and the later required lifecycle
  restart;
- exact production paths and sentinel-gated fixture isolation;
- non-emitting secret and evidence predicates;
- behavioral RED for missing parent, post-bootstrap readiness failure,
  pre-bootstrap failure, path escape, symlink, mode, and override attacks;
- GREEN, full verification, rollback, preserve, live, and terminal conditions.

The closure remains consistent with ADR-0011 and ADR-0012: it keeps one
authority, direct private IPC, the native app inside P2A-W1, no CLI/PTY
substitution, and no P2A-W4.

## Read-only baseline

The Reviewer independently reproduced:

- exact original `loom`, `loomd`, wrapper, plist, and SQLite hashes/modes;
- observer running from `loomd-clean`;
- provenance attribute names present;
- target-process five-marker count `0`;
- SQLite integrity `ok` with one Event;
- exact two-report historical crash inventory;
- Candidate app/run/socket/launcher and Swift cache absent;
- Git staging empty.

The review performed no edit, build, install, bootstrap/restart, app launch,
Computer Use action, staging, commit, or environment-value emission.

## Boundary

This `PASS` authorizes only mandatory RED. It grants no product-source change,
live installation, Candidate bootstrap, native-window canary, commit, or
P2A-W2 work.
