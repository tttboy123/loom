# P2A-W2 Credential Transaction and Joined Shutdown Closure Amendment

**Date**: 2026-07-30
**Status**: FROZEN — Contract Repair 1 Re-review PASS
**Parent**: `P2A-W2 Team Builder and Provider Onboarding`
**Reopens**: the same W2 credential transaction and product-daemon lifecycle
**Risk**: STRICT — OS Secret Store, child-process isolation, terminal Journal
fact and daemon shutdown

## 1. Reason for reopening

The deterministic Vertical Native Journey Closure Candidate and its fresh
Implementation Review passed. The only controlled attempt-005 then proved that
dynamic Runtime catalog refresh and TeamDefinition save/recovery work, but
failed because:

1. the one MiniMax `Test` appended no new terminal verification fact;
2. the daemon did not complete joined shutdown and required exact-PID forced
   cleanup.

The reviewed diagnosis proves one connected unbounded ownership gap consistent
with the two failures: production Keychain Put/Read/Delete enter synchronous
Security.framework calls after only a pre-call context check, while local IPC
shutdown correctly waits for every handler. Provider observation and terminal
commit are already bounded; the Secret Store operation is not. Attempt-005 did
not retain a goroutine dump, so this amendment does not claim the exact live
stack frame was directly observed.

This is one same-W2 vertical amendment. It does not create P2A-W4, does not
unlock P2A-W3 and does not authorize a live action before every gate below
passes.

## 2. Required product behavior

### 2.1 Process-owned, cancellable OS Secret Store operations

Production Keychain Put, Read and Delete must execute in a short-lived owned
helper process so the daemon can cancel and join the operation even if
Security.framework does not return.

The helper boundary must:

- use the exact post-Review daemon executable identity, not a shell, script,
  user PATH lookup or alternate binary;
- accept exactly one operation and exit;
- receive operation metadata and secret bytes only through exact inherited
  private request/response file descriptors using a strict bounded binary
  protocol; stdin, stdout and stderr may not carry the protocol or secret;
- place no secret in argv, environment, filesystem, logs, stderr, Journal,
  Evidence or screenshots;
- use an empty environment and no terminal prompt;
- bound request and response bytes and reject truncation, trailing bytes,
  unknown operation/status and duplicate fields;
- retain the existing fixed Keychain service name, non-synchronizable item and
  `AfterFirstUnlockThisDeviceOnly` accessibility;
- retain explicit non-interactive authentication behavior;
- clear parent and helper secret buffers;
- on cancellation or deadline, kill only the exact helper process, wait for
  it, close all pipes and return a closed credential error;
- leave no child, pipe, temporary file, socket or goroutine after completion.

Every Put, Read and Delete helper operation has an exact two-second total wall
clock budget from child start through protocol completion and `Wait`. The
parent uses the earlier of that deadline and the request context. Cancellation
or expiry kills only the exact child, waits within the same two-second budget,
closes every descriptor and returns fail-closed. This leaves at least eight
seconds for a verify transaction's five-second Provider and one-second commit
inside the ten-second IPC budget, and at least three seconds for metadata
commit and response inside the five-second configure/replace/revoke budget.

The internal helper mode is not a user-facing CLI capability. It must reject
before any Keychain call unless all of these are independently true:

1. `getppid` identifies a live normal-mode product daemon, not a shell, test
   runner, launchd invocation without `--socket`, or another helper;
2. the parent and child canonical executables have the same frozen regular-file
   device, inode, owner, mode and SHA-256 identity;
3. the parent start identity and process arguments prove a normal product
   daemon with explicit `--state`, `--isolation-root` and private `--socket`,
   and contain no helper mode;
4. a connection to that exact private `0600` product socket reports the parent
   PID as its kernel peer;
5. the exact inherited request/response descriptors are private anonymous
   pipes with no terminal or regular-file endpoint.

Missing, malformed, stale or mismatched parent/process/socket/descriptor
evidence fails closed before parsing an operation. Direct invocation,
pipe-only invocation and a hidden argv token alone are insufficient and must
be rejected deterministically. The mode may not accept a secret from argv,
environment, stdin/stdout/stderr or a file.

The helper response descriptor is an internal sensitive channel owned only by
the authenticated parent transaction. It is not an external product response;
no IPC, API or ordinary CLI response may contain a Keychain payload.

### 2.2 One verification transaction, one terminal fact

`credential_verify` retains exactly one transaction:

```text
bounded Secret Store Read
-> one fixed-origin non-generative Provider observation
-> one cancellation-independent terminal metadata commit
```

