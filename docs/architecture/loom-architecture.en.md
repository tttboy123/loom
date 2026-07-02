# Loom Architecture

> Language: [中文](./loom-architecture.md) | English

## Status

Current architecture snapshot with proposed stable-runtime overlays.

Last reviewed: 2026-07-03.

## Audience

This document is for contributors, maintainers, and agents that need to understand how Loom works before changing it.

It uses C4-style levels with standard Mermaid diagrams:

- **Context**: what Loom is, and what is outside it.
- **Containers**: local processes, services, and stores.
- **Components**: important modules inside the `devkit` runtime.
- **Runtime flows**: the main execution, repair, and Agentic MapReduce paths.

Legend:

- `current`: present in the repo today.
- `target`: planned stable-runtime overlay.
- `optional`: enabled only in expanded profiles or future modes.

## Architecture Summary

Loom is a local, quota-aware agent runtime for code and repository work. Today it runs a file-backed R&D loop over `backlog.json`, `devkit/runs/`, model carriers, and stage artifacts. The intended evolution is to keep `rdloop.py` as the execution kernel while adding a stable control layer around it.

The most important boundary is this:

**Execution agents produce artifacts. Control-plane components decide scheduling, repair, and final state.**

That boundary is what turns Loom from a prompt chain into a stable local agent runtime.

## Context View

This is the highest-level map. Loom sits between humans/upstream agents, local workspaces, model providers, and local verification tools.

```mermaid
flowchart TB
  human["Human operator\nsubmits goals, reviews gates"]
  upstream["Upstream agent\nsubmits bounded tasks"]

  subgraph loom["Loom local runtime"]
    entry["CLI / Console\ncurrent"]
    runtime["Devkit runtime\ncurrent"]
    gateway["LiteLLM gateway\ncurrent"]
    cockpit["Dashboard / observability\ncurrent -> target cockpit"]
  end

  workspace["Target workspace\nrepo files, tests, docs"]
  shell["Local shell\ncommands, tests, scripts"]
  providers["Model providers\nCodex, MiniMax, GLM, DeepSeek, Claude-compatible"]

  human -->|"goal, task, approval"| entry
  upstream -->|"task file / bounded request"| entry
  entry --> runtime
  runtime -->|"read context / apply approved output"| workspace
  runtime -->|"run checks"| shell
  runtime -->|"role model requests"| gateway
  gateway -->|"provider routing + fallback"| providers
  runtime --> cockpit
  human -->|"inspect state"| cockpit
```

Why this matters:

- Loom owns local orchestration and evidence.
- Model providers are replaceable execution backends, not the architecture.
- The workspace remains outside Loom and should only be changed through policy.

## Container View

This view shows Loom's current local containers. A container here means a runnable process, service, or persistent store, not a Python class.

```mermaid
flowchart TB
  user["Human / agent"]

  subgraph loom["Loom"]
    cli["Loom CLI\n./loom + python -m devkit"]
    console["Console / dashboard\nlocalhost UI"]
    devkit["Devkit runtime\nPython execution kernel"]
    autopilot["Autopilot supervisor\nshell daemon + watchdog"]
    litellm["LiteLLM gateway\nrole carrier routing"]
    agentui["AgentOS / Agent UI\noptional full profile"]
    state[("File state store\nbacklog, runs, logs, decisions")]
  end

  repo["Target workspace"]
  models["Model providers"]

  user --> cli
  user --> console
  cli --> devkit
  cli --> autopilot
  autopilot --> devkit
  console --> state
  devkit --> state
  devkit --> repo
  devkit --> litellm
  agentui --> litellm
  litellm --> models
```

Current control surfaces:

| Surface | Main files | Role |
| --- | --- | --- |
| Entry | `loom`, `devkit/__main__.py` | Start runs, inspect status, launch autopilot |
| Execution kernel | `devkit/rdloop.py` | Run staged agent workflow |
| Queue | `devkit/backlog.json`, `devkit/backlog.py`, `devkit/iterate.py` | Select and update work |
| Delivery policy | `devkit/delivery_mode.py`, `devkit/task_contract.py`, `devkit/applylock.py` | Control report-only vs apply behavior |
| Model routing | `devkit/model_aliases.py`, `devkit/carrier_fallback.py`, LiteLLM config | Map roles to model carriers and fallback |
| Observability | `devkit/agent_observability.py`, `devkit/dashboard.py`, task queue scripts | Project queue and run health |
| Background loop | `scripts/loom-iterate-daemon.sh`, `scripts/loom-iterate-supervisor.sh`, `./loom autopilot` | Keep local autonomy running |

## Component View: Devkit Runtime

This view zooms into the `devkit` runtime. The upper row is current. The lower row shows the target stable-runtime components that should be added without rewriting `rdloop.py`.

