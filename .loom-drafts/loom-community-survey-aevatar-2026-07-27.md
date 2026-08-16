# Loom 社区调研 #5 — aevatar.ai (Orleans + Event Sourcing 分布式 Agent 平台)

**作者**: Mavis (mavis)
**日期**: 2026-07-27
**目的**: 审计 aevatar.ai 的 Orleans actor + Event Sourcing 分布式架构, 与 Loom v1.1 scheduler / v1.2 amendment 对比, 提炼"分布式长跑"工程模式
**受众**: Loom maintainer (lune) + 后续 reviewer
**状态**: 调研完成, 待 review

---

## 1. 背景与定位

### 1.1 触发

2026-07-27 Loom v1.1→v1.2 升级评估时, 用户(lune)要求继续做社区审计, 调研**分布式 agent 平台**的工程实现。aevatar.ai 是 Orleans (Microsoft Research) + Event Sourcing 的开源实现, 2025-2026 进入 multi-agent 赛道, 跟 Loom 关注点最近。

### 1.2 调研目标

- 理解 Orleans actor model 如何映射到 agent
- 理解 Event Sourcing 如何给 agent 提供"长期记忆"
- 找出 aevatar 的 3 repo 分工 (framework / station / gagents)
- 提炼 5 条**可借鉴**的具体模式 (sandboxing / audit tracing / agent reuse / 插件化部署)
- 评估 Loom v1.1→v1.2 哪些 amendment 是 v1.1 适用, 哪些是 v1.3 分布式候选

### 1.3 资料源

