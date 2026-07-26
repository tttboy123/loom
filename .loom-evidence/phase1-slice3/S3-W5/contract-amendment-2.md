# S3-W5 Contract Amendment 2 — Authorized Output and Attempt Recovery

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Parent:
  `.loom-evidence/phase1-slice3/S3-W5/contract-amendment-1.md`
- Authority:
  `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-3.md`
- Decision:
  `docs/adr/0009-authorized-node-output-and-attempt-recovery.md`

This amendment preserves the parent contract and Contract Amendment 1 except
where it replaces the one-Run-per-node and batch-only output surfaces below.
No product edit begins until this amendment receives a fresh independent
review.

## Added owned files

The exact owned scope additionally includes:

- `internal/supervisor/managed_execution.go`
- `internal/supervisor/managed_execution_test.go`
- `internal/runtime/piadapter/execution_adapter.go`
- `internal/runtime/piadapter/execution_adapter_test.go`
- `internal/projection/team_execution.go`
- `internal/projection/team_execution_test.go`
- `docs/adr/0009-authorized-node-output-and-attempt-recovery.md`
- `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-3.md`
- `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-3-REVIEW-1.md`
- `.loom-evidence/phase1-slice3/S3-W5/contract-amendment-2.md`
- its review and later S3-W5 evidence

No other accepted S3-W4 file is reopened.

## Exact Supervisor and Adapter API

Package `internal/supervisor` adds only:

```go
var ErrAuthorizedFrameObserver = errors.New("authorized Frame observer failed")

type FrameSink interface {
    AcceptFrame(context.Context, bridgev1.Frame) error
}

type AuthorizedFrame

func (AuthorizedFrame) Frame() bridgev1.Frame
func (AuthorizedFrame) Binding() bridgev1.RunStreamBinding
func (AuthorizedFrame) Tentative() bool

type AuthorizedFrameObserver interface {
    ObserveAuthorizedFrame(context.Context, AuthorizedFrame) error
}
```

The existing structs gain exactly these fields:

```go
type AdapterRequest struct {
    // accepted existing fields
    FrameSink FrameSink
}

type ExecuteInput struct {
    // accepted existing fields
    FrameObserver AuthorizedFrameObserver
}
```

No other S3-W4 exported symbol changes.

Supervisor always supplies a non-nil private sink. The production Pi Adapter
must call `AcceptFrame` exactly once, synchronously, after exact line decode and
before appending the Frame to its terminal batch. A returned sink error stops
the child through the accepted bounded process-group cleanup path.

For every Frame the sink:

1. rejects raw Grant bytes and invalid type/order/ack/result semantics;
2. obtains an immutable candidate from `AdvanceBoundRunStream`;
3. validates any ack/result payload required by the managed session;
4. calls `AgentGrant.Authorize` with the exact Frame message ID and operation;
5. commits the candidate stream only after authorization succeeds;
6. stores one deep copy for terminal reconciliation; and
7. invokes the optional observer with a second immutable copy.

An observer error returns `ErrAuthorizedFrameObserver`, aborts execution, and
does not turn the callback into terminal authority. The observer may have
received the Frame it rejects; no later Frame is delivered.

After Adapter return, Supervisor requires exact length, order, immutable Frame
content, ack/result flags, and terminal result agreement between its accepted
sink stream and `AdapterResult`. It does not authorize or observe the batch a
second time. Missing sink calls or batch divergence is `ErrBridgeSession`.

Nil observer is valid; validation, authorization, buffering, and reconciliation
still occur incrementally. Nil/typed-nil sink or observer misuse fails closed.

## Exact logical-node plan API replacement

Contract Amendment 1's `ExecutionNodeInput`, `ExecutionNode`,
`ExecutionNodeState`, and `ReadyExecutionNodes` signatures are replaced by:

```go
type ExecutionNodeInput struct {
    LogicalNodeID string
    Title string
    AgentInstanceID string
    RuntimeInstanceID string
    Role ExecutionRole
    DependsOn []string
    MaxAttempts int
}

type ExecutionNode

func (ExecutionNode) LogicalNodeID() string
func (ExecutionNode) Title() string
func (ExecutionNode) AgentInstanceID() string
func (ExecutionNode) RuntimeInstanceID() string
func (ExecutionNode) Role() ExecutionRole
func (ExecutionNode) DependsOn() []string
func (ExecutionNode) MaxAttempts() int

type ExecutionNodeState struct {
    LogicalNodeID string
    Status string
    CurrentAttempt int
    RetryAt time.Time
}

func ReadyExecutionNodes(
    ExecutionPlan,
    []ExecutionNodeState,
    []RuntimeCapacityState,
    time.Time,
) ([]ExecutionNode, error)
```

`ExecutionPlanInput`, `ExecutionPlan`, roles, capacity state, build errors,
plan getters, digest, graph validation, and maximum three logical nodes remain
as frozen. `MaxAttempts` is 1–3. Every supplied time is nonzero UTC.

## Exact Work/Team attempt API replacement

Package `internal/work` additionally defines:

```go
var (
    ErrInvalidTeamAttempt = errors.New("invalid Team node attempt")
    ErrTeamAttemptLimit = errors.New("Team node attempt limit reached")
    ErrInvalidTeamRecovery = errors.New("invalid Team node recovery")
)

type TeamAttemptSelection struct {
    LogicalNodeID string
    AttemptNumber int
}
```

`TeamDispatchInput` is replaced by:

```go
type TeamDispatchInput struct {
    Plan teams.ExecutionPlan
    ReadyAttempts []TeamAttemptSelection
    ViewVersion string
    ExpectedHeads []journal.StreamHead
    AuthoritativeTime time.Time
    PrepareLeaseDuration time.Duration
    CorrelationID string
}
```

`TeamDispatchedNode.Node()` is replaced by:

```go
func (TeamDispatchedNode) LogicalNode() teams.ExecutionNode
func (TeamDispatchedNode) Attempt() TeamAttemptRecord
func (TeamDispatchedNode) WorkItem() WorkItemRecord
func (TeamDispatchedNode) Run() RunRecord
```

Attempt identities are deterministic functions of Team instance ID, plan
digest, logical node ID, and attempt number:

- `WorkItemID`: `team-work-` plus the first 32 lowercase SHA-256 hex digits;
- `RunID`: `team-run-` plus the first 32 lowercase SHA-256 hex digits;
- `EvidenceID`: `team-evidence-` plus the first 32 lowercase SHA-256 hex
  digits.

The exact input label (`work`, `run`, or `evidence`) is included in the digest,
so identities cannot alias.

Contract Amendment 1's `TeamNodeEvidenceInput`,
`CommitTeamNodeEvidence`, and `TeamNodeRecord` attempt-specific getters are
replaced by:

```go
type TeamAttemptEvidenceInput struct {
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
    EvidenceID string
    EvidenceDigest string
    CorrelationID string
}

func (*Authority) CommitTeamAttemptEvidence(
    context.Context,
    TeamAttemptEvidenceInput,
) (TeamExecutionRecord, error)

type TeamRecoveryAction string

const (
    TeamRecoveryRetry TeamRecoveryAction = "retry"
    TeamRecoveryFallback TeamRecoveryAction = "fallback"
    TeamRecoveryDegraded TeamRecoveryAction = "degraded"
    TeamRecoveryBlocked TeamRecoveryAction = "blocked"
    TeamRecoveryHumanRequired TeamRecoveryAction = "human_required"
)

type TeamRecoveryInput struct {
    TeamInstanceID string
    PlanDigest string
    LogicalNodeID string
    AttemptNumber int
    Action TeamRecoveryAction
    RetryAt time.Time
    NextAgentInstanceID string
    NextRuntimeInstanceID string
    DependencySatisfied bool
    CorrelationID string
}

func (*Authority) ScheduleTeamNodeRecovery(
    context.Context,
    TeamRecoveryInput,
) (TeamExecutionRecord, error)

type TeamNodeRecord
type TeamAttemptRecord

func (TeamNodeRecord) LogicalNodeID() string
func (TeamNodeRecord) Status() string
func (TeamNodeRecord) DependencySatisfied() bool
func (TeamNodeRecord) CurrentAttempt() int
func (TeamNodeRecord) RetryAt() time.Time
func (TeamNodeRecord) Attempts() []TeamAttemptRecord
func (TeamAttemptRecord) AttemptNumber() int
func (TeamAttemptRecord) WorkItemID() string
func (TeamAttemptRecord) RunID() string
func (TeamAttemptRecord) ClaimID() string
func (TeamAttemptRecord) ClaimGeneration() int64
func (TeamAttemptRecord) RuntimeInstanceID() string
func (TeamAttemptRecord) AgentInstanceID() string
func (TeamAttemptRecord) Status() string
func (TeamAttemptRecord) EvidenceID() string
func (TeamAttemptRecord) EvidenceDigest() string
```

