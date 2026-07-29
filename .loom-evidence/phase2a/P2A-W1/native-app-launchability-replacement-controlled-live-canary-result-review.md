# P2A-W1 Native App Launchability Replacement Result-Evidence Review

**Date**: 2026-07-28  
**Role**: independent read-only Reviewer  
**Verdict**: `PASS`  
**Findings**: none

## Confirmations

1. The result records one post-activation `launchctl bootstrap` invocation and
   allowance consumption despite no ready socket or app launch. It discloses
   the incorrect `bootstrap_consumed=0` bookkeeping placement and correctly
   applies the frozen successful-bootstrap/post-bootstrap rule to the
   successful launchctl call plus Candidate `daemon unavailable` execution.
2. The cause is deterministic:
   `newProductDaemonRunner` constructs `localipc.NewServer` before `Run`;
   `NewServer` rejects invalid socket paths; `validateSocketPath` requires an
   existing owned, resolved, non-symlink mode-`0700` parent; and `run` maps
   builder failure to `daemon unavailable`.
3. Current read-only state reproves exact original hashes and modes for
   `loom`, `loomd`, wrapper, plist, and SQLite; the original observer is
   running through `loomd-clean`; provenance attribute names are present;
   target-process five-marker count is `0`; SQLite integrity is `ok` with one
   Event; Candidate app/run/socket/launcher/native process and Swift cache are
   absent; crash inventory is the exact two historical reports; staging is
   empty.
4. The contract permits no second bootstrap, replacement, or hidden retry.
5. Workspace evidence discloses the local tool-transcript credential exposure
   without copying values. Affected credentials require rotation.
6. The terminal result is:

   ```text
   P2A-W1: FAIL — ROLLED_BACK — HUMAN_REQUIRED
   replacement allowance: 0
   P2A-W2: LOCKED
   commit: forbidden
   ```

## Reviewer boundary

The review performed no edit, build, install, bootstrap/restart, app launch,
Computer Use action, staging, commit, retry authorization, or secret-value
emission.

`PASS` means the failed-live result and rollback evidence are coherent. It does
not mean the native product passed, does not authorize a retry, and does not
unlock the atomic W1 commit or P2A-W2.
