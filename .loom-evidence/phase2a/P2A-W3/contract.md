# P2A-W3 Controlled Execution Experience Contract

**Date**: 2026-08-01  
**Status**: FROZEN — Contract Repair 1 Re-review PASS  
**Authority**: Product Owner authorization and active P2A-W3 Goal  
**Baseline branch**: `codex/loom-platform-slice2`  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Risk**: STRICT — local Agent execution, Provider boundary, Grants, Evidence,
generation fencing, process cancellation and native product mutation

## 1. One vertical WorkItem

This is the only `P2A-W3 Controlled Execution Experience`. It closes the
complete controlled-execution product journey in one Candidate.

It creates no `P2A-W3a`, `P2A-W3b`, `P2A-W4`, adapter-only WorkItem, writer-only
WorkItem or UI-only WorkItem. Repairs remain inside this contract unless a
review proves that an accepted authority or Event schema must change; in that
case implementation stops `HUMAN_REQUIRED` before the authority change.

The unrelated commits at the baseline (`ee1d5cb`, `fe4b8c9`, `848f068`) and all
pre-existing modified/untracked paths are preserved. In particular this
Candidate excludes `internal/mcp/**`, `go.mod`, `go.sum`, unrelated
`.loom-drafts/**`, Phase 1 live-gate evidence, `AGENTS.md`, `PROGRESS.md` and
`README.md`.

## 2. Product outcome

An ordinary user can complete this journey without typing a Team ID, Runtime
ID, SQLite path, cursor, Provider environment variable or `launchctl` command:

```text
New Mission
  -> choose accepted WorkPackage kind
  -> choose one confirmed Team
  -> review exact preflight
  -> explicit Start
  -> observe DAG / node / Attempt / Run / tentative output
  -> resolve prepared approval or bounded recovery
  -> inspect source + Verifier Evidence and canonical terminal
  -> reconnect to the same lineage in History / Compare / Attention
```

The GUI and TUI use the same typed daemon API. Neither client executes a
Runtime, writes SQLite, reconstructs a Grant, retries a conflict, or decides a
recovery policy.

## 3. Authority and lifecycle

The only mutation path is:

```text
explicit product command
  -> strict local IPC
  -> LocalProductExecutionService
  -> accepted Rules / Work / TeamCoordinator authority
  -> AppendBatchIfStreamHeads CAS
  -> Event Journal
  -> Projection / GlobalReadView / TeamExecutionStream
  -> GUI and TUI
```

Rules:

1. Event Journal remains the sole durable state authority.
2. The daemon may own contexts and a bounded in-memory in-flight registry, but
   that registry is replaceable process state, not a queue, scheduler or truth.
3. `TeamCoordinator` remains the only execution coordinator and
   `DispatchTeamReadySet` remains the only team dispatch authority.
4. Start becomes successful only after the exact TeamExecution planning/first
   dispatch transaction is visible in a refreshed GlobalReadView. A socket
   write or goroutine launch is never success evidence.
5. Reconnect is read-only. It cannot repeat `start`.
6. Exact command digest/correlation retry is idempotent. A different command
   for the same Team execution conflicts. No hidden infinite retry exists.
7. Durable execution authority begins only when the accepted TeamExecution
   planning/dispatch facts commit. Daemon restart reopens only that
   Journal-visible non-terminal lineage and reconstructs the exact plan nodes,
   workflow paths, semantic contract digests, Runtime bindings and deterministic
   built-in execution recipes from accepted versioned catalog state. It does
   not reconstruct or claim a durable WorkPackage object. If an exact recipe
   cannot be selected by the persisted digests, it stops `human_required`; it
   never guesses from display text.
8. An executor can produce `ready_for_review`; only accepted source and
   independent Verifier Evidence can close acceptance and terminal aggregation.

## 4. WorkPackage and confirmed-Team boundary

P2A-W3 v1 allows the user to select one accepted immutable WorkPackage kind:

- `work.CodingWorkPackage()`; or
- `work.KnowledgeWorkPackage()`.

The WorkPackage object is a typed pre-start proposal/input, not a durable state
authority. The user supplies a bounded Mission title/objective that is
sanitized for display; the resulting exact node title is part of the accepted
ExecutionPlan digest and `TeamExecutionPlanned` payload. The service must not
persist a full sensitive prompt or later claim that the original WorkPackage
object can be reconstructed from Journal facts.

The selected WorkPackage ID/digest is present only in the immutable preflight
sheet. Confirmed Team identity/revision, plan digest, exact Runtime/model/auth
modes, permissions, accepted policy/contract digests and the safe plan node
titles are revalidated at start. After the first dispatch, History and restart
present the authoritative compiled TeamExecution plan; they do not present an
inferred WorkPackage lineage.

Only an accepted confirmed Team whose exact Agent/Runtime/model/permission
bindings still match the current GlobalReadView is eligible. Draft, archived,
stale, incompatible, offline or over-capacity selections fail with zero write.

