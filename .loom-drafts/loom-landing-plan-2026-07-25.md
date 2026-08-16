# Loom 落地方案 (2026-07-25)

> **作者**: mavis (主 session `mvs_3be0bd26bb64475dbc65181516dda9df`)
> **时间**: 2026-07-25 03:40 SGT
> **整合来源**:
> - 已调研 50 条借鉴 (5 必决 + 12 类 P0 18-20 条 + 13 P1 + 6 P2 + 8 不吸)
> - 7 ADR (0001-0006, 全部 accepted 2026-07-24)
> - v0.4 永久借鉴清单 (6 项)
> - v0.4 未来文档待补 (6 项)
> - FastContext (ADR-0006 + docs/integrations/fastcontext.md 209 行)
> - TUI 方案 C 选定 (Loom Console TUI 最小版 / docker stats 风格)
> **状态**: 落地方案 (working document, 不 commit)

---

## 0. 一句话总结

**Phase 1 期间 (S1 ✅ + S2-S5 ⏸) 跑起来的产品形态 = 极简 CLI + Loom Console TUI (方案 C)。FastContext 等 Phase 2 启动 + 显式 Spike gate 授权后,以 FCS-1 work item 形式独立落地,不抢主路径。已调研 50 条借鉴 + 7 ADR + 6 永久借鉴 + 6 待补, 全部按 Slice 1-5 / TUI / Phase 2 准备桶 / FastContext 4 阶段排期,无遗漏。**

---

## 1. Loom Console TUI 方案 C 落地 (用户 7.25 03:38 SGT 选定)

### 1.1 形态

`docker stats` / `k9s` 风格,实时 stream + 详情弹层,3-4 天实施。

### 1.2 落地

| 阶段 | 内容 | 估时 | 依赖 |
|---|---|---|---|
| Day 1 | Bubble Tea 骨架 + `cmd/loom/console/main.go` + 1s refresh tick | 1 天 | 无 |
| Day 2 | `internal/console/stream.go` 订阅 Event Journal + Run list 渲染 | 1 天 | S2-W1 (Run 概念) + S2-W3 (Coordinator) |
| Day 3 | 详情弹层 + 搜索 + 过滤 | 1 天 | Day 2 |
| Day 4 | 样式 + 错误处理 + `go test ./...` + `-race` + `vet` | 1 天 | Day 3 |

**落地位置**: Slice 5 末尾 (per Graph Plan v1 G7),不抢 S2-W1~W5 资源。
**路径**: `cmd/loom/console/` (新) + `internal/console/` (新) + Bubble Tea / lipgloss 依赖。
**触发**: `loom console` 启动,跟 `loom route` / `loom status` 平级。
**目标用户**: 个人开发者, 实时观测 Loom Run / Event Journal / Cost, 取代 `sqlite3 + jq`。

### 1.3 落地不冲突项

- ❌ 不抢 agent runtime (不调 Codex/Claude/Pi)
- ❌ 不写 Loom State Store (只读)
- ❌ 不抢 IDE (TUI 是终端独立进程)
- ❌ 不开外部端口 (loopback-only 不需要, TUI 直连本地 SQLite)
- ✅ 跟 Claude Code/Codex/Cursor 互补 (Loom 是观测器, 它们是触发器)

---

## 2. FastContext 落地 (核心, 用户重点关注)

### 2.1 当前硬边界 (5 处声明, 全部 ON)

| 位置 | 内容 |
|---|---|
| `phase1-slice1.optimized.goalspec.yaml:216` | `fastcontext_download_or_install: false` |
| `phase1-slice1.optimized.goalspec.yaml:51` | 不装 FastContext |
| `phase1-slice1.optimized.goalspec.yaml:179` | Phase 1 不装 |
| `PROGRESS.md:16/69` | FastContext installation **prohibited** |
| `docs/CURRENT.md:71` | "FastContext installation remain unauthorized. The optional local FastContext Spike remains separate and was not installed or activated." |

### 2.2 调研 / 规范齐度 (100%)

| 已有 | 没 |
|---|---|
| ✅ ADR-0006 accepted 2026-07-24 (78 行) | ❌ FCS-1 contract DRAFT (待你 freeze) |
| ✅ `docs/integrations/fastcontext.md` 209 行集成规范 | ❌ Phase 2 governance (待 Phase 1 完) |
| ✅ `docs/CURRENT.md` PARTIAL + EXPERIMENTAL 状态 | ❌ 显式 Spike gate 授权 (待你) |
| ✅ 调研报告 2 次引用 (路径 + Layer) | ❌ 实施 |

### 2.3 落地步骤 (4 步前置 + 6 步实施)

