# Loom Phase 1 Slice 1 Progress

Updated: 2026-07-24

## Execution truth

- Workspace: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- Candidate lineage: detached `8e207b8`, the same commit currently referenced by
  `codex/loom-platform`
- Preserved pre-existing untracked paths: `.codex/installation_id`,
  `.codex/skills/`, `.loom-drafts/`
- Terminal boundary: Slice 1 only
- Commit, push, merge, release, activation, credential changes, paid remote work,
  and FastContext installation: prohibited

## WorkItems

| WorkItem | Risk | Status | Gate |
|---|---|---|---|
| S1-W1 | Standard | PASS | RED, GREEN, impact, race, vet, reviewer PASS |
| S1-W2 | Strict | PASS | RED, two repairs, strict GREEN, fresh reviewer PASS |
| S1-W3 | Strict | PASS | L2 bounded repair GREEN + fresh reviewer PASS |
| S1-W4 | Strict | PASS | Two repairs, strict GREEN, fresh scoped reviewer PASS |
| S1-W5 | Standard | PASS | One repair, GREEN, fresh reviewer PASS |

## Completed evidence

- S1-W1: `.loom-evidence/phase1-slice1/S1-W1/deliverable.md`
- S1-W2: `.loom-evidence/phase1-slice1/S1-W2/deliverable.md`
- S1-W3 historical failed lineage:
  `.loom-evidence/phase1-slice1/S1-W3/deliverable.md`
- S1-W3 accepted repair lineage:
  `.loom-evidence/phase1-slice1/S1-W3-L2/deliverable.md`
- S1-W4: `.loom-evidence/phase1-slice1/S1-W4/deliverable.md`
- S1-W5: `.loom-evidence/phase1-slice1/S1-W5/deliverable.md`

## Slice 1 terminal gate

- Final repository `go test ./... -count=1`: PASS
- Final repository `go test -race ./... -count=1`: PASS
- Final `go vet ./...`, build, formatting, and `git diff --check`: PASS
- Five accepted WorkItem deliverables end with `VERDICT: PASS`
- Final evidence:
  `.loom-evidence/phase1-slice1/final-verification.md`
- State: Slice 1 locally complete, uncommitted, not activated; stop before
  Slice 2

## Drafts (in `.loom-drafts/`, untracked, awaiting review)

- `phase1-slice1.optimized.goalspec.yaml` — 220 lines, v2 optimized GoalSpec
- `new-session-prompt.md` — 68 lines, new Codex session launch prompt
- `phase-roadmap-supplements.md` — v0.4 (~770 lines), 4-version evolution
  - v0.1: Phase 1-4 structure with "可吸 / 不吸 / 主动排除"
  - v0.2: + Omnigent + Codex App + 5-layer design model + 7 invariants
  - v0.3: + Claude Code Hooks 22 events + SKILL.md 开放标准 + Codex Record&Replay
  - v0.4: + MCP 2025-11-25 wire format + envelope diff vs Loom Bridge + 3 alignment options + recommended option C (JSON-RPC 2.0 v1.1) + 5 Phase 1 必决决策
- `community-survey-v1.md` — 7 个用户指定 repo 实地验证 (web_fetch GitHub 主页)
- `community-survey-v2.md` — 14 个项目按 4 category 横向调研 (Goose/OpenCode/Aider/Cline/Continue/Pi + Letta/LangGraph/PraisonAI/Pydantic-AI + Temporal/Inngest/Restate/Prefect + Langfuse/Helicone/Phoenix/LangSmith)
  - **AOS Community Edition (aos-ce) ⭐6.9k**：强相关 — Rust, 21 capsules, `aos mcp serve` 是 Codex/Claude/Grok 共享 product edge
  - **open-connector**：4+ 同名低星项目，最相关 `openconnector-dev/openconnector` (1★) — 还在 source code 阶段
  - **colibri (trending 17.5k)**：实际 GitHub `Colibri` 账号 0 star — trending 数字疑似错配
  - **wloc (trending 5.9k)**：`wloc-org/wloc` 404 — 不存在
  - **torlink**：多个 Tor P2P / Tor link 列表项目 — 不相关
  - **Codex-Dream-Skin ⭐12.1k**：Codex 桌面端换肤 (CDP 远程调试) — 不相关
  - **exploitarium**：公开 exploit PoC 仓库 — 不相关

## 2026-07-24 社区调研 v1 关键发现 (community-survey-v1.md)

