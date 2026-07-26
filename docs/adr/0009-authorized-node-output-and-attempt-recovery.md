# ADR-0009: Authorized node output and explicit attempt recovery

**Date**: 2026-07-26
**Status**: accepted
**Deciders**: lune, Codex

## Context

The accepted managed Runtime Adapter returns a bounded batch of Bridge Frames
only after its child exits. Supervisor then validates and authorizes the batch.
That ordering can prove a terminal result but cannot safely expose incremental
node output: a caller-visible callback before Supervisor validation would let a
malformed, stale-generation, or unauthorized Frame cross the trust boundary.

A logical Team node may also need another attempt after transient or
data-quality failure. Reusing one Run or silently retrying a scheduler command
would blur generation, Grant, Evidence, and recovery authority.

## Decision

The Adapter receives a Supervisor-owned synchronous Frame sink. For each
decoded inbound Frame, Supervisor first validates the full Bridge binding,
ordering, sequence, ack/result semantics, and raw-token exclusion; computes an
immutable candidate BoundRunStream; obtains exact AgentGrant authorization;
commits that candidate in memory; and only then invokes an optional authorized
Frame observer.

Observed Frames are deeply copied and tentative. The terminal AdapterResult
must exactly reconcile with the accepted stream. Observer failure, missing sink
delivery, or reconciliation mismatch fails the managed execution and uses the
accepted cleanup/terminal path. The observer is not execution or acceptance
authority.

Team execution separates a stable logical node from attempts. Attempt numbers
start at one and are capped at three. Each attempt has deterministic distinct
WorkItem, Run, Grant, and Evidence identities and explicit Agent/Runtime and
generation binding. Retry or fallback is scheduled only by an explicit
recovery directive with canonical `retry_at`; a scheduler pass receives an
authoritative time, makes one CAS attempt, and never sleeps or retries a
conflict.

Authorized Frames are also written to a private, bounded, non-authoritative
attempt capture before an application observer receives them. A successful
Run requires an exact child result Frame; accepted Supervisor-generated failed
or cancelled terminals may finalize without one. Finalization publishes one
content-addressed artifact and persists a durable Evidence-ID-to-digest
receipt. Only the Journal transaction that commits Evidence metadata and the
Team attempt terminal changes authoritative state.

Restart recovery follows one explicit cross-store order: reclaim the expired
claimed Run through Journal CAS, rebind an empty capture to generation N+1,
append the Team rebound Event that fences generation N, issue a new Grant, and
execute once. Exact split states at those boundaries are idempotent. A terminal
Run can finalize a complete capture or reuse its receipt without executing
again. A Run already in `running` without an accepted terminal is
indeterminate and returns typed human-required recovery instead of replaying or
fabricating a terminal.

Slice 3 implements this mechanism but does not classify empty output or choose
retry/fallback/degraded/blocked/human-required semantics. Those decisions
belong to Slice 4 policy. Client buffering, reconnect, and delivery belong to
Slice 5.

## Alternatives Considered

### Alternative 1: Publish Adapter Frames before Supervisor validation

- **Pros**: Minimal latency and no callback into Supervisor.
- **Cons**: Malformed, unauthorized, or stale output can escape before the
  authority rejects it.
- **Why not**: Observation crosses the same trust boundary as result
  acceptance.

### Alternative 2: Stream after process exit from AdapterResult

- **Pros**: Preserves existing batch validation.
- **Cons**: It is replay, not incremental observation, and cannot support a
  live local timeline.
- **Why not**: It does not close the queued observable-output capability.

### Alternative 3: Retry the same Run and Grant

- **Pros**: Fewer records and simpler UI identity.
- **Cons**: Old output and credentials remain ambiguous across attempts and
  stale generation fencing is weakened.
- **Why not**: Every attempt needs independent replayable authority.

### Alternative 4: Let the scheduler infer empty-output recovery

- **Pros**: One component and immediate automation.
- **Cons**: Execution mechanics would invent business acceptance and Provider
  fallback semantics.
- **Why not**: Verification and recovery policy must be explicit authorities in
  Slice 4.

## Consequences

### Positive

- No caller sees a Frame before exact validation and authorization.
- Logical progress remains stable while every attempt has isolated authority
  and Evidence.
- Recovery is time-explicit, bounded, restartable, and conflict-visible.
- Process loss between Run terminal, artifact publication, receipt persistence,
  and Evidence metadata commit does not require duplicate execution.
- Later client delivery can consume tentative deltas without making them
  Journal or terminal authority.

### Negative

- Adapter stdout processing may backpressure on bounded authorization and
  observer work.
- Terminal reconciliation retains a bounded batch in addition to incremental
  callbacks.
- Attempt capture adds a private filesystem spool whose lifecycle must be
  reconciled with, but never replace, Journal authority.
- Failed attempts may remain `awaiting_recovery` until Slice 4 supplies an
  explicit directive.

### Risks

- A slow observer could consume the Runtime deadline. Observer calls use the
  execution context and their errors fail closed.
- An Adapter could skip the sink and still return a batch. Exact reconciliation
  rejects missing or divergent sink delivery.
- Incremental payload could be mistaken for final output. Every observed item
  is explicitly tentative until terminal/Evidence authority is committed.
- Per-token persistence could inflate the Journal. Output payload deltas are
  never Journal Events; only accepted authorization and lifecycle metadata are
  persisted.
- A crash can leave Run and capture one generation ahead of the Team stream.
  Only the reviewed N+1 split is accepted; other divergence fails closed.
