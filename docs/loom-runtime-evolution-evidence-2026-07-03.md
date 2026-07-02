# Loom Runtime Evolution Evidence

## Status

Draft analysis for product and architecture discussion.

## Date

2026-07-03

## Purpose

This document is the evidence companion to
`docs/loom-stable-agent-runtime-blueprint.md`.

The blueprint says what Loom should become. This file answers a harder
question: which parts are actually worth building, what real systems already
prove the demand, and where Loom should keep a strict boundary instead of
growing into a generic workflow platform.

## Current Repo Reality

As of 2026-07-03, the local repo already shows that Loom is no longer just a
prompt wrapper:

- `devkit/rdloop.py` is the execution kernel.
- `python3 -m devkit iterate` and `./loom autopilot` already form a local
  autonomous loop.
- `devkit/agent_observability.py`, `devkit/dashboard.py`, and
  `scripts/loom-task-queue-status.py` expose runtime state.
- `devkit/task_contract.py`, `devkit/applylock.py`, and delivery-mode routing
  show Loom has already started separating "produce output" from "apply side
  effects".
- `devkit/model_aliases.py`, `devkit/carrier_fallback.py`, and LiteLLM routing
  show that provider instability is already a first-order systems problem, not
  an edge case.

This matters because the blueprint is not speculative from zero. It is a
cleanup and hardening of pressures the repo already contains.

## Executive Judgment

Loom should continue, but with a narrower and stricter product definition:

- Loom should be a **local stable agent runtime**.
- It should **not** try to become a general hosted SaaS agent platform first.
- Its moat is **durable local control**, **evidence-backed completion**, and
  **bounded multi-agent execution for code and repo work**.
- The right analogy is not "another chat UI" but "a local controller runtime
  for long-running agent work", borrowing ideas from Kubernetes, Temporal,
  Prefect, Dagster, and modern agent SDKs.

The strongest evidence is that real systems in adjacent categories have already
converged on the same primitives:

1. desired state + reconciliation
2. explicit run/task states
3. lease / heartbeat / retry
4. observability as a product feature
5. guarded side effects
6. artifact or evidence lineage
7. bounded parallelism with synthesis
8. model routing and fallback as runtime policy

Loom is valuable if it assembles these primitives for local autonomous coding
and repo evolution. It becomes low-value if it stays as a fragile chain of role
prompts plus backlog rewrites.

## External Evidence By Blueprint Topic

## 1. Declarative Goal Objects Are Necessary

Blueprint anchor:

- `GoalSpec`
- declarative entry
- `single-agent` / `agent-team` / `cluster`

External evidence:

- Kubernetes controllers operate against desired state and continuously push the
  system toward it, then report current state back for other loops to observe:
  https://kubernetes.io/docs/concepts/architecture/controller/
- Prefect uses deployments, work pools, queues, and explicit states rather than
  ad hoc CLI flags:
  https://docs.prefect.io/v3/concepts/work-pools
  https://docs.prefect.io/v3/concepts/states

Why it matters for Loom:

- Today users still think in internal flags such as carrier, cascade, iterate,
  apply, and reflect-carrier.
- That is the same failure mode early infrastructure tools had before they
  stabilized on declarative objects.
- If Loom stays flag-shaped, every new runtime feature multiplies user-facing
  complexity and breaks automation.

Product conclusion:

- `GoalSpec` is not optional cleanup. It is the API boundary that turns Loom
  from a scripting harness into a runtime product.
- The first public abstraction should be "goal + policy", not "which carrier
  runs which stage".

## 2. Control Plane And Execution Plane Must Be Separated

Blueprint anchor:

- entry, control, execution, observability, evidence, ledger, cockpit planes

External evidence:

- Kubernetes separates controllers from workloads. Controllers reconcile and
  write status; workloads do not authoritatively declare themselves healthy:
  https://kubernetes.io/docs/concepts/architecture/controller/
- OpenAI Agents SDK separates the agent run, guardrails, tracing, hooks, and
  approvals rather than treating the LLM output as the whole system:
  https://openai.github.io/openai-agents-python/tracing/
  https://developers.openai.com/api/docs/guides/agents/guardrails-approvals

Why it matters for Loom:

- Loom's historical failures came from execution agents doing too much:
  planning, mutating files, deciding completion, and indirectly mutating queue
  state.
- Once one agent can both produce artifacts and declare them complete, runtime
  truth collapses into persuasive text.

