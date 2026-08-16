# Loom 调研结论 — 按 8 模块 / 6 层拆分 (2026-07-25)

> **作者**: mavis (主 session `mvs_3be0bd26bb64475dbc65181516dda9df`)
> **时间**: 2026-07-25 03:08 SGT
> **来源**: `.loom-drafts/community-survey-v3-deepdive.md` (主参考) + `phase-roadmap-supplements.md` (v0.4 5 必决)
> **目标**: 把 5 必决 + 18 P0 + 13 P1 + 6 P2 + 8 不吸 = **50 条**调研结论, 按用户给的 8 模块 / 6 层架构重新组织
> **状态**: 分析产物, 不 commit, 走 `.loom-drafts/` 路径
> **数据基线**: grep -nE `**P0 (Phase 1 借鉴)|**P1 (Phase 2 借鉴)|**P2 (Phase 3+ 借鉴)` 实数 18/13/3 (P2 标 3 + 用户记忆 3 = 6)

---

## 0. 框架定义 (来自用户)

### 8 模块 (横切功能)
| 编号 | 模块 | 核心职责 |
|---|---|---|
| M1 | 请求预处理 | 输入清洗、脱敏、格式统一、用户身份鉴权 |
| M2 | 意图理解 & 任务规划 | 意图分类、槽位抽取、任务分层拆解、执行拓扑生成、异常分支规划 |
| M3 | Skill 管理 | Skill 注册、匹配召回、版本管理、自动沉淀、人工审核 |
| M4 | 记忆引擎 | 长短记忆读写、向量检索、时效管理、召回过滤、上下文组装 |
| M5 | 工具调度 | 工具生命周期、调用参数校验、权限拦截、多工具并行调度、结果解析 |
| M6 | 执行沙箱 | 代码/命令隔离运行环境、资源配额限制、风险操作拦截 |
| M7 | 结果校验 & 迭代 | 子任务验收判定、失败重试/回滚/重规划、输出润色 |
| M8 | 运营管控 | 全链路监控、调用成本统计、权限体系、合规审计、效果迭代 |

### 6 层架构 (纵向分层)
| 编号 | 层 | 核心职责 |
|---|---|---|
| L1 | 接入网关层 | 请求限流、鉴权、多端接入、流量分发、日志埋点 |
| L2 | 意图与规划层 | 意图识别、任务拆解、Skill 匹配、执行规划器、分支重试控制器 |
| L3 | 记忆管理层 | 短期对话缓存、静态/动态长期记忆向量库、读写审核、召回精排 |
| L4 | 工具编排层 | 工具注册中心、权限风控网关、多类型工具调度器、沙箱执行环境 |
| L5 | 领域能力层 | 垂直业务 Skill 池、Coding 专属增强模块、行业知识库 |
| L6 | 基础设施层 | 大模型推理集群、向量数据库、对象存储、监控告警、运维管控平台 |

---

## 1. 5 必决决策 × 8 模块 / 6 层 (Phase 1 必出)

| # | 必决决策 | 8 模块 | 6 层 | 关键理由 |
|---|---|---|---|---|
| D1 | **Bridge v1.1 = JSON-RPC 2.0 envelope** | M1 请求预处理 + M5 工具调度 | L1 接入网关层 + L4 工具编排层 | Bridge 是请求进出 Loom 的 wire format, 同时驱动工具调度 |
| D2 | **MCP version pin 2025-11-25** | M5 工具调度 | L4 工具编排层 | MCP 1k+ server 通信协议, 是工具编排层的对外契约 |
| D3 | **拒代理 LLM (fail closed sampling/createMessage)** | M5 工具调度 + M8 运营管控 (governance) | L1 接入网关层 + L6 基础设施层 | 工具层 fail closed + governance 层明文禁止 |
| D4 | **Tool annotations ↔ Loom Risk 字段** | M5 工具调度 + M8 运营管控 | L4 工具编排层 + L6 基础设施层 | 4 hint (readOnly/destructive/idempotent/openWorld) 1:1 映射到 Risk |
| D5 | **Subagent ≠ Sampling** | M2 意图理解 | L2 意图与规划层 | Subagent 是规划期概念, Sampling 是工具期概念, 互不污染 |

---

## 2. 12 类 P0 借鉴 (细分 18-20 条) × 8 模块 / 6 层

