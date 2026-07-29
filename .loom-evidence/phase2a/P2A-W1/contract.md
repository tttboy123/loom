# P2A-W1 Local App Shell and Read Experience Contract

**Date**: 2026-07-28
**Status**: FROZEN — fresh independent Contract Review PASS
**Parent**: frozen Phase 2A Local Product Experience Exit Contract
**Depends on**: ADR-0011 and commit `3a2c3da`

## 1. Vertical capability

P2A-W1 delivers one runnable, read-oriented Loom product:

- `loom` with no arguments opens the Bubble Tea TUI;
- the TUI and ordinary read-only CLI commands use a typed client over a private,
  versioned Unix-domain-socket API;
- the daemon API calls the accepted Journal/Projection/timeline read services;
- Home, Runtimes, Teams, Runs, History, Evidence, Compare, Attention, and Team
  timeline are usable without a SQLite path, Team ID, cursor, or
  service-manager command;
- client exit, reconnect, stale cursor, daemon restart, projection failure, and
  legacy Phase 1 canary timeline behavior are explicit and fail closed.

W1 is read-only. It does not create or confirm Teams, configure Provider
credentials, create WorkPackages, dispatch Runs, approve work, retry execution,
or mutate authoritative state.

No W1a/W1b, screen, IPC, adapter, launcher, or compatibility WorkItem may be
created. All work below remains inside P2A-W1.

## 2. Baseline and exclusions

- Branch: `codex/loom-platform-slice2`
- Baseline commit: `3a2c3da5f1682e1acabfebefb6c55e5a57183a20`
- Existing Runtime observer LaunchAgent:
  `com.earendilworks.loom.runtime-observer`
- The LaunchAgent remains running and unchanged through Contract Review, RED,
  implementation, deterministic verification, and Implementation Review.
- Pre-existing shared-worktree modifications and untracked files remain
  user-owned and excluded.

The current LaunchAgent does not expose a product socket. W1 deterministic
tests therefore use isolated fixture daemons. A controlled resident-daemon
upgrade is permitted only after Implementation Reviewer `PASS` under section
16.

## 3. Exact owned files

### Existing files

W1 may modify only:

1. `go.mod`
2. `go.sum`
3. `cmd/loom/query.go`
4. `cmd/loom/main_test.go`
5. `cmd/loomd/run.go`
6. `cmd/loomd/run_test.go`
7. `internal/api/team_execution_stream.go`
8. `internal/api/team_execution_stream_test.go`
9. `internal/projection/global_read_view.go`
10. `internal/projection/global_read_view_test.go`
11. `docs/CURRENT.md`

### New product files

W1 may create only:

1. `cmd/loom/tui.go`
2. `cmd/loomd/product_daemon.go`
3. `cmd/loomd/product_daemon_test.go`
4. `internal/api/local_product_read.go`
5. `internal/api/local_product_read_test.go`
6. `internal/localipc/protocol.go`
7. `internal/localipc/protocol_test.go`
8. `internal/localipc/socket.go`
9. `internal/localipc/socket_test.go`
10. `internal/localipc/peercred_darwin.go`
11. `internal/localipc/peercred_linux.go`
12. `internal/localipc/peercred_unsupported.go`
13. `internal/localipc/server.go`
14. `internal/localipc/server_test.go`
15. `internal/localipc/client.go`
16. `internal/localipc/client_test.go`
17. `internal/tui/model.go`
18. `internal/tui/model_test.go`
19. `internal/tui/program.go`
20. `internal/tui/program_test.go`
21. `scripts/install-loom-local-product.sh`
22. `scripts/test-install-loom-local-product.sh`

### New evidence files

W1 may create files only under:

```text
.loom-evidence/phase2a/P2A-W1/
```

No other file is owned. In particular, W1 does not own Journal migrations,
StateWriter, Team creation, WorkPackage, Scheduler, Supervisor, authorization,
Credential Broker, Runtime adapters, Evidence authority, accepted ADRs, Phase 1
evidence, the current LaunchAgent plist, or installed binaries before the
post-Review live gate.

An unexpected need to modify an unowned product file stops for a reviewed
contract amendment. It does not create another WorkItem.

## 4. Dependency lock