```mermaid
flowchart TB
  subgraph current["Current devkit runtime"]
    entry["Command entry\ndevkit/__main__.py"]
    iterate["Iterate controller\ndevkit/iterate.py"]
    backlog["Backlog manager\ndevkit/backlog.py"]
    rdloop["R&D stage kernel\ndevkit/rdloop.py"]
    routing["Model routing\naliases + fallback"]
    delivery["Delivery policy\ncontract + applylock"]
    observe["Observability collector\nagent_observability + dashboard"]
    reflect["Reflection loop\nfollow-up tasks"]
  end

  subgraph target["Target control overlay"]
    goal["GoalSpec"]
    workitem["WorkItem"]
    scheduler["Scheduler\nlease + heartbeat"]
    writer["State writer\nsingle transition path"]
    evidence["EvidencePacket"]
    gate["Gatekeeper\nGO / NO-GO / blocked"]
    repair["Observer / triager / repairer"]
  end

  state[("File state store")]
  gateway["LiteLLM gateway"]
  repo["Target workspace"]

  entry --> iterate --> backlog --> rdloop
  rdloop --> routing --> gateway
  rdloop --> delivery --> repo
  rdloop --> state
  rdloop --> reflect --> backlog
  observe --> state

  goal --> workitem --> scheduler
  scheduler -.->|dispatches| rdloop
  rdloop -.->|artifact refs| evidence --> gate --> writer --> state
  writer --> workitem
  workitem --> repair
  repair -.->|repair / reclaim| scheduler
```

Design rule:

**Do not let implementation agents be the final authority for completion.**

They can submit artifacts. Completion should go through evidence, review, and a gatekeeper transition.

## Mixed-Model Execution Strategy

Loom already uses different model classes for different kinds of work. That is the same architectural idea behind Agentic MapReduce: spend frontier-model tokens on high-leverage judgment and cheaper-model tokens on bounded local work.

```mermaid
flowchart LR
  goal["Goal or WorkItem"]

  subgraph frontier["Frontier-oriented judgment"]
    product["product\nproduct judgment"]
    orchestrator["orchestrator\nplanning + dispatch"]
    reviewer["reviewer\nindependent review"]
    reducer["reducer target\ncross-shard synthesis"]
    gatekeeper["gatekeeper target\nfinal state decision"]
  end

  subgraph value["Value-oriented throughput"]
    dev["dev\nimplementation"]
    tester["tester\nverification draft"]
    mapper["mapper target\nbounded shard scan"]
  end

  subgraph deterministic["Deterministic work"]
    selector["selector pass\nrg / AST / static scans"]
    tests["tests and commands\nshell"]
    state["state transition\nstate_writer"]
  end

  subgraph gateway["Role carrier gateway"]
    litellm["LiteLLM\nfallback + usage logs"]
  end

  goal --> product --> orchestrator --> dev --> tester --> reviewer
  goal --> selector --> mapper --> reducer
  tester --> tests
  reviewer --> gatekeeper --> state
  product --> litellm
  orchestrator --> litellm
  dev --> litellm
  tester --> litellm
  reviewer --> litellm
  mapper --> litellm
  reducer --> litellm
```

Policy implication:

- Planning, review, reduction, and gates should prefer stronger or independent models.
- Mapping, local implementation drafts, and routine verification can prefer value models.
- Selector passes, test runs, and state transitions should be deterministic whenever possible.

## Runtime Flow: Current R&D Run

This is the current happy path for a normal task.

```mermaid
sequenceDiagram
  participant U as User
  participant CLI as CLI
  participant I as Iterate
  participant R as rdloop.py
  participant M as LiteLLM
  participant FS as File state
  participant Repo as Workspace
  participant O as Observability

  U->>CLI: submit task or run backlog
  CLI->>I: start auto or iterate
  I->>FS: load backlog and run history
  I->>R: dispatch selected task
  R->>M: call product / plan / dev / tester / reviewer roles
  R->>Repo: read context or apply approved output
  R->>FS: write run artifacts and decisions
  FS->>O: expose queue and latest run state
  I->>FS: reflection adds, splits, or reprioritizes work
```

## Runtime Flow: Target Stable Control Loop

This is the target shape for reliable local autonomy.

```mermaid
sequenceDiagram
  participant S as Scheduler
  participant W as StateWriter
  participant R as Runner
  participant V as Validator
  participant Rev as Reviewer
  participant G as Gatekeeper
  participant O as Observer
  participant T as Triager
  participant Repair as Repairer

  S->>W: acquire lease for ready WorkItem
  W-->>S: lease accepted or rejected
  S->>R: dispatch bounded task context
  R->>W: heartbeat and artifact refs
  R->>V: submit artifact for verification
  V->>Rev: send evidence packet
  Rev->>G: independent review verdict
  G->>W: write done / request_changes / blocked
  O->>W: read status, heartbeat, run events
  O->>T: anomaly packet if stale or suspicious
  T->>Repair: classified incident
  Repair->>W: whitelisted repair or repair WorkItem
```

Key difference from today:

- The runner does not own the final state.
- The scheduler owns leases.
- The observer owns anomaly detection.
- The gatekeeper owns final status transitions.

## Runtime Flow: Target Agentic MapReduce

Agentic MapReduce is a target `cluster` strategy, not the default mode for every task. Use it when the result only becomes trustworthy after broad coverage: repo-wide audits, backlog health analysis, failure-pattern mining, migration planning, or later security-style scans.

