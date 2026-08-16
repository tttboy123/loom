# P2D-W2C/W2D Agent Input Pre-Model Crash Recovery V7

Status: `SOURCE VERIFIED / BOUNDED PRE-MODEL WINDOW / DAEMON RESTART OPEN`

Date: 2026-08-14

## Closed failure window

Agent Inbox consumption commits authoritative Journal facts before the Vault
status is changed from `pending` to `consumed`. A store failure or process exit
after the Journal batch could therefore leave an exact Step or Turn transition
and `AgentInputConsumed` fact while Runtime plaintext had not been released.
The earlier reconciliation path aligned Vault status but could not redeliver
that input, producing an authoritative-consumed/model-unseen state.

The coordinator now permits an exact consumed-input recovery only while:

- the frozen Step exists with the same sequence and model-input digest, has no
  outcome, and has no `ModelRequestAdmitted`; or
- the frozen Queue Turn exists, remains open, and has no Step, or only its exact
  first open Step with no `ModelRequestAdmitted`.

Recovery idempotently aligns the encrypted Vault record to consumed, decrypts
the exact binding, rechecks content digest and status, returns mutable bytes,
and retains normal caller zeroization. A mismatched binding, digest, status,
target, later Step, terminal state, or admitted ModelRequest fails closed.

## Product Runtime recovery

The Loom product `AgentInputSource` now recognizes the same authoritative
pre-model windows. It requires the prior Step/Turn output digest to equal the
Runtime checkpoint, finds only consumed inputs frozen for the current target,
recomputes the model-input digest, restores the exact cursor, and admits one
deterministic ModelRequest before returning a batch.

For Queue, recovery covers both a committed Turn with no Step and a committed
first Step with no ModelRequest. For Steer/Inject, it covers the committed next
Step with no ModelRequest. A wrong checkpoint cannot retrieve plaintext or
advance state. Once ModelRequest authority exists, stale-cursor recovery is
rejected rather than replaying a Provider request.

## Verification

- Coordinator RED reproduced both lost-delivery states as
  `ErrAgentInboxConflict` before the implementation.
- Product tests inject the first Vault status-write failure, then prove exact
  Step and Queue recovery through the real `AgentInputSource`.
- Queue verification advances through the deeper Turn+Step/no-ModelRequest
  crash state before recovery.
- Recovered payloads are zeroized; wrong checkpoint and post-ModelRequest
  recovery fail closed.
- Focused coordinator/product recovery race matrix passed 20 runs.
- `go vet ./...` passed.
- `git diff --check` passed before evidence updates.
- Frozen-source `go test -p 1 ./... -count=1` passed.

## Open gates

This result does not claim general exactly-once Provider execution. Daemon
restart orchestration still needs to reconstruct the correct Runtime checkpoint
and invoke this recovery path. A crash after `ModelRequestAdmitted` has an
uncertain Provider-dispatch boundary and must enter explicit recovery-required
governance rather than automatic replay. Pi/Codex/Claude input consumption,
installed CV6, mixed-Team ATL9, and COMP2-E remain open. No App, network, real
Provider, credential, user workspace, or external Runtime was accessed.
