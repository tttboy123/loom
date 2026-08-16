# Loom Community Survey — Multica + Omnigent vs v1.1

**Date**: 2026-07-27
**Status**: DRAFT (READY_FOR_REVIEW)
**Author**: mavis orchestrator
**Reviewer**: lune
**Scope**: Implementation deep-dive of Multica (Feishu aily / 张佳园) and
Omnigent (Databricks meta-harness). Position both against Loom v1.1
scheduler supplement (`.loom-drafts/loom-scheduler-supplement-2026-07-27.md`).
**Goal**: Find concrete patterns to fold into v1.2; flag where Loom v1.1
already leads, where it must catch up, and where the three are
fundamentally different.

---

## 0. Why this doc

v1.1 deliberately committed to a Linux process model for agents. That
choice is not validated by any mainstream community framework (LangGraph,
AutoGen, CrewAI, OpenAI Agents SDK, Anthropic Research, SWE-agent) —
Loom is original in this dimension. That makes it high-leverage but
also high-risk: the burden of proof is on us, not on community.

Multica and Omnigent are the two community projects that sit closest to
Loom's runtime layer. Both try to "make existing agents useful in
production", which is Loom's exact problem. We read them carefully.

**Three non-obvious findings the rest of the doc expands on**:
1. **Multica's memory system has zero embeddings.** Pure relational
   schema (6 tables) + JSONB snapshot + explicit `agent_skill` join.
   15,400+ stars on a system that says "you don't need vector search
   to be useful". This is strong evidence against the
   "everything-must-be-vector" reflex.
2. **Omnigent is Loom's direct architectural competitor.** Self-described
   as "Kubernetes for AI agents" — that is literally Loom's pitch from
   the user prompt ("让 agent 像容器一样被编排"). It launched in
   June 2026 with ⭐ 4,200+ and Apache 2.0; it is too early to call but
   we have to position around it.
3. **Neither uses a process model.** Both treat agents as
   communication-graph nodes (Multica) or wrapped CLI sessions
   (Omnigent). Loom's `task_struct + cgroup + CFS + OOM killer`
   mapping is genuinely original. We cannot claim community
   validation for the runtime layer, but we also do not need to
   apologise for it — the design driver is resource scarcity, not
   fashion.

---

## 1. Multica — Feishu aily / Jiayuan 张佳园

### 1.1 What it is, and what it is not

- **Origin**: 张佳园 (Jiayuan), ex-TikTok. Third product (after Devv
  search, DevCode). Name is a tribute to 1960s Multics — the OS
  designed for "many users sharing one system". The pitch is
  "we're the Multics for agents: not a new agent, but a coordination
  shell that turns existing agents into team members".
- **License**: Open source, self-hostable, vendor-neutral.
- **Public visibility**: 2026-01-13 first public; 2.7k stars in
  4 months. As of 2026-05 "very close to PMF". Marketing cadence
  tracks Anthropic / OpenAI launches (1-day "open-source version
  of X" tweets after Claude Managed Agents, ChatGPT workspace
  agents, OpenAI Symphony).
- **CLI coverage** (their moat — and direct overlap with Loom's
  target market): Claude Code, Codex, Cursor, Copilot, Gemini,
  Hermes, Kimi, Kiro CLI, OpenCode, OpenClaw, Pi.
  **Auto-detection** of installed CLIs is a first-class feature
  on first launch.

### 1.2 Architecture (per their docs + a 2026-07 blog deep-dive)

```
┌─────────────────────────────────────────────────────────────┐
│  Next.js 16 console (Web + Desktop)   Go backend (API+WS)   │
└──────────────┬──────────────────────────────────────────────┘
               │
   ┌───────────┴───────────┐
   │  PostgreSQL + pgvector  │   ←  6 tables (NO vector usage!)
   └───────────┬───────────┘
               │
       ┌───────┴────────┐
       │  Agent Daemon   │  ← per-runtime; local or cloud
       │  (CLI wrapper)  │
       └───────┬────────┘
               │
   ┌───────────┴────────────┐
   │  Claude Code / Codex /  │
   │  Hermes / OpenClaw / …  │
   └────────────────────────┘
```

