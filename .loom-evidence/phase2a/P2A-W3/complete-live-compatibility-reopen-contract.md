# P2A-W3 Complete Live Compatibility Reopen Contract

**Date**: 2026-08-02  
**Status**: `FROZEN REPAIR 1 / IMPLEMENTATION LOCKED PENDING RE-REVIEW`  
**Risk**: `STRICT`  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Parent WorkItem**: unique `P2A-W3 Controlled Execution Experience`  
**New WorkItem**: none; `P2A-W4` does not exist

## 1. Why this is one complete reopen

The reviewed P2A-W3 Candidate passed deterministic gates and Implementation
Review 4, but its three one-shot live gates exposed a cross-layer compatibility
closure failure:

1. one Pi metadata timeout terminated the shared product daemon before the
   independent Codex product path could reach preflight;
2. one explicit MiniMax `Test` action produced no new authoritative terminal
   verification fact and no visible causal result; and
3. a saved, executable Pi Team failed exact Mission preflight because discovery
   records the qualified model identity `loom-local/qwen...` while the mission
   binding compared it with an unqualified model ID.

The Pi live path also exposed an internal TeamDefinition identifier in the
ordinary Team selector. Terminal input appeared to collapse spaces, but current
source does not yet prove that as a product defect; the RED must distinguish a
real TUI input bug from a Computer Use/PTY input artifact.

These failures span Runtime observation, product daemon lifecycle, credential
operation causality, execution binding and client presentation. They are
therefore repaired once inside the existing W3 vertical Candidate. No
single-point Amendment, wrapper WorkItem, retry-only repair or P2A-W4 is
permitted.

## 2. Accepted architecture remains binding

- ADR-0004 remains authoritative for brokered MiniMax credentials. Raw secrets
  stay inside OS Secret Store/Credential Broker and never enter product state,
  process arguments, logs, Journal facts or evidence.
- ADR-0010 remains authoritative for Pi RPC translation. The Pi adapter remains
  behind Supervisor, Grant, Frame authorization and Evidence; `loom.bridge.v1`
  is unchanged.
- ADR-0011/0012 remain authoritative for TUI/native clients over one private
  typed daemon IPC. Clients never parse CLI output, read SQLite or become a
  command/state authority.
- The Event Journal remains the only state authority. Projection remains
  rebuildable. No Event type, Event payload field, Journal writer, Grant,
  Evidence, Rules, Supervisor or Scheduler authority is reopened.

## 3. Acceptance boundary A — qualified Pi model identity

The Candidate must use one exact, typed interpretation of the discovered Pi
model identity:

```text
qualified discovery identity = loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m
provider                    = loom-local
adapter model               = qwen2.5-coder-1.5b-instruct-q4-k-m
```

Required behavior:

1. One small typed parser requires exactly one `/`, parses the two non-empty
   components once and accepts only exact provider `loom-local` plus exact model
   `qwen2.5-coder-1.5b-instruct-q4-k-m`. Call sites must not repeat inline
   string splitting.
2. Mission binding accepts only that exact qualified discovery identity for the
   locked local Pi execution recipe.
3. It derives the exact provider and adapter model components; it does not
   strip an arbitrary prefix, accept aliases, suffix-match or take an arbitrary
   first model.
4. Saved-Team binding continues to prove that the selected RuntimeProfile model
   exists in the current discovery and that discovery digest, Runtime instance,
   profile, accepted binding and capacity all match.
5. The Pi adapter remains fixed to the same provider/model pair. Unknown
   provider, wrong model, extra slash, empty component, multiple candidate
   models, stale discovery digest or Runtime identity drift fails closed before
   preflight output or execution construction.
6. The user-facing preflight presents a human-safe model label without treating
   presentation text as identity authority.

Mandatory causal RED: a real-Pi-format discovery table containing the exact
qualified ID must traverse Runtime discovery -> Saved Team materialization ->
authoritative product snapshot -> Mission preflight. It fails in the frozen
Candidate at the current unqualified comparison and passes only after repair.

## 4. Acceptance boundary B — Runtime failure containment

