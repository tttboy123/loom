# P2A-W1 Native App Launchability Replacement Post-Review Activation Audit

**Date**: `2026-07-28`
**Status**: `PASS — PRE-LIVE ONLY`
**Candidate bootstrap count**: `0`
**Replacement allowance**: `0`
**Candidate installed/launched**: no

## Reviewed lineage

The audit follows:

1. consumed native canary and exact rollback;
2. Result-Evidence Re-review 2 `PASS`;
3. immutable dyld crash diagnosis;
4. Launchability Closure Contract Review 1 `FAIL`;
5. Contract Review Repair 1;
6. Contract Re-review 2 `PASS`;
7. mandatory pre-spawn RED;
8. deterministic GREEN and full matrix;
9. Implementation Review `PASS`, findings none;
10. Replacement Live Gate Contract Review `PASS`, findings none.

No earlier activation or blanket authorization is reused.

## Candidate rebuild

A private `0700` audit root and private Go build cache were created. The first
Go command used the older Reopen 4 `-trimpath -buildvcs=false` methodology and
correctly produced the distinct historical artifacts:

```text
loom
7ba4b41d143ea64fef7cafbfc463f4524ecd3790f93291a38a9c1812278d6ffd

loomd
f9529cf62a1bccbebe4d812dc7408624d2c1ee4c138b92fe69f1dfa497280bbb
```

The exact-hash guard stopped before Swift build, installer dry-run, or any
live mutation. This was a non-consuming audit-method mismatch, not a Candidate
bootstrap or product failure.

The current reviewed product installer fixture uses default `go build`, whose
VCS build metadata is part of the locked Candidate. Rebuilding with that exact
current method from the same private cache reproduced:

```text
loom
3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f

loomd
ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357
```

After `swift package reset`, the reviewed builder reproduced:

```text
LoomLocalApp
f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b

arm64 LC_UUID
CE91F84E-4333-35DB-B493-88FADCBC6EC1

Info.plist
554a6c8b990a75b5e86318a2a4e4ed7002fd2b8c4063c6a6071e5900d5132ba5

CodeResources
6686de10a28a2fe11b36cbb86dcbacc827cfc4ea116b4dabf1845e5aee629e9b

canonical complete bundle manifest
e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd
```

Strict signature verification passed. Exact live-destination app and daemon
installer dry-runs passed without mutation.

## Focused post-Review verification

Passed after the Replacement Contract Review:

```text
scripts/test-build-loom-local-app.sh
scripts/test-install-loom-local-app.sh
scripts/test-install-loom-local-product.sh
```

The build fixture again completed two package-reset byte-identical builds,
private exact-child survival/quiescence, and unchanged crash-report inventory.
Swift package cache was reset after all focused fixtures.

## Original-state guard

Fresh checks matched:

- original `loom` hash and mode `0755`;
- original `loomd` hash and mode `0755`;
- original wrapper hash and mode `0700`;
- original plist hash and mode `0600`;
- original provenance xattr names present on `loomd` and wrapper;
- original observer `running`;
- actual target-process Provider marker count `0`;
- resident SQLite frozen hash, integrity `ok`, Event count `1`;
- two historical app crash reports at their frozen hashes;
- Candidate app/run/socket/launcher/native process absent;
- Swift package cache absent;
- Git staging empty.

No xattr bytes, environment values, credential, prompt, output, raw process
row, or hidden reasoning was emitted.

## Activation decision

Every reviewed deterministic and pre-live gate now passes. No app/daemon was
installed, no service was booted out or bootstrapped, no Computer Use action
occurred, and no live allowance exists yet.

The sole remaining activation is a new user message containing exactly:

```text
P2A-W1 LC_UUID Launchability Repair and one replacement native-window canary
```

Only that post-Review message changes the replacement allowance from `0` to
`1`. Any earlier or blanket authorization remains ineligible.
