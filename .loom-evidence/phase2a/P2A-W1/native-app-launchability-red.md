# P2A-W1 Native App Launchability Mandatory RED

**Date**: `2026-07-28`
**Status**: `RED — EXPECTED FAILURE CONFIRMED`
**Product implementation changed before RED**: no
**Process spawned**: no

## Command

```text
scripts/test-build-loom-local-app.sh
```

Result: nonzero, with the closed failure:

```text
native app executable is missing LC_UUID
```

The repaired fixture built the current Candidate, validated its architecture
and signature, detected the missing UUID, rechecked crash-report inventory, and
exited before the private launch smoke.

## External-state guard

DiagnosticReports still contained exactly the two preserved canary reports:

```text
LoomLocalApp-2026-07-28-213614.ips
f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0

LoomLocalApp-2026-07-28-213628.ips
4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54
```

No third report appeared.

Additional RED predicates:

- installed Candidate app absent;
- product run directory/socket/launcher absent;
- original observer `running`;
- resident SQLite hash
  `91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4`;
- SQLite Event count `1`;
- Git staging empty.

This RED proves the regression guard would have rejected the consumed
no-UUID Candidate without launching it. Implementation may now change only the
frozen builder.