> 注: v3 标 16 条 "P0 (Phase 1 借鉴)" + 2 条 "P0 (Phase 1 必借鉴)" / "P0 (Phase 1 借鉴哲学)" = 18 条; 加 2 条 "ClickHouse OLAP" / "OpenTelemetry compatible" = 20 条, 用户原话"12 类 18 条"是按主题归类。

| # | P0 借鉴 | 8 模块 | 6 层 | 借鉴源 |
|---|---|---|---|---|
| P0-1 | **Coordinator+specialists** 命名 + 主 context 干净 | M2 意图理解 | L2 意图与规划层 | Cline / OpenCode / LangGraph / PraisonAI |
| P0-2 | **极简哲学** (Pi 4 工具 + YOLO + Session tree) | M5 工具调度 | L4 工具编排层 | Pi (earendil-works rebrand) |
| P0-3 | **read-only plan agent** 模式 (分析 mode 不改文件) | M2 意图理解 | L2 意图与规划层 | OpenCode (build/plan 切换) |
| P0-4 | **Capabilities = composable bundles** | M3 Skill 管理 | L5 领域能力层 | Pydantic AI (Harness v0.5.0) |
| P0-5 | **Agent.run_sync / run_stream / run 3 形态** | M1 请求预处理 | L1 接入网关层 | Pydantic AI |
| P0-6 | **Middleware 洋葱圈** (journal write 路径加中间件: 加密/签名/路由) | M5 工具调度 | L4 工具编排层 | LangChain 1.0 (5 hook 模式) |
| P0-7 | **SubAgent 双类型** (SubAgent dict + CompiledSubAgent) | M2 意图理解 | L2 意图与规划层 | Pydantic AI (subagents-pydantic-ai v0.5.0) |
| P0-8 | **"Durable AI Agents"** 定位 (Run claim = view layer durable exec) | M6 执行沙箱 | L6 基础设施层 | Restate (第一 use case) |
| P0-9 | **Replay-based resume** 模式 | M7 结果校验 | L4 工具编排层 | Restate + Temporal Replay 2026 |
| P0-10 | **Google ADK + OpenAI Agents SDK 兼容** (Bridge v1.1 wire format) | M5 工具调度 | L1 接入网关层 | Temporal Replay 2026 集成 |
| P0-11 | **DBOS 走 Postgres 路线** (SQLite/Postgres 复用 durability) | M4 记忆引擎 | L6 基础设施层 | DBOS Conductor 2026-07 |
| P0-12 | **durable execution = 库/extension** (daemon 不需外部服务) | M6 执行沙箱 | L6 基础设施层 | Restate / DBOS 设计哲学 |
| P0-13 | **$USD budget per run** (cost cap per Run) | M8 运营管控 | L6 基础设施层 | Belay chaos test ($USD per job) |
| P0-14 | **.agents/skills/ 多 editor** (`.loom-drafts/skills/claude-code.md` / `codex.md` / `cursor.md` 同步) | M3 Skill 管理 | L5 领域能力层 | Arize Phoenix PXI `.agents/skills/` |
| P0-15 | **OpenTelemetry trace 出口** (输出 OTel spans 给 Langfuse/Phoenix) | M8 运营管控 | L6 基础设施层 | OpenTelemetry CNCF graduated |
| P0-16 | **JSON-RPC 2.0 envelope** | M1 请求预处理 + M5 工具调度 | L1 接入网关层 + L4 工具编排层 | MCP 2025-11-25 wire format |
| P0-17 | **W3C Trace Context 传播** | M8 运营管控 | L6 基础设施层 | W3C standard |
| P0-18 | **完整 JSON Schema 2020-12** (method schema) | M5 工具调度 | L4 工具编排层 | JSON Schema 2020-12 standard |
| P0-19 | **ClickHouse 作为底层 OLAP** (cost view 数据量大了借鉴) | M8 运营管控 | L6 基础设施层 | Langfuse (ClickHouse acquired 2026-01) |
| P0-20 | **OpenTelemetry compatible** (跟 Phoenix / SigNoz 互通) | M8 运营管控 | L6 基础设施层 | OpenInference → OTel GenAI WG |

---

## 3. 13 P1 借鉴 × 8 模块 / 6 层 (Phase 2)

