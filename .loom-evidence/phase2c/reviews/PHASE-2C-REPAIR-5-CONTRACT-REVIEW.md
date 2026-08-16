# Phase 2C Repair 5 Contract and Implementation Review

**Date**: 2026-08-08  
**Reviewer**: Leibniz (`019fe10a-5a92-74a2-973c-54af0a87b6de`)  
**Mode**: fresh independent read-only review  
**Verdict**: `FAIL`  
**Counts**: `P0=0`, `P1=1`, `P2=0`

## Reviewed Byte Bindings

| Path | SHA-256 |
|---|---|
| `.loom-evidence/phase2c/contracts/PHASE-2C-REPAIR-AMENDMENT.md` | `8323b6aecdcba4d67a803cd464ee744a08de8ecb2dafa1a04b6afe0271d8eee7` |
| `.loom-evidence/phase2c/repair-candidate-boundary.md` | `957d07ca28980f66f8b1232fbb8ad332c9b9b642766bf6bfa9faec4ad5a83e74` |
| `docs/CURRENT.md` | `2a5e855b04c17944456f988e65b1f5bc09d452473915305e4ce52b730f5cfc49` |
| `journeys/repair-2026-08-08/attempt-003/FAILURE.md` | `0db15521264391716371234a37f7f820d3f8549f7f43a168c724287e591bbbc4` |
| `internal/localipc/client.go` | `cdbb9f120501e011555fcf0c2683916eeb53ec3f6cf0c586eeaed8ed4a479f2a` |
| `internal/localipc/client_test.go` | `33f5ca3268d5a488edd3dc31147610acade61a71d4d1c2b3f132476eb8d1ab15` |
| `internal/localipc/server.go` | `509e7911cd0120f6b48f3af03b8ddd27fc30d7a73f862e2f7b92e33eddd03d52` |
| `internal/localipc/server_test.go` | `1e0ce1c44f6fbbfefde87fd93fe9638413d836c4b78e89aac630034bd07fe9f5` |
| `cmd/loom/tui.go` | `ab0b82cfaafbce0d57aa879193fab52a90911775e74786618634bdff1419cd45` |
| `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift` | `81c75a60c328d8b0050db6fa1f260bcbc4cb33fd4d9b0987f3cffeec899ad17d` |
| `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift` | `38a61ce419c8102e2a568d7b4c66976a42d2d45bb30e7b931e431a7cdb2e5712` |

## Finding

`P1`: the Candidate boundary correctly lists 44 paths, but the existing
`repair-source-lock.json` is still the historical Repair 4 lock with 36 hashed
paths and therefore omits all seven Repair 5 IPC files. A fresh Repair 5 lock
must bind the 43 non-lock paths before source-lock authorization.

## Passed Surfaces

The implementation-only review passed. Go server, Go client, TUI composition,
and Swift agree that exactly `credential_verify`, `mission_execution`, and
`chat_message` receive ten seconds; checked ordinary methods remain five
seconds. Context cancellation and connection shutdown remain bounded. No Team,
Mission, Run, Journal, credential, filesystem, or execution-authority behavior
changed.

Attempt 003 remains immutable failed evidence with zero Team/Mission/Run facts,
integrity `ok`, and clean shutdown. Phase 2C remains `PARTIAL`.

## Independent Commands

- `go test ./internal/localipc ./cmd/loom`: `PASS`.
- `go test -race ./internal/localipc ./cmd/loom`: `PASS`.
- Focused Swift long-operation timeout test: `PASS`.
- Boundary inventory: 44 existing unique paths; expected non-lock paths: 43;
  historical lock paths: 36; missing Repair 5 paths: 7.

The required remedy is to generate the expanded Repair 5 lock and submit those
exact bytes for independent lock re-review. This FAIL does not authorize a
journey, deterministic acceptance matrix, staging, commit, or Phase acceptance.
