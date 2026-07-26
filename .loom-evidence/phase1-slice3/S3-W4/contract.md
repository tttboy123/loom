# S3-W4 Contract — Managed Workspace Runtime Adapter and Supervisor

- WorkItem: `S3-W4`
- Frozen baseline: `47b4b50`
- Date: `2026-07-26`
- Parent authority:
  `.loom-evidence/phase1-slice3/EXIT-CONTRACT.md`
- Capability: one complete managed filesystem/process execution boundary

This WorkItem is the one permitted vertical boundary for managed workspace,
configured stdio execution, Bridge session semantics, cancellation/timeout,
process cleanup, Run terminal generation, and AgentGrant revocation. It must
not be decomposed into adapter-, writer-, coordinator-, trigger-, or
scheduler-only WorkItems.

## Accepted prerequisites

S3-W4 consumes without reopening:

- S3-W1 exact Bridge v1 `Frame`, `BoundRunStream`, encoding, and decoding;
- S3-W2 Run generation, prepare lease, start, terminal, capacity, and
  projection authority;
- S3-W3 AgentGrant issue, per-frame authorization, expiry, and revocation;
- Slice 2 validated RuntimeProfile/RuntimeInstance binding.

No S3-W1, S3-W2, S3-W3, Journal, StateWriter, projection, AgentDefinition,
Team, or Evidence authority may be duplicated.

## Owned files

Product and tests:

- `internal/supervisor/managed_execution.go`
- `internal/supervisor/managed_workspace.go`
- `internal/supervisor/managed_execution_test.go`
- `internal/supervisor/managed_workspace_test.go`
- `internal/runtime/piadapter/execution_adapter.go`
- `internal/runtime/piadapter/execution_adapter_test.go`
- `internal/runtime/piadapter/execution_process_unix.go`
- `internal/runtime/piadapter/execution_process_other.go`

Governance only:

- `.loom-evidence/phase1-slice3/S3-W4/**`
- Controller-owned S3-W4 hunks in `docs/CURRENT.md` and `PROGRESS.md`

No other product file is owned. If an accepted prerequisite is insufficient,
freeze and review one amendment before changing it; do not silently expand
scope.

## Public API

Package `internal/supervisor`:

```go
var (
    ErrInvalidManagedExecution = errors.New("invalid managed execution")
    ErrManagedWorkspace = errors.New("managed workspace failure")
    ErrSourceChanged = errors.New("source changed during managed execution")
    ErrRuntimeAdapter = errors.New("runtime adapter failure")
    ErrBridgeSession = errors.New("bridge session failure")
    ErrRuntimeTimeout = errors.New("managed execution timeout")
    ErrRuntimeCancelled = errors.New("managed execution cancelled")
    ErrProcessCleanup = errors.New("managed process cleanup failed")
)

const (
    WorkspaceChangeAdded WorkspaceChangeKind = "added"
    WorkspaceChangeModified WorkspaceChangeKind = "modified"
    WorkspaceChangeDeleted WorkspaceChangeKind = "deleted"
)

type WorkspaceChangeKind string

type RuntimeAdapter interface {
    AdapterType() string
    RuntimeInstanceID() string
    Execute(context.Context, AdapterRequest) (AdapterResult, error)
}

type AdapterRequest struct {
    WorkspacePath string
    HomePath string
    TempPath string
    Binding bridgev1.RunStreamBinding
    Dispatch bridgev1.Frame
    Grant authorization.Token
}

type AdapterResultInput struct {
    InboundFrames []bridgev1.Frame
    Stderr []byte
    ExitCode int
    DispatchAcknowledged bool
    ResultAcknowledged bool
    CancelAcknowledged bool
}

type AdapterResult

func NewAdapterResult(AdapterResultInput) (AdapterResult, error)
func (AdapterResult) InboundFrames() []bridgev1.Frame
func (AdapterResult) Stderr() []byte
func (AdapterResult) ExitCode() int
func (AdapterResult) DispatchAcknowledged() bool
func (AdapterResult) ResultAcknowledged() bool
func (AdapterResult) CancelAcknowledged() bool

type Config struct {
    WorkspaceRoot string
    CleanupTimeout time.Duration
}

type ExecuteInput struct {
    SourcePath string
    Profile runtime.RuntimeProfile
    Instance runtime.RuntimeInstance
    Generation work.RunGenerationInput
    Grant authorization.IssuedGrant
    Dispatch bridgev1.Frame
}

type WorkspaceChange

func (WorkspaceChange) Path() string
func (WorkspaceChange) Kind() WorkspaceChangeKind
func (WorkspaceChange) Mode() fs.FileMode
func (WorkspaceChange) Digest() string
func (WorkspaceChange) Content() []byte

type Outcome

func (Outcome) WorkItem() work.WorkItemRecord
func (Outcome) Run() work.RunRecord
func (Outcome) Stream() bridgev1.BoundRunStream
func (Outcome) SourceDigest() string
func (Outcome) WorkspaceDigest() string
func (Outcome) Changes() []WorkspaceChange
func (Outcome) Stderr() []byte

type Supervisor

func New(
    Config,
    *work.Authority,
    *authorization.Authority,
    RuntimeAdapter,
) (*Supervisor, error)

func (*Supervisor) Execute(context.Context, ExecuteInput) (Outcome, error)
```

