# S4-W1 Contract — Customer Rule and Durable Approval Authority

Status: FROZEN — independent Contract Review Round 4 PASS.

- WorkItem: `S4-W1`
- Risk: `HIGH`
- Baseline: `7bb9881`
- Date: `2026-07-26`
- Depends on: accepted Slice 3 and frozen Slice 4 Exit Contract
- Amendment:
  `../EXIT-CONTRACT-AMENDMENT-1.md`
- Capability: versioned customer Rule evaluation plus restart-safe exact-action
  approval authority

S4-W1 is one policy/persistence vertical boundary. Rule codec, matcher, effect
merge, approval writer, WorkItem replay, Projection, and View are internal
parts of this Candidate. No parser-only, writer-only, projection-only, or
resume-wrapper WorkItem may follow.

## Owned files

New product and tests:

- `internal/rules/authority.go`
- `internal/rules/authority_test.go`
- `internal/projection/approval.go`
- `internal/projection/approval_test.go`

Accepted Slice 3 files reopened only by reviewed Exit Contract Amendment 1:

- `internal/work/run_authority.go`
- `internal/work/run_authority_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`

Governance:

- `.loom-evidence/phase1-slice4/EXIT-CONTRACT.md`
- `.loom-evidence/phase1-slice4/EXIT-CONTRACT-REVIEW-1.md`
- `.loom-evidence/phase1-slice4/EXIT-CONTRACT-AMENDMENT-1.md`
- `.loom-evidence/phase1-slice4/EXIT-CONTRACT-AMENDMENT-1-REVIEW-1.md`
- `.loom-evidence/phase1-slice4/S4-W1/**`
- Controller-owned S4-W1 hunk in `docs/CURRENT.md`

No other file is owned. `AGENTS.md`, `PROGRESS.md`, `.codex/**`,
`.loom-drafts/**`, and the post-S3 scratch queue remain untouched/unstaged.

## Domain limits

All values are trimmed valid UTF-8 without control characters.

- opaque IDs, action keys, actor refs, and scope IDs: 1–128 bytes;
- RuleSet: 1–32 Rules;
- matched Rule references: at most 16;
- approver refs per Rule: 1–8, sorted unique;
- warning/record markers: at most 16 total, each at most 128 bytes;
- continuation digest and authorization digest: lowercase SHA-256 hex;
- rule version: positive and monotonically increasing by exactly one;
- approval timeout: 1 second through 30 days;
- stored payload: bounded by existing Journal Event limits.

Scopes are exactly `project`, `team`, `work_package`, or `work_item`. Risk is
`low`, `medium`, or `high`. Effects are exactly `record`, `warn`,
`require_approval`, or `reject`. The default with no matching Rule is `allow`.

## Public immutable surface

Exact names may be reduced during minimal implementation but no additional
authority API may be added without amendment.

```go
type Scope struct
type Condition struct
type Effect struct
type Rule struct
type RuleSet struct
type ActionContext struct
type Decision struct
type RuleSetReference struct

func NewRuleSet(
    scope Scope,
    version int,
    rules []Rule,
) (RuleSet, error)

func Evaluate(
    ruleSets []RuleSet,
    action ActionContext,
) (Decision, error)
```

All constructors snapshot input. Accessors return values or deep copies.
Canonical digesting uses schema-tagged length-prefixed fields and sorted rule
records; it is not delimiter concatenation. Equal semantic inputs produce
equal digests regardless of caller slice/map order.

`ActionContext` contains exact project/team/work-package/work-item IDs, action
key, risk, WorkItem/Run/logical-node/attempt identity, claim generation, and
contract digest. Empty non-applicable scope IDs are allowed only where the
specific Rule scope does not require them.

S4-W1 persists approval only for the exact action key `start_run`, before the
assigned Run is claimed. The persisted binding therefore requires empty claim
ID and claim generation zero. Pure `Evaluate` may evaluate other bounded action
keys for future consumers, but S4-W1 does not persist or enforce them without a
reviewed amendment.

## Deterministic matching and merge

Applicable RuleSets are selected only by exact scope identity. Within them:

1. conditions match exact action and optional exact risk;
2. matched Rule references are sorted by scope specificity
   (`work_item > work_package > team > project`), then Rule ID and version;
3. `reject` has highest precedence;
4. otherwise `require_approval` pauses the action;
5. otherwise `warn` and `record` preserve sorted unique markers;
6. otherwise the decision is `allow`.