Required semantics:

- Secret Store failure or timeout before Provider observation performs no
  Provider request and appends no synthetic fact;
- once the Provider returns a valid closed observation
  (`valid`, `rejected` or `unavailable`), exactly one terminal metadata fact is
  attempted even if the client disconnects or its request context is canceled;
- the terminal commit remains bounded to at most one second and uses the same
  expected stream revision and idempotency key;
- no hidden retry occurs at Secret Store, Provider, commit, IPC or Swift
  layers;
- the returned native state must be the exact committed revision/status/reason,
  followed by Projection refresh;
- a lost response may be recovered by the next snapshot, but may not cause a
  duplicate Provider request or duplicate Event.

Before invoking the Broker for `credential_verify`, the production
product-daemon credential wrapper must rebuild the authoritative Provider
projection and compare the current reference, revision and status to the
request:

- exact current revision equal to expected revision performs the one Provider
  observation;
- exact same reference at expected revision plus one with a closed terminal
  `verified` or `rejected` status returns that already committed result without
  Keychain or Provider access and without a Journal append;
- any other stale revision, reference change, gap or non-terminal state fails
  with the existing metadata-conflict surface before Keychain or Provider
  access.

This is the stable lost-response recovery identity. A retry may receive a fresh
application command ID, but it never reaches Broker, Provider or StateWriter
after the exact terminal precheck. No StateWriter, Journal or IPC protocol
change is required.

The five-second Provider, one-second commit, ten-second Go
`credential_verify`, ten-second Swift `credential_verify` and five-second
default IPC budgets remain unchanged. This amendment does not solve an
unbounded owner by increasing a timeout.

### 2.3 Joined daemon shutdown

On `SIGINT`, `SIGTERM`, parent cancellation, client disconnect or app exit:

- local IPC cancels active handler contexts and closes accepted connections;
- every in-flight credential helper is terminated if necessary and waited;
- every handler returns;
- local IPC, observer, setup/native-auth owner and database close in the
  reviewed order, exactly once;
- the product socket and product lock are removed;
- the daemon exits without `SIGKILL`, detached work or leaked child processes.

Shutdown may return a stable internal stage failure, but it may not wait
without a finite owner boundary.

### 2.4 Safe diagnostics

Deterministic tests and internal errors may identify only these safe stages:

```text
credential_helper_start
credential_helper_protocol
credential_helper_timeout
credential_helper_exit
credential_store_denied
credential_store_unavailable
credential_provider
credential_commit
```

No external response may contain a secret, Keychain payload, Provider body,
arbitrary child stderr, environment, command line or private path.

## 3. Mandatory RED

Before production behavior changes, tests must reproduce:

1. a blocking Secret Store operation that ignores context prevents the current
   joined IPC shutdown from completing;
2. current in-process Keychain Read has no in-flight cancellation boundary;
3. a client disconnect during an observed Provider result cannot lose or
   duplicate the required terminal fact;
4. direct helper invocation, pipe-only activation, parent identity mismatch,
   missing or mismatched product-socket peer and terminal or regular-file
   descriptor endpoints are rejected before a Keychain call;
5. secrets would be exposed if passed through argv, environment or stdio, so
   the helper contract rejects all three surfaces;
6. every non-`credential_verify` IPC method retains the five-second budget.

The RED evidence must not execute the installed Keychain item, Provider,
native app, Pi, Codex, resident daemon or network.

## 4. Deterministic GREEN

Required focused proof:

- exact helper success for Put/Read/Delete through the strict pipe protocol;
- helper hang, malformed response, stderr output, output overflow, identity
  drift, non-zero exit and cancellation all fail closed and leave no process;
- exact two-second helper wall-clock limits for Read, Put and Delete and
  preserved ten/five-second IPC budgets;
- direct, pipe-only, wrong-parent, stale-parent, helper-parent,
  wrong-socket-peer and non-private-descriptor activation all fail before
  Keychain access;
- secret-negative argv/environment/log/evidence assertions;
- Secret Store pre-observation failure yields zero Provider calls and zero
  facts;
- Provider valid/rejected/unavailable each yields exactly one terminal fact;
- cancellation or response loss after Provider observation still commits once;
- stale exact `expected+1` terminal recovery returns the committed result with
  zero Keychain calls, zero Provider calls and zero appends; every other stale
  shape fails before those boundaries;
- real local IPC cancellation joins an in-flight helper and removes the socket;
- `SIGINT` and `SIGTERM` fixture processes exit within a bounded interval with
  no `SIGKILL`;
