# Phase 1 Engineering Acceptance Audit

Status: `READY_FOR_FINAL_USER_SIGNOFF`

This audit covers engineering evidence only. It does not claim a real
installed Runtime task, Provider/model response, credential readiness, live
external effect, customer satisfaction, or user signature.

## TECH-PLAN section 15

| # | Status | Engineering evidence |
|---|---|---|
| 1 | DONE | Ordinary conversation creates no Team/Agent execution object; router, CLI, and final canary tests pass. |
| 2 | DONE | Explicit saved-Team selection builds a direct TeamInstance plan without a model Team Draft. |
| 3 | DONE | Explicit Agent use without a saved Team resolves to the Team Draft path; no process exists before acceptance. |
| 4 | DONE | Team Draft has one unresolved question per revision and answers advance revision. |
| 5 | DONE | TeamInstance projection and saved-Team instantiation enforce exactly one Main. |
| 6 | DONE | Main has no deliverable-write grant/tool path; execution grants remain Run-scoped. |
| 7 | DONE | AgentDefinition role identity is independent from RuntimeProfile selection. |
| 8 | DONE | Binding requires a compatible online RuntimeInstance; offline/incompatible bindings fail. |
| 9 | DONE | Draft model/Skill/member/Runtime references resolve against bounded snapshots; invented IDs fail. |
| 10 | DONE | Every activated SubAgent has a logical node plus independent WorkItem, Run, Grant, and Evidence lineage. |
| 11 | DONE | Agent execution uses AgentGrant rather than Client/Daemon identity and is limited to current-Run operations. |
| 12 | DONE | Old claim-generation start, Frame/Evidence, heartbeat, and terminal paths fail closed. |
| 13 | DONE | Expired prepare lease produces one fenced generation; late old output cannot replace current state. |
| 14 | DONE | Executor can submit `ready_for_review` but cannot mark its own WorkItem Done. |
| 15 | DONE | `require_approval` persists across new Authority/Projection instances and resumes only after an accepted decision. |
| 16 | DONE | Duplicate/replayed inputs do not duplicate Events, Runs, Grant lifecycle, Evidence, Done, or the test effect marker. |
| 17 | DONE | Cancel/timeout cleanup terminates managed processes, commits terminal state, and revokes AgentGrant. |
| 18 | DONE | Client delivery may disconnect while the Run continues; cursor plus Projection reconnect returns the same authoritative result. |
| 19 | DONE | High-risk acceptance uses independent verifier Runtime/Run/Grant/Evidence lineage. |
| 20 | DONE | `native_auth` remains explicitly distinct from Broker isolation; no UI misrepresents it. |
| 21 | DONE | Journal rebuilds task/board state, committed Evidence digests resolve to immutable artifacts, and private modes are `0700`/`0600`. |
| 22 | DONE | Coding and knowledge-work packages use the same authority/state-machine trace and differ only by frozen WorkPackage fields. |

## Slice 5 capability status

| Capability | Status |
|---|---|
| Versioned local event interface | DONE |
| Authorized tentative node output | DONE |
| Journal-authoritative timeline and reconnect | DONE |
| Slow-consumer recovery and stream gap | DONE |
| Local board and Attention CLI | DONE |
| Coding/knowledge WorkPackage parity | DONE |
| Controlled Phase 1 engineering demo and restart | DONE |

## Phase 1 exclusions

The following remain unimplemented and unauthorized as required:

- real Credential Broker Provider delegation;
- Provider automatic fallback;
- TUI/Web UI;
- Sidecar automatic evaluation or activation;
- cloud synchronization;
- shared marketplace or multi-user permissions;
- automatic persistence of temporary Agents;
- model-context/session/process-image checkpoint authority;
- incremental Projection checkpoints;
- standing orders/Autopilot;
- external notification integrations;
- unreviewed third-party Skill execution;
- per-token Journal or Sidecar recording.

## Remaining final human gate

The exact unresolved inputs are:

```text
installed_runtime_instance_id
private_source_root
approval_decision
live_execution_authorization
final_review_signoff
```

Until the user supplies those inputs and explicitly authorizes the bounded
live task, Phase 1 is not `COMPLETE`.

VERDICT: PASS
