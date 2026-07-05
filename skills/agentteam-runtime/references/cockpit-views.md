# Cockpit views

> The user-facing surface of a team run. Source:
> `loom-agent-team-runtime-proposal.md` §6.

## Why the cockpit matters

Loom runs are not just artifacts on disk. They are observable to
both the user (human) and adjacent agents (in cluster mode). The
cockpit is what makes the team feel like a team, not a hidden batch
job. Claude Agent Teams got this right and Loom inherits the shape.

The cockpit is **a runtime projection of `.loom/runs/<id>/`**. There
is no parallel state — what you see in the cockpit is exactly what is
on disk.

## Required views (build all seven)

| View | What it shows | Key interaction |
|---|---|---|
| Team Roster | Roles, model policy, permissions, current task assignment | Filter, sort, focus |
| Task Board | Task graph + state machine in real time | State transition with reason; click to drill |
| Mailbox | Append-only inter-role message stream | Inject user message; jump to referenced task |
| Evidence | Producer artifacts + reviewer verdicts | Open file, view diff, view commands |
| Gates | Plan / handoff / evidence / acceptance / improvement verdicts | Drill into blocking rule; jump to evidence |
| Cost / Latency | Per-role, per-task cost + token consumption | Compare runs; spot overruns |
| Improvement | Improvement candidates emitted by recent runs | Approve / reject with note |

A view that doesn't have a README entry in `runtime-contract.md`
probably shouldn't be there.

## User intervention principles

The user can do these at any time:

1. Send a message to the Lead (no approval needed).
2. Send a message to any other role (no approval needed).
3. Request that a specific task be re-reviewed.
4. Approve a low-risk improvement candidate.
5. Refuse to approve any gate.

The user **cannot**:

1. Demand a Done verdict without a passing acceptance_gate.
2. Overrule a blocking rule without the gate's reviewer agreeing to
   lift it (logged as a `gate_override` event, never silent).
3. Bypass the producer-must-not-self-review rule.

## Layout suggestion

Default cockpit layout:

```
+--------------------+--------------+---------------+
| Team Roster        | Task Board   | Cost / Latency |
+--------------------+--------------+---------------+
| Mailbox           | Evidence      | Gates         |
+-------------------+--------------+---------------+
|                                            |
|               Improvement                  |
+--------------------------------------------+
```

All seven views fed by the same `.loom/runs/<id>/` directory. No
hidden state. Click-drill anywhere.

## Implementation status

The cockpit is part of **Phase 2** of the implementation plan. The
data model (.loom/runs/<id>/) ships in Phase 1. Views land in
Phase 2.

## What the cockpit is NOT

- Not a chat interface — chat lives at `loom team message --to <role>`.
- Not an editor — edits flow through the producer role + handoff.
- Not a dashboard for ops monitoring — that's `devkit/logs/` and
  `/api/agent-observability`.
