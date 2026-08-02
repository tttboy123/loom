# P2A-W3 Controlled Execution Experience Exit Gap Matrix

**Date**: 2026-08-01  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Status**: FROZEN INPUT — pending independent Contract Review  
**Scope**: current repository truth only; no product implementation has begun

## Classification rule

- `DONE`: accepted production code and accepted evidence already close the row.
- `PARTIAL`: an accepted lower-layer capability exists, but the ordinary-user
  product journey or current-lineage proof is incomplete.
- `MISSING`: no production path closes the capability.

Historical test fixtures prove reusable kernel behavior; they do not by
themselves make a product capability `DONE`.

## Parent Exit Contract reconciliation

| ID | State | Current evidence | P2A-W3 obligation |
|---|---|---|---|
| PX-01 | DONE | Accepted P2A-W1 native app, private default socket, native host and atomic socket/lock cleanup; accepted P2A-W2 attempt 004 launched signed app and daemon without terminal configuration. | Preserve; execution wiring must use the same daemon and must not signal the resident no-socket Runtime observer. |
| PX-02 | PARTIAL | `LocalProductReadService` and strict IPC expose Runtime, Provider, Team, Mission, Run, Evidence and Attention state. The current product snapshot has no explicit product-daemon/Journal health record. | Add truthful bounded service health derived from existing open/rebuild results, not a second heartbeat authority. |
| PX-03 | DONE | Accepted Board, Teams, Runs, History, Evidence, Compare, Attention and Timeline views are Projection-backed and bounded. | Preserve and populate them with the new execution lineage. |
| PX-04 | DONE | Accepted Team-bound Mission/Timeline projection and no-typed-ID selection passed native and TUI evidence. | Preserve strict Team scope validation. |
| PX-05 | DONE | `TeamExecutionStream`, cursor paging, gap records, coalescing and reconnect semantics are accepted. | Connect the execution observer to this existing stream; do not journal tokens. |
| PX-06 | DONE | Accepted once-per-question builder, save/edit/validate/confirm journey. | Start may select only an exact confirmed Team; drafts remain ineligible. |
| PX-07 | DONE | Accepted Team preview exposes exact Runtime/model/permission/compatibility inputs and fails closed. | Revalidate the same bindings at preflight and again at start. |
| PX-08 | DONE | Accepted Codex `native_auth` discovery and product presentation without OAuth extraction. | Canary proves selection and preflight through the product path; W3 does not import OAuth material. |
| PX-09 | DONE | Accepted MiniMax Keychain/Credential Broker configure, verify, replace and revoke flow. | Canary proves brokered credential selection and bounded verification through the product path; W3 does not copy the secret. |
| PX-10 | DONE | Accepted isolated Pi catalog, exact capability/model metadata and Pi 0.82.1 bridge compatibility. | Pi is the only currently accepted production execution adapter and is the controlled execution canary Runtime. |
| PX-11 | MISSING | Immutable `work.WorkPackage`, confirmed Teams, execution plan/authority and test-only full demo exist, but no production product service/IPC/UI starts a DAG. | Close typed WorkPackage selection, preflight and explicit start through one application service. WorkPackage remains pre-start proposal input; durable authority begins at accepted TeamExecution facts. |
| PX-12 | PARTIAL | Authoritative TeamExecution projection/timeline already expose node, Attempt, Run, generation, Runtime, recovery and Evidence; `NodeOutputObserver` carries authorized tentative frames. No production product start path feeds them. | Bind the accepted coordinator observer to the existing stream and product views. |
| PX-13 | PARTIAL | Existing authority provides approval claim fencing, Supervisor cancellation, cancelled Run terminal, deterministic retry/fallback/degraded/blocked/human-required recovery, stale-generation rejection and restart-safe CAS. No product control facade owns an in-flight execution lifecycle. | Add only prepared typed controls. `pause` means an accepted approval/claim fence at an authority boundary, never arbitrary process suspension. |
| PX-14 | PARTIAL | History/Compare already display projected Runtime/model/auth/Evidence/failure data. There is no real W3 execution outcome to compare and Skill revisions are shown only when already present in accepted state. | Prove one real lineage and preserve truthful omission of unavailable usage/cost/Skill data. |
| PX-15 | DONE | Attention is an existing Projection over actionable approval, blocked, human-required, retry and verification facts. | Preserve it; no UI-local notification authority. |
| PX-16 | PARTIAL | Native product install/start/exit and socket cleanup are accepted. Execution ownership, daemon restart reconciliation and bounded stop/cancel are absent. | Add daemon-owned, non-authoritative in-flight lifecycle and Journal-based reconciliation without creating a scheduler or queue. |
| PX-17 | MISSING | Provider onboarding and Pi component canaries exist, but not distinct W3 product-path gates bound to the W3 Candidate. | After Implementation Review PASS, freeze three separate manifests: Codex native-auth selection, MiniMax brokered verification, and Pi controlled execution. They are distinct claims; Codex/MiniMax are not falsely claimed as execution adapters. |
| PX-18 | MISSING | No W3 no-terminal walkthrough or whole-Phase2A review exists. | Run only after all W3 implementation and live evidence passes. |

## Reusable accepted kernel

The W3 Candidate must compose, not replace:

- `work.WorkPackage` and the accepted Coding/Knowledge package identities;
- confirmed Team/Agent/Runtime projection records;
- `teams.ExecutionPlan` and deterministic ready-set planning;
- `work.Authority.DispatchTeamReadySet`, generation/capacity CAS and recovery;
- `app.TeamCoordinator`, `supervisor.ManagedExecution`, AgentGrant and
  BoundRunStream;
- source/Verifier Evidence and terminal aggregation;
- `api.TeamExecutionStream`, `LocalProductReadService`, Mission projection,
  History, Compare and Attention;
- the accepted private local IPC, Bubble Tea client and strict Swift client.

## Semantic boundary discovered before freeze

There is no accepted generic process-suspension state. W3 therefore must not add
`paused` to a Team/Node/Run state machine in the UI, daemon or scheduler.
The only permitted `pause` behavior is a prepared Rules approval request that
fences a future claim or leaves an already-authoritative approval unresolved.
An already-running process may be explicitly cancelled, but not suspended and
resumed from an unrecorded process image.

There is also no Codex or MiniMax Runtime execution adapter in the current
repository. Codex and MiniMax W3 canaries therefore prove their exact accepted
authentication/provider product paths. Only Pi may claim controlled Agent
execution in P2A-W3. Any later Codex/MiniMax execution adapter requires a
separate future phase contract; it cannot be smuggled into this WorkItem.
