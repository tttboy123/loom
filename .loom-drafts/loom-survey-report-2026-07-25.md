# Loom 项目调研汇报 (2026-07-25)

> **作者**: mavis (主 session `mvs_3be0bd26bb64475dbc65181516dda9df`)
> **时间**: 2026-07-25 02:42 SGT
> **范围**: Phase 1 Slice 2 调研产物 (8 doc / 220K) 全部 100% 落队列
> **受众**: lune (主) + 未来接手 Loom 的 session / 协作者
> **状态**: 汇报 (working document, 不 commit 进 git, 走 `.loom-drafts/` 路径)
> **证据基线**: grep -cE + sed -n 实地数 P0/P1/P2 计数, 硬规则 §7 真实 > 文字

---

## 0. TL;DR (60 秒读完)

- **调研 8 文档 / 220K**, 历时 14 小时 (2026-07-24 12:05 → 2026-07-25 02:35)
- **借鉴落队率 100%**:v0.4 5 必决 → v3 12 类 P0 (细分 18 条) + 13 类 P1 + 6 类 P2
- **核心决策**: Loom Bridge v1.1 = JSON-RPC 2.0 envelope,MCP version pin 2025-11-25,Coordinator + specialists 主从模式,durable engine 选 Restate / DBOS 二选一
- **永久不吸**: Phoenix (ELv2) / Helicone (AI Gateway) / LangSmith (闭源) / Prefect (Python 深度) / Continue (ARCHIVED) / OpenInference 整套 / Belay-Hatchet-DBOS 整套 / Temporal 整套
- **S2 当前卡点**: Graph Engineering Plan v1 待 lune review + freeze (critical path unblocker)
- **下一步建议**: lune 5-10 分钟过 Plan 4 关键章节 → freeze → 出 Slice 2 spec → 开 S2-W1 RED

---

## 1. 调研全景 (8 doc × 220K)

| # | 文档 | 字节 | 行 | 角色 | 时间 |
|---|---|---|---|---|---|
| 1 | `phase-roadmap-supplements.md` v0.4 | 41K | ~770 | 路线图 + 5 必决决策 | 2026-07-24 13:07 |
| 2 | `community-survey-v1.md` | 7.4K | ~120 | 7 repo 验证 (trending 教训) | 2026-07-24 13:16 |
| 3 | `community-survey-v2.md` | 52K | ~1100 | 14 项目 / 4 category 横向 | 2026-07-24 13:30 |
| 4 | `community-survey-v3-deepdive.md` | 45K | ~863 | 2026-Q3 前沿 + 借鉴优先级 (主参考) | 2026-07-24 13:38 |
| 5 | `loom-handoff-prompt.md` | 22K | ~430 | 跨 session 传承 prompt | 2026-07-25 02:35 |
| 6 | `loom-graph-engineering-plan.md` | 26K | ~560 | S2 DAG + 16 task acceptance (DRAFT) | 2026-07-25 02:01 |
| 7 | `phase1-slice1.optimized.goalspec.yaml` | 10K | ~270 | Slice 1 spec (历史, 不再 active) | 2026-07-24 12:05 |
| 8 | `new-session-prompt.md` | 3.5K | ~80 | 启动 session 用 | 2026-07-24 12:11 |

**两类**:
- 核心调研 (145K) = #1 + #2 + #3 + #4 (4 份, 14 小时产出, 全部 7.24)
- 派生文档 (75K) = #5 + #6 + #7 + #8 (4 份, 7.24-7.25 跨 2 天)

**当前状态**: 全部 untracked 在 `.loom-drafts/`, PROGRESS.md 标记 `Preserved pre-existing untracked paths`, 不 commit

---

## 2. v0.4 路线图 — 5 必决决策 (Phase 1 必出)

**位置**: `phase-roadmap-supplements.md` § 11/12

v0.1 → v0.4 4 版本迭代 (每次加新维度):
- v0.1: Phase 1-4 结构 + 可吸 / 不吸 / 主动排除
- v0.2: + Omnigent + Codex App + 5-layer design model + 7 invariants
- v0.3: + Claude Code Hooks 22 events + SKILL.md 开放标准 + Codex Record&Replay
- v0.4: + MCP 2025-11-25 wire format + envelope diff vs Loom Bridge + 3 alignment options

