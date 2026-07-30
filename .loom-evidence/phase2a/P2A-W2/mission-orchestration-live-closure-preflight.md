# P2A-W2 Mission Orchestration Live Closure Preflight

**Date**: 2026-07-30
**Status**: FAIL — replacement lineage not consumed
**Classification**: `excluded_build_cache_mutation`
**Attempt**: `p2a-w2-mission-workbench-live-20260730-002`
**Candidate**: `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`

## Verdict

The deterministic product matrix and canonical Codex executable transaction
passed, but the excluded-path immutability gate failed before daemon start.
One Swift bin-path discovery command unexpectedly wrote generated metadata to
the pre-existing untracked `apps/macos/.build/` cache even though the command
also supplied an attempt-local `--scratch-path`.

No daemon process was invoked and the controlled fixture artifact root does
not exist. Under section 7 of the reviewed reopen contract, the replacement
live lineage is not consumed. Live nevertheless remains locked because the
preflight gate is `FAIL`.

The Controller did not clean, delete, reset, timestamp-rewrite or otherwise
attempt to conceal the excluded cache mutation.

## Canonical Codex transaction

Configured entry:

```text
/Users/lune/Documents/Codex/devtools/npm/bin/codex
```

Canonical regular executable:

```text
/Users/lune/Documents/Codex/devtools/npm/lib/node_modules/@openai/codex/bin/codex.js
```

Frozen identity:

```text
device = 16777229
inode = 23304913
uid = 501
mode = 0755
size = 7236
sha256 = 134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477
```

The status-only observation exited `0`; stdout was empty and stderr contained
the single accepted line:

```text
Logged in using ChatGPT
```

No login, OAuth mutation or credential material was requested or recorded.

## Deterministic verification

PASS after resource-isolated sequencing:

```text
go test -count=1 ./...
go test -count=1 -race ./...
go vet ./...
swift test --package-path apps/macos --scratch-path <attempt>/caches/swift/debug
swift test --package-path apps/macos --scratch-path <attempt>/caches/swift/tsan --sanitize=thread
swift build --package-path apps/macos --scratch-path <attempt>/caches/swift/release -c release
```

Both Swift test runs executed 50 XCTest cases with one expected
visual-export-only skip and four Swift Testing cases, with zero failures.
Swift Release passed.

The first Go run was intentionally preserved as a verification-orchestration
failure: it ran concurrently with a completely fresh Swift build and five
process-start fixtures exceeded their five-second threshold. After Swift
completed, all five focused tests passed immediately, followed by the complete
Go, race and vet matrix. No product byte changed.

Focused PASS:

- real Go IPC Server to strict Swift Client prepared-decision fixture;
- production daemon default fail-closed decision registry;
- controlled production Mission fixture;
- exact Authorization/Review/Recovery authority bindings;
- stale view/generation rejection;
- concurrent submission one-winner behavior;
- replay-after-view-advance rejection.

The 31-file source lock reproduced with zero mismatches:

```text
de25e3e4687e558de3fe7f409b07ff513f72812c2ccf7f977c0391bcd9ecaff7
```

Product source under `cmd/`, `internal/` and `apps/` has no diff from Candidate
commit `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`.

## Frozen build identities

```text
loomd   dc24ebc942be07e514de8921aa2acccb78bcf2e71944ac64063779f350d148f4
loom    7bcef2c1e695aefa8bdd7ee642b74c574d8ac3435cdf2802b7536ef3fba126d6
native  19831d13b474103b70fd7230119ac678cf0a0875ecb75fd671ba1f442dadcb59
manifest 194e698a4c6954bc98e38c1380e58798be22eee07c01214c98e107d866e08970
```

The ad-hoc signed native bundle passed strict codesign verification, contained
no symlink and used bundle identifier `com.earendilworks.loom.local`.

## Failed excluded-path gate

The repository dirty-state digest remained byte-for-byte identical:

```text
before = 84545b1bcb2f814525c9111c9d2ac6129870e59eaf2bbd313097e4eff0715697
after  = 84545b1bcb2f814525c9111c9d2ac6129870e59eaf2bbd313097e4eff0715697
```

The excluded-path metadata digest changed:

```text
before = 9c26eecd071d175a6ae3101cb9e55c6305b60e7da34f96f3b8c19cee05618d1c
after  = 9ad85089c963bd1f2405e99bc6650e1b14a76113d9a79b66d979360660ba7924
```

Read-only timestamp inspection identifies the mutation at
`2026-07-30T21:56:12+0800` through `21:56:14+0800` under
`apps/macos/.build/`, including:

- `.lock`;
- `release` symlink;
- `plugin-tools.yaml`;
- `release.yaml`;
- `build.db`;
- release `output-file-map.json` files;
- release build descriptions.

The triggering controller command was the first native bundle materialization
step that asked SwiftPM for `--show-bin-path`. Later exact bundle
materialization used only a separate attempt-local release scratch tree, but
that cannot undo or excuse the earlier excluded write.

The complete path/metadata listing is preserved at:

```text
/Users/lune/Library/Application Support/Loom/p2a-w2-mission-workbench-live-20260730-002/logs/excluded-build-cache-mutation.txt
```

## No live consumption

Post-stop checks prove:

- controlled daemon process: never started;
- native app: never started;
- TUI: never started;
- product socket: absent;
- product socket lock: absent;
- controlled artifact root: absent;
- controlled SQLite: empty regular `0600`, SHA-256
  `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`;
- controlled DB handle: absent;
- isolation contents: empty;
- failed lineage tree digest remains
  `2e81d2a50ddff47cb6fc07920fafdd80e149610658cd0802bcd7937e36a8ebcc`;
- direct SQLite mutation and manual socket cleanup: none.

The replacement canary may not begin under this failed preflight.