- Swift presents the committed terminal revision/status and reconstructs it
  after restart.

Complete GREEN:

```text
focused credentials/provider/localipc/app/loomd tests
focused race repetitions
go test -p 1 ./... -count=1
go test -race -p 1 ./... -count=1
go vet ./...
go mod tidy -diff
go mod verify
gofmt and git diff --check
Swift debug tests
Swift release build
Swift thread-sanitizer tests
owned-file and secret-negative scans
fresh independent Implementation Review PASS
```

Normal tests must use fixture processes and fake Keychain/Provider boundaries.
They may not read, write, delete or enumerate the real Keychain item or contact
MiniMax.

## 5. Exact ownership

Only these files may change:

```text
cmd/loomd/main.go
cmd/loomd/main_test.go
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
internal/credentials/credential_broker.go
internal/credentials/credential_broker_test.go
internal/credentials/keychain_darwin.go
internal/credentials/keychain_darwin_test.go
internal/credentials/keychain_helper_darwin.go
internal/credentials/keychain_helper_darwin_test.go
internal/localipc/server.go
internal/localipc/server_test.go
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
docs/CURRENT.md
.loom-evidence/phase2a/P2A-W2/
```

An owned file may remain unchanged. Adding another source file, widening this
list or changing Journal schema/Event payload, StateWriter authority,
Projection authority, Team domain, Runtime discovery, execution, Grant,
Evidence, Scheduler, LaunchAgent or resident configuration requires Contract
Repair and fresh Re-review before implementation.

## 5.1 Contract Repair 1

Contract Review 1 returned `FAIL` with two P1 and two P2 findings. Repair 1:

- authenticates helper activation with the exact normal product-daemon parent
  executable, start and argv lineage, kernel product-socket peer PID and
  private inherited descriptor types; pipes or a hidden token alone cannot
  activate it;
- freezes a product-daemon pre-Provider exact `expected+1` terminal recovery,
  avoiding expansion into app, API, StateWriter, Journal or protocol
  authority;
- freezes two seconds as the total Put, Read and Delete helper budget while
  preserving the ten/five-second request hierarchy;
- changes diagnosis wording from direct live causality to a proven unbounded
  owner consistent with both failures.

No owned-file expansion is required. Fresh independent Contract Repair 1
Re-review is required before RED or production implementation.

## 6. One replacement native closure

Only Contract Review `PASS`, mandatory RED, complete deterministic GREEN and
fresh independent Implementation Review `PASS` amend the consumed allowance to
permit exactly one replacement lineage:

```text
p2a-w2-live-20260730-006
```

It must start from the exact private attempt-002 five-Event source database and
use:

- a fresh private `0700` root and regular `0600` clone;
- exact post-Implementation-Review Candidate daemon and native app;
- the already reviewed Pi, canonical Node, native Codex, llama-server and GGUF
  identities;
- the default private product socket only after proving it absent;
- the existing credential only through Credential Broker and OS Secret Store;
- no LaunchAgent or resident-observer mutation.

The resident LaunchAgent is currently outside this Candidate and may restart
independently. Preflight and postflight record its label, executable hash,
configured arguments and observed service-manager state without requiring PID
continuity. No Controller signal or configuration command may target it.

One daemon start and one ordinary native journey must prove:

1. exactly one model-capable Runtime discovery delta;
2. Codex `Available/native_auth` without login when already connected;
3. exactly one MiniMax `Test` and exactly one new closed
   `ProviderCredentialVerified` fact;
4. one bounded Team Builder flow and exactly one `TeamDefinitionSaved`;
5. no TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence, dispatch or
   execution fact;
6. app restart reconstructs Provider, Runtime and saved Team;
7. one normal `SIGINT` produces bounded joined daemon exit, absent helper,
   app, attempt, Pi and llama processes, empty isolation and absent product
   socket/lock;
8. final SQLite integrity, exact Event delta, permissions, artifact identities
   and secret-negative surfaces pass.

There is no retry, second Test, second daemon start, alternate executable,
argument change or second replacement canary.

## 7. Exit semantics

Fresh independent Result-Evidence Review must verify the exact deterministic
and live evidence. P2A-W2 is accepted only if the replacement canary and its
Review pass every item above.

Anything less leaves P2A-W2 unaccepted, P2A-W3 locked and no P2A-W4. A failed
replacement consumes the amendment and stops `HUMAN_REQUIRED`; it may not
produce another single-point amendment or silent retry.