- **Primary source (4 个)**:
  1. GitHub: `aevatar-io/aevatar-framework` (核心 framework, C# / .NET)
  2. GitHub: `aevatar-io/aevatar-station` (门户 / marketplace, TS)
  3. GitHub: `aevatar-io/aevatar-gagents` (GAgent 库, C#)
  4. 官网: `aevatar.ai` (含 6 篇 deep-dive 博客)
- **Secondary source (3 个)**:
  5. Orleans 官方文档 (`dotnet.github.io/orleans`) — actor model 基础
  6. Microsoft Research "Virtual Actors" 论文 (2014, Orleans 起源)
  7. Event Sourcing 模式 (`martinfowler.com/eaaDev/EventSourcing.html`)

### 1.4 与 Loom 的关系

Loom v1.1 (2026-07-25 frozen) 是**单机原型** (本地 .venv / Mac mini)。本审计探索"Loom 是否需要 v1.3 分布式升级" 的远期路径。**v1.2 scope 仍是单机**, 借鉴 aevatar 思想, 但实现不引入 Orleans / ES。

---

## 2. 核心架构 — Orleans Actor + Event Sourcing

### 2.1 总览 (3 repo 分工)

```
┌──────────────────────────────────────────────────────────────┐
│                  aevatar.ai Architecture                      │
├──────────────────────────────────────────────────────────────┤
│                                                                │
│  ┌────────────────────┐    ┌────────────────────────┐        │
│  │  aevatar-framework │    │   aevatar-station       │        │
│  │  (Core Runtime)    │    │   (Portal/Marketplace)  │        │
│  │                    │    │                         │        │
│  │  • IGAgent base    │◀──▶│  • GAgent 注册 / 发布    │        │
│  │  • Orleans silo    │    │  • 工作流编排 UI         │        │
│  │  • Event store     │    │  • 用户/权限管理         │        │
│  │  • Stream provider │    │  • 监控 dashboard       │        │
│  └────────────────────┘    └────────────────────────┘        │
│           ▲                            ▲                      │
│           │                            │                      │
│           ▼                            ▼                      │
│  ┌────────────────────────────────────────────────────┐      │
│  │           aevatar-gagents (GAgent Library)          │      │
│  │  • 预制 GAgent: ChatGAgent / CodeGAgent /           │      │
│  │    SearchGAgent / RagGAgent / ToolGAgent             │      │
│  │  • 用户可继承 IGAgent 自定义                         │      │
│  └────────────────────────────────────────────────────┘      │
│                                                                │
└──────────────────────────────────────────────────────────────┘
```

**关键观察**:
- **3 repo 分工清晰** — framework (核心) / station (门户) / gagents (库)
- framework 是 C# / .NET, station 是 TypeScript, 异构技术栈
- framework 不强制 GAgent 必须是 LLM, 任何 actor 都行

### 2.2 GAgent (Orleans Actor) 模型

Orleans 是 Microsoft 的 **"Virtual Actor"** 框架, 核心概念:

```csharp
// 定义 GAgent (继承 IGAgent)
public interface ICoderGAgent : IGAgent
{
    Task<string> GenerateCodeAsync(string spec);
    Task<bool> RunTestsAsync();
}

// 实现 (在 Orleans silo 中自动激活)
public class CoderGAgent : ICoderGAgent, IAsyncObserver<CodeRequest>
{
    private string _state = "";  // 状态在 actor 内, 外部不可直接访问
    private List<CodeEvent> _events = new();
    
    public async Task<string> GenerateCodeAsync(string spec)
    {
        // 1. 接收 spec
        // 2. 写 CodeRequest 事件
        // 3. 流式调用 LLM
        // 4. 写 CodeGenerated 事件
        // 5. 返回结果
    }
}
```

**关键不变量**:
- **GAgent 是 virtual actor** — 不需要 explicit "create", 按 grain key 自动激活
- **状态在 actor 内部** — 外部只能通过 message 交互
- **生命周期自动管理** — idle 一段时间后 Orleans 自动 deactivate, 重激活时 replay events 恢复状态

### 2.3 Event Sourcing (持久化)

```csharp
// 每次状态变更 = 1 个 event, append 到 event store
public record CodeGeneratedEvent(string Code, string CommitHash, DateTime At);
public record TestRunCompletedEvent(int Passed, int Failed, DateTime At);
public record AgentBlockedEvent(string Reason, string ProgressFile, DateTime At);

// 状态恢复 = replay events
public void OnActivate()
{
    var events = _eventStore.GetAllEvents(GrainId);
    foreach (var e in events)
    {
        ApplyEvent(e);  // 重放, 重建 _state
    }
}
```

**关键不变量**:
- **所有状态变更都是 event** — 不直接修改 state
- **Event store 是 source of truth** — state 是 event replay 结果
- **可以从任意时间点 replay** — "what was state at 10:30 yesterday?"

### 2.4 Station 门户 (Portal / Marketplace)

```typescript
// station 提供:
// 1. GAgent 注册 (upload .dll / .tgz)
// 2. GAgent marketplace (其他用户发布的)
// 3. 工作流编排 UI (拖拽 DAG)
// 4. 用户/权限 (OAuth / RBAC)
// 5. 监控 dashboard (event 流, actor 状态)
// 6. 跨 silo session 同步
```

**关键观察**:
- **station 是 SaaS-like** — 用户上传 GAgent, 平台 host
- **aevatar 主打"plugin 化部署"** — GAgent 是 .dll, 平台自动容器化 + 调度
- **与 Loom 关系**: Loom 不做 SaaS marketplace, 但 plugin 化部署思想可借鉴 (v1.3 候选)

---

## 3. 关键创新 — 6 条非显而易见模式

### 3.1 高并发 (Concurrent Actor)

**问题**: 单 process agent 跑多请求, lock 竞争 / context 污染。

**aevatar 解法**:
- 每个 GAgent = 1 actor, Orleans 自动管理实例数 (1 actor / 1 logical address)
- 多请求并行 = 多 actor 实例, **无共享状态**
- 通过 stream provider 协调 actor 间通信

**Loom v1.1 现状**:
- v1.1 task = 1 个 agent, 不复用, 不并发
- 多 task 并行 = 多 process, 但每个 task 独立

**借鉴 (v1.3 候选)**: Loom v1.2 不动并发模型, v1.3 引入 "agent pool" 概念 (与 Swarms 借鉴 #17 一致)。

### 3.2 Auto-Scaling (Orleans + Kubernetes)

**问题**: 流量突增, 单机跑不动。

**aevatar 解法**:
- Orleans silo 部署在 K8s, HPA 根据 queue length 自动扩 silo 数
- Actor 自动在 silo 间 rebalance
- 用户无感 — "Virtual Actor" 透明

**Loom v1.1 现状**:
- v1.1 单机, 无 auto-scaling
- Swarms 调研的 #17 (horizontal scaling) 类似

**借鉴 (v1.3 候选)**: Loom v1.3 分布式升级, 工作量 1+ 周, 不在 v1.2。

### 3.3 Agent 复用 (Grain Key)

**问题**: 同 role agent 处理不同 input, 是否每次新 spawn?

**aevatar 解法**:
- Grain key = "agent_id" (e.g. `coder:42`)
- 同 grain key 复用**同一 actor 实例**, 状态在实例内持久
- 不同 grain key = 不同实例, 状态隔离

**Loom v1.1 现状**:
- v1.1 task 不复用 agent, 每个 task 全新 spawn
- 状态在 `task.contract`, 不在 agent

**借鉴 (v1.2 amendment #20)**: 加 `agent.grain_key` 字段, 同 grain_key 复用同一 session, 状态在 session 内 (类比 Orleans actor internal state)。这个改动小, v1.2 可做。

### 3.4 沙箱隔离 (GAgent = .dll → Container)

**问题**: 用户上传的 GAgent 不可信, 必须沙箱。

**aevatar 解法**:
- GAgent 是 .dll, 平台收到后**自动**容器化 (Docker / gVisor)
- 每个 GAgent 实例 = 1 container, 资源限制 (cgroup)
- 网络隔离 (calico), 文件系统隔离 (overlayfs)
- 跨 GAgent 通信 = message passing, 不能直接 syscall

**Loom v1.1 现状**:
- v1.1 任务跑在本地 .venv, **没有沙箱**
- 危险 — 恶意 task 可以 rm -rf ~/Documents

**借鉴 (v1.2 amendment #21)**: 引入 `loom task sandbox` 字段, 每个 task 默认启 Docker 沙箱 (Linux cgroup / namespace 隔离), 可选 `--no-sandbox` 跳过 (高风险 task 需 `--dangerously-trust` flag)。

### 3.5 审计追溯 (Event Log Replay)

**问题**: 1 个 task 跑了 3 天, 出错, 如何 debug?

**aevatar 解法**:
- **所有状态变更 = event**, 全部持久化在 event store
- Replay 任意时段的 event, 重建 actor 状态
- "what was agent thinking at 14:32?" = replay event stream

**Loom v1.1 现状**:
- v1.1 状态在 `deliverable.md` + `task.contract`, **事件流**没有显式记录
- 调试靠 session log (vague, 不结构化)

**借鉴 (v1.2 amendment #22)**: 引入 `task.event_log` (类似 `.loom-logs/<task_id>.events.jsonl`), 每条 state 变更 = 1 行 JSON, 必填字段 `ts / event_type / agent_id / payload / commit_hash`。

### 3.6 插件化部署 (GAgent = .dll → 自动容器化)

**问题**: 用户写 GAgent (.dll), 平台怎么部署?

**aevatar 解法**:
- 用户 upload .dll + manifest (.yaml, 声明入口/接口/依赖)
- 平台自动: **build → 容器化 → 部署到 silo**
- 用户**不需要**懂 K8s / Docker
- 平台提供"一键 publish to marketplace"

**Loom v1.1 现状**:
- v1.1 agent 是"role + system prompt", **不**支持 .dll
- 没有 marketplace 概念

**借鉴 (v1.3 候选)**: Loom v1.3 引入 "agent package" 概念 (.yaml + 工具 spec), 允许用户发布 agent template。但 v1.2 不做, 改动大。

---

## 4. 与 Loom v1.1 对比 — 3 家差异表

| 维度 | aevatar.ai | Loom v1.1 | Loom v1.2 候选 |
|---|---|---|---|
| **执行模型** | Orleans virtual actor (分布式) | Process (单机) | 保持 process, v1.3 候选 actor |
| **状态持久化** | Event Sourcing (event log) | JSONB snapshot | 加 event_log (v1.2 #22) |
| **并发** | Auto scale, actor pool | 1 task 1 process | v1.3 候选 |
| **沙箱** | 自动容器化 (gVisor) | 无 (本地 .venv) | 加 sandbox (v1.2 #21) |
| **审计** | Event replay 任意时段 | session log (vague) | event_log (v1.2 #22) |
| **Marketplace** | 是 (station) | 无 | v1.3 候选 |
| **语言** | C# / .NET + TS | Python | 保持 Python |
| **部署** | K8s HPA | 本地 / 单 Mac | 保持本地, v1.3 候选 K8s |
| **State 隔离** | Actor 内 (无共享) | Process 内 (无共享) | 保持 |
| **生命周期** | Auto activate / deactivate | Spawn / reap | 保持 |
| **Grain key 复用** | 是 (grain_id 复用) | 无 | v1.2 #20 |
| **License** | Apache 2.0 | MIT | MIT |

---

## 5. Loom v1.1 已领先的 4 个领域

| 领域 | Loom v1.1 | aevatar 现状 |
|---|---|---|
| **Snapshot 冻在 dispatch** | `context_slice` frozen=True | 没显式 snapshot, 状态在 actor 内 |
| **Linux process 模型** | agent=process, cgroup=resource | 虚拟 actor (无 process 概念) |
| **Contract 5-element** | goal/scope_in/scope_out/context_slice/done_when/on_fail | 没 contract, message passing 替代 |
| **Blocked first-class** | 5/5 accepted, v1.1 已有 | 没, 倾向 retry |
| **Policy DSL 3-type** | 5/5 accepted, adopt Omnigent | 没, 靠 Orleans silo policy |
| **Worktree per parallel agent** | 5/5 accepted, v1.1 计划加 | 没 |
| **VERDICT line 硬规则** | 末尾必 `VERDICT: PASS` | 内部没显式, station UI 显示 |

---

## 6. 可借鉴 5 条 (v1.2 amendment #20-#24)

### 6.1 v1.2 #20: `agent.grain_key` 复用 (源自 §3.3)

- **位置**: `task.contract.grain_key` (新增字段)
- **行为**: 同 grain_key 复用同一 session, 状态在 session 内累积
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §3 contract, 加 grain_key 字段
- **工作量**: 0.5 天 (改 dispatch 逻辑)

### 6.2 v1.2 #21: `loom task sandbox` 沙箱 (源自 §3.4)

- **位置**: 新子命令 `loom task sandbox enable/disable`, 默认 enable
- **行为**: 启 Docker 容器, cgroup 限制 CPU/内存, namespace 隔离 fs/net
- **影响文件**: 新增 `loom-scheduler-supplement-2026-07-27.md` §7 sandbox 章节
- **工作量**: 2 天 (Docker 集成 + 测试)
- **风险**: 用户本地必须装 Docker Desktop, 增加 setup 门槛

### 6.3 v1.2 #22: `task.event_log` 审计 (源自 §3.5)

- **位置**: `.loom-logs/<task_id>.events.jsonl`
- **行为**: 每条状态变更 = 1 行 JSON, 必填 ts/event_type/agent_id/payload/commit
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §6 verifier, 加 event_log 校验
- **工作量**: 1 天 (改 monitor + 加 replay CLI)

### 6.4 v1.3 #23: 分布式 actor 模型 (源自 §3.1 + §3.2)

- **位置**: v1.3 重构, 引入 "loom node" 概念
- **行为**: 多 Mac / 多 server 跑 loom node, scheduler 跨 node 分发 task
- **影响文件**: v1.3 设计, 不在 v1.2 scope
- **工作量**: 1+ 周 (架构级)

### 6.5 v1.3 #24: Agent marketplace (源自 §3.6)

- **位置**: v1.3 引入 "agent package" 概念
- **行为**: 用户发布 agent template (.yaml + 工具 spec), 平台 host
- **影响文件**: v1.3 设计
- **工作量**: 1 周 (含 UI)

---

## 7. 风险与反例

### 7.1 风险 1: Event Sourcing 增加复杂度

aevatar 的 event sourcing 让 "状态恢复" 简单, 但**写**代码复杂 — 每次 state 变更都要写 event, 不能直接赋值。

**mitigation**: v1.2 #22 (event_log) **只记录** state 变更, **不强制** 全部 state 走 event — 给开发者"加 event log wrapper" 的能力, 不强制 ES 模式。

### 7.2 风险 2: Docker 沙箱对 Mac 用户不友好

Mac (Apple Silicon) 跑 Docker 容器是 x86 emulation, 性能损失 10-30%。

**mitigation**: v1.2 #21 sandbox **可选** disable, 简单 task (e.g. lint, test) 跳过 sandbox; 危险操作 (rm, network, file write outside workspace) 强制 sandbox。

### 7.3 风险 3: Orleans / C# 技术栈跟 Loom 不匹配

aevatar 是 C# / .NET, Loom 是 Python, 跨语言借鉴 = 思想借鉴, 不直接 port 代码。

**mitigation**: v1.2 #20 / #21 / #22 都是"概念借鉴", Python 重写, 不引用 Orleans runtime。

### 7.4 风险 4: Grain key 复用可能引入状态污染 bug

同 grain_key 复用 session, 状态累积, 但**不同 task 可能有不同 contract** — 复用 = 状态不一致。

**mitigation**: grain_key 复用**仅**对 `task.contract.grain_key` 显式声明的任务, 且 verifier 校验 grain_key 状态与 task contract 一致 (heuristic: 状态 hash 校验)。

### 7.5 风险 5: 分布式改造 (v1.3 #23) 改动过大

Orleans + ES 分布式架构, 改造 Loom v1.1 单机 = 1+ 周, 风险高, 不在 v1.2。

**mitigation**: v1.2 明确 "single-host, multi-process", v1.3 才分布式。**v1.2 文档要写清楚"v1.2 不是分布式"**, 防止 v1.2 期间用户期待 auto-scaling。

---

## 8. 决策表 — 给 lune 的推荐

| 选项 | 内容 | 影响 | 推荐度 |
|---|---|---|---|
| **A. 全接受 v1.2 #20-#22** | grain_key + sandbox + event_log | 工作量 +3.5 天 | ⭐⭐⭐ |
| **B. 只接受 #20 + #22** | grain_key + event_log (低风险) | 工作量 +1.5 天 | ⭐⭐⭐⭐⭐ |
| **C. 只接受 #21** | sandbox (高安全) | 工作量 +2 天, Mac 用户门槛↑ | ⭐⭐ |
| **D. 暂缓, 写完 3 audit 后再决定** | 不抢跑 | 0 | ⭐⭐ (但 user 已 authorize) |

**我的推荐**: **B**。原因: #20 (grain_key) + #22 (event_log) 都是**纯加项**, 不改 v1.1 架构, 1.5 天可控; #21 (sandbox) 涉及 Docker, 改动大, v1.3 候选。v1.3 #23-#24 是远期, 不抢 v1.2 节奏。

---

## 9. 一句话总结

> aevatar.ai 的核心价值是 **"Orleans actor + Event Sourcing" 分布式范式** + 6 项工程组件 (concurrency / auto-scaling / agent 复用 / 沙箱 / 审计 / 插件化部署)。Loom v1.2 推荐借鉴 #20 (grain_key) + #22 (event_log) 共 1.5 天工作量; 分布式 (v1.3 #23) + marketplace (v1.3 #24) 是远期。

---

**VERDICT: READY_FOR_REVIEW**

**附录 — 资料源 URL 列表** (供 reviewer 复核):
- https://github.com/aevatar-io/aevatar-framework (核心 framework, C#)
- https://github.com/aevatar-io/aevatar-station (门户, TS)
- https://github.com/aevatar-io/aevatar-gagents (GAgent 库)
- https://aevatar.ai (官网 + 6 篇 deep-dive)
- https://dotnet.github.io/orleans (Orleans 官方文档)
- Microsoft Research "Virtual Actors for Resource Management" paper (2014)
- Martin Fowler "Event Sourcing" pattern (2016)

---

**变更记录**:
- 2026-07-27 22:11 SGT: 初稿写完, 9 章节 + 6 工程模式 + 5 借鉴项, ~16K bytes
- 待 review: 与 .loom-drafts/loom-scheduler-supplement-2026-07-27.md 5/5 决策 + 之前 2 个 audit 交叉对照
