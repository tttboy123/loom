# S4-W2 Contract — Output Contract and Bounded Recovery Integration

Status: FROZEN — Contract Repair Review 2 and Amendment 2A Review 1 PASS.

- WorkItem: `S4-W2`
- Risk: `HIGH`
- Baseline: `87ea092`
- Date: `2026-07-26`
- Depends on: accepted S4-W1 and frozen Slice 4 Exit Contract
- Required amendment: `../EXIT-CONTRACT-AMENDMENT-2.md`
- Ownership repair: `../EXIT-CONTRACT-AMENDMENT-2A.md`
- Capability: exact authorized-output classification and bounded
  Journal-authoritative Team recovery

S4-W2 is one vertical semantic recovery boundary. Evidence summary, output
classifier, recovery policy, Team attempt writer, scheduler integration, and
projection are internal parts of this Candidate. No classifier-only,
policy-only, writer-only, scheduler-only, projection-only, retry-wrapper, or
coordinator WorkItem may follow.

## Owned files

New product and tests:

- `internal/verification/output_contract.go`
- `internal/verification/output_contract_test.go`
- `internal/rules/recovery_policy.go`
- `internal/rules/recovery_policy_test.go`

Accepted files reopened only by reviewed Exit Contract Amendment 2:

- `internal/evidence/attempt_capture.go`
- `internal/evidence/attempt_capture_test.go`
- `internal/evidence/attempt_capture_windows.go`
- `internal/work/team_execution_authority.go`
- `internal/work/team_execution_authority_test.go`
- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`
- `internal/projection/team_execution.go`
- `internal/projection/team_execution_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`

Governance:

- `.loom-evidence/phase1-slice4/EXIT-CONTRACT-AMENDMENT-2.md`
- `.loom-evidence/phase1-slice4/EXIT-CONTRACT-AMENDMENT-2-REVIEW-*.md`
- `.loom-evidence/phase1-slice4/EXIT-CONTRACT-AMENDMENT-2A*.md`
- `.loom-evidence/phase1-slice4/S4-W2/**`
- Controller-owned S4-W2 hunk in `docs/CURRENT.md`

No other file is owned. `AGENTS.md`, `PROGRESS.md`, `.codex/**`,
`.loom-drafts/**`, and the post-S3 scratch queue remain untouched/unstaged.

## Bounded value rules

All opaque IDs, workflow fallback keys, and actor/approval references are
trimmed valid UTF-8 without control characters and at most 128 bytes. Digests
are lowercase SHA-256 hex. Times are nonzero canonical UTC.

- one Team has 1–3 logical nodes;
- attempts are numbered 1–3 and never exceed the ExecutionPlan node ceiling;
- an output summary inherits at most 1,024 accepted Frames and 1 MiB durable
  attempt-capture bytes;
- retry delay is zero through 24 hours;
- retry/fallback attempt credits are 0–2;
- prior classifications are bounded to the two earlier attempts;
- one policy has at most one explicit workflow fallback path; and
- one coordinator invocation performs at most nine state-changing passes,
  never recursion, sleep, polling, or an unbounded loop.

Canonical digests use schema-tagged length-prefixed fields, sorted records, and
exact UTC timestamp strings. They never use delimiter concatenation or raw
Go/JSON map iteration.

## Evidence output summary

The accepted Evidence Store extends `AttemptReceipt` with an immutable
`AttemptOutputSummary`. Exact public names may be reduced during minimal
implementation:

```go
type AttemptOutputSummary struct

func (AttemptReceipt) OutputSummary() AttemptOutputSummary
func (AttemptOutputSummary) EvidenceID() string
func (AttemptOutputSummary) EvidenceDigest() string
func (AttemptOutputSummary) AuthorizedFrameCount() int
func (AttemptOutputSummary) OutputFrameCount() int
func (AttemptOutputSummary) OutputPayloadBytes() int
func (AttemptOutputSummary) ResultObserved() bool
func (AttemptOutputSummary) TerminalStatus() string
func (AttemptOutputSummary) Digest() string
```

The Store computes the summary only while finalizing/reopening the exact
authorized attempt capture. `event` and `evidence` Frames are output-bearing;
heartbeat, ack, cancel, and terminal `result` Frames are not output content.
The summary stores counts and digests only. Raw payload, canonical Frame line,
terminal reason, Grant/token, credential, prompt, or hidden reasoning is not
returned.

The summary digest binds the complete attempt capture identity, Evidence
digest, Frame/output counts, output payload-byte count, result observation, and
terminal status. Finalize exact retry and reopened receipt return exactly the
same summary. Artifact, receipt, or summary disagreement is
`ErrAttemptCaptureConflict` and does not rewrite accepted content.

`AttemptReceipt` and `AttemptOutputSummary` keep all fields unexported and have
no public constructor. Only successful Evidence Store finalize/reopen returns
a nonzero valid value. Work authority accepts this receipt object directly and
derives Evidence ID, Evidence digest, and summary from its accessors.
Caller-supplied Evidence or summary identity strings are removed from the
commit authority boundary.

## Pure output contract

Package `internal/verification` defines:

```go
var (
    ErrInvalidOutputContract = errors.New("invalid output contract")
    ErrInvalidOutputObservation = errors.New("invalid output observation")
)

type OutputClassification string

const (
    OutputValidNonEmpty OutputClassification = "valid_nonempty"
    OutputValidEmpty OutputClassification = "valid_empty"
    OutputTransientEmpty OutputClassification = "transient_empty"
    OutputInvalid OutputClassification = "invalid"
)

type EmptyOutputPolicy string

const (
    EmptyOutputValid EmptyOutputPolicy = "valid"
    EmptyOutputTransient EmptyOutputPolicy = "transient"
    EmptyOutputInvalid EmptyOutputPolicy = "invalid"
)

type OutputContract struct
type OutputObservation struct
type Classification struct

func NewOutputContract(version int, empty EmptyOutputPolicy) (OutputContract, error)
func NewOutputObservation(
    evidenceID string,
    evidenceDigest string,
    summaryDigest string,
    authorizedFrameCount int,
    outputFrameCount int,
    outputPayloadBytes int,
    resultObserved bool,
    terminalStatus string,
) (OutputObservation, error)
func Classify(OutputContract, OutputObservation) (Classification, error)
```

Inputs and outputs are immutable. Accessors return values. Classification
digest binds exact OutputContract version/digest and complete observation
identity/digest/counts/status.

Classification is:

- `invalid` when the exact result is missing, terminal status is not
  `succeeded`, counts are inconsistent/out of bounds, or observation binding
  is invalid;
- `valid_nonempty` when a succeeded exact result has at least one authorized
  output-bearing Frame and positive payload bytes;
- otherwise `valid_empty`, `transient_empty`, or `invalid` exactly according
  to the immutable EmptyOutputPolicy.

`valid_empty` is therefore accepted only when the WorkItem OutputContract
explicitly allows it. `transient_empty` is a semantic fact, not retry
authority. The classifier is pure and imports no Journal, Projection,
Runtime, Provider, scheduler, or credential package.

## Pure bounded recovery policy

Package `internal/rules` adds immutable recovery values in
`recovery_policy.go`; it does not expand the S4-W1 Rule/Approval authority.

```go
var (
    ErrInvalidRecoveryPolicy = errors.New("invalid recovery policy")
    ErrInvalidRecoveryInput = errors.New("invalid recovery input")
)

type RecoveryAction string

const (
    RecoveryNone RecoveryAction = "none"
    RecoveryRetry RecoveryAction = "retry"
    RecoveryFallback RecoveryAction = "fallback"
    RecoveryDegraded RecoveryAction = "degraded"
    RecoveryBlocked RecoveryAction = "blocked"
    RecoveryHumanRequired RecoveryAction = "human_required"
)

type ExhaustionAction string // degraded | blocked | human_required
type WorkflowFallback struct
type RecoveryPolicy struct
type RecoveryInput struct
type RecoveryDecision struct

func NewRecoveryPolicy(...) (RecoveryPolicy, error)
func DecideRecovery(RecoveryPolicy, RecoveryInput) (RecoveryDecision, error)
```

The frozen constructor/input fields are:

- policy version, retry delay, attempt-credit ceiling, exhaustion action;
- whether retry/fallback requires recovery approval;
- optional one-shot workflow fallback key;
- exact Team instance, plan, logical node, current attempt/max attempts;
- exact current Agent/Runtime binding;
- exact Evidence, output summary, OutputContract, and Classification digests;
- bounded prior classifications and whether fallback was already consumed;
- remaining attempt credits;
- authoritative decision time; and
- the recovery-approval requirement frozen for the node.

No raw output, Rule payload, authorization presentation, credential, Grant,
prompt, hidden reasoning, Provider, model, or session identifier enters the
policy.

Rules:

1. `valid_nonempty` and `valid_empty` produce `none`.
2. `transient_empty` selects `retry` when an attempt and credit remain and the
   frozen node does not require recovery approval.
3. `invalid` selects the configured unused workflow `fallback` first; absent
   fallback it may use retry only when the policy explicitly permits invalid
   retry.
4. Retry/fallback consumes one attempt credit, binds attempt+1, and sets exact
   `retry_at = decision_time + retry_delay`.
5. Workflow fallback binds its explicit key but preserves the current
   AgentInstance and RuntimeInstance. It selects alternate workflow data/step
   input, never Provider/model/ambient Runtime fallback.
6. Any recovery-approval requirement produces `human_required`, with no
   scheduled attempt. S4-W2 accepts no ApprovalRequest ID/digest, projected
   approval, actor string, authorization digest, or caller-supplied approved
   boolean because S4-W1 authorizes only pre-claim `start_run`.
7. Attempt or credit exhaustion produces exactly the configured
   `degraded`, `blocked`, or `human_required` action.

The decision digest binds the full policy/input/output. Callers cannot mutate
the returned value. No time, budget, action, fallback, approval, or binding is
read from ambient state.

## Attempt classification commit

`TeamAttemptEvidenceInput` replaces caller-supplied Evidence identity with:

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
    Receipt evidence.AttemptReceipt
    Classification verification.Classification
    CorrelationID string
}
```

`Receipt` and `Classification` are non-constructible immutable values returned
only by Evidence Store and the pure classifier. The accepted
`CommitTeamAttemptEvidence` transaction derives Evidence ID/digest/summary from
the receipt and validates:

- exact Team/plan/node/attempt and current claim generation;
- exact terminal Run, Evidence ID/digest, attempt summary Evidence binding,
  OutputContract digest, classification observation digest, and
  classification digest;
- Run terminal status and summary terminal status equality; and
- output counts/digests within the inherited attempt bounds.

It atomically appends the existing `EvidenceSubmitted` and
`TeamNodeAttemptTerminal` facts. The terminal payload adds only:

```text
output_contract_version
output_contract_digest
output_classification
output_classification_digest
output_summary_digest
```

No raw output enters Journal. Exact retry returns the committed record.
Divergent output/contract/classification is
`ErrTeamExecutionConflict` with zero Events.

RED must prove that zero receipts/classifications and forged caller-supplied
Evidence IDs, Evidence digests, or summary digests cannot reach the commit API
or create an Event.

A valid classification makes a successful Runtime attempt satisfy the logical
node as before. `transient_empty` or `invalid` always makes the node
`awaiting_recovery`, including when the Runtime process itself succeeded.
Failed/cancelled Runtime terminals also await the explicit policy decision.
Attempt exhaustion alone does not invent a terminal outcome.

## Recovery decision commit and scheduling

Before the first attempt is scheduled or dispatched, `TeamDispatchInput`
requires exactly one immutable semantic binding for each ExecutionPlan node:

```go
type TeamNodeSemanticBinding struct {
    LogicalNodeID string
    OutputContractVersion int
    OutputContractDigest string
    RecoveryPolicyVersion int
    RecoveryPolicyDigest string
    AttemptCredits int
    PrimaryWorkflowPath string
    WorkflowFallbackKey string
    RecoveryApprovalRequired bool
}
```

Bindings are sorted by logical node ID, duplicate-free, complete, and included
in `TeamExecutionPlanned` plus its deterministic Event identity. Attempt
credits are 0–2 and cannot permit more attempts than the plan node's
`MaxAttempts`. The optional fallback key is fixed once and cannot equal the
primary path. Re-entry after any crash must reproduce every binding exactly.
Changed OutputContract/RecoveryPolicy version or digest, credits, path/fallback
key, or approval requirement is `ErrTeamExecutionConflict` before dispatch.

Accepted historical S3 `TeamExecutionPlanned` schema-v1 Events without the new
binding field replay as `legacy_semantic_unbound`. They remain queryable and
are never rewritten or treated as having a default policy. S4-W2
classification/recovery against an unbound legacy Team returns
`ErrTeamAttemptRecoveryRequired` with zero mutation. Only a newly planned Team
whose bindings were committed before its first WorkItem/Run may use S4-W2.

Each `TeamNodeAttemptScheduled` stores its exact workflow-path key. Initial
attempt uses the frozen primary path; retry preserves the current path;
fallback may switch once to the frozen fallback key. AgentInstance and
RuntimeInstance remain unchanged.

`ScheduleTeamNodeRecovery` accepts the immutable RecoveryDecision plus exact
correlation ID. It no longer accepts a caller-selected action/budget/fallback.
The authority transaction-consistently replays the Team stream and validates:

- current node is exactly `awaiting_recovery`;
- decision Team/plan/node/attempt and Evidence/classification binding exactly
  equals the committed attempt terminal fact;
- policy version/digest, prior classifications, fallback-consumed state,
  attempt ceiling/credit accounting, frozen approval requirement, decision time,
  `retry_at`, action, and next binding are internally exact;
- retry preserves the current workflow path/Agent/Runtime;
- fallback is unused, preserves current Agent/Runtime, and binds one explicit
  workflow fallback key; and
- terminal actions carry zero `retry_at`, no next binding, and no fallback
  key.

One `AppendBatchIfStreamHeads` CAS appends:

```text
TeamNodeRecoveryRecorded
[TeamNodeAttemptScheduled]
[TeamExecutionTerminal]
```

`TeamNodeRecoveryRecorded` stores only bounded policy/decision,
classification, budget, approval-required marker, workflow fallback key, and
attempt-binding metadata. It stores no raw authorization or output.

Exact retry is idempotent. Concurrent schedulers have one winner. There is no
hidden conflict retry. A stale generation, Evidence/classification/policy
change, divergent retry, exceeded attempt/credit ceiling, attempted approval
bypass, or changed Team head writes zero Events and returns the existing typed
`ErrInvalidTeamRecovery`, `ErrTeamAttemptLimit`, or
`ErrTeamExecutionConflict`.

## Coordinator execution

`TeamExecutionRequest` binds one OutputContract and RecoveryPolicy per logical
node, the frozen semantic binding values above, and the already-defined
execution for each possible attempt. The first dispatch persists all bindings
before creating any WorkItem/Run/claim. A fallback
attempt's execution is explicitly labeled with the exact workflow fallback
key; retry keeps the prior workflow path. Every next execution preserves the
node's accepted Agent/Runtime binding.

After an attempt terminal:

1. finalize/reopen the exact Evidence receipt and summary;
2. construct the immutable output observation and classify it;
3. commit Evidence plus attempt classification;
4. if awaiting recovery, derive one pure RecoveryDecision;
5. commit that exact decision once; and
6. only a persisted, due scheduled attempt may enter the existing ready-set
   dispatch.

The Coordinator does not read raw artifact bytes and does not infer a decision
from Projection alone. Projection supplies heads/state; Evidence receipt and
frozen request contracts supply exact semantic inputs.

If `retry_at` is after the supplied authoritative time, the invocation returns
`ErrTeamExecutionIncomplete` with zero execution of that attempt. It does not
sleep. Re-entry with the same contracts/policy recognizes exact classification
and recovery facts; changed contract/policy/budget/approval input conflicts
without duplicate Run/Grant/Evidence.

At most nine state-changing passes are allowed by the three-node,
three-attempt ceiling. Each dispatch remains the accepted one-CAS ready-set
operation. Scheduler conflict is returned, never hidden-retried.

## Projection and view

The existing Team projection adds copied fields for:

- attempt OutputContract version/digest;
- output classification/digest and summary digest;
- recovery policy version/digest and decision digest;
- frozen per-node attempt credits, primary/fallback workflow keys, and
  recovery-approval requirement;
- recovery action, decision time, `retry_at`, budget before/after;
- workflow-path and fallback-use metadata.

Replay revalidates legal state, exact attempt order, digest formats, count/
budget bounds, action-specific zero/nonzero fields, retry/fallback next attempt
identity, and terminal aggregation. Unknown/malformed/duplicate/divergent
facts fail the entire rebuild. Projection failure preserves the exact old
Snapshot and GlobalReadView. Accessors deep-copy requested Team records only.
Projection/View never classify, decide, schedule, authorize, or accept.
Historical S3 streams expose the explicit legacy-unbound marker and otherwise
rebuild identically.

## Mandatory RED

Behavioral tests are written before behavior changes and must fail for:

1. immutable attempt summaries and reopen/exact-retry digest binding;
2. all four output classifications, empty-output policy, invalid bounds, and
   input mutation isolation;
3. pure retry/fallback/degraded/blocked/human-required decisions, exact
   `retry_at`, budget/attempt exhaustion, approval gating, and immutable
   digests;
4. pre-dispatch semantic binding freeze and crash/re-entry rejection of any
   changed OutputContract, RecoveryPolicy, credits, workflow fallback, or
   approval requirement, while accepted S3 unbound replay remains readable
   but cannot gain S4-W2 authority;
5. atomic classification commit through a Store-returned receipt and rejection
   of zero/forged/stale/mismatched Evidence, output summary, contract,
   generation, and exact retry;
6. recovery decision CAS, concurrent one-winner, future-due fencing, distinct
   next Run/generation/Grant/Evidence lineage, and no hidden retry;
7. Coordinator non-empty success, allowed empty success, transient-empty
   retry, invalid workflow fallback, exhaustion, restart/re-entry, and stale
   policy/view behavior; and
8. Projection/View replay, exact metadata, mutation isolation, malformed fact
   failure preservation, and no raw output exposure.

Static marker tests supplement but do not replace behavioral RED.

## Acceptance

All must pass:

1. Attempt summary, classifier, and policy are deterministic, bounded,
   immutable, and exact-digest-bound.
2. All four classifications are observable from the same authorized attempt
   capture; allowed empty is not mistaken for transient empty.
3. Classifier produces no authority; policy alone chooses recovery and
   scheduler executes only the persisted exact decision.
4. Retry/fallback is bounded by attempts and credits, has explicit UTC
   `retry_at`, never sleeps, and exhaustion becomes the configured terminal
   action.
5. Workflow fallback is explicit, one-shot, local, preserves Agent/Runtime,
   and is not Provider/model fallback.
6. Every next attempt has a distinct WorkItem/Run/generation/Grant/Evidence
   lineage; stale output/generation cannot affect it.
7. Exact retry and restart/re-entry produce no duplicate Event, Run, Grant,
   capture, receipt, or Evidence.
8. Concurrent decision/scheduler calls have one CAS winner; stale view/head/
   policy/budget conflicts and attempted approval bypass are visible and never
   hidden-retried.
9. Projection/View rebuild exactly, copy records, fail closed, and preserve
   the old view on malformed recovery facts.
10. Journal receives only bounded classification/decision metadata and
    digests, never raw Frame/output, Grant, credential, prompt, hidden
    reasoning, or per-token data.
11. Existing S4-W1 approval, S3 generation/Grant/Frame fencing, Bridge v1,
    Supervisor/Adapter, Runtime capacity, Evidence artifact, and normal Team
    success semantics remain compatible.
12. No deterministic acceptance, Verifier, WorkItem Done, API/CLI/daemon,
    Provider/model fallback, checkpoint, dependency, S4-W3, S4-W4, or Phase 2
    implementation is added.

## Required checks

Focused:

```text
go test ./internal/verification ./internal/rules ./internal/evidence ./internal/work ./internal/app ./internal/projection -count=1
go test -race ./internal/verification ./internal/rules ./internal/evidence ./internal/work ./internal/app ./internal/projection -count=10
```

Impact/full:

```text
go test ./internal/teams ./internal/authorization ./internal/supervisor ./internal/runtime/piadapter -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
gofmt -d <all owned Go files>
git diff --check
GOOS=windows GOARCH=amd64 go build ./internal/evidence ./internal/verification ./internal/rules
```

Audits:

- exact owned-file diff and accepted-boundary amendment;
- no caller-selected recovery action remains on the coordinator path;
- no raw Frame/output/terminal reason/Grant/credential/prompt/hidden reasoning
  in Journal, Projection, log, or error;
- no Provider/model/ambient Runtime fallback or session/checkpoint authority;
- no normal Team/Run write `ReadAll`, second Journal/StateWriter/Projection/
  scheduler/recovery authority, dependency, or test weakening;
- user dirty files and post-S3 scratch queue unstaged; and
- complete RED, GREEN, summary/classification/policy mutation, concurrency,
  restart, projection-failure, and Reviewer evidence.

After all checks pass, a fresh independent implementation Reviewer must return
`PASS`. The Developer may report only `ready_for_review`; the Controller then
accepts and creates one local atomic S4-W2 commit.

## Trust and activation boundary

Tests use deterministic Bridge Frames, in-process executors, fake clocks, local
SQLite, and private temporary Evidence state. They do not authenticate a real
user, call a Provider/model, use installed Runtime/credentials/network, start
a daemon, stream to a client, resume a model session, or activate autonomy.

VERDICT: FROZEN
