# Four-layer framework

> The conceptual backbone of this skill. Source:
> `model-self-improvement-and-boundary-awareness.md` §2, §3.

## What problem is being decomposed?

"Self-evolution" and "blindspot discovery" sound like one capability.
They are four. Mixing them is the source of most over-promises in
agent capability docs.

| Layer | Question | Output |
|---|---|---|
| **L1 User Blindspot Discovery** | Where did the user not specify something that could change the answer? | `UnknownsMap` |
| **L2 Model Boundary Awareness** | Is the current model / tool / context capable of doing this task? | `BoundaryCard` |
| **L3 Evidence-Grounded Execution** | Is the claim backed by evidence from this run? | claim tagged `verified \| inferred \| unverified` |
| **L4 Agentic Self-Improvement** | What should we change for next time? | `ImprovementCandidate` |

## Why four layers, not three or five

- **Not three**: collapsing L1 (user unknown) into L2 (model unknown)
  blurs who is responsible for filling the gap. L1 is the agent's
  *job*; L2 is its *limit*. Mix the two and you cannot tell whether
  to ask the user or change the model.
- **Not five**: a fifth "self-awareness" or "meta-cognition" layer
  adds noise without changing the artifacts. Self-awareness maps to
  L2 + L3.
- **Four keeps each output object single-purpose and machine-emittable.**

## What Fable 5 / Opus do well

Claude Fable 5 (released 2026-06-09, $10/$50 per million tokens)
naturally covers L1-L4 in a single session because:

- Long context + memory: can keep L1 unknowns alive across tool calls
  (reduces garbage-collected blindspots).
- Long-horizon planning: catches L2 limits mid-execution and asks
  before going off the rail.
- Effort-aware planning: emits L3 evidence before claiming
  completion (rare; most models skip this step).
- Reflection loop: produces L4 candidate patches without prompting
  (closed-loop maintenance).

Open-source and cheaper models can do all four with explicit
scaffolding — that's exactly what this skill provides.

## Where do these artifacts live in Loom?

| Layer | Object | Loom storage |
|---|---|---|
| L1 | `UnknownsMap` | `.loom/runs/<id>/unknowns-map.yaml` |
| L2 | `BoundaryCard` | per-task `.loom/runs/<id>/boundary/<task_id>.yaml` |
| L3 | `evidence-grounding` | per-claim tagging in progress reports + EvidencePacket.source field |
| L4 | `ImprovementCandidate` | `.loom/runs/<id>/improvement/candidates/` |

## When does each layer activate?

| Trigger | Activates |
|---|---|
| User goal received, before team plan | L1 |
| Task about to start (per task) | L2 |
| Producer about to claim progress or Done | L3 |
| Run completes (per run) | L4 |

Each layer's output becomes input to the next. L1 → L2 → L3 → L4
is the canonical flow.

## Why this skill won't replace the agent's character

Fable 5 is genuinely more capable than alternatives on long-horizon
planning. The skill cannot synthesize that character into MiniMax
overnight. But it can give MiniMax the same artifacts + gates so
the team has *visible capability ground truth* instead of guessing
what any given model can do.

In other words: this skill does not pretend models are equally
capable. It makes capability differences *explicit and verifiable*
so the orchestrator can route appropriately.