If the accepted TeamExecution plan, semantic bindings, workflow paths and
versioned recipe catalog cannot reconstruct the exact non-terminal execution
without a new Event field, implementation stops before changing that Event
schema.

## 5. Single product command surface

The one new IPC method is exactly:

```text
mission_execution
```

Its strict `operation` field is one of:

- `preflight`
- `start`
- `control`

No method alias, per-control method, CLI-text forwarding or version-suffixed
duplicate is permitted.

Every request includes schema version, unique request/correlation ID, exact
Mission/Team identity, expected GlobalReadView version and operation-specific
digest. Authority-affecting unknown fields are rejected.

### Preflight

Preflight is read-only and returns a bounded immutable sheet containing:

- WorkPackage and confirmed Team identities/revisions/digests;
- DAG nodes, dependencies, roles and attempt ceilings;
- Runtime/model/profile/auth mode and current capacity;
- exact permission/tool/resource scope;
- accepted output, recovery and acceptance contract versions/digests;
- approval points, external side effects and cancellation limits;
- budget ceiling plus truthful `unavailable` usage/cost where no accepted fact
  exists;
- a canonical preflight digest bound to the current view.

Preflight never creates a TeamInstance, WorkItem, Run, Grant, Evidence record or
execution Event.

### Start

Start accepts only the exact unexpired preflight digest and revalidates the
current view, confirmed Team, Runtime status/capacity and authentication mode.
It delegates an immutable `TeamExecutionRequest` to the accepted coordinator.
It does not accept raw Grant material, Provider secrets, arbitrary executable
paths, shell fragments or client-generated Run IDs.

Two concurrent starts of the same preflight must produce one authoritative
planning/dispatch lineage and one idempotent result or typed conflict, with no
duplicate Run, Grant, Evidence, capacity reservation or side effect.

### Control

Control accepts one closed action and exact current lineage/generation:

- `pause_at_gate`: submit or leave unresolved an already-prepared Rules
  approval that fences a future claim; it is not process suspension;
- `approve`, `reject`, `not_now`, `edit_scope`: delegate to the accepted Mission
  Decision prepared-command boundary;
- `cancel`: cancel the daemon-owned execution context, require Supervisor
  cancellation/terminal evidence, and expose only the resulting authoritative
  `cancelled` state;
- `retry`, `fallback`, `degraded`, `blocked`, `human_required`, `restart`:
  execute only an existing prepared deterministic `TeamRecoveryInput` and
  accepted RecoveryDecision.

The UI cannot manufacture a control merely because a button is visible.
Unavailable controls are disabled with a truthful reason. Stale view,
generation, Grant, attempt, request digest or prepared command fails closed.

## 6. Streaming and result presentation

The Candidate connects `app.NodeOutputObserver` to the accepted bounded
`api.TeamExecutionStream`:

```text
Runtime adapter frame
  -> Supervisor binding / sequence validation
  -> AgentGrant.Authorize
  -> BoundRunStream
  -> NodeOutputObserver
  -> bounded tentative delivery
```

Every live delta is labelled `tentative`. The product never publishes an
unauthorized, stale-generation, malformed or identity-drifting frame. Raw
Grant, credential, hidden reasoning and full sensitive prompt are prohibited.
Text deltas may be coalesced for a slow client; warning, approval, retry,
degraded, blocked, human-required and terminal milestones may not be dropped.
Overflow emits the accepted stream gap/artifact digest behavior. Tokens are not
written individually to the Journal.

Source Evidence, independent Verifier Evidence and the canonical terminal are
authoritative and must appear consistently in Mission Room, Timeline, History,
Compare and Attention after projection refresh.

## 7. Runtime and Provider claims

Current repository truth freezes three distinct live claims:

1. **Codex**: product path selects an installed Codex `native_auth` observation
   and passes exact preflight without importing OAuth material. This is not a
   Codex Runtime execution claim because no accepted Codex execution adapter
   exists in this baseline.
2. **MiniMax**: product path selects a brokered credential reference and passes
   one bounded non-generative verification plus exact preflight. The raw secret
   never enters the command, process environment snapshot, Journal or evidence.
   This is not a MiniMax execution-adapter claim.
3. **Pi**: product path performs one isolated controlled DAG execution through
   the accepted Pi 0.82.1 RPC/`loom.bridge.v1` adapter, Supervisor, Grant,
   authorized frames, source/Verifier Evidence and terminal aggregation.

Claiming Codex or MiniMax Agent execution would expand the frozen Runtime
adapter boundary and is not permitted by this contract.

## 8. Exact owned files

Only these product and test paths may change after Contract Review PASS:

### New application/API boundary

- `internal/app/local_product_execution.go`
- `internal/app/local_product_execution_test.go`
- `internal/api/local_product_execution.go`
- `internal/api/local_product_execution_test.go`
- `internal/api/local_product_read.go`
- `internal/api/local_product_read_test.go`

