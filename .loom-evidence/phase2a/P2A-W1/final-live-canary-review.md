# P2A-W1 Reopen 3 Final Live-Canary Result Review

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer
**Verdict**: `PASS`
**Blocking findings**: none

## Findings

1. The evidence correctly records the final canary as
   `FAIL — ROLLED_BACK — HUMAN_REQUIRED`, with allowance `0`, no TUI, no
   explicit daemon restart, and no claimed product capability.
2. The Candidate was installed and bootstrapped once before rollback.
   Therefore the final allowance is consumed even though the blocking
   predicate was later classified as a harness `test_defect`.
3. The Reviewer independently reproduced the safe process-scan behavior
   without emitting process rows or values:
   - `ps -eww -p <restored-pid>` selected 1,073 current host rows and matched;
   - target-only `ps` selected one row and did not match.

   The all-host count changed from the recorded 1,074 due to ordinary process
   churn. The selection behavior and match split are the same.
4. Exact rollback was independently verified:
   - original `loom`, `loomd`, wrapper, plist, and SQLite hashes and modes
     match the recorded pre-state;
   - SQLite is `integrity_check=ok`, contains one Event, and has canonical
     stream-head digest
     `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`;
   - launcher, product run directory, default socket, and historical socket
     are absent;
   - the restored LaunchAgent is loaded through the reviewed clean wrapper;
   - staged diff is empty.
5. The loaded service marker-name set is exactly the reviewed five names. The
   disk plist and wrapper remain secret-negative. No credential value was
   emitted or persisted by Review.
6. The terminal no-retry conclusion matches Reopen 3 authorization. It does
   not accept W1, authorize a fourth canary, or permit P2A-W2 to start.

## Verdict

`PASS` validates the fail-closed result evidence, harness-defect
classification, allowance accounting, and exact rollback only.

P2A-W1 remains `HUMAN_REQUIRED`, uncommitted, and not live-delivered. There is
no remaining canary allowance.
