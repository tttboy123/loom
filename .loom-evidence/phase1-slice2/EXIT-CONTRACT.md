# Phase 1 Slice 2 Exit Contract

- Frozen against branch: `codex/loom-platform-slice2`
- Frozen baseline: `39a9e0a`
- Date: `2026-07-25`
- Authority: `TECH-PLAN.md` section 14, Slice 2
- Purpose: stop horizontal WorkItem growth and close Slice 2 with one vertical
  Runtime-observation capability

This is a governance checkpoint, not a new product WorkItem. It preserves the
accepted S2-W1 through S2-W38 code and evidence. It does not reopen those
contracts or authorize Slice 3.

## Status vocabulary

- `DONE`: the Slice 2 boundary is implemented and has accepted code-level or
  controlled persistence proof. No further Slice 2 WorkItem is required.
- `PARTIAL`: accepted prerequisites exist, but the user-observable vertical
  capability or its lifecycle proof is incomplete.
- `MISSING`: no accepted implementation or sufficient verification exists at
  the frozen baseline.

“Controlled persistence proof” means real local SQLite/Journal execution in an
isolated test state directory. It is not evidence of a resident daemon or an
installed user Runtime. “Controlled live verification” means starting the real
compiled daemon entry with isolated configuration and state, observing real
clock/process/SQLite behavior, canceling it, restarting it against the same
state, and inspecting the rebuilt projection. A unit test alone is not live
evidence.

## TECH-PLAN Slice 2 capability ledger

| Capability | Status | Accepted implementation | Verification and evidence | Exit treatment |
|---|---|---|---|---|
| `AgentDefinition` / `RuntimeProfile` / `RuntimeInstance` separation and binding | `DONE` | `internal/agents/definition.go`, `internal/runtime/catalog.go` | `internal/agents/definition_test.go`, `internal/runtime/catalog_test.go`, S2-W1 evidence | Closed |
| Project, reusable, and temporary/transient Agent scopes | `DONE` | `ScopeProject`, `ScopeReusable`, `ScopeTransient` and fail-closed resolution in `internal/agents/definition.go`; saved-Team scope resolution in `internal/teams/team_definition.go` | scope precedence, identity, shadowing, and transient-generation tests in `internal/agents/definition_test.go`, `internal/teams/team_definition_test.go`, and saved-Team tests | Closed |
| Default Main Agent selection and exactly-one-Main records | `DONE` | default-Main Draft seed in `internal/teams/resolver.go`; exact Main cardinality in accepted Draft/saved-Team plans and instance records | `internal/teams/resolver_test.go`, `internal/teams/instantiation_plan_test.go`, `internal/teams/saved_team_instances_test.go`, S2-W9/S2-W10/S2-W13 evidence | Closed; no Agent process is started |
| Defined Team direct load | `DONE` | resolution → online Runtime binding → instantiation plan → immutable Team/Main records → atomic Journal commit → rebuildable projection in `internal/teams`, `internal/state/saved_team_writer.go`, and `internal/projection/projection.go` | saved-Team binding/plan/record tests, real SQLite writer tests, projection rebuild/restart tests, S2-W8 through S2-W16 evidence | Closed at the Slice 2 state-authority boundary; writable CLI and Agent execution are not claimed |
| Bounded catalog snapshot with invented-ID rejection | `DONE` | `internal/teams/catalog.go` and structured content validation | `internal/teams/catalog_test.go`, `internal/teams/draft_content_test.go`, S2-W3/S2-W5 evidence | Closed |
| Live Team Draft revisions, one unresolved question, answer/edit, and explicit accept/reject/expire | `DONE` | `internal/teams/draft.go`, `internal/teams/structured_draft.go`, `internal/teams/draft_decision.go` | Draft revision, question cardinality, stale command, catalog revalidation, and explicit decision tests; S2-W4/S2-W6/S2-W7 evidence | Closed at the versioned domain boundary; model output remains non-authoritative |
| Isolated local Pi metadata discovery for version, models, and capabilities | `DONE` | bounded probe core, isolated process runner, configured fixed-name locator, deterministic discovery and canonical writers/projections in `internal/runtime`, `internal/runtime/piadapter`, `internal/runtime/discoveryscan`, `internal/state`, and `internal/projection` | S2-W17 through S2-W31; real child-process fixture tests and real SQLite discovery/status writer and replay tests | Closed as reusable components; no resident scheduling is claimed |
| Projection-aware discovery/status observation cycle | `DONE` | write priority, writer selection, configured one-shot observation, prepared observer, injected trigger, and same-projection pre/post refresh in `internal/app` | S2-W32 through S2-W38; direct none/discovery/status proofs and real SQLite discovery→rediscovery→status chains | Closed as a serial one-shot seam |
| One daemon automatically discovers local Runtime metadata | `PARTIAL` | all one-shot discovery/status/projection components are accepted | no concrete clock, recurrence owner, validated daemon configuration, lifecycle entry, restart/recovery canary, or controlled live daemon evidence exists at `39a9e0a` | Must be closed by the sole remaining merged WorkItem below |

