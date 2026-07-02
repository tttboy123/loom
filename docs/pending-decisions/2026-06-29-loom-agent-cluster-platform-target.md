# Loom Agent Cluster Platform Target

## Status

Proposed / pending decision

## Date

2026-06-29

## Context

Loom today is a local, quota-aware, cross-agent R&D harness. Its current core is `devkit/rdloop.py`: a staged loop that produces product judgment, implementation planning, TDD implementation draft, verification, and independent review. The existing role/carrier design already keeps role names stable while allowing the underlying model vendor to change through LiteLLM configuration.

Recent exploration compared the current Loom shape with emerging agent products and platforms such as Paperclip-style AgentOps control planes, long-running harness patterns, multi-agent frameworks, and cloud coding agents. The main conclusion is that the market already has pieces of the target shape, but not a single settled product that combines low-friction launch, user-defined roles, long-running agent clusters, R&D-grade evidence gates, multi-model routing, and an operational cockpit.

This document records the proposed target product shape so Loom can decide whether and how to evolve from a single-loop devkit into a production-grade agent cluster runtime.

The latest implementation-facing blueprint for this direction is
`docs/loom-stable-agent-runtime-blueprint.md`. It keeps this target Loom-only,
adds concrete Mermaid architecture and flow diagrams, and defines the control
plane, evidence plane, repair lane, and Agentic MapReduce cluster strategy in a
form that future agents can implement against.

## Proposed Product Direction

Loom should not become a fixed CEO / PM / Engineer role-playing product. Those roles can exist as presets, but they should not be hard-coded into the product worldview.

The target product is:

**A configurable, long-running Agent Cluster Runtime that lets users submit a goal, define or select roles, run multiple agents over time, observe task distribution and cost, recover from failures, and gate results through evidence and review.**

In shorter product language:

**Loom lets users launch the smallest useful agent organization, from a single agent to a team to a cluster, using their own roles, tools, budgets, and rules.**

Cluster is the highest-level operating mode, not the only mode. Loom must continue to support smaller operating shapes because a cluster is composed from them:

- `Single Agent`: one role or one existing agent handles a bounded task.
- `Agent Team`: a small role set cooperates through a stable handoff pattern.
- `Cluster`: multiple single agents and/or teams are scheduled as a run group with dependencies, shared budget, shared observability, and final synthesis.

The product should make these modes explicit instead of implying that every task must become a cluster. Small tasks should stay small; cluster mode should be used when decomposition, parallelism, or long-running recovery is worth the overhead.

## Product Shape

### 1. Operating Modes

Loom should expose three first-class operating modes:

| Mode | What It Runs | Best For | Output |
| --- | --- | --- | --- |
| `single-agent` | One role / carrier / executor path | Small bounded tasks, quick checks, one-off research or implementation drafts | Single run + artifacts |
| `agent-team` | A small role graph such as planner -> implementer -> reviewer | Normal R&D loop, research synthesis, bugfix with review | Team run + stage artifacts + gate |
| `cluster` | Multiple single agents and/or teams as a run group | Large goals, parallel research, multi-module implementation, long-running work | Run group + work item graph + synthesis |

This keeps the user mental model simple: choose the smallest organization shape that can safely complete the task.

### 2. Loom Launcher

The user-facing task entry point.

Users should not need to know `stage`, `carrier`, `cascade`, `contract`, `blind_review`, or `iterate` before starting. A minimal task should be launchable with:

```yaml
mode: cluster
template: product-build
goal: Turn the settings page PRD into a testable implementation draft.
budget: 20
risk: medium
workspace: ./my-app
```

Launcher translates this into a structured task specification and selects safe defaults for roles, tools, routing, review, budget, and reporting.

### 3. Role Registry

Roles are user-configurable runtime entities, not fixed product characters.

Each role should define:

- `id`
- display name
- responsibilities
- model policy
- allowed tools
- permissions
- budget limit
- input/output contract
- handoff and review relationships

Example:

```yaml
roles:
  - id: planner
    name: Product Planner
    responsibilities:
      - clarify goal
      - split work items
      - define acceptance
    model_policy: planning-heavy
    tools: [repo-read, docs-read]
    permissions: read-only

  - id: implementer
    name: Engineer
    responsibilities:
      - produce patch
      - write tests
      - report blockers
    model_policy: coding-main
    tools: [repo-read, terminal, repo-write-sandbox]
    permissions: sandbox-write

  - id: reviewer
    name: Independent Reviewer
    responsibilities:
      - inspect evidence
      - check tests
      - decide go/no-go
    model_policy: review-independent
    tools: [repo-read, test-report]
    permissions: read-only
```

