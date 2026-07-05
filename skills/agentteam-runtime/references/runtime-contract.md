# Runtime contract

> The data objects a Loom team run emits and consumes. Source:
> `loom-agent-team-runtime-proposal.md` §4 runtime-contract.md.

## Object catalog

| Object | Purpose | Persistence |
|---|---|---|
| `TeamSpec` | team definition | `team.yaml` |
| `TeamSession` | one team run | run root |
| `RoleProfile` | agent role | part of TeamSpec |
| `TaskGraph` | task DAG | `task-board.yaml` snapshot |
| `TaskState` | state machine node | inside TaskGraph |
| `MailboxMessage` | inter-role communication | `mailbox.jsonl` |
| `HandoffEnvelope` | cross-role transition | `handoffs/<task>.yaml` |
| `EvidencePacket` | verifiable proof | per-task subdirectory |
| `GatePolicy` | review strategy | `gate-policy.yaml` |
| `RunTrace` | full audit trail | `trace/events.jsonl` |
| `ImprovementCandidate` | self-improvement proposal | `improvement/candidates/` |

Existing Loom primitives are **referenced, not redefined**:
- `devkit/protocol_schemas/incident.schema.json`
- `devkit/protocol_schemas/goal_spec.schema.json`
- `devkit/protocol_schemas/evidence_packet.schema.json`
- `devkit/repairer.WHITELIST_ACTIONS` (5 actions)

If a new object is needed that does not map to one of these, file a
proposal in `references/` first; do not invent in-line.

## Run directory layout

```
.loom/runs/<run_id>/
  team.yaml                          # TeamSpec
  task-board.yaml                    # live TaskGraph snapshot
  mailbox.jsonl                      # append-only mail log
  handoffs/
    T1-orchestrator-to-builder.yaml
    T2-builder-to-reviewer.yaml
  evidence/
    T1/
      commands.jsonl
      outputs/
      review.md
    T2/
      commands.jsonl
      outputs/
      review.md
  trace/
    events.jsonl                    # RunTrace events
    metrics.json                     # duration, cost, token counts
  gates/
    plan-gate.md
    acceptance-gate.md
  improvement/
    candidates/
      ic_<id>.yaml                   # ImprovementCandidate, never auto-applied
```

## RunTrace event schema

Each event is appended to `trace/events.jsonl`:

```json
{
  "event_id": "evt_00042",
  "timestamp": "2026-07-06T12:00:00+08:00",
  "run_id": "run_20260706_001",
  "task_id": "T2",
  "actor": "builder",
  "type": "tool_call",
  "summary": "...",
  "inputs_ref": "trace/inputs/evt_00042.json",
  "outputs_ref": "trace/outputs/evt_00042.json",
  "cost": {"input_tokens": 12000, "output_tokens": 1800},
  "result": {"status": "success"}
}
```

Event types: `task_state_change`, `tool_call`, `mailbox_message`,
`gate_evaluation`, `incident_opened`, `incident_resolved`,
`improvement_candidate_emitted`.

## Mailbox semantics

A `MailboxMessage` is not a side-channel for tool calls. It is a
domain object that:

- Carries `from`, `to`, optional `task_id`, `body`, `references`.
- Is **append-only** — no edits or deletes after emission.
- Is threadable via `in_reply_to` (optional).
- Has priority `low | medium | high | urgent` (default `medium`).

The runtime mediates routing: messages to retired roles fail; messages
to `lead` get answered last in the lead's cycle.

## Why this matters

- **Gate agents can replay a run by reading the run root.** Every
  decision, every tool call, every cost is recoverable from disk.
  Without this, gates degenerate to opinion-based approval.
- **Self-improvement loops read this layout.** Candidate improvements
  must cite specific traces, evidence paths, and gate verdicts. A
  run without `.loom/runs/<id>/` produces no candidates.
- **Cost and model observability.** `trace/metrics.json` and per-event
  cost fields are the substrate for cost control (see the
  control-theory blueprint's cost model in the companion doc).

## What is NOT in this skill

- The actual `devkit/` runtime code (Phase D covers this).
- GoalSpec decomposition (see the Loom Blueprint).
- Repairer dispatch (see `devkit/repairer.py` + Phase 4 acceptance).
