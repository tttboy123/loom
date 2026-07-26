# S5-W1 Contract — Local Observation Stream and CLI Timeline Integration

Status: FROZEN — Exit Contract Amendment 1 Review 1 PASS.

- WorkItem: `S5-W1`
- Risk: `HIGH`
- Baseline: `006db8c`
- Date: `2026-07-26`
- Depends on: accepted Slices 1–4 and the frozen Slice 5 Exit Contract
- Capability: one bounded, read-only, user-observable Team timeline, Attention,
  board, reconnect, and authorized tentative-output delivery boundary

S5-W1 is the first of exactly two Slice 5 product WorkItems. Journal paging,
cursor validation, selective `GlobalReadView` accessors, authoritative record
mapping, the Team-bound tentative queue, Attention/board derivation, and CLI
wiring are internal parts of this one Candidate. No cursor-reader, queue,
Attention, CLI-wrapper, S5-W3, or other thin WorkItem may follow.

## Exact ownership

New product/tests:

- `internal/api/team_execution_stream.go`
- `internal/api/team_execution_stream_test.go`

Reopened product/tests:

- `internal/app/team_execution.go`
  - only if Slice 5 Exit Contract Amendment 1 passes: refresh the accepted
    Projection exactly once after dispatch/rebound and before executing the
    resulting tasks;
- `internal/app/team_execution_test.go`
  - only the Amendment 1 freshness/failure tests; it never imports
    `internal/api`;
- `internal/journal/store.go`
  - add only the bounded transaction-consistent after-head page reader, its
    immutable result value, limits, and typed read errors;
- `internal/journal/journal_test.go`
  - add only the reader contract/property/concurrency tests;
- `internal/projection/global_read_view.go`
  - add only Team-filtered copied accessors required to derive a bounded scope,
    Attention, and board without exporting internal maps;
- `internal/projection/global_read_view_test.go`
  - add only accessor filtering, stable ordering, deep-copy, and old-view tests;
- `internal/app/team_execution_stream_test.go`
  - new external-package (`package app_test`) integration test only; it injects
    the accepted `app.NodeOutputObserver` and proves post-capture authorized
    tentative delivery without making `package app` import its consumer;
  - no `internal/app` production file or existing app test file is reopened;
- `cmd/loom/query.go`
  - add only the finite read-only `timeline` command and injected dependency;
- `cmd/loom/main_test.go`
  - add only CLI schema, ordering, reconnect, gap, read-only, sanitization,
    cancellation, and side-effect tests.

Governance:

- `.loom-evidence/phase1-slice5/EXIT-CONTRACT.md`
- `.loom-evidence/phase1-slice5/EXIT-CONTRACT-AMENDMENT-1*.md`
- `.loom-evidence/phase1-slice5/EXIT-CONTRACT-REVIEW-*.md`
- `.loom-evidence/phase1-slice5/S5-W1/**`
- the Controller-owned S5-W1 hunk in `docs/CURRENT.md`

No other file is owned. In particular, no migration, dependency, daemon,
adapter, Supervisor, bridge-v1, Evidence store, writer, scheduler, rule,
verification, WorkPackage, or S5-W2 file is reopened. `AGENTS.md`,
`PROGRESS.md`, `.codex/**`, `.loom-drafts/**`, and the post-S3 scratch queue
remain untouched and unstaged.

## Package direction and reused authority

The dependency direction is exactly:

```text
cmd/loom
  -> internal/api
       -> internal/app (accepted NodeOutput value/interface only)
       -> internal/journal (read-only page API only)
       -> internal/projection (immutable view only)
       -> internal/supervisor and protocol/bridge/v1 (accepted copied
          AuthorizedFrame accessors only)
```

`internal/api` never imports `internal/work`, `internal/rules`,
`internal/verification`, a Runtime adapter, or SQLite. `cmd/loom` continues to
own read-only SQLite opening and passes accepted Journal/Projection readers to
`internal/api`; it contains no Journal SQL. Existing Projection rebuild remains
the only read-model builder and existing Journal remains the only fact
authority.

No S5-W1 API writes, migrates, vacuums, approves, dispatches, retries, verifies,
accepts, completes, creates, or activates anything.

## Exact local API surface

`internal/api` adds only this public behavior surface:

```go
type ViewSource interface {
    Rebuild(context.Context) error
    GlobalReadView() projection.GlobalReadView
}

type TeamExecutionStreamConfig struct {
    TeamInstanceID string
    Journal        *journal.Store
    Projection     ViewSource
    Now            func() time.Time
}

func NewTeamExecutionStream(
    config TeamExecutionStreamConfig,
) (*TeamExecutionStream, error)

func (stream *TeamExecutionStream) ReadPage(
    ctx context.Context,
    cursor string,
    limit int,
) (TimelinePage, error)

func (stream *TeamExecutionStream) Subscribe(
    ctx context.Context,
    cursor string,
) (*Subscription, error)

func (stream *TeamExecutionStream) ObserveNodeOutput(
    ctx context.Context,
    output app.NodeOutput,
) error

func (subscription *Subscription) Next(
    context.Context,
) (StreamItem, error)
func (subscription *Subscription) Close() error
```

The constructor validates and copies configuration. `Journal`, `Projection`,
and `Now` are required; `Now` must return non-zero UTC and is called only for a
request-gap occurrence time. The stream is permanently bound to the
constructor Team ID. `Close` is idempotent and returns `nil`.

Immutable public result values use private storage plus these copied accessors:

```go
func (page TimelinePage) TeamInstanceID() string
func (page TimelinePage) ViewVersion() string
func (page TimelinePage) NextCursor() string
func (page TimelinePage) HasMore() bool
func (page TimelinePage) Gap() (StreamGap, bool)
func (page TimelinePage) Records() []DeliveryRecord
func (page TimelinePage) Board() TeamBoard
func (page TimelinePage) Attention() []AttentionItem

func (item StreamItem) Delivery() (DeliveryRecord, bool)
func (item StreamItem) Gap() (StreamGap, bool)
```

`TimelinePage`, `DeliveryRecord`, `StreamGap`, `TeamBoard`, node rows,
`AttentionItem`, payload, and cost implement JSON encoding only through their
frozen wire structs. Their ordinary accessors return copied scalars/slices and
never a mutable map or raw payload. `TimelinePage.MarshalJSON` emits the exact
CLI response body fields except `command`; `cmd/loom` wraps it in the exact
`timeline` command response without reinterpreting records.

Recoverable page gaps use:

```go
type TimelineGapError struct { /* private copied page and causes */ }

func (err *TimelineGapError) Error() string
func (err *TimelineGapError) Unwrap() []error
func (err *TimelineGapError) Page() TimelinePage
```

`Unwrap` contains `ErrStreamGap` and exactly one specific
`ErrInvalidTimelineCursor`, `ErrTimelineCursorConflict`,
`ErrTimelineScopeOverflow`, or `ErrTimelinePageOverflow`. It never exposes a
database/path/raw-payload error. `ReadPage` returns the zero `TimelinePage` for
non-gap errors and returns the same copied safe page as
`TimelineGapError.Page()` for a recoverable gap.

`cmd/loom` extends its injected dependencies with exactly:

```go
type timelineInput struct {
    StatePath     string
    TeamInstanceID string
    Cursor        string
    Limit         int
}

timeline func(context.Context, timelineInput) (api.TimelinePage, error)
```

Production wiring opens the accepted read-only database, constructs one
Journal Store and Projection over that handle, constructs the Team stream, and
performs one finite `ReadPage`. Test injection does not open SQLite.

## Bounded Journal page API

`internal/journal` adds:

```go
const MaxCursorStreams = 96
const MaxReadPageEvents = 128

type StreamPage struct { /* immutable private storage */ }

func (page StreamPage) Events() []Event
func (page StreamPage) Heads() []StreamHead
func (page StreamPage) HasMore() bool

func (store *Store) ReadPageAfterHeads(
    ctx context.Context,
    heads []StreamHead,
    limit int,
) (StreamPage, error)
```

Inputs are copied, sorted by `StreamID`, and must contain 1–96 unique non-empty
stream IDs. A zero head is exactly sequence `0` plus empty Event ID. A non-zero
head must have sequence greater than zero and a non-empty Event ID that resolves
to the exact immutable Event at that stream/sequence. `limit` is 1–128.