Package `internal/runtime/piadapter`:

```go
var (
    ErrInvalidPiExecutionAdapter = errors.New("invalid Pi execution adapter")
    ErrPiExecutionBindingChanged = errors.New("Pi execution binding changed")
    ErrPiExecutionProcess = errors.New("Pi execution process failed")
    ErrPiExecutionProtocol = errors.New("Pi execution protocol failed")
    ErrPiExecutionOutputTooLarge = errors.New("Pi execution output too large")
    ErrPiExecutionCleanup = errors.New("Pi execution cleanup failed")
)

type PiExecutionAdapterConfig struct {
    ExecutablePath string
    Arguments []string
    RuntimeInstanceID string
    RuntimeSearchPaths []string
    CancelGrace time.Duration
    Now func() time.Time
    Random io.Reader
}

func NewPiExecutionAdapter(
    PiExecutionAdapterConfig,
) (supervisor.RuntimeAdapter, error)
```

No additional exported symbol is permitted without a reviewed amendment.
Public results and mutable slices are deeply copied. Zero values never carry
authority.

## Managed workspace and source contract

Limits are fixed in product code:

- at most 4,096 regular source entries;
- at most 64 MiB total source bytes;
- at most 1 MiB per source file;
- at most 1,024 changed entries;
- at most 16 MiB total returned changed-file content;
- at most 8 MiB per changed file.

The exact flow is:

1. Validate non-nil context and an absolute, clean source directory path.
2. Reject a symlink source root and any symlink, device, socket, FIFO,
   hardlinked regular file, or other non-regular/non-directory entry below it.
   Unix regular files require exact link count one, a no-follow open, exact
   pre-open path versus descriptor device/inode/type/link identity, root
   containment, and unchanged identity/mode/size after the bounded read.
   Unsupported or uncertain identity proof fails closed.
3. Ignore only a top-level `.git` file or directory. It is neither copied nor
   made visible to the child.
4. Compute a deterministic SHA-256 source manifest over sorted slash-separated
   relative paths, entry kind, normalized executable bit, byte length, and
   file-content digest.
5. Require the configured workspace root to be an absolute, clean,
   non-symlink directory with exact `0700` permissions and stable bound
   identity.
6. Create one random per-call invocation directory at `0700` containing
   sibling `workspace`, `home`, and `tmp` directories at `0700`.
7. Copy proven single-link regular source content into `workspace`;
   directories are `0700`,
   regular files are `0600` or `0700` when the source executable bit is set.
   No hard link, symlink, socket, device, xattr, ACL, `.git`, ambient home, or
   credential material is copied.
8. Recompute the original source manifest immediately before Run start. Any
   difference fails with `ErrSourceChanged` before process execution.
9. After execution stops, recompute the original source manifest again and
   fail terminal as `source_changed` on any difference.