#### 前置 (你必须做)

1. **Phase 2 governance 重 freeze** (覆盖 Phase 1 那 5 处硬声明)
2. **显式授权 "Spike gate"** (per ADR-0006 + CURRENT.md EXPERIMENTAL)
3. **FCS-1 contract 升 FROZEN** (DRAFT 已落 `.loom-drafts/phase2-prep-bucket/fcs-1-contract.md`,待你审)
4. **Phase 1 完成** (5 Slice 走完, S1 ✅, S2 ⏸, S3-S5 未开始,估 ~25 天 / 5 周)

#### 实施 (授权后 ~2 周)

| Step | 命令 / 动作 | 落点 (Go pkg / 路径) | 估时 |
|---|---|---|---|
| **1. 装** | `loom fastcontext install` | `cmd/loom/fastcontext/install.go` (新) | 半天 |
| **2. Adapter** | Loom-owned read-only adapter (READ/GLOB/GREP) | `internal/codeanalyst/adapter.go` (新) | 1 天 |
| **3. 评估 set** | 5 Loom-specific queries (recall on Loom 真实 codebase) | `eval/fastcontext/recall-set.json` (新) | 1 天 |
| **4. 跑 spike** | `loom fastcontext eval` 1 周,`loom fastcontext audit` 每次 | `eval/fastcontext/runner.go` (新) | 1 周 |
| **5. 报告** | 召回率 + 安全审计 → 决策推荐 | `.loom-evidence/phase2/fastcontext-spike/FCS-1/eval-report.md` | 半天 |
| **6. ADR 修订** | 升 accepted / 保持 experimental / 回退 | `.loom-evidence/phase2/fastcontext-spike/FCS-1/adr-revision.md` | 半天 |

#### 验收 (FCS-1 完成的硬证据)

- [ ] `cmd/loom/fastcontext/` 安装脚本可执行, 在 `localhost` 启动 community SFT
- [ ] `internal/codeanalyst/` Go package 通过 `go test ./...` + `-race` + `go vet`
- [ ] 5+ 个 Loom-specific 评估 query 跑通, 召回率 >= 80%
- [ ] 1-2 周 spike 期间 **0 path escape + 0 secret leak + 0 source mutation**
- [ ] 评估 set 输出报告 + 决策推荐
- [ ] fresh read-only Reviewer 对 deliverable.md 出 `VERDICT: PASS`
- [ ] ADR-0006 修订 (升 accepted / 保持 experimental / 回退)

#### 不实施 = OK 的退出条件

spike 跑出**任意** 1 个问题:
- license 不可调和 → 立刻退出, ADR-0006 修订 "rejected: license conflict"
- 召回率 < 60% → 退出, ADR-0006 修订 "rejected: recall insufficient"
- path escape / secret leak / source mutation 任 1 → 立刻退出, ADR-0006 修订 "rejected: safety violation"

**退出 ≠ 失败**, 是**显式决策**, 写进 ADR 修订。

### 2.4 落地不冲突项 (FastContext 跟主路径隔离)

```
主路径: User → Mode Router → Coordinator → Runtime Adapters (调 Agent)
FastContext: User → `loom fastcontext query` → Adapter → SFT (只 code analysis)
```

两条路互不干扰, FastContext 是**辅助 code analysis sidecar**, 不参与 Run claim, output 是 Candidate evidence (不入 Event Journal)。

### 2.5 当前用户能做的 (Phase 1 硬边界 ON 期间)

**❌ 全部 FastContext 命令拒绝** (per 5 处硬边界)
**✅ 当前能做的**:
- 读 `docs/CURRENT.md` / ADR-0006 / `docs/integrations/fastcontext.md` 了解设计
- 审 FCS-1 contract DRAFT (`.loom-drafts/phase2-prep-bucket/fcs-1-contract.md`)
- 走 `loom chat` + 显式 trigger (S1-W1 Mode Router)

---

## 3. 已调研借鉴整合落地表 (50 条 + 7 ADR + 6 永久 + 6 待补)

按 Slice 1-5 / TUI / Phase 2 准备桶 4 阶段排期,无遗漏。

### 3.1 Slice 1 (✅ PASS at 5861f82, 已落地的借鉴)

