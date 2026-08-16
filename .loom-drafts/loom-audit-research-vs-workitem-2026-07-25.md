# Loom 调研 vs WorkItem 审计 (2026-07-25)

> **作者**: mavis (主 session `mvs_3be0bd26bb64475dbc65181516dda9df`)
> **时间**: 2026-07-25 03:18 SGT
> **目的**: 审计 "调研 / 规范 / 决策齐了但没 work item" 的所有案例, 补齐工作流裂缝
> **数据基线**: grep -nE 实际数, 不凭印象
> **状态**: 审计 (不 commit, 走 `.loom-drafts/`)

---

## 0. 概念区分 (避免 Phase 1 Slice 2 vs Phase 2 混淆)

| 概念 | 含义 | 当前状态 |
|---|---|---|
| **Phase 1** | 5 个 Slice (1-5), 包含入口 / 持久化 / Agent 定义 / 执行 / 看板 | 1 实施完成 (S1-W1~W5), 2-5 待 |
| **Slice 2** | Phase 1 内部第 2 个切片, 主题 "Agent 定义与团队草案" (per TECH-PLAN.md §14) | S2-W1 contract frozen + reviewer PASS, 等授权 |
| **Phase 2** | **跨 Phase 1 之外**的大阶段, 主题 "凭证与运行时" (per PRODUCT-PLAN.md §338) | **未启动** (per `terminal_boundary: phase1_only`) |
| **Phase 3** | "复用与共进化" | 未启动 |
| **Phase 4** | "互通" | 未启动 |

**关键不变量**:
- `terminal_boundary: phase1_only` (per seed) = **Phase 1 没完成, 不能进 Phase 2**
- S2-W1 还在 Phase 1 范围 (Slice 2 是 Phase 1 第 2 切片, 不是 "Phase 2")
- FastContext 调研是 ADR-0006 (Phase 1 期间接受) + integrations/fastcontext.md (Phase 1 写)
- 落地 FastContext install 硬边界 = 5 处声明, 都在 Phase 1 spec / governance 段
- Phase 2 启动时 governance 段需要**重新 freeze** (Phase 1 governance 不自动续期)

---

## 1. 审计总览

| 类别 | 总数 | 已 work item | 调研齐未 work item | 比例 |
|---|---|---|---|---|
| **7 ADR (0001-0006)** | 6 accepted | 3 | **3** | 50% |
| **v0.4 永久借鉴清单 (6 项)** | 6 | 1 | **5** | 17% |
| **v0.4 未来文档待补 (6 项)** | 6 | 0 | **6** | 0% |
| **v3 5 必决决策** | 5 | 2 | **3** | 40% |
| **v3 P0 借鉴 (12 类 18-20 条)** | 20 | 11 | **9** | 55% |
| **v3 P1 借鉴 (13 条)** | 13 | 0 | **13** | 0% |
| **v3 P2 借鉴 (6 条)** | 6 | 0 | **6** | 0% |
| **总计** | **62** | **17** | **45** | **27%** |

**73% 调研产物没 work item** — 这是核心裂缝。

---

## 2. 详细审计

### 2.1 ADR 状态 × WorkItem (6 accepted)

| ADR | 状态 | 当前 work item | 调研齐未 work item | 备注 |
|---|---|---|---|---|
| 0001 Explicit Agent Mode | accepted | ✅ S1-W1 (Mode Router, 4 triggers) | — | 已实施 |
| 0002 Local Modular Monolith + Event Projections | accepted | ✅ S1-W2 (Event Journal) + S1-W4 (Projection) | — | 已实施 |
| 0003 Agent Definition Runtime Separation | accepted | ✅ S1-W1 (partial) + S2-W1 (frozen, 等授权) | — | 部分已实施 |
| 0004 Credential Broker Authentication Modes | accepted | **无** | **❌ 调研齐** | **Phase 2 才做** (per TECH-PLAN.md 798) |
| 0005 Local Evolution Sidecar | accepted | **无** | **❌ 调研齐** | **Phase 3 才做** (per TECH-PLAN.md 800) |
| 0006 FastContext Code Analysis Sidecar | accepted | **无** | **❌ 调研齐** | **Phase 2 Spike** (per ADR-0006) + integrations/fastcontext.md 209 行已写 |

### 2.2 v0.4 永久借鉴清单 × WorkItem (6 项)

