# Runtime hooks

> Where this skill's artifacts attach to Loom runtime. Source:
> `model-self-improvement-and-boundary-awareness.md` §9.

## When each artifact is emitted

| Hook | When | Produces |
|---|---|---|
| `before_plan` | User goal received, before team plan | `UnknownsMap` |
| `before_route` | Model selection per task | `BoundaryCard` |
| `before_execute` | Producer starts | `EvidencePlan` (within BoundaryCard) |
| `before_report` | Producer reports progress | tagged claims (verified/inferred/unverified) |
| `before_done` | Task transitions Done | `EvalPatch` + gate verdict |
| `after_run` | Run completes | `ImprovementCandidate` × ≤ 5 |
| `before_promote` | Candidate to be applied | `EvalPatch` + gate verdict |

In Loom terms, these hook into `devkit/state_writer.transition_task`:

- `before_plan` → observer/triager preflight, emit UnknownsMap
- `before_route` → scheduler consults ModelCapabilityProfile + emits BoundaryCard
- `before_execute` → repairer consults BoundaryCard for stop_conditions
- `before_report` → progress_auditor tags claims
- `before_done` → gatekeeper consumes EvidencePacket
- `after_run` → improvement_auditor emits ImprovementCandidate
- `before_promote` → improvement_gate consumes EvalPatch

## Where the artifacts live

```
.loom/
  models/profiles/<model_id>.yaml       # ModelCapabilityProfile (manual)
  runs/<run_id>/
    unknowns-map.yaml                   # L1 output
    boundary/<task_id>.yaml             # L2 output per task
    evidence/<task_id>/                 # L3 output
      commands.jsonl
      outputs/
      claims.yaml                       # tagged claims
    improvement/candidates/
      ic_<n>.yaml                       # L4 output (≤ 5 per run)
      eval_<n>.yaml                     # L4 eval patches
```

## Loom runtime code mapping

This skill does not replace existing Loom runtime modules; it
extends them:

| Loom file (existing) | This skill's hook |
|---|---|
| `devkit/repairer.dispatch` | bounded by BoundaryCard `stop_conditions` |
| `devkit/goal_controller.future` | consumes UnknownsMap's acceptance_unknowns for distance function |
| `devkit/control-theory v2 SCDES supervisor` | consumes BoundaryCard `risk_domain` to decide if action is admissible |
| `devkit/ranking function (v2 doc)` | candidate ImprovementCandidate priority is added to the ranking |

The skill is **layered above** the runtime, not a replacement.

## Phase wiring

| Phase | Hooks added |
|---|---|
| Phase 0 | none — templates only |
| Phase 1 | `before_plan` + `before_route` |
| Phase 2 | `before_execute` + `before_report` |
| Phase 3 | `before_done` + `after_run` + `before_promote` |

This mirrors the `agentteam-runtime` skill's phases.

## What this skill does NOT do

- Modify `devkit/state_writer.py`, `devkit/repairer.py`,
  `devkit/incident.schema.json`, or any existing Loom code.
- Add new top-level Loom modules. Where new modules are needed
  (e.g. `devkit/profile_loader.py`), the skill proposes them; the
  `agentteam-runtime` workstream decides whether to implement.

## Cross-reference

- `agentteam-runtime` skill's `runtime-contract.md` — for the run
  directory layout.
- The control-theory doc — for `devkit/goal_controller.py`
  (proposed) and the 3-tier runtime structure.
