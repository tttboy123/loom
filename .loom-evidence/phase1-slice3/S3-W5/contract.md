# S3-W5 Contract — Team DAG Execution Integration

- WorkItem: `S3-W5`
- Risk: `HIGH`
- Frozen baseline: `cd1e594`
- Date: `2026-07-26`
- Depends on: accepted `S3-W1`, `S3-W2`, `S3-W3`, `S3-W4`;
  Exit Contract Amendments 1 and 2
- Decision:
  `docs/adr/0008-stream-set-views-and-team-dispatch-cas.md`
- Capability: one complete Team DAG execution, state-read, dispatch,
  concurrency, recovery, and controlled local canary boundary

This is the fifth and final Slice 3 product WorkItem. Stream-set reading,
GlobalReadView, Grant identity, planner, writer, coordinator, scheduler,
recovery, and canary are internal parts of this one vertical Candidate. No
S3-W6 or thin follow-up WorkItem is permitted.

## Accepted prerequisites

S3-W5 consumes and does not duplicate:

- S3-W1 bounded Bridge v1 Frames and per-Run stream binding;
- S3-W2 WorkItem/Run claim, lease, generation, Runtime capacity, terminal, and
  Journal CAS authority;
- S3-W3 hash-only AgentGrant issue, authorization, expiry, and revocation;
- S3-W4 managed workspace, production Runtime Adapter, Supervisor, process
  cleanup, and failure terminal;
- Slice 2 saved-Team records, exactly-one-Main and at-most-two dormant
  SubAgents, Runtime discovery/status facts, Projection, Journal, and immutable
  Evidence Artifact Store.

The only accepted-boundary changes are those reviewed in the two Exit Contract
amendments. `AppendBatchIfStreamHeads` remains the only authoritative
multi-stream writer.

## Owned files

Product and direct tests:

- `internal/journal/store.go`
- `internal/journal/journal_test.go`
- `internal/work/run_authority.go`
- `internal/work/run_authority_test.go`
- `internal/work/team_execution_authority.go`
- `internal/work/team_execution_authority_test.go`
- `internal/authorization/authority.go`
- `internal/authorization/authority_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`
- `internal/teams/execution_plan.go`
- `internal/teams/execution_plan_test.go`
- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`

Compatibility-only accepted integration tests, if required solely to invoke
the explicit Grant identity initializer:

- `internal/projection/grant_authority_test.go`
- `internal/supervisor/managed_execution_test.go`
- `internal/runtime/piadapter/execution_adapter_test.go`

Their existing assertions, production timeout values, fixtures, and accepted
product behavior may not be weakened.

Governance:

- `docs/adr/0008-stream-set-views-and-team-dispatch-cas.md`
- `.loom-evidence/phase1-slice3/S3-W5/**`
- `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-1.md`
- `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-1-REVIEW-1.md`
- `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-2.md`
- `.loom-evidence/phase1-slice3/EXIT-CONTRACT-AMENDMENT-2-REVIEW-1.md`
- Controller-owned S3-W5 hunks in `docs/CURRENT.md` and `PROGRESS.md`

No other file is owned. `AGENTS.md`, the user-owned historical `PROGRESS.md`
tail, `.codex/**`, and `.loom-drafts/**` remain untouched and unstaged.

## Public behavior

Exact exported names may be reduced during minimal implementation, but no
additional authority-bearing API may be added without a reviewed amendment.

### Journal related-stream set

Package `internal/journal` provides:

```go
type StreamHead struct {
    StreamID string
    Sequence int64
    EventID string
}

type StreamSetSnapshot

func (*Store) ReadStreamSet(
    context.Context,
    []string,
) (StreamSetSnapshot, error)

func (StreamSetSnapshot) Events() []Event
func (StreamSetSnapshot) Heads() []StreamHead
func (StreamSetSnapshot) Head(string) (StreamHead, bool)
```

The request contains 1–16 non-empty unique stream IDs. The read runs inside one
SQLite transaction, returns every requested head including sequence-zero empty
streams, orders streams and Events canonically, checks cancellation, and
deeply copies Event payloads and result slices. Caller mutation cannot alter
the Store or another read.

`AppendBatchIfStreamHeads` accepts 1–16 unique valid expectations. Seventeen or
more remains `ErrInvalidEventBatch`; the Event limit remains 32.

### Touched-stream authority replay

Every normal Run/WorkItem and AgentGrant write command reads one related stream
set and replays only the streams it validates and CASes. Normal write methods
must not call `ReadAll`, directly or through `Snapshot`.

Run/WorkItem command sets are exact:

- create/assign: WorkItem;
- claim/start/lease/terminal: WorkItem, Run, Runtime status, Runtime capacity;
- Team dispatch: Team execution plus every dispatched WorkItem, Run, Runtime
  status, and Runtime capacity stream;
- Evidence/aggregation: Team execution, Evidence, WorkItem, and Run streams
  for that node.

AgentGrant command sets include the WorkItem, Run, Runtime status/capacity,
per-Run Grant, and global Grant identity streams needed by the command.
Diagnostic full snapshots may still use deliberate full replay.

### Grant identity compatibility and uniqueness

Package `internal/authorization` provides one explicit idempotent initializer
for `agent-grant-identity/v1`. It is the only S3-W5 operation allowed to
`ReadAll`. It validates historical Grant replay, appends deterministic missing
identity reservations in bounded CAS batches, then writes a canonical
initialization marker. Conflict returns immediately; a caller may explicitly
invoke the initializer again to resume.

Normal Grant commands fail closed until initialization is complete. Every new
Issue atomically writes the per-Run `AgentGrantIssued` and global
`AgentGrantIdentityReserved` Events. Cross-Run Grant-ID and token-hash
collisions retain the accepted typed errors without randomness retry. Only the
token SHA-256 hash is stored.

### Immutable GlobalReadView

Every successful `projection.Projection.Rebuild` constructs one candidate
Snapshot and one candidate immutable GlobalReadView before taking the publish
lock, then atomically replaces both. Any source, decode, validation,
cancellation, or view-build error leaves both old values unchanged.

The view version is lowercase SHA-256 over sorted canonical tuples:

```text
stream_id NUL decimal_head_sequence NUL head_event_id LF
```

The empty Journal uses the SHA-256 of an empty byte sequence. The view exposes
typed accessors for WorkItem, Run, AgentGrant, Evidence, TeamInstance,
AgentInstance, RuntimeInstance, Team execution, and stream head. Accessors copy
only the requested record and its mutable fields; they never clone the whole
view or return internal maps/slices.

The view is a rebuildable cache. Its version and records are dispatch
provenance, not mutation authority. Dispatch rereads and CASes every touched
Journal head captured by the view.

### Pure Team DAG planner

Package `internal/teams` builds an immutable execution plan with:

- exactly three or fewer nodes;
- exactly one `main`;
- zero to two `subagent` nodes;
- unique node, WorkItem, Run, and AgentInstance IDs;
- explicit Runtime binding and sorted unique dependencies;
- no missing dependency, self-edge, cycle, or duplicate edge; and
- a deterministic SHA-256 plan digest.

The ready-set planner is pure: it receives a validated plan plus immutable
node states and Runtime capacity observations, then returns sorted pending
nodes whose dependencies succeeded. Failed/cancelled dependencies block their
dependents. It never reads time, randomness, Journal, Projection, process, or
global mutable state and never returns more nodes than available observed
capacity.

### Team execution authority

The coordination stream is exactly:

```text
team-execution/<team_instance_id>
```

Its accepted Event types are:

- `TeamExecutionPlanned`
- `TeamReadySetDispatched`
- `TeamNodeEvidenceCommitted`
- `TeamExecutionTerminal`

One dispatch:

1. validates the plan, Team identity, immutable view version, selected ready
   set, and every touched view head;
2. rereads one transaction-consistent related set;
3. replays the authoritative Team, WorkItem, Run, Runtime status, and Runtime
   capacity facts;
4. generates one random claim ID and generation-bound WorkItem/Run/capacity
   lineage per node;
5. appends Team, WorkItem creation/assignment, Run claim, and capacity
   reservation Events in one `AppendBatchIfStreamHeads` transaction; and
6. returns immutable dispatched records.

Two schedulers using the same view/ready set produce exactly one success and
one typed conflict. There is no hidden retry. Distinct Runtime status and
capacity heads are included. Capacity cannot be oversold.

Evidence bytes are published first to the accepted content-addressed Artifact
Store. A generation-bound aggregation transaction then appends exactly one
`EvidenceSubmitted` Event on `evidence/<evidence_id>` and one
`TeamNodeEvidenceCommitted` Event. Exact retry is idempotent; a different
digest, generation, node binding, or terminal fact conflicts. Stale generation
is rejected.

When all nodes are terminal, aggregation deterministically appends one
`TeamExecutionTerminal`: `succeeded` only if every node succeeded; otherwise
`failed` with a canonical reason. An executor may produce
`ready_for_review`; it never marks a WorkItem accepted/done.

### Application integration and recovery

Package `internal/app` owns one bounded Team execution coordinator. It consumes
the accepted Projection, Work/Grant authorities, Evidence Store, and
Supervisor through explicit interfaces. For a validated plan it:

- resumes already-dispatched nonterminal nodes before scheduling new work;
- issues one independent generation-bound Grant per node;
- starts ready nodes concurrently and waits with caller cancellation;
- never exceeds two concurrently active SubAgents;
- publishes one canonical Evidence artifact per terminal generation;
- commits node Evidence/terminal aggregation;
- rebuilds Projection between dependency waves; and
- stops after at most the plan's three nodes and one terminal aggregation.

The coordinator has no resident loop, timer, daemon activation, unbounded
queue, automatic CAS retry, Provider/model call, ambient credential lookup, or
user Runtime discovery. A caller invokes it explicitly with controlled
Supervisor fixtures or separately authorized production bindings.

After a crash:

- committed dispatch is discovered from Journal/Projection;
- absent Grant material causes an expired prepare lease to be reclaimed with a
  higher generation before a replacement Grant is issued;
- stale prior generations cannot start, submit Evidence, or commit terminal;
- committed Artifact bytes without Evidence metadata are safely republished
  and the missing metadata transaction is completed;
- existing Run and Evidence identities are reused rather than duplicated.

## Mandatory RED

Before behavior edits, tests must compile or fail for the missing:

- `ReadStreamSet`, deep-copy heads, and 16-head bound;
- no-`ReadAll` write-command trace;
- Grant identity initializer and cross-Run collision reservation;
- GlobalReadView version/accessor/failure-preservation behavior;
- DAG validation and deterministic ready set;
- atomic concurrent dispatch and capacity fencing;
- generation-bound Evidence/terminal aggregation; and
- controlled restart/concurrency canary.

RED evidence must record exact failing commands and expected missing behavior.
Tests may not be weakened to create RED.

## Acceptance

All must pass:

1. `ReadStreamSet` returns one-transaction canonical Events/heads, including
   empty heads, with cancellation and mutation isolation.
2. Seventeen CAS expectations fail before mutation; sixteen remain accepted.
3. Static and behavioral tests prove normal Run/Work and Grant write commands
   do not call or depend on `ReadAll`, and unrelated malformed streams do not
   poison a touched command.
4. Legacy Grant identity initialization is deterministic, resumable,
   hash-only, conflict-visible, and required before normal writes.
5. Cross-Run Grant ID/hash collisions remain typed and no randomness retry is
   added.
6. GlobalReadView version is canonical, immutable, record-local in copying,
   stable across equivalent rebuilds, changed by any head/Event-ID change, and
   preserved with Snapshot on rebuild failure.
7. Planner rejects invalid/cyclic/oversized plans and deterministically returns
   capacity-bounded dependency-ready nodes.
8. One two-node dispatch on distinct Runtime instances is atomic across all
   nine or more touched streams.
9. Two concurrent schedulers over one view produce one dispatch and one
   conflict; no duplicate WorkItem, Run, reservation, or Team dispatch Event
   exists.
10. Two independent SubAgent fixture executions overlap in wall-clock
    execution and never exceed observed Runtime capacity.
11. Each active SubAgent has an independent WorkItem, Run, Grant, and Evidence
    lineage; Main becomes ready only after dependencies succeed.
12. Stale view heads and stale generations are rejected without partial
    mutation.
13. Crash/reopen recovery completes without duplicate Run or Evidence, and
    terminal aggregation is deterministic and terminal-once.
14. A failed Projection rebuild preserves the exact previous GlobalReadView
    version/data and Snapshot.
15. The controlled canary uses only private temporary directories, local
    SQLite, deterministic Bridge-compatible fixtures, fake clocks/randomness,
    and no installed user Runtime, model, credentials, network, resident
    service, or autonomous activation.

## Required checks

Focused:

```text
go test ./internal/journal ./internal/work ./internal/authorization \
  ./internal/projection ./internal/teams ./internal/app -count=1
go test -race ./internal/journal ./internal/work ./internal/authorization \
  ./internal/projection ./internal/teams ./internal/app -count=10
go test ./internal/app \
  -run '^TestTeamDAGExecutionControlledCanary$' -count=10
go test -race ./internal/app \
  -run '^TestTeamDAGExecutionControlledCanary$' -count=3
```

Impact and full:

```text
go test ./internal/supervisor ./internal/runtime/piadapter -count=1
go test -race ./internal/supervisor ./internal/runtime/piadapter -count=3
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
gofmt -d <all owned Go files>
git diff --check
```

Audits must additionally prove:

- no production or test dependency addition;
- no normal authority write path reaches `ReadAll`;
- no raw Grant token reaches Events, Evidence, logs, errors, or view;
- no second writer/cache/checkpoint authority;
- no goroutine/process leak in cancellation paths;
- no edit or staging of user-owned dirty files/hunks;
- exact owned-file diff and RED/evidence completeness.

## Trust and live boundary

This Candidate may execute only deterministic local fixtures in test-owned
temporary roots. It does not authorize installed Pi, any user Runtime,
Provider/model traffic, credentials, network actions, long-running daemon,
autonomous scheduling, external publication, push, merge, rebase, reset,
force, or release.

After all checks pass, a fresh independent implementation Reviewer must return
`PASS`. The Developer may then record `ready_for_review`; only the Controller
may accept and create the one local atomic S3-W5 commit.

VERDICT: FROZEN