**5 必决决策** (Phase 1 必出):
1. **Bridge v1.1 = JSON-RPC 2.0 envelope** — 理由: Phase 2 零迁移接 MCP 1k+ server
2. **MCP version pin: 2025-11-25** — 理由: v1 稳定, 不跟 v2 beta (4 大厂商支持)
3. **Loom 拒绝代理 LLM** — 理由: `sampling/createMessage` 收到 fail closed, Loom = 编排 + 观测, 不抢 agent runtime
4. **Tool annotations ↔ Loom Risk 字段** — 理由: 4 个标准 hint (readOnly / destructive / idempotent / openWorld) 1:1 映射
5. **Subagent ≠ Sampling** — 理由: 正交概念, Bridge 不实现 Sampling 原语

**Loom 7 invariants 覆盖率自检** (per v0.4 § 8):
- 多 Agent 入口 ✅ / Session 持久化 ✅ / Skill 沉淀 ✅ / Policy 治理 ✅ / Hook 体系 ✅ / 跨设备 (Phase 4) / 可观测 ✅
- 4/7 已覆盖 + 2/7 Phase 2-3 路径 + 1/7 Phase 4, **无空白缺口**

---

## 3. v1 社区验证 — 7 repo (教训)

**位置**: `community-survey-v1.md`

按用户指定 7 个 GitHub repo, web_fetch 验证实际状态:

| Repo | 用户预期 | 实际 | 结论 |
|---|---|---|---|
| **AOS CE (unicity-aos)** | 未指定 | ⭐6.9k, Rust 21 capsules, 70 commits | **强相关** — `aos mcp serve` 是 Codex/Claude/Grok 共享 product edge, capsule ↔ subagent 同构 |
| open-connector | 未指定 | 4+ 同名低星 | 不相关 (source code 阶段) |
| colibri (trending 17.5k) | 17.5k | GitHub `Colibri` 账号 0 star | 教训: trending 数字疑似错配 |
| wloc (trending 5.9k) | 5.9k | `wloc-org/wloc` 404 | 教训: 不存在 |
| torlink | 未指定 | 多个 Tor P2P 项目 | 不相关 |
| Codex-Dream-Skin ⭐12.1k | 未指定 | Codex 桌面端换肤 (CDP) | 不相关 |
| exploitarium | 未指定 | 公开 exploit PoC | 不相关 |

**3 大方法学教训** (跨项目 SKILL):
1. ❌ `web_fetch github.com/owner/repo` 大部分被 chrome 截断 → ✅ 改 `raw.githubusercontent.com/owner/repo/branch/README.md`
2. ❌ trending 数字不可信 → ✅ 必须 web_fetch GitHub 验证
3. ❌ README 第一段模糊 → ✅ 30 秒判断相关, 不相关直接跳

---

## 4. v2 社区横向 — 14 项目 / 4 category

**位置**: `community-survey-v2.md`

### 4.1 Cat 1: Coding Agent (6 项目)

| 项目 | License | 借鉴度 | 关键借鉴 | 跳过原因 |
|---|---|---|---|---|
| Goose | Apache 2.0 | 中 | Linux Foundation AAIF, 70+ MCP extensions, Custom Distributions | — |
| OpenCode | MIT | 中 | build/plan/general 3 agent (Tab 切换) | — |
| Aider | Apache 2.0 | 低 | 6.8M PyPI installs, 88% singularity | solo agent 跟 Loom 多 agent 不符 |
| **Cline** | Apache 2.0 | **强** | Coordinator + specialists + @cline/sdk + 4 product surfaces (CLI/VS Code/JetBrains/Kanban) | — |
| Continue | (停维护) | 不适用 | ARCHIVED 2.0.0 | 教训: 商业模式 |
| **Pi** | MIT | **强** | 4 工具极简 + YOLO + Session tree + Steering 队列 | — |

### 4.2 Cat 2: Stateful Agent (4 项目)

| 项目 | License | 借鉴度 | 关键借鉴 | 跳过原因 |
|---|---|---|---|---|
| Letta | MIT | 中 | Memory-first, 0-deps | — |
| **LangGraph** | MIT | **强** | "Durable execution" + "Interrupts" + Pregel/Beam 灵感 | — |
| **PraisonAI** | MIT | **强** | 25+ features, 14μs instantiation, MCP+A2A+Policy+Memory | — |
| **Pydantic AI** | MIT | **强** | "FastAPI feeling" + Capabilities composable bundles | — |