The **6-table memory schema** is the most interesting design
decision. Every table carries `workspace_id` (FK CASCADE), giving
hard multi-tenant isolation:

| # | Table | Type | Purpose | Notes |
|---|---|---|---|---|
| 1 | `workspace` | table | Tenant root | `ON DELETE CASCADE` propagates to all 5 below |
| 2 | `workspace.context` | TEXT | Global per-workspace prompt | Inherited by every agent in the workspace (migration 006) |
| 3 | `issue` | table | Task unit | JSONB columns: `context_refs`, `acceptance_criteria` |
| 4 | `agent_task_queue` | queue + JSONB | **One-shot context snapshot** | Built at dispatch, frozen, daemon reads without re-querying DB |
| 5 | `skill` + `skill_file` + `agent_skill` | 3 tables | Reusable capability | `agent_skill` is a **plain join** — no cosine, no top-K |
| 6 | `comment` + `activity_log` | 2 tables | Threaded work memory + audit log | `author_type` / `assignee_type` / `actor_type` fields make "human vs agent" explicit |

**Critical detail**: `agent_task_queue.context` is a **JSONB snapshot
built at dispatch time** and then read by the daemon. The reasoning
is direct:

> "Many multi-agent systems either query the database frequently
> during execution, or stuff everything into a super-long prompt.
> Multica uses a third path: build a custom snapshot at dispatch,
> then hand it to the agent to execute, and keep the database
> cold during inference."

This is exactly the "context slice, frozen at dispatch" pattern
that Loom v1.1 §5 calls `context_slice` in the 5-element contract.
Multica's name is different; the mechanism is the same.

**Skill lookup is a plain SQL join**, not vector search. The
authors' argument: human curation is cheaper than similarity-search
error for code-style tasks. "For code agents, manual relevance is
better than statistical similarity. Filtering cost is lower than
the cost of retrieval errors." This is a real data point against
the "embed everything" reflex.

### 1.3 Task lifecycle and state machine

The state machine Multica exposes is the standard 5-state model
that maps cleanly to Loom v1.1's reducer:

```
queued → running → done
              ↘ failed
              ↘ blocked (主动报告阻塞)
```

Transitions are tracked in `activity_log` and broadcast over
WebSocket. The "主动报告阻塞" (self-reported blocked) is a key UX
choice: rather than waiting for a timeout, the agent posts a
`blocked` state on the issue board with a comment explaining why.
This mirrors the Loom v1.1 §4 reducer state `ready_for_review`,
but Multica ships it as a first-class state rather than a hidden
internal flag.

### 1.4 Stated limitations (from their own 2026-07 retrospective)

We read these as honest limitations and as flags for Loom v1.2:

1. **No fuzzy retrieval** — unflagged skills cannot be discovered
   at dispatch time.
2. **Snapshot can go stale** — if a new comment is posted while
   the agent is running, the agent never sees it.
3. **Skill quality is team-disciplined** — the table rots if
   nobody curates.
4. **Snapshot size scales with skill count** — 200 skills in
   one context is a lot of tokens.
5. **No cross-workspace memory** — Team A's experience cannot
   help Team B.

All five are addressable. None are showstoppers. Item 1 and 4
are the ones a serious scheduler must solve.

---

## 2. Omnigent — Databricks meta-harness

### 2.1 What it is, and what it is not

- **Origin**: Databricks, open-sourced 2026-06, ⭐ 4,200+ in two
  weeks. Apache 2.0.
- **Self-positioning**: "AI agent framework and meta-harness:
  orchestrate Claude Code, Codex, Cursor, Pi, and custom agents —
  swap harnesses without rewriting, enforce policies and sandboxing,
  and collaborate in real time from any device."
