# S2-W15 Frozen WorkItem Contract

- ID: `S2-W15`
- Title: Saved-Team Instance StateWriter
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W13, accepted S2-W14, and local S2-W14 commit
  `f293a9f`
- Corresponds to: `PRODUCT-PLAN.md §3.2, §4.6`,
  `TECH-PLAN.md §3.2, §4, §5, §6, §14 Slice 2, §15.2, §15.5, §15.8,
  §15.16, §15.21`, ADR-0001, ADR-0002, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `f293a9f`

## Owned files

- `internal/state/saved_team_writer.go`
- `internal/state/saved_team_writer_test.go`
- `.loom-evidence/phase1-slice2/S2-W15/deliverable.md`

Accepted Slice 1 and S2-W1 through S2-W14 product/test files remain unchanged.
Any ownership amendment requires a recorded Controller amendment and fresh
contract Reviewer PASS.

## Objective

Add the first bounded authoritative StateWriter for the direct saved-Team path:

1. revalidate the exact S2-W13 record set from all current S2-W9/S2-W11/S2-W12
   routing, catalog, discovery, selection, binding, plan, and identity sources;
2. build exactly two canonical schema-v1 Journal facts:
   `TeamInstanceCreated` and `AgentInstanceCreated`;
3. append both facts through accepted S2-W14 `AppendBatch` in one transaction;
4. accept exact idempotent whole-batch retry without creating duplicates; and
5. return a deterministic immutable commit Candidate only after both committed
   Events are verified.

This WorkItem creates authoritative Journal facts for one TeamInstance and one
Main AgentInstance. It does not create or activate dormant SubAgents, create
WorkItems/Runs/grants/workspaces/processes, update a projection, or execute.

## Frozen interfaces and input

```go
type EventBatchAppender interface {
    AppendBatch(context.Context, []journal.Event) ([]journal.Event, error)
}

type SavedTeamCommitInput struct {
    TeamEventID            string
    TeamIdempotencyKey     string
    MainAgentEventID       string
    MainAgentIdempotencyKey string
    EmittedAt              time.Time
}

CommitSavedTeamInstanceRecordSet(
    ctx context.Context,
    appender EventBatchAppender,
    records teams.SavedTeamInstanceRecordSetCandidate,
    plan teams.SavedTeamInstantiationPlanCandidate,
    intent mode.Intent,
    scope agents.ScopeIdentity,
    catalog teams.TeamResolutionCatalogInput,
    binding teams.SavedTeamRuntimeBindingCandidate,
    discovery runtime.RuntimeDiscoverySnapshot,
    selections []teams.SavedTeamRuntimeSelection,
    identity teams.SavedTeamInstanceIdentityInput,
    commit SavedTeamCommitInput,
) (SavedTeamCommitCandidate, error)
```

The writer:

- rejects a nil/typed-nil appender;
- calls accepted S2-W13 validation and requires `Valid=true`;
- requires non-empty, pairwise-distinct Event IDs and idempotency keys;
- requires nonzero `EmittedAt`, normalized to UTC;
- copies all mutable source/result bytes; and
- builds both Events fully before calling `AppendBatch` exactly once.

Existing S2-W9/S2-W11/S2-W12/S2-W13/S2-W14 typed errors propagate. New typed
errors distinguish invalid commit input, invalid source/result shape, appender
result mismatch, and commit digest mismatch.

## Canonical Events

The first Event is:

- `ID=TeamEventID`;
- `StreamID="team_instance:"+TeamInstanceID`;
- `Seq=1`;
- `IdempotencyKey=TeamIdempotencyKey`;
- `Type="TeamInstanceCreated"`;
- `SchemaVersion=1`;
- exact UTC `EmittedAt`;
- `CorrelationID=WorkRequestID`;
- empty `CausationID`; and
- canonical JSON payload with exact Team record, dormant SubAgent records,
  S2-W13 source plan/record-set digests, and counts.

The second Event is:

- `ID=MainAgentEventID`;
- `StreamID="agent_instance:"+MainAgentInstanceID`;
- `Seq=1`;
- `IdempotencyKey=MainAgentIdempotencyKey`;
- `Type="AgentInstanceCreated"`;
- `SchemaVersion=1`;
- exact UTC `EmittedAt`;
- `CorrelationID=WorkRequestID`;
- `CausationID=TeamEventID`; and
- canonical JSON payload with exact Main Agent record, accepted Runtime binding,
  S2-W13 source plan/record-set digests, and Team creation timestamp.

