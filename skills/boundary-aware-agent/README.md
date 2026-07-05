# boundary-aware-agent

> Externalize Fable-5/Opus-style capability boundary into reusable
> artifacts so other models benefit too.
> Derived from `model-self-improvement-and-boundary-awareness.md`
> (2026-07-05).

## When to invoke this skill

Activate when the user, orchestrator, or producer agent needs:

- A **blindspot pass** before designing or coding — surface
  unknowns that the user didn't specify but matter.
- A **boundary card** for the current task: model fit, context
  sufficiency, tool sufficiency, risk domain, stop conditions,
  evidence required.
- A **model capability profile** for picking a model per role or
  per task — see `references/model-capability-profile.md`.
- A **grounded progress report** — every claim marked verified /
  inferred / unverified before reporting.
- A **self-improvement candidate** from a run trace — gated via
  the loop in `references/improvement-loop.md`.

Do NOT use this skill to:

- Override the model's system prompt or weights (forbidden regardless
  of how tempting).
- Auto-apply permission / safety / routing changes.
- Bypass gates set by `agentteam-runtime`.

## Routing map

| If you want to… | Read |
|---|---|
| Understand the 4-layer framework (L1-L4) | `references/four-layer.md` |
| Run a blindspot pass | `references/blindspot-pass.md` and use `assets/templates/unknowns-map.yaml` |
| Build a boundary card for a task | `references/boundary-card.md` and use `assets/templates/boundary-card.yaml` |
| Choose a model per role | `references/model-capability-profile.md` + `assets/templates/model-capability-profile.yaml` |
| Author or audit ground progress reports | `references/evidence-grounding.md` |
| Distinguish per-model scaffolding | `references/per-model-adaptation.md` |
| Operate the self-improvement loop | `references/improvement-loop.md` |
| Avoid common pitfalls | `references/pitfalls.md` |
| Pick metrics | `references/metrics.md` |
| Hook this into Loom runtime | `references/runtime-hooks.md` |
| See reusable prompts | `references/prompt-pack.md` |

## Why this skill exists (one-line summary)

> Make the Fable-5 / Opus-style "find your blindspot, know your
> boundary, prove with evidence, fix yourself" workflow model-agnostic
> by externalizing it into runtime artifacts.

Other models can then benefit from the same workflow without needing
Fable-5's native character — they get the workflow as scaffolding
(runtime + skill + template), and the model-specific role just fills
in the capability it actually has.

## Hard rules

These are runtime-enforced:

1. **No modifying system prompt, weights, permission policy, safety
   rule, or model routing.** Ever.
2. **No self-approval.** All gating must be done by a fresh-context
   model.
3. **No claim without evidence.** Every statement about state,
   result, or completion must be marked `verified | inferred |
   unverified`.
4. **Improvement candidates only — never auto-apply.** Even
   low-risk patches require explicit human or frontier-mythos-class
   approval.
5. **Boundary card is mandatory before execution** of any task
   classified `medium` or higher risk.

## Cross-references

- **Companion**: `agentteam-runtime` skill. This skill provides the
  per-agent artifacts (Blindspot, Boundary Card); `agentteam-runtime`
  consumes them at team-run preflight.
- **Companion document**: `loom-control-theory-extension-v2-2026-07-05.md`
  on `feat/docs-control-theory-extension`. Defines the runtime hard
  guarantees (SCDES supervisor, CBF barrier, ranking function with
  lexicographic priority, queueing admission) that this skill depends
  on for safe execution.
- **Source**: `model-self-improvement-and-boundary-awareness.md`
  (lune, 2026-07-05).