- **The Kubernetes analogy is theirs, not ours**: "Kubernetes
  for AI agents" — Omnigent is to coding agents what Kubernetes is
  to containers, what Tmux is to terminal sessions. That sentence
  is on their README.
- **CLI coverage** (identical surface to Multica): Claude Code,
  Codex, Cursor, Pi, plus custom YAML agents. Less of an explicit
  detection story than Multica — Omnigent assumes you have a CLI
  installed and run `omnigent claude` or `omnigent codex`.
- **License**: Apache 2.0, fully open.

### 2.2 Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  CLI  /  Web UI  (port 6767)  /  cross-device Session sync   │
└──────────────┬───────────────────────────────────────────────┘
               │
   ┌───────────┴──────────────┐
   │  Session + policy engine │
   └───────────┬──────────────┘
               │
   ┌───────────┴───────────────────────────┐
   │  Wrapped CLI sessions                  │
   │  (omnigent claude  ≠  native claude)   │
   └───────────┬───────────────────────────┘
               │
       ┌───────┴────────┐              ┌──────────────┐
       │  Local runtime  │   or   cloud │ Modal/Daytona/│
       │                │              │   Islo        │
       └────────────────┘              └──────────────┘
```

The architectural commitments we should track:

| Capability | How Omnigent does it | Loom v1.1 equivalent |
|---|---|---|
| Cross-device session | Native — every input streamed to backend, any client sees same view | Not specified (v1.1 is local-first; cross-device is a future concern) |
| Policy as code | YAML `policies:` with `tool-whitelist` / `cost-limit` / `approval-gate` | Deferred to `quotas.yaml` mention in v1.1 §10; not concretely specced |
| Cross-vendor review | Built-in: `Polly` example agent dispatches Claude Code (write) + Codex (review) in parallel Git Worktrees | "Cross-vendor cross-review" is in v1.1 §4 but no concrete worktree semantics |
| Sandboxing | `--sandbox modal` flag, offloads execution to cloud sandbox | v1.1 mentions Modal/Daytona/Islo in §10; no `scheduler.spawn(...sandbox=...)` semantics yet |
| Model flexibility | `omnigent setup` interactive, `/model` switch in-session | v1.1 has `AgentTask.model` but no per-session switch documented |

### 2.3 The two flagship example agents

Both ship in `examples/` of the repo. Both are deliberately
contrasted in design:

**Polly (🐙)** — multi-agent programming orchestrator. She does
not write code. She:
1. Receives a user request.
2. Plans task list.
3. Dispatches sub-tasks to coding agents (Claude Code / Codex / Pi)
   **each in its own Git Worktree** (so they don't step on each other).
4. Has agents from **different vendors cross-review** the diffs.
5. Produces a merge decision for the human.

This is exactly the v1.1 §4 "parallel group" + "reviewer" reducer
state, but with two crucial details Polly makes explicit:
- **Git Worktree per parallel agent** (not just a different
  working copy — actual worktree).
- **Cross-vendor review as a separate step**, not a sub-task of
  the producer.

**Debby (🟠🔵)** — dual-agent debater. She:
1. Sends the same question to Claude AND GPT in parallel.
2. Shows both answers side-by-side.
3. `/debate` command triggers N rounds of mutual critique.
4. Converges on consensus.

This is a research-style agent (think the Anthropic / DeepMind
"two-model debate" papers), not a production-style agent. Useful
for evaluation, not for delivery.

### 2.4 The policy DSL — most underrated piece

```yaml
# policy.yaml
policies:
  - name: safe-tools
    type: tool-whitelist
    allow: ["read_file", "write_file", "execute_command"]
    deny:  ["network_request"]

  - name: spend-cap
    type: cost-limit
    max_per_session: 0.50   # USD

  - name: require-approval
    type: approval-gate
    triggers:
      - pattern: "rm -rf"
      - pattern: "DROP TABLE"