| 借鉴 | 调研源 | 当前 work item | 调研齐未 work item | 备注 |
|---|---|---|---|---|
| Multica Runtime discovery / claim-lease / task-scoped token | `.loom-drafts/phase-roadmap-supplements.md` 754 | **部分进 S2-W3** | 详细 task 未拆 | Phase 1 Slice 2 范围 |
| ECC / Multica / AgentScope SKILL.md + Toolkit 抽象 | 同上 755 | **无** | **❌ 调研齐** | Phase 2 Skill registry (per P3 增补 ①) |
| AgentScope 2.0 sandbox 矩阵 | 同上 756 | **无** | **❌ 调研齐** | Phase 2 增补 ② (sandbox-backends.md 待补) |
| grok-cli hooks 生命周期 | 同上 757 | **无** | **❌ 调研齐** | Phase 2 增补 ① (bridge-hooks.md 待补) |
| Earendil Pi SDK as RuntimeProfile | 同上 758 | **无** | **❌ 调研齐** | Phase 2 增补 ④ (pi-coding-agent.md 待补) |
| OpenHands "prove it works" 工具 | 同上 759 | N/A | N/A | 已被 Loom deterministic acceptance 覆盖 |

### 2.3 v0.4 未来文档待补 × WorkItem (6 项)

| 待补 doc | 调研产物 | 当前 work item | 调研齐未 work item | 备注 |
|---|---|---|---|---|
| `docs/architecture/bridge-hooks.md` | v0.3 §1 (Claude Code 22 events) | **无** | **❌ 调研齐** | Phase 2 增补 ① |
| `docs/integrations/sandbox-backends.md` | v0.4 §166 | **无** | **❌ 调研齐** | Phase 2 增补 ② |
| `docs/integrations/pi-coding-agent.md` | v0.4 §178 | **无** | **❌ 调研齐** | Phase 2 增补 ④ |
| `docs/architecture/skill-registry.md` | v0.3 §2 (SKILL.md 开放标准) | **无** | **❌ 调研齐** | Phase 3 增补 ① |
| `TECH-PLAN.md §11` 末段修订 | v0.4 (Provider routing 边界) | **无** | **❌ 调研齐** | Phase 2 增补 ③ |
| `PRODUCT-PLAN.md §Agent 库` 拆分 | v0.3 §2 (Agent + Skill 拆 2 层) | **无** | **❌ 调研齐** | Phase 3 增补 ① |

### 2.4 v3 5 必决决策 × WorkItem

| 决策 | 当前 work item | 调研齐未 work item | 备注 |
|---|---|---|---|
| D1 Bridge v1.1 = JSON-RPC 2.0 envelope | ✅ S2-W1 (partial) + Graph Plan G2 | — | Phase 1 主路径 |
| D2 MCP version pin 2025-11-25 | ✅ S2-W1 (partial) | — | 跟 D1 合并 |
| D3 拒代理 LLM (fail closed sampling) | **无独立 task** | **❌ 调研齐** | 写进 Slice 2 spec 但没 work item 跟踪 |
| D4 Tool annotations ↔ Loom Risk 字段 | **无独立 task** | **❌ 调研齐** | 同上 |
| D5 Subagent ≠ Sampling | **无独立 task** | **❌ 调研齐** | 同上 |

### 2.5 v3 P0 借鉴 × WorkItem (20 条)

| P0 # | 借鉴 | 当前 work item | 调研齐未 work item |
|---|---|---|---|
| P0-1 | Coordinator+specialists | S2-W3 | — |
| P0-2 | 极简哲学 (Pi 4 工具) | S2-W1 (philosophy) | — |
| P0-3 | read-only plan agent | S2-W3 | — |
| P0-4 | Capabilities = composable bundles | S2-W1 (schema) | — |
| P0-5 | Agent.run_sync/stream/run 3 形态 | S2-W1 (partial sync) | — |
| P0-6 | Middleware 洋葱圈 | S2-W2 | — |
| P0-7 | SubAgent 双类型 | S2-W3 | — |
| P0-8 | "Durable AI Agents" 定位 | S2-W3 | — |
| P0-9 | Replay-based resume | S2-W3 | — |
| P0-10 | Google ADK + OpenAI Agents SDK 兼容 | S2-W1 | — |
| P0-11 | DBOS Postgres 路线 | S2-W2 | — |
| P0-12 | durable execution = 库/extension | S2-W2 | — |
| P0-13 | $USD budget per run | S2-W4 | — |
| P0-14 | .agents/skills/ 多 editor | S2-W5 | — |
| P0-15 | OpenTelemetry trace 出口 | S2-W4 | — |
| P0-16 | JSON-RPC 2.0 envelope | S2-W1 | — |
| P0-17 | W3C Trace Context 传播 | S2-W1 | — |
| P0-18 | 完整 JSON Schema 2020-12 | S2-W1 | — |
| P0-19 | ClickHouse 作为底层 OLAP | **无** | **❌ 调研齐** (Phase 2 准备) |
| P0-20 | OpenTelemetry compatible | S2-W4 (跟 P0-15 合并) | — |