The method opens one read-only transaction. For each stream it reads at most
`limit+1` Events after the supplied sequence, ordered by sequence and Event ID.
Every first result must be the supplied sequence plus one and all later results
for that stream must be contiguous. It merges the bounded candidates by:

```text
emitted_at UTC ascending, stream_id ascending, seq ascending, id ascending
```

It returns at most `limit` copied Events. Returned heads start as the supplied
heads and advance only through Events returned in this page; they never jump to
the current database head. `HasMore` is true exactly when at least one validated
candidate remains after the returned prefix. Payload bytes and all slices are
deep-copied on input, result construction, and access.

The reader never calls `ReadAll`, writes, retries, falls back to a different
scope, or normalizes a malformed/gapped history into success. Cancellation and
deadline errors remain discoverable with `errors.Is`.

## Selective immutable view access

`GlobalReadView` adds exactly:

```go
func (view GlobalReadView) WorkItemsForTeam(teamID string) []WorkItem
func (view GlobalReadView) ApprovalRequestsForTeam(
    teamID string,
) []ProjectedApprovalRequest
func (view GlobalReadView) AgentGrantsForRun(runID string) []AgentGrant
```

Each method returns only matching records, sorted respectively by WorkItem ID,
ApprovalRequest ID, and `(IssuedAt, Grant ID)`, with all nested slices copied.
Invalid/empty keys return a non-nil empty slice. No all-record map or mutable
reference is exposed. Existing keyed `Team`, `TeamExecution`, `Run`,
`Evidence`, and `RuntimeInstance` accessors remain unchanged and are used for
the rest of the bounded relation.

Projection rebuild still constructs a complete candidate and publishes
Snapshot plus `GlobalReadView` atomically. Failed rebuild retains the exact
previous view/version and never publishes a partial scope.

## Related Team stream scope

`api.TeamExecutionStream` is bound at construction to one non-empty
`team_instance_id`. On every page or subscription creation it obtains one
copied `GlobalReadView` and requires both the Team and TeamExecution.

The current related scope is the sorted unique set of:

1. `team-execution/<team_instance_id>`;
2. `work-item/<work_item_id>`, `run/<run_id>`, and
   `evidence/<evidence_id>` for every source/verifier attempt referenced by
   the TeamExecution or Team-filtered WorkItems;
3. exactly one `agent-grant/<run_id>` stream for every referenced source or
   verifier Run; copied `AgentGrantsForRun(run_id)` records cross-check Grant
   identities and bindings but do not change the accepted Run-keyed stream
   identity; and
4. `approval/<approval_request_id>` for copied Team approval requests.

Empty references are ignored. A referenced WorkItem/Run/Evidence/Grant/
Approval record whose identity or Team/attempt binding conflicts with the
TeamExecution fails closed as invalid view; it is not silently omitted.
Unreferenced Runtime, Agent, Team, mode, rule-set, and other project streams are
not added. Runtime offline Attention is derived from the current copied
RuntimeInstance record referenced by an attempt, not by widening the Journal
cursor.

The union of current scope and every stream retained by a supplied cursor is
limited to 96. Old cursor streams remain until the client intentionally starts
from a fresh cursor. New current streams absent from the old cursor receive a
zero head.

## Cursor v1

The opaque cursor is unpadded base64url of canonical compact JSON generated
from a struct in this exact field order:

```json
{
  "schema_version": 1,
  "team_instance_id": "team-id",
  "scope_digest": "lower-case-sha256",
  "view_version": "lower-case-sha256",
  "heads": [
    {"stream_id": "stream", "sequence": 1, "event_id": "event"}
  ]
}
```

There is no whitespace and no HTML escaping. Heads are sorted and unique.
`scope_digest` is lower-case SHA-256 over tag
`loom.team-stream-scope.v1` followed by each sorted stream ID as an unsigned
64-bit big-endian byte length and raw UTF-8 bytes. It must equal the digest of
the cursor's own head stream IDs. The complete encoded cursor is at most
32 KiB. The cursor digest used by gap records is lower-case SHA-256 of the
canonical decoded JSON bytes, not the base64 spelling.

Decode rejects invalid base64, non-canonical re-encoding, duplicate/unknown/
missing JSON fields, unsupported version, wrong Team, invalid UTF-8/control
characters, invalid digests, unsorted/duplicate heads, invalid zero/non-zero
head pairs, more than 96 heads, or over 32 KiB. Every non-zero head is then
validated by `ReadPageAfterHeads`.