- **AOS CE = Loom 最值得深挖的"下游消费者"候选** — Rust agent 操作系统, 21 first-party capsules, `aos mcp serve` 是 Codex/Claude/Grok 共享的 MCP edge
- **AOS CE 关键设计**：
  - Product CLI `aos` 拥有 `init/status/migrate/update/distro/mcp/serve-health` roots
  - Unicity Audit: Sigstore bundles + GitHub build-provenance attestations + `runtime-compatibility.toml` pins
  - 闭源 AOS 之上，用户用 Forge 工具自建 capsule（meta-harness 模式）
- **AOS CE vs Loom 对比**：
  - 同：capsule ↔ subagent 同构（用户空间能力块）
  - 同：`aos mcp serve` ↔ Loom Bridge v1.1（MCP 集成方向一致）
  - 异：AOS CE 是 product surface（CLI + HTTP API），Loom 是 view layer（3 views）
  - 异：AOS CE 鼓励"用户自造 capsule"，Loom 鼓励"Planner 派发已有 subagent"
  - 同：两者都不造 LLM runtime（Loom 拒绝代理 LLM，AOS CE 依赖 astrid runtime）
- **AOS CE 待深挖项**：`docs/meta-harness.md` / `docs/release-channels.md` / 21 capsules 分类 / `aos mcp serve` 协议实现
- **trending 数字不可信**：colibri 给 17.5k 实际 0, wloc 给 5.9k 实际 404 — 必须 web_fetch GitHub 验证
- **下次 v0.5 调研方向**：AOS CE 深挖（Rust 21 capsules + meta-harness 模型）作为 Phase 2/3 Loom 借鉴参考

## 2026-07-24 v0.4 调研摘要

- **MCP wire format = JSONL over stdio + JSON-RPC 2.0 envelope** — Loom Bridge Phase 1 已选 JSONL ✓ 对齐
- **Envelope 不兼容**：Loom 用 `type/payload`，MCP 用 `method/params`/`result`/`error`
- **推荐选项 C**：Loom Bridge v1.1 升级到 JSON-RPC 2.0 envelope（加 `jsonrpc: "2.0"` + 改 `type` → `method` + `payload` → `params`/`result`/`error` + 保留 `loom.*` 业务字段）
- **成本**：v1.0 → v1.1 是字段 rename，shim 兼容
- **收益**：Phase 2 接入 MCP 生态（1000+ 现成 Server）零迁移
- **MCP version pin**：2025-11-25（v1 现行稳定 + Anthropic / OpenAI / Google / MS 全员支持）
- **不跟进 v2 SDK**：2026-07-28 beta 出来后等 GA + 6 个月生产验证
- **Loom 拒绝代理 LLM**：`sampling/createMessage` 收到直接 fail closed，保持 Loom = 编排 + 观测
- **Tool annotations ↔ Loom Risk 字段**：直接映射（readOnlyHint / destructiveHint / idempotentHint / openWorldHint）
- **Subagent ≠ Sampling**：正交概念，Bridge 不实现 Sampling 原语
- **未来 Loom 文档待补**：5 个新文件（bridge-envelope-v1.1.md / mcp-host.md / mcp-server.md / mcp-version-pin.md / `TECH-PLAN.md §7` 修订）

## 2026-07-24 社区调研 v2 关键发现 (community-survey-v2.md)

**调研方法升级**：吸取 v1 教训，12 个 web_fetch 大部分截断。这次改用 `raw.githubusercontent.com/owner/repo/branch/README.md` 直接抓原始 markdown — 14 个项目全部成功。

**14 个项目按 4 类别横向对比**：

