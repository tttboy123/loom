# Loom Current Architecture And Flow

## Status

Snapshot pointer for the current repo shape on 2026-07-03.

## Purpose

The polished, open-source style architecture diagrams now live in:

- `docs/architecture/loom-architecture.md` (中文)
- `docs/architecture/loom-architecture.en.md` (English)

This file remains as a stable compatibility pointer because earlier planning and
conversation references used this path.

## Current Summary

Loom is currently a local, file-backed, quota-aware agent development harness.
Its core execution path is:

```text
task or backlog item
  -> iterate / autopilot
  -> rdloop.py stage kernel
  -> brainstorm / plan / implement / verify / review
  -> run artifacts, gates, reflection, backlog updates
```

The target direction is additive:

- keep `rdloop.py` as the execution kernel
- add `GoalSpec`, `WorkItem`, `EvidencePacket`, `scheduler`, `state_writer`,
  `observer`, `triager`, `repairer`, and `gatekeeper`
- preserve `single-agent`, `agent-team`, and `cluster` as first-class operating
  modes
- treat Agentic MapReduce as one `cluster` strategy, not the default for every
  task

## Read Next

1. `docs/architecture/loom-architecture.md` or `docs/architecture/loom-architecture.en.md`
2. `docs/loom-stable-agent-runtime-blueprint.md`
3. `docs/loom-runtime-evolution-evidence-2026-07-03.md`
4. `docs/loom-control-plane-evolution.md`
