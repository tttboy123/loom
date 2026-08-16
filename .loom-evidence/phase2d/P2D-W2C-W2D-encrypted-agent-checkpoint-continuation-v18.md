# P2D-W2C/W2D Encrypted Agent Checkpoint Continuation V18

Status: `SOURCE VERIFIED / PRODUCTION RECOVERY ACTION OPEN`

Date: 2026-08-14

## Boundary

This slice closes the source-level Loom Native continuation path left open by
V17. It does not expose a user-visible Resume action and does not claim an
installed or live Provider result.

V18 is split into three source boundaries:

1. V18A adds a dedicated encrypted Agent Checkpoint Store. Checkpoints use the
   Conversation DEK boundary but do not reuse Attempt Tool Result storage or
   enter the Event Journal. The authenticated record binds checkpoint,
   Conversation, Segment, Attempt, Agent, WorkItem, Run/generation, Runtime,
   frozen Execution Binding, Capsule, Turn, Step, content type, digest and
   schema version. Exact resolution rejects both zero and multiple candidates.
2. V18B requires Loom Native to persist the mutable final-output checkpoint
   before consuming the next Agent Input. Persistence failure prevents input
   consumption and prevents the next Provider request. Successful terminal
   completion removes the temporary checkpoint.
3. V18C restores the exact consumed Inbox material only while strict Journal
   replay still yields the same pre-model candidate. Trusted product
   composition consumes the one-use recovery grant, attaches and registers the
   exact active Attempt, restores Capsule/checkpoint/input bytes, admits one new
   ModelRequest, acquires the exact credential lease, dispatches Loom Native,
   terminalizes Step/Turn, removes the checkpoint, and unregisters the Attempt.

The resumed Provider request is assembled from the existing governed system
prompt, exact frozen Context Capsule dispatch payload, authenticated previous
output checkpoint, and exact consumed Agent Input. Binding, Capsule,
checkpoint, Segment, input, Provider Account, credential revision or Model
substitution fails before credential access or HTTP.

## Recovery and privacy invariants

- Only the V10 `pre_model_resume` path can continue. An admitted request with
  uncertain Provider outcome remains fail closed and is never replayed.
- The checkpoint is a separate encrypted artifact, not a Tool Result and not a
  Journal payload. Conversation crypto-erasure cascades to its records.
- A duplicate checkpoint digest for the same frozen route is ambiguous and
  fails closed; recovery never guesses the newest candidate.
- The current consumed Inbox payload is released only after exact authority and
  binding reconstruction. Mutable checkpoint and input buffers are cleared by
  their owners.
- Journal facts and operational diagnostics retain only content-free identity,
  digest, stage and terminal metadata. Tests assert that checkpoint, input,
  final reply and credential fixture bytes are absent from Journal storage.
- Recovery registers the active Attempt only after one-use grant consumption
  and session attachment. Failure and normal close both revoke that exact
  registration.
- Once the Attempt Loop reaches an authoritative terminal state, checkpoint
  deletion uses a separate five-second cleanup context. Request cancellation
  cannot leave terminal cleanup bound to an already-cancelled context.

## Source

- `internal/agentcheckpoint/model.go`
- `internal/credentials/vault/agent_checkpoint_store.go`
- `internal/credentials/vault/vault_store.go`
- `internal/runtime/agent_input.go`
- `internal/runtime/nativeadapter/agent_continuation.go`
- `internal/runtime/nativeadapter/deepseek_adapter.go`
- `internal/work/agent_inbox_recovery.go`
- `cmd/loomd/product_attempt_loop_runtime.go`
- `cmd/loomd/product_agent_attempt_recovery_runtime.go`
- `cmd/loomd/product_loom_native_attempt_recovery.go`
- `cmd/loomd/product_credential_vault.go`
- `cmd/loomd/product_composition_vault.go`
- `cmd/loomd/product_daemon.go`

## Verification

Focused RED/GREEN coverage proves encrypted persistence and restart, exact
binding and AAD authentication, tamper and ambiguity rejection, crypto-erasure,
capture-before-consume ordering, consumed Inbox reconstruction, continuation
wire assembly, pre-credential substitution rejection, active registry
lifecycle, Step/Turn terminalization and Journal non-disclosure.

Passed:

```text
go test -race ./internal/runtime ./internal/runtime/nativeadapter \
  ./internal/credentials/vault ./internal/work ./cmd/loomd \
  -run 'TestAgentCheckpointStore|TestAgentInputCheckpointPayload|TestLoomNativePersistsCheckpoint|TestLoomNativeCheckpointFailure|TestLoomNativeResume|TestAgentInboxCoordinatorReconstructsRestartCheckpoints|TestAgentInboxCoordinatorReconstructsConsumedStepCheckpoint|TestProductLoomNativeAttemptRecovery|TestProductAgentAttemptRecovery|TestProductLoomNativeConsumesAuthoritativeInboxAcrossProviderRounds|TestCOMP2CVaultBundleOwnsRuntimeAndSlotDelegates' \
  -count=10

go test -race ./cmd/loomd \
  -run 'TestProductRecoveryCheckpointCleanupOutlivesCancelledRequest|TestProductLoomNativeAttemptRecovery' \
  -count=10

go test -p 1 ./... -count=1
go test ./cmd/loomd -count=1
go vet ./...
git diff --check
```

The exact touched Go file set also produced no output from `gofmt -l`.

## Open gates

The production daemon does not yet construct and publish the continuation
service through an authenticated recovery route. There is no local IPC command,
Swift store action, governance confirmation, or user-visible Resume control.
That route must not call the continuation directly: it must also reconstruct a
recovery-specific authorized frame sink, commit the current Run terminal and
accounting facts, and close the prior execution authorization. Otherwise model
completion and Team board authority could diverge.
The end-to-end continuation is source-verified only through a local controlled
fixture with a simulated HTTP Provider and test credential accessor; no App,
network, real Provider, real credential, user workspace or external Runtime was
accessed.

Codex, Claude Code and Pi remain restart-ineligible. Installed CV6, mixed-Team
ATL9, full accounting/governance UI and COMP2-E parity/removal gates remain open
under the sole `ACTIVE / PARTIAL` Phase 2D Goal.