Product conclusion:

- The blueprint's plane split is correct.
- `rdloop.py` should remain the execution kernel, but gate, state, policy, and
  runtime truth should move outward into controllers and a state writer.

## 3. A Single Writer For State Is Worth Building

Blueprint anchor:

- ledger plane
- `state_writer`
- explicit state transitions

External evidence:

- Kubernetes object status is not a free-for-all; controllers reconcile through
  a stable API object model and publish current state back for other loops.
- Prefect emphasizes state history as a first-class runtime object:
  https://docs.prefect.io/v3/concepts/states

Why it matters for Loom:

- Loom has already experienced backlog overwrite, stale snapshot, and recovery
  confusion.
- Those are not "agent quality" issues. They are multi-writer state problems.

Product conclusion:

- A `state_writer` or equivalent serialized write path is one of the highest
  leverage engineering investments in the whole blueprint.
- Without it, stronger models will still produce weak runtime behavior because
  the control surface stays non-deterministic.

## 4. Lease, Heartbeat, And Stale Reclaim Are Core, Not Advanced

Blueprint anchor:

- lease controller
- heartbeat
- stale reclaim
- work item state machine

External evidence:

- Temporal documents Activity Heartbeats as the mechanism by which a worker
  proves progress and crash absence. It also supports carrying progress payloads
  forward into retries:
  https://docs.temporal.io/encyclopedia/detecting-activity-failures
- Temporal recommends heartbeat plus timeout for long-running activities so
  failed workers can be recovered in time.

Why it matters for Loom:

- Local agent loops run for long periods, depend on fragile providers, and may
  be killed by process exits, model errors, or operator restarts.
- Without lease and heartbeat, Loom cannot distinguish "slow but alive" from
  "dead and needs reclaim".
- This directly caused stale `running` records and operator uncertainty.

Product conclusion:

- Lease + heartbeat is not a future cluster-only feature. It is the minimum
  stability layer for local autonomy.
- If Loom only ships one major runtime primitive next, it should be this one
  plus deterministic reclaim.

## 5. Observability Must Become A Resident Role, Not Loose Scripts

Blueprint anchor:

- observer
- triager
- repairer
- governor
- cockpit plane

External evidence:

- OpenAI Agents SDK ships tracing by default and records generations, tool
  calls, handoffs, guardrails, and custom events:
  https://openai.github.io/openai-agents-python/tracing/
- Prefect work pools and queues expose queue priority and concurrency as visible
  control knobs, not hidden internal mechanics:
  https://docs.prefect.io/v3/concepts/work-pools

Why it matters for Loom:

- Right now Loom can explain some runtime state, but it still largely observes
  itself through logs and file scans.
- That is enough for manual debugging but not enough for self-repair or
  trustworthy background autonomy.
- A resident observer role is the bridge from passive logs to active runtime
  diagnosis.

Product conclusion:

- Building observer/triage/governor is justified.
- The first version should stay simple: detect stale work, repeated failure
  codes, queue starvation, provider degradation, and missing evidence.
- Do not start with a "smart AI operator". Start with deterministic snapshots
  plus narrow classification.

## 6. Evidence Packets And Source Classification Are A Real Product Need

Blueprint anchor:

- `EvidencePacket`
- `inner_sandbox` / `materialized_repo` / `unknown`
- validator + reviewer + gatekeeper

External evidence:

- Dagster distinguishes asset materializations from asset observations and
  explicitly tracks whether a persisted result actually exists:
  https://docs.dagster.io/guides/build/assets
- OpenAI's guardrails and human review docs explicitly separate validation from
  approval for sensitive actions:
  https://developers.openai.com/api/docs/guides/agents/guardrails-approvals

Why it matters for Loom:

- A major class of failures in Loom was "artifact looked fine in the inner
  sandbox but never materialized into repo truth".
- That is exactly the kind of ambiguity mature pipeline systems refuse to blur.
- If Loom does not classify evidence source, it will keep reporting plausible
  but non-materialized work as success.

Product conclusion:

- The blueprint is directionally correct and should be implemented.
- `EvidencePacket` is not extra ceremony; it is the contract that prevents fake
  completeness.
- "unknown evidence source" must remain a valid and visible state, never coerced
  into success.

## 7. Guarded Side Effects Are More Important Than Smarter Generation

Blueprint anchor:

- repair whitelist
- apply policies
- report-only vs autonomous vs apply-required vs apply-git
- gatekeeper rules

External evidence:

- OpenAI's guide treats edits, shell commands, and sensitive tool actions as
  cases where runs should pause for approval:
  https://developers.openai.com/api/docs/guides/agents/guardrails-approvals
- Argo Workflows distinguishes retries and controlled workflow execution from
  unrestricted arbitrary mutation:
  https://argo-workflows.readthedocs.io/en/latest/retries/

Why it matters for Loom:

- The repo already moved in this direction with delivery modes and applylock.
- That move is correct. It turns side effects into policy, not accidental
  consequences of whichever model happened to be used.

Product conclusion:

- `report-only` should remain a first-class mode, but not the mandatory default
  for all work.
- The runtime should make side-effect policy explicit on each task.
- This is one of Loom's strongest product differentiators versus pure chat
  coding agents.

## 8. Agentic MapReduce Is Valuable, But Only For Specific Classes Of Work

Blueprint anchor:

- `cluster` mode
- `strategy: agentic-mapreduce`
- deterministic selector
- bounded shards
- reducer synthesis

External evidence:

- Argo Workflows is explicitly built to orchestrate parallel jobs and DAGs:
  https://argoproj.github.io/workflows/
- LangGraph persistence and durability modes show why broader graph execution
  needs explicit durability tradeoffs and bounded state:
  https://docs.langchain.com/oss/python/langgraph/persistence
  https://docs.langchain.com/oss/javascript/langgraph/checkpointers

Why it matters for Loom:

- Repo-wide audits, backlog scans, failure mining, and doc consistency checks do
  not fit well in a single context window.
- But most coding tasks still do not need cluster mode.
- If Loom makes fanout the default, it will pay a complexity tax too early.

Product conclusion:

- Keep `cluster` as a composition mode, not the standard path.
- The blueprint is right to frame Agentic MapReduce as a strategy, not the
  runtime itself.
- The first MapReduce workloads should stay read-only: backlog audit, run
  artifact audit, and repo/doc consistency scans.

## 9. Model Policy Needs To Move Up Into Product Surface

Blueprint anchor:

- model policy
- role-based routing
- fallback reason
- degraded mode trace

External evidence:

- LiteLLM documents retries and fallbacks as explicit reliability features:
  https://docs.litellm.ai/docs/proxy/reliability
  https://docs.litellm.ai/docs/completion/reliable_completions
- OpenAI Agents SDK tracing shows that runtime visibility into model/tool
  behavior is not optional if you want to debug production agent workflows:
  https://openai.github.io/openai-agents-python/tracing/

Why it matters for Loom:

- Recent Loom failures were strongly shaped by model/vendor behavior:
  capacity errors, empty bodies, format drift, and provider-specific response
  quirks.
- Treating model selection as low-level flags leaves the user holding runtime
  complexity that the platform should absorb.

Product conclusion:

- The user should choose policies such as `resilient-local`, `no-gpt`, or
  `cost-first`, while Loom performs carrier routing and records the actual path.
- Model routing trace is a product requirement, not a debug convenience.
- This is especially important once multiple providers are used in the same
  Team.

## 10. Reviewer Freshness And Trust Gates Are Strongly Justified

Blueprint anchor:

- `reviewer_fresh`
- validator then reviewer then gatekeeper
- implementer cannot self-declare completion

External evidence:

- OpenAI's approval model separates automatic checks from approval decisions:
  https://developers.openai.com/api/docs/guides/agents/guardrails-approvals
- Dagster's asset and observation model exists because persisted truth must be
  distinguished from mere execution attempts:
  https://docs.dagster.io/guides/build/assets

Why it matters for Loom:

- Coding agents are especially vulnerable to self-confirmation: the same model
  writes, explains, and approves its own output.
- A fresh reviewer is not perfect, but it is a meaningful reduction in
  correlated error, especially when paired with deterministic verification.

Product conclusion:

- Keep the review gate.
- Over time, the product should let users tune how much freshness they pay for:
  lightweight review for low-risk report-only work, stronger review for repo
  mutation, policy changes, and repair actions.

## 11. The Blueprint's "Local First, Not Kubernetes First" Boundary Is Correct

Blueprint anchor:

- not literal Kubernetes deployment
- local stable control system
- no multi-tenant SaaS in current scope

External evidence:

- Kubernetes provides the right mental model for controllers and reconciliation,
  but it also brings heavy operational cost and assumptions that do not match
  single-user local development workflows.
- LangGraph and agent SDKs prove there is demand for durable agent execution,
  but they do not automatically solve local long-running operator behavior.

Why it matters for Loom:

- Loom's comparative advantage is that it can sit beside a local repo, a local
  toolchain, and a user's actual coding workflow.
- Jumping too early to hosted orchestration, tenancy, billing, and SSO would
  bury the core reliability problem under platform surface area.

Product conclusion:

- Keep borrowing operational ideas from Kubernetes.
- Do not turn Loom into "Kubernetes for agents" as a literal product promise.
- The right message is "stable local runtime with controller semantics".

## 12. What The Real Market Signal Actually Is

The evidence does not say "the world needs yet another generic multi-agent
framework."

It says:

- teams need agent runs that survive failure and can be resumed
- agent work needs auditable state and visible traces
- side effects need policy and approval lanes
- model routing and vendor degradation need to be absorbed by infrastructure
- broad repo or queue analysis needs bounded fanout and synthesis

That combination is still underserved for local coding and autonomous repo
maintenance. Most products solve only one layer:

- agent SDKs solve prompts, tools, and traces
- workflow systems solve retries, queues, and states
- data systems solve materialization and lineage
- gateways solve model routing

Loom is worth continuing only if it composes those into one local runtime for
developer-facing agent autonomy.

## Recommended Product Boundary For Loom

Loom should own:

- goal submission and policy normalization
- local scheduling and runtime state
- delivery mode and side-effect policy
- lease, heartbeat, reclaim, retry, and repair insertion
- evidence packets, verification, and review gates
- model routing policy and degradation trace
- bounded fanout for repo and backlog audits
- cockpit-grade observability for one user's local runtime

Loom should not own yet:

- hosted multi-tenant control plane
- enterprise identity, billing, marketplace, or broad team admin
- arbitrary freeform repair agents that mutate anything
- code hosting, PR management, or general CI replacement
- full distributed cluster execution beyond local bounded worker groups

## Recommended Build Order

If the goal is to maximize real product value rather than architecture purity,
the order should be:

1. `GoalSpec` + control policy object
2. serialized state writing and transition rules
3. lease + heartbeat + stale reclaim
4. evidence packet + source classification + gate transition rules
5. observer snapshot + incident schema + repair insertion
6. model policy presets + degraded-mode trace
7. cockpit views based on the now-stable objects
8. read-only Agentic MapReduce for Loom self-audit

This order matches the actual failure stack in the repo: state ambiguity first,
trust ambiguity second, orchestration scale third.

## Concrete Questions For Loom's Next Discussion

1. Is Loom's primary object a `GoalSpec`, a backlog item, or a run request?
2. Which states are authoritative enough to persist in one place now?
3. What is the smallest acceptable heartbeat contract for long-running local
   work?
4. Which repair actions are deterministic enough to whitelist in v1?
5. What evidence is required before a repo mutation can be called complete?
6. Which three cockpit views are mandatory before adding more runtime features?
7. Which workloads justify `cluster` immediately, and which should stay in
   `agent-team`?

## Source Links

- Kubernetes Controllers:
  https://kubernetes.io/docs/concepts/architecture/controller/
- Temporal Activity Heartbeats:
  https://docs.temporal.io/encyclopedia/detecting-activity-failures
- LangGraph Persistence:
  https://docs.langchain.com/oss/python/langgraph/persistence
- LangGraph Durability Modes:
  https://docs.langchain.com/oss/javascript/langgraph/checkpointers
- Dagster Assets:
  https://docs.dagster.io/guides/build/assets
- Prefect Work Pools:
  https://docs.prefect.io/v3/concepts/work-pools
- Prefect States:
  https://docs.prefect.io/v3/concepts/states
- LiteLLM Reliability:
  https://docs.litellm.ai/docs/proxy/reliability
  https://docs.litellm.ai/docs/completion/reliable_completions
- OpenAI Agents SDK Tracing:
  https://openai.github.io/openai-agents-python/tracing/
- OpenAI Guardrails And Human Review:
  https://developers.openai.com/api/docs/guides/agents/guardrails-approvals
- Argo Workflows:
  https://argoproj.github.io/workflows/
  https://argo-workflows.readthedocs.io/en/latest/retries/