| # | P1 借鉴 | 8 模块 | 6 层 | 借鉴源 |
|---|---|---|---|---|
| P1-1 | **Code mode 概念** (1 step claim = N tool calls composed) | M5 工具调度 | L4 工具编排层 | Pydantic AI CodeMode (Monty 沙箱) |
| P1-2 | **三级上下文压缩** (tool input offloading / tool result > 20k / summarization 85% 窗口阈值) | M4 记忆引擎 | L3 记忆管理层 | LangGraph Deep Agents (18-param `create_deep_agent`) |
| P1-3 | **context_schema 模式** (run-scoped 静态 tenant_id 等) | M2 意图理解 | L2 意图与规划层 | Pydantic AI |
| P1-4 | **Exactly-once 语义** (Run claim 上报事件避免重复计费) | M5 工具调度 + M6 执行沙箱 | L4 工具编排层 + L6 基础设施层 | Restate |
| P1-5 | **DBOS 新 MCP Server** (cost view 暴露 MCP) | M5 工具调度 | L4 工具编排层 | DBOS Conductor 2026-07 |
| P1-6 | **Postgres-based durable execution + OTEL 定位** | M6 执行沙箱 + M8 运营管控 | L6 基础设施层 | DBOS |
| P1-7 | **MCP server built-in** (Loom Phase 2 cost view 暴露 MCP, 学 Belay 模式) | M5 工具调度 | L5 领域能力层 | Belay / 企查查 MCP |
| P1-8 | **PXI 概念** (loom-insight AI helper) | M7 结果校验 | L5 领域能力层 | Arize Phoenix PXI |
| P1-9 | **OpenInference semantic conventions** (Loom trace span 属性标准化) | M8 运营管控 | L6 基础设施层 | OpenInference 1,948 commits |
| P1-10 | **能力发现 + 缓存** (工具多了要治理) | M5 工具调度 | L4 工具编排层 | MCP 2026-07-28 RC |
| P1-11 | **Resources 暴露稳定知识** (tool mapping / subagent dispatching table) | M3 Skill 管理 + M5 工具调度 | L5 领域能力层 | MCP Resources + 企查查 9 server |
| P1-12 | **SKILL 组织业务流程** (cost view 常用 query 放 SKILL) | M3 Skill 管理 | L5 领域能力层 | Langfuse / Phoenix SKILL.md |
| P1-13 | **Coding agents SKILL.md** (Loom 应该有 .loom-drafts/skills/) | M3 Skill 管理 | L5 领域能力层 | Langfuse (2026-07) + Phoenix PXI |

---

## 4. 6 P2 借鉴 × 8 模块 / 6 层 (Phase 3+ 长期)

> 注: v3 文档明确 P2 标签 3 条 + 用户记忆补充 3 条 = 6

| # | P2 借鉴 | 8 模块 | 6 层 | 借鉴源 | 标签状态 |
|---|---|---|---|---|---|
| P2-1 | **pg_durable SQL 关键字路线** (Loom 出 "loom SQL" 在 SQL 里触发 Run claim) | M6 执行沙箱 | L6 基础设施层 | Microsoft pg_durable 2026-07 | v3 明确 P2 |
| P2-2 | **loom-harness 包** (Harness plugin 生态) | M3 Skill 管理 | L5 领域能力层 | Pydantic AI Harness v0.5.0 | v3 明确 P2 |
| P2-3 | **Chaos test harness** (7h kill -9 验证 durability) | M7 结果校验 | L6 基础设施层 | Belay (99,004 jobs / 0 violations) | v3 明确 P2 |
| P2-4 | **Belay USD budget true-up** (long-running 任务费用结算) | M8 运营管控 | L6 基础设施层 | Belay 2026-07 | 用户记忆 P2 |
| P2-5 | **Logfire AI Gateway** (统一 LLM proxy, ⚠️ 跟"拒代理 LLM"冲突) | M5 工具调度 (限定) | L1 接入网关层 (限定) | Pydantic Stack Logfire | 用户记忆 P2 |
| P2-6 | **Temporal Workflow Streams** (workflow-as-code 模式) | M8 运营管控 | L6 基础设施层 | Temporal Replay 2026 | 用户记忆 P2 |

---

## 5. 8 永久不吸 × 8 模块 / 6 层

