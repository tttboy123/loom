# P2A-W2 Mission Decision Deterministic Cache Repair 1

**Date**: 2026-07-30
**Status**: FROZEN — quarantine and rerun locked pending Review
**Parent**: `mission-decision-vertical-closure-reopen.md`
**Scope**: pre-live deterministic cache isolation only

## Failure preserved

The repaired client passed focused Swift GREEN, complete Swift debug, Thread
Sanitizer, Release, Go full, repository race and vet. The Go matrices are not
accepted as complete deterministic evidence because
`internal/localipc` builds the real Swift contract probe without an explicit
scratch path. It recreated:

```text
apps/macos/.build
mode  = 0755
owner = 501
size  = 88M observed
birth = 2026-07-30T22:58:13+0800
```

This is `FAIL — REPOSITORY_SWIFTPM_CACHE_RECREATED`. No daemon, native app,
TUI, state fixture, product socket or fresh live attempt was started. The
single live lineage remains unconsumed and locked.

## Exact repair transaction

After fresh Review PASS, the Controller may:

1. prove no SwiftPM, Go test or compiler process and no open handle references
   the generated cache;
2. create a private attempt quarantine parent;
3. record a symlink-safe `lstat` manifest, regular-file hashes and symlink link
   text without following links;
4. atomically rename exactly:

```text
source =
/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/
loom-pi-rebuild/apps/macos/.build

destination =
/Users/lune/Library/Application Support/Loom/
p2a-w2-mission-workbench-decision-client-reopen-001/
quarantine/apps-macos-build-after-deterministic-matrix
```

5. prove the source is absent, destination root device/inode is unchanged and
   manifests match.

No file may be deleted, recursively rewritten, followed through a symlink,
restored, reused for Candidate output or committed.

## Controlled rerun

After quarantine, the Controller must use:

```text
SWIFTPM_BUILD_DIR =
/Users/lune/Library/Application Support/Loom/
p2a-w2-mission-workbench-decision-client-reopen-001/
swiftpm-go-contract
```

First, one bounded Swift `--show-bin-path` probe must demonstrate that this
environment variable routes output under the controlled attempt root while
`apps/macos/.build` remains absent.

Then, sequentially and with the same environment variable:

- rerun the real Go Server to strict Swift prepared-decision fixture;
- rerun `go test -count=1 ./...`;
- rerun `go test -count=1 -race ./...`;
- rerun `go vet ./...`;
- prove `apps/macos/.build` remains absent after every command.

If the environment variable is ignored, a repository cache reappears, any
matrix fails or any process/output escapes the controlled root, stop
`HUMAN_REQUIRED`. No additional product file becomes owned.

## Exit

This Repair passes only when fresh read-only Result Review confirms:

- exact recoverable cache quarantine;
- no deletion or reuse;
- controlled SwiftPM routing;
- real Go-to-Swift fixture, full Go, race and vet PASS;
- repository `.build` absent;
- product diff remains limited to the two owned Swift files; and
- no live process or fresh live state exists.

Only then may the repaired Candidate proceed to Implementation Review. This
Repair does not unlock live by itself and creates no point Amendment or P2A-W4.