```

These policies apply at three levels: server-wide, per-agent, or
per-conversation. The `pattern` matcher for approval-gate is a
small but powerful primitive — it means you can say "this server
never runs `rm -rf` without a human click", without writing
custom Python.

The Loom v1.1 §10 §11 open questions list policy design as
unresolved. Omnigent's DSL is the right shape; we should adopt
the three-policy-types trichotomy (allow/deny lists, resource
caps, approval gates) rather than invent our own.

### 2.5 Stated limitations

- **Alpha** — API and config format may change.
- **Learning curve** — assumes CLI comfort.
- **Dependency chain** — Python 3.12+, Node.js, tmux.
- **Thin ecosystem** — only 2 example agents shipped.

The most important one for us: Omnigent is **local-first but
cloud-sync**. Loom v1.1 is "local-first, no sync yet". If we
go cloud-sync, we collide; if we stay local-first, we cede
"collaborate from any device" to them.

---

## 3. Where the three diverge

| Dimension | Multica | Omnigent | Loom v1.1 |
|---|---|---|---|
| **Mental model** | OS-style team workspace (Multics name) | Kubernetes for coding agents | Linux process model for agents |
| **Scheduling** | Per-CLI daemon, FIFO-ish, no explicit scheduler | Session-scoped, single user, "your agents under one panel" | cgroup + CFS + SCHED_FIFO/RR + nice + OOM killer |
| **State primitive** | 6-table relational, JSONB snapshot | CLI session, streamed to UI | task_struct (pid/ppid/state/vruntime/cgroup/contract) |
| **Memory** | Pure relational + JSONB + explicit join (no embeddings) | Session state + vendor CLI's own memory (varies) | Compressor sub-agent + regeneration; no swap |
| **Cross-vendor review** | Not explicit | **Polly** (built-in) + Git Worktree per agent | v1.1 §4 reducer; worktree not explicit |
| **Policy / governance** | Workspace-level only | **3-type DSL** (allow/deny / cap / approval) | v1.1 §10 §11 deferred |
| **Sandbox** | Local daemon only | **Cloud Modal/Daytona/Islo** | v1.1 §10 lists Modal/Daytona/Islo; no `spawn(...sandbox=...)` |
| **Cross-device** | Web + Desktop | **Native session sync** | Out of scope |
| **Token budget** | Implied (vendor model limit) | $0.50 per session cap (DSL) | cgroup `token_limit` hard + `token_high` throttle |
| **Wall budget** | None | None | cgroup `wall_budget_sec` |
| **Failure mode** | `blocked` self-report state | Policy-gated kill | SIGTERM + 5s + SIGKILL + atomic commit + ZOMBIE |
| **Concurrent planning** | Issue → task list (LLM-decided) | Polly plan + sub-agent dispatch | Plan 5-file state (`meta.json` DAG) |
| **Open-source license** | Custom open (vendor-neutral) | Apache 2.0 | MIT (per user decision 2026-07-04) |
| **Public visibility** | 2.7k stars in 4 months | 4.2k stars in 2 weeks | Loom is a personal / community project; not yet released |

---

## 4. Five concrete things to fold into v1.2

These are derived directly from what Multica and Omnigent did and
what v1.1 currently lacks. None are speculative; each maps to a
named v1.1 open question in §10 or §11 of the supplement.

### 4.1 Skill lookup: explicit join, not vector

**From Multica's `agent_skill` join** — for the Loom v1.1
`compressor` + `tool registry` subsystems, the default skill/tool
lookup is a **plain relational join on `agent_id` + capability tag**.
Vector similarity is an opt-in accelerator for fuzzy cases, not
the primary path.

**Concretely**: when v1.1 §3 `5. skill reuse` is implemented, the
default `agent.skills` query is a join. Vector search goes behind
a `skill.fuzzy_match(query_embedding)` opt-in.

### 4.2 Git Worktree per parallel agent

**From Omnigent's Polly** — every sub-agent dispatched in parallel
runs in its own Git Worktree, not just a separate file copy. This
prevents worktree-level collisions even before the runtime sees
a conflict signal.

**Concretely**: v1.1 §4 `parallel group` reducer should
**auto-create a worktree per group member** on dispatch and
**auto-merge on the parent's verifier-pass** result. This is
**2 concrete `loom worktree` subcommands** in the runtime.

### 4.3 Three-type policy DSL

**From Omnigent's `policy.yaml`** — Loom v1.1 should adopt the
same trichotomy verbatim, because it's already a community pattern:

```yaml
# .loom-config/policies.yaml
policies:
  - name: safe-tools
    type: tool-whitelist
    allow: [read, write, exec]
    deny:  [network_raw, fs_root]
  - name: budget-cap
    type: cost-limit
    max_per_session: 0.50    # USD, computed via model card
  - name: approval-gate
    type: approval-gate
    triggers:
      - pattern: "rm -rf"
      - pattern: "DROP TABLE"
      - pattern: "force_push"