A changed current `GlobalReadView.Version()` is normal. It replaces
`view_version` in the returned cursor after the current scope is unioned; it
does not invalidate a structurally and positionally valid cursor.

For an empty input cursor, every current-scope stream starts at zero. For a
valid supplied cursor, new scope streams start at zero and old scope streams
remain. The Journal inclusion predicate is exactly:

```text
event.seq > cursor_head[stream_id].sequence
```

The next cursor advances only over source Journal Events returned by the
bounded Journal page, including safe-vocabulary-unknown Events that produce no
delivery record. It never advances over an Event the Journal page did not
return. Thus a page may contain zero delivery records while still advancing a
validated source position.

## Authoritative delivery record v1

`DeliveryRecord` is immutable and encodes as compact JSON in this exact field
order:

```text
schema_version, delivery_id, kind, authority,
team_instance_id, logical_node_id, attempt_number,
source_stream_id, source_sequence, source_event_id,
occurred_at, cursor, payload
```

- `schema_version` is `1`;
- `authority` is exactly `journal` or `tentative`;
- authoritative occurrence time is the exact UTC Journal `emitted_at`;
- authoritative source fields are exact Journal identity;
- tentative source stream is empty, source sequence is Frame sequence, source
  Event ID is the Frame message ID, and cursor is the subscriber's last
  authoritative cursor;
- control source fields are empty/zero;
- one encoded record is at most 8 KiB.

`payload` is a fixed struct in this exact field order:

```text
status, reason_code, action, warning_code, retry_at,
text_delta, evidence_digest, cost
```

Every field is present. Non-applicable strings are empty. `retry_at` is empty
or canonical UTC RFC3339Nano. `evidence_digest` is empty or a lower-case
SHA-256. `cost` is exactly:

```text
observed, amount_microunits, currency
```

For all current Phase 1 facts `observed=false`,
`amount_microunits=null`, and `currency=""`; missing cost is never rendered as
zero. S5-W1 adds no cost inference.

Authoritative delivery ID is lower-case SHA-256 over tag
`loom.delivery.journal.v1` and length-prefixed:

```text
team_instance_id, kind, source_stream_id,
decimal source_sequence, source_event_id
```

The mapper emits only this closed Event-to-kind vocabulary:

| Journal Event | Delivery kind |
|---|---|
| `TeamExecutionPlanned` | `team_planned` |
| `TeamNodeAttemptScheduled` | `node_scheduled` |
| `TeamReadySetDispatched` | `ready_set_dispatched` |
| `TeamNodeAttemptRebound` | `node_rebound` |
| `RunStarted` | `run_started` |
| `RunTerminalCommitted` | `run_terminal` |
| `WorkItemApprovalPaused` | `approval_required` |
| `ApprovalRequested` | `approval_requested` |
| `ApprovalDecided` | `approval_decided` |
| `ApprovalExpired` | `approval_expired` |
| `WorkItemApprovalResolved` | `approval_resolved` |
| `TeamNodeAttemptTerminal` | `node_attempt_terminal` |
| `WorkItemReadyForReview` | `ready_for_review` |
| `WorkItemVerificationCommitted` | `verification_recorded` |
| `WorkItemDone` | `work_item_done` |
| `WorkItemRejected` | `verification_rejected` |
| `TeamNodeAcceptanceCommitted` | `node_acceptance` |
| `TeamNodeRecoveryRecorded` | `node_recovery` |
| `EvidenceSubmitted` | `evidence_available` |
| `TeamExecutionTerminal` | `team_terminal` |

Mapping occurs only after a successful Projection rebuild. It copies only the
fixed safe payload fields named above after strict JSON primitive/type checks.
All raw payload keys not explicitly mapped are ignored and never serialized.
Unknown Event types advance the Journal cursor but emit no delivery record.
Malformed known facts, impossible lineage, or a mapped record over 8 KiB fails
closed; raw JSON is never passed through.

The mapper derives exact node/attempt lineage from validated safe payload
fields and cross-checks it against the copied TeamExecution. Work/Run/
Approval/Evidence stream Events are cross-checked against their projected
records and Team relation. Empty node/attempt is permitted only for Team-level
records.

