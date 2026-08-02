# P2A-W3 Authoritative Acceptance/Recovery Repair 4 Verification

**Date**: 2026-08-03  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Repair 4 contract SHA-256**: `4d54af39b1e08b6b499775107cda4d8c88e74fe929cd4c08d6b81ff3885798d4`  
**Repair 4 RED SHA-256**: `68f75db5a115babeaadbbf83a4e2850b773c32edcdd659f6a2af71b0b9faddc1`  
**Status**: `PASS — source eligible for independent Implementation Review`

## Implementation result

New recovery writes and recovery replay now use the same
`buildTeamRecoveryTransaction` constructor. Exact replay reconstructs the Team
state immediately before the recorded recovery Event, derives a genuine Rules
decision using the committed UTC decision time and replayed immutable semantics,
and compares the complete expected transaction.

The comparison binds every recovery and downstream Event field: ID, stream,
sequence, idempotency key, type, schema version, emitted time, correlation,
causation and byte-exact canonical payload. Missing scheduled attempts, missing
Team terminal facts, duplicate matching recovery Events and mismatched facts fail
closed. A valid exact replay remains clock-independent and performs no append.

## Focused proof

The two causal REDs and an additional mismatched-downstream proof are GREEN:

```text
go test -count=1 ./internal/work -run '^TestTeamRecoveryExactReplayRejects(MissingScheduledAttempt|MissingTeamTerminal|MismatchedDownstreamFact)$'
PASS
```

Existing retry, terminal, concurrency and idempotency coverage also passes in
normal and race modes:

```text
go test -count=1 ./internal/work -run '^(TestTeamRecoveryIsExplicitTimeBoundedAndStopsAtMaxAttempts|TestTeamTerminalRecoveryIsTerminalOnceAndExactRetryIsIdempotent|TestTeamRecoveryExactReplayRejects.*)$'
PASS

go test -count=1 -race ./internal/work -run '^(TestTeamRecoveryIsExplicitTimeBoundedAndStopsAtMaxAttempts|TestTeamTerminalRecoveryIsTerminalOnceAndExactRetryIsIdempotent|TestTeamRecoveryExactReplayRejects.*)$'
PASS
```

## Candidate verification matrix

```text
go test -count=1 ./internal/work ./internal/app ./internal/rules ./cmd/loomd
PASS

go test -count=1 -race ./internal/work ./internal/app ./internal/rules ./cmd/loomd
PASS

go test -count=1 -p=1 ./...
PASS

go test -count=1 -race -p=1 ./...
PASS

go vet ./...
PASS

swift test --package-path apps/macos
PASS — 60 XCTest, 1 explicitly visual-only skip, 4 Swift Testing cases

swift build -c release --package-path apps/macos
PASS

git diff --check -- <nine source-locked files and Repair 4 evidence>
PASS
```

No product daemon, native app, Runtime, Provider, live lineage, staging or commit
was used during Repair 4 implementation or verification. The only authorized
next action is a fresh independent Implementation Review of the new source lock.