| Category | 项目 | License | 关键定位 | Loom 借鉴度 |
|---|---|---|---|---|
| **Cat 1: Coding agent** | Goose | Apache 2.0 | Linux Foundation AAIF, 15+ providers, 70+ MCP extensions, Custom Distributions | 中 (Distro 概念) |
| | OpenCode | MIT | TypeScript, build/plan/general 三 agent (Tab 切换) | 中 (双 agent 模式) |
| | Aider | Apache 2.0 | 6.8M PyPI installs, 88% singularity (Aider 自写) | 低 (Singularity evidence) |
| | **Cline** | Apache 2.0 | Coordinator + specialists + SDK @cline/sdk + 4 product surfaces | **强** (multi-agent model) |
| | Continue | Apache 2.0 | ⚠️ **ARCHIVED** 2.0.0 final release — 不再维护 | 教训 (商业模式) |
| | **Pi** | MIT | 4 工具极简 + YOLO + Session tree + Steering 队列 | **强** (极简哲学 + 树状 history) |
| **Cat 2: Stateful agent** | Letta | MIT | Memory-first, 旧 repo 留 V1 server, active dev = letta-code | 中 (memory + skills) |
| | **LangGraph** | MIT | "Durable execution" + "Interrupts" + 灵感来自 Pregel/Beam | **强** (durable execution 命名) |
| | **PraisonAI** | MIT | 25+ features 全参考, 14μs instantiation, MCP+A2A+Policy+Memory | **强** (25+ features 列表 reference) |
| | **Pydantic AI** | MIT | "FastAPI feeling", Capabilities composable bundles, Pydantic Stack | **强** (FastAPI feeling + capabilities) |
| **Cat 3: Durable execution** | **Temporal** | MIT | 8+ years mature, Workflows/Activities/Workers + Replay | **强** (Replay 模式) |
| | Inngest | SSPL+DOSP | Event API → Stream → Runner → Queue → Executor | 中 (Flow control primitives) |
| | **Restate** | Apache 2.0 | **"Durable AI Agents" 显式 use case**, Exactly-once + Suspending + Durable Promises | **强** (exactly-once + suspend) |
| | Prefect | Source-available | Python workflow orchestration (decorator pattern) | 低 (Python 深度) |
| **Cat 4: LLM observability** | **Langfuse** | MIT | 16k★, ClickHouse-based, YC W23 (acquired by ClickHouse 2026-01), massive 集成生态 | **强** (OpenAPI spec + integration) |
| | Helicone | Apache 2.0 | AI Gateway 100+ models, Cloudflare Workers proxy | 中 (商业模式不同) |
| | **Arize Phoenix** | Elastic 2.0 ⚠️ | OTel-based, **Remote MCP Server** 内置, OpenInference standards | **强** (OTel + Remote MCP) |
| | LangSmith | Closed | LangChain 商业, TraceID/SpanID/ParentID | 中 (APM 模式) |

**3 大跨 category 模式（Loom 必须借鉴）**：

1. **"Coordinator + specialists" 是 multi-agent 事实标准**
   - OpenCode (build + plan + general) / Cline (coordinator + specialists) / Pi (Steering + Follow-up + 4 tools) / LangGraph (Deep Agents = subagents) / PraisonAI (Orchestrator Workers) 同构
   - **Loom 直接对位**: Planner = coordinator, subagent = specialists

2. **"Durable execution" 是 agent 必备**
   - Temporal (Workflows + Activities + Replay) / Inngest (Event-driven + Flow control) / Restate ("Durable AI Agents" 显式 use case + Exactly-once + Suspending + Durable Promises) / LangGraph / Pydantic AI
   - **Loom 直接对位**: Run claim + heartbeat = durable execution 的 view layer 实现（验证 Loom 方向正确）

3. **"MCP + Remote MCP Server" 是 2026 行业新趋势**
   - Goose 70+ extensions via MCP / Cline `cline mcp` / PraisonAI 4 transport / Pydantic AI MCP
   - **Arize Phoenix 直接把 observability 暴露为 Remote MCP Server**（让 Claude Code / Cursor 直接 query traces）
   - **Loom 直接对位**: Bridge v1.1 (JSON-RPC 2.0) + Phase 2 MCP + cost/governance view 暴露为 Remote MCP Server

**Loom 应该做 vs 不做（关键边界）**：

| ✅ 应该做 | ❌ 不应该做 |
|---|---|
| view layer (不抢 agent 产品) | agent runtime (那是 Goose / Cline / Aider) |
| Event Journal (append-only, rebuildable) | memory model (那是 Letta / Pydantic AI) |
| durable execution 的 view | durable execution 本身 (那是 Temporal / Restate) |
| "Coordinator + specialists" 编排协议 | multi-agent framework 本身 (那是 LangGraph) |
| cost / governance view | observability 平台 (那是 Langfuse / Phoenix) |
| Bridge v1.1 (JSON-RPC 2.0) | subagent 完整 SDK (那是 Cline @cline/sdk) |
| CLI + run claim | IDE 集成 / Desktop (那是 Cline / Continue) |
| local-first + MIT | SaaS-first + Elastic License (那是 Phoenix) |

