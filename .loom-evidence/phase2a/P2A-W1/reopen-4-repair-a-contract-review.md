# P2A-W1 Reopen 4 Repair A Contract Review

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer
**Verdict**: `PASS`
**Blocking findings**: none

## Findings

1. Current `fileIdentity` stores only device and inode; `identityOf` returns
   only those fields and `removeExactFile` compares only that pair before
   deletion.
2. The existing real replacement test reproduces an inode-reuse cleanup
   failure under `-count=100`.
3. Binding `Lstat` file kind into the identity is the minimal product fix. It
   does not change IPC protocol, path policy, authority, peer checks,
   connection lifecycle, or error taxonomy.
4. Repair A owns only the exact socket identity implementation/tests and the
   existing server replacement test. Its deterministic fake-`FileInfo` RED and
   real repeated gates are sufficient.
5. No retry loop, timeout increase, error-assertion weakening, replacement
   deletion, or recursive cleanup is authorized.

## Gate result

Repair A supplemental Contract Review is `PASS`. Its RED and implementation
may proceed. This Review does not authorize a live/TUI canary, service change,
credential access, activation, staging, or commit.
