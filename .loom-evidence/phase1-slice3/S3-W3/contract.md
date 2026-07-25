# S3-W3 Contract — AgentGrant Authority

- WorkItem: `S3-W3`
- Risk: `STRICT`
- Frozen branch: `codex/loom-platform-slice2`
- Frozen baseline: `5517a06`
- Date: `2026-07-26`
- Authority: `TECH-PLAN.md` sections 1.1, 5, 6, 8.1, 11.1, 13.1,
  14, and 15
- Depends on: accepted S3-W1 and S3-W2

## Purpose

Close one credential-like local security boundary for issuing, authorizing,
rotating, revoking, replaying, and projecting task-level AgentGrants. One
random plaintext token may be returned once to the daemon-controlled caller;
only its SHA-256 hash and frozen authorization boundary may enter the Event
Journal.

This is one vertical WorkItem. Token parsing/redaction, Run-generation
validation, Journal compare-and-append, codecs, authority methods, and
projection changes may not be split into new WorkItems.

## Owned files

Developer:

- `internal/authorization/authority.go`
- `internal/authorization/authority_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/grant_authority.go`
- `internal/projection/grant_authority_test.go`

Controller:

- `.loom-evidence/phase1-slice3/S3-W3/**`
- `docs/CURRENT.md`
- Controller-owned `PROGRESS.md` hunks

All other product, test, migration, protocol, policy, credential, Runtime
adapter, supervisor, evidence-store, root-governance, user-dirty, `.codex/**`,
and `.loom-drafts/**` files are locked.

No migration is required. Authoritative AgentGrant facts remain append-only
Events in the accepted Journal; the logical `agent_grants` schema in
`TECH-PLAN.md` is satisfied by replayable facts and a rebuildable read model.

## Frozen public surface

Package: `authorization`

```go
var (
    ErrInvalidGrantAuthorityInput error
    ErrGrantAuthorityConflict error
    ErrRunNotGrantable error
    ErrGrantAlreadyActive error
    ErrGrantNotFound error
    ErrGrantBindingMismatch error
    ErrGrantOperationDenied error
    ErrGrantExpired error
    ErrGrantRevoked error
    ErrGrantAlreadyRevoked error
    ErrGrantIDCollision error
    ErrGrantTokenCollision error
)

type Operation string

const (
    OperationBridgeAck Operation = "bridge.ack"
    OperationBridgeEvent Operation = "bridge.event"
    OperationBridgeEvidence Operation = "bridge.evidence"
    OperationBridgeResult Operation = "bridge.result"
    OperationBridgeHeartbeat Operation = "bridge.heartbeat"
    OperationContextRead Operation = "context.read"
    OperationEvidenceStage Operation = "evidence.stage"
)

type RevocationReason string

const (
    RevocationReplaced RevocationReason = "replaced"
    RevocationExpired RevocationReason = "expired"
    RevocationTerminal RevocationReason = "terminal"
    RevocationCancelled RevocationReason = "cancelled"
    RevocationTimeout RevocationReason = "timeout"
    RevocationOperator RevocationReason = "operator"
)

type Token

func ParseToken(string) (Token, error)
func (Token) Value() string
func (Token) String() string
func (Token) GoString() string

type IssueInput struct {
    WorkItemID string
    RunID string
    ClaimID string
    ClaimGeneration int64
    RuntimeInstanceID string
    AgentInstanceID string
    AllowedOperations []Operation
    Lifetime time.Duration
    CorrelationID string
}

type AuthorizeInput struct {
    Token Token
    WorkItemID string
    RunID string
    ClaimID string
    ClaimGeneration int64
    RuntimeInstanceID string
    AgentInstanceID string
    Operation Operation
    RequestID string
    CorrelationID string
}

type RevokeInput struct {
    GrantID string
    Reason RevocationReason
    CorrelationID string
}

type GrantRecord
type IssuedGrant
type AuthoritySnapshot
type Authority

func NewAuthority(
    *journal.Store,
    *work.Authority,
    func() time.Time,
    io.Reader,
) (*Authority, error)

func (*Authority) Issue(
    context.Context,
    IssueInput,
) (IssuedGrant, error)

func (*Authority) Authorize(
    context.Context,
    AuthorizeInput,
) (GrantRecord, error)

func (*Authority) Revoke(
    context.Context,
    RevokeInput,
) (GrantRecord, error)

func (*Authority) Snapshot(context.Context) (AuthoritySnapshot, error)

func (GrantRecord) ID() string
func (GrantRecord) WorkItemID() string
func (GrantRecord) RunID() string
func (GrantRecord) ClaimID() string
func (GrantRecord) ClaimGeneration() int64
func (GrantRecord) RuntimeInstanceID() string
func (GrantRecord) AgentInstanceID() string
func (GrantRecord) AllowedOperations() []Operation
func (GrantRecord) IssuedAt() time.Time
func (GrantRecord) ExpiresAt() time.Time
func (GrantRecord) RevokedAt() time.Time
func (GrantRecord) RevocationReason() RevocationReason

func (IssuedGrant) Record() GrantRecord
func (IssuedGrant) Token() Token

func (AuthoritySnapshot) Grants() []GrantRecord
```

