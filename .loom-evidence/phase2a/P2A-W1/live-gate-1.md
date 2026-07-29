# P2A-W1 Controlled Resident Live Gate 1

**Date**: 2026-07-28
**Status**: `FAIL — ROLLED_BACK`
**Product capability claimed**: no
**TUI opened**: no

## Authorized pre-state

- Implementation Review 3: `PASS`, no findings.
- Existing service:
  `com.earendilworks.loom.runtime-observer`, PID `64354`,
  last-exit field `4`.
- Existing `loom`:
  SHA-256
  `60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698`,
  mode `0755`.
- Existing `loomd`:
  SHA-256
  `e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a`,
  mode `0755`.
- Existing wrapper:
  SHA-256
  `ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4`,
  mode `0700`.
- Existing plist:
  SHA-256
  `2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4`,
  mode `0600`, `plutil` valid.
- Existing SQLite:
  SHA-256
  `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4`,
  mode `0600`, `integrity_check=ok`, one Event.
- Canonical pre-run stream-head digest:
  `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`.
- Authoritative default socket/run directory, historical demo-resident socket,
  and installed `Loom.command`: absent.

The disk plist, wrapper, current process command/environment, and logs passed
secret-negative scans. The already-loaded launchd service metadata still
contained the legacy marker keys `MINIMAX_BASE_URL`, `DEEPSEEK_API_KEY`, and
`STEPFUN_API_KEY`; no values were printed. The current wrapper's `env -i`
boundary kept those keys out of the observer process. The live gate required a
clean reload to eliminate the cached service metadata.

## Reviewed build

- Candidate `loom`:
  SHA-256
  `cfcf711da265a68ce924c9dc78e76103de66193378dc430fe4f79e6ce6a10a8a`,
  mode `0700`.
- Candidate `loomd`:
  SHA-256
  `1d76c641ad66142f8dd9706c05880a6953f6368815ef4068d3a25dfb5d25e4f2`,
  mode `0700`.
- Candidate plist: original arguments plus exactly `--socket` and
  `/Users/lune/Library/Application Support/Loom/run/loomd.sock`; mode `0600`,
  `plutil` valid, secret-negative.
- Candidate launcher: no-argument exec of the installed `loom`; mode `0700`,
  secret-negative.

## Failure

The transaction atomically installed the reviewed files, booted out the old
service once from an explicit credential-clean environment, and immediately
attempted the single authorized candidate bootstrap. macOS returned:

```text
Bootstrap failed: 5: Input/output error
```

No candidate daemon, socket, TUI, Provider, Runtime, model, Team, WorkPackage,
Run, or authoritative write was started. The gate did not retry.

Read-only unified-log evidence shows launchd declared the old service inactive
and began removal at `10:52:37.616`. Background Task Management observed the
restored legacy item at `10:52:37.752`, about 136ms later. This supports, but
does not conclusively prove, an immediate bootout/bootstrap namespace
quiescence race.

## Exact recovery

The transaction trap:

1. removed any candidate service;
2. atomically restored the exact original binaries and plist;
3. removed the candidate launcher, socket, and newly-created run directory;
4. bootstrapped the original service from the same clean command environment.

Post-recovery:

- all four original file hashes and modes match the pre-state exactly;
- SQLite hash, integrity, and one-Event count match exactly;
- default and historical sockets, run directory, and launcher are absent;
- the original observer is running again as PID `33235`, last-exit field `0`;
- no staging or repository commit occurred.

The changed file inodes are an expected consequence of atomic byte-for-byte
restoration; content, mode, owner, configuration, state, and running service
behavior are restored.

## Gate result

Live Gate 1 is consumed and failed safely. P2A-W1 remains `PARTIAL`; it is not
accepted or committed. A replacement canary requires a reviewed amendment and
must not silently retry this invocation.

## Evidence methodology correction

Result-Evidence Review 1 later found that the initially recorded
`5c7e59572c7a79ba1a30bdf9f5f187cea276cbd1ca69a0a7383e150b23417193`
was a stable pipe-delimited query-row hash, not the canonical
`GlobalReadView` digest. The corrected value above uses the implementation's
NUL-delimited format. SQLite bytes, integrity, Event count, and the underlying
head never changed.