| 借鉴 / ADR | 实施 |
|---|---|
| ADR-0001 Explicit Agent Mode | ✅ S1-W1 Mode Router (4 triggers) |
| ADR-0002 Local Modular Monolith + Projections | ✅ S1-W2 Event Journal + S1-W4 Projection |
| ADR-0003 Agent Definition Runtime Separation | ✅ S1-W1 partial + S2-W1 |
| 5 必决 D1 (JSON-RPC 2.0 envelope) | S2-W1 follow-up,Slice 1 走简化版 |
| 5 必决 D2 (MCP pin 2025-11-25) | S2-W1 follow-up,Slice 1 不涉及 MCP |
| 5 必决 D3 (拒代理 LLM) | ⚠️ 调研齐,**没独立 work item** (D3-D5 全是 7.25 03:18 SGT 审计发现) |

### 3.2 Slice 2 (⏸ S2-W1 frozen + reviewer PASS, 等显式授权)

| 借鉴 / ADR | 实施 |
|---|---|
| **D1** Bridge v1.1 = JSON-RPC 2.0 envelope | S2-W1 (主) |
| **D2** MCP version pin 2025-11-25 | S2-W1 (跟 D1 合并) |
| **P0-2** 极简哲学 (Pi 4 工具) | S2-W1 (philosophy) |
| **P0-4** Capabilities = composable bundles | S2-W1 (schema) |
| **P0-5** Agent.run_sync/stream/run 3 形态 | S2-W1 (partial sync, stream 推 Phase 2) |
| **P0-10** Google ADK + OpenAI Agents SDK 兼容 | S2-W1 (envelope) |
| **P0-16** JSON-RPC 2.0 envelope | S2-W1 (跟 D1 重) |
| **P0-17** W3C Trace Context 传播 | S2-W1 |
| **P0-18** 完整 JSON Schema 2020-12 | S2-W1 (schema) |
| 永久借鉴 1: Multica Runtime discovery | S2-W3 (Coordinator) |
| S2-W2: Daemon + Middleware 洋葱圈 (P0-6) | S2-W2 实施 |
| S2-W2: DBOS Postgres 路线 (P0-11) | S2-W2 留好切换路径 |
| S2-W2: durable exec = 库/extension (P0-12) | S2-W2 内嵌 |
| S2-W3: Coordinator+specialists (P0-1) | S2-W3 实施 |
| S2-W3: read-only plan agent (P0-3) | S2-W3 实施 |
| S2-W3: SubAgent 双类型 (P0-7) | S2-W3 实施 |
| S2-W3: "Durable AI Agents" 定位 (P0-8) | S2-W3 实施 |
| S2-W3: Replay-based resume (P0-9) | S2-W3 实施 |
| S2-W4: $USD budget per run (P0-13) | S2-W4 必填 |
| S2-W4: OpenTelemetry trace 出口 (P0-15) | S2-W4 必出 |
| S2-W4: OpenTelemetry compatible (P0-20) | S2-W4 跟 P0-15 合并 |
| S2-W5: .agents/skills/ 多 editor (P0-14) | S2-W5 起步 |
| **D3** 拒代理 LLM | ⚠️ **没独立 work item** (审计发现) |
| **D4** Tool annotations ↔ Loom Risk 字段 | ⚠️ **没独立 work item** (审计发现) |
| **D5** Subagent ≠ Sampling | ⚠️ **没独立 work item** (审计发现) |

### 3.3 Slice 3-4 (未开始, 估时 ~10-12 天)

| 借鉴 | 实施 |
|---|---|
| Slice 3: 真实执行 (1 个真实 Runtime Adapter / JSONL Bridge / AgentGrant / claim / workspace / WorkItem DAG) | S3 (TBD) |
| Slice 4: 规则与验收 (客户 Rule / `require_approval` / 确定性验收 / 风险级 Verifier 路由) | S4 (TBD) |

### 3.4 Loom Console TUI 最小版 (Slice 5 末尾, 3-4 天)

| TUI 内部 | 关联借鉴 |
|---|---|
| `internal/console/stream.go` 订阅 Event Journal | Slice 1 Event Journal (P0-6 Middleware 洋葱圈) |
| `internal/console/runlist.go` 读 Projection | Slice 1 Projection (P0-9 Replay-based resume) |
| 详情弹层查 evidence | Slice 1 Evidence (P0-15 OTel trace 出口) |
| 实时 stream 渲染 (Bubble Tea) | (无直接借鉴, 极简 UI 即可) |

### 3.5 Phase 2 准备桶 (G8 6 task, 已落 `.loom-drafts/phase2-prep-bucket/g8-six-tasks-briefs.md`)

