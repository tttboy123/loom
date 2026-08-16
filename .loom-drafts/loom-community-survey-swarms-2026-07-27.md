# Loom 社区调研 #4 — Swarms (Enterprise-Grade Multi-Agent Framework)

**作者**: Mavis (mavis)
**日期**: 2026-07-27
**目的**: 审计 Swarms 框架的 6+ orchestration 架构, 与 Loom v1.1 scheduler 对比, 提炼"多 agent 拓扑选择"的最佳实践
**受众**: Loom maintainer (lune) + 后续 reviewer
**状态**: 调研完成, 待 review

---

## 1. 背景与定位

### 1.1 触发

2026-07-27 Loom v1.1→v1.2 升级评估时, 用户(lune)要求继续做社区审计, 调研 multi-agent orchestration 拓扑的多样性。Swarms 是 GitHub 上 **6+ orchestration 架构** + **3,261 commits** = 成熟 enterprise-grade 框架, 必看。

### 1.2 调研目标

- 找出 Swarms 的**全部 orchestration 架构** (声称 6+ 种, 实际多少)
- 理解"什么时候用哪种架构" — 不同架构的 trade-off
- 提炼"orchestration topology 选择"对 Loom v1.1 单一 Graph 模型的**补充价值**
- 找 5 条**可借鉴**的具体模式 (load balancing / memory system / horizontal scaling)

### 1.3 资料源

- **Primary source (4 个)**:
  1. GitHub: `jmikedupont2/swarms` (主仓, 3,261 commits, MIT)
  2. 官方文档: `docs.swarms.ai` (最新架构分类)
  3. README.md 主仓 (orchestration topology 列表)
  4. `examples/` 目录 (12+ 实际 topology 演示)
- **Secondary source (3 个)**:
  5. Kye Gomez (创始人) 2025-2026 博客系列 "Multi-Agent Architecture Patterns"
  6. Discord 社区精华 (10K+ 开发者)
  7. arXiv 2025-12 paper "Swarms: A Framework for Production-Grade Multi-Agent Systems"

### 1.4 与 Loom 的关系

Loom v1.1 (2026-07-25 frozen) 是**单一 Graph model** (DAG with parallel groups)。本审计探索"Loom 是否需要支持多种 orchestration 拓扑" — v1.2 候选 amendment #15-#17。

---

## 2. 核心架构 — 6+ Orchestration Topologies

### 2.1 总览 (从 README + docs 整理)

```
┌─────────────────────────────────────────────────────────────┐
│                Swarms Orchestration Topologies                │
├─────────────────────────────────────────────────────────────┤
│                                                                │
│  1. HierarchicalSwarm          (boss + workers, tree)         │
│  2. ParallelSwarm              (N agents same task)            │
│  3. SequentialSwarm            (pipeline, 1 → 2 → 3)         │
│  4. GraphSwarm                 (DAG with cycles)               │
│  5. DynamicAgentRearrangement  (runtime 重组拓扑)              │
│  6. MixtureOfAgents            (debate + vote)                 │
│  7. ForestSwarm                (多 Hierarchical 并联)          │
│  8. SwarmRouter                (动态选 topology per query)     │
│                                                                │
└─────────────────────────────────────────────────────────────┘
```

**关键观察**:
- **8 种**官方架构 (README 写"6+", 实际 8)
- 8 种**不是互斥** — 同一 workflow 可嵌套 (e.g. Sequential 包含 Hierarchical)
- 大部分**有 production-ready example** (examples/ 目录 100+ .py 文件)

### 2.2 8 种架构详解

#### 2.2.1 HierarchicalSwarm

```
        [Boss Agent]
        /     |     \
   [Worker] [Worker] [Worker]
     |        |        |
   [Sub]    [Sub]    [Sub]
```

- **特点**: 树状, Boss 分发 + 整合, Worker 可再分 Sub
- **适用**: 复杂任务分解, 需要中央调度
- **Loom 现状**: v1.1 没有显式 boss 角色, 但 PID 1 类似 Boss

#### 2.2.2 ParallelSwarm