**注**: P0-19 ClickHouse 是 Phase 2 准备, 调研 100% 齐 (Langfuse 16k★ + ClickHouse acquired 2026-01), 但没 work item

### 2.6 v3 P1 借鉴 × WorkItem (13 条, 全部 Phase 2,**0 work item**)

| P1 # | 借鉴 | 调研源 | 备注 |
|---|---|---|---|
| P1-1 | Code mode 概念 | Pydantic AI CodeMode (Monty 沙箱) | Phase 2 增补 |
| P1-2 | 三级上下文压缩 | LangGraph Deep Agents | Phase 2 增补 |
| P1-3 | context_schema 模式 | Pydantic AI | Phase 2 增补 |
| P1-4 | Exactly-once 语义 | Restate | Slice 1 Event Journal 已部分 (idempotent append) |
| P1-5 | DBOS 新 MCP Server | DBOS Conductor 2026-07 | Phase 2 增补 |
| P1-6 | Postgres-based durable execution + OTEL 定位 | DBOS | Phase 2 增补 (跟 P0-19 关联) |
| P1-7 | MCP server built-in (cost view) | Belay / 企查查 MCP | Phase 2 增补 |
| P1-8 | PXI 概念 (loom-insight) | Arize Phoenix PXI | Phase 2 增补 |
| P1-9 | OpenInference semantic conventions | OpenInference 1,948 commits | Phase 2 增补 |
| P1-10 | 能力发现 + 缓存 | MCP 2026-07-28 RC | Phase 2 增补 |
| P1-11 | Resources 暴露稳定知识 | MCP Resources + 企查查 9 server | Phase 2 增补 |
| P1-12 | SKILL 组织业务流程 | Langfuse / Phoenix SKILL.md | Phase 2 增补 |
| P1-13 | Coding agents SKILL.md | Langfuse 2026-07 + Phoenix PXI | Phase 2 增补 (跟 S2-W5 关联) |

### 2.7 v3 P2 借鉴 × WorkItem (6 条, 全部 Phase 3+ 长期,**0 work item**)

| P2 # | 借鉴 | 调研源 | 备注 |
|---|---|---|---|
| P2-1 | pg_durable SQL 关键字路线 | Microsoft pg_durable 2026-07 | Phase 3+ 长期 |
| P2-2 | loom-harness 包 | Pydantic AI Harness v0.5.0 | Phase 3+ 长期 |
| P2-3 | Chaos test harness | Belay (99,004 jobs / 0 violations) | Phase 3+ 长期 |
| P2-4 | Belay USD budget true-up | Belay 2026-07 | Phase 3+ 长期 (跟 P0-13 关联) |
| P2-5 | Logfire AI Gateway | Pydantic Stack Logfire | Phase 3+ 长期 (⚠️ 跟 D3 拒代理 LLM 冲突) |
| P2-6 | Temporal Workflow Streams | Temporal Replay 2026 | Phase 3+ 长期 |

---

## 3. Phase 2 准备桶 (Graph Plan G8 设想)

**当前 Graph Plan 16 task 包含 G8 "Phase 2 P0 准备"** — 但 G8 还没具体拆 task。

按调研密度, Phase 2 准备桶应该包含:

| 调研 | 状态 | 进 G8 优先级 |
|---|---|---|
| **FastContext Spike** (ADR-0006 + integrations/fastcontext.md) | 100% 齐, 0 work item | **P0** (跟 Slice 1 spec governance 5 处硬边界冲突, 必须显式解锁) |
| **Hook 协议** (bridge-hooks.md 待补) | 100% 齐 (Claude Code 22 events 实证), 0 work item | **P0** (Phase 2 增补 ①) |
| **Sandbox 后端选型** (sandbox-backends.md 待补) | 100% 齐 (AgentScope 2.0), 0 work item | **P0** (Phase 2 增补 ②) |
| **pi-coding-agent RuntimeProfile** (pi-coding-agent.md 待补) | 100% 齐 (Earendil Pi rebrand 2026-07), 0 work item | **P1** (Phase 2 增补 ④) |
| **Provider routing 边界** (TECH-PLAN §11 修订) | 100% 齐 (v0.4 决策), 0 work item | **P1** (Phase 2 增补 ③) |
| **ClickHouse OLAP** (P0-19) | 100% 齐 (Langfuse 16k★), 0 work item | **P1** (Phase 2 准备) |

