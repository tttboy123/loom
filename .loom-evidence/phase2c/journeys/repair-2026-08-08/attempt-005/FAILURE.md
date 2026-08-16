# Phase 2C Repair Journey Attempt 005 Failure

**Date**: 2026-08-08  
**Journey ID**: `0b1c185f-59a0-4727-a0df-5c747de124d3`  
**Fixture root**: `/private/tmp/loom-phase2c-chat-20260808-005`  
**Source-lock SHA-256**: `b03dac95666bc590907149ba8f5604a4f97682a06207dcfa741c25cafaa3db68`  
**Ordered source digest**: `2afe8693ac07ed12d21fcc981485c9e8d1f2b7a61cde519c87238294cbd31971`  
**Outcome**: `FAIL - AMBIGUOUS_MISSION_START_CONTINUED_AFTER_ERROR`

## Failure

Repair 6 passed its complete source-lock-bound deterministic matrix. Attempt
005 then passed native and real-PTY chat-first entry, safe folder selection,
offline/reconnect continuity, and Team Draft creation without authority facts.
The TUI explicitly confirmed the Team and received a successful Mission
preflight. The Journal remained at six facts before the user explicitly chose
`Start`.

The explicit `mission_execution:start` request crossed the daemon boundary at
monotonic offset `735543967` microseconds. At `740598647`, after `5054680`
microseconds, the daemon returned `state_unavailable` with no authority event
IDs. The TUI therefore told the user that state was unavailable.

That response was false as an operation outcome. The daemon-owned background
flight continued after the response and later committed eight authoritative
facts for the same Journey and Team: `TeamExecutionPlanned`,
`TeamNodeAttemptScheduled`, `WorkItemCreated`, `WorkItemAssigned`,
`WorkRunIdentityReserved`, `RunClaimed`, `RuntimeCapacityReserved`, and
`TeamReadySetDispatched`. A later snapshot showed the Team as `Running`.

The lineage then stopped before `AgentGrantIssued`, `RunStarted`, any terminal
Run fact, evidence finalization, or a Team terminal/recovery fact. Its attempt
capture contains the correct binding but zero frames and `terminal: null`.
The user therefore received a retry-shaped failure while authority had already
accepted and partially dispatched the operation. Retrying could not be reasoned
about safely from the client response alone.

The production call order explains the split. `StartMission` launches a
daemon-owned flight and waits five seconds for a projected Team execution.
`productMissionExecutionRunner.Run` synchronously constructs the Mission
executor before entering `TeamCoordinator.Run`. Executor construction may wait
up to 60 seconds for the shared local model to become healthy. No
`TeamExecutionPlanned` fact exists during that wait. The projection timeout
returns `ErrTeamExecutionIncomplete`, mapped to `state_unavailable`, without
cancelling or joining the still-live flight. Once the model becomes ready, the
flight enters the coordinator and writes the execution facts.

Increasing the five-second projection timeout to ten seconds does not close
this defect: local-model startup is allowed to take 60 seconds, the IPC client
still has a bounded deadline, and a cancelled caller can leave the daemon-owned
flight running. Repair 7 must make the authoritative acceptance boundary
observable before slow runtime initialization and must terminalize initialization
failure through the existing Run, Grant, Evidence, and Team lifecycle.

## Completed Journey Evidence

- J1 Native and TUI: both opened chat-first with no folder requirement and
  completed ordinary tentative conversation.
- J2 Native: the system directory picker selected `Documents` and displayed
  only the safe folder name. J2 TUI used the controlled folder fixture. The
  private sentinel was not present in daemon, IPC, or TUI logs.
- J3 Native and TUI: both truthfully showed service unavailability while the
  daemon was stopped, preserved workspace state, and reconnected to the same
  restarted daemon without duplicating runtime identity facts.
- J4 Native and TUI: both completed the explicit Agent Team builder to a ready
  draft. The Journal still contained only the three initialization/runtime
  facts before confirmation.
- J5 through preflight: TUI confirmation added exactly
  `TeamDefinitionSaved`, `TeamInstanceCreated`, and `AgentInstanceCreated`.
  Mission preflight was read-only and showed runtime, permissions, approval
  points, and plan. The Journal remained at six facts until explicit `Start`.
