# S3-W5 Contract Amendment 1 — Run Identity and Exact Authority API

- Date: `2026-07-26`
- Baseline: `cd1e594`
- Parent: `.loom-evidence/phase1-slice3/S3-W5/contract.md`
- Repairs: `.loom-evidence/phase1-slice3/S3-W5/contract-review-1.md`

This amendment preserves the parent contract except where it replaces the two
P1 findings with the exact rules and exported surface below.

## Global Run identity

The accepted Work/Run Authority owns one additional Journal stream:

```text
work-run-identity/v1
```

Every new `WorkItemAssigned` that introduces a Run must commit exactly one
`WorkRunIdentityReserved` Event in the same
`AppendBatchIfStreamHeads` transaction. The reservation binds Run ID,
WorkItem ID, AgentInstance ID, assignment stream/sequence, and assignment Event
ID. A Run ID already reserved for a different immutable binding returns
`ErrRunAuthorityConflict`.

Before normal create/assign or Team dispatch commands are enabled against a
pre-S3-W5 Journal, `InitializeRunIdentityIndex` explicitly:

- performs the separately governed full compatibility replay;
- validates historical WorkItem/Run facts;
- appends deterministic missing reservations in batches of at most 15 distinct
  WorkItem streams plus the identity stream;
- appends a final initialization marker bound to the canonical digest of all
  indexed historical `WorkItemAssigned` Event IDs;
- resumes committed partial batches idempotently; and
- returns any conflict without automatic retry.

After initialization, create/assign touches its WorkItem stream plus
`work-run-identity/v1`; Team dispatch also includes that identity head. No
normal Run/Work write command calls `ReadAll`.

The maximum three-node Team dispatch now touches at most 14 heads: Team
execution, global Run identity, and up to three each of WorkItem, Run, Runtime
status, and Runtime capacity. This remains below the reviewed limit 16.

## Exact public API

No authority-bearing exported symbol outside this list may be added. Immutable
record types expose only the listed getters; mutable slices are copied.

### `internal/journal`

The parent contract's exact `StreamHead`, `StreamSetSnapshot`, and
`ReadStreamSet` API is unchanged.

### `internal/teams`

```go
var (
    ErrInvalidExecutionPlan = errors.New("invalid Team execution plan")
    ErrExecutionDependencyCycle = errors.New("Team execution dependency cycle")
    ErrInvalidExecutionState = errors.New("invalid Team execution state")
)

type ExecutionRole string

const (
    ExecutionRoleMain ExecutionRole = "main"
    ExecutionRoleSubAgent ExecutionRole = "subagent"
)

type ExecutionNodeInput struct {
    NodeID string
    WorkItemID string
    Title string
    RunID string
    AgentInstanceID string
    RuntimeInstanceID string
    Role ExecutionRole
    DependsOn []string
}

type ExecutionPlanInput struct {
    TeamInstanceID string
    Nodes []ExecutionNodeInput
}

type ExecutionNode
type ExecutionPlan

func BuildExecutionPlan(ExecutionPlanInput) (ExecutionPlan, error)
func (ExecutionPlan) TeamInstanceID() string
func (ExecutionPlan) Nodes() []ExecutionNode
func (ExecutionPlan) Digest() string
func (ExecutionNode) NodeID() string
func (ExecutionNode) WorkItemID() string
func (ExecutionNode) Title() string
func (ExecutionNode) RunID() string
func (ExecutionNode) AgentInstanceID() string
func (ExecutionNode) RuntimeInstanceID() string
func (ExecutionNode) Role() ExecutionRole
func (ExecutionNode) DependsOn() []string

type ExecutionNodeState struct {
    NodeID string
    Status string
}

type RuntimeCapacityState struct {
    RuntimeInstanceID string
    Capacity int
    Active int
}

func ReadyExecutionNodes(
    ExecutionPlan,
    []ExecutionNodeState,
    []RuntimeCapacityState,
) ([]ExecutionNode, error)
```

Accepted node statuses are `pending`, `claimed`, `running`, `succeeded`,
`failed`, and `cancelled`.

### `internal/work`

```go
var (
    ErrRunIdentityIndexRequired = errors.New("Run identity index required")
    ErrInvalidTeamExecution = errors.New("invalid Team execution")
    ErrTeamExecutionConflict = errors.New("Team execution conflict")
    ErrStaleGlobalReadView = errors.New("stale global read view")
    ErrTeamExecutionAlreadyTerminal = errors.New("Team execution already terminal")
)

func (*Authority) InitializeRunIdentityIndex(context.Context) error

type TeamDispatchInput struct {
    Plan teams.ExecutionPlan
    ReadyNodeIDs []string
    ViewVersion string
    ExpectedHeads []journal.StreamHead
    PrepareLeaseDuration time.Duration
    CorrelationID string
}

type TeamDispatchedNode
type TeamDispatchResult

func (*Authority) DispatchTeamReadySet(
    context.Context,
    TeamDispatchInput,
) (TeamDispatchResult, error)

func (TeamDispatchResult) TeamInstanceID() string
func (TeamDispatchResult) PlanDigest() string
func (TeamDispatchResult) ViewVersion() string
func (TeamDispatchResult) Nodes() []TeamDispatchedNode
func (TeamDispatchedNode) Node() teams.ExecutionNode
func (TeamDispatchedNode) WorkItem() WorkItemRecord
func (TeamDispatchedNode) Run() RunRecord

type TeamNodeEvidenceInput struct {
    TeamInstanceID string
    PlanDigest string
    NodeID string
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

type TeamExecutionRecord
type TeamNodeRecord

func (*Authority) CommitTeamNodeEvidence(
    context.Context,
    TeamNodeEvidenceInput,
) (TeamExecutionRecord, error)

func (*Authority) TeamExecution(
    context.Context,
    string,
) (TeamExecutionRecord, error)

func (TeamExecutionRecord) TeamInstanceID() string
func (TeamExecutionRecord) PlanDigest() string
func (TeamExecutionRecord) Status() string
func (TeamExecutionRecord) Nodes() []TeamNodeRecord
func (TeamNodeRecord) NodeID() string
func (TeamNodeRecord) WorkItemID() string
func (TeamNodeRecord) RunID() string
func (TeamNodeRecord) ClaimID() string
func (TeamNodeRecord) ClaimGeneration() int64
func (TeamNodeRecord) RuntimeInstanceID() string
func (TeamNodeRecord) AgentInstanceID() string
func (TeamNodeRecord) Status() string
func (TeamNodeRecord) EvidenceID() string
func (TeamNodeRecord) EvidenceDigest() string
```