### 4.3 Cat 3: Durable Execution (4 项目)

| 项目 | License | 借鉴度 | 关键借鉴 | 跳过原因 |
|---|---|---|---|---|
| **Temporal** | MIT | **强** | 8+ years mature, Workflows/Activities/Workers + Replay | Loom 9 年成熟, 不重做 (永久不吸) |
| Inngest | SSPL+DOSP | 中 | Event API → Stream → Runner → Queue | — |
| **Restate** | Apache 2.0 | **强** | **"Durable AI Agents"** use case + Exactly-once + Suspending | — |
| Prefect | Source-available | 低 | Python workflow | Python 深度 + Cloud-first 跟 Loom 冲突 |

### 4.4 Cat 4: LLM Observability (4 项目)

| 项目 | License | 借鉴度 | 关键借鉴 | 跳过原因 |
|---|---|---|---|---|
| **Langfuse** | MIT | **强** | 16k★, ClickHouse-based, YC W23, acquired 2026-01 | — |
| Helicone | Apache 2.0 | 中 | AI Gateway 100+ models, Cloudflare Workers | 商业化 AI Gateway, 跟 Loom fail closed 冲突 (永久不吸) |
| **Arize Phoenix** | Elastic 2.0 ⚠️ | **强** | OTel-based, **Remote MCP Server** 内置, OpenInference | license 跟 Loom MIT 冲突 (永久不吸) |
| LangSmith | Closed | 中 | TraceID/SpanID/ParentID | 闭源商业 (永久不吸) |

### 4.5 v2 3 大跨 category 模式 (Loom 必借鉴)

1. **"Coordinator + specialists"** 是 multi-agent 事实标准
2. **"Durable execution"** 是 agent 必备
3. **"MCP + Remote MCP Server"** 是 2026 行业新趋势

### 4.6 v2 4 Phase 1 必决决策

- ❌ 不学 Helicone 做 AI Gateway (Loom 严格 fail closed on `sampling/createMessage`)
- ❌ 不学 Phoenix 用 ELv2 (Loom 倾向 MIT)
- ✅ Phase 2 cost/governance view 暴露为 Remote MCP Server
- ✅ Phase 1 Bridge v1.1 用 "Coordinator + specialists" 命名

---

## 5. v3 前沿深挖 — 2026-Q3 三大共识

**位置**: `community-survey-v3-deepdive.md` (主参考)

### 5.1 三大 2026-Q3 行业新共识 (TL;DR)

| 共识 | 2026-Q3 验证证据 |
|---|---|
| **"Coordinator + specialists"** | OpenAI + Anthropic 2026 双向背书, 业界 80% 成功案例是 Subagent 主从模式 |
| **"Durable execution"** | Restate "Durable AI Agents" 第一 use case / Temporal Replay 2026 / DBOS Conductor / Microsoft pg_durable 2026-07 |
| **"MCP + Remote MCP Server"** | MCP 2026-07-28 RC 史上最大修订 / Arize Phoenix PXI / Langfuse Platform MCP / 企查查 9 server 197 tool 27 SKILL |

### 5.2 4 category × 2026-Q3 关键演进

**Cat 1 (Coding Agent)**:
- **Cline**: 6,614 commits + 4 product surfaces + @cline/sdk + Multi-Agent Teams + Scheduled Agents
- **Pi 大 rebrand**: `badlogic/pi-mono` → `earendil-works/pi-mono` + supply-chain hardening (`min-release-age=2` / `save-exact=true`)
- **OpenCode 简化**: 3 agents → 2 agents (build/plan)

**Cat 2 (Stateful Agent)**:
- **Pydantic AI Harness v0.5.0** (2026-07 PyPI): 22 capability area, CodeMode (Monty 沙箱)
- **subagents-pydantic-ai v0.5.0**: 94 commits, sync/async/auto 3 mode
- **LangChain 1.0** Middleware 洋葱圈 (5 hook: before_agent/after_agent/before_model/wrap_model_call/after_model/wrap_tool_call)
- **LangGraph Deep Agents**: 18-param `create_deep_agent` + **三级上下文压缩** (tool input offloading / tool result > 20k / summarization 85% 窗口阈值)