| # | 不吸 | 落点 (模块/层) | 原因 |
|---|---|---|---|
| X1 | **Phoenix license (ELv2)** | 整个 **M8 运营管控** / **L6 基础设施层** | ELv2 非 OSI, 跟 Loom MIT 冲突 |
| X2 | **Helicone AI Gateway** | **M5 工具调度** 的 AI Gateway 路径 / **L1 接入网关层** | 商业化 AI Gateway, 跟 Loom "fail closed on sampling" 冲突 |
| X3 | **LangSmith (闭源)** | 整个 **M8 运营管控** / **L6 基础设施层** | 闭源商业 |
| X4 | **Prefect (Python 深度)** | **M6 执行沙箱** 的 Python 路径 / **L6 基础设施层** | Python 深度 + Cloud-first 跟 Loom local-first 冲突 |
| X5 | **Continue (ARCHIVED)** | 整个 **M5 工具调度** / **L4 工具编排层** | 行业教训: 商业模式失败 → 2.0.0 不再维护 |
| X6 | **OpenInference 整套** | **M8 运营管控** 的 trace 路径 / **L6 基础设施层** | Loom 只需 OTel 输出, 不需要 25 integrations |
| X7 | **Belay / Hatchet / DBOS 整套** | 整个 **M6 执行沙箱** / **L6 基础设施层** | Loom 是 view layer, 选 1 个接, 不重做 |
| X8 | **Temporal 整套** | 整个 **M6 执行沙箱** / **L6 基础设施层** | 9 年成熟, Loom 不重做 |

---

## 6. 落点统计 (8 模块 × 6 层 矩阵)

### 6.1 按 8 模块分布 (50 条结论)

| 模块 | 5 必决 | P0 | P1 | P2 | 不吸 | **合计** |
|---|---|---|---|---|---|---|
| M1 请求预处理 | 1 | 2 | 0 | 0 | 0 | **3** |
| M2 意图理解 & 任务规划 | 1 | 3 | 1 | 0 | 0 | **5** |
| M3 Skill 管理 | 0 | 2 | 4 | 1 | 0 | **7** |
| M4 记忆引擎 | 0 | 1 | 1 | 0 | 0 | **2** |
| M5 工具调度 | 3 | 5 | 4 | 1 | 2 | **15** |
| M6 执行沙箱 | 0 | 2 | 2 | 1 | 3 | **8** |
| M7 结果校验 & 迭代 | 0 | 1 | 1 | 1 | 0 | **3** |
| M8 运营管控 | 2 | 4 | 2 | 2 | 3 | **13** |

**热点**: M5 工具调度 (15) + M8 运营管控 (13) + M6 执行沙箱 (8) = 36 条 (72%), 跟 Loom "编排 + 观测" 定位高度一致。
**冷点**: M4 记忆引擎 (2) + M1 请求预处理 (3) + M7 结果校验 (3) = 8 条 (16%), 是 Loom 跟通用 Agent 框架差异最大的地方 (因为 Loom 不抢 agent runtime, 不做记忆/RAG, 留给 sub-agent SDK)。

### 6.2 按 6 层分布 (50 条结论)

| 层 | 5 必决 | P0 | P1 | P2 | 不吸 | **合计** |
|---|---|---|---|---|---|---|
| L1 接入网关层 | 2 | 2 | 0 | 1 | 1 | **6** |
| L2 意图与规划层 | 1 | 3 | 1 | 0 | 0 | **5** |
| L3 记忆管理层 | 0 | 0 | 1 | 0 | 0 | **1** |
| L4 工具编排层 | 2 | 6 | 4 | 0 | 1 | **13** |
| L5 领域能力层 | 0 | 2 | 5 | 1 | 0 | **8** |
| L6 基础设施层 | 2 | 7 | 4 | 4 | 6 | **23** |

**热点**: L6 基础设施层 (23) + L4 工具编排层 (13) = 36 条 (72%), 跟 Loom "durable execution + tool dispatch" 核心定位一致。
**冷点**: L3 记忆管理层 (1) — Loom 不重做 RAG, 留给 sub-agent SDK。

### 6.3 跨模块/跨层 的"双归属"决策 (5 条)

| 决策 | 主模块 | 副模块 | 主层 | 副层 |
|---|---|---|---|---|
| D1 JSON-RPC 2.0 envelope | M1 | M5 | L1 | L4 |
| D3 拒代理 LLM | M5 | M8 (governance) | L1 | L6 |
| D4 Tool annotations ↔ Risk | M5 | M8 (governance) | L4 | L6 |
| P0-5 Agent.run_sync 3 形态 | M1 | — | L1 | — |
| P0-16 JSON-RPC 2.0 envelope (跟 D1 重) | M1 | M5 | L1 | L4 |

---

## 7. 关键观察 (对 Loom 架构设计的启示)