CEO / PM / Engineer can be shipped as an optional template pack, not as the canonical data model.

### 4. Task Pack

Loom should move from a single prompt to a structured task package.

The task pack should capture:

- operating mode
- goal
- context
- constraints
- expected artifacts
- risk level
- budget
- roles
- tools
- acceptance criteria
- human gates

It should then expand into work items:

```yaml
work_items:
  - id: spec-001
    owner_role: planner
    objective: Define the settings page scope and acceptance criteria.
    done: Scope, non-goals, and acceptance are explicit.

  - id: impl-001
    owner_role: implementer
    objective: Produce implementation draft and tests from the accepted scope.
    depends_on: [spec-001]

  - id: review-001
    owner_role: reviewer
    objective: Review evidence, tests, and implementation boundaries.
    depends_on: [impl-001]
```

This is the key shift from "one loop runs one task" to "one goal becomes a managed run group."

In `single-agent` mode, the task pack may contain one work item. In `agent-team` mode, it should contain a small ordered handoff graph. In `cluster` mode, it can contain multiple teams or independent work streams with dependencies.

### 5. Cluster Runtime

`rdloop.py` should remain the single-run execution kernel. The new platform layer should sit above it.

Target runtime responsibilities:

- execute single-agent runs without cluster overhead
- execute agent-team runs as a compact handoff graph
- create run groups
- start multiple child runs
- assign work items to roles
- respect dependencies
- enforce concurrency limits
- retry or escalate failed work items
- support pause, resume, cancel, and timeout
- preserve run state for long tasks
- roll up artifacts and gate decisions

The likely first implementation is a light `Swarm Orchestrator` that calls existing `run_loop` instances rather than rewriting the core loop.

### 6. Loom Cockpit

The cockpit should make long-running agent work observable and governable.

Minimum cockpit concepts:

- current goal
- run group status
- work item DAG
- role swimlane
- dispatch timeline
- cost and token usage
- model routing and fallback trace
- tool usage trace
- blocker taxonomy
- artifact list
- review and gate state
- human approval queue

The cockpit should support two views:

- `Execution View`: real stages, runs, artifacts, costs, and gates.
- `Role View`: user-defined roles, handoffs, assignments, rejected work, and synthesis.

## Capability Map

### A. Operating Modes

- Single agent execution
- Agent team execution
- Cluster / run group execution
- Mode-aware templates
- Mode-aware cost and risk defaults
- Upgrade path from single -> team -> cluster when the task needs more structure

### B. Role And Organization

- Custom roles
- Role templates
- Per-role model policy
- Per-role tool scope
- Per-role permissions
- Handoff relationships
- Review relationships

### C. Long-Running Tasks

- Durable state
- Resume from interruption
- Pause / resume / cancel
- Timeout handling
- Human steering during execution
- Partial retry
- Per-work-item status

### D. Multi-Agent Cluster Execution

- Goal to task-pack conversion
- Work item dependency graph
- Parallel child runs
- Independent work item gates
- Run group summary
- Synthesis report

### E. Model Routing

- Role-level model policy
- Task-level model policy
- Health probes
- Fallback chains
- Cost-aware routing
- Latency-aware routing
- Benchmark-gated model adoption

### F. Tools And Permissions

- Tool registry
- Tool allowlists
- Workspace/path allowlists
- Read-only / sandbox-write / real-write permission tiers
- External action approval
- Secret redaction
- Audit log

### G. Review And Acceptance

- Acceptance contracts
- Golden evals
- Blind review
- Independent reviewer roles
- Evidence gates
- Regression checks
- GO / NO-GO verdicts
- No fake completion rule

### H. Observability And AgentOps

- Run timeline
- Role swimlane
- Task DAG
- Cost ledger
- Token usage
- Fallback trace
- Tool trace
- Artifact versions
- Retry history
- Approval history

### I. Low-Friction Templates

Initial templates can include:

- `single-agent-chat`
- `single-agent-code`
- `agent-team-rdloop`
- `spec-to-plan`
- `coding-fast`
- `coding-safe`
- `bugfix-verify`
- `research-deep`
- `release-check`
- `multi-role-custom`

Each template should provide default roles, tools, model policy, budget, review mode, and risk posture.

## Current Loom Versus Target Shape