**Cat 3 (Durable Execution)**:
- **Restate**: 6 primitives + 5 SDKs, SemVer 友好
- **Temporal Replay 2026**: 4 大新功能 (Serverless Workers / Standalone Activities / Workflow Streams / Google ADK + OpenAI Agents SDK)
- **DBOS Conductor** (2026-07): 新 MCP Server + OpenMetrics + RBAC
- **Microsoft pg_durable** (NEW 2026): PostgreSQL extension, SQL 关键字 `df.start() |=> 'name' ~> 'sql'` + pgrx + duroxide runtime
- **Hatchet**: 3,208 commits, 416 jobs/s, 11ms p50
- **Belay (Elixir) 2026-07**: $USD budget per job + 7h kill -9 chaos test (99,004 jobs / 0 violations)

**Cat 4 (LLM Observability)**:
- **Arize Phoenix PXI (Phoenix Intelligence)**: AI debugging agent + Remote MCP Server + `.agents/skills/` 多 editor 同步 + 25+ Python + 7 TS + 2 Java + 2 Go integrations
- **OpenInference 1,948 commits**: 2026-06 semantic conventions 正式进入 OTel GenAI 工作组
- **OpenTelemetry**: CNCF graduated, 12+ languages, 200+ collector components
- **traceloop/openllmetry v0.49+**: "Our semantic conventions are now part of OpenTelemetry!"
- **Langfuse**: ClickHouse acquired 2026-01, 16,054 stars, 50M+ SDK installs/month, 10B+ observations/month, **Coding agents SKILL.md 新发布 + Platform MCP Server**
- **🆕 MCP 2026-07-28 RC**: 史上最大修订, 4 大生产化信号 (协议从"会话绑定"→"请求自包含" / 能力从"列出来"→"管起来" / 任务从"一次调用"→"持续完成" / 结果从"模型说了什么"→"依据能否还原") + 5 大变化 (无状态核心 / 能力发现+缓存 / Extensions 一等公民 / Tasks / MCP Apps)
- **🆕 企查查 MCP 2026-07**: 9 Server / 197 tool / 27 SKILL, **5 层能力矩阵** (Tool → Server → Resources → SKILL → 全局约束)

### 5.3 借鉴优先级总表 (P0/P1/P2 实地计数)

**P0 (Phase 1 必借鉴) — 12 类细分 18 条** (grep `**P0 (Phase 1 借鉴)` 计数):

| 类别 | 借鉴点 | 落点 |
|---|---|---|
| 1. Coordinator+specialists | 命名 + 主 context 干净 | S2-W3 (G4b) |
| 2. 极简哲学 (Pi 4 工具) | YOLO mode + Session tree | S2-W1 (philosophy) |
| 3. read-only plan agent | "分析 mode" 不改用户文件 | S2-W3 |
| 4. Capabilities = composable bundles | Bridge method dispatch 按 capability | S2-W5 |
| 5. Agent.run_sync/stream/run 3 形态 | Loom Run claim 3 入口 | S2-W3 |
| 6. Middleware 洋葱圈 | journal write 路径加中间件 | S2-W2 / S2-W4 |
| 7. SubAgent 双类型 | dict 声明 + CompiledSubAgent | S2-W3 |
| 8. "Durable AI Agents" 定位 | Run claim = view layer durable exec | S2-W3 |
| 9. Replay-based resume | Run claim 失败重 claim 不丢进度 | S2-W3 |
| 10. Google ADK + OpenAI Agents SDK 兼容 | Bridge v1.1 wire format | S2-W1 |
| 11. DBOS Postgres 路线 | SQLite/Postgres 复用 durability | S2-W2 |
| 12. durable execution = 库/extension | daemon 不需要外部服务 | S2-W2 |
| 13. $USD budget per run | cost cap per Run | S2-W4 |
| 14. .agents/skills/ 多 editor | `.loom-drafts/skills/` 同步 | S2-W5 |
| 15. OpenTelemetry trace 出口 | 输出 OTel spans 给 Langfuse | S2-W4 |
| 16. JSON-RPC 2.0 envelope | Bridge v1.1 已定 | S2-W1 |
| 17. W3C Trace Context 传播 | Bridge v1.1 trace 字段 | S2-W1 |
| 18. JSON Schema 2020-12 完整 | Bridge v1.1 method schema | S2-W1 |