`Evaluate` accepts at most four RuleSets: at most one current RuleSet for each
exact project/team/work-package/work-item scope. Duplicate scope, duplicate or
stale revision, scope/input mismatch, or more than four unique RuleSet streams
fails closed. The Decision references at most four sorted unique RuleSet
streams and at most sixteen matched Rules.

All matching `require_approval` Rules must have the same timeout and
`on_timeout` effect. Their approver refs are unioned and sorted. Incompatible
approval policies fail closed with typed `ErrRuleEffectConflict`; iteration
order never selects a winner.

`on_timeout` is exactly `reject` or `cancel`. `human_required` belongs to the
S4-W2 recovery decision and is not a S4-W1 WorkItem status. Timeout policy
controls only the recorded resolution/status and never performs an external
side effect.

## Customer authorization port

Rule activation and approval decision require an injected, non-nil
`CustomerAuthorizer`:

```go
type CustomerAuthorizer interface {
    AuthorizeRuleSet(
        context.Context,
        RuleSetActivationRequest,
    ) (AuthorizedRuleSetActivation, error)

    AuthorizeApprovalDecision(
        context.Context,
        ApprovalDecisionRequest,
    ) (AuthorizedApprovalDecision, error)
}
```

`RuleSetActivationRequest` and `ApprovalDecisionRequest` contain the exact
bounded command plus an opaque, bounded authorization presentation. The
presentation is snapshotted, passed only to the injected authorizer, then
discarded. It never enters a digest intended for public comparison, Journal,
Projection, View, log, error, or Evidence.

The authorized values have unexported authority material and expose only
bounded actor reference, command digest, authorization digest, issued/expiry
time, and exact request identity. The Rule Authority verifies the complete
binding and expiry. Tests use a deterministic fake; S4-W1 adds no CLI/API/user
authentication implementation.

`AuthorizedRuleSetActivation` binds the exact scope kind/ID, revision,
canonical RuleSet digest, actor reference, command digest, authorization
digest, issued/expiry time, correlation ID, and request identity. Any mismatch
or expired value is a zero-write authorization denial.

An Agent, model Proposal/Candidate, raw actor string, Projection record, or
ApprovalRequest ID alone cannot activate a RuleSet or decide an approval.
Authorization raw material never enters Journal, Projection, logs, errors, or
Evidence.

## RuleSet activation authority

```go
type Authority struct

func NewAuthority(
    store *journal.Store,
    authorizer CustomerAuthorizer,
    now func() time.Time,
) (*Authority, error)

func (*Authority) ActivateRuleSet(
    context.Context,
    RuleSetActivationRequest,
    string, // correlation ID
) (RuleSetRecord, error)
```

Stream:

```text
rule-set/<scope-kind>/<scope-id>
```

Event:

```text
RuleSetActivated
```

The exact payload includes scope kind/ID, revision, canonical RuleSet digest,
complete bounded Rule definitions, actor reference, authorization digest, and
authorization command digest. Revision one requires an empty stream; later
revisions require the previous exact head and revision. Exact retries return
the committed record. Same revision with different bytes, skipped/older
revision, stale authorization, digest mismatch, or head conflict returns a
typed conflict without mutation.

`ActivateRuleSet` calls the injected authorizer itself. Callers cannot supply
or construct `AuthorizedRuleSetActivation`. The Authority compares every
returned binding to the original request and correlation ID before any new
Journal mutation. It may first read the target stream only to recognize an
exact already-committed retry and return its public record without performing
another authorization or mutation; a divergent retry never uses this path.
Otherwise it reads the exact RuleSet stream after authorization and uses
`AppendBatchIfStreamHeads`; it has no hidden retry.

## Approval request authority

```go
type ApprovalRequestInput struct {
    Context            ActionContext
    ContinuationDigest string
    Decision           Decision
    RequestedAt        time.Time
    CorrelationID      string
}

func (*Authority) RequestApproval(
    context.Context,
    ApprovalRequestInput,
) (ApprovalRequestRecord, error)
```

The Decision must be an immutable `require_approval` result for `start_run`
produced from the exact referenced RuleSet records. The Authority
transaction-consistently reads the WorkItem, empty Run head, every referenced
RuleSet, and target approval streams. It recomputes the Decision and validates:

- complete immutable ActionContext including exact project/team/work-package/
  work-item scope, risk, assigned WorkItem/Run/Agent binding, logical
  node/attempt, action, contract digest, empty claim ID, and generation zero;