10. Only after the child process group is stopped and reaped, compute the final
    managed-workspace manifest and a sorted immutable change set against the
    initial copy. Every returned regular file repeats the exact Unix
    single-link/no-follow/identity proof. Added/modified changes include
    bounded content; deleted changes have no content.
11. Always remove the entire invocation directory after collecting the
    immutable outcome. Cleanup failure joins `ErrProcessCleanup`; no success is
    reported while cleanup is unproved.

Source digest excludes only top-level `.git`; that entry is never opened.
Therefore an unrelated external
source edit is conservatively treated as a failure. S3-W4 provides
least-exposure cwd/environment and detects source mutation. It does not claim a
kernel filesystem or network sandbox, chroot, container, or protection against
a malicious executable that independently knows another absolute path.

Unix is the supported managed-execution platform for S3-W4. Non-Unix builds
compile but fail closed before source copy or process start because the
single-link/no-follow and process-group proofs are unavailable.

## Configured Pi execution adapter

The production adapter executes only the constructor-bound absolute
executable. Construction:

- resolves symlinks once and binds regular-file identity, size, mode, and
  SHA-256 bytes;
- rejects a non-executable, mutable-shape, oversized, relative, or non-clean
  executable;
- binds each configured absolute search directory by non-symlink identity;
- validates immutable copied arguments, a canonical RuntimeInstance ID,
  `CancelGrace > 0 && <= 5s`, a non-zero UTC clock, and a non-nil random
  source;
- rejects NULs and all protected/ambient environment injection.

Before every execution it revalidates executable and search-directory
bindings. The child receives exactly:

- the configured executable and copied argument vector;
- `Dir = WorkspacePath`;
- `HOME = HomePath`;
- `TMPDIR = TempPath`;
- `PATH` built only from configured bound search paths;
- `LANG=C.UTF-8`, `LC_ALL=C.UTF-8`;
- `LOOM_AGENT_GRANT=<raw current token>`.

No ambient `HOME`, `PATH`, shell, environment, Client/Daemon identity,
credential, Provider key, user Runtime config, or inherited file descriptor is
used. The Grant is never placed in arguments, stdout, stderr, Bridge payload,
errors, Events, Evidence, or returned values.

Stdin/stdout are Bridge protocol only. Stderr is separate and bounded to
256 KiB. A stdout line uses S3-W1's one-line and buffer limits. Any diagnostic,
malformed/oversized line, EOF before terminal result, non-zero exit after a
successful result, or stdout after terminal result fails closed.

On Unix, the adapter owns a new child process group. On cancel, timeout,
protocol failure, output overflow, or caller failure it:

1. attempts the required Bridge cancel handshake when the protocol is still
   writable;
2. sends `SIGTERM` to the process group;
3. waits at most `CancelGrace`;
4. sends `SIGKILL` to the entire group if still alive;
5. waits/reaps the direct child and closes all pipes.

Non-Unix builds must compile and provide bounded direct-child cleanup, but only
Unix controlled evidence can claim process-group cleanup. Cleanup errors are
never hidden by the primary error.

## Bridge session contract

Process direction defines trust: Supervisor-to-child frames are outbound;
child stdout frames are inbound. Because accepted Bridge v1 has one bound
AgentInstance field and no direction field, both directions use the exact
current Run AgentInstance binding; fd direction is not inferred from the
field.

The session is:

1. Validate the caller's outbound `dispatch` Frame: exact generation binding,
   type `dispatch`, sequence `1`, and exact JSON object payload.
2. Write its exact encoded line to child stdin.
3. Require the first inbound Frame to be `ack`, sequence greater than `1`, and
   exact payload `{"message_id":"<dispatch message UUID>"}`.
4. Accept zero or more inbound `event`, `evidence`, and `heartbeat` Frames.
5. Require exactly one inbound terminal `result` with exact payload:

   ```json
   {"status":"succeeded|failed","reason":"string"}
   ```

   `succeeded` requires empty reason; `failed` requires one bounded opaque
   reason. `done`, `ready_for_review`, unknown fields, duplicate keys, missing
   fields, and trailing values are rejected.