`InitializeRunIdentityIndex` and diagnostic `Snapshot` are the only Work
Authority methods permitted to full-replay. All other listed methods use
`ReadStreamSet`.

### `internal/authorization`

```go
var ErrGrantIdentityIndexRequired = errors.New("Grant identity index required")

func (*Authority) InitializeGrantIdentityIndex(context.Context) error
```

Existing accepted `NewAuthority`, `Issue`, `Authorize`, `Revoke`, `Snapshot`,
Token, IssuedGrant, GrantRecord, inputs, operations, and errors remain the
public Grant surface. No other export is added.

`InitializeGrantIdentityIndex` and diagnostic `Snapshot` are the only Grant
Authority methods permitted to full-replay. Normal Grant commands require the
initialization marker and use `ReadStreamSet`.

### `internal/projection`

```go
type TeamExecution struct {
    TeamInstanceID string
    PlanDigest string
    Status string
    Nodes []TeamExecutionNode
}

type TeamExecutionNode struct {
    NodeID string
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

type GlobalReadView

func (*Projection) GlobalReadView() GlobalReadView
func (GlobalReadView) Version() string
func (GlobalReadView) Head(string) (journal.StreamHead, bool)
func (GlobalReadView) WorkItem(string) (WorkItem, bool)
func (GlobalReadView) Run(string) (Run, bool)
func (GlobalReadView) AgentGrant(string) (AgentGrant, bool)
func (GlobalReadView) Evidence(string) (Evidence, bool)
func (GlobalReadView) Team(string) (TeamInstance, bool)
func (GlobalReadView) AgentInstance(string) (AgentInstance, bool)
func (GlobalReadView) RuntimeInstance(string) (RuntimeInstance, bool)
func (GlobalReadView) TeamExecution(string) (TeamExecution, bool)
func (GlobalReadView) ActiveRunCount(string) int
```

No map or full-snapshot getter is added to GlobalReadView.

### `internal/app`

```go
var (
    ErrInvalidTeamCoordinator = errors.New("invalid Team coordinator")
    ErrTeamExecutionIncomplete = errors.New("Team execution incomplete")
)

type ManagedNodeExecutor interface {
    Execute(
        context.Context,
        supervisor.ExecuteInput,
    ) (supervisor.Outcome, error)
}

type TeamNodeExecution struct {
    NodeID string
    SourcePath string
    Profile runtime.RuntimeProfile
    Instance runtime.RuntimeInstance
    Dispatch bridgev1.Frame
    Executor ManagedNodeExecutor
}

type TeamExecutionRequest struct {
    Plan teams.ExecutionPlan
    Nodes []TeamNodeExecution
    PrepareLeaseDuration time.Duration
    GrantLifetime time.Duration
    CorrelationID string
}

type TeamExecutionResult
type TeamCoordinator

func NewTeamCoordinator(
    *work.Authority,
    *authorization.Authority,
    *projection.Projection,
    *evidence.Store,
) (*TeamCoordinator, error)

func (*TeamCoordinator) Run(
    context.Context,
    TeamExecutionRequest,
) (TeamExecutionResult, error)

func (TeamExecutionResult) Team() work.TeamExecutionRecord
func (TeamExecutionResult) ExecutedNodeIDs() []string
```

`Run` performs the parent contract's bounded dependency waves and restart
recovery. It has no exported scheduler, writer, trigger, loop, or retry API.

## Event surface

The complete new S3-W5 Event set is exactly:

- `WorkRunIdentityReserved`
- `WorkRunIdentityIndexInitialized`
- `AgentGrantIdentityReserved`
- `AgentGrantIdentityIndexInitialized`
- `TeamExecutionPlanned`
- `TeamReadySetDispatched`
- `EvidenceSubmitted`
- `TeamNodeEvidenceCommitted`
- `TeamExecutionTerminal`

Every payload uses exact decoding with unknown and duplicate fields rejected.
Identity reservations are same-transaction facts, not Projection authority.

## Additional RED and acceptance

Mandatory RED must also prove:

- two different WorkItems cannot reserve one Run ID;
- legacy Run identity initialization is deterministic, resumable, and required;
- normal create/assign and Team dispatch never call `ReadAll`;
- three-node dispatch including the Run identity head remains within 16; and
- compile-time API assertions expose exactly the listed authority surface.

All parent acceptance and checks remain required.

VERDICT: FROZEN
