# Model Capability Profile

> Per-model record of what it's good at and what it should never do.
> Source: `model-self-improvement-and-boundary-awareness.md` §4.1.

## Why per-model profiles matter

The original Doc 2's insight: models don't have the same capability
distribution. Models with similar benchmark scores can have very
different blindspot patterns (Claude Fable 5 is best at long-horizon
planning; Codex is best at repo-grounded engineering; Claude Mythos
5 is best at scientific reasoning — and Claude Fable 5 plus the
capability classifier routes cyber/bio/distillation requests back
to Opus 4.8).

Two models that both score 80% on the same benchmark can fail in
different 20%. The profile lets the orchestrator pick the right one
for each task and refuse the wrong one.

## Schema (version 1)

```yaml
model_capability_profile:
  schema_version: 1

  id: claude-fable-5                       # machine id
  human_name: "Claude Fable 5"
  vendor: anthropic
  family: claude
  release_date: 2026-06-09

  class: frontier_long_horizon            # see Classes below

  pricing_usd_per_million_tokens:
    input: 10
    output: 50

  context_window_tokens: 1000000

  strengths:
    - "long-horizon planning"
    - "code generation (SWE-Bench Pro 80.3%)"
    - "vision-grounded reasoning"
    - "scientific review"

  weak_spots:
    - "short chats are over-priced for the capability"
    - "tendency to over-plan under 'high effort' prompts"
    - "may perform unrequested cleanup actions"

  recommended_tasks:
    - "complex multi-day R&D runs"
    - "code review on critical paths"
    - "designing new architecture"

  forbidden_or_gate_required:
    - "permission policy change"
    - "safety rule change"
    - "system prompt modification"
    - "model routing change"

  required_scaffolding:
    - "strict output schema"
    - "handoff envelope (5-element)"
    - "external verifier for risks ≥ high"

  capability_classifier:
    present: true
    routes_to: claude-opus-4.8
    triggers:
      - "cybersecurity instructions"
      - "biological/chemical weapon content"
      - "model distillation requests"

  rate_limits:
    requests_per_minute: 50
    tokens_per_minute: 5000000

  public_evidence:
    swe_bench_pro: 0.803
    gdp_pdf_vision: 0.298
    public_claude_fable_5_repo: "https://github.com/anthropics/claude-fable-5-public-evals"
```

## Capability classes (the spectrum)

The profile declares a `class` for routing. Recommended classes:

| Class | Description | Examples (illustrative, verify before use) |
|---|---|---|
| `frontier_long_horizon` | Long context, memory, multi-day planning | Claude Fable 5, Claude Mythos 5 |
| `frontier_engineering` | Repo-grounded engineering, test/verify | Codex, Claude Fable 5 + engineering scaffolding |
| `team_ux_native` | Persistent team runtime, mailbox, task board | Claude Agent Teams (when used as runtime) |
| `team_orchestrator` | Plan, route, summarize; cheap at scale | MiniMax orchestrator-tier |
| `cost_effective_worker` | Bulk extraction, drafting, formatting | MiniMax worker-tier, open-source small models |
| `research_specialist` | Citations, recency, structured analysis | mid-tier hosted with browsing |
| `open_source_local` | Local, no API cost, lower reliability | locally hosted models |

These are routing hints, not absolute. The orchestrator can
override per Boundary Card.

## What should NEVER be in a profile

- **No ratings scored by this skill.** Profiles are user/team-curated
  or trace-derived. They don't pretend to be objective measurements.
- **No "is generally bad at X" claim.** Profiles are about *what
  this profile routes the model toward / away from*, not broad
  judgement.
- **No copy of model system prompts.** Profiles are capability
  routing, not instruction redistribution.

## Where profiles live

```
.loom/models/profiles/
  claude-fable-5.yaml
  claude-mythos-5.yaml
  claude-opus-4.8.yaml
  codex.yaml
  minimax-orchestrator.yaml
  minimax-worker.yaml
  local-llama-3-70b.yaml
  ...
```

A profile is loaded by the orchestrator at preflight and consulted
whenever a BoundaryCard is emitted.

## Profile updates

Profiles get stale. Two triggers for update:

1. **Trace-derived**: a run trace shows the chosen model failed in
   `weak_spots`-listed scenarios at a rate > 1%. → emit ImprovementCandidate
   on the profile.
2. **Vendor release**: a model release (e.g. new Claude / new Codex
   version) → human or frontier reviewer curates a new profile.

## Cross-reference

- `references/per-model-adaptation.md` adjusts scaffolding per
  model-class. Profiles are the input to that decision.
- `references/boundary-card.md` reads profile fields to compute
  `model_fit.score`.
