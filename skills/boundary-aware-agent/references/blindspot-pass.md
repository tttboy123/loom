# Blindspot Pass

> Surface unknowns before acting. Source:
> `model-self-improvement-and-boundary-awareness.md` §3.1, §6, §13.1.

## What a Blindspot Pass asks

It does NOT ask "what do you want me to do?". It asks the questions
that change the answer:

1. What must be true for the task to be successful?
2. What did the user not specify that could change the solution?
3. Which missing facts can be verified cheaply?
4. Which assumptions are safe to make now?
5. Which actions are out of scope or need approval?
6. What evidence will prove completion?
7. Which model / role should handle planning, execution, review,
   and acceptance?

The point is not exhaustive questioning — it is identifying the
**unknowns that change the answer**. Over-asking wastes the model's
context; under-asking misses reversibility.

## Output schema

```yaml
blindspot_pass:
  run_id: run_20260706_001
  task_id: T1                  # optional; if per-task, include

  success_conditions:
    - "..."

  unspecified_decisions:
    - "..."

  cheap_verifications:
    - "..."                   # name cheap verifications that can run now

  safe_assumptions:
    - "..."                   # assumption + reasoning

  approval_required:
    - "..."                   # list of actions needing human / frontier approval

  evidence_required:
    - "..."                   # what counts as 'done'

  recommended_routing:
    planner: "codex_or_high_reasoning"
    worker: "balanced"
    reviewer: "fresh_context_high_reasoning"

  proceed:
    allowed: true
    reason: ""                # required if allowed=false
```

## Per-model adaptation table

| Model type | Blindspot Pass demand | Why |
|---|---|---|
| Claude Fable 5 / Claude Mythos 5 | minimal — "minimum necessary unknowns", no long listing | Long context handles open-ended exploration; over-asking wastes effort |
| Claude Opus 4.8 / Sonnet 4.5 | medium — fixed fields, ≤ 7 items | Strong but not infinite context; boundedness keeps them focused |
| Codex / coding models | strict — bound to files / commands / tests / acceptance | Concrete deliverables; abstract questions hurt |
| Research models | strict — bound to sources / dates / uncertainty | Cite-or-bust; they need ground truth |
| Open-source / local small | minimal — only classify: missing / assumption / approval / evidence | They lack context for nuance; abstract questions confuse them |

The actual prompt to invoke Blindspot Pass is in
`references/prompt-pack.md` §1.

## What is intentionally out of scope

- Re-asking the user for things they already said.
- Making the agent refuse to start because of an ambiguity it can
  resolve itself at low cost.
- Generating a "compliance checklist" — the goal is action-relevant
  unknowns, not paper trails.

## Why this matters

The original Doc 2 phrasing: "Fable 5 给人的新鲜感，不只是'模型更强'，
而是它把三件事同时做得更像一个长程工作者." Those three things —
blindspot before, boundary during, evidence after — are the workflow,
not the personality. The skill reifies them so any model can follow
the workflow.

## Cross-reference

- The Blindspot Pass fills the **UnknownsMap** YAML, which the
  `agentteam-runtime` skill consumes at `plan_gate`.
- A failed Blindspot Pass (allowed=false) blocks the team run's
  Inbox → Assigned transition.