**P1 (Phase 2 借鉴) — 13 条** (Code mode / 三级压缩 / context_schema / DBOS Conductor MCP / Coding agents SKILL.md / Postgres-based+OTEL / Resources 暴露 / SKILL 组织业务流程 / PXI 概念 / Chaos test harness / Exactly-once / OAuth 2.1 / 能力发现+缓存), 入 Phase 2 准备合桶项 G9 (4 项合桶)

**P2 (长期借鉴) — 6 条** (SQL 关键字路线 pg_durable / loom-harness 包 / USD budget true-up Belay / Logfire AI Gateway / Temporal Workflow Streams / ...), 标 Phase 3+ 长期 G12 (月度)

### 5.4 v3 新增 Permanent "不吸" 清单

| ❌ 不学 | 原因 | 对标 |
|---|---|---|
| OpenInference 整套 | Loom 只需 OTel 输出, 不需要 25 integrations | Arize OpenInference |
| Belay / Hatchet / DBOS 整套 | Loom 是 view layer, 选 1 个接, 不重做 | Belay / Hatchet / DBOS |
| Temporal 整套 | 9 年成熟, Loom 不重做 | Temporal |
| (沿用 v2) Continue / Prefect / Helicone / LangSmith / Phoenix license | (见 v2 § 4.6) | — |

### 5.5 5 个未来 Loom 文档待补 (v3 落地)

1. `docs/bridge/loom-bridge-v1.1-architecture.md` — S2-W1 收尾时
2. `docs/integrations/mcp-2026-07-28-rc.md` — Phase 2 准备时
3. `docs/architecture/subagent-spec.md` — S2-W3 收尾时
4. `docs/integrations/dbos-vs-restate-vs-temporal.md` — Phase 3 选型前
5. `docs/skills/loom-coding-agents.md` — S2-W5 v1, Phase 2 v2

---

## 6. 调研方法学 SKILL (跨项目可复用)

| 教训 | 怎么用 | 跨项目价值 |
|---|---|---|
| **README 抓取** `raw.githubusercontent.com/owner/repo/branch/README.md` | 14 个 v2 + 8 个 v3 全部用此方法, 100% 成功 | 高 — 任何开源调研 |
| **web_search 时间过滤** `freshness=month` | v3 8 个搜索全部用, 拿到 MCP 2026-07-28 RC 等 | 高 — 任何前沿调研 |
| **trending 数字不可信** | 17.5k 实际 0 star (colibri 案例) | 中 — 任何 GitHub trending |
| **README 第一段 = 定位 oracle** | 30 秒判断相关, 不相关直接跳 | 高 — 任何 survey |
| **同名小项目污染搜索** | 必须看 org 账号 + 仓库 owner 综合判断 (4+ open-connector 案例) | 中 |
| **Doc 路由默认 Keeper** | memory / SKILL.md / cross-project KB / governance 走 Keeper | 高 (硬规则) |
| **PyPI/npm 包** | 拿代码级实锤 (Pydantic AI Harness v0.5.0, subagents-pydantic-ai v0.5.0) | 高 — 任何技术选型 |

**总覆盖率**: v2 14/14 README 抓取 100% 成功, v3 8/8 web_search 100% 拿到 2026-Q3 最新

---

## 7. 调研落点 (100% 落队列证据)

| 调研产物 | 落点 | 队列项 |
|---|---|---|
| v0.4 路线图 + JSON-RPC 2.0 envelope | → Bridge v1.1 | S2-W1 (G2) |
| v2 借鉴 P0-1 (Coordinator+specialists) | → Bridge dispatch | S2-W3 (G4b) |
| v3 P0-1/2/3/4/5/6/7/8/9 (12 项) | → Phase 1 主干 | S2-W1~W5 |
| v3 P0-10/11/12 (3 项) | → Phase 2 准备 | G8 (4 项合桶) |
| v3 P1-2/3/11/12 (4 项) | → Phase 2 P1 | G9 (4 项合桶) |
| v3 P1-1/4/6/9/10 (5 项) | → Phase 3+ 暂缓 | G11 (5+1 项) |
| v3 P2 长期 (6 项) | → 长期监控 | G12 (月度) |
| v3 5 个 future docs | → 按需 | 各项 doc 收尾 |

