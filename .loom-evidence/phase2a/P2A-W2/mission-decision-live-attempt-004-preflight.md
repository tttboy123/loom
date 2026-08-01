# P2A-W2 Mission Decision Live Attempt 004 Preflight

**Date**: 2026-08-01
**Attempt**: `p2a-w2-mission-decision-live-20260801-004`
**Candidate**: `c94f30b38eb631f3cae6ac36b630e49edcae2c46`
**Verdict**: `PASS — daemon not started`

## Gate

The complete Final Mission Decision View Lifecycle Closure Reopen passed a
fresh independent Implementation Re-review with no P0/P1/P2. The Candidate was
then committed atomically. The single replacement live allowance is unused.

An orphaned native process from failed Attempt 003 was inspected before this
root was created. It held no socket, database or state handle and was exited
normally through its real macOS GUI. The unrelated resident Runtime observer
remains running and untouched.

## Fresh isolation

The fresh owner-private attempt root is:

```text
/Users/lune/Library/Application Support/Loom/
p2a-w2-mission-decision-live-20260801-004

device = 16777229
inode  = 77162880
owner  = 501
mode   = 0700
```

Its empty SQLite and controlled manifest files are regular owner-private
`0600` files. The manifest binds the exact Candidate commit, attempt identity,
fresh state path, absent artifact root and fixed UTC authority time
`2026-08-01T11:08:32.000000000Z`. Isolation is empty.

## Frozen identities

```text
manifest
c1025bd00903f4ca5be7de0e541102d00f9b70ea78cf9ea39b679497729bd12e

source-lock copy
04180e4c5c26fb1c7dcc8f8424269b9bbdaaa7a38eaf736be712813c1b5e2b62

source-lock combined digest
c5c103e3ba540f694685673430f464ba0c4be523ae810a6acc38345fe9068724

loomd
c162f423f36e473bdd1083eb9fb86107b2913fb18345edaef367e2980a428800

loom TUI
11fb7cca024f1b65099db18178489d377115818b21b086b48ed67dfb8b4a45c7

native LoomLocalApp
3e036efe2a43182c1e9058ebbfe4dfa00f4c3a822aef322227ae18226ce08534

canonical Codex native executable
29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a

installed Pi 0.82.1 resolved executable
af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca

controlled Node executable
1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8
```

The native bundle is ad-hoc signed with identifier
`com.earendilworks.loom.local`, passes strict deep code-sign verification and
contains `LC_UUID 7E6578B9-1241-39F3-A566-B5B3114AB78A`.

## Build and pre-start truth

Go daemon/TUI outputs used an attempt-local build cache and `GOPROXY=off`.
Swift Release output used only the attempt-local scratch tree. Repository
`apps/macos/.build` is absent. Product source under `cmd/`, `internal/` and
`apps/macos/` is clean against the Candidate commit.

Before daemon start:

- product socket and lock are absent;
- controlled artifact root is absent;
- isolation is empty;
- no controlled daemon, native app or TUI process exists;
- failed Attempt 003 state and evidence remain unchanged; and
- the unrelated resident observer is neither signalled nor reconfigured.

Exactly one daemon start and one native-bundle invocation are now permitted.
