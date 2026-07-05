# Loom skills (capability layer)

> Two skills packaged from lune's 2026-07-05 design drafts:
> - `agentteam-runtime/`: orchestrating single / team / cluster runs in
>   Loom with team protocol, handoff envelopes, gate policy, cockpit.
> - `boundary-aware-agent/`: externalizing Fable-5/Opus-style
>   capability boundary so other models can follow the same
>   workflow via runtime scaffolding.

Each skill follows the Mavis skill layout (SKILL.md + _meta.json +
references/ + assets/templates/).

## Read order

1. **This README** for the index + cross-references.
2. Pick the skill you need:
   - `agentteam-runtime/` for orchestration questions.
   - `boundary-aware-agent/` for per-agent capability questions.
3. Each skill's `README.md` → `SKILL.md` → `references/<topic>.md`.

## Skill interaction diagram

```
              user goal
                  │
                  ▼
    ┌────────────────────────────────┐
    │   agentteam-runtime (skill)    │   ← orchestrates team runs
    │                                │
    │   TeamSpec + UnknownsMap +     │
    │   HandoffEnvelope + GatePolicy│
    │   + Cockpit + ImprovementCand │
    │                                │
    │   consumes BoundaryCard from   │
    │   ──────────────────────────┐ │
    └────────────────────────────────┘
                                  │
                                  ▼
    ┌─────────────────────────────────┐
    │ boundary-aware-agent (skill)  │   ← per-agent capability
    │                                │
    │ Blindspot + Boundary + Evidence│
    │ + Improvement + Profile         │
    │                                │
    │ consumes team output as RunTrace│
    └─────────────────────────────────┘
                  │
                  ▼
         Loom runtime (devkit/*,
         state_writer, repairer,
         goal_controller (planned),
         etc.)
                  │
                  ▼
            .loom/runs/<id>/
            (durable artifacts)
```

The two skills are **layered**: agentteam-runtime is the runtime
producer / consumer; boundary-aware-agent supplies the per-agent
*evidence* + *profile* artifacts.

## Shared types (kept compatible between the two skills)

| Type | producer | consumer |
|---|---|---|
| `UnknownsMap` | boundary-aware-agent (Blindspot Pass) | agentteam-runtime (plan_gate) |
| `ImprovementCandidate` | both | both |
| `BoundaryCard` | boundary-aware-agent | agentteam-runtime (per-task) |
| `ModelCapabilityProfile` | boundary-aware-agent | agentteam-runtime (model selection) |
| `EvidencePacket` | (Loom primitive) | both consume |
| `EvalPatch` | boundary-aware-agent | agentteam-runtime (canary) |

Schemas are kept **identical** between the two skills. See
`cross-reference.md` for the schema-mapping tables.

## Source documents

- **Doc 1**: `loom-agent-team-runtime-proposal.md` (lune, 2026-07-05)
  → packaged into `agentteam-runtime/`.
- **Doc 2**: `model-self-improvement-and-boundary-awareness.md`
  (lune, 2026-07-05) → packaged into `boundary-aware-agent/`.

## Companion documents

- **`loom-control-theory-extension-v2-2026-07-05.md`** on branch
  `feat/docs-control-theory-extension` (commit `1826bf0`).
  Defines the runtime hard guarantees (SCDES supervisor, CBF
  barrier, ranking function with lexicographic priority, queueing
  admission) that **both** skills depend on for safe execution.

## Cross-cutting rules (both skills enforce)

1. **Producer cannot self-approve.** Reviewer is always a
   fresh-context model.
2. **No modification of system prompt, weights, permission policy,
   safety rule, model routing.** Ever.
3. **Improvement candidates only — never auto-apply.** Even
   `template_text` patches require explicit approval unless their
   risk level is `low` AND they pass synthetic eval.
4. **UnknownsMap first** for any non-trivial run. No Inbox → Assigned
   without it.
5. **Every claim tagged** `verified | inferred | unverified`.
   Unverified is allowed; unsupported is not.

## Reusing these skills elsewhere

These skills are Loom-runtime agnostic in concept — they describe
a team + boundary workflow that any agent orchestration runtime can
adopt. To reuse outside Loom:

- Re-implement the run directory layout (`.loom/runs/<id>/`).
- Re-implement the gate enforcement.
- Keep the data object schemas unchanged.

See `cross-reference.md` for the canonical object catalog.