The shared product IPC must not be terminated by an exact Runtime-scoped Pi
metadata timeout.

Required behavior:

1. Only an observer error chain containing the typed
   `piadapter.ErrPiMetadataProcessTimeout`, when the closed command is exactly
   Pi version or list-models, is a containable Runtime observation health
   failure. Its external closed reason remains exactly
   `observer_version_timeout` or `observer_models_timeout`.
2. The product server remains available for Codex native-auth status, MiniMax
   credential management, saved-Team inspection and other non-execution
   journeys after one such failure.
3. No immediate hidden retry occurs. A resident observer may make its next
   ordinary scheduled observation only at the configured interval, with the
   failure visible as unavailable/partial health; a one-cycle fixture performs
   exactly one metadata invocation.
4. `ErrPiMetadataProcessFailed`, executable or metadata binding drift, stderr,
   output-limit, invalid/oversized/unknown metadata, duplicate model identity,
   probe/factory error, Projection/StateWriter/Journal failure, socket failure
   and shutdown failure are explicitly excluded from containment and remain
   fatal. Unknown error chains are fatal.
5. Product shutdown still joins observer/server/execution resources and removes
   the exact socket and product lock without SIGKILL or manual deletion.

Mandatory causal RED: a deterministic Pi `--list-models` timeout must leave the
real Go UDS server responsive to product `ping` and setup/Codex reads, prove one
metadata invocation, then close cleanly on controlled cancellation. A separate
test proves binding/metadata integrity failure still terminates the daemon.

## 5. Acceptance boundary C — causal MiniMax verification

Every explicit ordinary-user `Test` action must have one visible, attributable
terminal result and at most one authoritative credential metadata transition.

Required behavior:

1. Swift creates one fresh strict `operation_id` for each explicit Test action.
   The product-daemon method-parameter decoder carries and validates it only for
   `credential_verify`; unknown, missing, malformed or duplicate fields fail
   closed. `credential_configure|replace|revoke` require it to be absent/empty.
   Generic `internal/localipc` framing does not interpret the field and is not
   modified.
2. The application derives one deterministic bounded command/idempotency
   identity from that operation ID. Replaying the same operation recovers its
   exact prior immutable Event/result through the existing Journal Event
   idempotency key; it does not rely on an in-memory-only cache and does not call
   the Provider or write twice.
3. A different operation presented with a stale revision returns canonical
   conflict. It must not reuse an unrelated prior `Verified` result or silently
   claim that a new Provider check occurred.
4. A current new operation makes one bounded Broker verification. Valid,
   rejected, timeout and unavailable responses commit exactly one existing
   `ProviderCredentialVerified` fact with revision `previous + 1` and closed
   status/reason. No new Event type or payload field is introduced.
5. Native UI renders `Testing` and then the exact terminal
   `Verified|Rejected|Unavailable|Conflict` state. The Test control is disabled
   while its operation is in flight; errors are visible in the Provider sheet,
   not only in hidden store state.
6. The client refresh accepts success only when the returned result and rebuilt
   setup snapshot both match the operation's expected next revision/status.
7. No secret, Authorization header, credential reference beyond the existing
   opaque reference, Provider body or raw error becomes UI, log or evidence.

Mandatory causal REDs:

- a real Swift Client -> Go UDS Server -> LocalProductSetupService -> Broker
  fixture starts from an already verified revision, executes one Test and
  proves one new terminal fact and revision;
- same-operation replay proves one Provider observation and one write;
- different-operation stale revision proves conflict and zero Provider/write;
- timeout/rejection proves a visible closed result without secret disclosure.

## 6. Acceptance boundary D — ordinary-user identity and text

1. Snapshot Team summaries resolve the current saved TeamDefinition name from
   the same GlobalReadView. They never use TeamDefinitionID or TeamInstanceID as
   ordinary display text when the definition exists.
2. Missing or inconsistent definitions remain fail-closed/human-safe and do not
   invent a name.
3. TUI Mission objective input preserves ordinary spaces and valid UTF-8. A
   deterministic KeyRunes test must first reproduce the reported collapse. If
   the current code already preserves spaces, record the report as a PTY input
   artifact and make no speculative production change.