## Sole remaining product WorkItem

Title: `Local Runtime Observation Daemon Integration`

This is the only additional Slice 2 product WorkItem authorized by this exit
contract. It must be one Candidate and one acceptance boundary. It must not be
split into W39/W40-style wrappers, and it must not defer any item below to a
later WorkItem.

| Required closure | Baseline | Required result |
|---|---|---|
| Concrete trigger and clock | `MISSING` | A production clock/trigger owns initial observation and bounded recurrence; tests use an injected fake/manual clock without sleeping |
| Validated configuration | `MISSING` | Explicit state path, trusted Pi search directories, device/instance identity, observation interval, process timeout, and bounded Event metadata sources are validated before side effects; ambient `PATH`, `HOME`, cwd, credentials, and user Pi state are not implicit configuration |
| Daemon lifecycle | `MISSING` | A real non-auto-starting daemon object and compiled entry run serially, reject duplicate concurrent start, expose deterministic startup failure, and stop on context/signal cancellation without orphaned work |
| Projection refresh and discovery/status recurrence | `PARTIAL` | Compose the accepted S2-W38 one-shot boundary; rebuild committed projection before each decision and after success; never overlap cycles |
| Cancellation | `MISSING` | Cancellation while waiting and in-flight reaches the trigger, probe, process runner, writer, and rebuild boundaries; child process groups and temporary state are cleaned |
| Restart | `MISSING` | A new daemon instance opens the same Journal, rebuilds projection before observation, and continues without losing or shadowing accepted facts |
| Recovery | `MISSING` | A controlled failed observation or interrupted process does not corrupt Journal/projection state; the next explicitly scheduled cycle or restarted daemon rebuilds before deciding what to write; no blind same-input retry is allowed |
| No Runtime activation | `PARTIAL` | Static and runtime proof show the daemon performs metadata-only `--version` / offline model-list observation and Event/projection updates only; it never starts an Agent session, calls a model, mutates user Pi state, or creates Slice 3 resources |
| Controlled live verification | `MISSING` | Build and run the real daemon entry with isolated state and a deterministic local Pi-compatible executable; observe at least discovery, restart/rebuild, status or inventory change, cancellation, and final Journal/projection facts. If a real configured Pi installation is absent, state that explicitly; the isolated executable proves the daemon lifecycle, not user Pi readiness |

## Required tests and evidence for the merged WorkItem

The Candidate must include all of the following in one contract and review:

1. Unit and race tests for configuration, clock/trigger, serialization,
   cancellation, startup failure, lifecycle idempotence, restart, and recovery.
2. Real SQLite integration covering:
   initial discovery → no-change → inventory rediscovery → status change,
   with projection rebuilt from committed Journal facts.
3. Process cleanup proof for cancellation and timeout.
4. Static proof that no Agent execution, model call, credential access, ambient
   Runtime discovery, Slice 3 resource, autonomous activation, or overlapping
   observation authority was added.
5. A controlled live canary log and inspection transcript under
   `.loom-evidence/phase1-slice2/daemon-integration/`.
6. Focused, impacted-package, repeated race, repository, repository-race, vet,
   format, diff, and exact-scope verification.
7. One fresh independent implementation Reviewer after the complete Candidate,
   followed by the Slice 2 exit review below.

The integration contract may reopen existing observation composition files
only where required to close lifecycle semantics. Thin forwarding functions,
constructor-only wrappers, or separate scheduler/entry/config WorkItems are
forbidden.

## Slice 2 exit gate

Slice 2 may be marked `PASS` only when:

1. every ledger row is `DONE`;
2. every required closure in the merged WorkItem is proven in the same
   Candidate;
3. the controlled live canary has been run and its limitations are stated;
4. a fresh independent Reviewer audits the whole Slice 2 capability against
   `TECH-PLAN.md`, current code, accepted evidence, and live evidence;
5. the Reviewer returns `PASS` with no blocking findings; and
6. `docs/CURRENT.md` no longer claims that daemon discovery is only a target.

If the merged WorkItem cannot close this list coherently, stop at
`HUMAN_REQUIRED`. Do not create another thin Slice 2 WorkItem to route around
the failed exit gate.

## Explicit non-authority

This contract does not authorize:

- Runtime or Agent execution;
- Bridge, WorkItem dispatch, AgentGrant, claim generation, prepare lease,
  managed workspace, or other Slice 3 behavior;
- credentials, Provider calls, model calls, paid work, network mutation, or
  user Pi state mutation;
- resident service installation, launch-at-login activation, push, merge,
  release, or publication.

Controlled foreground daemon canaries using isolated configuration and state
are authorized solely for the exit evidence above.