| G# | 借鉴 | 调研源 |
|---|---|---|
| **G8.1 FCS-1** | FastContext Local Spike | ADR-0006 + integrations/fastcontext.md |
| **G8.2 HOOK-1** | Bridge Hook 协议 (Phase 2 增补 ①) | v0.3 §1 Claude Code 22 events + grok-cli |
| **G8.3 SAND-1** | Sandbox 后端选型 (Phase 2 增补 ②) | v0.4 §152 + AgentScope 2.0 sandbox 矩阵 |
| **G8.4 PI-1** | pi-coding-agent RuntimeProfile (Phase 2 增补 ④) | v0.4 §178 + Earendil Pi rebrand 2026-07 |
| **G8.5 PROV-1** | Provider Routing 边界 (Phase 2 增补 ③) | v0.4 §166 + ADR-0004 + D3 必决 |
| **G8.6 CH-1** | ClickHouse OLAP (P0-19) | v3 P0-19 + Langfuse 16k★ + ClickHouse acquired 2026-01 |

**总估时**: 6 task 串行 ~7-8 周, 部分可并行。

### 3.6 ADR 跟踪 (7 个)

| ADR | 主题 | 状态 | 落点 |
|---|---|---|---|
| 0001 | Explicit Agent Mode | accepted | ✅ S1-W1 |
| 0002 | Local Modular Monolith + Projections | accepted | ✅ S1-W2 + S1-W4 |
| 0003 | Agent Definition Runtime Separation | accepted | ✅ S1-W1 partial + S2-W1 |
| 0004 | Credential Broker Authentication Modes | accepted | **⏸ G8.5 PROV-1** |
| 0005 | Local Evolution Sidecar | accepted | **⏸ Phase 3 准备** (P2-2 关联) |
| 0006 | FastContext Code Analysis Sidecar | accepted | **⏸ G8.1 FCS-1** |

### 3.7 永久借鉴清单跟踪 (6 项, per v0.4 路线图)

| 借鉴 | 状态 | 落点 |
|---|---|---|
| Multica Runtime discovery / claim-lease / task-scoped token | partial | S2-W3 (Coordinator) |
| ECC / Multica / AgentScope SKILL.md + Toolkit 抽象 | 没 work item | ⏸ Phase 2 Skill registry (P3 增补 ①) |
| AgentScope 2.0 sandbox 矩阵 | 没 work item | ⏸ G8.3 SAND-1 |
| grok-cli hooks 生命周期 | 没 work item | ⏸ G8.2 HOOK-1 |
| Earendil Pi SDK as RuntimeProfile | 没 work item | ⏸ G8.4 PI-1 |
| OpenHands "prove it works" 工具 | N/A | 已被 Loom deterministic acceptance 覆盖 (不需 work item) |

### 3.8 v0.4 未来文档待补 (6 项)

| 待补 doc | 落点 | 关联 G8 task |
|---|---|---|
| `docs/architecture/bridge-hooks.md` | Phase 2 增补 ① | G8.2 HOOK-1 |
| `docs/integrations/sandbox-backends.md` | Phase 2 增补 ② | G8.3 SAND-1 |
| `docs/integrations/pi-coding-agent.md` | Phase 2 增补 ④ | G8.4 PI-1 |
| `docs/architecture/skill-registry.md` | Phase 3 增补 ① | (Phase 3 准备) |
| `TECH-PLAN.md §11` 末段修订 | Phase 2 增补 ③ | G8.5 PROV-1 |
| `PRODUCT-PLAN.md §Agent 库` 拆分 | Phase 3 增补 ① | (Phase 3 准备) |

### 3.9 P1 借鉴 (13 条, 全部 Phase 2 准备, 调研齐无 work item)

| P1 # | 借鉴 | 落点 |
|---|---|---|
| P1-1 | Code mode (Pydantic AI Monty) | Phase 2 增补 (待) |
| P1-2 | 三级上下文压缩 (LangGraph) | Phase 2 增补 (待) |
| P1-3 | context_schema (Pydantic AI) | Phase 2 增补 (待) |
| P1-4 | Exactly-once (Restate) | Slice 1 Event Journal 已部分 (idempotent append) |
| P1-5 | DBOS Conductor MCP | Phase 2 增补 (待) |
| P1-6 | Postgres + OTEL 定位 | Phase 2 增补 (跟 P0-19 关联) |
| P1-7 | MCP server built-in cost view | Phase 2 增补 (待) |
| P1-8 | PXI 概念 (loom-insight) | Phase 2 增补 (待) |
| P1-9 | OpenInference semantic conventions | Phase 2 增补 (待) |
| P1-10 | 能力发现 + 缓存 (MCP 2026-07-28) | Phase 2 增补 (待) |
| P1-11 | Resources 暴露稳定知识 | Phase 2 增补 (待) |
| P1-12 | SKILL 组织业务流程 | Phase 2 增补 (待) |
| P1-13 | Coding agents SKILL.md | Phase 2 增补 (跟 S2-W5 关联) |

