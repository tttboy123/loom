# Phase 1 Slice 4 Exit Contract Amendment 2

Status: FROZEN — independent Contract Repair Review 2 PASS.

Date: 2026-07-26

Baseline: `87ea092`

Parent: `EXIT-CONTRACT.md`

## Name

S4-W2 Authorized Output Summary and Team Recovery Integration Boundary

## Necessity

Accepted S3-W5 already owns the only Team attempt mechanism:

- durable authorized Frame capture and immutable attempt receipts;
- `TeamNodeAttemptTerminal`, explicit `awaiting_recovery`, `retry_at`, and
  distinct attempt/Run/generation/Grant/Evidence lineage;
- the single Team recovery Journal writer;
- deterministic ready-set execution in the TeamCoordinator; and
- the rebuildable Team execution Projection.

That mechanism deliberately accepts an explicit caller-supplied recovery
action and does not infer output semantics. S4-W2 must connect a trusted,
bounded summary of the exact captured output to a pure classifier and pure
recovery policy, then make the accepted Team authority persist and execute
only that bound decision.

This cannot be implemented in isolated new pure-function files. Leaving the
existing caller-selected recovery action unchanged would let the scheduler
invent retry/fallback semantics. Creating a second attempt store, Team writer,
coordinator, or recovery projection would violate accepted authority.

## Exact reopened scope

S4-W2 may reopen only:

- `internal/evidence/attempt_capture.go`
- `internal/evidence/attempt_capture_test.go`
- `internal/evidence/attempt_capture_windows.go`
- `internal/work/team_execution_authority.go`
- `internal/work/team_execution_authority_test.go`
- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`
- `internal/projection/team_execution.go`
- `internal/projection/team_execution_test.go`

The S4-W2 contract must enumerate its new pure verification/rules files and
all governance/evidence files before RED. No other accepted product file is
reopened.

## Permitted accepted-boundary changes

### Evidence attempt summary

- Extend the immutable attempt receipt with a bounded output summary computed
  only from the exact authorized Frames already persisted by the accepted
  attempt capture.
- Keep the receipt and summary non-constructible outside package `evidence`.
  The Team authority accepts that Store-returned receipt object directly and
  derives Evidence ID, Evidence digest, and every summary field from it; no
  caller-supplied string can substitute for receipt authority.
- The summary may expose only:
  - exact Evidence/artifact digest binding;
  - authorized Frame count;
  - output-bearing `event`/`evidence` Frame count and bounded payload-byte
    count;
  - whether the exact terminal `result` Frame was observed;
  - exact Run terminal status; and
  - a canonical summary digest.
- Heartbeat, ack, cancel, and terminal status payloads are not output content.
  An `event` or `evidence` Frame is non-empty output; no Provider-specific or
  hidden-reasoning parser is added.
- Raw payloads, Frame lines, terminal reason, Grant, credential, prompt, or
  hidden reasoning are not returned by the summary and never enter Journal,
  Projection, log, or error.
- Finalize/reopen/exact retry must reproduce the same summary. Summary
  corruption or artifact mismatch fails closed without rewriting accepted
  evidence.
- The Windows unsupported-platform stub may change only enough to preserve the
  public compile surface; no Windows behavior is claimed.

### Team attempt and recovery authority

- Before any first-attempt dispatch, extend `TeamExecutionPlanned` with exactly
  one semantic binding for every logical node: OutputContract version/digest,
  RecoveryPolicy version/digest, total attempt-credit budget, primary workflow
  path, optional one-shot workflow fallback key, and whether recovery approval
  is required. Re-entry must match every frozen binding exactly.
- Accepted historical S3 Team streams without semantic bindings still replay
  as legacy unbound records. They are never silently upgraded or rewritten;
  S4-W2 classification/recovery against such a stream fails closed to the
  existing human-required recovery boundary with zero mutation.
- Bind `TeamNodeAttemptTerminal` to the exact OutputContract digest,
  classification, classification digest, attempt summary digest, and Evidence
  digest in the same existing CAS transaction.
- A valid classification may satisfy the node. `transient_empty` or `invalid`
  leaves it `awaiting_recovery` even when the Runtime process terminal is
  `succeeded`; policy, not process status, decides the next action.
- Replace caller-selected recovery semantics with a validated immutable
  RecoveryDecision produced by the S4-W2 pure policy. The decision binds exact
  Team/plan/node/attempt, current attempt Evidence and classification,
  RecoveryPolicy version/digest, frozen budget/approval requirement, authoritative
  decision time, action, `retry_at`, and optional workflow fallback key.
- S4-W2 cannot satisfy recovery approval. When the frozen binding requires it,
  the pure policy returns `human_required`; no ApprovalRequest ID/digest,
  projected approval, actor string, or caller-supplied approved flag is
  accepted.
- `ScheduleTeamNodeRecovery` replays the exact current attempt, validates the
  complete decision binding, and performs one existing Team-stream CAS with no
  hidden retry.
- Retry/fallback atomically appends `TeamNodeRecoveryRecorded` plus the next
  `TeamNodeAttemptScheduled`. Terminal recovery appends the recovery fact and
  the existing terminal aggregation fact when applicable.
- Exact retries are idempotent. Stale attempt/generation/Evidence,
  classification/policy/budget/approval mismatch, divergent decision, changed
  Team head, exhausted attempts, or invalid next binding produces a typed
  conflict with zero partial Events.
- Every scheduled attempt retains a distinct WorkItem, Run, claim generation,
  AgentGrant, and Evidence lineage. Old generations remain fenced.

### TeamCoordinator

- Supply the complete per-node semantic bindings to the first Team plan
  transaction. It cannot execute an attempt until those bindings are durably
  frozen; any changed binding on restart is an exact conflict.
- Consume only an attempt summary returned by the accepted Evidence Store,
  classify it with the frozen OutputContract, decide recovery with the frozen
  RecoveryPolicy, commit the classification, and submit the exact immutable
  decision to the accepted Team authority.
- Execute only a recovery decision that was successfully persisted. It may
  dispatch a due attempt in a later bounded pass; it never sleeps, polls, or
  retries a CAS conflict.
- A future `retry_at` returns the current non-terminal result without executing
  the next attempt. Restart/re-entry recomputes the same classification and
  decision from immutable inputs, recognizes exact committed facts, and
  resumes only when due.
- One invocation is bounded by the three-node, three-attempt plan ceiling and
  a fixed finite pass count. There is no recursive or open-ended scheduler
  loop.
- Workflow fallback selects only an explicitly configured local workflow path
  for the next attempt. It does not change Provider/model, discover a
  replacement Runtime, or use ambient fallback.

### Projection and GlobalReadView

- Project OutputContract/classification/summary and RecoveryPolicy/decision
  metadata from the two accepted Team Events.
- Copy nested attempt and recovery records during rebuild and access; no
  mutable slice/map aliases may escape.
- Publish a new Team execution view only after complete replay succeeds.
  Malformed classification, policy, budget, fallback, or decision facts fail
  closed and preserve the exact previous Projection/GlobalReadView.
- Projection/View remain rebuildable and never classify, decide, schedule, or
  authorize recovery.

## Preserved authority

This amendment does not allow:

- Bridge v1, Supervisor, Pi Adapter, AgentGrant, Journal storage, Runtime
  discovery, approval authority, WorkItem acceptance, or Evidence artifact
  publication changes;
- raw output or per-token Journal Events;
- Provider/model automatic fallback, session/context checkpoint, process
  replay, hidden retry, or retry based only on a Projection;
- Executor `done`, deterministic acceptance, independent Verifier, S4-W3,
  API/CLI/daemon/client streaming, S4-W4, or Phase 2 work; or
- installed Runtime, Provider/model traffic, credentials, network, external
  approval, autonomous execution, or publication.

## Verification

S4-W2 must prove:

- authorized non-empty, allowed empty, transient empty, and invalid attempt
  summaries classify deterministically and immutably;
- summary exact retry/reopen, digest binding, corruption rejection, and no raw
  payload leakage;
- pure recovery decision for retry, workflow fallback, degraded, blocked, and
  human-required, including attempt/budget exhaustion and fail-closed approval
  gating;
- pre-dispatch semantic binding freeze and crash/re-entry rejection of changed
  OutputContract, RecoveryPolicy, budget, workflow fallback, or approval
  requirement;
- classification and decision facts are exact-Evidence/attempt/generation
  bound, terminal/recovery writes are CAS atomic, and concurrent schedulers
  have one winner;
- future `retry_at` causes zero execution, due retry/fallback creates a new
  lineage, stale generations are rejected, and restart produces no duplicate
  Run/Grant/Evidence;
- malformed Projection replay preserves the old view;
- accepted historical S3 Team journals rebuild without semantic authority,
  while any attempt to apply S4-W2 recovery to an unbound legacy stream fails
  closed; and
- full repository/race/vet/format/platform-build/diff/scope/trust gates pass.

VERDICT: FROZEN