W1 adds exactly one direct dependency:

```text
github.com/charmbracelet/bubbletea v1.3.4
```

The lock is:

- upstream repository:
  `https://github.com/charmbracelet/bubbletea`
- tag: `refs/tags/v1.3.4`
- resolved commit: `bf1216dfaf642b73b639262ab91a7e7c86095d34`
- module checksum:
  `h1:kCg7B+jSCFPLYRA52SDZjr51kG/fMUEoPoZrkaDHyoI=`
- `go.mod` checksum:
  `h1:dtcUCyCGEX3g9tosuYiut3MXgY/Jsv9nKVdibKKRRXo=`
- upstream module language floor: Go 1.18

Bubble Tea v2 and Bubble Tea v1.3.5+ are excluded because their current module
metadata raises the language/toolchain floor above Loom's declared Go 1.22
baseline. W1 may not change Loom's `go` directive.

Dependency materialization uses direct GitHub VCS bytes and the signed public Go
checksum record. The controlled network may use
`sum.golang.google.cn` only as an alternate endpoint for the same signed
`sum.golang.org` transparency record when the default endpoint is unreachable.
No unverified module mirror, `GONOSUMDB`, `GOSUMDB=off`, `replace`, vendored
fork, Bubbles component library, Lip Gloss direct dependency, or extra TUI
library is permitted.

The exact transitive dependency diff must be reviewed after `go mod tidy`.

## 5. Dependency direction

The only new production direction is:

```text
cmd/loom ─> internal/tui ─> internal/localipc client
cmd/loomd ─> internal/localipc server ─> internal/api read service
internal/api read service ─> projection + journal + existing timeline API
internal/tui ─> Bubble Tea
```

Forbidden directions:

- `internal/tui` importing `database/sql`, SQLite, Journal, Projection,
  StateWriter, Scheduler, Supervisor, Runtime adapters, or shell execution;
- `internal/localipc` importing CLI packages or writing authoritative state;
- `internal/api` importing TUI or transport packages;
- CLI/TUI parsing each other's output;
- daemon handlers constructing Journal facts or calling a StateWriter.

## 6. Bounded Projection read surface

`GlobalReadView` gains stable, copied, bounded enumeration methods only for the
records required by W1:

```go
Teams(afterID string, limit int) ([]projection.TeamInstance, bool)
TeamExecutions(afterID string, limit int) ([]projection.TeamExecution, bool)
Runs(afterID string, limit int) ([]projection.Run, bool)
EvidenceRecords(afterID string, limit int) ([]projection.Evidence, bool)
RuntimeInstances(afterID string, limit int) ([]projection.RuntimeInstance, bool)
```

Rules:

- `limit` is `1..64`; invalid input returns an empty non-nil slice and
  `hasMore=false`;
- records sort by canonical ID ascending;
- `afterID` is exclusive and must be empty or a valid bounded identifier;
- only the selected page is copied;
- nested slices are deep copied;
- mutation of returned records cannot mutate the published view;
- enumeration never exposes heads, raw Event payloads, Grant tokens, prompts,
  or credential material;
- the GlobalReadView version remains the canonical stream-head/event-ID digest.

No second projection table or cache is added.

## 7. Local product read service

`internal/api.LocalProductReadService` is the typed, transport-independent read
boundary shared by the TUI client and ordinary CLI reads.

### Configuration

The production service is built inside `loomd` from:

- the daemon-owned state path;
- a query-only SQLite connection with foreign keys and a five-second busy
  timeout;
- one `journal.Store`;
- one `projection.Projection`;
- a UTC clock used only for existing timeline gap records.

It does not migrate, append, update, or delete.

### Snapshot query

```go
ReadLocalProductSnapshot(
    context.Context,
    LocalProductSnapshotRequest,
) (LocalProductSnapshot, error)
```

The request contains only:

- `after_team_id`
- `after_runtime_id`
- `after_run_id`
- `after_evidence_id`
- one common `limit` in `1..64`

The result contains:

- schema version `1`;
- GlobalReadView version;
- partial/stale flags and a closed safe reason code;
- bounded Runtime, Team, Run, Evidence, and actionable Attention summaries;
- per-list next cursor and `has_more`;
- counts only for the bounded returned page, never an inferred global total.

