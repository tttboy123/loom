# Pitfalls (don't do these)

> 12 named failure modes for this skill specifically. Source:
> `model-self-improvement-and-boundary-awareness.md` §11 + cross-reviewer
> feedback in `loom-control-theory-extension-v2-2026-07-05.md` §9.

## 1. Self-evolution as self-empowerment

Bad:
> Agent discovers it's frequently blocked by permissions → patches
> permission policy → now executes more dangerous actions.

Good:
> Agent notices the block → emits ImprovementCandidate →
> gate / human / frontier reviews → if approved, change is
> documented and version-controlled. Permission expansion never
> auto-applies.

## 2. Confidence as truth

A model saying "I'm 90% sure" doesn't mean it's calibrated. The skill
must rely on:

- Historical eval rates.
- Per-task-class pass rate.
- Evidence coverage.
- Boundary Card `model_fit.score`.
- Verifier reviews where risk ∈ {high, critical}.

Not on the model's stated confidence.

## 3. Blindspot pass as long survey

The point is identifying unknowns that **change the answer**, not
cataloguing every conceivable open question. Worst forms:

- "Are there any other features you want?"
- "Anything else I should know?"
- Long unsorted lists of every conceivable concern.

Best forms (concrete, actionable):

- "Is this a discussion draft or a runtime change?"
- "Which file is the authority — docs/X.md or code/Y.py?"
- "Done = runnable? Reviewable? Deployable?"
- "What's the latest date this stays valid?"

## 4. Producer = final reviewer

The agent's self-critique is useful but never a substitute for a
fresh-context reviewer. Always emit ImprovementCandidate and route
it to a different model.

## 5. Over-tuning per model version

A model's behavior drift between versions is small but real.
Profile by **capability class**, not version. New version →
inherit the class profile, run a calibration eval to confirm.

## 6. Eval drift

An EvalPatch that doesn't predict run success is a vanishing asset.
Track `eval_pass_rate_that_predicts_run_success`; alert when it
drops below 0.7.

## 7. Patch explosion

Limit: ≤ 5 ImprovementCandidates per run. Older candidates
auto-close at 30 days if not advanced.

## 8. Reward hacking

The improvement loop can game its own metrics (close incident as
"won't-fix" to lower incident_count). Counter-measure:

- Require diversity in `finding.type` over a 30-day window
  (≥ 4 types seen; not just one type).
- Sample-validate canary outcomes by human or frontier reviewer.

## 9. Boundary card always emits "high"

If a card consistently reports `model_fit ≥ 0.9`, the model may be
self-overconfident OR the score rubric may be too loose. Counter:

- Calibrate `model_fit` against a held-out set of validated outputs.
- Reject profile if calibration fails twice.

## 10. Skill covers everything

This skill is about boundary awareness + evidence grounding +
controlled self-improvement. It does NOT cover:

- Code execution → `agentteam-runtime` skill.
- Scheduler logic → Loom runtime (`devkit/`).
- Goal decomposition → Loom Blueprint.

If a need lives in another layer, route there.

## 11. Profile snapshot instead of capability contract

Profiles are about **what the model is good at + what it must not do**.
Not about version snapshots. Version drift should be tracked
separately (release_date field).

## 12. Forbidden patch types re-classified

If you see a candidate that "shouldn't" be one of the patch_types
list (e.g., a `permission_policy` patch disguised as `template_text`),
the skill does not redefine the category at runtime — it REFUSES
and emits a hard error. The classification is hard-coded.

## Cross-reference

- See `agentteam-runtime` skill's `failure-modes.md` for team-side
  failures that this skill can amplify (e.g., reward hacking).
- See the companion control-theory doc for guard rails on
  continuous self-modification.
