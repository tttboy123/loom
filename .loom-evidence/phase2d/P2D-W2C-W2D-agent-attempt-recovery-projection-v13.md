# P2D-W2C/W2D Agent Attempt Recovery Projection V13

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / READ-ONLY GOVERNANCE / EXPLICIT RESUME AUTHORITY OPEN  
**Contracts**: `contracts/P2D-W2C-agent-attempt-binding-dispatch.md`, `contracts/P2D-W2D-observability-governance.md`

## Accepted boundary

The Team board now admits the closed `agent_attempt_reconcile` diagnostic stage
and joins a daemon-restart recovery outcome only to the Agent row with the exact
Incident, Provider Account, and Model identity. A peer Agent does not inherit
that failure.

The Swift diagnostic vocabulary recognizes the same stage. Mission governance
renders the four controlled V10 outcomes as `Resume approval required`,
`Provider outcome uncertain`, `Encrypted input unavailable`, or
`Recovery state conflict`. The existing View diagnostics and Copy incident ID
actions remain available.

This is a read-only governance projection. V10 recovery records remain
non-retryable. The UI does not add Retry or Resume, recovered state is not added
to the active-Attempt registry, and no Runtime or Provider call is made. Codex
and Claude continuation remain same-process only and provide no restart-safe
native session reattachment.

## RED and verification

The mandatory REDs first proved three missing boundaries:

- the Go Team board dropped `agent_attempt_reconcile`;
- the Swift IPC error stage rejected that exact value;
- Mission UI had no controlled recovery-status mapping.

After the bounded implementation, these gates pass:

```text
go test ./internal/api -run 'TestPhase2DBoardProjectsAgentAttemptRestartRecoveryWithoutRetryAuthority|TestPhase2DBoardProjectsAgentLocalDiagnostics' -count=1
go test -race ./internal/api -run 'TestPhase2DBoardProjectsAgentAttemptRestartRecoveryWithoutRetryAuthority|TestPhase2DBoardProjectsAgentLocalDiagnostics' -count=10
swift test --package-path apps/macos --filter LocalProductModelsTests/testAgentAttemptRestartRecoveryStageIsStrictlyRecognized
swift test --package-path apps/macos --filter LocalProductExperienceViewTests/testAgentRestartRecoveryLabelsDoNotInventRetryAuthority
swift test --package-path apps/macos
go test -p 1 ./internal/api ./cmd/loomd -count=1
go vet ./...
git diff --check
```

The complete Swift run passed 207 XCTest cases with one existing skip and ten
Swift Testing contracts. The existing unrelated Swift 6 generic `Sendable`
warning in `LocalQueueModels.swift` remains unchanged.

## Remaining boundary

An explicit recovered-Attempt command still requires a separate authority
slice: a versioned Journal fact, exact frozen binding and checkpoint
revalidation, Runtime session capability gating, one-use transition semantics,
and a fail-closed distinction between safe pre-model continuation and uncertain
Provider outcome. Automatic Provider replay remains forbidden.

Installed CV6, restart-safe native session persistence, ATL9 mixed-Team live
acceptance, accounting completion, and COMP2-E remain open. No App, network,
Provider, credential, user workspace, or external Runtime was accessed.
