# P2A-W1 Native App Launchability Replacement Controlled Live Canary

**Date**: 2026-07-28  
**Result**: `FAIL — ROLLED_BACK — HUMAN_REQUIRED`  
**Replacement allowance remaining**: `0`  
**Candidate bootstrap calls**: `1`  
**Native app launch calls**: `0`  
**Candidate daemon restart calls**: `0`

## Bound Candidate

The transaction revalidated the exact reviewed Candidate before mutation:

- `loom`: `3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f`
- `loomd`: `ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357`
- native executable:
  `f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b`
- arm64 `LC_UUID`: `CE91F84E-4333-35DB-B493-88FADCBC6EC1`
- canonical app manifest:
  `e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd`

The exact activation phrase was present and the pre-mutation checks passed.

## Failure

The reviewed installers placed the exact Candidate app and product. The
transaction then booted out the original observer, installed the Candidate
plist with the exact default socket arguments, and invoked
`launchctl bootstrap` once from a credential-clean environment.

The bootstrap call succeeded, so the frozen replacement allowance was
consumed. The Candidate process then exited before producing its required
private socket. The only emitted daemon classification was
`daemon unavailable`; Computer Use was never initialized and no native app was
launched.

The temporary transaction's terminal diagnostic printed
`bootstrap_consumed=0` because its bookkeeping variable was incorrectly set
only after the socket-readiness wait. That diagnostic does not change the
contract's consumption point. The explicit `launchctl bootstrap` returned
success under `set -e`, and the subsequent Candidate `daemon unavailable`
records prove that launchd executed the Candidate. There was one explicit
bootstrap invocation; automatic process handling inside that loaded launchd
job is not represented as another bootstrap call.

Read-only source diagnosis located the deterministic cause:

1. `newProductDaemonRunner` constructs `localipc.Server` before `Run`;
2. `localipc.validateSocketPath` requires the socket parent to already exist as
   an owned, resolved, non-symlink directory with mode `0700`;
3. the live transaction expected the Candidate to create the product run
   directory, so that parent was absent at constructor time;
4. `localipc.NewServer` therefore rejected the socket path and the daemon
   returned `daemon unavailable`.

This is a live transaction implementation defect, not a frozen-runbook or
`LC_UUID` regression. The already-reviewed native canary runbook explicitly
required creating the exact product run directory as user-owned `0700`; the
temporary transaction failed to carry that step across. The reviewed native
executable was not launched, so this canary produced no new native crash
report.

The contract states that every post-bootstrap failure consumes the allowance
and terminates without a second replacement or hidden retry. No attempt was
made to create the directory and bootstrap the Candidate again.

## Exact rollback

The armed transaction immediately:

1. booted out the failed Candidate;
2. removed the exact Candidate app, launcher, product run directory, socket,
   lock, and installer rollback files;
3. restored the exact original product binaries and plist;
4. bootstrapped the original observer through the provenance-aware recovery
   path and restored captured provenance attributes byte-for-byte;
5. revalidated all terminal predicates.

Post-rollback proof:

| Predicate | Result |
|---|---|
| original `loom` SHA-256 / mode | `60c90ada...698`, `0755` |
| original `loomd` SHA-256 / mode | `e5ab283c...14a`, `0755` |
| original wrapper SHA-256 / mode | `ff556602...3e4`, `0700` |
| original plist SHA-256 / mode | `2a6dc3e1...1f4`, `0600` |
| original observer | `running` |
| target-process five-marker count | `0` |
| SQLite SHA-256 / mode | `91ae07e0...8a4`, `0600` |
| SQLite integrity / Event count | `ok`, `1` |
| Candidate app/run/socket/launcher | absent |
| native process | absent |
| crash inventory | exact two preserved reports |
| Git staging | empty |

## Non-disclosure incident

A subsequent read-only diagnostic command displayed launchd inherited
environment rows in the local Codex tool transcript. No value is copied into
this evidence file, source control, app/daemon logs, Journal, or screenshot.
Because the transcript can contain credential values, affected Provider
credentials must be treated as exposed and rotated outside this WorkItem.

## Terminal status

```text
P2A-W1:
FAIL — ROLLED_BACK — HUMAN_REQUIRED

replacement allowance:
0

P2A-W2:
LOCKED
```

The deterministic repair and its reviewed Candidate remain uncommitted. A
fresh independent Result-Evidence Review may audit this result but cannot
authorize another bootstrap.