6. Send one outbound `ack` for the result before normal child exit.
7. Reject any inbound `dispatch`, `cancel`, second result, frame after result,
   duplicate message ID, non-increasing sequence, binding mismatch, or missing
   required acknowledgment.

Outbound `ack`/`cancel` payload is exactly
`{"message_id":"<obligation UUID>"}` for ack and
`{"reason":"cancelled|timeout"}` for cancel. Generated outbound frames use
cryptographic UUID-v4 IDs, one clock read, the same correlation and binding,
and a sequence strictly greater than the last observed sequence. Inbound
sequences may skip those outbound values; S3-W1 already permits gaps.

The adapter returns immutable inbound frames only. Supervisor reconstructs one
S3-W1 `BoundRunStream` and, before accepting each inbound frame, calls S3-W3
`Authorize` using:

| Frame type | AgentGrant operation | RequestID |
|---|---|---|
| `ack` | `bridge.ack` | Frame `message_id` |
| `event` | `bridge.event` | Frame `message_id` |
| `evidence` | `bridge.evidence` | Frame `message_id` |
| `result` | `bridge.result` | Frame `message_id` |
| `heartbeat` | `bridge.heartbeat` | Frame `message_id` |

An unauthorized frame causes no Run success and no workspace output
acceptance.

## Supervisor state transition contract

`Execute` validates all input and immutable bindings before side effects:

- accepted RuntimeProfile and RuntimeInstance plus
  `runtime.ValidateBinding`;
- adapter type and RuntimeInstance ID equal both catalog records and generation;
- dispatch, Grant record/token, Run, WorkItem, claim, generation, Runtime,
  AgentInstance, and required `bridge.ack`/`bridge.result` permissions all
  match;
- the Run snapshot is exactly current, `claimed`, and within prepare lease.

One Supervisor instance permits at most one active execution per Run ID.

The exact successful path is:

```text
prepare private workspace and initial manifest
→ recheck source unchanged
→ work.Authority.Start(current generation)
→ execute configured adapter under Profile.Timeout
→ validate/authorize every inbound Frame into one BoundRunStream
→ require dispatch ACK + one result + result ACK + zero process exit
→ recheck source unchanged and collect bounded workspace changes
→ work.Authority.CommitTerminal(result status/reason)
→ authorization.Authority.Revoke(reason=terminal)
→ cleanup invocation root
→ return immutable Outcome
```

A child may report Run terminal `succeeded` with empty reason or `failed` with
one bounded opaque reason. `succeeded` is projected by S3-W2 only to WorkItem
`ready_for_review`; neither child result can mark the WorkItem `done`.
`cancelled` remains Supervisor-generated only.

Failure mapping is deterministic:

| Cause | Run status | Run reason | Grant revoke reason |
|---|---|---|---|
| caller cancellation | `cancelled` | `operator_cancelled` | `cancelled` |
| Profile deadline | `failed` | `runtime_timeout` | `timeout` |
| original source changed | `failed` | `source_changed` | `terminal` |
| Bridge/session violation | `failed` | `bridge_protocol_failed` | `terminal` |
| process start/exit failure | `failed` | `runtime_process_failed` | `terminal` |
| workspace/bounds failure after Start | `failed` | `workspace_failed` | `terminal` |
| cleanup failure | preserve prior status if already committed; return joined `ErrProcessCleanup` | preserve | preserve |

After Run start, every failure attempts one current-generation terminal commit
and one Grant revoke. Caller cancel/timeout cleanup and authority writes use an
internal context bounded by `CleanupTimeout` (`>0 && <=30s`) rather than the
already-cancelled execution context. There is no hidden retry.

Terminal commit and Grant revoke are two existing authorities, not a fabricated
cross-authority transaction. If terminal commits and revocation fails, return
the exact terminal Outcome plus the revocation error; retrying exact Revoke is
safe. If terminal commit fails, still attempt fail-closed revocation and return
joined errors. Stale generation/start rejection never permits the old
generation to commit terminal; its Grant is still revoked when safely
identifiable.

