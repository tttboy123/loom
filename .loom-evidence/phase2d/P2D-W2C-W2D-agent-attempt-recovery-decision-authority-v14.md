# P2D-W2C/W2D Agent Attempt Recovery Decision Authority V14

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / DECISION AUTHORITY ONLY / DISPATCH LEASE AND UI OPEN  
**Contracts**: `contracts/P2D-W2C-agent-attempt-binding-dispatch.md`, `contracts/P2D-W2D-observability-governance.md`

## Accepted boundary

`AgentAttemptRecoveryAuthority` now records one explicit, versioned user
decision for a V10 restart candidate. The command binds a domain-separated
candidate digest, a separately domain-separated restart-capability digest, an
authenticated principal identity placeholder, and a correlation ID. Only
`pre_model_resume` is eligible. `provider_outcome_uncertain` is rejected before
any authority fact is written.

The restart capability is not accepted from the decision caller. A trusted
`AgentAttemptRestartCapabilityResolver` supplies the exact Runtime capability,
which binds Attempt, Runtime instance, frozen Execution Binding, Context Capsule,
restart-safe checkpoint mode, and native session-binding digest. Current product
composition provides no resolver, so this source boundary cannot activate a
real Runtime.

Authorization rebuilds the protected candidate twice around a consistent
Journal snapshot. Its append CAS freezes the recovery-decision stream, Attempt
Loop stream, and Run stream heads. Candidate drift, Run terminalization, a new
ModelRequest, binding substitution, capability drift, and a concurrent second
decision fail closed. An exact repeat of the already committed decision is
idempotent.

The `AgentAttemptRecoveryAuthorized` fact contains only non-secret identities,
digests, controlled mode, and sequence metadata. It contains no Inbox content,
Prompt, transcript, Provider response, credential, Authorization Header, native
session handle, or process environment.

## RED and verification

The mandatory RED failed only because the recovery authority, decision,
capability, and digest contracts did not exist. The completed implementation
passes:

```text
go test ./internal/work -run 'TestAgentAttemptRecoveryAuthority' -count=1
go test -race ./internal/work -run 'TestAgentAttemptRecoveryAuthority' -count=10
go test ./internal/work -count=1
go test -p 1 ./... -count=1
go vet ./...
git diff --check
```

Tests prove exact and idempotent approval, content-free Journal persistence,
trusted capability resolution, capability-digest drift rejection, uncertain
Provider rejection, and exactly one winner between concurrent distinct
decisions.

## Remaining boundary

V14 intentionally returns no executable dispatch token. A later RED slice must
add one-use consumption of the decision, revalidate the same Run/Loop/candidate
heads, require a production Runtime resolver with restart-safe native session
reattachment, and only then register the Attempt and dispatch. Commit-outcome
ambiguity must fail closed; automatic Provider replay remains forbidden.

Authenticated IPC, Swift approval controls, recovery projection of the decision,
native session persistence, installed CV6, ATL9 mixed-Team acceptance,
accounting completion, and COMP2-E remain open. No App, network, Provider,
credential, user workspace, or external Runtime was accessed.