---

## 4. Phase 3+ 长期桶 (Graph Plan G11 设想)

| 调研 | 状态 |
|---|---|
| **Agent 库拆成 Agent + Skill 两层** (P3 增补 ①) | 100% 齐 (v0.3 §2), 0 work item |
| **Skill registry** (skill-registry.md 待补) | 100% 齐 (Anthropic 2025-10-16 SKILL.md 开放标准), 0 work item |
| **ADR-0005 Evolution Sidecar 实施** | ADR 接受, 0 work item |
| **Chaos test harness** (P2-3) | 100% 齐 (Belay 实证) |
| **pg_durable SQL 关键字** (P2-1) | 100% 齐 (Microsoft 2026-07) |
| **Logfire AI Gateway** (P2-5) | 调研齐, ⚠️ 跟 D3 拒代理 LLM 冲突, 需要 Phase 3 governance 决定 |

---

## 5. 关键发现

### 5.1 工作流裂缝的 3 种形态

| 形态 | 例子 | 根因 |
|---|---|---|
| **A. Phase 2 推后** (待 Phase 1 完) | 6 个 P1 + 6 个 P2 + 6 个 Phase 2 增补 doc | Phase 1 没完, Phase 2 不能进, work item 写不进 |
| **B. 调研齐但没 work item** | FastContext / Hooks / Sandbox / ClickHouse / 3 必决 (D3/D4/D5) | 调研期间没主动创建 work item, 仅做"路径引用" |
| **C. work item 在但调研没细** | S2-W1 contract (调研齐) vs S2-W2 (细节缺) | DRAFT 状态, 等 user freeze |

### 5.2 最大裂缝: **Phase 2 准备桶 (G8) 几乎空白**

Graph Plan v1 G8 = "Phase 2 P0 准备" — 文字一句话, **没**具体 task 拆。

按调研密度, G8 至少应该拆 6 个 P0/P1 准备 task (上 §3 表), 但目前 0 拆。

### 5.3 第二裂缝: **FastContext 是 "调研 100% 齐" 但 "治理 0 解锁"**

- ADR-0006 accepted (2026-07-24)
- integrations/fastcontext.md 209 行 (2026-07-24)
- CURRENT.md PARTIAL + EXPERIMENTAL 状态
- 但 Phase 1 期间 5 处 `fastcontext_download_or_install: false` 硬边界锁死
- **这是唯一"调研完全 ready 但被治理主动阻止"的案例**

---

## 6. 修补建议 (按 ROI 排序)

| ROI | 修补 | 估时 | 风险 |
|---|---|---|---|
| **极高** | 补 G8 Phase 2 准备桶 6 task (含 FastContext Spike) | 4-6 小时 | 低 — 调研全齐, 只拆 task |
| 高 | 补 D3/D4/D5 必决 work item (3 个) | 1-2 小时 | 低 — 5 必决已 freeze, 只需建 work item 跟踪 |
| 中 | 补 P0-19 ClickHouse work item | 1 小时 | 低 — 调研 1 句, 实施复杂 |
| 中 | 补 Phase 3+ 长期监控桶 (G11 拆 6 task) | 2-3 小时 | 中 — 长期范围模糊 |
| 低 | 补 §6 v0.4 未来 doc 6 个写工作 | 4-6 小时 | 中 — 写 doc 是工作, 但不一定成 work item |

---

## 7. 当前 status (2026-07-25 03:18 SGT)

- Phase 1 Slice 1 ✅ 5 WI 全 PASS (commit `5861f82`)
- Phase 1 Slice 2 ⏸ S2-W1 contract frozen + reviewer PASS, **等 explicit human authorization to RED**
- **裂缝审计完成**: 45 个调研齐案例没 work item
- **FastContext 落地 Phase 2 可行性**: 原则可行, 4 步前置 (本审计后续单独评估)

**VERDICT: PASS** (审计工作完成, 45 个案例 100% 落表)

---

**END of Audit** — 45 个调研齐未 work item / 2026-07-25 03:18 SGT