```mermaid
flowchart TB
  goal["GoalSpec\nmode: cluster\nstrategy: agentic-mapreduce"]
  planner["Planner\nwrites selector + shard plan\nfrontier model"]
  selector["Deterministic selector pass\nno model in loop"]
  shards[("Shard queue\nbounded files / ids / ranges")]

  subgraph map["Parallel map phase\nvalue models"]
    mapper1["Mapper 1\nfocused shard"]
    mapper2["Mapper 2\nfocused shard"]
    mapper3["Mapper N\nfocused shard"]
  end

  reports[("Shard reports\nfindings + evidence refs")]
  reducer["Reducer\ndedupe + cross-shard synthesis\nfrontier model"]
  verifier["Validator\ncommands / sandbox / repo evidence"]
  reviewer["Fresh reviewer\nindependent judgment"]
  gate["Gatekeeper\nGO / NO-GO / blocked"]
  summary["RunGroup summary\ntrusted findings + limitations"]

  goal --> planner --> selector --> shards
  shards --> mapper1 --> reports
  shards --> mapper2 --> reports
  shards --> mapper3 --> reports
  reports --> reducer --> verifier --> reviewer --> gate --> summary
```

Invariants:

- The selector must be saved as an artifact.
- Each shard must have explicit boundaries.
- Mapper agents should be read-only by default.
- Reducer must preserve evidence references.
- Verifier must distinguish `inner_sandbox`, `materialized_repo`, and `unknown`.
- Gatekeeper must report limitations instead of fabricating certainty.

## Runtime Flow: Target Repair Lane

Repair is intentionally narrow. A repair agent should not freely mutate the system. It either performs a whitelisted deterministic action or inserts a repair work item that still passes gates.

```mermaid
flowchart TB
  observe["Observer\nreads status and heartbeat"]
  incident["Incident packet\nkind, severity, evidence refs"]
  triage["Triager\nclassifies cause family"]
  known{"Whitelisted repair?"}
  deterministic["Deterministic repair\nrelease lease / reclaim stale"]
  repairTask["Repair WorkItem\nhigh priority"]
  human["Human gate\nunsafe or ambiguous"]
  verify["Gatekeeper\nrepair evidence check"]
  resume["Scheduler resumes normal work"]

  observe --> incident --> triage --> known
  known -- "yes" --> deterministic --> verify --> resume
  known -- "no, actionable" --> repairTask --> verify --> resume
  known -- "unsafe" --> human
```

Whitelisted examples:

- reclaim stale `running` work after lease expiry
- release orphaned lease
- insert follow-up task for missing evidence
- pause a noisy retry loop
- mark unrecoverable work as `blocked` with evidence

Not whitelisted:

- arbitrary code edits
- deleting failed work to make the queue look healthy
- bypassing review
- marking `done` without evidence

## WorkItem State Machine

This is the target state model that should replace ad hoc status mutation.

```mermaid
stateDiagram-v2
  [*] --> pending
  pending --> leased: scheduler acquires lease
  leased --> running: worker starts
  running --> running: heartbeat
  running --> verifying: artifact submitted
  verifying --> reviewing: validation pass
  verifying --> request_changes: validation fail
  reviewing --> gate_pending: review complete
  gate_pending --> done: GO
  gate_pending --> request_changes: actionable gaps
  gate_pending --> blocked: unsafe or missing input
  request_changes --> pending: retry allowed
  leased --> stale: lease expired
  running --> stale: heartbeat expired
  stale --> repair_queued: observer inserts repair
  repair_queued --> pending: repair completed
  blocked --> pending: human updates contract
  done --> [*]
```

## Current Versus Target

| Area | Current Loom | Target Loom |
| --- | --- | --- |
| Entry | CLI flags and backlog items | `GoalSpec` plus policy |
| Unit of work | Task / run | `WorkItem` inside `RunGroup` |
| State | JSON, Markdown, JSONL files | Object-centric status plus event log |
| Scheduling | Iterate selects next task | Scheduler with lease and heartbeat |
| Execution | `rdloop.py` stages | `rdloop.py` as kernel under control plane |
| Completion | Stage output plus gate text | `EvidencePacket` plus gatekeeper transition |
| Repair | Supervisor scripts and reflection | Observer, triager, repairer, repair work items |
| Parallelism | Limited and mostly ad hoc | `cluster` strategies such as Agentic MapReduce |
| Model mix | Role carrier defaults | Role and risk based `model_policy` |

## Reading Order

For a first pass:

1. `README.md`
2. `docs/autonomous-agent-team.md`
3. `docs/architecture/loom-architecture.md` for Chinese or `docs/architecture/loom-architecture.en.md` for English

For implementation planning:

1. `docs/loom-stable-agent-runtime-blueprint.md`
2. `docs/loom-control-plane-evolution.md`
3. `docs/pending-decisions/2026-06-29-loom-agent-cluster-platform-target.md`
4. `docs/pending-decisions/2026-07-01-parallel-agent-team-iteration.md`
