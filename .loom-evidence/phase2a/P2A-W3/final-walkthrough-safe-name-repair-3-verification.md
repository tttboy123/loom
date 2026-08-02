# P2A-W3 Final Walkthrough Safe-Name Repair 3 Verification

Date: 2026-08-03

## Closed behavior

- `LocalProductMissionSummary.Title` no longer carries `team_definition_id`,
  `team_instance_id`, or `mission_id`.
- A version/scope/digest-matched TeamDefinition supplies its validated Team
  display name through the existing read-view resolver.
- A saved Team whose definition cannot be validated uses `Saved team`.
- An execution with no Team record uses `Historical mission`.
- The real SQLite projection → LocalProductReadService → Go IPC server → Go TUI
  fixture asserts the safe title and rejects `team.delivery` on the rendered
  primary Board.
- Swift/TUI defensive safe-name helpers remain in place for strict client-side
  fail-closed behavior.

Journal facts, schema, TeamDefinition/TeamInstance identities, Mission command
IDs, CAS, view-version and generation fencing are unchanged. Projection/read
models remain rebuildable caches rather than authority.

## Focused checks

PASS:

```text
go test -count=1 ./internal/api -run 'TestLocalProductMissionFacadeUsesRealProjectionAndPreservesStaleView|TestLocalProductReadServicePublishesBoundedSnapshotAndPreservesStaleView'
go test -count=1 ./internal/api ./internal/tui
go test -count=1 ./cmd/loomd -run TestProductDaemonServesRealReadOnlySQLiteOverPrivateUDSAndCleansUp
```

## Complete matrix

PASS after the real vertical fixture was updated to the safe product contract:

```text
go test -count=1 -p=1 ./...
go test -count=1 -race -p=1 ./...
go vet ./...
swift test --package-path apps/macos
swift build -c release --package-path apps/macos
git diff --check
```

Both Go matrices exited `0`. Swift executed 63 XCTest tests with one
visual-export-only skip and all 4 Swift Testing tests; every executed test
passed.