No other exported `internal/authorization` symbol is permitted.

`Token.String` and `Token.GoString` return exactly `[REDACTED]`; only `Value`
reveals the immutable token for bounded process-environment or IPC handoff.
`Token` has no JSON or text marshaler. The zero value is invalid.

## Token and operation contract

1. `Issue` reads exactly 48 random bytes with `io.ReadFull`: 16 bytes form a
   canonical lowercase UUID Grant ID and 32 bytes form the token secret.
2. The token wire form is exactly
   `loom_grant_v1.<grant_uuid>.<43-character unpadded base64url secret>`.
   `ParseToken` accepts only that canonical bounded form and returns generic
   errors that never contain input bytes.
3. The persisted hash is lowercase hex SHA-256 over the complete token wire
   bytes. The raw token, secret, or reversible encoding never enters Event
   fields other than the in-memory return value, projection, error, log,
   Evidence, Bridge payload, or database bytes.
4. Grant ID or token-hash reuse against any historical Grant returns the
   corresponding typed collision error. There is no hidden randomness retry.
5. Allowed operations are non-empty, unique, canonically sorted, and drawn
   only from the seven frozen constants. They cannot authorize Provider/model
   access, credentials, Client sessions, daemon administration, policy
   changes, process launch, arbitrary filesystem access, or another Run.
6. `Lifetime` is positive and at most one hour. Clock output is non-zero UTC,
   read exactly once per operation, and must not overflow expiry.

## Run binding and linearization

The Authority consumes the accepted S3-W2 `work.Authority` only through
`Snapshot` and the same accepted Journal Store.

For every mutation it must:

1. read and retain the exact current `run/<run_id>` head;
2. read the accepted Run snapshot;
3. validate exact WorkItem, Run, Claim ID, generation, RuntimeInstance, and
   AgentInstance binding against the referenced Run fact; and
4. compare the retained Run head and `agent-grant/<run_id>` head in the same
   `AppendBatchIfStreamHeads` transaction.

Thus Grant-stream contenders are mutually exclusive, while a Grant mutation
racing a Run mutation is serializable: if the Run mutation commits first, the
Grant CAS fails with `ErrGrantAuthorityConflict`; if the Grant mutation commits
first, the later Run mutation may also succeed in that safe order because the
Grant fact does not advance the Run stream. No Grant decision may commit
against a Run transition that it already observed. There is no hidden retry.

`Issue` is allowed only while the exact current Run generation is `claimed`
and its prepare lease is unexpired. `Authorize` is allowed only while that
same generation is either claimed with an unexpired prepare lease or running
and non-terminal. Terminal, missing, unclaimed, expired-lease, mismatched, or
stale-generation Runs fail closed before authorization.

`Authorize` is not a read-only check. It appends one
`AgentGrantAuthorized` fact keyed by the canonical `RequestID`, making the
authorization decision linearizable against Run and Grant changes. Exact
request retry while the Grant remains valid returns the original decision;
after expiry, revocation, reclaim, or terminal it is denied and cannot renew
authority. Consumers must use the same RequestID as their own idempotency key.
RequestID is unique within the Run across all Grants and generations. Reuse is
accepted only for the exact same current valid Grant, binding, and operation;
all cross-Grant, cross-generation, or different-operation reuse fails closed
without a new fact.

## Stream and Event contract

Stream:

- `agent-grant/<run_id>`

All payloads are exact JSON objects with no unknown or missing fields. Schema
version is `1`; timestamps are UTC. Every Grant fact persists the exact
`run/<run_id>` stream, sequence, and Event ID observed by the transaction.
Event IDs and idempotency keys are deterministic from command identity;
correlation and causation are preserved.

Required facts:

1. `AgentGrantIssued` stores Grant ID, token hash, exact Run binding, sorted
   operations, issued/expiry timestamps, and exact Run-head reference.
2. `AgentGrantAuthorized` stores Grant ID, exact binding, one allowed
   operation, RequestID, authorization timestamp, and exact Run-head
   reference. It never stores a token or token hash.
3. `AgentGrantRevoked` stores Grant ID, frozen reason, revoked timestamp, and
   exact Run-head reference. It never stores a token or token hash.
4. At most one unexpired, unrevoked Grant is active for a Run. Issuing for a
   new generation atomically revokes the old active Grant as `replaced`.
   Issuing after expiry in the same generation atomically revokes the old
   Grant as `expired`. An unexpired active Grant in the same generation returns
   `ErrGrantAlreadyActive`.
5. Explicit `Revoke` derives binding from the persisted Grant rather than
   caller input. It may revoke a current or historical generation and remains
   allowed after Run terminal so cleanup can be recorded. Exact same retry
   returns the same record; a different second revocation returns
   `ErrGrantAlreadyRevoked`.
