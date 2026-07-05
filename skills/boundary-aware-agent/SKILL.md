---
name: boundary-aware-agent
description: "Externalize Fable-5/Opus-style capability boundary into reusable artifacts so any model (MiniMax, Codex, Claude, open-source, mid-tier) can apply the 'find your blindspot, know your boundary, prove with evidence, fix yourself' workflow. Use when the orchestrator or producer agent needs to recognize capability boundaries, run a Blindspot Pass, build a BoundaryCard, evaluate model_fit vs risk_domain, gate evidence, or apply the 8-step controlled self-improvement loop."
license: MIT
compatibility: Requires Loom runtime (devkit/state_writer.py, devkit/repairer.py) + capability evidence schema
metadata:
  author: "lune"
  version: "0.1.0"
  source: "model-self-improvement-and-boundary-awareness.md (lune, 2026-07-05)"
  refers_to_models:
    - "Claude Fable 5 / Mythos 5 (Anthropic, 2026-06-09, $10/$50 per million tokens)"
    - "MiniMax-M3, codex-sub, DeepSeek, GLM (Loom local cluster)"
  derived_from:
    - "Blindspot Pass + Boundary Card + Evidence-grounding + Controlled self-improvement (4-layer framework)"
  tags: "loom,model-capability,boundary,blindspot,evidence,improvement-loop,per-model-adaptation"
  category: "loom-runtime"
---

# boundary-aware-agent

Externalize Fable-5/Opus-style capability boundary into reusable
artifacts so other models — MiniMax, Codex, open-source, mid-tier —
benefit from the same "find your blindspot, know your boundary,
prove with evidence, fix yourself" workflow.

## Routing

When the orchestrator (or a producer agent, or the user) needs to
recognize capability boundaries and act on evidence, route to this
skill. From here:

1. **For pre-execution**: run `references/blindspot-pass.md` first,
   then build `references/boundary-card.md`. Both are mandatory for
   `medium` risk and above.
2. **During execution**: keep `references/evidence-grounding.md` in
   force. Every claim before reporting must be classified.
3. **For model selection**: consult `references/model-capability-profile.md`
   + `references/per-model-adaptation.md`. The chosen tier must
   match the task's risk domain.
4. **After run**: route traces to `references/improvement-loop.md`.
   Emit candidates only — never auto-apply.

## Stop conditions

Refuse and route to human if any of:

- The user / orchestrator asks the model to modify its own system
  prompt.
- The user / orchestrator asks the model to relax safety, permission,
  routing policy.
- A boundary card reports `model_fit < 0.4` and the user still wants
  to proceed on the same model. → escalate to frontier_safety_consulted.
- An improvement candidate targets a prohibited patch type (see
  `references/improvement-loop.md`).

## Boundary contracts

- **Upstream**: this skill assumes the goal has been expressed as a
  written goal, a GoalSpec YAML, or a chat request. If none, ask the
  user for the goal before blindspot pass.
- **Downstream**: this skill produces YAML artifacts that the Loom
  runtime reads and that the `agentteam-runtime` skill consumes at
  team-run preflight.
- **Lateral**: this skill does not execute work itself; it produces
  evidence and configuration that other agents execute.

## Required artifacts

Before any execution of a non-trivial task, produce at least:

- `unknowns-map.yaml`
- `boundary-card.yaml`
- `model-capability-profile.yaml` (referenced or new)

For model-assisted operations, also:

- `evidence-grounding.md` for any progress report
- `improvement-candidate.yaml` post-run

## Style

- Use the prompt pack verbatim (`references/prompt-pack.md`) when
  called — do not paraphrase.
- Keep YAML examples short — they should fit on one screen.
- Cite the source document and section when borrowing content.
- Prefer "boundary card says X" to "I think X" in any emission.
