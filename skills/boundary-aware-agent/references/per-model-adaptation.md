# Per-model adaptation

> How scaffolding differs by model. Source:
> `model-self-improvement-and-boundary-awareness.md` §4.2, §6 (model adaptation), §10.

## Why scaffolding is model-specific

A Blindspot Pass that works for Claude Fable 5 is too verbose for a
local 7B model. A HandoffEnvelope that captures everything Fable 5
needs overwhelms MiniMax. The same workflow, different scaffolding.

The rule:

> **Strong models get less hand-holding. Weak models get more.**
> Loom must adapt the briefing per model without changing the
> downstream artifacts.

## Adaptation table

| Model class | Blindspot prompt | Boundary card | Evidence-level demand | Self-improvement access |
|---|---|---|---|---|
| `frontier_long_horizon` (Claude Fable 5, Claude Mythos 5) | "minimum necessary unknowns", no listing | optional; default emits | any | full read-only |
| `frontier_engineering` (Codex, code-grounded) | "concrete files / commands / tests / acceptance" | required; concrete evidence paths | strict (re-runnable commands) | full read-only |
| `team_orchestrator` (MiniMax orchestrator-tier) | structured fields, ≤ 7 items | required | any | full read-only |
| `cost_effective_worker` (MiniMax worker-tier) | 4-category classify only (missing / assumption / approval / evidence) | required but minimal | strict; sentences not paragraphs | read-only, no ImprovementCandidate emission |
| `research_specialist` (mid-tier hosted with browse) | bound to sources / dates / uncertainty | required | strict (citation count) | full read-only |
| `open_source_local` (locally hosted) | 4-category classify only | required; small schema | strict; deterministic validators | none — must be reviewed by stronger model |

## What stays constant

Regardless of model class:

- The handoff envelope's **5 elements** are mandatory.
- The BoundaryCard's `risk_domain` and `stop_conditions` are mandatory.
- The EvidencePacket's `source` field is mandatory and the gate
  enforces `materialized_repo` for `code_change` class.
- The UnknownsMap's 3 categories (user / agent / execution) are
  mandatory — only **how many entries per category** is adapted.

## What changes per model

| Aspect | Strong model | Weak model |
|---|---|---|
| Prompt verbosity | terse, structured | explicit examples, longer instructions |
| Output schema strictness | structural only | field-by-field validation |
| Number of allowed retries | 3 | 1 (with hard cap) |
| Allowed tool surface | full | allowlist only |
| Cost ratio | `1x` (can afford re-runs) | `0.1x` (one-shot matters) |

## Why this skill avoids over-tuning

Per-model adaptation risks** over-tuning to a specific model version**
that goes stale. To mitigate:

1. Define adaptations by **capability class**, not by model name.
   A new version of MiniMax falls into `cost_effective_worker`
   automatically.
2. Keep adaptations in YAML, not prompt strings. Easier to track
   drift.
3. Review the adaptation table quarterly against trace data.

## Relationship to ModelCapabilityProfile

`ModelCapabilityProfile` (separate reference doc) declares the
profile. `Per-model adaptation` (this doc) is the **scaffolding
response** to the profile. The orchestrator reads both.

## Cross-reference

- `references/boundary-card.md` — uses the profile to score
  `model_fit`.
- `references/pitfalls.md` — over-tuning is one of the pitfalls.