The service rebuilds Projection before publishing a new snapshot. If rebuild
fails after a prior success, it returns the last immutable snapshot with
`stale=true` and safe reason `projection_refresh_failed`; it never publishes a
partially rebuilt view. First-build failure returns `state_unavailable`.

### Timeline query

```go
ReadLocalProductTimeline(
    context.Context,
    LocalProductTimelineRequest,
) (LocalProductTimelinePage, error)
```

The request contains a selected user-visible `team_instance_id`, opaque cursor,
and limit `1..128`. The result maps the accepted `TeamExecutionStream` page,
board, Attention, gap, view version, and next cursor into copied DTOs. No Event
payload, raw Grant, credential, hidden reasoning, or unbounded text is exposed.

### History and Compare

W1 History is the bounded Run/Evidence page in the snapshot. Compare is a pure
client-side comparison of two returned Run summaries and their exact Evidence
references. It does not query a second store or claim usage/cost fields that
Phase 1 did not persist.

## 8. Legacy Phase 1 timeline compatibility

The accepted final Phase 1 live canary has a valid TeamExecution/WorkItem/Run/
Grant/Evidence lineage but lacks a projected saved-Team record. Existing
`deriveRelatedScope` therefore returns `ErrTeamTimelineNotFound`.

W1 may add one read-only compatibility anchor:

```text
saved Team exists
OR
legacy execution-only anchor is exact and fully related
```

An execution-only anchor is accepted only when all conditions hold:

1. `GlobalReadView.Team(requestedID)` is absent.
2. `GlobalReadView.TeamExecution(requestedID)` exists and its
   `TeamInstanceID` exactly equals `requestedID`.
3. `Head("team-execution/"+requestedID)` exists with positive sequence and
   non-empty canonical Event ID.
4. The execution has at least one node and at least one attempt.
5. Every WorkItem, Run, Grant, Evidence, approval, verifier, node, attempt
   number, generation, Runtime, and Agent relation already checked by
   `deriveRelatedScope` passes unchanged.
6. The TeamExecution status is terminal `succeeded` or `failed`; an active,
   empty, partial, or unknown execution-only record is rejected.

The anchor:

- is labeled `historical_execution_only`;
- is `confirmed=false`, `executable=false`, and `read_only=true`;
- may appear only in W1 read summaries and timeline scope;
- cannot satisfy Team selection for a new execution, Draft confirmation,
  permissions, policy, Grant, or StateWriter input;
- does not synthesize a Team Journal fact or `projection.TeamInstance`;
- does not skip any downstream lineage validation.

Malformed, ambiguous, nonterminal, or partially related legacy data continues
to fail closed.

## 9. Private IPC v1

### Socket

Production uses an explicit `loomd --socket <absolute-path>` option. Omitting
`--socket` preserves the Phase 1 observer-only behavior for backward
compatibility.

The path must:

- be absolute, clean, non-empty, and at most 96 UTF-8 bytes;
- have basename `loomd.sock`;
- have an existing parent directory owned by the effective user, mode `0700`,
  with no symlink in the resolved parent chain;
- not be a symlink or regular file.

Server behavior:

- obtains an exclusive sibling `loomd.sock.lock` before listen;
- refuses a live socket or lock owner;
- removes a stale socket only after exclusive lock acquisition, `Lstat`
  confirms socket type, a bounded dial returns connection-refused/not-found,
  and a second `Lstat` matches device and inode;
- creates the socket with effective `0600` access and verifies it;
- records the created device/inode and removes the socket on close only if both
  still match;
- never recursively deletes a directory or follows a symlink;
- bounds accepted connections to 16 and rejects excess work;
- closes all listeners and connections on context cancellation.

The lock file is `0600`, contains no credential, and is removed only when it is
still the server's exact file.

### Peer authorization

On macOS, the server obtains the peer effective UID from the accepted Unix
socket and requires it to equal the daemon effective UID. Linux uses
`SO_PEERCRED` with the same equality rule. Unsupported platforms fail closed
with `unsupported_platform`; they do not silently trust the peer.

Directory permissions are necessary but not sufficient for peer authorization.

### Framing

