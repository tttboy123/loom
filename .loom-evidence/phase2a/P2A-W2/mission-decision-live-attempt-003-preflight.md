# P2A-W2 Mission Decision Live Attempt 003 Preflight

**Date**: 2026-07-30
**Attempt**: `p2a-w2-mission-decision-live-20260730-003`
**Candidate**: `700086db1d28837b09dd3f9f72b3bf847f0c400e`
**Verdict**: `PASS — daemon not started`

## Gate

Fresh independent Mission Decision Vertical Closure Implementation Review
returned `PASS` with no P0/P1/P2 before this attempt root was created.

The attempt root is a fresh owner-private `0700` directory:

```text
/Users/lune/Library/Application Support/Loom/
p2a-w2-mission-decision-live-20260730-003

device = 16777229
inode  = 76257911
owner  = 501
mode   = 0700
```

The empty state and controlled fixture manifest are regular owner-private
`0600` files. The manifest binds the exact Candidate commit, attempt identity,
fresh state path, fresh artifact root and fixed UTC authority time. The
artifact root itself is absent and isolation is empty before daemon start.

## Frozen identities

```text
manifest
2894e17b7b8487bea4f8d32b7d4410d6dcf6a2741bed8f95789384789ed7eaea

source-lock copy
e44f6670abf6c9446678c8945bccfbb2b727fd2c2bebe44e4b85ef0c7e80fe2d

source-lock combined digest
68ef6b9ab387fb5a4058a967f50795f4a988add34caf2854780e8fe6681abc66

loomd
dc24ebc942be07e514de8921aa2acccb78bcf2e71944ac64063779f350d148f4

loom TUI
7bcef2c1e695aefa8bdd7ee642b74c574d8ac3435cdf2802b7536ef3fba126d6

native LoomLocalApp
e36ed46317a21620ef35034cd88333a6bd19de4a6c013fa507501675d64c3a9c

canonical Codex executable
134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477

installed Pi 0.82.1 resolved executable
af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca

controlled Node executable
1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8
```

The native bundle is ad-hoc signed with identifier
`com.earendilworks.loom.local`, passes strict deep code-sign verification and
contains `LC_UUID 1265F0F8-F700-3C70-8A4B-1790157141C0`.

## Build and isolation truth

Go daemon/TUI output used an attempt-local build cache and the already present
local Go module cache with `GOPROXY=off`. An earlier attempt-local module
download was stopped before producing any binary because the exact 39MB module
was transferring too slowly; its partial cache is retained unused under this
attempt and is not a live retry or alternate executable.

Swift Release output used only the attempt-local scratch tree. Repository
`apps/macos/.build` remains absent. Product source under `cmd/`, `internal/`
and `apps/macos/` is clean against Candidate commit `700086d`.

Before daemon start:

- product socket and lock are absent;
- controlled artifact root is absent;
- isolation is empty;
- no controlled daemon, native app or TUI process exists;
- both earlier failed live lineages remain unchanged; and
- the unrelated resident Runtime observer remains untouched.

Exactly one daemon start and one native-bundle invocation are now permitted.
