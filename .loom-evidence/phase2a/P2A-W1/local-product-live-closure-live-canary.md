# P2A-W1 Local Product Vertical Live Closure Canary

**Date**: 2026-07-29  
**Result**: `FAIL - ROLLED_BACK - HUMAN_REQUIRED`  
**Allowance before**: `1`  
**Initial Candidate bootstrap count**: `1`  
**Allowance remaining**: `0`  
**Retry**: no  
**Native App launch count**: `0`  
**Classified lifecycle restart count**: `0`

## Gate

Contract Re-review 2, deterministic GREEN, Implementation Re-review 2, exact
Candidate validation, activation audit, and empty staging were `PASS` before
the transaction. The activation audit bound one and only one bootstrap.

## Candidate result

The transaction armed rollback before mutation, installed the exact manifest
Candidate, replaced the plist with the reviewed private-socket form, booted out
the original observer, and performed one successful launchd bootstrap.

The Candidate daemon was spawned once and then exited with the closed
`daemon failed` surface before the transaction emitted
`LOCAL_PRODUCT_CLOSURE_READY`. No native App was opened. Therefore there was:

- no Home/Work/Teams/Inbox/System window proof;
- no Refresh;
- no App quit/relaunch recovery proof;
- no classified later daemon lifecycle restart;
- no Provider or Agent Runtime execution;
- no Team, Run, Grant, Evidence, approval, credential, or user-state action.

The transaction returned:

```text
LOCAL_PRODUCT_CLOSURE_RESULT result=rollback_incomplete initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0
```

No retry or alternate bootstrap was attempted.

## Rollback completion

At the transaction return, these rollback surfaces were already exact:

- installed `loom`, `loomd`, wrapper, plist, and SQLite hashes/modes;
- App, App backup, compatibility launcher/backups, product run root, socket,
  and binary backups absent;
- Journal integrity `ok`, Event count `1`, and original SQLite digest;
- two historical crash report bytes;
- Git staging empty.

The only incomplete surface was original observer stabilization. Launchd
initially cycled the exact restored program while reporting only the closed
daemon/signing surfaces. Read-only inspection and private-copy diagnostics
showed:

- exact original `loomd` succeeds for one and two cycles against a private
  SQLite copy and fresh isolation root;
- the installed program bytes and strict signature remained valid;
- macOS `com.apple.provenance` is persistent special metadata even when its
  delete command returns success;
- launchd eventually converged on a stable exact-original observer.

Rollback completion re-established the exact saved original bytes at the
governed paths, preserved the original modes and exact provenance values, and
let the exact original LaunchAgent converge without another Candidate
bootstrap. The same observer PID then remained running for more than 60
seconds across local Pi metadata probes. No inode-identity claim is made.

During final audit, macOS had automatically moved the two pre-existing crash
reports into `DiagnosticReports/Retired`. Their hashes were unchanged. The two
explicit files were moved back to their frozen parent paths and revalidated.
No report was deleted or generated.

## Final exact state

```text
loom
60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698
uid 501 mode 0755

loomd
e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a
uid 501 mode 0755

loomd-clean
ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4
uid 501 mode 0700

LaunchAgent plist
2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4
uid 501 mode 0600

resident SQLite
91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
uid 501 mode 0600
integrity ok
Events 1

historical crash reports
f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0
4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54
```

Both expected provenance values match their frozen pre-state. The observer is
running from the exact wrapper, its closed Provider-marker count is zero, the
native App/process and product socket are absent, and staging is empty.

## Diagnostic non-disclosure incident

One early read-only diagnostic used a broader-than-intended `launchctl print`
view and transiently displayed inherited environment credential values in the
tool output. No value was copied into repository evidence, Candidate
artifacts, screenshots, Journal, logs created by Loom, Agent definitions, or
the final report. All later diagnostics emitted only closed state/program/PID
fields and marker counts.

This over-capture is a security-process failure in addition to the failed
product canary. Credential rotation is outside this transaction and was not
performed silently.

## Verdict

The live product requirement did not pass and no native-window evidence
exists. Exact rollback did pass after bounded completion. The sole allowance
is consumed:

```text
FAIL - ROLLED_BACK - HUMAN_REQUIRED
```

P2A-W1 is not accepted or committed. P2A-W2 remains locked.