6. `Authorize` hashes the presented token and compares in constant time,
   requires exact Grant/binding/operation/current generation, and rejects
   unknown, malformed, expired, revoked, stale, terminal, or wrong-operation
   requests with zero authorization fact.

Every failed operation returns zero records/token, appends nothing, consumes no
clock or randomness before all cheaper input/context checks, and preserves the
last successful snapshot.

## Projection contract

Extend the accepted `projection.Snapshot` with:

```go
AgentGrants map[string]AgentGrant

type AgentGrant struct {
    ID string
    WorkItemID string
    RunID string
    ClaimID string
    ClaimGeneration int64
    RuntimeInstanceID string
    AgentInstanceID string
    AllowedOperations []string
    IssuedAt time.Time
    ExpiresAt time.Time
    RevokedAt time.Time
    RevocationReason string
}
```

Projection replay must:

- run after accepted WorkItem/Run replay and validate every exact Run-head
  reference against historical accepted Run facts;
- enforce unique Grant IDs and token hashes, one active Grant per Run,
  monotonic stream sequence, exact binding, canonical operations and times;
- reject issue outside claimed/unexpired prepare state, authorize after
  expiry/revocation/reclaim/terminal or for a disallowed operation, and
  duplicate/conflicting authorization or revocation;
- discard token hashes after validation and never expose them in Snapshot;
- preserve the previous Snapshot on any rebuild failure; and
- deeply copy all new maps, records, and operation slices.

Existing Mode, Evidence, Team, AgentInstance, Runtime, WorkItem, and Run
projection behavior remains unchanged.

## Mandatory RED

Before any owned product file changes, add complete tests and capture failure
only on missing frozen S3-W3 symbols/behavior. Required exact markers:

```text
s3_w3_issue_hash_only_run_cas
s3_w3_authorize_binding_operation_expiry
s3_w3_rotate_revoke_generation
s3_w3_concurrency_collision_mutation
s3_w3_projection_rebuild_failure_isolation
s3_w3_token_redaction_fuzz_static
```

## Required proof

1. Token canonical parse/redaction/value isolation, exact 48-byte randomness,
   error/short-reader behavior, clock behavior, collision behavior, and
   mutation isolation.
2. Exact Event envelope/payload/order/causation and hash-only SQLite bytes for
   issue, authorize, automatic rotation, and explicit revoke.
3. Complete invalid input, malformed source, missing/wrong/expired/terminal
   Run, binding, operation, token, Grant state, head-conflict, cancellation,
   query, insert, and commit-failure matrix.
4. Real temporary SQLite races for concurrent same-Run issue,
   authorize-versus-revoke, authorize-versus-reclaim/start/terminal, and
   token/Grant collision. Same-Grant-stream contenders commit exactly one
   decision. Run-versus-Grant contenders prove either Run-first with Grant CAS
   failure or Grant-first followed by the legal Run transition; no Grant fact
   may reference the post-transition Run head while authorizing the
   pre-transition state. No partial facts are permitted.
5. Reclaim rotates and revokes the prior-generation Grant; expired same-
   generation rotation works; late old token, wrong binding, and cross-Run
   token use fail closed.
6. Real Journal close/reopen and rebuild, deterministic ordering, malformed
   Grant/Run reference rejection, no hash exposure in projection, and
   previous-snapshot preservation.
7. Concurrent snapshot reads and accessor mutation isolation; fuzz token and
   Grant Event replay never panics.
8. Static proof: no process, filesystem workspace, network, goroutine,
   Provider/model, credential resolution, Bridge payload transport, Runtime
   activation, Client/Daemon identity, policy expansion, or Slice 4 authority.

## Verification

```text
go test ./internal/authorization ./internal/projection -count=1
go test -race ./internal/authorization ./internal/projection -count=30
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./internal/authorization -run '^$' \
  -fuzz '^FuzzGrantTokenAndReplayNeverPanic$' -fuzztime=5s
gofmt -d internal/authorization/authority.go \
  internal/authorization/authority_test.go \
  internal/projection/projection.go internal/projection/projection_test.go \
  internal/projection/grant_authority.go \
  internal/projection/grant_authority_test.go
git diff --check
```

No dependency audit is required unless a dependency is added, which this
contract forbids.

## Explicit exclusions

S3-W3 creates no Provider CredentialGrant, Broker, secret-store access,
Client/Daemon token, workspace, file/process group, Runtime Adapter, Bridge
transport, Agent/model execution, scheduler, heartbeat supervisor, Evidence
artifact publication, approval, Verifier, Team DAG, UI, resident daemon
activation, or Slice 4 behavior.

It uses no installed user Runtime, model, credential, ambient environment,
network, or long-running process. Fresh independent Contract Review must
return `PASS` before RED; fresh independent Implementation Review must return
`PASS` before acceptance or local commit.

VERDICT: PASS
