# Phase 1 Slice 3 Exit Contract Amendment 3 — Observable Node Output and Bounded Recovery Boundary

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Parent:
  `.loom-evidence/phase1-slice3/EXIT-CONTRACT.md`
- Queued authority: observable authorized node output plus bounded attempt
  recovery, without W6

## Necessity

S3-W4's accepted production Adapter returns all inbound Bridge Frames only
after the process exits in one `AdapterResult`. Supervisor validates and
authorizes that batch afterward. Therefore S3-W5 cannot expose real incremental
node output while proving that every published Frame first passed Supervisor
binding/sequence validation and AgentGrant authorization.

S3-W5's reviewed contract also models one fixed Run per logical node. It cannot
record an explicit bounded retry time or give each attempt an independent
Run/generation/Grant/Evidence lineage.

Both gaps affect the final S3-W5 vertical execution boundary. Adding W6 or
silently broadening S3-W4/S3-W5 would violate the accepted Exit Contract.

## Exact reopened S3-W4 files

S3-W5 may make bounded compatibility edits only to:

- `internal/supervisor/managed_execution.go`
- `internal/supervisor/managed_execution_test.go`
- `internal/runtime/piadapter/execution_adapter.go`
- `internal/runtime/piadapter/execution_adapter_test.go`

No other S3-W4 product or test file is reopened.

## Authorized Frame observation

The S3-W5 contract amendment must freeze an exact API equivalent to:

```go
type FrameSink interface {
    AcceptFrame(context.Context, bridgev1.Frame) error
}

type AuthorizedFrame

type AuthorizedFrameObserver interface {
    ObserveAuthorizedFrame(context.Context, AuthorizedFrame) error
}
```

`AdapterRequest` carries the Supervisor-owned `FrameSink`; `ExecuteInput`
carries an optional caller-owned `AuthorizedFrameObserver`. The production Pi
Adapter calls the sink synchronously exactly once for every successfully
decoded inbound Frame.

The authoritative order is:

1. Adapter performs bounded line read and exact Bridge decode.
2. Supervisor rejects raw Grant bytes, forbidden message type/order, binding,
   identity, sequence, duplicate, late-after-result, and invalid ack/result
   semantics.
3. Supervisor computes the candidate `BoundRunStream` advance without
   publishing it.
4. `AgentGrant.Authorize` validates the exact Run, Agent, generation,
   operation, expiry, and Frame message ID.
5. Supervisor commits the candidate BoundRunStream in memory.
6. Only then may Supervisor invoke `ObserveAuthorizedFrame`.

Malformed, unauthorized, expired, revoked, stale-generation, duplicate,
out-of-sequence, post-result, or observer-rejected Frames fail the managed
execution and trigger the accepted bounded process-group cleanup/terminal path.
They are never observed.

The observer receives a deeply copied Frame, immutable Run generation binding,
and `tentative=true`. It receives no raw Grant/token, credential, environment,
hidden reasoning, or mutable internal stream. Observer errors are typed and
bounded by the execution context.

The existing batch `AdapterResult` remains the terminal reconciliation proof.
Supervisor must compare it exactly with the Frames accepted by its sink; the
Adapter cannot manufacture a separate streamed result. A Runtime Adapter that
does not honor the supplied sink fails closed.

No Bridge v1 message, payload, protocol version, encoder/decoder, sequence
rule, maximum buffered Frame count, process timeout, cleanup rule, or Grant
operation is changed.

## S3-W5 logical-node and attempt mechanism

S3-W5 remains one WorkItem and may extend its reviewed owned scope with:

- `internal/projection/team_execution.go`
- `internal/projection/team_execution_test.go`

The S3-W5 contract amendment must replace the one-Run-per-node model with:

- stable `logical_node_id`;
- monotonic `attempt_number`, starting at one and bounded to at most three;
- explicit immutable attempt-specific WorkItem, Run, AgentInstance,
  RuntimeInstance, claim generation, Grant, and Evidence bindings;
- node states `pending`, `running`, `retry_scheduled`, `fallback_scheduled`,
  `degraded`, `blocked`, `human_required`, `succeeded`, `failed`, and
  `cancelled`;
- attempt states `scheduled`, `dispatched`, `running`, `succeeded`, `failed`,
  and `cancelled`;
- canonical UTC `retry_at` for scheduled retry/fallback;
- append-only planned/dispatched/started/recovery/terminal/Evidence
  milestones; and
- deterministic restart replay and terminal aggregation across attempts.

Each retry/fallback attempt has a new Run ID, claim generation lineage,
AgentGrant, and Evidence ID/digest. A prior attempt's generation or Grant
cannot start, publish a Frame, submit Evidence, or commit terminal for a later
attempt.

One scheduler pass may dispatch only attempts whose explicit `retry_at` is not
after the caller-supplied authoritative time. It performs one CAS attempt and
never sleeps, polls, or automatically retries a conflict. Maximum attempts and
the three-node Team bound keep recurrence finite.

S3-W5 implements mechanism only. It may accept an explicit already-authorized
recovery directive, but it may not infer `retry`, `fallback`, `degraded`,
`blocked`, or `human_required` from empty output or business content.

## Queued later-Slice boundaries

These are explicit future contract inputs, not S3-W5 scope:

### Slice 4 semantic policy

- `internal/verification/output_contract.go` classifies
  `valid_nonempty`, `valid_empty`, `transient_empty`, or `invalid`.
- `internal/rules/recovery_policy.go` decides
  `retry`, `fallback`, `degraded`, `blocked`, or `human_required`.
- Scheduler executes that decision and never invents acceptance semantics.
- Data-source fallback is not Provider/model automatic fallback.

### Slice 5 client delivery

- `internal/api/team_execution_stream.go` plus bounded daemon/CLI wiring.
- Tentative text deltas use bounded memory and are not Journal Events.
- Authoritative started/retry/degraded/terminal milestones come from
  Journal/Projection.
- Reconnect uses cursor/Last-Event-ID plus GlobalReadView.
- Slow consumers may coalesce text deltas but never lose
  warning/retry/degraded/terminal; overflow emits `stream_gap` plus an
  `artifact_digest`.
- Phase 1 claims only a local event interface and CLI timeline. Web/TUI remains
  excluded.

No per-token Journal write is permitted.

## Security and non-authority

- Incremental output is tentative until terminal and Evidence acceptance.
- Raw Grant material, credentials, hidden reasoning, and ambient private state
  are never observed or delivered.
- Observer/callback state is not execution, terminal, Evidence, or acceptance
  authority.
- No installed user Runtime, Provider/model call, credentials, network
  publication, daemon activation, autonomous loop, or external client is
  authorized by this amendment.
- No S3-W6, second writer, checkpoint/cache authority, Bridge protocol change,
  push, merge, release, or publication is authorized.

## Review and gate

This amendment requires a fresh independent read-only review. After PASS,
S3-W5 must freeze and review a Contract Amendment 2 with the exact Frame
observation and attempt APIs, Events, owned files, RED, canary, recovery,
security, and verification matrix before any product edit.

VERDICT: FROZEN
