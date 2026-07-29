# P2A-W1 Native App Final Exit Closure Post-Review Audit

**Date**: 2026-07-29  
**Status**: `FAIL — ROLLED_BACK — HUMAN_REQUIRED`
**Allowance**: `0`
**Candidate bootstrap count**: `1`  
**Native app launch count**: `1`

## Reviewed gates

- Final Exit Closure Contract Review: `PASS`;
- mandatory RED: preserved;
- Repair 2 deterministic GREEN: `PASS`;
- fresh complete deterministic matrix after Repair 2: `PASS`;
- fresh independent Implementation Review after Repair 2: `PASS`;
- Git staging: empty.

## Exact Candidate

```text
loom
3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f

loomd
ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357

native executable
f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b

arm64 LC_UUID
CE91F84E-4333-35DB-B493-88FADCBC6EC1

canonical bundle manifest
e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd
```

The bundle signature is strict-valid. Candidate binaries, root, and app remain
private mode `0700`, owned by uid `501`.

## Exact original state

```text
loom
60c90ada487dc74b4670320664ed8ce6434c81be815667d651fe7212b2d47698
mode 0755

loomd
e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a
mode 0755

loomd-clean
ff55660248804ae9ad231ab950875fc75c266e4b7687d89192411a3d7cb4f3e4
mode 0700

LaunchAgent plist
2a6dc3e1c87778e20cb7b8141be716d1b097349ac481fdd3a28b8189e35bf1f4
mode 0600

resident SQLite
91ae07e0a46868c80fd9bb7233fde9d3b9dffac1b92c49a0b25dd264d949c8a4
mode 0600
integrity ok
Events 1

canonical GlobalReadView head digest
6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
```

The exact original observer remains running from `loomd-clean`. Both preserved
provenance attribute names are present. The closed five-marker target-process
count is `0`; no process row or environment value was emitted.

The historical crash inventory remains exactly two reports:

```text
f284754aeca95cbceaf10070b2dd7465e4646ce3ab39a4e484c6ee04aff5e6c0
4804009d0fda5b9f29706be8a0373156706af9c845af5dd995dedc8054810f54
```

Candidate app, product run directory, socket, compatibility launcher and
backup, native process, Swift cache, and staged Git changes are absent.

## Activation

The user supplied the exact new post-Review activation message on 2026-07-29:

P2A-W1 Final Native Launch Transaction Closure and one controlled native-window canary

This atomically changes the single reviewed allowance from `0` to `1`.
The allowance permits only the exact governed transaction harness, one initial
Candidate bootstrap, the frozen native-window verification, and the one
classified lifecycle restart. It does not permit retry, alternate launch,
product-source change, Provider/Runtime execution, commit, or P2A-W2 work.

## Consumed result

The one initial Candidate bootstrap succeeded, so the allowance was consumed.
The first native Home view and its one bounded Refresh both returned
`Offline: invalid_response` with zero Runtimes. The frozen Home requirement was
not met. The harness received `ui_failed` and returned:

```text
FINAL_EXIT_RESULT result=rolled_back initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0
```

No other screen, native relaunch, Candidate lifecycle restart, or second
bootstrap was attempted. Exact rollback passed. This audit is terminal for
the Final Exit Closure allowance.
