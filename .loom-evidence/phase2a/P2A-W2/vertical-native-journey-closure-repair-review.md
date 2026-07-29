# P2A-W2 Vertical Native Journey Closure Repair — Contract Review 1

**Date**: 2026-07-30
**Mode**: fresh independent read-only
**Verdict**: `FAIL`

## Findings

- P0: none.
- P1: the owned Swift test path named
  `apps/macos/Tests/LoomLocalAppCoreTests/`, but the package's real test target
  is `apps/macos/Tests/LoomLocalAppTests/`. The frozen boundary was therefore
  insufficient for its mandatory Swift RED/GREEN.
- P2: the Provider/commit/IPC deadline hierarchy was directionally correct but
  did not freeze exact budgets.
- P2: attempt-005's phrase `exact reviewed Candidate binaries` was ambiguous
  about post-repair versus attempt-004 artifacts.

The Reviewer otherwise confirmed:

- one same-W2 vertical repair, no W4 or micro-split;
- unchanged Journal/Projection/StateWriter authority;
- accurate static-catalog and equal-five-second deadline diagnosis;
- accurate treatment of the shutdown stage as unknown;
- correct Contract Review → RED → deterministic GREEN → fresh Implementation
  Review gate before any attempt-005 action.

The Reviewer made no filesystem change and ran no live, Provider, Keychain,
network, installed-Pi, daemon or native-app action.

## Contract Repair 1 Re-review

**Mode**: different fresh independent read-only Reviewer
**Verdict**: `PASS`

- P0: none.
- P1: none.
- P2: none.

The Re-reviewer confirmed:

- the real Swift package test target is now owned;
- the exact 5s Provider, at-most-1s terminal commit, exact 10s
  `credential_verify` Go/Swift and unchanged 5s default budgets are coherent;
- conditional IPC/Swift ownership is sufficient for the mandatory RED;
- attempt-005 binds post-Implementation-Review Candidate binaries;
- the exact attempt-002 five-Event source hash, private mode and SQLite
  integrity reproduce and correctly live-test catalog refresh after observer
  append;
- no W4, authority expansion, detached-work workaround or premature live
  allowance exists.

The Re-reviewer performed only read-only file and SQLite metadata inspection.
It edited, staged and committed nothing and ran no live, Keychain, network,
installed Pi, app or daemon action.
