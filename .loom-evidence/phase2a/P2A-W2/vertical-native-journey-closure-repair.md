# P2A-W2 Vertical Native Journey Closure Repair

**Date**: 2026-07-30
**Status**: FROZEN — Contract Repair 1 fresh independent Re-review PASS
**Parent**: `P2A-W2 Team Builder and Provider Onboarding`
**Repairs**: consumed attempt `p2a-w2-live-20260730-004`
**Risk**: STRICT — Projection freshness, OS Secret Store, Provider network
verification, Journal fact closure, IPC lifecycle and native Team confirmation

## 1. Why this is one vertical repair

The reviewed attempt-004 result proved the Codex 0.144.1 stderr compatibility
repair at the narrow native-availability boundary, then failed the ordinary
W2 journey in three connected places:

1. MiniMax `Test` returned to the existing `Verified` presentation without
   appending a new `ProviderCredentialVerified` fact.
2. `Create Candidate team` returned `Incompatible` before the first bounded
   question.
3. cancellation closed the socket listener but the daemon did not exit after
   bounded `SIGINT` and `SIGTERM`.

These are not three new WorkItems or three single-point Amendments. They are
one product transaction:

```text
fresh authoritative Runtime view
→ Provider verification closes one Journal fact
→ one-question Candidate uses the same frozen view
→ explicit TeamDefinition confirmation
→ app restart reconstructs the result
→ daemon exits cleanly
```

The repair remains inside P2A-W2. It creates no P2A-W4, does not unlock P2A-W3
and does not permit execution.

## 2. Evidence-led failure model

### 2.1 Static setup catalog

At attempt-004 construction, `buildProductSetupService` read the inherited
five-Event baseline before the observer's first cycle. The inherited Pi Runtime
was online but its authoritative `model_ids` value was `null`. The setup
service therefore froze an empty role catalog.

The observer subsequently appended the model-capable Runtime fact:

```text
runtime_instance:runtime.pi.earendil-works.0.82.1
model_ids:
  - loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m
```

Home/System rebuilt from Projection and showed the Runtime, while the already
constructed setup service retained the stale empty catalog. This explains the
native `Incompatible` result without treating Projection as a second authority
or inventing a model.

### 2.2 Equal Provider and IPC deadlines

The production MiniMax verifier timeout, Go IPC handler deadline and Swift
socket deadline are all five seconds. That leaves no bounded interval for a
terminal Provider result to be validated and committed to the Journal. The
attempt retained three inherited verification facts and appended none.

The evidence does not prove whether the external request succeeded, timed out
or was cancelled, so the repair may not synthesize success. It must close the
actual observed result exactly once.

### 2.3 Unclassified shutdown stall

The attempt did not retain a goroutine dump or a safe lifecycle-stage result.
It proves only:

- the signal reached the product;
- the socket listener closed;
- the process remained alive past bounded graceful waits.

The repair must not guess whether IPC handler join, observer cancellation,
setup connector closure or database closure was responsible. Deterministic
blocking seams must cover every stage and preserve joined causes internally
without publishing private text.

## 3. Required product behavior

### 3.1 Versioned fresh setup catalog

Provider/Team setup must derive its Runtime catalog from a completed
`Projection.Rebuild` and the current immutable `GlobalReadView`, not from a
daemon-construction snapshot.

The catalog source must:

- use only authoritative Runtime facts already present in the Event Journal;
- normalize nil collections at the wire boundary without changing Journal
  bytes;
- select a model-capable online Runtime deterministically;
- include exact Runtime instance, adapter, version, model and capability
  bindings;
- produce a canonical catalog digest from those bindings;
- return no role options if no compatible model-capable Runtime exists;
- never call a Runtime, Provider or model while constructing the catalog.

`SetupSnapshot` and `StartBuilder` must refresh the source after Projection
rebuild. A Builder session then freezes its exact catalog, domain catalog,
view version and binding digest. `Answer`, `Edit`, `Validate` and `Confirm`
must reject a stale or changed catalog before mutation; they may never silently
rebind an in-flight Candidate. A later refresh may create a new Candidate but
cannot mutate or rescue the old session.