IPC uses one request and one response per connection:

```text
4-byte unsigned big-endian length
+ exact compact UTF-8 JSON bytes
```

Limits:

- request body: `1..65536` bytes;
- response body: `1..524288` bytes;
- request ID: `1..64` ASCII `[A-Za-z0-9._:-]`;
- one connection deadline: five seconds;
- no extra frame, trailing byte, or half-open request;
- no compression, file descriptor passing, shell, HTTP, or public TCP.

### Request envelope

```json
{
  "version": 1,
  "request_id": "client-generated-id",
  "method": "snapshot",
  "params": {}
}
```

Exact methods:

- `ping`
- `snapshot`
- `timeline_page`

Unknown or duplicate JSON fields, invalid UTF-8, noncanonical JSON numbers,
unknown method, wrong version, empty params, oversized values, and trailing
data fail closed.

### Response envelope

```json
{
  "version": 1,
  "request_id": "same-id",
  "ok": true,
  "result": {},
  "error": null
}
```

Closed safe error codes:

```text
invalid_request
unsupported_version
unknown_method
unauthorized_peer
unsupported_platform
not_found
cursor_conflict
stream_gap
state_unavailable
timeout
busy
internal
```

Errors contain a safe fixed message and optional recoverable flag only. They
must not contain filesystem paths, SQL, raw upstream errors, payload excerpts,
Event JSON, secret values, environment variables, or stack traces.

`ping` returns only protocol version, daemon availability, and a non-secret
build identifier. It does not prove Journal health.

## 10. Daemon integration

`cmd/loomd` composes:

- the existing `LocalRuntimeObservationDaemon`; and
- the optional W1 local IPC read server.

Rules:

- observation remains the only writer;
- server start failure occurs before observer `Run` and closes all resources;
- after both start, failure or cancellation of either component cancels the
  sibling exactly once;
- close order is IPC listener/connections, read-only database, observer;
- no goroutine survives `Run`/`Close`;
- output keeps the accepted finite daemon result for `--max-cycles`;
- `--socket` omitted preserves all current tests and behavior;
- `--socket` never activates Runtime execution or Provider traffic.

## 11. CLI routing

`cmd/loom` behavior becomes:

| Invocation | Behavior |
|---|---|
| `loom` | starts the TUI using the default user socket |
| `loom app [--socket PATH]` | starts the same TUI; explicit socket is diagnostic |
| `loom status [--socket PATH]` | reads the local API and prints deterministic JSON |
| `loom timeline --team ID [--cursor C] [--limit N] [--socket PATH]` | reads the local API and prints deterministic JSON |
| `loom status --state PATH` / `loom timeline --state PATH ...` | explicit offline recovery read of the existing finite implementation |
| `loom route ...` | preserves the existing pure mode router |

The default socket is:

```text
$HOME/Library/Application Support/Loom/run/loomd.sock
```

The client validates the resolved home directory and socket path. It does not
read a Provider environment variable. `LOOM_SOCKET` is not a product
configuration path.

Offline `--state` and online `--socket` are mutually exclusive. Offline output
is labeled `source_mode=offline_recovery`; ordinary online output is labeled
`source_mode=daemon_api`. The TUI has no offline/direct-database mode.

## 12. TUI model

W1 uses Bubble Tea's `Model`, `Init`, `Update`, `View`, commands, window-size
messages, `NewProgram`, and `WithContext`. External daemon data enters the
update loop only as copied typed messages.

### Screens

W1 has these read screens inside one model:

1. Home
2. Runtimes
3. Teams
4. Runs / History
5. Evidence
6. Compare
7. Attention
8. Team Timeline

These are screens, not separate WorkItems or services.

### Interaction

- `tab` / `shift+tab` or left/right: change top-level screen;
- up/down or `j`/`k`: move bounded selection;
- enter: open selected Team timeline or select a Run for Compare;
- `r`: explicit refresh;
- escape: close detail/back;
- `?`: toggle help;
- `q` or `ctrl+c`: cancel the client and exit.

No key dispatches a mutation in W1.

### View states

Every screen has deterministic:

- loading;
- ready;
- empty;
- partial;
- stale;
- daemon offline;
- recoverable stream gap;
- conflict;
- state unavailable;
- fatal protocol mismatch.

