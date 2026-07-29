# P2A-W1 Amendment 2 Replacement Controlled Live Gate

**Date**: 2026-07-28
**Status**: `FAIL — ROLLED_BACK — HUMAN_REQUIRED`
**Replacement allowance remaining**: `0`
**Product capability claimed**: no
**TUI opened**: no

## Preconditions

- Implementation Review 3: `PASS`.
- Amendment 2 Review: `PASS`.
- Exact restored pre-state independently confirmed.
- Candidate hashes unchanged:
  - `loom`:
    `cfcf711da265a68ce924c9dc78e76103de66193378dc430fe4f79e6ce6a10a8a`
  - `loomd`:
    `1d76c641ad66142f8dd9706c05880a6953f6368815ef4068d3a25dfb5d25e4f2`
- Pre-transition observer PID: `33235`.
- SQLite: hash
  `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4`,
  `integrity_check=ok`, one Event, stream-head digest
  `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`.

## Replacement execution

The reviewed Candidate and clean plist were installed atomically. Clean
`bootout` was followed by Amendment 2's bounded polling. Both exact service
absence and old-PID absence became true within the ten-second bound. The one
authorized candidate bootstrap then succeeded.

Before the metadata gate:

- loaded service PID: `70755`;
- default parent:
  `/Users/lune/Library/Application Support/Loom/run`, mode `0700`, uid `501`;
- default socket: `loomd.sock`, Unix socket, mode `0600`, uid `501`;
- historical demo-resident socket: absent;
- installed Candidate hashes and modes: exact;
- plist: exact two-argument socket addition, mode `0600`;
- launcher: installed at mode `0700`;
- process command/environment secret-negative;
- SQLite remained `integrity_check=ok`, one Event, unchanged database hash and
  unchanged stream-head digest.

## Blocking metadata result

The loaded launchd service metadata still contained these Provider marker
keys:

```text
DEEPSEEK_BASE_URL
STEPFUN_BASE_URL
MINIMAX_BASE_URL
DEEPSEEK_API_KEY
STEPFUN_API_KEY
```

No value was printed, copied, stored in repository evidence, or passed to the
daemon. The disk plist contains none of the keys, the wrapper uses `env -i`,
and the candidate process environment scan was clean. The finding is confined
to launchd service-manager metadata.

Amendment 2 makes any loaded Provider key an immediate rollback and
`HUMAN_REQUIRED` condition. Therefore the TUI was not opened, no Computer Use
action ran, and the post-TUI restart was not attempted.

## Exact rollback

The recovery path waited for exact candidate-service/PID quiescence, then:

1. restored original `loom`, `loomd`, and plist bytes atomically;
2. retained the original wrapper unchanged;
3. removed the launcher, product socket, and product run directory;
4. bootstrapped the original observer service.

Post-recovery:

- original `loom` SHA-256
  `60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698`,
  mode `0755`;
- original `loomd` SHA-256
  `e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a`,
  mode `0755`;
- original wrapper SHA-256
  `ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4`,
  mode `0700`;
- original plist SHA-256
  `2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4`,
  mode `0600`;
- SQLite hash, integrity, one-Event count, and stream-head digest unchanged;
- authoritative and historical sockets, run directory, and launcher absent;
- original observer running as PID `76780`;
- staged diff empty.

## Terminal result

Both governed live allowances are consumed. The same exact Candidate may not
be bootstrapped a third time. Clearing the residual launchd metadata appears to
require global service-manager environment/credential mutation or another
broader lifecycle method explicitly excluded by the parent contract and
Amendment 2.

P2A-W1 stops `HUMAN_REQUIRED`. It is not accepted, installed, committed, or
live-delivered. P2A-W2 cannot begin under the frozen sequencing rule.

## Evidence methodology correction

Result-Evidence Review 1 found that the initial
`5c7e59572c7a79ba1a30bdf9f5f187cea276cbd1ca69a0a7383e150b23417193`
measurement hashed pipe-delimited query text and was incorrectly called the
canonical digest. The corrected value above exactly reproduces
`GlobalReadView`'s NUL-delimited stream-head/event-ID digest. The database hash,
integrity, one-Event count, underlying head, fail-closed result, and rollback
state are unchanged.
