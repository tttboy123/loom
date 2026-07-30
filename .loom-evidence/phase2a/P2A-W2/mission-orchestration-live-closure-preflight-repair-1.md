# P2A-W2 Mission Orchestration Live Closure Preflight Repair 1

**Date**: 2026-07-30
**Status**: FROZEN — fresh independent Contract Review PASS
**Authority**: explicit Product Owner selection `隔离后继续`
**Parent**:
`mission-orchestration-live-closure-reopen-contract.md`
**Repairs**: excluded generated Swift build-cache disposition only
**Candidate**: unchanged
`d0252064e43dc2c7d9e047aaa36942ef7b92d97b`

## 1. Scope and preserved failure

The reviewed preflight result remains:

```text
FAIL — excluded_build_cache_mutation
```

This repair does not rewrite that failure as PASS and does not claim the
pre-existing cache was untouched. It implements the Product Owner's explicit
choice to preserve the currently observed generated cache in a recoverable
quarantine and continue the same complete P2A-W2 live closure.

This is not a W4, product Amendment, product-source change, second Candidate or
replacement canary. The replacement daemon has never been invoked, the
controlled fixture artifact root does not exist and lineage `...-002` remains
unconsumed.

## 2. Exact cache transaction

Source:

```text
/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild/apps/macos/.build
```

Destination:

```text
/Users/lune/Library/Application Support/Loom/p2a-w2-mission-workbench-live-20260730-002/quarantine/apps-macos-build-after-preflight-failure
```

Before mutation, the Controller must require:

- source exists as a directory and is not itself a symlink;
- source is user-owned;
- destination and its parent do not yet exist;
- no process has an open handle under the source;
- no SwiftPM process is running for this repository;
- product socket/lock and controlled fixture artifact root are absent;
- controlled SQLite remains empty;
- a bounded file/content/metadata manifest is written under the attempt root.

The Controller then:

1. creates only the private user-owned `0700` quarantine parent;
2. performs one same-volume atomic rename of the exact source directory to the
   exact destination;
3. does not delete, rewrite, copy, merge, chmod recursively or follow cache
   symlinks;
4. verifies the destination device/inode and manifest after rename;
5. verifies the repository source path is absent;
6. verifies Git dirty-state outside the removed untracked cache is unchanged.

The quarantine is retained as recoverable user data. It is not automatically
restored, deleted, uploaded, committed or used for later builds.

## 3. Remaining preflight

No Swift build, test, `--show-bin-path` or package-manager command may run after
the quarantine transaction. The exact already-built daemon, TUI, signed native
bundle, manifest, source lock, canonical Codex identity and passing
deterministic logs remain frozen.

Before daemon start a fresh independent Preflight Repair Review must confirm:

- the original preflight failure remains explicit;
- the exact cache is preserved at the quarantine destination;
- no open handle or SwiftPM process remains;
- product source still equals Candidate `d0252064`;
- source lock and binary hashes remain the recorded values;
- canonical Codex identity remains unchanged;
- product socket/lock and controlled artifact root are absent;
- SQLite is empty and isolation is empty;
- the attempt-specific daemon/app/TUI have never run.

## 4. Live and no-retry boundary

Only after the fresh Review returns `PASS` may the same unconsumed replacement
lineage invoke its first and only daemon process. All original 15 vertical
native/TUI/Provider/Decision/cleanup requirements remain unchanged.

There is still no daemon retry, second replacement, alternate executable,
direct SQLite mutation, manual product socket cleanup or hidden live path.

If the cache cannot be preserved by the exact atomic rename, identity changes,
an open handle exists, or any post-rename check fails, work stops
`HUMAN_REQUIRED`.
