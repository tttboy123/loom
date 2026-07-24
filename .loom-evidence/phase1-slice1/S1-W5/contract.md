# S1-W5 Frozen WorkItem Contract

- WorkItem: `S1-W5`
- Risk: Standard
- Depends on: accepted `S1-W4`
- Corresponds to: `TECH-PLAN.md §14 Slice 1.5, §15.1-§15.4`

## Owned files

- `cmd/loom/main.go`
- `cmd/loom/query.go`
- `cmd/loom/*_test.go`

Do not modify mode, Journal, Evidence, projection, migrations, dependencies,
docs, or Slice 2 behavior.

## Objective

Provide a minimal local CLI with two explicit, structured commands:

1. `loom route --trigger <trigger> [--target <id>] [--text <text>]`
   calls the accepted pure Mode Router and returns the observable routed mode.
2. `loom status --state <journal.db>`
   opens existing local state read-only, rebuilds the accepted projection, and
   returns a deterministic read-only snapshot.

The CLI must not create Team, Agent, Run, daemon, background process, or network
behavior.

## Frozen existing values

Use the existing `internal/mode` values without adding or changing enums:

- modes: `conversation`, `agent`
- triggers: `plain_input`, `use_agent`, `select_agent`, `select_team`, `assign`

Unknown trigger strings are valid structured input to the Router and remain
conversation mode, matching S1-W1. An omitted `--trigger` is invalid CLI input.

## Frozen output

Successful commands write exactly one JSON object plus newline to stdout and
nothing to stderr.

- Route result:
  `{"command":"route","mode":"conversation|agent"}`
- Status result:
  `{"command":"status","modes":[...],"work_items":[...],"evidence":[...]}`

Status arrays are sorted by stable ID/stream key. Empty accepted projection
state is a successful result with empty arrays. JSON must not expose database
paths, artifact filesystem paths, credentials, or internal errors.

## State boundary

- Production status opening uses SQLite read-only mode and query-only
  enforcement.
- CLI code must not issue SQL queries or mutations; it passes the read-only
  database handle to `projection.New`, invokes `Rebuild`, and reads
  `Snapshot`.
- Missing database, unreadable database, absent Journal schema, corrupt or
  unreplayable Journal, open/query failure, and canceled context are explicit
  state-unavailable failures. They must not emit a fabricated empty snapshot.
- The command closes its database handle before returning.

## Frozen exit and error policy

- exit `0`: success
- exit `2`: invalid command/flag/value; stderr begins `invalid input:`
- exit `3`: state unavailable; stderr begins `state unavailable:`

Failures write no stdout. Do not add other exit codes or public status enums.
Usage text may follow an invalid-input error but must be deterministic.

## Mandatory RED tests

1. `TestRunRouteDistinguishesConversationAndExplicitAgentTriggers`
   covers `plain_input`, all four existing explicit triggers, and an unknown
   trigger; proves no state opener is called.
2. `TestRunStatusReadsOnlyProjectionInDeterministicOrder`
   appends committed Journal facts, runs the production read-only path, and
   verifies sorted JSON without direct CLI SQL behavior.
3. `TestRunRejectsInvalidInput`
   covers no command, unknown command, missing trigger, unexpected positional
   arguments/flags, and missing state path with exit `2`.
4. `TestRunStatusUnavailableFailsClearlyWithoutEmptySuccess`
   covers missing file, schema absence, replay failure, and canceled context;
   asserts exit `3`, no stdout, and the frozen stderr prefix.
5. `TestRunHasNoBackgroundOrNetworkSideEffects`
   uses injected dependencies/counters to prove route and status perform only
   their declared calls and return synchronously.

## Deterministic checks

- RED/focused:
  `go test ./cmd/loom -run 'TestRun' -count=1`
- Repeated focused race:
  `go test -race ./cmd/loom -run 'TestRun' -count=50`
- CLI full: `go test ./cmd/loom -count=1`
- CLI race: `go test -race ./cmd/loom -count=1`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static analysis: `go vet ./...`
- Formatting/diff: `gofmt`, trailing-whitespace check, `git diff --check`
- Binary smoke:
  `go run ./cmd/loom route --trigger plain_input`

## Gate and non-goals

A new read-only Reviewer must return `VERDICT: PASS` before Slice 1 may close.

No daemon/API server, Team Draft, Team instance, Agent execution, writable
state command, direct table query, TUI/Web UI, network call, background process,
credential behavior, FastContext work, or Slice 2 implementation is authorized.
