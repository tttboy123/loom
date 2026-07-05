# Self-improvement loop

> Improvement candidates from run traces. Source:
> `loom-agent-team-runtime-proposal.md` §9 Phase 3.

## What this loop is

This loop takes `.loom/runs/<run_id>/` traces and produces
**ImprovementCandidate** files. It does not apply them. The
owner of the run must approve them in a separate step (gate-driven).

```
trace/events.jsonl -> failure pattern analyzer -> candidate YAML
                                                      |
                                                      v
                                            eval patch generator
                                                      |
                                                      v
                                            canary promotion flow
                                                      (rejected candidates
                                                       go to .loom/improvement/rollback/)
```

## ImprovementCandidate YAML

```yaml
improvement_candidate:
  id: ic_20260706_001
  source_runs:
    - run_20260706_001
  finding:
    type: redundant_tool_call
    summary: "Researcher re-fetched the same source twice in run_20260706_001."
  proposed_change:
    target: references/governance-protocol.md
    patch_type: routing_rule
    summary: "Add shared source cache step before parallel research fan-out."
  expected_gain:
    latency: "-10%"
    token_cost: "-8%"
  risk:
    level: medium
    notes:
      - "Cache may preserve stale context if not invalidated."
  eval_plan:
    - "Replay 3 previous research tasks; compare source coverage and cost."
  approval:
    required_by:
      - reviewer
      - human_for_core_policy    # required when patch_type ∈ prohibited
```

## Patch types and their allowed automation level

| Patch type | Auto-generate? | Auto-apply? |
|---|---|---|
| `template_text` (low-risk wording) | yes | maybe, only if `risk.level = low` and `evaluation_synthetic_verified = true` |
| `eval_case` (add a regression test) | yes | maybe, with shadow run first |
| `routing_hint` (which model for which task) | yes | **no** |
| `skill_instruction` (skill spec text) | yes | **no** |
| `model_capability_profile` | yes | **no** |
| `permission_policy` | yes | **never** |
| `safety_rule` | yes | **never** |
| `final_acceptance_criteria` | yes | **no** |
| `system_prompt` | yes | **never** |

The "never" columns correspond to what the chat-level policy
already prohibits; this loop also surfaces them as candidates so they
have an audit trail.

## Eval patch

Each candidate must come with at least one new eval that turns the
finding into a testable invariant:

```yaml
eval_patch:
  id: eval_001
  candidate_ref: ic_20260706_001
  test_type: trace_replay | static | live
  test_command: "scripts/replay_research_redundancy.py --task T3"
  expected: "no duplicate source fetch within same run"
```

The eval is added to the project's regression suite before the
candidate can be promoted.

## Canary promotion

```text
candidate ----> canary run (small subset, fresh run-ids)
       |
       +--> eval passes in canary  --> human approval  --> promote
       |
       +--> eval fails in canary  --> archive as failed
```

A candidate that touches `prohibited` targets (permission, safety,
routing, system prompt, model profile) **cannot reach the human
approval step**; the canary itself is forbidden.

## Pitfalls

- **Eval drift**: the eval patch can itself be gaming. Counter
  metric: rolling "eval pass rate that doesn't predict run success"
  should be 0.
- **Candidate explosion**: limit candidates per run to 5. Older
  candidates auto-close if not advanced within 30 days.
- **No improvement for failure modes 13, 14, 16, 17, 18** — these
  require human review, not candidate emission.

## Cross-reference

- See `loom-control-theory-extension-v2-2026-07-05.md` §6.9
  (GoalValidator) — analogous anti-reward-hacking primitive at the
  goal layer.
- See `failure-modes.md` §17 (Reward hacking / goal gaming) — the
  flip side of this loop.