- empty Run stream, no capacity reservation, and no Grant;
- current RuleSet heads/revisions/digests and contract digest;
- requested time from the injected canonical clock, not caller authority;
- WorkItem status exactly `assigned`, not already paused/blocked/cancelled; and
- the complete continuation digest.

`RequestedAt` is evidence only and must equal the canonical operation time; it
cannot select expiry.

Streams:

```text
approval/<approval-request-id>
work-item/<work-item-id>
```

Events appended in one `AppendBatchIfStreamHeads` call:

```text
ApprovalRequested
WorkItemApprovalPaused
```

Approval request identity is a deterministic UUID derived from the complete
ActionContext/continuation/Decision digests. The payload freezes all context
fields, matched
RuleSet references, sorted approvers, timeout/on-timeout, previous WorkItem
status, requested/expires times, and causation.

The request uses at most seven unique read/CAS streams: at most four RuleSet
streams plus one WorkItem, one empty Run, and one Approval stream. Exact retry
is idempotent. A different payload for the same request, stale
RuleSet/Run/WorkItem head, paused/blocked/cancelled WorkItem, already-claimed
Run, malformed stream, duplicate RuleSet scope, or more than seven required
heads returns a typed error with zero Events. No unbounded retry or second
status writer is added.

## Approval decision authority

```go
func (*Authority) DecideApproval(
    context.Context,
    ApprovalDecisionRequest,
    string, // correlation ID
) (ApprovalRequestRecord, error)

func (*Authority) ExpireApproval(
    context.Context,
    string, // approval request ID
    string, // correlation ID
) (ApprovalRequestRecord, error)
```

Decisions are exactly `approved`, `rejected`, or `cancelled`. `ExpireApproval`
is clock-driven and valid only at/after the frozen expiry. `DecideApproval`
calls the injected authorizer itself; callers cannot construct or submit an
`AuthorizedApprovalDecision`. The returned authorization binds approval
request ID/digest, decision, actor ref, command digest, authorization digest,
issued/expiry, and correlation identity, and the Authority compares the whole
binding to the original request before any new Journal mutation. It may first
read the Approval stream only to recognize an exact already-committed decision
retry and return its public record without another authorization or mutation;
a different decision or payload never uses this path. For `approved`, actor
ref must match one frozen approver ref. Rejected/cancelled also require
authorized customer identity; expiration does not.

The Authority reads approval, WorkItem, empty Run, and referenced RuleSet
streams in one transaction-consistent set. It revalidates at most seven exact
heads, the still-unclaimed generation-zero Run, contract/continuation/RuleSet
digests, WorkItem `waiting_approval`, and authorization.

One CAS appends:

```text
ApprovalDecided | ApprovalExpired
WorkItemApprovalResolved
```

Resolution is terminal-once:

- `approved` restores exact `assigned` and produces
  an immutable resume Candidate containing the continuation digest and exact
  heads; it performs no action itself;
- `rejected` and `on_timeout=reject` expiration set WorkItem `blocked`;
- `on_timeout=cancel` expiration sets WorkItem `cancelled`;
- `cancelled` sets WorkItem `cancelled`.

Exact same decision retry returns the committed record. A different decision,
wrong actor, expired authorization, a concurrent successful Run claim, changed
RuleSet/contract/continuation, non-paused WorkItem, or head conflict returns a
typed error without partial mutation. No client presence, Projection,
notification, or resume Candidate can decide or resume work.

## Replay, Projection, and View

Accepted Work Authority replay recognizes only the two frozen WorkItem approval
Events and enforces their exact transition/causation/approval identity. Normal
`Claim` requires WorkItem status `assigned`, so it fails closed while the
WorkItem is `waiting_approval`, `blocked`, or `cancelled` before allocating
claim identity or Runtime capacity. A request-vs-claim race has one CAS winner.
Because S4-W1 cannot request approval after claim, existing claimed/running
start, failure, cancellation, terminal, capacity-release, and Grant-revocation
semantics are unchanged and cannot race an ApprovalRequest.

The existing Projection atomically rebuilds:

- current RuleSet revision per exact scope;
- ApprovalRequest with complete binding, status, decision actor/time, and
  continuation digest; and
- WorkItem approval status.

Malformed, duplicate, out-of-order, trailing JSON, unknown version,
cross-stream, cross-WorkItem, or dangling references fail the full rebuild and
preserve the old Snapshot and `GlobalReadView`.

