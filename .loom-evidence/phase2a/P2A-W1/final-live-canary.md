# P2A-W1 Reopen 3 Final Controlled Live Canary

**Date**: 2026-07-28
**Status**: `FAIL — ROLLED_BACK — HUMAN_REQUIRED`
**Final allowance remaining**: `0`
**Product capability claimed**: no
**TUI opened**: no
**Daemon restart performed**: no

## Preconditions

- Reopen 3 independent Review: `PASS`.
- Exact Reopen 3 activation phrase supplied by the user.
- Candidate hashes:
  - `loom`:
    `cfcf711da265a68ce924c9dc78e76103de66193378dc430fe4f79e6ce6a10a8a`
  - `loomd`:
    `1d76c641ad66142f8dd9706c05880a6953f6368815ef4068d3a25dfb5d25e4f2`
- Original installed hashes:
  - `loom`:
    `60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698`
  - `loomd`:
    `e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a`
  - clean wrapper:
    `ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4`
  - launchd plist:
    `2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4`
- SQLite hash:
  `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4`;
  `integrity_check=ok`; one Event; canonical stream-head digest
  `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`.
- Ambient manager values for the reviewed five keys were captured only in
  process memory for non-emitting equality checks. No value was printed,
  persisted, passed to Loom, or added to process arguments.

An initial guard invocation stopped before installation because a multiline
shell `test` was malformed. `installed=0`; hashes, service, launcher, socket,
run directory, and SQLite were rechecked as exact original pre-state. That
pre-install invocation did not consume the live allowance. The syntax was
repaired before the controlled transition.

## Controlled transition

The reviewed Candidate, launcher, and two-argument socket plist were installed
atomically. The original service was cleanly booted out, exact service/PID
absence was observed, the Candidate was bootstrapped once, and the private
socket became ready.

The first post-bootstrap aggregate validator failed and immediately invoked
the reviewed rollback path. The guard intentionally emitted no secret,
service-manager value, process environment, or command line.

## Evidence-led failure classification

Read-only post-recovery diagnosis reproduced a harness defect in the process
environment predicate:

```text
guard_ps_row_count=1074
target_only_ps_row_count=1
guard_scan_match=yes
target_only_scan_match=no
```

The guard used:

```text
ps -eww -p <candidate-pid> -o command=
```

On this macOS host, `-e` selected all processes; `-p` did not narrow the
result. The secret-negative scan therefore examined 1,074 unrelated process
rows and matched outside the Candidate. The target-only form examined exactly
one row and was secret-negative.

The disk plist, launcher, and reviewed `env -i` wrapper were independently
secret-negative. The exact service-visible marker-name set remained the
reviewed ambient set:

```text
DEEPSEEK_API_KEY
DEEPSEEK_BASE_URL
MINIMAX_BASE_URL
STEPFUN_API_KEY
STEPFUN_BASE_URL
```

No Provider value was emitted or persisted. This establishes a
`test_defect`/live-harness false positive, not a demonstrated product secret
inheritance. It does not turn the failed canary into a product PASS: the
installed TUI journey, relaunch, and explicit daemon restart never ran.

## Exact rollback

The automatic recovery path restored:

- original `loom`, `loomd`, wrapper, and plist bytes and modes;
- original observer service;
- absent launcher;
- absent product run directory and default socket;
- absent historical demo-resident socket;
- unchanged SQLite hash, integrity, one-Event count, and canonical head digest.

Post-recovery hashes exactly equal the pre-state values above. The restored
observer was loaded with PID `31342` when captured; launchd may replace this
short-lived KeepAlive process without changing the restored service
definition.

## Terminal result

The final authorized Candidate bootstrap occurred, so the Reopen 3 allowance
is consumed even though the blocking predicate was a harness false positive.
The contract permits no fourth canary or hidden retry.

P2A-W1 therefore remains unaccepted, uninstalled, uncommitted, and not
live-delivered. It stops `HUMAN_REQUIRED`; P2A-W2 cannot begin under the frozen
sequencing rule. Any further live attempt requires a newly reviewed and
explicitly authorized governance boundary.