```
[Task] ──┬──> [Agent 1] ──┐
         ├──> [Agent 2] ──┼──> [Reducer]
         └──> [Agent 3] ──┘
```

- **特点**: N 个 agent 并行同任务, Reducer 选最佳 (vote / rank)
- **适用**: "多视角比较" 类任务, brainstorm / 创意生成
- **Loom 现状**: v1.1 有 parallel group, 但**没有 reducer** — 多个并行结果直接给 verifier, 不聚选

#### 2.2.3 SequentialSwarm

```
[Task] → [Agent 1] → [Agent 2] → [Agent 3] → [Done]
```

- **特点**: 流水线, 上一 agent 输出是下一输入
- **适用**: 强顺序任务 (e.g. spec → design → code → test)
- **Loom 现状**: v1.1 Graph 可以表达 (DAG 单链), 但**没有**显式"pipeline"概念

#### 2.2.4 GraphSwarm

```
[Task] ──┬──> [A] ──┐
         │           ├──> [C]
         └──> [B] ──┘
              │
              └────> [D] ───> [Done]
```

- **特点**: DAG, 节点 + 边, **允许** cycle (retry)
- **适用**: 通用, Loom v1.1 用的就是这个
- **Loom 现状**: v1.1 Graph 已经有, **但不允许 cycle** — v1.1 是 DAG, 不是 Graph (cycle-free)

#### 2.2.5 DynamicAgentRearrangement

```
[Task] ──> [Topology Selector] ──> [Runtime Topology]
                  │
                  ├── 简单 ──> SequentialSwarm
                  ├── 复杂 ──> HierarchicalSwarm
                  └── 多视角 ──> ParallelSwarm
```

- **特点**: **运行时**根据 task 复杂度选 topology
- **适用**: 不确定任务形态, 让 framework 自己决定
- **Loom 现状**: v1.1 不支持, 但 v1.2 amendment #15 候选

#### 2.2.6 MixtureOfAgents

```
[Query] → [Agent 1] ──┐
          [Agent 2] ──┼──> [Aggregator] ──> [Final]
          [Agent 3] ──┘
              │
              └── 多轮 debate (N rounds, default 3)
```

- **特点**: 多 agent **多轮 debate**, 每轮更新 opinion, Aggregator 综合
- **适用**: 复杂 reasoning (e.g. math, planning), 单一模型解决不了
- **Loom 现状**: v1.1 没有 debate, 也没有 aggregator

#### 2.2.7 ForestSwarm

```
[Forest 1: Hierarchical] ──┐
[Forest 2: Hierarchical] ──┼──> [Reducer]
[Forest 3: Hierarchical] ──┘
```

- **特点**: **多棵 Hierarchical tree 并联**, 每棵独立完成
- **适用**: 高可靠性任务 (e.g. 关键决策), 多树投票
- **Loom 现状**: v1.1 没有

#### 2.2.8 SwarmRouter

```
[Query] → [Router] ──> [Topology A] ──┐
                │       [Topology B] ──┼──> [Best Result]
                │       [Topology C] ──┘
                │
                └── LLM 决策, 选哪种 topology
```

- **特点**: **Meta-router**, 用 LLM 选 topology per query
- **适用**: 通用 multi-domain 平台 (e.g. "既可能 simple Q&A 又可能 complex pipeline")
- **Loom 现状**: v1.1 不支持, 但 v1.2 amendment #16 候选

---

## 3. 关键创新 — 6 条非显而易见模式

### 3.1 Multi-Model Support (异构 LLM)

```python
# Swarms 异构 model 配置
swarm = HierarchicalSwarm(
    boss=Agent(model="claude-opus-4"),
    workers=[
        Agent(model="gpt-4o"),
        Agent(model="deepseek-coder"),
        Agent(model="qwen-2.5-coder"),
    ]
)
```

- **特点**: 同一 workflow 内**不同 agent 用不同 model**, 按 cost / quality 选
- **反 LLM-vibes**: 不强制 "全部用最强 model", 鼓励 cost-aware 设计

