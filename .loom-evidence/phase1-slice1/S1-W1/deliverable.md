# S1-W1 Deliverable

## 1. What changed

- Initialized the `loom-pi-rebuild` Go 1.22 module without third-party
  dependencies.
- Added a pure `internal/mode` domain router.
- Added a minimal buildable `cmd/loom` entry point.
- Added tests for plain input, all four explicit Agent triggers, and
  unknown/empty/ambiguous fallback.

## 2. Result and exact evidence

- Initial bootstrap RED:
  `go test ./internal/mode -run TestRoute -count=1` exited non-zero because no
  `go.mod` existed.
- Behavioral mutation RED: temporarily removing `TriggerAssign` from the
  explicit route set made
  `TestRouteExplicitAgentTriggersUseAgentMode/assign` fail with
  `Route() mode = "conversation", want "agent"`.
- After restoring the Candidate, the following command union exited 0:
  `gofmt -w ...`; focused `go test`; `go test ./...`;
  `go test -race ./...`; `go vet ./...`; `git diff --check`.
- Fresh Reviewer reran the focused, full, race, vet, and diff checks, reported
  no findings, and returned PASS.

## 3. Affected files and behavior

- `go.mod`
- `cmd/loom/main.go`
- `internal/mode/router.go`
- `internal/mode/router_test.go`
- Plain and ambiguous input remains conversation mode. Only `use_agent`,
  `select_agent`, `select_team`, and `assign` enter Agent mode.
- No TeamDraft, TeamInstance, AgentInstance, Provider, SQLite, UI, or Slice 2
  behavior was introduced.

## 4. Remaining risk and unverified boundary

- Candidate product files remain untracked by design because commit/stage was
  not authorized. Plain `git diff --check` therefore does not inspect them.
  `gofmt` was run on every Go Candidate file, and both Controller and Reviewer
  inspected their contents.
- No daemon, persistence, real Agent runtime, or live Agent execution exists at
  this checkpoint.

## 5. Next executable step

Freeze S1-W2 and implement the append-only, idempotent SQLite Event Journal
under strict trust-boundary and failure-path tests.

VERDICT: PASS