The footer always identifies read-only W1 behavior. A stale snapshot displays
its reason and last accepted view version.

### Rendering safety

- minimum supported viewport: 60 columns × 16 rows;
- smaller windows render one bounded resize instruction;
- display values are clipped to the current viewport;
- C0/C1 controls, ANSI escape sequences, bidirectional override/isolate
  controls, invalid UTF-8, and unbounded whitespace are replaced or removed;
- tabs/newlines are normalized for single-line cells;
- no raw Event payload, Grant, credential, prompt, hidden reasoning, or
  tentative Runtime output is rendered;
- `View()` is pure and performs no I/O.

The model keeps only copied current page data, selection, current cursor, and
presentation state. It is not persisted and is replaced on refresh/reconnect.

## 13. Installer and ordinary launch

`scripts/install-loom-local-product.sh` is a user-level, non-networked installer
for already-built reviewed `loom` and `loomd` binaries.

It:

- requires explicit source binary paths and an install root;
- rejects symlinked roots, broad roots, world-writable parents, non-owned
  inputs, missing files, and unexpected executable names;
- creates directories `0700` and binaries/launchers `0700`;
- atomically installs versioned bytes, preserving a one-version rollback copy;
- creates a Finder-launchable `Loom.command` containing only an absolute path to
  the installed `loom` binary and no credentials/environment values;
- never edits or loads a LaunchAgent in the deterministic gate;
- supports `--dry-run` and exact rollback;
- never removes user state or performs recursive deletion.

The test script installs into a private temporary root, verifies modes and exact
bytes, launches the fixture TUI with controlled input/output, exits with `q`,
rolls back, and proves paths outside the root are unchanged.

Resident LaunchAgent installation/replacement remains post-Implementation-
Review live-gate work in section 16 and final lifecycle product work in W3.

## 14. Mandatory RED

Before production implementation, W1 captures failing tests for:

1. bounded deep-copy Projection enumeration;
2. exact legacy execution-only timeline compatibility and all rejection cases;
3. typed snapshot/timeline read service and stale-view preservation;
4. IPC framing, exact JSON, bounds, peer UID, stale socket, symlink/TOCTOU,
   timeout, cancellation, concurrency, and safe error mapping;
5. daemon optional-server lifecycle and observer-only compatibility;
6. CLI default TUI, daemon API reads, and explicit offline recovery;
7. TUI navigation, view states, resize, sanitization, reconnect, cursor gap,
   no mutation, and pure rendering;
8. isolated real-SQLite + real-UDS + headless-TUI read E2E;
9. installer dry-run/install/exit/rollback safety.

Each RED must fail for the missing behavior, not for a syntax error, unavailable
network, changed fixture, or weakened precondition. RED evidence records the
exact command and relevant failure.

## 15. Deterministic acceptance

W1 is GREEN only when all assertions pass:

1. TUI and ordinary CLI contain no `database/sql`, SQLite driver, Journal,
   Projection, `os/exec`, shell, or service-manager dependency.
2. A real fixture Journal is rebuilt once into the same version observed by the
   daemon API, CLI, and TUI.
3. Home and all eight screens show the same bounded state.
4. Selecting a Team uses its typed ID internally; the user does not type it.
5. The accepted Phase 1 legacy fixture becomes a read-only historical Team
   timeline, while every malformed/nonterminal/cross-Team variant fails closed.
6. Cursor reconnect returns no duplicate authoritative record.
7. Projection rebuild failure preserves the prior immutable view and marks it
   stale.
8. Daemon restart produces one recoverable disconnect, then the same Journal-
   authoritative view without resubmitting or mutating anything.
9. Slow client polling receives an explicit cursor gap and recovers the durable
   warning/retry/degraded/blocked/human-required/terminal facts.
10. Concurrent connections cannot exceed 16; cancellation leaves no goroutine,
    socket, lock, or database handle leak.
11. Wrong UID, unsupported platform, symlink parent/socket, socket replacement,
    stale inode mismatch, oversized frame, duplicate JSON field, wrong version,
    unknown method, timeout, and trailing byte all fail closed.