**Permanent "不吸" 清单 (v2 新增)**：
- **Continue**: 已停维护 = 行业教训 ("vibe-coded weekend project" 死法)
- **Prefect**: 商业化强 + Python 深度集成 + Cloud-first (跟 Loom local-first 冲突)
- **Helicone**: 商业化 AI Gateway (跟 Loom "fail closed on sampling/createMessage" 冲突)
- **LangSmith**: 闭源商业 (跟 Loom MIT 倾向冲突)
- **Arize Phoenix (license only)**: Elastic License 2.0 ≠ OSI 开源 (Loom 应该避免)

**借鉴优先级**：
- **P0 (Phase 1 必借鉴)**: Cline multi-agent model / LangGraph durable execution 命名 / Pydantic AI capabilities bundle
- **P1 (Phase 2 应该借鉴)**: Phoenix Remote MCP Server + OTel / Langfuse 集成生态 / Restate exactly-once
- **P2 (Phase 3+ 长期)**: Temporal replay-based resume / Langfuse OpenAPI spec / LangSmith env var auto-integration

**决策（Phase 1 必决）**：
1. **不学 Helicone 做 AI Gateway** — Loom 严格 fail closed on sampling/createMessage
2. **不学 Phoenix 用 Elastic License 2.0** — Loom 倾向 MIT
3. **Phase 2 cost/governance view 暴露为 Remote MCP Server** — 学 Phoenix 模式
4. **Phase 1 Bridge v1.1 用 "Coordinator + specialists" 命名** — 学 Cline

**调研方法改进（memory 候选）**：
- ❌ 之前 `web_fetch github.com/owner/repo` 大部分被 chrome 截断
- ✅ 改用 `web_fetch raw.githubusercontent.com/owner/repo/branch/README.md` 直接拿原始 markdown
- 14 个项目全部成功，是 v1 方法的有效升级

## 2026-07-24 社区调研 v3 前沿深挖 (community-survey-v3-deepdive.md, 45K 字节)

**调研范围**: v2 14 项目 / 4 category 基础上, 选最活跃项目深挖 2026-Q3 演进, 抓 8 README + 8 web_search 2026 月度新鲜。

**3 大 2026-Q3 行业共识 (TL;DR)**：

1. **"Coordinator + specialists" 是 multi-agent 事实标准** — OpenAI + Anthropic 2026 双向背书 (能用单 Agent 解决的不堆 Agent, 业界 80% "成功案例" 是 Subagent 主从模式)
2. **"Durable execution" 是 agent 必备** — Restate "Durable AI Agents" 第一 use case, Temporal Replay 2026 主推, DBOS Conductor + 新 MCP server, Microsoft pg_durable 内嵌 SQL
3. **"MCP + Remote MCP Server" 是 2026 行业新趋势** — MCP 2026-07-28 RC 史上最大修订, Arize Phoenix / Langfuse / 企查查 9 server / 197 tool 全暴露

**Cat 1 (Coding Agent) 2026-Q3 深挖**：
- **Cline** 6,614 commits + 4 product surfaces (CLI/Kanban/VS Code/JetBrains) + @cline/sdk + Multi-Agent Teams (`cline --team-name auth-sprint`) + Scheduled Agents (`cline schedule create`)
- **Pi 大 rebrand** `badlogic/pi-mono` → `earendil-works/pi-mono`, 4 packages (pi-ai / pi-agent-core / pi-coding-agent / pi-tui), supply-chain hardening (`min-release-age=2` / `save-exact=true`), 3 containerization patterns (Gondolin / Docker / OpenShell)
- **OpenCode 简化** 3 agents → 2 agents (build/plan), 22 语言本地化, Desktop BETA
- **共识**: 拒绝内置复杂 permission / Coordinator+specialists / 多 provider / 简化极简 / Session 持久化