### Existing daemon and IPC composition

- `cmd/loomd/product_daemon.go`
- `cmd/loomd/product_daemon_test.go`
- `cmd/loomd/run.go`
- `cmd/loomd/run_test.go`
- `internal/localipc/protocol.go`
- `internal/localipc/protocol_test.go`
- `internal/localipc/swift_contract_test.go`

### Bubble Tea product path

- `internal/tui/model.go`
- `internal/tui/model_test.go`

### Strict Swift product path

- `apps/macos/Sources/LoomLocalAppCore/LocalProductExecutionModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- `apps/macos/Sources/LoomLocalAppCore/MissionOrchestration.swift`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`
- `apps/macos/Sources/LoomLocalAppContractProbe/main.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductExecutionModelsTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductModelsTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/MissionOrchestrationTests.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift`

### Governance/evidence

- `.loom-evidence/phase2a/P2A-W3/**`
- `docs/CURRENT.md`

No accepted authority, Journal, Projection, Rules, Work, Grant, Evidence,
Supervisor, Runtime adapter or bridge file is owned. Imports may compose those
packages through their existing exported APIs.

## 9. Mandatory RED and deterministic acceptance

Before product implementation, tests must fail only because the frozen W3
symbols/method are absent and must prove at least:

1. preflight is zero-write and binds exact view/Team/WorkPackage/policy data;
2. Home service health is derived from the actual daemon/Journal/projection
   open/rebuild path and round-trips through the strict Go and Swift models;
3. stale/incompatible/unconfirmed/offline/over-capacity preflight/start rejects;
4. two concurrent starts create one lineage and no duplicate authority facts;
5. client reconnect reads the same lineage without a second start;
6. daemon restart either resumes from exact reconstructable plan/semantic/
   recipe inputs or emits
   `human_required`, never a guessed dispatch;
7. post-start History/restart never fabricates a WorkPackage identity absent
   from Journal facts;
8. stale generation/control and replayed mutation reject;
9. authorized tentative frames publish; unauthorized/malformed/stale frames do
   not;
10. cancel joins the process and produces authoritative cancellation evidence;
11. prepared approval and bounded recovery use existing typed authorities;
12. source + independent Verifier Evidence drive the canonical terminal;
13. History/Compare/Attention agree with the same GlobalReadView;
14. strict Go IPC to the real Swift contract probe exercises
    `mission_execution` and rejects null collections, unknown fields,
    duplicate JSON keys and identity drift;
15. GUI and TUI require no internal IDs or Provider environment variables;
16. no raw secret/OAuth/Grant/hidden reasoning/sensitive prompt appears in
    source, arguments, environment snapshots, logs, Journal, Evidence or UI.

## 10. Verification gates

After RED, the Candidate must pass:

- focused Go tests for app/API/IPC/daemon/TUI;
- repeated concurrency and race tests for start/control/reconnect/cancel;
- real SQLite fixture E2E through
  `LocalProductReadService -> Go IPC Server -> Go + Swift clients`;
- deterministic loopback Pi component execution with Supervisor/Grant/frame/
  Evidence closure;
- Swift package tests and native app compile outside the repository build tree;
- whole-repository `go test`, `go test -race`, `go vet`, format and dependency
  checks;
- authority/import/scope/secret/protocol/platform checks;
- fresh independent Implementation Review `PASS`.

No live action is permitted before Implementation Review PASS.

## 11. Live gates

After Implementation Review PASS, freeze three separate source-locked manifests
and consume each at most once:

1. Codex native-auth product-path canary;
2. MiniMax brokered-verification product-path canary;
3. Pi isolated controlled-execution product-path canary.

Each uses a fresh `0700` attempt root, `0600` SQLite/manifest files, bounded
deadline and no hidden retry. The Pi canary additionally proves one explicit
start, authorized tentative output, source/Verifier Evidence, terminal,
reconnect without redispatch, process exit and socket/lock cleanup.

Then run one no-terminal native-window/TUI walkthrough covering WorkPackage,
Team selection, preflight, start, approval/recovery, result, History, Compare
and Attention without terminal data entry. A fresh independent Result Review
must return `PASS` before acceptance.

## 12. Stop conditions and rollback

Stop `HUMAN_REQUIRED` before implementation or live action if:

- a new Event field/schema or modification to an accepted authority is needed;
- exact restart input cannot be reconstructed without a second durable store;
- Codex/MiniMax execution is required to satisfy the parent claim;
- a credential or OAuth token would cross the accepted broker/native-auth
  boundary;
- the single W3 owned boundary cannot close the journey safely;
- the unrelated resident observer or user-owned dirty paths would be changed.

Rollback is the single atomic W3 commit plus attempt-local roots. Rollback must
not delete Journal history, user credentials, accepted Teams or unrelated
evidence. Material attempt roots are removed only after preserving reviewed
digests and only when deletion is explicitly within the live runbook.
