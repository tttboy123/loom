# P2A-W3 Final Native Timeline Pagination Repair 7 Contract

**Date**: 2026-08-03  
**Status**: FROZEN — Contract Review pending  
**Parent**: `contract.md`  
**Trigger**: `final-whole-candidate-review-1.md`  
**WorkItem**: the existing unique `P2A-W3 Controlled Execution Experience`

## Outcome

Close the native first-page-only defect without adding a WorkItem, authority,
writer, durable cursor or second read model. Mission Room must receive one
complete bounded read-only Timeline assembled from the accepted strict
`timeline_page` API, so Changes and Evidence can render the already-authoritative
Source Evidence, independent Verifier Evidence and canonical terminal present
in the frozen 103-Event live fixture.

## Exact behavior

1. The native Store starts at the empty cursor and requests pages with the
   existing limit of 64.
2. It may read at most 8 pages and aggregate at most 512 records. Reaching either
   bound while `has_more=true` fails closed; there is no hidden unbounded read.
3. Every page must have schema version 1, the requested Team identity, the same
   non-empty view version, the same Board and the same Attention projection.
   Every page must have `gap=nil`.
4. A page with `has_more=true` must return a non-empty next cursor different
   from the request cursor and every previously returned cursor. A final page
   must have `has_more=false`. Cursor replay, empty continuation, page overflow
   and daemon `cursor_conflict` / `stream_gap` are whole-load failures.
5. Every delivered record must have a non-empty unique `delivery_id`, schema
   version 1 and the exact requested Team identity. Duplicate delivery, identity
   drift or record overflow rejects the whole aggregation.
6. No partial aggregate becomes visible. Only the final complete page is
   published with all records in delivery order and the final server cursor.
7. Each load has a Store-local ephemeral generation. Cancellation, Team /
   Mission selection change, or a newer load prevents every older page or error
   from changing Timeline, Timeline state or connection state. This generation
   is process memory only and is not authority or recovery state.
8. The strict Swift decoder remains fail-closed. No unknown-field relaxation,
   null-collection relaxation or wire-method change is allowed.

## Mandatory RED

Before product implementation, Swift tests must fail on the current one-page
Store and prove:

- two valid pages merge in exact order and issue exact cursors with limit 64;
- the merged result exposes Source Evidence, Verifier Evidence and terminal
  records with `gap=nil` and `has_more=false`;
- gap, view/Team/Board/Attention drift, duplicate delivery, repeated/empty
  continuation, and page-bound overflow publish no partial Timeline;
- cancellation and selection switching fence late success and late error;
- a complete single page still performs exactly one request.

The existing real Go server -> strict Swift probe test must additionally cover
the multi-page wire sequence. The final replacement walkthrough uses a fresh
private copy of the reviewed 103-Event SQLite fixture and records native
screenshots plus a durable TUI transcript or screenshot.

## Exact owned files

- `apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- `apps/macos/Sources/LoomLocalAppContractProbe/main.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`
- `internal/localipc/swift_contract_test.go`
- `.loom-evidence/phase2a/P2A-W3/**`
- `docs/CURRENT.md`

No Go authority, Journal, Projection, Rules, Work, Grant, Evidence, Supervisor,
Runtime adapter, IPC protocol, Swift decoder strictness, daemon assembly or UI
presentation file is reopened. `AGENTS.md`, `README.md`, `PROGRESS.md`, Phase 1
evidence and every non-W3 dirty path remain untouched and excluded.

## Verification and exit

After RED/GREEN, run focused Swift and real-Go/Swift tests, all Swift tests,
Swift Release build, complete serialized Go normal/race matrices, `go vet`,
module verification, format/diff, scope and secret checks. Freeze an exact
Repair 7 source lock and obtain a fresh independent Implementation Review PASS.

Only after that PASS may one fresh isolated replacement walkthrough be run. It
is read-only over a copy of the accepted 103-Event fixture: no preflight, Start,
Provider Test, approval, recovery, cancel or terminal mutation. The walkthrough
must prove complete native Changes/Evidence/terminal presentation, TUI paging,
byte-identical SQLite, cleanup and non-disclosure. Then rebuild a narrow W3-only
final inventory and obtain a fresh whole-Candidate Review PASS before staging or
one atomic commit.
