# Loom Agent Process Model & Scheduler Supplement v1.1

**Date**: 2026-07-27
**Status**: DRAFT (READY_FOR_REVIEW)
**Supplements**: `loom-graph-engineering-plan.md` (v1, 2026-07-25)
**Author**: mavis orchestrator
**Reviewer**: lune

---

## 0. Why this doc

v1 defined **what** to build (DAG, state machine, 5-file state, 4-level checkpoint).
This supplement defines **how the runtime schedules and bounds** that work.

The design follows the **Linux process model**:
- Agent = process
- cgroup = resource limits per role/priority
- CFS scheduler = default queue
- SCHED_FIFO/RR + nice = bounce-back priority
- Context window = memory (handled via **compression + regeneration**, not swap)
- OOM killer = token budget exhaustion

**Resource scarcity is the design driver** (same as Linux on PDP-11), not a reason to
diverge. v1 left the runtime layer implicit; this doc makes it explicit and code-ready.

---

## 1. Process model

### 1.1 Agent = process

```python
class AgentTask:  # mirrors Linux task_struct
    pid: int                       # unique within session
    ppid: int | None               # parent
    role: str                      # dev / test / verifier / summary / compressor
    state: State                   # pending / running / blocked / zombie / done
    sched_policy: str              # fifo / rr / normal / batch / idle
    nice: int                      # -20 to 19, like Linux nice
    vruntime: float                # CFS virtual runtime
    cgroup: AgentCgroup            # resource limits (see §2)
    contract: Contract             # 5-element handoff (see §5)
    retry_count: int = 0
    max_retry: int = 3
    token_used: int = 0
    wall_used_sec: int = 0
    enqueued_at: float
    started_at: float | None = None
    finished_at: float | None = None
    output: TaskOutput | None = None
    failure_context: dict | None = None
```

### 1.2 fork / exec / wait / kill

| Linux syscall | Agent primitive | Notes |
|---|---|---|
| `fork()` | `scheduler.spawn(ppid, role, contract)` | child gets cgroup copy, capped by parent |
| `vfork()` | `scheduler.spawn_borrowed(ppid, contract)` | child shares parent's context, run-to-completion |
| `exec()` | `task.switch_role(new_role)` | same pid, swap role + contract |
| `wait4(-1, …)` | `parent.wait_any()` | block until any child done |
| `wait4(pid, …)` | `parent.wait(pid)` | block until specific child done |
| `kill -SIGTERM` | `scheduler.terminate(pid, reason)` | graceful: save + cleanup + exit |
| `kill -SIGKILL` | `scheduler.kill(pid)` | immediate: drop + notify parent |
| `sigaction(SIGCHLD)` | `parent.on_child_exit(handler)` | edge-triggered on child state→zombie |
| `_exit(code)` | `task.exit(exit_code, output)` | write deliverable, mark zombie |
| `getpid()` / `getppid()` | `task.pid` / `task.ppid` | introspection |

**PID 1 = main agent**. Owns user context, conversation, IM/cron/signal sources, the
top-level scheduler. Cannot be OOM-killed (§10 open question 5).

### 1.3 Process tree

Linux process tree (parent/child) gives:
- **Resource accounting** (children roll up to parent)
- **Signal cascade** (kill parent → kill children)
- **Cleanup** (children zombie until parent wait()s)

Agent tree gives the same, plus:
- **State propagation**: if parent goes blocked, children can either wait or be reparented to PID 1
- **Cost rollup**: a fork's tokens count against the parent's cgroup, with the parent's cap

---

## 2. cgroup — resource limits

cgroup v2 has subsystems: memory, cpu, io, pids, … Agent has the same axes:

```python
class AgentCgroup:
    # memory.* equivalent
    token_limit: int = 50_000           # hard cap (仿 memory.max)
    token_high: int = 40_000            # throttle threshold (仿 memory.high)
    context_window_max: int = 200_000   # raw LLM window (仿 vm.overcommit_memory)

    # cpu.* equivalent
    token_weight: int = 1024            # CFS share (仿 cpu.weight)
    wall_budget_sec: int = 1800         # (仿 cpu.max)

    # pids.* equivalent
    max_forks: int = 5                  # max concurrent direct children

    # io.* equivalent (for compressor I/O)
    disk_read_bw: int | None = None
    disk_write_bw: int | None = None
```

**Inheritance**: child cgroup is a copy of parent's, capped by parent's limit.
Like Linux: child can never exceed parent. If parent hits cap, child gets throttled.

**Default values** (overridable in `quotas.yaml`, see §6):

| role | token_limit | wall | weight | max_forks |
|---|---|---|---|---|
| dev | 50,000 | 1800 | 1024 | 3 |
| test | 80,000 | 1800 | 1024 | 2 |
| verifier | 30,000 | 600 | 2048 | 1 |
| summary | 10,000 | 300 | 256 | 2 |
| compressor | 8,000 | 300 | 256 | 0 |

---

## 3. Scheduler — CFS + SCHED_FIFO/RR

```python
class AgentScheduler:
    def __init__(self):
        self.rt_queue: list[int] = []     # SCHED_FIFO/RR — bounce-back, gate unlock
        self.cfs_queue: list[int] = []    # SCHED_NORMAL — default work
        self.idle_queue: list[int] = []   # SCHED_IDLE — background
        self.batch_queue: list[int] = []  # SCHED_BATCH — long-running
        self.tasks: dict[int, AgentTask] = {}
        self.token_accountant = TokenAccountant()
        self.compressor = CompressorSubAgent()  # see §4
        self.quotas = load_quotas()             # see §6

    def pick_next(self) -> int | None:
        # 1. SCHED_FIFO/RR (bounce-back, gate unlock) — always first
        if self.rt_queue:
            return self.rt_queue[0]

        # 2. SCHED_NORMAL — pick smallest vruntime (CFS)
        if self.cfs_queue:
            return min(self.cfs_queue, key=lambda p: self.tasks[p].vruntime)

        # 3. SCHED_BATCH — long-running, lower priority than NORMAL
        if self.batch_queue:
            return self.batch_queue[0]

        # 4. SCHED_IDLE — only when nothing else
        if self.idle_queue:
            return self.idle_queue[0]
        return None

    def on_bounce_back(self, pid: int, reason: str):
        """sched_setscheduler(SCHED_FIFO) + setpriority(-20)"""
        task = self.tasks[pid]

        # Demote from CFS → RT
        if pid in self.cfs_queue:
            self.cfs_queue.remove(pid)
        if pid in self.batch_queue:
            self.batch_queue.remove(pid)
        self.rt_queue.insert(0, pid)  # FIFO: head of queue

        task.sched_policy = "fifo"
        task.nice = -20
        task.retry_count += 1
        task.failure_context = reason
        task.contract = task.contract.with_failure_context(reason)
        task.cgroup.token_limit = int(
            task.cgroup.token_limit * self.quotas.bounce_back.retry_token_bonus
        )

    def check_oom(self, pid: int) -> bool:
        """仿 Linux OOM killer: throttling at .high, kill at .max"""
        task = self.tasks[pid]
        used = self.token_accountant.read(task.cgroup)

        if used > task.cgroup.token_high:
            self._throttle(pid)  # ask model to yield, persist progress

        if used > task.cgroup.token_limit:
            # Try one last compression
            freed = self.compressor.emergency_compress(pid)
            if self.token_accountant.read(task.cgroup) > task.cgroup.token_limit:
                self.oom_kill(pid)  # graceful kill (§7)
                return True
        return False
```

### 3.1 Linux scheduler parity

