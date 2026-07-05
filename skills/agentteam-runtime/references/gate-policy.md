# Gate policy

> Reviewers, gates, and refusal rules. Source:
> `loom-agent-team-runtime-proposal.md` §4 gate-policy.md + §8 model
> routing.

## Default reviewer

| Tier | Default model | When to use |
|---|---|---|
| `default_reviewer` | high_reasoning | Codex / Claude Fable 5 / Claude Opus 4.8 |
| `code_change_reviewer` | Codex or equivalent | diff + test + risk review |
| `research_reviewer` | high_reasoning with web browsing | citations + recency check |
| `design_doc_reviewer` | high_reasoning | ambiguity + decision-table + risk |

If a project has only one tier, default to `high_reasoning`.

## Required evidence per artifact class

```yaml
gate_policy:
  required_evidence:
    code_change:
      - diff
      - tests_or_reason         # passing tests, or a justified "no tests needed"
      - risk_review
      - rollback_plan
    research:
      - source_list
      - uncertainty_notes
      - date_sensitivity_check
    design_doc:
      - decision_table          # what was decided and why
      - runtime_contract        # how it plugs into existing system
      - open_questions
    incident_resolution:
      - root_cause_or_assumption
      - reproducer_or_evidence_chain
      - regression_test_plan
```

## Blocking rules (hard refusal)

A gate refuses acceptance if any of these is true:

| Rule | Severity | What it catches |
|---|---|---|
| `missing_acceptance` | high | Task moved to Done without acceptance defined |
| `self_review_only` | high | Producer is also the only reviewer |
| `unverifiable_claim` | medium | Claim with no evidence or labeled assumption |
| `verification_command_missing` | medium | Handoff has verification step but command not in allowlist |
| `cost_blast_radius_exceeded` | medium | Action would consume > 30% of team budget without explicit approval |
| `unknowns_unresolved_high_risk` | high | Required Unknowns Map entry unresolved and risk is high |

## Soft rules (warning, not block)

| Rule | Severity | What it catches |
|---|---|---|
| `redundant_tool_call` | low | Same source re-fetched within run |
| `unknowns_over_clarifying` | low | > 7 Unknowns (likely over-asking) |
| `gate_round_trip` | medium | > 3 rounds of review for same task (probably ambiguous spec) |

Soft rules emit warnings in the gate report but do not refuse
acceptance. They feed into the improvement candidate generator
(`self-improvement-loop.md`).

## Gate types

| Gate | When | Focus |
|---|---|---|
| `plan_gate` | Before execution | scope, unknowns, task graph, risk, cost budget |
| `handoff_gate` | Per cross-role handoff | 5 elements complete, reviewer ≠ producer |
| `evidence_gate` | After artifact | required evidence present and reproducible |
| `integration_gate` | Before merging to main | diff, tests, rollback |
| `acceptance_gate` | Before Done | acceptance criteria met end-to-end |
| `improvement_gate` | Before promotion | candidate is verifiable, regression eval added |

## Gate escalation policy

These must be routed to a human or to the highest-tier reviewer
(Codex or Claude Fable 5 / Mythos 5):

- Modifies runtime policy, model routing, permission policy, safety
  boundary, or system prompt.
- Touches a critical-path action (writing to main, schema migration,
  production deployment, secret-bearing operation).
- Combines multiple agents' conclusions that materially conflict.
- Requires a value judgment (cost vs quality vs risk) the runtime
  cannot reach.

## Cost allocation per gate

A rough allocation that prevents one gate from consuming half the team
budget:

| Stage | Budget share |
|---|---|
| `plan_gate` | ≤ 5% |
| `evidence_gate` per artifact | ≤ 10% |
| `integration_gate` | ≤ 10% |
| `acceptance_gate` | ≤ 10% |
| `improvement_gate` (when triggered) | ≤ 5% |
| Sum | ≤ 40% of budget for the run |

Anything beyond 40% signals the team is over-gating and should be
reshaped. Reference: the broader cost framework in
`loom-control-theory-extension-v2-2026-07-05.md` §7.
