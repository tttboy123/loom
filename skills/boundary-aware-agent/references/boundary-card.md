# Boundary Card

> Per-task statement of capability / context / tool / risk limits.
> Source: `model-self-improvement-and-boundary-awareness.md` §3.2, §5.

## Why a Card (not a layer, not a singleton)

The original Doc 2 used the term "CBAL" (Capability Boundary Awareness
Layer) for this concept. This skill renames it to **Boundary Card**
because:

1. A "layer" suggests a runtime subsystem that hooks into every
   request. We don't need that — the card is emitted per task, not
   per token.
2. The "CB" prefix collides with `CBF` (Control Barrier Functions)
   in the companion control-theory extension document. Two "CB"
   namespaces in the same runtime = confusion.
3. "Card" matches the existing Loom vocabulary (WorkItem has a
   card-like metadata block; Gateway emits BoundaryCard per route).

## Required fields

```yaml
boundary_card:
  task_id: T2
  boundary_id: bc_<run>_<task>     # unique id; runtime emits this
  emitted_at: <iso8601>
  emitted_by: <agent role + model id>

  model_fit:
    score: 0.72                   # [0, 1]; from ModelCapabilityProfile match
    candidate_models:
      - "<model_a>"
      - "<model_b>"
    chosen: "<model_a>"
    rationale: "..."

  context_sufficiency:
    status: sufficient | partial | insufficient
    missing:
      - "..."

  tool_sufficiency:
    status: sufficient | partial | insufficient
    missing:
      - "..."

  date_sensitivity:
    status: fresh | recency_risk | time_critical
    notes:
      - "..."

  risk_domain:
    level: low | medium | high | critical
    risk_factors:
      - type: external_side_effect
        severity: high
        rationale: "touches production schema"
      - type: irreversibility
        severity: medium

  required_evidence:
    - type: tests
      command: "pytest tests/test_T2.py"
      evidence_path: "evidence/T2/"
      reproducible: true

  stop_conditions:
    - "Do not modify runtime/scheduler.py without human approval."
    - "Abort if cost > 0.5 USD per task."

  escalation:
    auto_to: "<reviewer role>"        # if risk == high, default reviewer
    on_violation: "<what happens if a stop_condition fires>"

  audit:
    schema_version: 1
    source_skill: boundary-aware-agent@v0.1.0
```

## How it interacts with GatePolicy

A `gate_policy.yaml` can reference fields of `BoundaryCard`:

```yaml
gate_policy:
  rules:
    - id: boundary_low_confidence_block
      severity: high
      rule: "Refuse Done if BoundaryCard.model_fit.score < 0.5"
    - id: high_risk_requires_reviewer_audit
      severity: high
      rule: "Refuse handoff if BoundaryCard.risk_domain.level ∈ {high, critical} and no escalation set"
```

(This is a future enhancement — see `runtime-hooks.md` §3.)

## Why per-task (not per-run)

A boundary card is per-task because:

- Different tasks in the same run have different model fits (a
  data-extraction task fits `cost_effective`; a sign-off task
  doesn't).
- Context needs change as you move through subtasks.
- Tools become available (or unavailable) mid-run.

A per-run boundary card would be coarser than needed and miss
mid-task escalations.

## The minimum viable boundary card

If you must shrink, the minimum is:

```yaml
{ model_fit.score: ..., risk_domain.level: ..., required_evidence: [...], stop_conditions: [...] }
```

Anything else is enhancement.