## Security and activation boundary

- Raw Grant disclosure tests cover arguments, formatting, JSON, stderr,
  returned result, workspace, source, and SQLite.
- Executable/source/workspace bindings are checked before execution.
- All untrusted paths, bytes, lines, frames, payloads, counts, times, and
  process waits are bounded.
- Child stdout cannot directly mutate Journal, Run, Grant, projection, or
  Evidence state.
- No shell, network client/listener, Provider/model SDK, credential access,
  user home, package manager, daemon loop, scheduler, service install, or
  ambient Runtime discovery is added.

Product code is a real configured adapter. Tests and controlled live evidence
use only a deterministic temporary Bridge-compatible fixture executable in a
private temporary root. Installed Pi, user Pi state, credentials, Provider/
model calls, resident daemon activation, and real task execution remain
unauthorized.

## Mandatory RED

All complete tests must exist before product implementation and fail only on
missing S3-W4 symbols/behavior. Required exact markers:

```text
s3_w4_workspace_copy_digest_changes_cleanup
s3_w4_adapter_config_env_binding
s3_w4_bridge_dispatch_ack_result
s3_w4_grant_frame_authorization
s3_w4_cancel_timeout_process_group
s3_w4_terminal_failure_revoke
s3_w4_stale_generation_source_changed
s3_w4_bounds_failure_fuzz_static
```

## Required proof

1. Workspace root/source validation, permissions, `.git` exclusion, stable
   digest, executable-bit normalization, exact sorted change content, Unix
   single-link/no-follow/device/inode proof, source and child-created hardlink
   rejection, symlink-race/special-file rejection, mutation isolation, bounds,
   non-Unix fail-closed behavior, and cleanup.
2. Configured executable/search-path identity and digest revalidation,
   immutable args, exact environment, nil stdin inheritance, no shell, and
   no ambient env.
3. Real temporary executable dispatch/ack/event/evidence/heartbeat/result/
   result-ack session with exact frame order, both child `succeeded` and
   `failed` terminal projection, and one S3-W3 authorization fact per inbound
   frame.
4. Malformed/oversized/diagnostic stdout, stderr overflow, duplicate/out-of-
   order/mismatched/after-result frames, missing ack/result, nonzero exit, and
   raw-Grant leak matrix.
5. Cancellation before start, during workspace, during child execution, after
   dispatch, and after result; timeout; cancel ACK/no-ACK; process plus
   grandchild cleanup; exact terminal/revocation facts and no orphan directory.
6. Adapter failure, source-change race, stale generation, expired lease,
   Runtime offline/mismatch, terminal/revoke partial failures, idempotent
   authority behavior, and user-dirty isolation.
7. Real SQLite reopen/projection proof for started and every terminal class.
8. Concurrent same-Run execution one-winner proof and unrelated-Run isolation.
9. Fuzz malformed stdout/session payload and hostile workspace trees; no panic,
   unbounded allocation, path escape, or raw-token disclosure.
10. Static AST/import proof of the exact owned boundary and forbidden network,
    shell, credential, Provider/model, daemon, scheduler, service, and ambient
    environment capabilities.

## Verification

```text
go test ./internal/supervisor ./internal/runtime/piadapter -count=1
go test -race ./internal/supervisor ./internal/runtime/piadapter -count=30
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./internal/supervisor -run '^$' \
  -fuzz '^FuzzManagedWorkspaceAndSessionNeverPanic$' -fuzztime=5s
gofmt -d <all S3-W4 owned Go files>
git diff --check
```

No dependency may be added. Fresh independent Contract Review must return
`PASS` before mandatory RED. Fresh independent Implementation Review must
return `PASS` before acceptance or local commit.

## Explicit exclusions

S3-W4 does not add Team DAG scheduling, Main/SubAgent aggregation, Evidence
metadata persistence, rules/approval/Verifier, Provider credential brokerage,
network sandboxing, containerization, resident daemon activation, automatic
retry/reclaim, installed user Runtime execution, Phase 2, push, merge, release,
or publication. Those boundaries remain closed.

VERDICT: FROZEN
