# P2A-W2 Final Mission Decision View Lifecycle Closure — Deterministic GREEN

**Date**: 2026-08-01

**Contract**: `mission-decision-view-lifecycle-closure-reopen.md`

**Base commit**: `3eea638`

**Live execution**: not run; replacement lineage remains locked

## Behavior closed

The Candidate adds an explicit prepared-command view query with two closed
modes:

- `refresh_current` receives the exact successful GlobalReadView version,
  refreshes every unconsumed entry, and mutates no sheet until all entries
  prove that same version;
- `preserve_stale` performs no refresh and accepts only entries already bound
  to the prior cached view.

`LocalProductReadService` now stores the last successful immutable
view/prepared-command pair. Projection failure verifies the registry still
matches that pair and then returns the cached commands; any mismatch fails as
`local product state unavailable` rather than publishing mixed state.

Submission is unchanged: it exact-matches the current prepared sheet, fences
in-flight work, independently refreshes the current view immediately before
the existing Rules/Work authority, rejects an old command/generation/replay,
and performs no hidden retry.

## RED to GREEN

The preserved RED failures were:

```text
prepared command view = 9020e40f...6b39
authoritative view    = 55c0cc9b...2885

current stale-pair command view = 40c875c9...9376e
re-read stale command view      = bbbbbbbb...bbbb
```

The same tests now pass. Additional focused coverage proves:

- real Runtime discovery advances the view;
- the newly published command succeeds once while the pre-discovery command
  conflicts before authority;
- four-entry refresh is all-or-nothing on view mismatch and refresh error;
- in-flight listing conflicts;
- stale view/generation, replay and concurrent loser remain rejected;
- Projection failure preserves the prior pair;
- a mixed stale pair returns state unavailable; and
- real Journal fixture -> `LocalProductReadService` -> Go IPC handler publishes
  one exact snapshot/command view and rejects the old IPC command without an
  Event write.

Focused results:

```text
go test -count=1 ./internal/app ./internal/api ./cmd/loomd
PASS
```

## Complete Go and cross-language matrix

All SwiftPM output was routed beneath:

```text
/private/tmp/loom-p2a-w2-view-lifecycle-verify.WEwuFh
```

The first unconstrained package-parallel Go run was not accepted: two packages
started real Swift builds against one controlled `SWIFTPM_BUILD_DIR`, and one
reported `build.db: database is locked`. No product test failed. The matrix was
rerun with `GOFLAGS=-p=1`, retaining the exact Go test selection while
serializing package-owned Swift builds into separate attempt-local roots.

Accepted results:

```text
GOFLAGS=-p=1 go test -count=1 ./...         PASS
GOFLAGS=-p=1 go test -count=1 -race ./...   PASS
GOFLAGS=-p=1 go vet ./...                    PASS

go test -count=1 ./internal/localipc \
  -run TestStrictSwiftClientReadsPreparedDecisionFromRealGoServer
PASS

swift test --package-path apps/macos --scratch-path <debug>
51 tests, 0 failures, 1 intentional visual-export skip

swift test --package-path apps/macos --scratch-path <tsan> --sanitize=thread
51 tests, 0 failures, 1 intentional visual-export skip

swift build --package-path apps/macos --scratch-path <release> -c release
PASS
```

`apps/macos/.build` was absent before and after every accepted command.

## Scope and negative checks

The pre-Repair-1 five-file lock was
`mission-decision-view-lifecycle-source-lock.json` with combined digest:

```text
10ae688306cab607d3de8b266ce358bbb5d0f972fb69eb62882030e59b8d1dea
```

Implementation Review 1 correctly rejected that evidence because the
lifecycle test called the product handler directly. Repair 1 now starts one
real private Go IPC server and uses `localipc.Client.Call` for both snapshots,
the old-command conflict and current-command read. The source-lock digest
method text was corrected to files-array order. The final repaired lock is:

```text
c5c103e3ba540f694685673430f464ba0c4be523ae810a6acc38345fe9068724
```

Repair 1 then reran focused Go/race, complete serialized Go/race/vet, the real
Go-server-to-strict-Swift fixture and Swift debug/TSan/Release successfully.
The exact commands and reviewer-created cache quarantine are recorded in
`mission-decision-view-lifecycle-repair-1-result.md`.

The diff contains exactly two production and three test files from the frozen
boundary. `git diff --check`, secret-pattern scan and forbidden-authority diff
scan are empty. No Event/Journal/StateWriter/Projection/Rules/Work/Grant/
Evidence schema or authority file, Swift source/decoder, IPC method, daemon
production assembly, Provider or credential code changed.

No external daemon, native app, TUI, Provider request, credential mutation,
installed application or replacement canary was used. All SQLite, socket,
Runtime discovery and authority activity was test-owned and isolated.

Fresh independent Implementation Review is the only next gate.