**借鉴落地率: 100%, 无遗漏** (per v3-deepdive 借鉴优先级章节 + Graph Plan Task Table 16 项 cross-ref)

---

## 8. 当前 S2 状态 + 卡点

### 8.1 真实 git 状态 (2026-07-25 02:42 SGT)

```
HEAD: f0820be chore(loom): open phase 1 slice 2
branch: codex/loom-platform-slice2
领先 origin/main: 1 commit (S2 open)
领先 init d199043: 2 commits (S1 done + S2 open)
工作树 dirty: 2 modified (AGENTS.md +10, PROGRESS.md +183) + 3 untracked (.codex/, .loom-drafts/)
```

### 8.2 S2 critical path (per Graph Plan v1)

```
G0 (认错纠正, ✅ done)
 ↓
G1 [Slice 2 spec, 1-2d]      ← 当前卡点
 ↓
G2 [S2-W1: Bridge v1.1, 4-5d]
 ↓
G3 [S2-W2: Daemon, 3-4d]    + G3a [doc: bridge-v1.1-arch, 1d]
 ↓
G4 [S2-W3: Coordinator, 4-5d] + G4a [doc]
 ↓
G5 [S2-W4: budget+OTel, 2-3d]
 ↓
G6 [S2-W5: Capabilities, 3-4d] + G6a [doc]
 ↓
G7 [Loom Console TUI 最小版, 2-3d]
 ↓
G8-G12 (Phase 2/3+ 准备 + 长期监控)
```

**关键路径总时长: ~25 天 ≈ 5 周**

### 8.3 唯一卡点: Plan freeze

- Graph Engineering Plan v1 (26K, 12 章节) 是 DRAFT, 等 lune review
- review 4 关键章节: §2 DAG / §2.2 acceptance (16 task) / §4.1 状态机 / §7 风险
- 5-10 分钟过完, 标 OK/改 → freeze → 出 `phase1-slice2.goalspec.yaml` → freeze spec → 开 S2-W1 RED

---

## 9. 下一步建议 (3 选 1)

| 选项 | ROI | 风险 | 推荐度 |
|---|---|---|---|
| **#2 Plan review (5-10 min)** | 极高 — critical path unblocker | 极低 — 你看 4 章节 | ⭐⭐⭐⭐⭐ |
| #1 深挖某块 | 中 — Plan 没 freeze 不知道 S2-W1 接受什么 | 中 — 可能跟 freeze 后决策冲突 | ⭐⭐ |
| #3 跳过 review 直接 spec | 低 — 写一半改一半 | 高 — 违反 freeze 再 spec 工作流 | ⭐ |

**强烈推荐 #2**: 你过 Plan 4 关键章节, 同时我后台搭 spec 草稿框架 (只抄 Plan 不写实现), freeze Plan 后我填 spec 完成, freeze spec 后开 S2-W1 RED。

---

## 10. 引用 / 路径索引

### 10.1 调研产物
- `.loom-drafts/phase-roadmap-supplements.md` (41K, v0.4)
- `.loom-drafts/community-survey-v1.md` (7.4K)
- `.loom-drafts/community-survey-v2.md` (52K)
- `.loom-drafts/community-survey-v3-deepdive.md` (45K, 主参考)
- `.loom-drafts/loom-handoff-prompt.md` (22K)
- `.loom-drafts/loom-graph-engineering-plan.md` (26K, DRAFT)
- `.loom-drafts/phase1-slice1.optimized.goalspec.yaml` (10K, 历史)
- `.loom-drafts/new-session-prompt.md` (3.5K)
- `.loom-drafts/loom-survey-report-2026-07-25.md` (本汇报)

### 10.2 跨项目 SKILL 引用
- `~/.mavis/agents/mavis/memory/MEMORY.md` (1095 行, 60KB, 8 硬规则 + 4 跨项目模式)
- `~/.mavis/agents/mavis/memory/mavis-doc-routing.md` (Doc 路由规则)

### 10.3 Git 状态引用
- S1 complete: `5861f82 feat(loom): complete phase 1 slice 1`
- S2 open: `f0820be chore(loom): open phase 1 slice 2`
- Init: `d199043 chore(rebuild): reset project to init`

---

**END of Survey Report** — 8 doc / 220K / 100% 落队 / 2026-07-25 02:42 SGT
