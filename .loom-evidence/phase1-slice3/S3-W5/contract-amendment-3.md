# S3-W5 Contract Amendment 3

Status: FROZEN — independent Review 1 Round 4 PASS.

This amendment is valid only with Slice 3 Exit Contract Amendment 5.

## Durable attempt capture API

Package `internal/evidence` adds:

```go
var (
    ErrInvalidAttemptCapture = errors.New("invalid attempt capture")
    ErrAttemptCaptureConflict = errors.New("attempt capture conflict")
    ErrAttemptCaptureIncomplete = errors.New("attempt capture incomplete")
)

type AttemptCaptureInput struct {
    EvidenceID string
    TeamInstanceID string
    PlanDigest string
    LogicalNodeID string
    AttemptNumber int
    WorkItemID string
    RunID string
    ClaimID string
    ClaimGeneration int64
    RuntimeInstanceID string
    AgentInstanceID string
}

type AttemptTerminal struct {
    Status string
    Reason string
}

type AttemptReceipt
type AttemptCaptureState

func (AttemptReceipt) EvidenceID() string
func (AttemptReceipt) Digest() string
func (AttemptCaptureState) Binding() AttemptCaptureInput
func (AttemptCaptureState) FrameCount() int
func (AttemptCaptureState) ResultObserved() bool

func (*Store) BeginAttemptCapture(context.Context, AttemptCaptureInput) error
func (*Store) AppendAttemptFrame(
    context.Context,
    string,
    []byte,
) error
func (*Store) RebindAttemptCapture(
    context.Context,
    AttemptCaptureInput,
) error
func (*Store) FinalizeAttemptCapture(
    context.Context,
    string,
    AttemptTerminal,
) (AttemptReceipt, error)
func (*Store) AttemptReceipt(
    context.Context,
    string,
) (AttemptReceipt, bool, error)
func (*Store) AttemptCapture(
    context.Context,
    string,
) (AttemptCaptureState, bool, error)
```

The Evidence ID is the deterministic S3-W5 attempt Evidence ID. Begin is
idempotent only for an exact binding. Rebind is allowed only when the capture has
zero Frames, WorkItem/Run/Team/plan/node/attempt/Runtime/Agent are unchanged,
and `ClaimGeneration` increases by exactly one with a different valid Claim ID.
Rebind advances the filesystem capture before the Team rebound Journal Event.
It is idempotent when the capture already has the exact new binding. An old
binding may advance only to the exact accepted Run head generation; a binding
ahead by more than one generation or any divergent field conflicts.

`AppendAttemptFrame` accepts one complete canonical Bridge JSON line after
Supervisor authorization. It rejects invalid JSON, blank/multiple lines,
payloads containing the raw-Grant token marker, binding/sequence divergence,
post-result Frames, more than `bridgev1.MaxBufferedFrames`, or more than 1 MiB
total capture bytes. Exact duplicate append is idempotent; divergent duplicate
sequence conflicts.

Finalize requires a valid accepted terminal status/reason. When a terminal
result Frame is present, it must be the final captured Frame and must reconcile
exactly with the supplied terminal. A missing result Frame is accepted only for
an accepted Supervisor-generated `failed` or `cancelled` terminal with a
non-empty bounded reason; this covers timeout, cancellation, workspace/source,
adapter/protocol, observer, and cleanup failure paths without fabricating child
output. A `succeeded` terminal always requires one exact result Frame.

Finalize creates canonical artifact JSON from the immutable binding, captured
canonical Frame lines, accepted terminal, and a boolean
`child_result_observed`. It publishes by SHA-256 through the existing
content-addressed Store, then atomically persists a private receipt mapping the
Evidence ID to that digest. Exact retry reuses the same digest. A different
terminal or capture conflicts. A begun zero-Frame capture is complete enough to
finalize only for the accepted Supervisor-generated terminal rule above.

Receipt/capture directories remain descriptor-bound, mode `0700`; files are
mode `0600`; symlinks, replacement, traversal, unknown/duplicate JSON fields,
oversized content, and digest mismatch fail closed. Windows returns the accepted
unsupported-platform error until an equivalent no-follow atomic implementation
exists.

## Team generation rebound

Package `internal/work` adds:

```go
var ErrTeamAttemptRecoveryRequired =
    errors.New("Team node attempt recovery requires human action")

type TeamAttemptRebindInput struct {
    TeamInstanceID string
    PlanDigest string
    LogicalNodeID string
    AttemptNumber int
    WorkItemID string
    RunID string
    PreviousClaimID string
    PreviousClaimGeneration int64
    ClaimID string
    ClaimGeneration int64
    RuntimeInstanceID string
    AgentInstanceID string
    CorrelationID string
}

func (*Authority) RebindTeamAttempt(
    context.Context,
    TeamAttemptRebindInput,
) (TeamExecutionRecord, error)
```

The exact new Team Event is `TeamNodeAttemptRebound`. The Authority reads the
Team, WorkItem, Run, Runtime status, and Runtime capacity streams as one related
set. It accepts only the current dispatched attempt, the same deterministic
WorkItem/Run identity, an accepted claimed Run at the exact higher generation,
and an expired previous prepare lease. Exact retry is idempotent; any stale or
different binding conflicts. Projection applies the complete binding and fences
the old generation.

The exact Event payload is:

```json
{
  "team_instance_id": "...",
  "plan_digest": "...",
  "logical_node_id": "...",
  "attempt_number": 1,
  "work_item_id": "...",
  "run_id": "...",
  "previous_claim_id": "...",
  "previous_claim_generation": 1,
  "claim_id": "...",
  "claim_generation": 2,
  "runtime_instance_id": "...",
  "agent_instance_id": "...",
  "run_stream": "run/...",
  "run_sequence": 2,
  "run_event_id": "..."
}
```

The Event ID is the deterministic hash ID over Event type, Team instance, plan
digest, logical node, attempt number, WorkItem, Run, previous Claim/generation,
new Claim/generation, Runtime, Agent, and accepted new Run Event ID. Its
causation is the previous Team stream Event ID. The payload Run reference must
equal the current Run stream head and must identify the accepted higher-
generation `RunClaimed` Event.

Replay and Projection accept the transition only when the current logical node
and attempt are `running` / `dispatched`, the complete previous binding matches,
the new Claim ID differs, the generation is exactly previous plus one, the
WorkItem/Run/Runtime/Agent do not change, and the Run reference is non-empty.
They replace only Claim ID/generation, retain attempt status `dispatched`, clear
no terminal/Evidence fields, and fence any later old-generation terminal.

`replayTeamExecution` validates the complete WorkItem/Run/Claim/generation/
Runtime/Agent binding for rebound and terminal Events. Evidence commit replays
the bounded accepted Run/runtime/capacity lifecycle; scanning for a
terminal-shaped Event is forbidden.

## Read-view and coordinator recovery

`GlobalReadView` adds:

```go
func (GlobalReadView) LatestAgentGrantForRun(string) (AgentGrant, bool)
```

It deterministically returns the highest-sequence Grant record for the Run,
active or revoked, as a record-local copy and never exposes token material.

`TeamCoordinator.Run` processes recovery before planning new nodes:

1. terminal Run + receipt: complete missing Evidence metadata;
2. terminal Run + complete capture: finalize/republish, then commit metadata;
3. dispatch-committed claimed Run + absent capture is valid only when no Grant
   was ever issued for that exact binding. Recovery first begins the old binding
   as an empty capture. Any absent capture with a Grant, running Run, terminal
   Run, authorization Event, or later-generation Run conflicts and performs no
   execution;
4. claimed Run before lease expiry: after the bounded absent-capture repair in
   item 3, return incomplete without Journal, Grant, Run, Team, or execution
   mutation;
5. claimed Run after lease expiry: inspect the latest Grant for the exact old
   binding. If active, revoke it with the accepted `operator` reason. If
   it is already revoked with that exact binding and reason, continue
   idempotently. If no Grant was ever issued, continue only when the capture has
   zero Frames. Any different/latest binding conflicts. Reclaim the same Run
   through the accepted Work Authority, rebind the empty filesystem capture to
   generation N+1, append the Team rebound Journal Event, rebuild the caller's
   dispatch at generation N+1, issue a new Grant, and execute once;
6. running Run without terminal: return
   `ErrTeamAttemptRecoveryRequired`, preserving state unchanged.

The cross-store order is exact and intentionally not described as atomic:

```text
Run reclaim Journal CAS
-> empty capture filesystem rebind
-> Team rebound Journal CAS
-> new Grant issue
-> execution
```

If restart occurs after higher-generation `RunClaimed` but before capture
rebind, the Coordinator rebinds the old empty capture and then appends Team
rebound. If restart occurs after capture rebind but before Team rebound, exact
capture rebind is an idempotent no-op and the Coordinator appends the missing
Team rebound. If Run and capture are at N+1 while Team is at N, this is the
single accepted split state. If Team is at N+1 while capture remains at N, or
any surface is ahead by more than one generation or differs in any binding,
recovery conflicts without Grant issue or execution.

The Coordinator never invents retry/fallback/degraded policy, retries a CAS
conflict, resumes an unknown process, or creates a second Run for the same
logical attempt.

For a new dispatch, the exact side-effect order is: atomic Journal dispatch,
durable `BeginAttemptCapture` for every dispatched attempt, Grant issue, then
managed execution. No Grant is issued and no executor is invoked if capture
begin fails. This makes the only legitimate absent-capture state the explicitly
recoverable crash window immediately after dispatch and before capture begin.

## Canonical Evidence replacement

The in-memory `canonicalTeamEvidence` path is replaced by the durable capture.
The canonical artifact contains:

- deterministic attempt binding;
- canonical authorized Frame lines;
- accepted Run terminal status and reason.

It does not include raw Grant, credentials, ambient environment, hidden
reasoning, Go error strings, or unstable process diagnostics. Live deltas remain
tentative; only accepted terminal plus Evidence metadata are authoritative.

## Required evidence

- exact capture begin/append/rebind/finalize/reopen and filesystem hardening;
- Pi/Supervisor authorized-output reconciliation into capture;
- concurrent SubAgent capture overlap without shared bytes;
- crash injection at every Amendment 5 boundary;
- expired claimed generation reclaim with old Grant revocation;
- stale old generation output/Evidence/terminal rejection;
- indeterminate running state returns the typed human-action error unchanged;
- no Frame bytes in Journal/Projection;
- exact one Run identity, one receipt digest, one Evidence Event, and one Team
  terminal Event after repeated reopen.

VERDICT: FROZEN