GlobalReadView adds typed accessors for exact RuleSet scope and ApprovalRequest
ID, copying only the requested record. It does not expose authorization raw
material and cannot activate/decide/resume.

## Typed failures

At minimum:

- `ErrInvalidRuleInput`
- `ErrRuleEffectConflict`
- `ErrRuleAuthorityConflict`
- `ErrCustomerAuthorizationRequired`
- `ErrCustomerAuthorizationDenied`
- `ErrInvalidApprovalInput`
- `ErrApprovalNotRequired`
- `ErrApprovalAlreadyPending`
- `ErrApprovalNotPending`
- `ErrApprovalAlreadyTerminal`
- `ErrApprovalExpired`
- `ErrApprovalStale`
- `ErrApprovalRunAlreadyClaimed`

Journal conflicts are mapped without hiding their cause category. Errors do not
include raw authorization material, Grant, credentials, sensitive Rule
payloads, or continuation bytes.

## Mandatory RED

Behavioral tests are written before product symbols/behavior and must fail for
missing:

1. immutable RuleSet construction, canonical digest, exact scope matching, and
   deterministic precedence/merge/conflict;
2. authorizer-required RuleSet activation with exact retry and stale revision
   conflict;
3. atomic `ApprovalRequested + WorkItemApprovalPaused`;
4. approve/reject/cancel/expire terminal-once resolution and exact status;
5. restart/reopen, non-empty Run head, stale head/RuleSet/contract/continuation
   rejection, concurrent request-vs-claim, and concurrent decision one-winner
   behavior;
6. Work Authority compatibility and no Claim bypass while
   paused/blocked/cancelled; and
7. Projection/View rebuild, mutation isolation, and failure preservation.

Static boundary tests supplement but do not replace behavioral RED.

## Acceptance

All must pass:

1. Rule evaluation is pure, deterministic, bounded, immutable, and independent
   of input/map order.
2. RuleSet activation requires unforgeable injected authorization, monotonic
   revision, exact digest, CAS, and no hidden retry.
3. Model/Agent/Projection/raw actor strings cannot activate or decide.
4. Approval request atomically freezes exact `start_run`, assigned
   WorkItem/Run/attempt, generation zero,
   contract, RuleSet, continuation, approvers, timeout, and WorkItem pause.
5. Decisions/expiration are terminal-once, actor/time/binding checked,
   restart-safe, and atomically resolve WorkItem status.
6. Approval never itself performs or resumes the action; the resume Candidate
   is immutable, exact-head-bound, and non-authoritative.
7. Existing Work Authority rejects Claim while paused/blocked/cancelled,
   request-vs-claim has one CAS winner, and claimed/running terminal semantics
   remain unchanged because no post-claim approval can exist.
8. Projection/GlobalReadView are rebuildable, immutable, failure-preserving,
   and never a decision authority.
9. Concurrent request-vs-claim and decision races have one winner and no
   partial Events; maximum legal four-scope approval stays within seven CAS
   heads.
10. No raw authorization/Grant/credential/continuation bytes reach Journal
    beyond frozen digests/metadata, Projection, error, log, or Evidence.
11. No dependency, daemon/API/CLI, Runtime/Provider, output/retry/Verifier/Done,
    checkpoint, second authority, or S4-W2 implementation is added.

## Required checks

Focused:

```text
go test ./internal/rules ./internal/work ./internal/projection -count=1
go test -race ./internal/rules ./internal/work ./internal/projection -count=10
```

Impact/full:

```text
go test ./internal/app ./internal/supervisor ./internal/runtime/piadapter -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
gofmt -d <all owned Go files>
git diff --check
```

Audits:

- exact owned-file diff and accepted-file amendment;
- no normal Run write `ReadAll`;
- no raw authorization/Grant/credential/continuation leak;
- no model/Agent activation or approval authority;
- no second Journal/StateWriter/Projection/approval authority;
- no test/dependency weakening;
- user dirty files and post-S3 scratch queue unstaged;
- complete RED, GREEN, mutation/concurrency/restart, and Reviewer evidence.

After all checks pass, a fresh independent implementation Reviewer must return
`PASS`. The Developer may report only `ready_for_review`; the Controller then
accepts and creates one local atomic S4-W1 commit.

## Trust and activation boundary

Tests use only deterministic in-process customer-authorizer fakes, fake clocks,
local SQLite, and private temporary state. They do not authenticate a real
user, expose an approval API, execute an approved action, use installed
Runtime/Provider/credentials/network, start a daemon, or activate autonomy.

VERDICT: FROZEN
