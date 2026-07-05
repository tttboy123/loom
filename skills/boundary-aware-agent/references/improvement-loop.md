# Improvement loop (8 steps, gated)

> From run traces to ImprovementCandidates, with canary + eval
> gating. Source: `model-self-improvement-and-boundary-awareness.md`
> §7 + `agentteam-runtime` skill's `self-improvement-loop.md`.

## Why this loop exists

After a run, traces contain failure patterns: redundant calls, missing
acceptance, repeated verifications, hard refusals, etc. Without a
loop, those patterns persist. With a loop, the system improves — but
**only via gated candidates**, never by auto-application.

## The 8 steps

```
1. Capture    — collect trace, tool calls, messages, handoffs, artifacts, cost, latency
2. Judge      — deterministic checks + rubric checks + human feedback
3. Cluster    — group failure modes: blindspot miss, tool misuse, context short, over-execution, false-done, cost waste
4. Propose    — emit ImprovementCandidate (this skill's primary output)
5. Convert    — each candidate must yield at least one regression eval
6. Gate       — high-reliability reviewer (boundary_card escalation) judges risk/gain/scope
7. Canary     — run on a subset or historical replay
8. Promote    — on canary pass + reviewer approval, apply patch; on fail, archive
```

## ImprovementCandidate v1 (this skill's emit)

```yaml
improvement_candidate:
  schema_version: 1

  id: ic_<run>_<n>
  source_runs:
    - run_20260706_001

  finding:
    type: <enum below>
    summary: "..."
    evidence_refs:
      - "trace/events.jsonl#events_42_to_55"

  proposed_change:
    target: "<file path or skill/spec>"
    patch_type: <enum below>
    summary: "..."

  expected_gain:
    latency: "-10%"
    token_cost: "-8%"
    quality: "+0.05 (rubric)"

  risk:
    level: low | medium | high
    notes:
      - "..."

  eval_plan:
    - test_type: trace_replay | static | live
      command: "scripts/replay_<id>.py"
      expected: "..."

  approval:
    required_by:
      - reviewer
      - human_for_core_policy    # required if patch_type ∈ prohibited
```

### `finding.type` enum

`redundant_tool_call` |
`missing_acceptance` |
`boundary_card_overconfident` |
`blindspot_pass_skipped` |
`producer_self_review` |
`weak_evidence_path` |
`human_gate_overload` |
`cost_blast_radius` |
`context_exhaustion_observed` |
`gate_round_trip`

### `proposed_change.patch_type` enum

| Patch type | Auto-emit | Auto-apply |
|---|---|---|
| `template_text` (low-risk wording) | yes | **maybe**, only if `risk.level = low` and eval synthetic verified |
| `eval_case` (add regression test) | yes | **maybe**, with shadow run first |
| `routing_hint` (model-per-task) | yes | **no** |
| `skill_instruction` (skill spec text) | yes | **no** |
| `model_capability_profile` (update profile) | yes | **no** |
| `permission_policy` | yes | **NEVER** |
| `safety_rule` | yes | **NEVER** |
| `final_acceptance_criteria` | yes | **no** |
| `system_prompt` | yes | **NEVER** |

## Loop enforcement

- **Step 4 (Propose)** is the only step this skill owns end-to-end.
  Steps 5-8 are owned by `agentteam-runtime`'s
  `self-improvement-loop.md` and the Loom runtime.
- **Steps 1-3** can be run by a `cost_effective_worker`; they don't
  need a frontier model. So the loop is cheap by default.
- **Step 6 (Gate)** must use a fresh-context reviewer that did NOT
  produce the proposal. See `references/pitfalls.md`.
- **Step 7 (Canary)** must run at ≥ 0.05 of original run volume. Less
  is not statistically meaningful.

## Eval patch

For each candidate, generate one EvalPatch:

```yaml
eval_patch:
  id: eval_<n>
  candidate_ref: ic_<run>_<n>
  test_type: trace_replay | static | live
  expected: "..."
  command: "scripts/replay_<id>.py"
  regression_target: <metric from references/metrics.md>
```

The EvalPatch becomes part of the regression suite — without it,
the candidate cannot be promoted.

## What kills a candidate

- Finding has no evidence (`evidence_refs` empty).
- Eval doesn't exist (no EvalPatch).
- Canary fails even once.
- Cost `expected_gain` overshoots `risk.level = high`.
- Touches any patch_type marked `NEVER`.

## Cross-reference

- Loom runtime signals for `improve` trigger:
  `.loom/runs/<id>/improvement/candidates/`.
- `agentteam-runtime` skill's `self-improvement-loop.md` is the
  consumer side.