| Linux | Agent |
|---|---|
| `sched_setscheduler(SCHED_FIFO)` | `on_bounce_back()` promote to rt_queue |
| `setpriority(PRIO_PROCESS, pid, -20)` | `task.nice = -20` |
| `setpriority(PRIO_PROCESS, pid, 19)` | `task.nice = 19` |
| `sched_yield()` | `task.yield()` — checkpoint and reschedule |
| `cpu.weight` cgroup | `cgroup.token_weight` |
| `cpu.max` cgroup | `cgroup.wall_budget_sec` |
| `cgroup.memory.high` | `cgroup.token_high` |
| `cgroup.memory.max` | `cgroup.token_limit` |
| `cgroup.pids.max` | `cgroup.max_forks` |
| `try_to_free_pages()` | `compressor.emergency_compress()` |
| `oom_kill_process()` | `scheduler.oom_kill()` (graceful) |

### 3.2 RT bandwidth cap (the open question resolved)

Linux caps RT to 95% of CPU by default (`sched_rt_runtime_us`). LLM bounce-back
cascades more aggressively than Linux RT, so we set **30%**:

```yaml
# In quotas.yaml
priorities:
  rt:    { max_share: 0.30 }   # 仿 sched_rt_runtime_us = 300_000
  normal:{ max_share: 0.65 }
  idle:  { max_share: 0.05 }
```

If RT queue exceeds 30% of system budget, the **oldest** RT task is demoted back to
CFS. Prevents bounce-back storms from starving normal work.

---

## 4. Context = compression + regeneration (NOT swap)

Linux swap = write cold pages to disk, read back. Lossless, slow I/O.

**Agent doesn't need swap.** It has two LLM-native primitives:

1. **Compression** — old messages → structured summary (lossy, fast, no I/O)
2. **Regeneration** — file/commit references → re-read / git show (lossless fidelity)

| Linux | Agent |
|---|---|
| `swap_out(page)` | `compressor.summarize(old_messages)` |
| `swap_in(page)` | `compressor.regenerate(ref)` (re-read file, re-run tool) |
| `memory.pressure_level=low` | context < 50% |
| `memory.pressure_level=medium` | 50-80%: pre-compress old turns |
| `memory.pressure_level=critical` | > 80%: throttle + auto-compress |
| `OOM kill` | post-compress still over → kill (§7) |

### 4.1 Compressor sub-agent

A dedicated sub-agent role (`role=compressor`), configured in cgroup table.
Cheap model (qwen2.5-coder-1.5b or haiku-class). Tasks:

| Task | Input | Output |
|---|---|---|
| `summarize(messages)` | N old messages | structured summary (decisions / file refs / commit refs / next action) |
| `regenerate(ref)` | file path or commit ref | re-read / git show output |
| `emergency_compress(pid)` | full task context | minimal summary to free window |

The compressor **preserves**:
- File references (so dev can re-read)
- Commit references (so dev can git show)
- Decision log verbatim (decisions are why, not just what)
- Test failure assertion + log excerpt (so dev can reproduce)

The compressor **discards**:
- Verbose discussion
- Intermediate tool output
- Back-and-forth retries

### 4.2 Pressure response

```python
def on_context_pressure(self, pid: int):
    used = self.token_accountant.context_used(pid)
    cap = self.tasks[pid].cgroup.context_window_max
    ratio = used / cap

    if ratio < 0.50:
        return  # normal

    if ratio < 0.80:
        # pre-compress: oldest 20 messages → summary
        self.compressor.summarize(pid, count=20)
        return

    if ratio < 0.95:
        # throttle: tell task to checkpoint and yield
        self._throttle(pid)
        # heavy compress: oldest 100 messages
        self.compressor.summarize(pid, count=100)
        return

    # >= 0.95
    self.compressor.emergency_compress(pid)
    if self.token_accountant.context_used(pid) / cap >= 0.95:
        # still over after compress → ask user
        self._escalate_to_user(pid, "context pressure critical")
```

### 4.3 Pressure stall information (PSI)

