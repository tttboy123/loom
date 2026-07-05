# Implementation phases

> The 4-phase build-out. Source: `loom-agent-team-runtime-proposal.md`
> §9 + §11 acceptance criteria.

## Phase 0 — Skill & template pack (no Loom runtime change)

**Deliverables**:
- `team.yaml` template
- `task-board.yaml` template
- `handoff.yaml` template
- `unknowns-map.yaml` template
- `improvement-candidate.yaml` template
- `gate-policy.yaml` template
- `scripts/validate_team_spec.py`
- `scripts/lint_handoff.py`

**Acceptance**:
- `validate_team_spec.py` rejects specs where reviewer == producer.
- `lint_handoff.py` rejects handoffs missing any of the 5 elements.
- A user can author a team spec from a written goal in < 5 minutes.

## Phase 1 — Runtime state files

**Deliverables**:
- `.loom/runs/<run_id>/` directory created on `loom team run`.
- `loom team status <run>` reads from `.loom/runs/<run_id>/team.yaml`
  + `task-board.yaml` + `mailbox.jsonl`.
- Cross-role handoff writes HandoffEnvelope to
  `.loom/runs/<run_id>/handoffs/<task>.yaml`.
- Acceptance gate writes gate report + refuses Done without it.

**Acceptance**:
- A team run starting from scratch leaves all required files in
  `.loom/runs/<id>/`.
- No code in `devkit/` is replaced; this phase only adds new
  directory + write primitives.

## Phase 2 — Cockpit

**Deliverables**:
- TUI or web cockpit rendering the 7 views.
- Live mailbox read-and-respond.
- Task board state transitions (with reason field).
- Gate panel with drill-down to evidence.
- Cost / Latency panel.
- Improvement panel.

**Acceptance**:
- A user can run a team, watch its cockpit, intervene via message,
  and the run reaches Done or runs out of budget.

## Phase 3 — Self-improvement loop

**Deliverables**:
- Improvement candidate generator reading `trace/events.jsonl`.
- Eval patch generator adding regression test per candidate.
- Canary promotion flow (candidate -> canary run -> promote/rollback).
- Human / `frontier_safety_consulted` approval flow.

**Acceptance**:
- Candidates emit only — no auto-promote.
- Each candidate has an eval that gates its promotion.
- Permission / safety / routing / prompt changes require `frontier_safety_consulted`
  approval (no exemption).

## Phase dependencies

Phase 0 → can start today.
Phase 1 → needs Phase 0 + the existing Phase D gatekeeper / scheduler.
Phase 2 → needs Phase 1 + a terminal or web renderer.
Phase 3 → needs Phase 1 + Phase 2 (need run traces to generate from).

## What is "done"

A run reaches Done when ALL of the following are true:

1. TeamSpec exists in `.loom/runs/<id>/team.yaml`.
2. Unknowns Map exists and was reviewed at plan_gate.
3. Every task has owner, state, acceptance.
4. Every cross-role transition has HandoffEnvelope (5 elements).
5. Critical artifacts have EvidencePacket (source ≥ materialized_repo).
6. Producer is never the only reviewer.
7. Every gate report is resolve to pass / fail / repair_requested.
8. RunTrace is replayable (cost, latency, errors reconstructible).
9. Self-improvement candidates emitted only — none auto-applied.
