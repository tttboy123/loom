# Prompt pack (4 reusable prompts)

> Verbatim prompts to invoke this skill's four core capabilities.
> Source: `model-self-improvement-and-boundary-awareness.md` §13.

These prompts can be cut-and-pasted into any chat / API / agent
runtime that has a tool surface.

## 1. Blindspot Pass

```text
Before solving, identify the blind spots that could change the answer.

Return:
- success_conditions
- missing_context
- unsafe_assumptions
- cheap_verifications
- required_evidence
- out_of_scope_actions
- routing_recommendation

Only ask the user if the missing information blocks a safe next step.
Otherwise state your assumptions and proceed.
```

Output schema: see `references/blindspot-pass.md` §"Output schema".

## 2. Boundary Awareness

```text
Assess whether you are the right model/agent for this task.

Classify:
- context sufficiency
- tool sufficiency
- domain risk
- recency risk
- execution risk
- verification method
- whether a stronger reviewer is required

If you proceed, name the evidence that will prove completion.
```

Output schema: see `references/boundary-card.md` §"Required fields".

## 3. Evidence-Grounded Progress

```text
Before reporting progress, audit each claim against evidence from this run.

For each claim, mark:
- verified: supported by tool result or artifact
- inferred: reasonable but not directly verified
- unverified: should not be presented as done

Do not claim completion unless the acceptance evidence exists.
```

## 4. Self-Improvement Audit

```text
Review the run trace and identify improvement candidates.

Do not modify prompts, skills, permissions, or routing directly.

Return candidates with:
- source evidence
- failure pattern
- proposed change
- expected gain
- risk
- eval case to add
- approval required
```

Output schema: see `references/improvement-loop.md` §"ImprovementCandidate v1".

## Per-model adaptation of these prompts

| Model | Adaptation |
|---|---|
| Claude Fable 5 / Mythos 5 | Use verbatim. Optional: add "Stay terse; do not pad." |
| Claude Opus 4.8 / Sonnet 4.5 | Use verbatim. |
| Codex | Add: "Include exact file paths and commands where claims require them." |
| MiniMax orchestrator-tier | Use verbatim; emit YAML directly. |
| MiniMax worker-tier | Add: "Reply with: missing / assumption / approval / evidence categories only." |
| Open-source / small local | Strip to: "Reply: missing?| assumption?| approval?| evidence?". |

See `references/per-model-adaptation.md` for the broader rationale.
