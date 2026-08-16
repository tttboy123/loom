# P2D-W2C/W2D Loom Native Agent Input Consumption V5

Status: `SOURCE VERIFIED / LOOM NATIVE ONLY / IPC AND INSTALLED LIVE OPEN`

Date: 2026-08-14

## Acceptance boundary

This slice connects the durable encrypted Agent Inbox to one real production
Runtime without re-invoking the whole Runtime Adapter. Loom Native can now
consume ordered Steer/Inject inputs as the exact next Step and Queue input as
the exact next Turn while retaining one frozen Execution Binding, one
authoritative Route Segment, one credential lease, and one final Bridge result.

It does not expose daemon IPC or Swift controls and does not claim Pi, Codex,
Claude Code, installed App, or mixed-Team conformance.

## Source changes

- `internal/runtime/agent_input.go` defines the bounded Runtime input source,
  output checkpoint, zeroizable input batch, strict target/digest/scope checks,
  and ownership cleanup.
- `internal/work/agent_inbox_coordinator.go` releases plaintext only after the
  exact Step/Turn transition and encrypted storage consumption status agree.
- `cmd/loomd/product_attempt_loop_runtime.go` owns the per-Attempt input source,
  prioritizes the exact next Steer/Inject batch, consumes Queue only after the
  current Turn succeeds, admits each next model request, and moves a
  content-free Turn/Step cursor used by Context delivery.
- `internal/runtime/nativeadapter/deepseek_adapter.go` keeps one Provider
  conversation and credential lease across input rounds, combines accounting,
  and publishes only the final assistant/result frames.
- Runtime input bytes remain mutable. Inbox payloads, Runtime batches, message
  wire buffers, JSON request bodies, and temporary HTTP payloads are cleared at
  their ownership boundary. No input body enters Journal facts or diagnostics.

## Verified behavior

1. Turn 1 Step 1 final output plus pending Steer starts Turn 1 Step 2 with one
   atomic Inbox-consumption/`StepStarted` batch.
2. Pending Queue does not wake the active Turn. After Step 2 final output it
   closes Turn 1, atomically consumes Queue with `TurnStarted`, then admits Turn
   2 Step 1.
3. The real Loom Native DeepSeek-compatible adapter performs three Provider
   rounds under one exact Provider Account, credential revision, model, Route
   Segment, and credential lease.
4. Intermediate assistant output is Provider context only. Supervisor receives
   one Ack, one final assistant Event, and one terminal Result.
5. Context tool delivery reads the current content-free Step cursor rather than
   attributing later calls to Step 1.
6. Provider/request and Agent input plaintext are absent from Event Journal
   payloads. Mutable input references are zero after completion.

## Verification

Passed:

```text
go test -race ./internal/runtime ./internal/work ./internal/runtime/nativeadapter ./cmd/loomd -run 'Test(NewAgentInputBatch|AgentInboxCoordinatorConsumes|LoomNativeConsumesStepAndQueueInputsWithinOneFrozenAttempt|ProductAttemptLoopRuntimeConsumesSteerThenQueueWithoutDuplicateTerminal|ProductLoomNativeConsumesAuthoritativeInboxAcrossProviderRounds)' -count=10
go test ./... -count=1
```

The full repository Go run passed, including daemon, Supervisor, Pi Adapter,
Vault, Context Capsule, API, app, projection, and Bridge packages.

## Open gates

- authenticated daemon Agent Inbox admission and active-Attempt lookup route;
- Swift Queue/Steer/Inject controls and Agent-local recovery presentation;
- Pi, Codex, Claude Code, and additional Loom Native Provider conformance;
- explicit crash recovery for interruption between a completed Step and the
  next Runtime input transition;
- installed CV6, mixed-Team ATL9, and COMP2-E removal gates.

No App was built, signed, launched, installed, or modified in Applications. No
network, real Provider, real credential, user workspace, or external Runtime
was accessed.