## Authorized tentative output

`TeamExecutionStream` implements the accepted `app.NodeOutputObserver`. It
accepts only a copied `supervisor.AuthorizedFrame` with bridge type
`MessageEvent` and exact payload:

```json
{"delta":"bounded UTF-8 text"}
```

The object has exactly one key, no duplicate/unknown keys, and a non-empty
delta of at most 2048 bytes. Control characters other than tab and newline are
rejected. Evidence/result/ack/heartbeat/cancel/dispatch Frames never become
tentative client output.

Before enqueue, the observer cross-checks:

- bound Team and logical node;
- positive exact attempt number;
- Frame WorkItem, Run, generation, Runtime, and Agent binding against that
  TeamExecution attempt;
- positive Frame sequence and non-empty canonical message identity; and
- `AuthorizedFrame.Tentative()==true`.

This is a defense-in-depth check after the accepted Adapter decode,
Supervisor sequence/binding validation, Grant authorization,
`BoundRunStream`, and durable private attempt capture. Verifier output is
excluded because verifier execution is never given this Team observer.

Tentative delivery ID is lower-case SHA-256 over tag
`loom.delivery.tentative.v1` and length-prefixed:

```text
team_instance_id, logical_node_id, decimal attempt_number,
work_item_id, run_id, decimal claim_generation,
frame message_id, decimal frame sequence
```

## Subscription and slow-consumer bounds

One Team stream supports at most 8 active subscribers. Each subscriber owns:

- at most 64 queued records;
- at most 64 KiB encoded tentative text/control state;
- one last authoritative cursor of at most 32 KiB; and
- at most one pending `tentative_overflow` gap.

`Subscribe(cursor)` validates the cursor and returns a subscription with
`Next(context.Context) (StreamItem, error)` and idempotent `Close()`.
`StreamItem.Delivery()` and `StreamItem.Gap()` return copies and exactly one
reports present. `Next` may block only the client goroutine and honors
cancellation. The observer performs bounded validation plus one
mutex-protected non-blocking enqueue/coalesce; it never waits on `Next` or a
channel send.

Adjacent queued tentative records coalesce only when Team, node, attempt, Run,
and generation match and their joined text remains at most 2048 bytes. The
coalesced record keeps the first delivery identity and occurrence time, the
last source Frame identity/sequence, and concatenated text. It remains
tentative.

When item or byte capacity would be exceeded, the new tentative delta is
dropped, no authoritative state changes, and one pending gap replaces any
older pending gap. A subscription returns an immutable `StreamItem` tagged
union whose copied accessor contains exactly one `DeliveryRecord` or one
`StreamGap`; the next successful `Next` yields that gap item before later
tentative delivery items. Subscriber absence, close, cancellation, slowness,
or overflow is never returned by `ObserveNodeOutput`; the method returns
`nil`. Only malformed input or impossible Team/attempt authority binding
returns `ErrInvalidNodeOutput`.

No subscriber state, acknowledgement, cursor, gap, or text delta is written to
Journal, Evidence, Sidecar, Projection, disk, or a daemon.

## Stream gap v1

A `StreamGap` is a separate immutable control value, not a Journal/tentative
`DeliveryRecord`. It encodes with exactly the Exit Contract fields:

```text
schema_version, delivery_id, kind="stream_gap", team_instance_id,
reason, previous_cursor_digest, current_view_version,
artifact_available, artifact_digest, recoverable=true, occurred_at
```

It has no raw error or path. Reason is exactly `invalid_cursor`,
`cursor_conflict`, `scope_overflow`, `page_overflow`, or
`tentative_overflow`. Artifact data is the latest accepted source or verifier
Evidence digest visible for the Team, selected by latest acceptance decision
time then WorkItem ID; absence is `artifact_available=false` plus empty digest.

Gap ID is lower-case SHA-256 over tag `loom.delivery.gap.v1` and
length-prefixed Team ID, reason, previous cursor digest, current view version,
artifact digest, and canonical UTC occurrence time. Cursor/request gaps use
the injected clock exactly once. Tentative overflow uses the dropped Frame
time. The gap is recoverable delivery control only.

## Read page, refresh, and continuity

`ReadPage(ctx, cursor, limit)` performs exactly:

1. reject invalid Team/cursor/limit before I/O where possible;
2. rebuild/read copied view A and derive the bounded current scope;
3. validate/decode cursor, union old/current scope, and read one Journal page
   in one transaction;
4. rebuild/read copied view B once after the page;
5. require view B to contain every returned Event at the exact stream
   sequence/Event ID and preserve Team binding;
6. derive view B board/Attention and map the returned page; and
7. return records, next cursor, `HasMore`, view B version, board, and Attention.

There is no hidden retry. A concurrent append after step 4 remains for the next
cursor page. A failed second rebuild leaves the Projection's prior published
view intact and returns state unavailable; it never publishes a mixed partial
view. Scope expansion in view B is added at zero to the returned cursor so it
is read next, without skipping facts.

Invalid cursor encoding produces `invalid_cursor`; exact head mismatch,
sequence gap, or view/page disagreement produces `cursor_conflict`; more than
96 union streams produces `scope_overflow`; invalid page limit produces
`page_overflow`. These return a typed gap carrying the current safe
board/Attention when available. No condition falls back to `ReadAll`.

## Board and Attention v1

`TeamBoard` contains exactly:

```text
schema_version, team_instance_id, plan_digest, status,
view_version, nodes, cost
```

Each node row contains exactly:

```text
logical_node_id, status, dependency_satisfied, current_attempt,
work_item_id, run_id, runtime_instance_id, agent_instance_id,
verification_status, recovery_action, retry_at
```

Rows are sorted by logical node ID and copied. The selected attempt is the
exact current attempt; missing optional IDs are empty. Cost uses the exact
unobserved representation above.

`AttentionItem` contains exactly:

```text
schema_version, attention_id, kind, severity,
team_instance_id, logical_node_id, work_item_id,
approval_request_id, runtime_instance_id, status,
occurred_at, action_required
```

Kinds are only `pending_approval`, `expired_approval`, `blocked`,
`human_required`, `retry_exhausted`, `runtime_offline`,
`verification_failed`, or `stream_gap`. Severity is `warning` or `critical`.
Action is a closed non-executing label: `review_approval`, `inspect_failure`,
`provide_input`, `restore_runtime`, or `reconnect`.

Attention is derived as follows:

- copied Approval status `pending` or `expired`;
- Team node status `blocked` or `human_required`;
- node with zero credits after a recovery decision and no retry/fallback;
- referenced Runtime status other than `online`;
- WorkItem verification/acceptance status `rejected` or recovery trigger
  `verification_rejected`; and
- the current page/subscription gap.

Identity is SHA-256 over tag `loom.attention.v1`, kind, Team/node/WorkItem/
Approval/Runtime identities, status, and authoritative occurrence identity.
Items sort by severity (`critical` first), occurred time, kind, and ID.
Duplicates collapse only by exact ID. Board/Attention never mutate state.

## CLI

The only new command is:

```text
loom timeline --state PATH --team TEAM
              [--cursor OPAQUE] [--limit 1..128]
```

Default limit is `128`. No positional arguments or other flags are accepted.
There is no `--follow`, daemon, socket, HTTP, WebSocket, Web, or TUI behavior.
The command is one finite read from an existing SQLite state file opened with
the accepted read-only/query-only URI.

Success writes one compact JSON object plus newline to stdout with exact fields:

```text
schema_version=1, command="timeline", team_instance_id,
view_version, next_cursor, has_more, gap, records, board, attention
```

`gap` is JSON `null` for a valid page or the exact `StreamGap` object for exit
4. Arrays are non-nil and deterministically ordered. Records are ordered by
source Journal order. `stderr` is empty.

Exit codes are:

- `0`: valid page;
- `2`: command/flag/Team/limit syntax or semantic input invalid before a
  recoverable cursor page exists;
- `3`: state file/open/rebuild/read/close unavailable, with only
  `state unavailable: unavailable` on stderr;
- `4`: recoverable stream gap, with the safe gap response on stdout and empty
  stderr.

No stdout/stderr contains a state path, raw SQL/JSON payload, Grant/token,
credential, prompt, hidden reasoning, raw artifact path, stack trace, or
underlying database error. Existing `route` and `status` output and exit codes
remain byte-for-byte unchanged.

## Public typed errors

`internal/journal` adds exactly:

- `ErrInvalidStreamCursor`
- `ErrStreamCursorConflict`
- `ErrStreamSequenceGap`
- `ErrStreamPageLimit`

`internal/api` adds exactly:

- `ErrInvalidTimelineRequest`
- `ErrTeamTimelineNotFound`
- `ErrInvalidTimelineCursor`
- `ErrTimelineCursorConflict`
- `ErrTimelineScopeOverflow`
- `ErrTimelinePageOverflow`
- `ErrInvalidDeliveryRecord`
- `ErrInvalidNodeOutput`
- `ErrTooManySubscribers`
- `ErrStreamGap`

Validation failures wrap the applicable sentinel. Journal invalid head shape
maps to `ErrInvalidTimelineCursor`; exact head mismatch or sequence gap maps to
`ErrTimelineCursorConflict`; stream/page bounds map to their exact API errors.
A gap error wraps `ErrStreamGap` and the specific cursor/scope/page sentinel
and exposes only copied safe gap/response values. Context cancellation and
deadline remain discoverable with `errors.Is`; database errors remain causes
internally but are sanitized by CLI. Subscriber cancellation is returned only
from `Subscription.Next`, never from the observer.

## Mandatory behavioral RED

Before any product edit, tests must compile against the frozen names and fail
because behavior is absent. RED evidence must prove failure in all four
vertical layers:

1. Journal after-head page exact-head validation, deterministic merge,
   no-jump cursor, contiguity, deep-copy, cancellation, 96/128 bounds, and
   concurrent append snapshot behavior;
2. selective view accessors, filtering/order/deep-copy, and failed-rebuild
   old-view preservation;
3. API cursor canonicality, current-scope union, safe mapping, tentative
   authority binding, verifier exclusion, queue/coalescing/overflow, gap,
   Attention, and board; and
4. CLI exact JSON/exit/read-only/sanitization/reconnect behavior.

RED may add only frozen tests/governance. No production behavior may be added
until the independent Contract Review passes and RED is recorded.

## GREEN and verification

The Candidate must pass:

```text
go test ./internal/journal ./internal/projection ./internal/api ./internal/app ./cmd/loom
go test -race -count=10 ./internal/journal ./internal/projection ./internal/api ./internal/app ./cmd/loom
go test ./internal/work ./internal/rules ./internal/verification ./internal/supervisor ./internal/runtime/piadapter
go test ./...
go test -race ./...
go vet ./...
gofmt -d <every frozen Go file>
git diff --check
GOOS=windows GOARCH=amd64 go test ./internal/journal ./internal/projection ./internal/api ./cmd/loom
```

Additional required evidence:

- cursor round-trip/property tests across deterministic stream permutations;
- at least 50 repeated disconnect/reconnect page walks with no loss/duplicate;
- at least 100 concurrent slow-consumer/overflow iterations under race;
- a private SQLite fixture proving external concurrent append is delivered on
  this or the next cursor without position fabrication;
- one accepted app coordinator fixture proving durable attempt capture occurs
  before tentative observer delivery and subscriber failure does not fail Run;
- malformed/old-generation/wrong-Team/verifier Frames never publish;
- known authoritative milestones remain recoverable after tentative overflow;
- Projection rebuild failure preserves the old view/version;
- CLI DB remains write-rejecting and command sources contain no SQL verbs;
- no new dependency, migration, executable, daemon, network listener, writer,
  or external activation; and
- contract-to-test traceability for every frozen schema, bound, error, mapping,
  exit code, and trust exclusion.

## Acceptance and exclusions

S5-W1 is accepted only after GREEN evidence, scope/trust/diff audit, a fresh
independent implementation Reviewer `PASS`, and one exact local atomic commit.
It does not close Slice 5 or Phase 1; only S5-W2 and the later whole-Slice
Review can reach `READY_FOR_FINAL_USER_SIGNOFF`.

S5-W1 explicitly does not add:

- a writer, CAS, state authority, acknowledgement authority, notification
  authority, or second Projection;
- direct SQL outside the accepted Journal/Projection reader;
- persisted tentative/token output or hidden reasoning;
- WorkPackages, Demo fixtures, installed Runtime execution, Provider/model
  traffic, credentials, network, daemon, resident process, external action,
  Web/TUI, push, merge, release, or user sign-off.

VERDICT: PASS
