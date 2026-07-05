# Metrics

> What to measure when this skill is in production. Source:
> `model-self-improvement-and-boundary-awareness.md` §8 + companion
> doc's calibration methodology.

## Three categories of metrics

| Category | What | Examples |
|---|---|---|
| **Blindspot coverage** | Did the agent surface the right unknowns? | `unknown_coverage`, `premature_action_rate` |
| **Boundary fit** | Did routing match reality? | `model_fit_accuracy`, `fallback_success_rate`, `false_done_rate` |
| **Self-improvement safety** | Is the loop improving without breaking invariants? | `eval_regression_rate`, `candidate_acceptance_rate`, `prohibited_patch_auto_attempt_count` |

## Blindspot metrics

| Metric | Definition | Target |
|---|---|---|
| `unknown_coverage` | Of rework reasons in the last N runs, what fraction was actually surfaced at Blindspot Pass time? | ≥ 0.7 |
| `assumption_accuracy` | Of assumptions made, what fraction held in retrospective review? | ≥ 0.8 |
| `clarification_precision` | Of agent questions that caused user rework, what fraction was necessary? | ≥ 0.6 |
| `premature_action_rate` | Of tasks that started execution with at least one `unresolved_high_risk` unknown, what fraction? | 0 |

## Boundary metrics

| Metric | Definition | Target |
|---|---|---|
| `model_fit_accuracy` | Of BoundaryCard `model_fit.score` predictions, what fraction matched the actual outcome (pass/fail)? | ≥ 0.7 |
| `fallback_success_rate` | Of escalations, what fraction succeeded at the escalated tier? | ≥ 0.6 |
| `overreach_rate` | Of all model actions, what fraction was unrequested? | ≤ 0.05 |
| `unsupported_claim_rate` | Of claims in reports, what fraction had no evidence path? | ≤ 0.05 |
| `false_done_rate` | Of tasks marked Done, what fraction fails acceptance_gate within 24h? | ≤ 0.05 |

## Self-improvement metrics

| Metric | Definition | Target |
|---|---|---|
| `eval_regression_rate` | After a candidate is promoted, how often do old evals fail? | ≤ 0.05 |
| `repair_loop_count` | For a given task, how many repair cycles before Done? | ≤ 3 |
| `tool_thrash_rate` | Repeated / ineffective tool calls per run? | ≤ 0.1 |
| `cost_per_accepted_task` | Per Done task, total cost (USD-equivalent) | documented trend, not target |
| `candidate_acceptance_rate` | Of candidates emitted, what fraction cleared gate? | trend up |
| `canary_lift` | On canary, what was the metric lift over baseline? | per-candidate |
| `prohibited_patch_auto_attempt_count` | Number of auto-apply attempts on prohibited patch types | **0** |

## Calibration methodology

- **N = 30 minimum** for any binary claim ("model_fit ≥ X predicts Done").
- **60 tasks × 10 runs** for steady-state calibration.
- Stratified 30/40/30 (algorithm / integration / heavy refactor).
- See `references/evaluation-modes.md` for fast/long/headless modes.

The companion doc (`loom-control-theory-extension-v2-2026-07-05.md`)
§10 has a fuller methodology with statistical power calculations.

## Anti-metric: don't measure

- **Average model score alone.** Models with similar averages can
  fail in different ways; class-based metrics are richer.
- **Improvement count alone.** Many tiny improvements can mask a
  large regression in one place.
- **Token velocity (tokens per minute).** This rewards verbosity,
  not quality.

## What to do when a metric is bad

1. **Diagnose first**: read 10 traces; sample 5 candidates; classify
   failure patterns.
2. **Emit ImprovementCandidate(s)** with `finding.type` set to the
   diagnosed class.
3. **Never patch the metric** to make it look better.