The attempt-004 six-Event fixture — one legacy Runtime with `model_ids:null`
followed by a different model-capable Runtime — must expose role options for
the model-capable Runtime and complete the one-question flow.

### 3.2 Closed MiniMax verification transaction

One explicit native `Test` performs exactly one Brokered verification with no
hidden retry. It must:

1. read the existing exact credential reference only through the OS Secret
   Store;
2. make one bounded request to the exact reviewed MiniMax origin;
3. classify the actual response as valid, provider-rejected, unavailable or
   timeout;
4. append exactly one terminal credential metadata fact for that observation;
5. return the same revision/status/reason to the client;
6. refresh the native presentation from Projection.

The exact budget hierarchy is:

```text
MiniMax external verification:              5 seconds
terminal metadata commit after observation: 1 second
Go IPC credential_verify request:          10 seconds
Swift credential_verify request:           10 seconds
all other existing local IPC requests:       5 seconds
```

Only `credential_verify` receives the ten-second request budget. The server
must apply it after strict request decoding and the Swift client must request
it only for the exact verify method; this does not widen ping, reads, Builder,
Team mutation or other Provider operations.

The five-second external budget plus one-second terminal commit budget leaves
four seconds for the non-interactive Keychain read, framing and response.
After a real Provider result exists, caller cancellation may not silently erase
that result: the Broker may use a fresh, at-most-one-second,
cancellation-independent commit context only for the terminal metadata append.
It may not retry the Provider, change the secret, borrow an older result or
hide a commit failure.

If the Secret Store read fails or is denied before Provider observation, no
verification fact is appended. macOS Keychain reads must fail closed rather
than open an untracked authentication UI. Secrets remain excluded from source,
arguments, environment, prompt, logs, Journal, Evidence, AgentDefinition,
screenshots and error strings.

### 3.3 Bounded daemon lifecycle

Cancellation must close the complete product lifecycle in a deterministic
order:

```text
stop accepting IPC
→ cancel and join tracked handlers
→ cancel and join Runtime observer
→ close setup/native-auth controller
→ close each database/lock exactly once
→ remove exact socket/lock
→ exit
```

Every tracked handler and observer must observe cancellation. No stage may be
detached into an untracked goroutine merely to make shutdown return. Tests must
block each stage independently, release it through cancellation, and prove the
whole process joins within a bounded deadline. The duplicate product database
close is removed.

Internal errors retain the lifecycle stage and cause for tests. The CLI may
continue publishing only the closed `daemon failed: shutdown` surface; child
output, paths, Provider details, secrets and arbitrary wrapped text remain
private.

## 4. Mandatory RED

Before production changes, tests must fail for the current behavior:

1. a setup service created before the model-capable Event must refresh from the
   current GlobalReadView and start a compatible blank Candidate afterward;
2. an in-flight Candidate rejects a changed Runtime catalog without silent
   rebinding;
3. the exact attempt-004 Runtime fixture with legacy `model_ids:null` selects
   the later model-capable Runtime deterministically;
4. a Provider result arriving near its external deadline still appends exactly
   one terminal fact before the IPC/client deadline;
5. caller cancellation after Provider observation cannot erase or duplicate
   the terminal fact;
6. Secret Store failure before Provider observation appends zero facts;
7. Keychain read is non-interactive and fail-closed;
8. cancellation during IPC handler, observer wait/cycle, setup close and
   database close joins without goroutine/process/socket residue;
9. the product database is closed exactly once;
10. Swift presents the returned verification revision/status rather than a
    stale pre-request value.

Fixtures may use only local deterministic stores, HTTP transports, clocks,
listeners and SQLite. Normal tests may not access real Keychain, network,
installed Pi, Codex, llama-server, GGUF, native application or resident daemon.

## 5. Owned boundary

This repair may modify only:

```text
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
internal/app/local_product_setup.go
internal/app/local_product_setup_test.go
internal/api/local_product_setup.go
internal/api/local_product_setup_test.go
internal/credentials/credential_broker.go
internal/credentials/credential_broker_test.go
internal/credentials/keychain_darwin.go
internal/credentials/keychain_darwin_test.go
internal/provider/minimax_verifier.go
internal/provider/minimax_verifier_test.go
internal/localipc/server.go
internal/localipc/server_test.go
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Tests/LoomLocalAppTests/
docs/CURRENT.md
.loom-evidence/phase2a/P2A-W2/
```

`internal/localipc` and Swift client files are conditionally owned only where
RED proves their deadline/join behavior participates in the vertical failure.
No Journal schema, Event payload, StateWriter authority, Projection authority,
Runtime probe/parser, Team domain, execution, Grant, Evidence, Scheduler,
LaunchAgent, install path or unrelated dirty file is owned.

## 6. Verification

Required GREEN:

```text
focused setup/credential/provider/localipc/loomd tests
focused race repetitions
attempt-004 six-Event SQLite fixture integration test
real Go IPC Server → Swift Client fixture
go test -p 1 ./... -count=1
go test -race -p 1 ./... -count=1
go vet ./...
go mod tidy -diff
go mod verify
gofmt and git diff --check
Swift debug, release and thread-sanitizer matrices
owned-file and secret-negative scans
fresh independent Implementation Review
```

Reviewer must verify that setup remains Projection-backed, terminal Provider
metadata still uses the accepted Writer/Journal authority, lifecycle work is
joined rather than detached, and no secret or raw Provider result crosses a
surface.

## 7. One vertical replacement canary

Only Contract Review `PASS`, mandatory RED, complete deterministic GREEN and
fresh independent Implementation Review `PASS` unlock exactly one lineage:

```text
p2a-w2-live-20260730-005
```

It uses a fresh private root and regular `0600` clone of the exact attempt-002
five-Event source database with SHA-256
`b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22`,
the exact post-Implementation-Review Candidate binaries, the same locked Pi,
Node, Codex, llama-server and GGUF identities, the existing credential only
through Broker/Keychain, one daemon start and one ordinary native journey.

The user must be able to:

1. see Codex `Available`, MiniMax recovered and the model-capable Pi Runtime;
2. select MiniMax `Test` once and observe exactly one new terminal verification
   fact;
3. start from blank, answer one bounded question at a time without internal
   IDs, inspect exact compatibility/permission/cost bindings and explicitly
   confirm exactly one TeamDefinition;
4. quit and reopen the native app and recover the same Provider, Runtime and
   saved Team from Journal/Projection;
5. quit the app and stop the daemon gracefully within the frozen deadline.

There must be no TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence,
dispatch or execution fact; no secret disclosure; no duplicate Event; no
attempt process, socket, lock or isolation residue; and no resident daemon or
LaunchAgent mutation.

No automatic retry, alternate executable, argument change, inherited process
environment, component execution B, further single-point amendment or second
replacement canary is permitted.

## 8. Contract Repair 1

Contract Review 1 returned `FAIL` only because the owned Swift test path did not
name the package's real test target. Repair 1 changes:

- `apps/macos/Tests/LoomLocalAppCoreTests/` to the actual
  `apps/macos/Tests/LoomLocalAppTests/`;
- the qualitative timeout hierarchy to the exact 5s Provider, 1s terminal
  commit, 10s exact `credential_verify` Go/Swift and unchanged 5s default
  budgets;
- `exact reviewed Candidate binaries` to the exact post-Implementation-Review
  Candidate binaries;
- attempt-005's source from the already model-capable six-Event result to the
  exact five-Event attempt-002 baseline, so the native journey proves setup
  refresh after the observer's first model-capable append.

No behavior, authority, secret boundary, owned production file, live allowance
count or exit requirement is otherwise changed. Repair 1 requires fresh
independent Contract Re-review `PASS` before RED or production work.

## 9. Exit

Fresh independent Result-Evidence Review must pass the complete attempt-005
journey. Only then may P2A-W2 be accepted and P2A-W3 governance begin.

Anything less leaves P2A-W2 unaccepted, P2A-W3 locked and no P2A-W4.