**Cat 2 (Stateful Agent) 2026-Q3 深挖**：
- **Pydantic AI Harness v0.5.0** (官方 capability 库, 2026-07) — 22 capability area 含 CodeMode (Monty 沙箱) / ToolSearch / Sub-agents / Memory / StuckLoopDetection / CostTracking / SecretRedaction。Pydantic Stack = Pydantic AI + Logfire + **Logfire AI Gateway**
- **subagents-pydantic-ai v0.5.0** (94 commits) — sync/async/auto 3 mode, nested subagents, dynamic agent creation, SubAgentSpec YAML/JSON, question mode
- **LangChain 1.0** (2025-10) — Middleware 洋葱圈 (5 hook: before_agent/after_agent/before_model/wrap_model_call/after_model/wrap_tool_call), `create_agent` 10 行起步, PIIMiddleware
- **LangChain 1.1.0** (2025-11-24) — Model Profiles `.profile` 属性
- **LangGraph 1.0** — 5 pillars, Deep Agents (高级包: planning + filesystem + subagents + memory), 灵感 Pregel/Beam/NetworkX
- **create_deep_agent** 18 参数签名 + **三级上下文压缩** (tool input offloading / tool result offloading > 20k token / summarization 85% 窗口阈值) + 10 中间件栈固定顺序
- **OpenAI + Anthropic 2026 双向背书** Subagent 主从模式

**Cat 3 (Durable Execution) 2026-Q3 深挖**：
- **Restate "Durable AI Agents"** 6 primitives + 5 SDKs + SemVer 友好 (x.y → x.y+1 无需 manual migration)
- **Temporal Replay 2026** 4 大新功能: Serverless Workers / Standalone Activities / Workflow Streams / Google ADK + OpenAI Agents SDK 集成。NVIDIA / Salesforce / Twilio / Descript / OpenAI Venkat (ex-Rockset) 全推荐
- **DBOS Conductor** (2026-07) — 新 MCP Server + OpenMetrics + RBAC + Bulk Workflow forking + Google ADK plugin + DBOS Transact for Java 1.0
- **Microsoft pg_durable** (NEW 2026) — **PostgreSQL extension, SQL 关键字** `df.start() |=> 'name' ~> 'sql'` + pgrx + duroxide runtime + 0.2.2 latest
- **Hatchet** — 3,208 commits, Postgres-based, **~416 jobs/s**, 11ms p50 insert→result, multi-tenant OTEL
- **Belay (Elixir)** NEW 2026-07 — Journal-based, 1.0.0-rc.5, **$USD budget per job** (3 层 cost 控制), 99,004 jobs / 7h soak test 0 violations, MCP server built-in
- **共识**: AI workflow 是 2026 主 use case / Postgres 作为底层 / 不自己造 durable engine (Loom 应该 view layer 选 Restate/DBOS 接入)

