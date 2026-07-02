# Loom Control Plane Evolution

## Purpose

This document captures the current Loom runtime architecture and the intended
evolution from a script-oriented multi-agent development loop into a stable
local control plane for long-running Agent Teams.

The target audience is any model or engineer reasoning about Loom's next
platform phase.

For the diagram-first, human-and-agent-readable blueprint of the latest target
architecture, see `docs/loom-stable-agent-runtime-blueprint.md`. This file keeps
the control-plane evolution rationale; the blueprint turns that rationale into
planes, roles, state machines, Agentic MapReduce flow, and near-term contracts.

## Current System

Loom today is a local, quota-aware agent development harness with:

- a staged execution kernel in `devkit/rdloop.py`
- autonomous iteration in `python3 -m devkit iterate`
- background supervision via `scripts/loom-iterate-daemon.sh` and
  `scripts/loom-iterate-supervisor.sh`
- per-run artifacts under `devkit/runs/<run-id>/`
- LiteLLM-based model indirection in `litellm/config.full.yaml`

### Current Agent Team

The currently active execution team is:

1. `discover` (not always inserted into each run)
2. `brainstorm`
3. `plan`
4. `implement`
5. `verify`
6. `review`

Default model split:

- product/planning/review: GPT-5.4
- implementation/testing: MiniMax-M3
- reflection loop: MiniMax

### Current Runtime Shape

```text
goal or backlog item
  -> devkit iterate / auto
  -> run_loop (brainstorm/plan/implement/verify/review)
  -> run artifacts + run-log
  -> reflection
  -> backlog rewrite
  -> next round
```

### Current Observability

Current observability exists, but is file-centric rather than object-centric:

- `devkit/logs/iterate-daemon.log`
- `devkit/logs/iterate-supervisor.log`
- `devkit/logs/task-queue-status.log`
- `devkit/runs/<run-id>/run-log.md`
- `/api/progress`
- `/api/backlog-health`
- `/api/agent-observability`

## Current Problems

### 1. Goal submission is not declarative

Users still think in terms of CLI flags such as:

- `--carrier`
- `--cascade`
- `--iterate`
- `--reflect-carrier`

This exposes internal orchestration mechanics instead of a stable control API.

### 2. Backlog mutation is fragile

The current backlog write pattern historically allowed stale in-memory backlog
snapshots to overwrite newer on-disk tasks. This is especially dangerous when:

- a human injects strategic tasks
- reflection adds new tasks
- multiple loops or recovery flows write close together

### 3. Observability is not a first-class subAgent

Loom can observe itself, but the observer is not yet modeled as a durable role
 in the Team. There is no explicit:

- `observe`
- `triage`
- `repair`
- `governor`

plane.

### 4. Controller semantics are weak

The current system behaves more like a resilient loop runner than a proper
control plane. It lacks durable notions of:

- desired state
- lease ownership
- heartbeat
- stale reclaim
- reconciliation status

### 5. Strategic platform work can be drowned by local repair churn

Without priority governance, long chains of low-level fix tasks can dominate the
autonomous queue and delay higher-value platform evolution.

## Target System

The target shape is not literal Kubernetes, but it should borrow the same
operational ideas:

- declarative desired state
- controllers that reconcile actual state toward desired state
- durable per-object status
- clear observability and event streams
- automatic degradation and recovery

### Target Architecture

```text
Goal Spec Layer
  - goal yaml/json
  - mode: single-agent / agent-team / cluster
  - policy: standard / no-gpt / cheap / resilient

Control Plane
  - scheduler
  - model-policy resolver
  - backlog controller
  - retry/degrade controller
  - lease/heartbeat controller

Execution Plane
  - discover
  - brainstorm
  - plan
  - implement
  - verify
  - review

Observability Plane
  - observe
  - triage
  - repair
  - governor

Evidence Plane
  - artifacts
  - tests
  - verify results
  - review verdicts
  - failure codes
  - decisions

Cockpit
  - run graph
  - agent swimlane
  - model routing
  - queue health
  - incidents
  - budgets
```

## Evolution Plan

### Phase 1: Declarative entry

Introduce a goal spec file and normalize user intent into a stable object.

Deliverables:

- `goal-spec-v1`
- `goal-submit-cli-v1`

Expected result:

- users submit a goal file instead of stitching together long CLI commands

### Phase 2: Model policy layer

Introduce stable policy presets so model routing is a product decision surface,
not a raw flag surface.

Deliverables:

- `model-policy-presets-v1`
- `autopilot-policy-degrade-v1`

Expected result:

- GPT exhaustion or capacity issues can trigger policy downgrade automatically
- the user selects a mode like `no-gpt` or `resilient`, not raw carriers

### Phase 3: Observability as a Team capability

Promote observability from scattered scripts to explicit machine-readable runtime
contracts and API surfaces.

Deliverables:

- `observer-subagent-contract-v1`
- `team-observability-api-v1`
- `team-cockpit-summary-v1`

Expected result:

- the system can explain who is running, what is stuck, what degraded, and why

### Phase 4: Controller primitives

Introduce minimal durability primitives before attempting a true cluster mode.

Deliverables:

- `controller-lease-heartbeat-v1`

Expected result:

- running tasks can be reclaimed safely after worker loss or stale execution

### Phase 5: Cluster mode

Only after the earlier phases are stable should Loom expand into a true
multi-child run-group runtime.

Expected future features:

- work-item DAG
- per-work-item lease
- per-team budget
- final synthesis run

## Proposed Additional Team Roles

The execution team should be augmented with a platform operations team:

1. `observe`
2. `triage`
3. `repair`
4. `governor`

### Role Intent

- `observe`: collect runtime signals and summarize anomalies
- `triage`: classify incidents by cause family
- `repair`: execute known remediations or generate precise fix tasks
- `governor`: throttle, pause, degrade, or reroute work under quota or health pressure

## Near-Term Priority

The immediate engineering priority is:

1. prevent backlog overwrite and stale-write loss
2. preserve strategic tasks once injected
3. shift user interaction from raw runtime flags to goal spec + policy
4. expose team observability in a stable control-plane contract

## Success Criteria

Loom should be considered to have crossed into a control-plane architecture when
all of the following are true:

1. users submit goals declaratively
2. model fallback is expressed as policy, not ad hoc CLI wiring
3. observability exists as a first-class machine-readable Team capability
4. stale workers can be reclaimed via lease/heartbeat logic
5. strategic platform tasks are not silently dropped by queue mutation
6. the cockpit clearly shows run state, agent state, model routing, incidents,
   and budget posture
