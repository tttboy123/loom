# S1-W1 Frozen WorkItem Contract

- ID: `S1-W1`
- Title: Go bootstrap and explicit Mode Router
- Risk: Standard
- Depends on: none
- Corresponds to: `TECH-PLAN.md §14 Slice 1.1-1.2, §13.1 package layout`
- Candidate lineage: detached `8e207b8` in `loom-pi-rebuild`

## Owned files

- `go.mod`
- `go.sum` only if a dependency requires it
- `cmd/loom/main.go`
- `internal/mode/router.go`
- `internal/mode/router_test.go`

## Acceptance boundary

1. Initialize a Go 1.22+ module without unrelated framework dependencies.
2. Keep routing as a pure domain package with no CLI, provider, SQLite, or UI
   imports.
3. Route `plain_input` to conversation mode.
4. Route only the structured explicit triggers `use_agent`, `select_agent`,
   `select_team`, and `assign` to agent mode.
5. Keep unknown, empty, or ambiguous triggers in conversation mode.
6. Routing must not construct or request TeamDraft, TeamInstance, or
   AgentInstance.
7. Provide a minimal buildable CLI entry point; do not implement Slice 2
   behavior.

## Deterministic checks

- RED: `go test ./internal/mode -run TestRoute -count=1`
- Focused GREEN: `go test ./internal/mode -run TestRoute -count=1`
- Impact: `go test ./...`
- Race: `go test -race ./...`
- Static analysis: `go vet ./...`
- Diff hygiene: `git diff --check`

## Governance

- One Developer writer owns only the files above.
- The Controller runs deterministic tests.
- A fresh read-only Reviewer must return PASS before S1-W2 opens.
- No commit, push, merge, release, activation, credential change, network
  dependency, or FastContext installation.