Payload keys are explicit lowercase snake case. `json.Marshal` over frozen
internal payload structs is the canonical encoder. No `intent.Text`, prompt,
credential, secret, raw customer content, process environment, or mutable
source object enters either payload.

The Team Event precedes the Main Agent Event in every append call. The Events
use separate resource streams, so both start at sequence 1.

## Frozen result

`SavedTeamCommitCandidate` contains:

- `Committed=true`;
- exact source record-set digest;
- copied committed Team Event;
- copied committed Main Agent Event;
- `EventCount=2`; and
- deterministic lowercase SHA-256 commit digest over every immutable Event
  field plus source record-set digest and count.

After `AppendBatch` returns, the writer requires exactly two returned Events in
requested order with exact immutable content. Any shorter, longer, reordered,
mutated, or otherwise unexpected result fails closed with zero Candidate.

## Acceptance criteria

1. A valid project or reusable saved-Team record set commits exactly two facts
   and no other row.
2. Team and Main Agent facts have exact stream, sequence, type, version,
   correlation, causation, timestamp, payload, and source digests.
3. Same-ID project/reusable shadow and project-Team/reusable-Agent fallback facts
   preserve the exact S2-W13 resolved scope/version.
4. Main-only/one/two dormant-member Teams still commit exactly one Team and one
   Main Agent fact; dormant members appear only in Team payload metadata.
5. Exact retry returns the same commit Candidate and row count remains two.
6. Changed Event identity/idempotency content, partial pre-existing facts, or
   occupied streams fail with accepted S2-W14 conflicts and no additional fact.
7. Zero/tampered/stale/route/catalog/discovery/selection/binding/plan/record
   sources fail before calling the appender.
8. Invalid commit IDs, duplicate IDs/keys, zero time, nil appender, cancelled
   context, closed database, append failure, or mismatched appender result
   returns zero Candidate.
9. Input/source/result mutation cannot alter committed facts or Candidate
   digest.
10. Equal semantics produce equal payloads and commit digest; changing any
    Event immutable field, source record/digest, dormant member, Main definition
    version/scope, Runtime binding, identity, timestamp, or count changes the
    applicable payload/digest.
11. Existing Slice 1 and S2-W1 through S2-W14 behavior remains green.
12. No projection mutation, direct table write, new schema, Team Draft mutation,
    dormant SubAgent AgentInstance, WorkItem/Run/Evidence/grant, capacity
    reservation, workspace, process/model/runtime execution, Bridge, claim,
    lease, credential, daemon/CLI/UI, network/filesystem/environment access,
    production goroutine, external action, or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer adds `internal/state/saved_team_writer_test.go` before production.
RED must fail on missing frozen S2-W15 symbols only.

Required groups:

1. project/reusable and scope fallback success with exact persisted Events;
2. Main-only/one/two dormant cardinality and canonical payload fields;
3. exact whole-batch retry and row-count idempotency;
4. event/idempotency/partial/stream conflict failure with no extra facts;
5. invalid/tampered/stale source matrix proving appender is not called;
6. invalid commit/nil/cancelled/closed/append/mismatched-result failure with zero
   Candidate;
7. payload/commit-digest sensitivity and input/result mutation isolation; and
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/state -run
  'TestCommitSavedTeamInstanceRecordSet' -count=1`
- Package full: `go test ./internal/state -count=1`
- Repeated focused race:
  `go test -race ./internal/state -run
  'TestCommitSavedTeamInstanceRecordSet' -count=50`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/state/saved_team_writer.go
  internal/state/saved_team_writer_test.go` and `git diff --check`
- Import boundary: standard library plus accepted `internal/agents`,
  `internal/journal`, `internal/mode`, `internal/runtime`, and
  `internal/teams` contracts only.

## Explicit exclusions

No changes to accepted Slice 1 or S2-W1 through S2-W14 product/test files. No
projection mutation, direct SQL/table write, migration/schema change, Team Draft
mutation, dormant SubAgent AgentInstance, WorkItem/Run/Evidence/grant, capacity
reservation, workspace, process/model/runtime execution, Bridge, claim
generation, lease, credential, network/filesystem/environment access,
production goroutine, daemon/CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