Linux PSI exposes `some/full` pressure metrics. Agent exposes:
- `agent_psi.pressure_some` — fraction of tasks waiting on context compress
- `agent_psi.pressure_full` — fraction of time ALL tasks were stalled

When `pressure_full > 0.20` for 10s, main agent (PID 1) is notified to refuse new
work and prompt user: "system context pressure high, let me compress and continue."

---

## 5. Contract (5-element handoff)

Every agent task carries a Contract. Linux has `struct subprocess_info` + argv/envp;
Contract is the typed version.

```python
@dataclass
class Contract:
    # 1) 任务主体
    goal: str                              # one-line task description
    scope_in: list[str]                    # explicit whitelist
    scope_out: list[str]                   # explicit boundary

    # 2) 上下文切片 (NOT full context)
    context_slice: ContextSlice            # see below

    # 3) 资源约束 (mirrors cgroup)
    token_budget: int
    wall_budget_sec: int
    must_not_modify: list[str]

    # 4) 完成定义
    done_when: list[str]                   # ["tests pass", "deliverable.md exists", "VERDICT: PASS"]
    deliverable_path: str | None

    # 5) 失败处理
    on_fail: Literal["bounce_back", "escalate", "kill"] = "bounce_back"
    max_retry: int = 3

@dataclass
class ContextSlice:
    files: list[str]                       # paths to re-read
    commits: list[str]                     # git refs
    symbols: list[str]                     # class/function names (resolvable on demand)
    previous_failure: dict | None = None   # populated on bounce-back
    verifier_feedback_raw: str | None = None  # original, not rewritten
```

### 5.1 Bounce-back contract

On `on_bounce_back()`, contract is rewritten to include failure context:

```json
{
  "goal": "fix race condition in agent-tool-abort.test.ts",
  "context_slice": {
    "files": ["src/agents/agent-tool-abort.ts", "tests/agent-tool-abort.test.ts"],
    "commits": ["a4f8b2c"],
    "previous_failure": {
      "test": "should_abort_with_partial_output",
      "assertion_failed": "expected 'interrupted at' in stderr, got ''",
      "stderr_lines": ["L1", "L2", "L3"],
      "commit_at_test": "a4f8b2c"
    },
    "verifier_feedback_raw": "raw verifier output, NOT rewritten by parent"
  },
  "scope_in": ["src/agents/agent-tool-abort.ts"],
  "scope_out": ["src/agents/batch-completion.ts", "src/agents/tool-approval.ts"],
  "must_not_modify": ["tests/agent-tool-abort.test.ts (test must stay)"],
  "done_when": [
    "npx vitest run agent-tool-abort.test.ts → 0 failures",
    "deliverable.md exists with VERDICT: PASS"
  ],
  "max_retry": 2
}
```

### 5.2 Three invariants

1. **ContextSlice is explicit whitelist, not "all you can see"** — child sees exactly what's in the slice.
2. **Verifier feedback is forwarded raw** — parent does not rewrite, summarize, or "improve" it. Compressor summarizes, parent forwards.
3. **Failure context includes commit + assertion + log excerpt** — child can checkout + reproduce, no re-investigation.

---

## 6. Quota policies (config-driven, not hardcoded)

`.loom-config/quotas.yaml`:

```yaml
# Loom agent quotas — tunable, no code change to adjust
version: 1

roles:
  dev:        { token_limit: 50000,  wall_budget_sec: 1800, weight: 1024, max_forks: 3 }
  test:       { token_limit: 80000,  wall_budget_sec: 1800, weight: 1024, max_forks: 2 }
  verifier:   { token_limit: 30000,  wall_budget_sec:  600, weight: 2048, max_forks: 1 }
  summary:    { token_limit: 10000,  wall_budget_sec:  300, weight:  256, max_forks: 2 }
  compressor: { token_limit:  8000,  wall_budget_sec:  300, weight:  256, max_forks: 0 }

priorities:
  rt:     { max_share: 0.30 }    # 仿 sched_rt_runtime_us
  normal: { max_share: 0.65 }
  batch:  { max_share: 0.10 }
  idle:   { max_share: 0.05 }

bounce_back:
  retry_token_bonus: 1.5         # 50% extra on retry
  max_concurrent_rt: 5           # max RT tasks at once
  demote_oldest_when_over: true  # if RT > max_share, demote oldest
  auto_escalate_after: 3         # max retries before human

pressure:
  warn:     0.50
  throttle: 0.80
  block:    0.95
  psi_window_sec: 10

context_compression:
  pre_compress_threshold: 0.50   # start summarization at 50% used
  pre_compress_count: 20         # how many old messages per pass
  throttle_count: 100            # aggressive count
  model: "qwen2.5-coder-1.5b-instruct-q4_k_m"  # cheap, local
```

**This file is the policy.** Code reads it; humans tune it; CI verifies it's present.

---

## 7. OOM = token budget

Linux OOM killer is destructive. Agent OOM is **graceful**:

```python
def oom_kill(self, pid: int):
    task = self.tasks[pid]
    log_oom(task)

    # 1. Persist progress (atomic commit, like sync(1) before kill -9)
    task.request_checkpoint()

    # 2. Save partial output to deliverable.md
    if task.contract.deliverable_path:
        write_partial_deliverable(task, reason="oom-killed at token limit")

    # 3. Mark zombie
    task.state = State.ZOMBIE
    task.output = TaskOutput(exit_code=137, reason="oom-killed", partial=True)

    # 4. Notify parent
    self._notify_parent(task.ppid, {
        "type": "child_oom",
        "pid": pid,
        "token_used": task.token_used,
        "token_limit": task.cgroup.token_limit,
        "partial_deliverable": task.contract.deliverable_path,
    })

    # 5. Parent decides: bounce_back / escalate / new attempt
    # (handled in main loop)
```

Exit code 137 = 128 + SIGKILL(9), like Linux convention.

### 7.1 Throttling (memory.high equivalent)

```python
def _throttle(self, pid: int):
    task = self.tasks[pid]
    task.state = State.BLOCKED
    task.throttle_reason = "token_high"
    # When token dips below .high, unblock
    self._register_unblock_when(pid, threshold=task.cgroup.token_high * 0.8)
```

---

## 8. Loom v1 gaps closed by this supplement

| v1 had | This supplement adds |
|---|---|
| Plan / DAG / state | Process tree (parent/child, PID, PPID) |
| Producer spawn | Token budget per cgroup, fork inheritance |
| Verifier verdict | OOM killer (token-aware, graceful) |
| 5-state machine | Sched policy (fifo/normal/batch/idle) + nice |
| `deliverable.md` | Contract 5-element protocol |
| 3 retries | Bounce-back auto-promote to RT |
| 4-level checkpoint | Context compression at boundaries |
| (implicit) queue | Explicit CFS + RT scheduler |
| (implicit) priority | RT/CFS/Batch/Idle classes + 30% RT cap |
| (implicit) capacity | cgroup inheritance + system-wide PSI |

---

## 9. Implementation roadmap (mapping to Slice 2)

| Supplement component | S2 task | Effort | Depends on |
|---|---|---|---|
| AgentTask struct + state | S2-W1 Bridge | 1-2d | — |
| Contract 5-element | S2-W1 Bridge | 1-2d | — |
| Quotas YAML schema | S2-W1 Bridge | 0.5d | — |
| Scheduler (CFS + FIFO) | S2-W2 Daemon | 3-4d | S2-W1 |
| cgroup accounting | S2-W2 Daemon | 2-3d | S2-W1 |
| OOM killer | S2-W2 Daemon | 1-2d | Scheduler |
| Compressor sub-agent | S2-W2 Daemon | 2-3d | S2-W1 |
| PSI monitoring | S2-W3 Coordinator | 1-2d | Daemon |
| Bounce-back auto-promote | S2-W3 Coordinator | 1d | Scheduler |
| Pressure response | S2-W3 Coordinator | 2d | Compressor + PSI |
| Token budget integration | S2-W4 budget | 2-3d | Scheduler |