**Cat 4 (LLM Observability) 2026-Q3 深挖**：
- **Arize Phoenix 2026-Q3** — **PXI (Phoenix Intelligence) AI debugging agent** + **Remote MCP Server 内置** + `.agents/skills/` 多 editor (Claude Code/Codex/Cursor 同步) + 25+ Python + 7 TS + 2 Java + 2 Go integrations
- **OpenInference 1,948 commits** — **2026-06 semantic conventions 正式进入 OpenTelemetry GenAI 工作组** (`spec/reasoning` PR #3112) — 这是 2026 observability 行业最大事件
- **OpenTelemetry** — **CNCF graduated**, 12+ languages, 200+ collector components, 1020+ integrations
- **traceloop/openllmetry v0.49+** — "Our semantic conventions are now part of OpenTelemetry!"
- **Langfuse (ClickHouse 2026-01 acquired)** — 16,054 stars, **50M+ SDK installs/month**, **10B+ observations/month**, 2,300+ customers, 99.9% uptime, **Coding agents SKILL.md 新发布 + Platform MCP Server**
- **🆕 MCP 2026-07-28 RC 史上最大修订** — 4 大生产化信号: **协议从"会话绑定"走向"请求自包含"** + **能力从"列出来"走向"管起来"** + **任务从"一次调用"走向"持续完成"** + **结果从"模型说了什么"走向"依据能否还原"**。5 大变化: 无状态核心 + 能力发现 + 缓存 + Extensions 一等公民 + Tasks + MCP Apps
- **🆕 企查查 MCP 2026-07** — 9 Server / 197 tool / 27 SKILL, **5 层能力矩阵** (Tool → Server → Resources → SKILL → 全局约束), 适配 WorkBuddy/QoderWork/QClaw/IMA/MiniMax/LobsterAI
- **共识**: MCP 是 2026 必备 surface / Coding agents 是 2026 必备用户 / OTel 是 trace 标准 / 不重建 observability 平台

**v3 新增借鉴优先级**：

| P0 (Phase 1 必做) | 来源 | Loom 落地 |
|---|---|---|
| **Coordinator + specialists** 命名 | Cline / OpenCode / Pi | Bridge v1.1 `loom.dispatch` |
| **JSON-RPC 2.0 envelope** | MCP 2026-07-28 | Bridge v1.1 (已定) |
| **W3C Trace Context 传播** | MCP 2026-07-28 | Bridge v1.1 trace 字段 |
| **完整 JSON Schema 2020-12** | MCP 2026-07-28 | Bridge v1.1 method schema |
| **Capabilities = composable bundles** | Pydantic AI Harness v0.5.0 | Bridge v1.1 method dispatch |
| **Middleware 洋葱圈** 模式 | LangChain 1.0 | daemon 中间件栈 |
| **SubAgent 双类型** (SubAgent dict + CompiledSubAgent) | LangGraph Deep Agents | subagent spec |
| **$USD budget per run** | Belay | Run claim 必填 |
| **OTel spans 作为 trace 出口** | OpenTelemetry CNCF | daemon observability |
| **多 editor skill 同步** (`.agents/skills/`) | Phoenix | `.loom-drafts/skills/` |
| **Remote MCP Server 暴露** (Phase 2 但 P0) | Phoenix / Langfuse | cost view 准备 |
| **5 层能力矩阵** (Phase 2 但 P0) | 企查查 MCP | cost view 准备 |

| P1 (Phase 2) | P2 (Phase 3+) |
|---|---|
| "Code mode" 概念 (Pydantic) | SQL 关键字路线 (Microsoft pg_durable) |
| 三级上下文压缩 (LangGraph Deep Agents) | Loom 出 "loom-harness" 包 |
| context_schema 模式 (LangGraph) | USD budget true-up (Belay) |
| DBOS 新 MCP server (DBOS Conductor 2026-07) | Logfire AI Gateway 统一 LLM proxy |
| Coding agents SKILL.md (Langfuse) | Temporal Workflow Streams |
| Postgres-based + OTEL (Hatchet) | |
| Resources 暴露稳定知识 (企查查 MCP) | |
| SKILL 组织业务流程 (企查查 MCP) | |
| PXI 概念 (Phoenix) — "loom-insight" AI helper | |
| Chaos test harness (Belay) — 7h kill -9 | |
| Exactly-once 语义 (Restate) | |
| OAuth 2.1 (MCP 2026-07-28) | |

**v3 新增 Permanent "不吸" 清单**：
- ❌ **Phoenix license (ELv2)** — 跟 Loom MIT 冲突
- ❌ **Helicone AI Gateway** — 跟 Loom "fail closed on sampling/createMessage" 冲突
- ❌ **LangSmith** — 闭源商业
- ❌ **Prefect** — Python 深度 + Cloud-first 跟 Loom local-first 冲突
- ❌ **Continue** — 已 ARCHIVED 2.0.0 行业教训
- ❌ **OpenInference 整套** — Loom 只需 OTel 输出, 不需要 25 integrations
- ❌ **Belay / Hatchet / DBOS 整套** — Loom 是 view layer, 选一个接, 不重做 durable engine
- ❌ **Temporal 整套** — 9 年成熟, Loom 不重做

**5 个新未来 Loom 文档待补（v3 落地）**：
1. `docs/bridge/loom-bridge-v1.1-architecture.md` — JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 + Coordinator dispatch
2. `docs/integrations/mcp-2026-07-28-rc.md` — 借鉴 4 大生产化信号, 写 Loom Bridge v1.1 的 MCP 兼容性
3. `docs/architecture/subagent-spec.md` — SubAgent dict + CompiledSubAgent 双类型 + YAML 配置
4. `docs/integrations/dbos-vs-restate-vs-temporal.md` — Loom 推荐选哪个 durable engine 做底层
5. `docs/skills/loom-coding-agents.md` — 学 Langfuse / Phoenix, 写 Loom 自己的 SKILL.md 供 Claude Code / Codex / Cursor 调

**v3 调研方法学**：
- README 抓取: 全部用 `raw.githubusercontent.com/owner/repo/branch/README.md` (8/8 成功)
- web_search 时间过滤: `freshness=month` 拿 2026-Q3 最新
- 多角度交叉验证: GitHub README (官方) + web_search 月度新鲜 (2026-Q3 趋势) + PyPI/npm 包 (代码级实锤)
- Loom 借鉴落地: 每个项目先问 "该不该学" (P0/P1/P2/不吸), 再问 "学到 Bridge v1.1 / Phase 2 / Phase 3 哪里"
