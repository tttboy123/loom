# Evaluation modes

> How to actually measure metrics from this skill. Source:
> companion control-theory doc §10 + Doc 2 §7.

## Three modes

| Mode | Latency | Sample size | When |
|---|---|---|---|
| **fast** | minutes | small (1-3 tasks) | smoke test before each release |
| **standard** | hours | medium (10-30 tasks × 5-10 runs) | post-improvement canary |
| **headless** | days | large (60+ tasks × 30 runs) | quarterly calibration + release gate |

## Calibration budget

For each skill release, run standard mode on a stratified task set
(60 tasks, 30/40/30 split). Total compute:

- 60 tasks × 10 runs × ~30 sec/run = 5 hours wall-clock
- Cost: depends on model + context. Documented per run in
  `trace/metrics.json`.

For headless mode:

- 60 tasks × 30 runs × ~30 sec/run = 15 hours wall-clock
- Run async; do not gate human workflow.

## What goes into the evaluation harness

The harness uses these tools, in order:

1. **Replay**: load historical runs as test inputs.
2. **Distill**: per task, calculate metric value (e.g. `unknown_coverage`
   from prior Blindspot Pass artifacts).
3. **Compare**: candidate improvement vs. baseline same task.
4. **Report**: aggregate + per-class metrics, with confidence intervals.

## Selecting the canonical task set

- 60 tasks sampled from the last 30 days of runs (real-world
  distribution).
- Stratified 30 / 40 / 30: algorithmic / integration / heavy refactor.
- 20% pathological cases (ambiguous, missing deps).
- Fixed seed (`random_state = 42`) so the same tasks appear in
  every calibration run.

## What to exclude

- Tasks with self-improving labels (gaming the eval).
- Tasks whose ground truth is disputed (mark as
  `ground_truth_disputed: true` and exclude from accuracy calcs).
- Tasks scheduled in the same model version as the candidate being
  evaluated (contamination).

## Pitfalls specific to eval

- **Eval doesn't drift predictively**: track
  `eval_pass_rate_that_predicts_run_success`; drop the eval if < 0.7.
- **Train-test divergence**: 20% of tasks reserved as holdout.
- **Metric fixation**: agents start optimizing the metric itself.
  Mitigation: rotate metrics quarterly.

## Output format

```json
{
  "calibration_run_id": "cal_20260706_001",
  "date": "2026-07-06",
  "mode": "standard",
  "task_set": "loom_canonical_2026_07",
  "metrics": {
    "unknown_coverage": {"mean": 0.72, "ci95": [0.66, 0.78]},
    "false_done_rate": {"mean": 0.03, "ci95": [0.01, 0.06]},
    "canary_lift": {"mean": 0.08, "ci95": [0.04, 0.12]}
  },
  "promotable_candidates": 5,
  "rejected_candidates": 12,
  "evaluator_notes": "see run_..."
}
```

## Cross-reference

- `references/metrics.md` — what each metric means.
- The companion control-theory doc §10 has statistical power
  calculations.