Initial attempt one is scheduled by `TeamExecutionPlanned`. Retry/fallback may
schedule only `current_attempt+1`, never above the plan's `MaxAttempts`, and
requires nonzero UTC `RetryAt`. It binds the explicit next Agent/Runtime.
Degraded/blocked/human-required actions schedule no attempt and require zero
RetryAt and empty next binding.

The Authority validates but never invents the supplied Action or
`DependencySatisfied`. Retry/fallback must set `DependencySatisfied=false`.
Blocked/human-required must set it false. Degraded may use the explicit value
selected by later policy.

## Explicit state and Event model

Logical node statuses are exactly:

```text
pending
running
awaiting_recovery
retry_scheduled
fallback_scheduled
degraded
blocked
human_required
succeeded
failed
cancelled
```

Attempt statuses are exactly:

```text
scheduled
dispatched
running
succeeded
failed
cancelled
```

Team statuses are exactly:

```text
pending
running
awaiting_recovery
succeeded
failed
degraded
blocked
human_required
cancelled
```

Contract Amendment 1's Team Event list is replaced by:

- `WorkRunIdentityReserved`
- `WorkRunIdentityIndexInitialized`
- `AgentGrantIdentityReserved`
- `AgentGrantIdentityIndexInitialized`
- `TeamExecutionPlanned`
- `TeamNodeAttemptScheduled`
- `TeamReadySetDispatched`
- accepted existing `RunStarted`
- `TeamNodeRecoveryRecorded`
- `EvidenceSubmitted`
- `TeamNodeAttemptTerminal`
- `TeamExecutionTerminal`

Output Frame payload/delta bytes are never Journal Events. Authorization Events
contain only accepted Grant decision metadata, not Frame payload.

`CommitTeamAttemptEvidence` validates an accepted terminal Run at the exact
attempt generation, publishes no bytes itself, and atomically commits
`EvidenceSubmitted` plus `TeamNodeAttemptTerminal`. A failed/cancelled attempt
below `MaxAttempts` makes the logical node `awaiting_recovery`; no retry is
scheduled until the explicit recovery command. Success satisfies dependencies.

Recovery actions are generation- and attempt-bound exact-once facts.
Retry/fallback appends `TeamNodeRecoveryRecorded` plus the next
`TeamNodeAttemptScheduled` atomically. Terminal recovery actions append the
recovery fact and, when the Team becomes terminal, one
`TeamExecutionTerminal`.

## Projection replacement

Contract Amendment 1's `TeamExecution` and `TeamExecutionNode` are replaced by:

```go
type TeamExecution struct {
    TeamInstanceID string
    PlanDigest string
    Status string
    Nodes []TeamExecutionNode
}

type TeamExecutionNode struct {
    LogicalNodeID string
    Status string
    DependencySatisfied bool
    CurrentAttempt int
    RetryAt time.Time
    Attempts []TeamExecutionAttempt
}

type TeamExecutionAttempt struct {
    AttemptNumber int
    WorkItemID string
    RunID string
    ClaimID string
    ClaimGeneration int64
    RuntimeInstanceID string
    AgentInstanceID string
    Status string
    EvidenceID string
    EvidenceDigest string
}
```

GlobalReadView's `TeamExecution` accessor remains record-local and deep-copies
nodes and attempts. Projection rebuild is full and failure preserves the old
Snapshot and view.

## Exact application observation and execution replacement

Package `internal/app` adds:

```go
type NodeOutput

func (NodeOutput) LogicalNodeID() string
func (NodeOutput) AttemptNumber() int
func (NodeOutput) AuthorizedFrame() supervisor.AuthorizedFrame
func (NodeOutput) Tentative() bool

type NodeOutputObserver interface {
    ObserveNodeOutput(context.Context, NodeOutput) error
}
```

`TeamNodeExecution` and `TeamExecutionRequest` are replaced by:

```go
type TeamNodeExecution struct {
    LogicalNodeID string
    AttemptNumber int
    SourcePath string
    Profile runtime.RuntimeProfile
    Instance runtime.RuntimeInstance
    Dispatch bridgev1.Frame
    Executor ManagedNodeExecutor
}

type TeamExecutionRequest struct {
    Plan teams.ExecutionPlan
    Nodes []TeamNodeExecution
    AuthoritativeTime time.Time
    PrepareLeaseDuration time.Duration
    GrantLifetime time.Duration
    CorrelationID string
    OutputObserver NodeOutputObserver
}
```

`NewTeamCoordinator`, `Run`, `TeamExecutionResult`, and result getters remain
as frozen. `Run` executes only attempts explicitly present in the request and
ready at `AuthoritativeTime`. It never sleeps until `RetryAt`, infers a recovery
action, or retries a CAS conflict.

For each attempt, the Coordinator wraps the request observer with logical node
and attempt identity, collects the same bounded authorized Frames for canonical
Evidence, and passes the wrapper to Supervisor. Forwarded output is tentative.
Observer errors fail that attempt through the managed terminal path.

After terminal Run commit, the Coordinator serializes a canonical Evidence
artifact containing attempt binding, authorized Frames, managed outcome, and
error classification; publishes it to the accepted Artifact Store; then calls
`CommitTeamAttemptEvidence`. No raw Grant, credential, ambient environment, or
hidden Loom reasoning is included.

## Recovery, canary, and RED additions

Mandatory RED and the controlled canary additionally prove:

1. Pi calls the sink before terminal Adapter return and exactly once per Frame.
2. Supervisor observes only after candidate stream validation and successful
   Grant authorization.
3. stale generation, revoked/expired Grant, invalid sequence/type/payload,
   duplicate/post-result Frame, missing sink delivery, batch divergence, and
   raw-token payload are never forwarded.
4. observer error stops/reaps the fixture and produces terminal failure without
   later output.
5. two SubAgent observers receive overlapping tentative deltas while the final
   batch and Evidence reconcile exactly.
6. attempt IDs are deterministic, distinct across attempts, and stable after
   reopen.
7. explicit retry/fallback before `RetryAt` is not ready; at exact UTC
   `RetryAt`, two schedulers produce one dispatch and one conflict.
8. attempts stop at three; no scheduler sleep, hidden retry, or automatic
   recovery decision exists.
9. every attempt has independent Run/generation/Grant/Evidence lineage, and old
   attempt output/Evidence/terminal is fenced.
10. restart between attempt terminal, recovery scheduling, dispatch, Artifact
    publish, and Evidence metadata commit recovers without duplication.
11. output delta bytes do not appear in Team lifecycle Events or Projection
    state; only the content-addressed Evidence artifact contains the captured
    output.

Focused verification expands to:

```text
go test ./internal/supervisor ./internal/runtime/piadapter \
  ./internal/teams ./internal/work ./internal/authorization \
  ./internal/projection ./internal/app -count=1
go test -race ./internal/supervisor ./internal/runtime/piadapter \
  ./internal/teams ./internal/work ./internal/authorization \
  ./internal/projection ./internal/app -count=10
```

The parent full repository, full race, vet, format, scope, security, Evidence,
controlled canary, fresh implementation review, and atomic local commit gates
remain required.

## Deferred queue remains closed

S3-W5 does not implement:

- output-contract classification;
- recovery-policy choice;
- Provider/model or data-source fallback policy;
- API/SSE/WebSocket/daemon/CLI delivery;
- cursor/Last-Event-ID reconnect;
- slow-consumer coalescing or `stream_gap`;
- Web/TUI.

Those remain the reviewed Slice 4 and Slice 5 contract inputs from Exit
Amendment 3.

VERDICT: FROZEN