12. TUI renders malicious names/output without terminal escape or bidi control.
13. The installer changes only its private destination and rollback restores
    exact prior bytes.
14. Journal Event count and stream heads are byte-for-byte unchanged across the
    complete W1 read E2E.

## 16. Controlled resident-daemon live gate

Only after fresh independent Implementation Review `PASS`, one controlled local
read-only live gate may:

1. record current LaunchAgent plist, installed wrapper/binary, state database,
   socket absence, process state, file modes, and SHA-256 hashes;
2. build exact reviewed `loom` and `loomd` binaries;
3. create
   `/Users/lune/Library/Application Support/Loom/demo-resident/run` as `0700`;
4. atomically install the reviewed daemon binary and add exactly:

   ```text
   --socket
   /Users/lune/Library/Application Support/Loom/demo-resident/run/loomd.sock
   ```

   to the existing private LaunchAgent configuration;
5. reload the LaunchAgent once from a credential-clean service-manager context;
6. verify the daemon process environment, LaunchAgent/service-manager metadata,
   arguments, logs, Journal, socket, TUI, screenshots, and evidence contain no
   Provider secret or credential;
7. use the installed TUI to inspect the actual Pi Runtime and the accepted Phase
   1 historical timeline without a database path, Team ID, cursor, or command
   entered by the user;
8. quit and relaunch the TUI, then restart the daemon once and prove the same
   view/version/terminal lineage recovers with no duplicate Event or side
   effect;
9. preserve the upgraded service only on full PASS; otherwise atomically
   restore the exact prior binary/plist and running state.

The live gate performs no Runtime execution, Provider/model request, credential
write, Team mutation, WorkPackage creation, scheduler dispatch, or approval.

If service-manager inherited metadata cannot be made credential-clean without
mutating unrelated user credentials, stop `HUMAN_REQUIRED`; do not print the
values or weaken the check.

## 17. Verification matrix

After GREEN and after every repair that changes product behavior:

```text
gofmt on owned Go files
go test ./internal/projection ./internal/api ./internal/localipc ./internal/tui ./cmd/loom ./cmd/loomd
go test -race ./internal/api ./internal/localipc ./internal/tui ./cmd/loomd
go test ./...
go test -race ./...
go vet ./...
go mod tidy
go mod verify
GOOS=windows GOARCH=amd64 go test ./internal/tui ./cmd/loom
scripts/test-install-loom-local-product.sh
git diff --check
owned-file scope audit
dependency and license audit
authority/import-direction audit
secret-negative and terminal-control audit
SQLite integrity and unchanged-head audit
```

Commands that require network run only for the frozen dependency
materialization; deterministic tests use no external network.

## 18. Review and commit gate

1. This contract receives fresh independent Contract Review `PASS`.
2. RED is captured.
3. Implementation and deterministic evidence become GREEN.
4. Fresh independent Implementation Review returns `PASS`.
5. The controlled resident-daemon live gate passes or stops explicitly.
6. One local atomic P2A-W1 commit contains only owned files and W1 evidence.

P2A-W2 cannot be frozen until the W1 commit exists and `docs/CURRENT.md` names
the W2 child-contract gate.

## 19. Stop and amendment conditions

Stop for a reviewed W1 amendment before:

- modifying an unowned file or accepted authority;
- adding an IPC method or mutating command;
- adding a public/network listener or HTTP;
- changing Journal, Projection event semantics, CAS, StateWriter, Scheduler,
  Supervisor, Grant, Evidence, Team confirmation, or Runtime execution;
- widening the legacy anchor beyond the exact terminal read-only conditions;
- raising the Go language floor or changing the dependency lock;
- using an unverified dependency source;
- performing destructive migration or deleting user state;
- widening or retrying the single live gate.

Stop `HUMAN_REQUIRED` when:

- safe peer identity cannot be obtained on controlled macOS;
- a private owned `0700` socket parent cannot be established;
- the current resident service cannot be upgraded and rolled back atomically;
- secret-negative checks detect Provider material in product-controlled service
  metadata and safe cleanup requires new authority;
- the same blocking condition survives three governed attempts.

W1 must not report TUI delivery from model snapshots alone. Completion requires
deterministic E2E, independent Review, and the controlled installed read-product
proof.
