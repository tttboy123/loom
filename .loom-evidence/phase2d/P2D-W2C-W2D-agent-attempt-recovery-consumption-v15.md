# P2D-W2C/W2D Agent Attempt Recovery Consumption V15

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / ONE-USE DECISION CONSUMPTION / RUNTIME COMPOSITION OPEN  
**Contracts**: `contracts/P2D-W2C-agent-attempt-binding-dispatch.md`, `contracts/P2D-W2D-observability-governance.md`

## Accepted boundary

An authorized V14 `resume_pre_model` decision can now be consumed exactly once.
The authority rebuilds the exact V10 candidate, resolves the trusted Runtime
capability twice, and reads a consistent recovery/Attempt Loop/Run snapshot.
Only the exact decision, candidate digest, capability digest, frozen Run and
Loop heads may commit `AgentAttemptRecoveryConsumed`.

The authority generates a fresh lease ID internally from an injected random
source. The caller cannot choose an idempotency key that would make two
concurrent consumers both appear successful. Distinct or identical concurrent
consumption calls have one CAS winner. After commit, every later consumption
fails with `ErrAgentAttemptRecoveryConsumed` and receives no lease.

The returned `AgentAttemptRecoveryDispatchLease` is process-local and can be
`Take`n only once. It yields a cloned frozen candidate and restart capability to
trusted product composition. It is not exposed through IPC, is not an active
Attempt registration, and does not dispatch a Runtime or Provider.

Capability drift during the confirmation pass leaves the recovery stream at the
authorization fact. The consumed Journal fact carries only the lease/decision,
candidate/capability digests, frozen non-secret Attempt/Run/Runtime/Agent
identity, correlation, and authoritative causation. Inbox content, Prompt,
transcript, native handle, credential, Authorization Header, Provider response,
and environment remain absent.

## RED and verification

The mandatory RED failed on the missing random source, consume command,
consumed fact, dispatch lease, and already-consumed error. After implementation,
the following gates pass:

```text
go test ./internal/work -run 'TestAgentAttemptRecoveryAuthority' -count=1
go test -race ./internal/work -run 'TestAgentAttemptRecoveryAuthority' -count=10
go test ./internal/work -count=1
go test -p 1 ./... -count=1
go vet ./...
git diff --check
```

Tests prove one durable consumer, one in-process `Take`, concurrent one-winner
CAS, capability drift before commit, content-free persistence, and rejection of
all later consumption attempts.

## Remaining boundary

Production composition still supplies no restart-safe capability resolver and
does not consume the lease. The next RED must bind a version-locked Runtime
reattachment adapter to the exact lease, register the recovered Attempt only
after successful attachment, and revoke that registration on failure or close.
It must never translate an uncertain Provider outcome into a new request.

Authenticated IPC and Swift approval/consume actions, recovery decision
projection, restart-safe native session persistence, installed CV6, ATL9
mixed-Team acceptance, accounting completion, and COMP2-E remain open. No App,
network, Provider, credential, user workspace, or external Runtime was accessed.