- J5 start: failed at the ambiguous acceptance boundary described above.
- J6-J10: not attempted after the first P0 journey defect was proven.

## Additional Review Findings

1. **P1 - stale cross-client Team Draft**: after the TUI confirmed the Team,
   the still-open native draft retained its old view version. Three explicit
   native confirm attempts returned `conflict` and wrote no facts. Repair 7
   must collapse a stale confirm into one actionable conflict state, refresh
   authoritative setup state, and prevent repeated submission of the same
   stale draft until the user deliberately starts a new draft.
2. **P2 - tool-shaped tentative prose**: one ordinary model response displayed
   raw JSON shaped like a tool request. It was tentative text only, executed no
   tool, read no sentinel, and wrote no authority facts. The UI must not render
   tool-shaped model prose as an action affordance; it must remain visibly
   untrusted or be replaced by a sanitized recoverable response.

## Authority and Shutdown Audit

- Final Journal events: exactly `14`; the first six are initialization/runtime
  and confirmed-Team facts, and the final eight are the partial execution
  lineage listed above.
- Duplicate event IDs and idempotency keys: `0`.
- `AgentGrantIssued`, `RunStarted`, terminal Run, finalized Evidence, and Team
  terminal/recovery facts: `0`.
- SQLite `PRAGMA integrity_check`: `ok`.
- Attempt capture: one bound capture, zero frames, `terminal: null`, and empty
  finalized digest.
- The private J2 sentinel appeared only in its source file.
- The Repair 6 source digest revalidated exactly before shutdown.
- TUI, Native, daemon, and shared local-model processes all stopped. The
  journey socket and socket ownership lock were absent after shutdown.

## Artifact Bindings

| Artifact | SHA-256 |
|---|---|
| `tui/transcript.txt` | `2ef09dbc42a783684cee2efa6f1dca32256583a2605b00881486c23d02d2715c` |
| `ipc/request-response-summary.jsonl` | `403e534c337c2880bd744368f2d896e78a0748632bcf077f9bf16864eb21b998` |
| `daemon/structured-log.jsonl` | `4031d1452a7b86d79b02da4544d34852040344a08e76e2215436030f52fb6a3d` |
| `state/loom.db` | `aafe0be36b20491920d0f7e975b9dc764a596e3b99a34d54c646b0fe38fb1b93` |
| `state/chat-threads.json` | `838ff868281e374e6fd000e4a00d15f2a9800f88c4ceb459f65aa095cdc2c22e` |
| Attempt capture | `514105a217e496712e4d6e8066717085957cd68df14792081623380143f3a2f0` |
| `source/source-lock.json` | `b03dac95666bc590907149ba8f5604a4f97682a06207dcfa741c25cafaa3db68` |
| `bin/loom` | `9dd4ea8930160094f649c3bdea470a6aa059c47f7a042d484b0e8f518780d0be` |
| `bin/loomd` | `a96d0df6390fc784d90d6a234ce31b742730c45f13b412f510567eda0dbaade8` |
| Native executable | `cd79df38f5277873cfd02822e5e0f7c5ff9477725ee9444a6a73ac53a2859a1f` |
| Native J1 chat-first screenshot | `16f4eddd31d5286c71f17314c4d97940c10fd9f9d241c30547a77954151a31cc` |
| Native J2 folder-picker screenshot | `574d88d5de51b251575f35de14802d20fda635b0fd29a60d835fa4434d9f684c` |
| Native J3 offline screenshot | `7043b1e3b8b1f3f912e8fd704d94b688c6bd9aebc13f69527b1751a4f7c068fb` |
| Native J3 reconnected screenshot | `20c05b9148ef337cd8d102ec91fe8b4f25f9e508ab05f8fdfb1d1a974b5b8fa6` |
| Native J4 ready-draft screenshot | `a4c766f7d06c4a84e40c62c0ce0b5a8351cd67670719b2d2759f8983b82c3ffe` |

`daemon/loomd-sample.txt` is a local diagnostic only and is not acceptance
evidence. No screenshot or partial result from this consumed attempt is
promoted. Attempts 001-005 remain immutable historical records.