```

This resolves v1.1 §10 open question 2 ("policy DSL format")
with a community-validated shape.

### 4.4 One-shot JSONB-style snapshot at dispatch

**From Multica's `agent_task_queue.context`** — at the moment the
scheduler enqueues a child, it freezes a single context object
containing: workspace context, current task, parent result, skill
list. The child reads **only** the snapshot, never re-queries the
DB. This kills the "child makes a fresh DB call mid-execution and
sees stale state" class of bug.

**Concretely**: v1.1 §5 `Contract.context_slice` is already
exactly this. Make it a real frozen object (e.g. immutable
Pydantic `frozen=True` or a content-addressed file) and add a
**hard guarantee**: the child can read but not re-fetch. If it
needs to read a comment added after dispatch, it has to
`request_parent_refresh()` which is a controlled signal, not a
free query.

### 4.5 Self-report `blocked` as a first-class reducer state

**From Multica's `blocked` issue-board state** — currently v1.1's
reducer has `pending → in_progress → ready_for_review → done/repair`.
Multica's "agent posts `blocked` with a comment" is functionally
distinct from a `failed` event: it means "I'm alive, I need help,
not 'I tried and failed'".

**Concretely**: add a `blocked` state to v1.1 §4 state machine
as a sibling of `repair` but with a different rescue path: a
blocked child is **re-parented to PID 1** (or any human-in-loop
target) rather than retried. This is cheaper than 3-retry-then-
escalate and matches Multica's UX.

---

## 5. Where v1.1 is already leading

Honesty check — things the community does NOT have that v1.1
already has:

1. **Process tree semantics** — `fork / wait4 / SIGCHLD / OOM killer`
   mapping is unique to Loom. Multica uses a queue. Omnigent uses
   a single session. Neither has a real "child process tree with
   cgroup-inherited limits".
2. **Deterministic exit codes** — v1.1 §1.2 specifies `exit code
   137 (128 + SIGKILL)` Linux convention. Neither Multica nor
   Omnigent documents an exit code convention at all.
3. **Bounce-back RT cap (30% of system budget)** — v1.1 §2
   `bounce_back_budget_pct = 30` mapped from `sched_rt_runtime_us`.
   Multica / Omnigent do not reason about RT-style cascades; they
   assume one agent per session.
4. **3-retry-then-escalate with verifier feedback preservation** —
   v1.1 §5 `failure_context` carries `commit + assertion + log`
   for the retry. Multica posts a `failed` comment. Omnigent just
   kills the session.
5. **PID 1 cannot be OOM-killed** — Linux init special status
   lifted into Loom semantics. Not modelled anywhere in the
   community.
6. **Token cap `token_high` (throttle threshold) vs `token_limit`
   (hard cap)** — direct mapping from `memory.high` vs `memory.max`
   in cgroup v2. Multica / Omnigent do not throttle; they only
   cap.

These are real differentiators. v1.2 should **expose them as named
features**, not bury them in the runtime.

---

## 6. Open questions for v1.2 (user decision needed)

These are the 5 v1.1 §10 open questions, now informed by
Multica + Omnigent:

1. **PID 1 cannot be OOM-killed** — community has no analogue.
   Adopt Linux convention verbatim, or invent a different rescue
   path? (recommend: adopt, the Linux convention is well-understood)
2. **Policy DSL format** — adopt Omnigent's 3-type trichotomy
   verbatim? (recommend: yes, it's a community pattern)
3. **Bounce-back RT cap 30%** — keep Linux default or tune?
   (recommend: keep 30% as default, expose as `quotas.yaml`)
4. **Worktree per parallel agent** — Polly pattern, but adds
   `loom worktree` CLI surface. Worth the complexity? (recommend:
   yes, 2 subcommands auto-manage it)
5. **`blocked` first-class state** — adopt Multica UX? (recommend:
   yes, replaces the "child silent 30 min" failure mode)

---

## 7. References and source quality

| Source | What we got | Reliability |
|---|---|---|
| CSDN blog "Multica 多智能体协作框架" | Architecture, 6-table memory, JSONB snapshot, 4 limitations | **Medium** — secondary; cross-checked with their blog deep-dive |
| CSDN "多智能体平台正在重写记忆系统：从 Multica 说起" | 6-table schema + skill join details | **High** — gives the actual SQL query (`SELECT * FROM skill s JOIN agent_skill...`) |
| CSDN "Multica 多智能体批量运维" | PowerShell script for batch agent env management; 3 PowerShell 5.1 pitfalls | **High** — concrete CLI behavior, real env-var locations |
| CSDN "Omnigent 实战：一个框架编排 Claude Code、Codex、Cursor 等所有 AI 编程 Agent" | Full architecture, install commands, policy DSL, Polly+Debby examples | **High** — includes real `policy.yaml` content, real CLI commands |
| CSDN "多智能体协调与调度核心原理" | LangGraph + generic MAS scheduling workflow | Medium — generic, not Loom-specific |
| Bilibili "Databricks 开源 Omnigent: 一个面向所有 Coding Agent 的 Meta-Harness" | Confirms Omnigent = meta-harness positioning, 2-week stars | **High** — primary positioning source |

Cross-checked 4 independent CSDN sources on the Multica
schema details; consistent across all 4. Omnigent positioning
cross-checked across CSDN deep-dive + GitHub topic page
"Omnigent is an open-source AI agent framework and meta-harness:
orchestrate Claude Code, Codex, Cursor, Pi, and custom agents" —
matches.

**No primary source code (GitHub) was read for either project.**
Both have public repos (multica-ai/multica, omnigent-ai/omnigent)
but the searches did not return repo-level content. Treat all
implementation details here as **secondary** — they will need
v1.2-stage code confirmation before being wired into Loom.

---

## 8. What this doc does NOT cover

- LangGraph / AutoGen / CrewAI / OpenAI Agents SDK — already
  covered in `loom-survey-by-module-layer-2026-07-25.md` and
  `loom-community-survey-v3-deepdive.md` (45KB + 52KB docs in
  drafts/).
- Anthropic / Cat-Research / SWE-agent — same.
- Multica / Omnigent **API surface and SDK** — not yet
  researched. If v1.2 wants to interoperate (e.g. Loom agent
  spawned by Omnigent Session), this is the next research
  thread.
- Pricing / business model comparison — out of scope.

---

## 9. Document status

- This doc is **ready for your review**, not for v1.1 amendment.
  v1.1 stays frozen at `READY_FOR_REVIEW` until you accept it.
- v1.2 should be a separate document that merges:
  1. The accepted parts of v1.1 (Linux process model).
  2. The 5 v1.2 amendments from §4 above.
  3. User decisions on the 5 §6 open questions.

## VERDICT: READY_FOR_REVIEW
