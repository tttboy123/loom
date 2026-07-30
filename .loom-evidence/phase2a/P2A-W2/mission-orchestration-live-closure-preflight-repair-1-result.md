# P2A-W2 Mission Orchestration Live Closure Preflight Repair 1 Result

**Date**: 2026-07-30
**Status**: PASS — awaiting fresh independent Repair Review
**Candidate**: `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`
**Replacement lineage**: unconsumed

## Outcome

The Product Owner-authorized recoverable cache quarantine completed by one
same-volume atomic rename. No cache byte was deleted, recursively rewritten,
copied, merged, restored or reused.

Source is now absent:

```text
/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild/apps/macos/.build
```

The complete observed cache is retained at:

```text
/Users/lune/Library/Application Support/Loom/p2a-w2-mission-workbench-live-20260730-002/quarantine/apps-macos-build-after-preflight-failure
```

The quarantine contains approximately `493M`, `7,308` filesystem entries,
`5,145` regular files and the two original relative symlinks:

```text
./debug   -> arm64-apple-macosx/debug
./release -> arm64-apple-macosx/release
```

## Precondition checks

Before rename:

- source was an existing user-owned directory and not a symlink;
- source and attempt root were on device `16777229`;
- quarantine parent and destination were absent;
- no SwiftPM process targeted this repository;
- `lsof +D` found no open handle under the cache;
- product socket and product lock were absent;
- controlled fixture artifact root was absent;
- controlled SQLite was empty;
- isolation was empty.

## Symlink-safe preservation

The Controller enumerated with `find -P`, recorded `lstat`-equivalent metadata
for every entry, hashed regular files only and recorded link text without
following symlinks.

Pre/post manifest digests match:

```text
lstat
  pre  a9d0e66d08a7f19e191f879c6645b1e282d5aeeeaca2041299beb20c9256fde5
  post a9d0e66d08a7f19e191f879c6645b1e282d5aeeeaca2041299beb20c9256fde5

regular-file hashes
  pre  e08f1e6a2046686ffb323151768d64c1fca3f20b4073acc30a76c8db62d09178
  post e08f1e6a2046686ffb323151768d64c1fca3f20b4073acc30a76c8db62d09178

symlink text
  pre  ecd239fd8de50b3a980d832565df7f828d8922dc7018fedb996bee4e404c06ae
  post ecd239fd8de50b3a980d832565df7f828d8922dc7018fedb996bee4e404c06ae
```

The moved root retained identical device, inode, owner, mode, size and mtime:

```text
16777229 | 73552551 | 501 | 0755 | 448 | 1785419774
```

Repository status excluding the intentionally moved untracked build cache
remained identical:

```text
04b8dc9f58938213bc9bfd85c92b1ca4019184f117785d96aafe2c39fad2d412
```

## Frozen live inputs

Post-rename hashes still match:

```text
loomd   dc24ebc942be07e514de8921aa2acccb78bcf2e71944ac64063779f350d148f4
loom    7bcef2c1e695aefa8bdd7ee642b74c574d8ac3435cdf2802b7536ef3fba126d6
native  19831d13b474103b70fd7230119ac678cf0a0875ecb75fd671ba1f442dadcb59
manifest 194e698a4c6954bc98e38c1380e58798be22eee07c01214c98e107d866e08970
codex   134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477
```

Canonical Codex remains the same regular user-owned executable. Product source
still has no diff from Candidate `d0252064`.

## No live consumption

After rename:

- no attempt-specific daemon, native app or TUI process exists;
- product socket and product lock are absent;
- controlled fixture artifact root is absent;
- controlled SQLite remains empty;
- isolation remains empty;
- no SwiftPM command ran after quarantine;
- no cache cleanup or manual product socket action occurred.

Live remains locked until fresh independent Repair Review returns `PASS`.