4. Native/TUI preflight and Start continue to bind internal IDs invisibly from
   authoritative selection; changing display text never changes identity.

## 7. Exact owned production and test files

Only these existing files may change after Contract Review PASS:

```text
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
internal/app/local_product_execution.go
internal/app/local_product_execution_test.go
internal/app/local_product_setup.go
internal/app/local_product_setup_test.go
internal/api/local_product_read.go
internal/api/local_product_read_test.go
internal/api/local_product_setup.go
internal/api/local_product_setup_test.go
internal/credentials/credential_broker.go
internal/credentials/credential_broker_test.go
internal/localipc/swift_contract_test.go
internal/tui/model.go
internal/tui/model_test.go
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift
apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
```

Strict `operation_id` enforcement is owned by
`cmd/loomd/product_daemon.go` plus `cmd/loomd/product_daemon_test.go` at the
method-parameter layer. `internal/localipc/swift_contract_test.go` proves the
real Go-server/Swift-client wire, but `internal/localipc/protocol.go` and
`internal/localipc/protocol_test.go` remain locked because framing semantics do
not change.

Governance/evidence may be added only under
`.loom-evidence/phase2a/P2A-W3/`; final volatile status may update
`docs/CURRENT.md`. Adding another production path requires Contract Review
repair before implementation. Unrelated dirty files remain excluded.

## 8. Pre-reopen byte lock

```text
5aa3f89d2fc608b2215057d93ced44c1f6f1a99be894322a4200488e8748a73b  cmd/loomd/product_daemon.go
e58b8a4ec563dcd861ca8012272719a79e43bf0e7ba89c2dd2052ebf6e3cc9e3  cmd/loomd/product_daemon_test.go
d5493e257416253e90e258d3a74d7882dc03bd863d70b0ad0bb8c1d08797be2d  internal/app/local_product_execution.go
2d90e6bf3f7c8e7eb64d62089c6ee48d7ce5ddb393910d7646c29e4a942e5abf  internal/app/local_product_execution_test.go
e25af9f6c5ecdc0b4b1b6e4077b942e6a93371968e18d17f65c4add926379d20  internal/app/local_product_setup.go
25ebec7a737cbc636a4ec3f33d5919bf4dc6b732523a0c13878203da1b82ceef  internal/app/local_product_setup_test.go
88c39af45acf58aa7b3cb4374ee5dbc69fdbdac3d0cd79e939eb26b7d6d3ee91  internal/api/local_product_read.go
112c1dbd88132142218cfe190ce151e830bd9f3ad994c697418903e6ea2158ca  internal/api/local_product_read_test.go
df2007a45765b395a3933d0740db836900648eb4a37564289f49dcdae02b38a6  internal/api/local_product_setup.go
a9688010a9777cc46333f8223b4169b0504cf0f49d43f6a45717317eb3acc86c  internal/api/local_product_setup_test.go
dc41a3e528a3f283a1d037ee0d19ecff6679de7db040d291f00cc8da0c3af498  internal/credentials/credential_broker.go
c937a988fbeaf74609c9b38f25d5cd279b1b3be7e88dbe7d5ff94f29c4b59d74  internal/credentials/credential_broker_test.go
8ca33f638c0b54d5ee871c9d13b84b35f95becad8b9dddbb8880ca8d461041a0  internal/localipc/swift_contract_test.go
4113b10b001704aef857aa5746fbc50df399ee476db166e9573626ef88a9d309  internal/tui/model.go
f0b2748861a17f3dbcdc095a2b19e0e1359f788592d1489bc7c1588cf06a271d  internal/tui/model_test.go
5806a8733714606a2b18129c199497c4e4e1b564ed069ad86933b5485a86c9d9  apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift
4a362047fc03e1086da2e4ba0b3a2a8b204b3bddc2f9375a935c6de983ce699c  apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift
24aee142a153f6de6100d9c25fb8e6817aeddcd6c043a602e2395d0da32edd95  apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift
ba5dddc5daa9bb242695dd40d471bfd08dd21b81bac42ef81dc01bff404d80fe  apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift
d0431d402d28fee7360b7165cbe4e11d73ee26e00006c2b11d716cd04bc12290  apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift
```

