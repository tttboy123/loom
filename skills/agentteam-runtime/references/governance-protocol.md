# Governance protocol

> Role state machine + 5-element handoff envelope. Source:
> `loom-agent-team-runtime-proposal.md` §4 governance-protocol.md.

## State machine

```
Inbox -> Assigned -> In Progress -> Review -> Done
                                      |
                                      -> Failed
                                      -> Repair Requested
```

| From | To | Required condition |
|---|---|---|
| Inbox | Assigned | Owner assigned, scope defined, acceptance defined |
| Assigned | In Progress | Owner confirms context is sufficient (owner's own Blindspot Pass passes) |
| In Progress | Review | Artifacts complete, HandoffEnvelope emitted |
| Review | Done | All required gates passed by non-producer reviewers |
| Review | Repair Requested | At least one gate reported "blocking" with fixable issues |
| Any active | Failed | Failure cause and impact known, next-step owner assigned |

Two important properties:

1. **Gate agents transition `Review -> Done`**, not the producer.
   This is enforced both in spec and in the runtime via
   `devkit/state_writer.py` audit trail (the transition event records
   the actor).
2. **`Failed` and `Repair Requested` are distinct.** Failed = not
   fixable by current team; the system inserts a repair task in the
   backlog (via `devkit/repairer.dispatch`). Repair Requested = the
   same team can iterate.

## 5-element handoff envelope

Every cross-role transition uses this envelope. Adding less is a
hallucinated handoff — gate will reject.

```yaml
handoff:
  task_id: T2
  from: builder
  to: reviewer
  status: review
  completed:
    - "..."
  artifacts:
    - path: "outputs/..."     # relative to run root
      type: markdown
  verification:
    commands:                 # reproducible commands the reviewer can re-run
      - "..."
    evidence_refs:            # links into EvidencePacket (Loom primitive)
      - "..."
  known_issues:
    - "..."
  next_action:
    type: review | retry | escalate
    requested_by: reviewer
    eta_sla: 3600              # reviewer should act within this many seconds
```

The five elements (memorize as CANTS):

| Element | What it tells the reviewer |
|---|---|
| **C**ompleted | What the producer claims is done |
| **A**rtifacts | Where to find the work |
| **N**otes (verification) | How the reviewer verifies |
| **T**hings gone wrong (known_issues) | What the producer couldn't fix |
| **S**uggested next step (next_action) | What the producer is asking for |

A handoff missing any element is rejected by the gate.

## Producer-vs-reviewer rule

The reviewer role **cannot** be the same as the producer role. This
is enforced at:

1. **TeamSpec authoring time** — `scripts/validate_team_spec.py`
   refuses specs where the gate's `reviewer` is also a task's `owner`.
2. **State-write time** — `devkit/state_writer.py` records the actor
   of every transition; the gate verifies actor ≠ producer at handoff.

This is the simplest defense against the "model self-praises"
anti-pattern (see `failure-modes.md` §"Self-approving producer").

## Refusal rules

A handoff is refused if:
- Any of the 5 elements is missing or empty.
- `artifacts` paths reference files that don't exist.
- `verification.commands` references commands not on the
  `AllowedCommands` allowlist.
- The `to` role is the same as the `from` role (in a static team;
  a producer may not self-handoff for review by themselves).
- The gate required for `Review -> Done` has not been declared.

## Empirical validation

When the team is first deployed, sample 10 historical handoffs (from
`.loom/runs/`) and run `scripts/lint_handoff.py` against them. A clean
pass means:

- All handoffs have 5 elements.
- Reviewer ≠ producer in all handoffs.
- All required gates fire before `Done`.

If 80%+ of handoffs need rework, the team is missing discipline and
the routing should bias toward fewer roles + stricter templates.
