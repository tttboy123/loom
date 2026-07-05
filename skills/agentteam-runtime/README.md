# agentteam-runtime

> Loom orchestration for the single / team / cluster operating modes.
> Derived from `loom-agent-team-runtime-proposal.md` (2026-07-05).
> Status: discussion draft, skill packaging in progress.

## When to invoke this skill

Activate when the user wants any of:

- A **multi-agent run** that should follow team protocol (handoff envelope,
  gate review, role separation).
- A **trace-and-evidence** orchestration where every handoff and every
  gate decision must be recorded.
- A **gate-driven acceptance** flow for an artifact bundle (research,
  code change, design doc, improvement candidate).
- A **cockpit view** of who is doing what in a team run.

Do NOT use this skill for:

- One-shot LLM calls (use the model's native prompt).
- Code edits with no protocol needs (use Codex / Claude Code directly).
- Self-modification of Loom runtime (out of scope; gated by policy).

## Routing map

The skill is split into focused references. Pick the one closest to your
question — do not read end-to-end.

| If you want to… | Read |
|---|---|
| Understand why this skill exists and its design source | `references/three-sources-capability.md` |
| Author or review a `team.yaml` | `references/teamspec.md` + `assets/templates/team.yaml` |
| Understand how roles hand off work | `references/governance-protocol.md` (state machine + 5-element handoff) |
| Author or review a `handoff.yaml` | `assets/templates/handoff.yaml` (plus the 5 elements explained) |
| Define or audit `gate-policy.md` content | `references/gate-policy.md` |
| Understand the runtime objects (TaskBoard, Mailbox, RunTrace) | `references/runtime-contract.md` |
| Plan what evidence reviewers must produce | `references/evidence-flow.md` |
| Author or review `unknowns-map.yaml` | `references/unknowns-map.md` |
| Design the cockpit views | `references/cockpit-views.md` |
| Pick a model per role | `references/model-routing.md` |
| Plan the build-out | `references/implementation-phases.md` (4 phases) |
| Harden against known failure modes | `references/failure-modes.md` |
| Write the self-improvement candidate loop | `references/self-improvement-loop.md` |

## How this skill avoids the common pitfalls

1. **No three-source lock-in.** MiniMax contributes governance protocol
   patterns; Claude contributes team UX patterns; Codex contributes
   acceptance-gate patterns. The runtime absorbs all three rather than
   committing to any one. See `three-sources-capability.md`.
2. **Producer cannot self-approve.** The gate policy makes a fresh-context
   reviewer mandatory. Same as the Loom Blueprint Phase 4 acceptance.
3. **Self-improvement is candidate-only.** No patch (skill, prompt,
   routing, permission, safety rule) is applied without explicit gate
   pass and a documented eval plan. See `self-improvement-loop.md`.
4. **Unknowns must surface before planning.** `unknowns-map.md` is the
   first artifact every run emits, not an afterthought.
5. **Failures are catalogued, not improvised.** 18 failure modes are
   enumerated with detector + mitigation + metric + owner. New failure
   modes get the same structure before they're fixed.

## Cross-references

- **Consumes**: `boundary-aware-agent` (per-agent Blindspot Pass +
  capability boundary card)
- **Refers to Loom runtime primitives**:
  - `EvidencePacket` (see Loom Blueprint §Phase 3 and `devkit/protocol_schemas/evidence_packet.schema.json`)
  - `GoalSpec` (see `devkit/protocol_schemas/goal_spec.schema.json`)
  - `Incident` + `devkit/repairer.dispatch` (Phase 4, 5 whitelist actions)
  - `devkit/goal_controller.py` + `devkit/fulfillment.py` (planned via the control-theory extension blueprint)
- **Companion document**: `loom-control-theory-extension-v2-2026-07-05.md`
  on `feat/docs-control-theory-extension`. Provides the runtime hard
  guarantees (SCDES supervisor, CBF barrier, queueing admission) that
  this skill depends on for safe execution.
- **Source**: `loom-agent-team-runtime-proposal.md` (lune, 2026-07-05)
  — this skill is the structured packaging of that proposal.