**Order**: S2-W1 (data structures) → S2-W2 (runtime) → S2-W3 (coordination) → S2-W4 (budgets).

**Total**: ~18-25 days for full implementation. Critical path: S2-W2 Daemon.

---

## 10. Open questions (5)

1. **Compressor model**: cheap local (qwen2.5-coder-1.5b) vs API-based (haiku). Tradeoff: cost vs quality of summary. Default proposal: cheap local for old-message summary, full model for verifier feedback preservation (raw + summary).
2. **Cross-session scheduler state**: scheduler state lives in plan state file (5-file pattern), but does it span sessions? If main agent (PID 1) restarts (new session), does the scheduler resume or restart from clean slate? Default: clean restart, only in-flight tasks survive (via plan state file).
3. **cgroup inheritance depth**: 3 levels deep (grandchild)? Or unlimited? Linux has no depth limit. Agent should cap at 3 to prevent resource fragmentation.
4. **PID 1 (main agent) safety**: if main agent's context hits 95%, what happens? Options: (a) refuse new tasks only, (b) auto-compress + refuse, (c) hard block + escalate to user. Default proposal: (b) — compress aggressively, refuse new, but never kill self.
5. **Bounce-back RT quota cap**: 30% chosen. But the cap interacts with `max_concurrent_rt=5` from quotas.yaml. Which wins? Default proposal: 30% system budget wins; if 30% would mean >5 RT tasks, both limits enforced (whichever smaller).

---

## 11. Deferred (NOT in this supplement)

- Cross-machine agent distribution (treat as remote process, RPC over SSH)
- Agent persistence across machine reboot
- Multi-tenant isolation (different users' agents on same scheduler)
- GPU/TPU-aware scheduling (token is a proxy for GPU time)
- Scheduler hot-reload / config change at runtime
- Fair-share across multiple users (currently single-user scheduler)

These are S3+ concerns, listed for completeness. Not blocked on this supplement.

---

## Appendix A — One-page summary

```
Loom agent = Linux process
├── task_struct  (pid, ppid, state, sched_policy, nice, vruntime, cgroup, contract)
├── fork()      → spawn(ppid, role, contract)
├── exec()      → task.switch_role()
├── wait4()     → parent.wait(pid)
├── kill -9     → scheduler.oom_kill() [graceful: commit + partial deliverable]
└── PID 1       = main agent (orchestrator)

Resources = cgroup
├── memory.*  → token_limit / token_high / context_window_max
├── cpu.*     → token_weight / wall_budget_sec
├── pids.*    → max_forks
└── io.*      → disk bandwidth (for compressor I/O)

Scheduler = CFS + SCHED_FIFO/RR
├── pick_next: RT → CFS → Batch → Idle
├── bounce-back: SCHED_FIFO + nice=-20 + 1.5x token bonus
└── RT cap: 30% system budget (sched_rt_runtime_us equivalent)

Context = compression + regeneration
├── NOT swap (no disk I/O needed)
├── compressor sub-agent (qwen2.5-coder-1.5b, local)
├── pressure: 50% pre-compress, 80% throttle, 95% block
└── PSI: notify PID 1 when pressure_full > 0.20 for 10s

Contract = 5-element handoff
├── goal / scope_in / scope_out
├── context_slice (files, commits, previous_failure, verifier_feedback_raw)
├── resource_budgets
├── done_when (exit criteria)
└── on_fail (bounce_back / escalate / kill)

Quotas = YAML config, no code change
└── .loom-config/quotas.yaml (see §6)
```

---

STATUS: READY_FOR_REVIEW
