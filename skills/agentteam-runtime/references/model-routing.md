# Model routing

> Picking model policies per role and per run mode. Source:
> `loom-agent-team-runtime-proposal.md` §8.

## Model-policy tier definition

| Tier | Use for | Suggested model |
|---|---|---|
| `cost_effective` | Bulk extraction, formatting, summaries, low-risk drafts | Local small model or cheap hosted API |
| `balanced` | Code generation, mid-complex reasoning, refactoring | Mid-tier hosted model |
| `high_reasoning` | Planning, code review, test failure analysis, final acceptance | Claude Fable 5 / Claude Opus 4.8 / GPT-5.5 / Codex |
| `frontier_safety_consulted` | Decisions touching safety, permission, model-routing, schema | Claude Fable 5 / Claude Mythos 5 (frontier) |

Reviewers (gate agents, reviewer role, acceptance signer) should
default to `high_reasoning`. Producers can be cheaper.

## Routing by run mode

| Mode | Default orchestrator | Default producers | Default reviewer |
|---|---|---|---|
| `single-agent` | n/a | `balanced` (with verifier) | verifier-only |
| `agent-team` | `high_reasoning` | split across roles | `high_reasoning` (fresh context) |
| `cluster` | `high_reasoning` for GoalSpec emission | per-team | per-team |

## Per-role routing table

| Role | Recommended tier | Forbidden tier |
|---|---|---|
| orchestrator / Lead | `high_reasoning` | `cost_effective` (orchestration needs judgment) |
| builder | `balanced` or `high_reasoning` | `cost_effective` for code review or design |
| reviewer | `high_reasoning` | `cost_effective` (reviewer must be able to disagree convincingly) |
| researcher | `cost_effective` (extraction), `balanced` (synthesis) | `high_reasoning` (overkill for most research tasks) |
| gates | `high_reasoning` | `balanced` for low-stakes gates (e.g., handoff_gate) |

## Routing escalations

These automatically escalate to `high_reasoning`:

- Any task touching `main` branch or production schema.
- Any task involving ambiguous user intent (see Unknowns Map).
- Any task that consumes more than 30% of team budget.

These automatically escalate to `frontier_safety_consulted`:

- Permission policy change.
- Safety rule change.
- Model routing change.
- Prompt or system-prompt change.

## Why this matters

- **Cost control**: routing cheap models to cheap tasks lowers burn.
- **Quality control**: routing expensive models to gate tasks raises
  agreement rate.
- **Bias control**: sticking with one tier biases outputs toward that
  tier's failure modes. Mix tiers per role.

## Reference

Existing Loom model routing lives in `loom.roles.example.toml`.
Per-team overrides are in the team's `model_overrides` field
(future enhancement).