| Dimension | Current Loom | Target Loom |
| --- | --- | --- |
| Product role | Local R&D loop devkit | Long-running agent cluster runtime |
| User entry | CLI task + flags | Launcher + task file + templates |
| Operating modes | Mostly one staged single-run shape | `single-agent`, `agent-team`, and `cluster` modes |
| Core unit | Stage and single run | Task pack, work item, run group |
| Roles | Mostly fixed stage roles | User-defined roles with templates |
| Execution | One vertical loop | Smallest viable mode: single run, team handoff, or multiple child runs with dependencies |
| Long-running support | Resume and iterate primitives | Durable queue, pause/resume/cancel, partial retry |
| Model routing | Role carrier mapping and fallback | Per-role and per-task policy with routing trace |
| Safety | L1/report-only default | Policy engine, sandbox tiers, approval queue |
| Review | Contract, golden eval, blind review | Platform-level evidence and gate system |
| Observability | run-log, artifacts, LiteLLM logs | Cockpit with DAG, swimlane, timeline, cost, gates |
| User mental model | Understand stages and flags | Submit goal, choose template, monitor team runtime |

## MVP Scope

The first version should prove that Loom can run bounded long tasks through single-agent, agent-team, and cluster modes without making users learn the internal loop machinery.

Required MVP capabilities:

- `mode` support for `single-agent`, `agent-team`, and `cluster`
- `roles.yaml`
- `loom.task.yaml`
- `TaskSpec`
- `WorkItem`
- `RunGroup`
- light `Swarm Orchestrator`
- at least two child `run_loop` executions
- persistent per-work-item status
- per-work-item artifacts and gate
- failed work item retry
- unified `summary.md`
- role timeline data
- task DAG data
- budget guard
- human gate markers

MVP should not force every task through cluster mode. It should prove that the same task-file entry model can run a small single-agent task, a normal agent-team task, and a bounded cluster task.

Explicitly out of MVP:

- multi-tenant SaaS
- marketplace
- enterprise SSO
- full billing
- complex permission UI
- automatic production deployment
- broad enterprise admin console

## Decisions Needed

### D1. Is Loom's target product a cluster runtime?

Recommended decision: yes.

Loom should evolve from a single-loop devkit into a long-running agent cluster runtime while keeping `rdloop.py` as the execution kernel.

### D2. Should organization roles be fixed?

Recommended decision: no.

CEO / PM / Engineer should be templates only. The core model should be user-defined roles.

### D3. Should the platform start with CLI/task-file or UI cockpit?

Recommended decision: CLI/task-file first, cockpit data model second, UI third.

This keeps the first implementation testable and avoids blocking runtime progress on UI polish.

### D4. Should Loom implement a heavy scheduler immediately?

Recommended decision: no.

Start with a lightweight local run-group orchestrator. Add queue, lease, heartbeat, and dead-letter behavior after task-pack execution is proven.

### D5. Should Loom keep report-only as the default?

Recommended decision: yes.

Default to read-only/report-only, then allow sandbox-write and real-write through explicit policy and approval.

### D6. Should cluster mode replace single-agent and team mode?

Recommended decision: no.

Cluster mode should compose smaller operating shapes. `Single Agent` and `Agent Team` remain first-class modes because many useful tasks do not need the overhead of a run group.

## Proposed Implementation Order

1. Define schema files for `roles.yaml`, `loom.task.yaml`, `TaskSpec`, `WorkItem`, `RunGroup`, and `mode`.
2. Implement `single-agent` mode as a thin wrapper over one existing `run_loop`.
3. Implement `agent-team` mode as a compact ordered handoff graph over existing stages.
4. Add a local `Swarm Orchestrator` for `cluster` mode that expands one task into multiple work items.
5. Run at least two child `run_loop` executions from a single run group.
6. Persist per-work-item state and artifacts.
7. Generate a unified `summary.md`.
8. Add retry for failed work items.
9. Add cockpit-ready JSON for task DAG, role timeline, costs, and gates.
10. Build UI cockpit on top of the persisted run-group state.

## Acceptance Criteria For The Direction

This target direction is considered validated when:

1. A user can start `single-agent`, `agent-team`, and `cluster` runs from one `loom.task.yaml` shape.
2. Single-agent mode can run without unnecessary task-pack fanout.
3. Agent-team mode can run a compact role handoff with visible artifacts and gate.
4. Cluster mode expands the task into multiple work items with roles and dependencies.
5. At least two work items can run independently through existing `rdloop.py`.
6. Each work item has visible status, artifacts, cost, and gate.
7. A failed work item can be retried without restarting the entire goal.
8. The run group produces a readable final summary.
9. The output clearly shows how the task was distributed, executed, reviewed, and closed.

## Notes

- This document is a target product decision candidate, not an accepted ADR.
- It should not be treated as permission to rewrite Loom core.
- The preferred direction is additive: preserve `rdloop.py`, add a platform layer above it.
- Paperclip should remain a reference for AgentOps and organizational visibility, not the source of Loom's data model.
- Cluster is a composition mode, not a replacement for single-agent or team execution.