### 7.1 Loom 8 模块的"自检"

| 模块 | 调研覆盖度 | Loom 实施状态 (per docs/CURRENT.md) | 差距 |
|---|---|---|---|
| M1 请求预处理 | 3 条 (薄) | Slice 1 Mode Router 已做 (route command) | 中 — 完整 run_sync/stream/run 待 S2-W1 |
| M2 意图理解 & 任务规划 | 5 条 (中) | Slice 1 普通对话 vs Agent 模式区分已做, Coordinator+specialists 待 S2-W3 | 大 — Coordinator 是 S2 核心 |
| M3 Skill 管理 | 7 条 (中) | .loom-drafts/skills/ 是 v3-deepdive 提出, 待 S2-W5 实施 | 大 — Phase 1 不做完整 skill registry, Phase 2 增补 |
| M4 记忆引擎 | 2 条 (薄) | Loom 不重做 RAG, 留给 sub-agent SDK | **设计差异** — Loom = view layer, 不抢 |
| M5 工具调度 | 15 条 (厚) | Bridge v1.1 是 S2-W1, 工具注册中心待 S2-W5 | 中 — 5 必决 + 5 P0 + 4 P1 集中在这 |
| M6 执行沙箱 | 8 条 (中) | Event Journal 已做, durable engine 选型待 Phase 3 | **设计差异** — Loom 选 1 个接 (Restate/DBOS) 不重做 |
| M7 结果校验 & 迭代 | 3 条 (薄) | Result 校验 + replay 模式待 S2-W3 | 中 — Replay-based resume 是 P0-9 |
| M8 运营管控 | 13 条 (厚) | OTel trace 出口 + $USD budget 待 S2-W4 | 中 — 4 P0 + 4 P2 集中在这 |

### 7.2 Loom 6 层的"自检"

| 层 | 调研覆盖度 | 实施状态 | 差距 |
|---|---|---|---|
| L1 接入网关层 | 6 条 | Bridge v1.1 = JSON-RPC 2.0 envelope (D1 必决) | 小 — S2-W1 核心 |
| L2 意图与规划层 | 5 条 | Coordinator+specialists 待 S2-W3 | 中 — P0-1/3/7 在这 |
| L3 记忆管理层 | 1 条 | Loom 不重做, 留给 sub-agent SDK | **设计差异** |
| L4 工具编排层 | 13 条 | 工具注册中心 + 权限风控 + 调度器待 S2-W5 | 大 — 核心建设层 |
| L5 领域能力层 | 8 条 | Skill 池 + Coding 增强待 Phase 2 增补 | 中 — Phase 1 不完整 |
| L6 基础设施层 | 23 条 | Event Journal 已做, durable engine 选型待 Phase 3 | **设计差异** — Loom 选 1 个接 |

### 7.3 调研"盲区"提示

- **M1 / M7 / M3 调研覆盖度低** (3 + 3 + 7), 跟 Loom "view layer" 定位相关 — 这些模块主要由 sub-agent SDK 负责, Loom 只暴露 governance 接口
- **L3 记忆管理层只 1 条** — 跟 Loom 设计原则一致 (不重做 RAG)
- **M4 记忆引擎只 2 条** — 同上

---

## 8. 引用 / 路径

### 8.1 源数据
- `.loom-drafts/community-survey-v3-deepdive.md` (45K, 主参考, P0/P1/P2 来源)
- `.loom-drafts/phase-roadmap-supplements.md` (41K, v0.4 5 必决 + 永久不吸清单)

### 8.2 关联文档
- `docs/adr/0006-fastcontext-compatible-code-analysis-sidecar.md` (FastContext 落地参考)
- `docs/integrations/fastcontext.md` (FastContext 集成规范)
- `docs/CURRENT.md` (Phase 1 真实状态, Slice 1 PASS / S2-W1 contract frozen / review PASS)
- `docs/architecture/c4-*.md` (C4 架构图, Layer 对应)

### 8.3 本次 mapping 文档
- `.loom-drafts/loom-survey-by-module-layer-2026-07-25.md` (本文件)
- 配套: `.loom-drafts/loom-survey-report-2026-07-25.md` (按"5 类 × 8 文档"全景)
- 配套: `.loom-drafts/loom-graph-engineering-plan.md` (S2 DAG, 16 task, 待 freeze)

---

**END of Module/Layer Mapping** — 50 条 / 8 模块 / 6 层 / 2026-07-25 03:08 SGT
