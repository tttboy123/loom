# P2A-W1 Native App Launchability Replacement Live Gate Contract Review

**Date**: `2026-07-28`
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `PASS`
**Findings**: none

## Assessment

The contract correctly preserves the consumed no-retry result while granting
no live authority at Review time:

- replacement allowance remains `0`;
- a fresh post-Review audit and exact later user phrase are both mandatory;
- earlier or blanket authorization cannot substitute;
- the repaired Candidate hashes, UUID, signature resources, and canonical
  manifest are exact;
- original hashes, modes, xattr capture, provenance-aware bootstrap recovery,
  Journal, crash reports, and absent paths are exact;
- initial Computer Use launch is no-retry;
- all eight screens, empty-Team proof, app relaunches, and required Candidate
  daemon restart remain mandatory;
- any new crash report fails closed and is never deleted;
- success still requires fresh Result-Evidence Review before commit/P2A-W2;
- failure consumes the one replacement and permits no second replacement or
  point Amendment.

## Read-only current-state checks

The Reviewer reproduced:

- original observer running from `loomd-clean`;
- original hashes and modes exact;
- provenance xattr names present on original `loomd` and wrapper;
- two historical crash reports unchanged;
- SQLite integrity `ok`, Event count `1`;
- Candidate app/run/socket/launcher absent;
- actual target-process Provider marker count `0`;
- Git staging empty.

No edit, build, install, bootstrap, restart, app launch, Computer Use action,
stage, or commit occurred.

## Boundary

This `PASS` freezes governance only. It authorizes the post-Review activation
audit, not the replacement live gate.