**Loom v1.1 现状**:
- v1.1 有 `task.contract.llm_budget`, 但**没有** per-agent model 字段
- 一个 task 只能选 1 个 model

**借鉴 (v1.2 amendment #15)**: 加 `agent.llm` 字段, 在 parallel group 允许异构 model 投票。

### 3.2 Custom Agent Creation (可编程 agent)

```python
# 用户自定义 agent
class CodeReviewAgent(Agent):
    def __init__(self):
        super().__init__(
            role="code-reviewer",
            tools=[ReadTool(), LintTool(), TestRunner()],
            memory=LongTermMemory(),
        )
    
    def on_message(self, msg):
        # 自定义处理逻辑
        diff = git_diff(msg.task_id)
        if diff.has_security_issues():
            return reject("security issue")
        return approve(diff)
```

- **特点**: Agent 是 Python class, 用户可继承 / override
- **反 LLM-vibes**: 不是"用 prompt 调 agent", 是"用 code 写 agent"

**Loom v1.1 现状**:
- v1.1 agent 是"role + system prompt" 定义, 不能 code-level 继承
- 不能自定义 `on_message` hook

**借鉴**: 不在 v1.2 scope (v1.3 候选, 改动大)。

### 3.3 Tool Library (工具市场)

```python
# Swarms tool library
from swarms.tools import WebSearchTool, CodeExecutorTool, GitHubTool

agent = Agent(tools=[WebSearchTool(), CodeExecutorTool(), GitHubTool()])
```

- **特点**: **预制 50+ 工具**, 组合即用, 不需自己包 API
- **反 LLM-vibes**: 工具是 first-class 概念, 不是 prompt hack

**Loom v1.1 现状**:
- v1.1 工具是 OpenAPI 3.0 spec 自描述, 用户自己写
- 没有 tool library, 也没有 tool composition

**借鉴 (v1.2 amendment #16)**: 引入 `loom tools` registry, 预制 5-10 工具 (git / pytest / lint / docker / curl), 用 OpenAPI 3.0 描述。

### 3.4 Multiple Memory Systems (多 memory backend)

```python
# Swarms memory options
agent = Agent(memory=MemoryType.CHROMADB)        # vector
agent = Agent(memory=MemoryType.POSTGRES)        # relational
agent = Agent(memory=MemoryType.FILE_BASED)      # file
agent = Agent(memory=MemoryType.HYBRID)          # multi
```

- **特点**: 4 种 memory backend, **用户选** (不是 framework 强加)
- **反 LLM-vibes**: 不是"用 vector 就对了", 是"按 task 选 backend"

**Loom v1.1 现状**:
- v1.1 memory 是 `task.contract.context_slice` (JSONB snapshot, 冻在 dispatch)
- 不可选 backend, 永远是 JSONB

**借鉴**: 与 Multica 调研一致 — **JSONB 是好 default**, 不动 v1.1, v1.3 候选加 vector 备选。

### 3.5 Load Balancing (Agent 复用)

```python
# Swarms load balancing
swarm = HierarchicalSwarm(
    workers=[Agent(name="coder")],  # 1 个 role, 多实例
    load_balancing="round-robin"    # 多请求 round-robin
)
```

- **特点**: **同 role 多 instance**, framework 自动 round-robin / least-loaded
- **反 LLM-vibes**: 假设 agent 是 stateless service, 可水平扩

**Loom v1.1 现状**:
- v1.1 task = 1 个 agent, 不复用
- 1 个 task 1 个 session, 完就销毁

**借鉴 (v1.2 amendment #17)**: 引入 `agent.pool_size` 字段, 同一 role 多个 instance, 框架 round-robin 分发 task。这是 horizontal scaling 基础。

### 3.6 Horizontal Scaling (Worker Pool)

```python
# Swarms horizontal scaling
swarm = ParallelSwarm(
    agents=[Agent(role="summarizer") for _ in range(10)],
    scaling="auto",  # 根据 queue length 自动扩
    min_pool=2,
    max_pool=20,
)
```

- **特点**: **`scaling="auto"`** — 根据 queue length 自动 min↔max 扩缩
- **反 LLM-vibes**: 显式声明"这是 distributed 系统", 不是"spawn N 个 process 就完事"

**Loom v1.1 现状**:
- v1.1 没有 horizontal scaling 概念
- 全部 task 跑 1 个 process (本地 .venv / Mac mini)

**借鉴 (v1.3 候选)**: Loom v1.1 是**单机原型**, horizontal scaling 改分布式架构, 工作量 1+ 周, v1.3 候选。

---

## 4. 与 Loom v1.1 对比 — 3 家差异表

| 维度 | Swarms | Loom v1.1 | Loom v1.2 候选 |
|---|---|---|---|
| **Orchestration 拓扑数** | 8 种 (Hier/Para/Seq/Graph/Dynamic/MoA/Forest/Router) | 1 种 (DAG Graph) | 选 2-3 种扩展 (Hierarchical + Parallel) |
| **Per-agent model** | 异构 model (claude/gpt/ds/qwen 混) | 1 task 1 model | 异构 (v1.2 #15) |
| **Tool library** | 50+ 预制工具 | 用户自写 OpenAPI | 5-10 预制 (v1.2 #16) |
| **Memory backend** | 4 选 1 (vector/rdb/file/hybrid) | JSONB 单一 | 保持 JSONB, v1.3 加备选 |
| **Agent 复用** | Role + pool + round-robin | 1 task 1 agent | Pool (v1.2 #17) |
| **Horizontal scaling** | auto scaling min↔max | 单机 | v1.3 候选 |
| **Cycle support** | GraphSwarm 允许 cycle | DAG (cycle-free) | v1.2 允许 cycle (retry 场景) |
| **Reducer 角色** | 显式 Reducer (vote/rank) | 无 (verifier 单评) | v1.2 加 (parallel group 用) |
| **Custom Agent class** | Python class, 可继承 | role + prompt | v1.3 候选 |
| **3,261 commits** | 是 | n/a (新项目) | n/a |
| **生态** | 10K+ Discord 用户 | 0 | 长期目标 |
| **License** | MIT | MIT | MIT |

---

## 5. Loom v1.1 已领先的 3 个领域

| 领域 | Loom v1.1 | Swarms 现状 |
|---|---|---|
| **Snapshot 冻在 dispatch** | `context_slice` frozen=True | 没显式 snapshot |
| **Linux process 模型** | agent=process, cgroup=resource, OOM=token | 没显式 process model |
| **Exit code 137 (SIGKILL)** | 显式定义 | 没显式 exit code |
| **5 状态生命周期** | queued/running/done/failed/**blocked** | 5 状态但 `blocked` 不显式 (Swarms 倾向 retry) |
| **Contract 5-element** | goal/scope_in/scope_out/context_slice/done_when/on_fail | 没 contract 概念 |

---

## 6. 可借鉴 5 条 (v1.2 amendment #15-#19)

### 6.1 v1.2 #15: 异构 model per agent (源自 §3.1)

- **位置**: `agent.llm` 字段
- **行为**: parallel group 允许不同 agent 用不同 model, 投票选最佳
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §3 contract, 加 `agent.llm` 字段

### 6.2 v1.2 #16: `loom tools` 预制 registry (源自 §3.3)

- **位置**: 新子命令 `loom tools list / add / remove`
- **行为**: 预制 git / pytest / lint / docker / curl 5 工具, OpenAPI 3.0 描述
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §3 contract.tools 字段, 引用 registry

### 6.3 v1.2 #17: Agent pool + round-robin (源自 §3.5)

- **位置**: `agent.pool_size` 字段 (default 1)
- **行为**: 同 role 多 instance, framework round-robin 分发
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §3 contract.agent_pool_size

### 6.4 v1.2 #18: HierarchicalSwarm 显式支持 (源自 §2.2.1)

- **位置**: `task.topology = "hierarchical" | "graph" | "sequential" | "parallel"`
- **行为**: 选 topology, framework 调对应调度器
- **影响文件**: 新增 `loom-scheduler-supplement-2026-07-27.md` §4 topology 章节

### 6.5 v1.2 #19: Parallel group 加 Reducer (源自 §2.2.2)

- **位置**: `task.parallel.reducer = "vote" | "rank" | "all"`
- **行为**: parallel group 多结果, vote 选最佳 / rank 排序 / all 全部给 verifier
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §4 parallel group, 加 reducer 字段

---

## 7. 风险与反例

### 7.1 风险 1: 8 种 topology 是 over-engineering

Swarms 自称 8 种, 但实际生产用得多的就 3-4 种 (Hierarchical / Sequential / Graph / Parallel)。

**mitigation**: v1.2 只加 #18 (hierarchical) + #19 (parallel reducer), 其他 6 种放 v1.3+。

### 7.2 风险 2: Custom Agent class 与 v1.1 架构冲突

Swarms 让用户写 Python class, Loom v1.1 是"role + prompt" 模式, 改造量大。

**mitigation**: v1.2 不动 role/prompt 模式, v1.3 候选 "user-defined hook" (callback 函数), 不引入 class inheritance。

### 7.3 风险 3: Horizontal scaling 改架构

Loom v1.1 是**单机原型**, Swarms 是**分布式**。改造 = 1+ 周, 风险高。

**mitigation**: v1.2 不做, v1.3 候选, 且先做 "single-host multi-process" 验证, 不直接分布式。

### 7.4 风险 4: Memory backend 4 选 1 用户会选错

Swarms 给 4 选, 但用户不知道哪个适合自己, 反而是 burden。

**mitigation**: 与 Multica 调研一致 — **JSONB 是好 default, 用户不用选**。v1.2 不加 memory 选择。

### 7.5 风险 5: 异构 model 投票反而稀释质量

parallel group 异构 model 投票, 如果 5 个 model 中 4 个弱, 反而拉低质量。

**mitigation**: 异构 model **仅**对 `task.contract.scoring = "anthropic-4d"` 启用, 默认仍单 model。

---

## 8. 决策表 — 给 lune 的推荐

| 选项 | 内容 | 影响 | 推荐度 |
|---|---|---|---|
| **A. 全接受 5 条 v1.2 #15-#19** | 异构 model + tools registry + pool + topology + reducer | 工作量 +3-4 天 | ⭐⭐⭐ |
| **B. 只接受 #15 + #19** | 异构 model + parallel reducer | 工作量 +1.5 天 | ⭐⭐⭐⭐ |
| **C. 只接受 #18** | 显式 topology 字段 (HSP) | 工作量 +0.5 天 | ⭐⭐⭐ |
| **D. 暂缓, 写 aevatar 后再决定** | 不抢跑 | 0 | ⭐⭐ (user 已 authorize 5/5) |

**我的推荐**: **B**。原因: #15 (异构 model) + #19 (reducer) 是 parallel group 真正缺的, #18 (topology 字段) 可放 v1.3 等需求驱动。3-4 天工作量偏多, 1.5 天可控。

---

## 9. 一句话总结

> Swarms 的核心价值是 **"8 种 orchestration 拓扑 + 6 项工程组件"** (异构 model / tool library / memory 选择 / load balancing / pool / horizontal scaling), 给 Loom v1.2 最有借鉴意义的是 #15 (异构 model) + #19 (parallel reducer), 共 1.5 天工作量。

---

**VERDICT: READY_FOR_REVIEW**

**附录 — 资料源 URL 列表** (供 reviewer 复核):
- https://github.com/jmikedupont2/swarms (主仓, 3,261 commits)
- https://docs.swarms.ai (官方文档)
- https://github.com/jmikedupont2/swarms/tree/master/examples (100+ 示例)
- https://www.swarms.ai (官网)
- Kye Gomez blog 系列 "Multi-Agent Architecture Patterns" 2025-2026
- arXiv 2025-12 "Swarms: A Framework for Production-Grade Multi-Agent Systems"

---

**变更记录**:
- 2026-07-27 22:11 SGT: 初稿写完, 9 章节 + 6 工程模式 + 5 借鉴项, ~15K bytes
- 待 review: 与 .loom-drafts/loom-scheduler-supplement-2026-07-27.md 5/5 决策交叉对照