Parent evidence remains immutable:

```text
80d947440c4f3d03e6922cd35a32cdf603dd446347423bb80f9c3fe383baca86  parent contract
03d213d9862ce1f7be95d4547bfaae67893e58a8c98018d6a6cd651bd251c3ce  reviewed Repair 3 source lock
eba43885f1b8e0ab5ca7c8b27592475e12ee482e3bfc33e9eaa343a2c8156053  Implementation Review 4
03c56e659421a616edd7b10adbe5150e07d24da730199881a8d4b38f1e09bc3c  remaining live Result Review
```

## 9. Rejected shortcuts

- increasing process timeout or retrying a consumed lineage;
- replacing the qualified model comparison with suffix matching or arbitrary
  prefix stripping;
- treating a stale MiniMax `Verified` projection as proof of a new Test;
- logging secret-bearing requests to diagnose the Provider action;
- making Runtime observation, WebSocket/UI state or notification a second
  authority;
- weakening Swift strict decoding;
- bypassing the product path with direct IPC, CLI execution or SQLite writes;
- fixing internal-ID presentation by changing authoritative IDs; and
- splitting observer, model, credential or UI wrappers into separate WorkItems.

## 10. Deterministic gates

The Controller must preserve causal RED output, then run against the final
digest:

```text
focused Go tests for each boundary
focused Go race tests for observer/credential concurrency
real Go UDS and strict Swift component fixtures
go test ./internal/api ./internal/app ./internal/credentials ./internal/localipc ./internal/tui ./cmd/loomd -count=1
go test -race ./internal/api ./internal/app ./internal/credentials ./internal/localipc ./internal/tui ./cmd/loomd -count=1
go test -timeout=8m -p=1 -count=1 ./...
go test -race -timeout=12m -p=1 -count=1 ./...
go vet ./...
go mod verify
swift test --package-path apps/macos
swift test --sanitize=thread --package-path apps/macos
swift build --package-path apps/macos -c release --product LoomLocalApp
git diff --check
scope, source-lock, protocol, authority and secret scans
fresh independent Implementation Review PASS
```

## 11. Live gates remain locked

Contract Review does not authorize live activity. After deterministic gates and
fresh Implementation Review PASS, freeze three new source-locked replacement
manifests with fresh private roots and consume each once:

1. Codex native-auth/status and exact product preflight while one deterministic
   Pi metadata timeout is contained;
2. MiniMax one explicit Test with a new terminal revision/fact and visible
   result; and
3. Pi one exact saved-Team preflight, explicit controlled execution,
   authorized tentative output, source/Verifier Evidence, terminal and
   reconnect without redispatch.

Only after all three pass may the no-terminal native/TUI walkthrough and fresh
Result Review run. No prior attempt root is reused. No hidden retry is allowed.

## 12. Stop conditions

Stop `HUMAN_REQUIRED` before further implementation if Contract Review finds
that closure requires:

- a new Event schema or change to Journal/Projection authority;
- raw credential or OAuth access outside the accepted boundary;
- modification of Pi bridge, Supervisor, Grant, Evidence or Rules authority;
- a second daemon, queue, cache, scheduler or state writer;
- production paths outside Section 7 without a reviewed contract repair; or
- a fourth bounded repair rejection in this same W3 lineage.

## 13. Contract Review questions

1. Does qualified model parsing preserve exact Pi/provider identity without
   aliasing or weakening saved-Team/runtime binding?
2. Is the observer failure classification narrow enough to preserve
   fail-closed trust and writer failures while keeping unrelated product paths
   available?
3. Can operation identity distinguish same-request recovery from a different
   stale MiniMax Test without new Event fields or a second authority?
4. Are the owned files sufficient and no broader than required?
5. Do the REDs exercise real UDS/Swift/SQLite boundaries rather than mocks that
   could reproduce the previous false green?
6. Are live replacement lineages correctly locked until a new Implementation
   Review PASS?