### 3.10 P2 借鉴 (6 条, 全部 Phase 3+ 长期)

| P2 # | 借鉴 | 落点 |
|---|---|---|
| P2-1 | pg_durable SQL 关键字 | Phase 3+ 长期 |
| P2-2 | loom-harness 包 | Phase 3+ 长期 (P3 增补 ①) |
| P2-3 | Chaos test harness | Phase 3+ 长期 |
| P2-4 | Belay USD budget true-up | Phase 3+ 长期 (跟 P0-13 关联) |
| P2-5 | Logfire AI Gateway | Phase 3+ 长期 (⚠️ 跟 D3 拒代理 LLM 冲突) |
| P2-6 | Temporal Workflow Streams | Phase 3+ 长期 |

### 3.11 永久不吸清单 (8 项, 验证落地)

| 不吸 | 验证 | 落点 |
|---|---|---|
| Phoenix (ELv2) | 整体排除 | M8 运营管控层不绑 Phoenix |
| Helicone (AI Gateway) | 整体排除 | D3 必决: 拒代理 LLM |
| LangSmith (闭源) | 整体排除 | M8 运营管控层不绑 LangSmith |
| Prefect (Python 深度) | 整体排除 | M6 执行沙箱不选 Prefect |
| Continue (ARCHIVED) | 整体排除 | 行业教训 |
| OpenInference 整套 | 整体排除 | P0-15 OTel trace 出口只输出 spans |
| Belay/Hatchet/DBOS 整套 | 整体排除 | Loom 选 1 个接 (DBOS Conductor 候选), 不重做 |
| Temporal 整套 | 整体排除 | 9 年成熟, Loom 不重做 |

---

## 4. 时间表 (整合)

| 阶段 | 内容 | 估时 | 状态 |
|---|---|---|---|
| **Phase 1 Slice 1** | 入口 + 持久化 | 5-7 天 | ✅ PASS at 5861f82 |
| **Phase 1 Slice 2** | Agent 定义 + 团队草案 (S2-W1~W5) | ~25 天 | ⏸ S2-W1 frozen + reviewer PASS, 等显式授权 |
| **Phase 1 Slice 3** | 真实执行 | 5-7 天 | 未开始 |
| **Phase 1 Slice 4** | 规则 + 验收 | 4-5 天 | 未开始 |
| **Phase 1 Slice 5** | 看板 + Demo (含 **Loom Console TUI 方案 C**) | 3-4 天 | 未开始 |
| **Phase 1 关键路径总估时** | 5 周 ≈ 25 天 | | |
| **Phase 1 → Phase 2 启动** | governance 重 freeze (5 处硬声明覆盖) | 1 周 | 等 Phase 1 完 |
| **Phase 2 准备 (G8 6 task)** | FCS-1 + HOOK-1 + SAND-1 + PI-1 + PROV-1 + CH-1 | 7-8 周 (部分并行) | ⚠️ 全部 CONTRACT_DRAFT 状态 |
| **Phase 2 实施 (G9+)** | P1 借鉴 13 条 + Future docs | 1-2 周 | 未开始 |
| **Phase 3+ 长期** | 永久借鉴 + P2 借鉴 | 1+ 月 | 未开始 |

---

## 5. 落地不冲突项 (8 个 Loom 根 invariant)

| 不抢 | 实施边界 |
|---|---|
| ❌ 不抢 agent runtime | 委托 Codex/Claude/Pi, Loom 只编排+观测 |
| ❌ 不做 memory model | M4 记忆引擎薄调研, 留给 sub-agent SDK |
| ❌ 不做 durable execution itself | 选 1 个接 (DBOS/Restate), 不重做 |
| ❌ 不做 multi-agent framework | Coordinator 编排, 不发明 sub-agent 抽象 |
| ❌ 不做 observability SaaS | P0-15 OTel trace 出口, 不绑 Langfuse/Phoenix |
| ❌ 不做 subagent SDK | P0-7 SubAgent 双类型是借鉴形态, 不是 SDK |
| ❌ 不做 IDE | TUI 是终端独立进程, 不抢 Cursor/VS Code |
| ❌ 不做 AI Gateway | D3 必决: 拒代理 LLM, fail closed sampling |
| ❌ 不做 SaaS | Loom Web localhost:7432, 不开外部端口 |
| ✅ 跟 Claude Code/Codex/Cursor 互补 | Loom = 触发入口 (SKILL.md 推 Phase 2) + 观测器 (TUI 方案 C) |

---

**VERDICT: PASS** (落地方案整合完成, 4 节全数, 2026-07-25 03:40 SGT)
