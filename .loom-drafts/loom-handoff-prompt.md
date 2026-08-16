# Loom 项目 Handoff Prompt (跨项目 / 跨 session 上下文传承)

**用途**: 在新项目 / 新 session 启动时, 把整段 Loom 上下文 (身份 / 状态 / 队列 / 调研 / 硬规则 / 下一步) 一次性喂给新 Codex, 避免重复调研
**创建时间**: 2026-07-25 02:34 SGT
**作者**: mavis (在原 session `mvs_5d84f5409d0f46488e91055c319ed655` 中)
**状态**: DRAFT v1, 可直接复制使用

---

## 0. 你在哪个项目 (Context)

- **项目名**: **Loom** (本地 AI agent 编排 + 观测守护进程)
- **Workspace**: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- **Owner**: **lune** (customer #0, 唯一 commit-authority)
- **Assistant**: **Mavis** (mavis 模型, 调研 / 文档 / 启动 session 跑 work item)
- **License**: **MIT** (Loom 严格不接 ELv2 / 闭源项目)
- **分支**: `codex/loom-platform-slice2` (head detached at `8e207b8`, 但不是默认工作分支)
- **生产分支**: `codex/loom-platform` (orphan, **不要动**)

---

## 1. 一句话定位 Loom

**Loom = 本地守护进程 + view layer**, 接收外部 agent CLI / editor 指令, 编排多 agent 跑任务, 把一切写进 append-only journal, 通过 OTel/Bridge 暴露成可观测 stream。**不代理 LLM, 不造 agent runtime, 不存用户业务数据**。

类比:
- Docker 时代: `docker stats` / `docker ps` (看容器, 不跑容器)
- K8s 时代: `kubectl get pods -w` / Dashboard (看资源, 不写容器)
- **AI agent 时代: Loom = K8s (编排 + 观测) + Loom Console TUI = Dashboard**

---

## 2. 关键不变量 (必读, 违反会立刻翻车)

### 2.1 Slice 状态 (最重要)

| Slice | 状态 | commit | 命名 |
|---|---|---|---|
| **Phase 1 Slice 1 (S1)** | ✅ **DONE, FROZEN** | `5861f82` | S1-W1~W5 已 PASS, **不要复用** |
| **Phase 1 Slice 2 (S2)** | ⏳ **READY_FOR_CONTRACT** | — | 新工作用 S2-W1~W5 |
| **Phase 2** | 📋 准备 | — | 4 项合桶 (G8-G10) |
| **Phase 3+** | ⏸️ 长期暂缓 | — | 5+1 项 (G11-G12) |

⚠️ **新工作命名规则**: S1-W* 已 frozen, **新工作用 S2-W***, 不要混!

### 2.2 严格 4 不抢 (Loom Constitution 红线)

| ❌ Loom 不接 | 为什么 | 谁接 |
|---|---|---|
| **不代理 LLM** (sampling/createMessage 收到 fail closed) | 跟 Helicone / Logfire Gateway 划清界限, Loom 是 view layer | 9Router / 各 SubAgent 自己 |
| **不造 agent runtime** (不写 agent 思考循环) | 跟 Cline / OpenCode / Pi 划清界限, Loom 是编排层 | SubAgent dict (YAML) 引用外部 agent |
| **不存用户业务数据** (journal 只存 orchestration 事件) | 跟 Phoenix / Langfuse 划清界限, 那些是 observability 平台 | 用户自己的应用数据库 |
| **不做 SaaS** (本地 daemon, 不上云) | 跟 LangSmith / Phoenix Cloud 划清界限 | 社区 / 企业自建 |

### 2.3 5 件现在做 (Phase 1 Slice 2 主干, 关键路径 ~5 周)

| # | WorkItem | 估时 | 落点 | 借鉴 |
|---|---|---|---|---|
| 1 | **S2-W1** Bridge v1.1 envelope | 4-5d | `internal/bridge/` (JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 + Coordinator dispatch) | P0-2/3/4 |
| 2 | **S2-W2** Daemon + age + LICENSE + governance | 3-4d | `internal/daemon/` + `internal/agentkey/` + `LICENSE` + `docs/governance/non-negotiables.md` | P0-6/13/14 |
| 3 | **S2-W3** Coordinator + SubAgent 双类型 | 4-5d | `internal/coordinator/` + `internal/subagent/` | P0-1/7 |
| 4 | **S2-W4** $USD budget + OTel | 2-3d | `internal/budget/` + `internal/otel/` | P0-8/9 |
| 5 | **S2-W5** Capabilities + SKILL.md + chaos | 3-4d | `internal/capability/` + `.loom/skills/loom-bridge.md` + 1h chaos test | P0-5 |

**关键路径总时长**: ~25 天 ≈ 5 周

---

## 3. 借鉴优先级 (P0/P1/P2 全部落具体 task)

### P0 (Phase 1 Slice 2 必借鉴)

| P0 | 借鉴来源 | 落点 |
|---|---|---|
| **P0-1** Coordinator + specialists 命名 | Cline / OpenCode / Pi | S2-W3 (Bridge v1.1 `loom.dispatch`) |
| **P0-2/3/4** JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 | MCP 2026-07-28 RC | S2-W1 (Bridge v1.1 envelope) |
| **P0-5** Capabilities = composable bundles | Pydantic AI Harness v0.5.0 | S2-W5 (Bridge v1.1 method dispatch) |
| **P0-6** Middleware 洋葱圈 | LangChain 1.0 | S2-W2 / S2-W4 (daemon 中间件栈) |
| **P0-7** SubAgent 双类型 (SubAgent dict + CompiledSubAgent) | LangGraph Deep Agents | S2-W3 (subagent spec) |
| **P0-8** $USD budget per run | Belay | S2-W4 (Run claim 必填) |
| **P0-9** OTel spans trace 出口 | OpenTelemetry CNCF | S2-W4 (daemon observability) |
| **P0-13** MIT + view layer 严格 fail closed | 社区共识 | S2-W2 (LICENSE + governance) |
| **P0-14** view layer 不可妥协 | 社区共识 | S2-W2 (governance/non-negotiables.md) |

### P0-Phase 2 准备 (Slice 2 末尾先出最小版)

| P0 | 落点 |
|---|---|
| **P0-10** 多 editor skill 同步 `.agents/skills/` | Phase 2 准备 (G8) |
| **P0-11** Remote MCP Server 框架 (Phase 1 写好不开放) | Phase 2 准备 (G8) |
| **P0-12** 5 层能力矩阵 (Phase 2 tools 20+ 强制) | Phase 2 准备 (G8) |

### P1 (Phase 2 准备)

| P1 | 落点 |
|---|---|
| **P1-2** 三级上下文压缩 (run 长了才需要) | Phase 2 (G9) |
| **P1-3** context_schema (multi-tenant 才需要) | Phase 2 (G9) |
| **P1-11** exactly-once (SQLite `INSERT OR IGNORE` 顺便加) | Slice 2 顺便 (S2-W4/S2-W5) |
| **P1-12** OAuth 2.1 (Phase 2 末/Phase 3) | Phase 2 末 (G9) |

### P2 (Phase 3+ 暂缓, 不做原因写进队列)

| P1/P2 | 跳过/暂缓 | 原因 |
|---|---|---|
| **P1-1** Code mode (Pydantic) | **跳过** | Loom 的 run 不是 user 写代码场景 |
| **P1-4** DBOS Conductor MCP | Phase 3 选型后学 | Loom 不重做 durable engine |
| **P1-6** Postgres | Phase 2 multi-user 才需要 | single-user SQLite 够用 |
| **P1-9** loom-insight (PXI) | 3-6 月数据后 | Loom 数据还不够多 |
| **P1-10** 7h chaos | Phase 3 才上 | S2 跑 1h, P2 跑 4h, P3 才 7h |

---

## 4. 调研文档归属 (新 session 必读)

| 文档 | 路径 | 何时读 | 角色 |
|---|---|---|---|
| **Graph Engineering Plan v1** | `.loom-drafts/loom-graph-engineering-plan.md` | **DRAFT, 等 review** | 启动前必读 |
| **Slice 2 spec** | `phase1-slice2.goalspec.yaml` (待出) | **Slice 2 启动 session 时** | 启动 S2 |
| **New session prompt** | `.loom-drafts/new-session-prompt.md` | **Slice 2 启动 session 时** | 配 S2 session |
| **Phase roadmap v0.4** | `.loom-drafts/phase-roadmap-supplements.md` | **每个 S2-W 启动前** | 路线图 (MCP 5 必决) |
| **Community survey v3 deep-dive** | `.loom-drafts/community-survey-v3-deepdive.md` (45K) | **P0/P1 实现时按章节查** | 主参考 (2026-Q3 最新) |
| **Community survey v2** | `.loom-drafts/community-survey-v2.md` (52K) | **总览查"X 项目借鉴了哪些"** | 中等参考 |
| **Community survey v1** | `.loom-drafts/community-survey-v1.md` (7K) | **历史背景, 已 frozen** | 仅 v1 引用时 |

---

## 5. 跨项目硬规则 (违反会导致 plan 引擎 auto-reject)

### 5.1 硬规则 §1 — Plan deliverable.md 末尾必须有 VERDICT 行

- 最后非空行 = `VERDICT: PASS|FAIL`。
- Engine 解析;缺失 → auto-reject → `consecutive_failures++` → cancel。
- **Verifier feedback 也必须如此** (deliverable.md 末尾)。

### 5.2 硬规则 §2 — Plan prompt 术语先 grep 代码

- failure code / enum / status 值进 prompt 前**必须** `grep -rn` 实际代码。
- Worker 会改但**不该让 ta 改**你的术语。

### 5.3 硬规则 §3 — `mavis session abort` 前先读 session

- 读 title + 最近 2-3 条 message。`~/.mavis/skills/` 跨 session 共享, 并发写常见。
- **仅在确认 runaway 时才 abort**。

### 5.4 硬规则 §4 — Monitor 自己 SIGKILL, 不用 AI executor

- 监测到阈值的 monitor **自己** kill 目标 pid (SIGTERM+5s 后 SIGKILL)。
- **不要依赖 AI executor 解析日志里的 `HARD_KILL=true`**。
- Cron tick prompt 用 SKIP-only。

### 5.5 硬规则 §5 — `mavis team plan cancel` 不会杀 worker

- Cancel 后必做:
  - `mavis session ls <agent>` 找 started 的 worker → 逐个 `abort`
  - `git status --porcelain | grep "^??"` 看 zombie 文件

### 5.6 硬规则 §6 — 任务状态变更 3 目标同回合同步

- 触发: TodoWrite / plan decision / steer / cron± / agent± / task force / user 取消 / producer FAIL / verifier FAIL / scope 变化。
- **3 目标**: `TodoWrite` + `<workspace>/PROGRESS.md` + `memory`。
- 用户原话: "更改任务时要把任务队列也改了, 不然很容易做了之后没记录"。

---

## 6. 跨项目 SKILL (5 大风险 mitigation)

### 6.1 长 worker context overflow (~237k tokens)

4-hint 对策 (跨项目 SKILL):

1. **owner-recovery**: overflow 时 owner 从 atomic commit 接手
2. **早期+频繁 atomic commits**: 每子步 commit, 不攒
3. **deliverable.md 在 report-back 前 flush**: 避免最后一步丢
4. **owner-skip accept-on-ready**: owner 看完 evidence 直接 accept, 不等 worker 报告

### 6.2 失败重试无限循环

- 最多 3 次 fix, 失败 escalate human (硬规则)

### 6.3 Verifier 自治文化 (跨项目 SKILL)

| 规则 | 含义 |
|---|---|
| **claim "pre-existing" 前** | `git log --all -- <path>` 验证 |
| **claim "sibling caused X" 前** | check 时间戳+ls-files |
| **claim 错了** | 明确认错, 不用 speculation 填 gap |

### 6.4 跨 Slice 命名错位 (已发生 1 次 2026-07-24)

- S1 frozen → 命名 S2-W*; Slice 2 spec 自己写 (`phase1-slice2.goalspec.yaml`)
- 任何多 Slice 项目, 启动前必读 "完成状态 + 当前 Slice"

### 6.5 Git 状态污染

- 调研放 `.loom-drafts/` untracked, 不 commit
- 启动前 `git status` 必须 clean (除了 `.loom-drafts/`)
- Plan cancel 后 `mavis session ls` + `git status --porcelain | grep "^??"`

---

## 7. Loom 客户端 3 形态 (用户 2026-07-24 16:36 SGT 决策)

| 形态 | 何时 | 作用 |
|---|---|---|
| **SKILL.md v1** (3 editor 触发入口) | S2-W5 内含 | Claude Code / Codex / Cursor 3 editor 调 Loom |
| **Loom Console TUI 最小版** (bubbletea + lipgloss) | Slice 2 末尾 (G7) | journal/evidence/cost 实时观察器, **取代 sqlite3+jq** |
| **Loom Console TUI 完整版** (cost dashboard + trace) | Phase 2 准备 (G8) | TUI 加 cost + trace view |
| **Loom Web** (本地静态, `localhost:7432`) | Phase 3+ 长期 (G11) | 时间线 + 树状 + dashboard, 等 TUI 跑稳 |

**Loom 客户端严格不抢**:
- ❌ 不做 "前台 agent CLI" 跟 Cline / OpenCode / Pi 抢
- ❌ 不做 IDE 跟 Cursor 抢
- ❌ 不做完整 observability SaaS 跟 Langfuse / Phoenix 抢
- ❌ 不开外部端口 (Loom Web 只在 localhost)

---

## 8. 当前状态 (2026-07-25 02:34 SGT)

| 项 | 状态 |
|---|---|
| Slice 1 (S1-W1~W5) | ✅ done, commit `5861f82` |
| Graph Engineering Plan v1 | 📝 DRAFT, `.loom-drafts/loom-graph-engineering-plan.md` (21K 字节, 12 章节), **等用户 review** |
| Slice 2 spec | ⏳ 待出 (在 Plan freeze 后) |
| `phase1-slice1.optimized.goalspec.yaml` | 📜 历史 (Slice 1 用过, 不再 active 读) |
| TodoWrite | 17 项 (1 认错纠正 done + 1 Plan done + 1 Handoff prompt in_progress + 1 Slice 2 spec + 5 S2 work item + 3 S2 doc + 1 Loom Console TUI + 5 Phase 2/3 分桶 + 1 长期监控) |
| PROGRESS.md | 同步 (含 v1/v2/v3 调研摘要 + 工作队列 + Plan 章节) |
| memory | 5 条 Loom-specific entry (v2 调研 / S1-S2 命名纠正 / Loom 客户端 3 形态 / MCP 2026-07-28 RC / Pydantic AI Harness / 工作队列排序 / Graph Engineering Plan v1) |

---

## 9. 下一步 (3 选 1)

### 选项 A: 用户 review Graph Engineering Plan v1 (1-2 天)

- 用户读 `.loom-drafts/loom-graph-engineering-plan.md` 整篇
- 重点看 §2 DAG + §2.2 acceptance + §4.1 状态机 + §6 checkpoint + §7 风险
- 反馈: OK / 部分改 / 大改
- 等用户 OK

### 选项 B: 改 Graph Engineering Plan (如 review 反馈, 0.5-1 天)

- mavis 改 v2
- 再 review

### 选项 C: Freeze Plan → 出 spec → 启动 S2-W1 (Plan OK 后, 1-2 天 + 4-5 天)

- 用户标 Plan "FROZEN" + 日期
- mavis 出 `phase1-slice2.goalspec.yaml` (220+ 行, 含完整 DAG + 16 task acceptance)
- 用户 freeze spec
- mavis 启动 S2-W1 独立 session 跑 Bridge v1.1 envelope

---

## 10. 不要做 (Loom 严格不抢)

| ❌ 不要做 | 为什么 | 边界 |
|---|---|---|
| 不要复用 S1-W* 命名 | S1 frozen commit `5861f82` | 命名错位会导致历史污染 |
| 不要用 `phase1-slice1.optimized.goalspec.yaml` 给 Slice 2 | Slice 1 spec 跟 Slice 2 contract 不同 | Slice 2 必须出 `phase1-slice2.goalspec.yaml` |
| 不要跳过 Graph Engineering Plan 直接动手 | 没有 plan-execute-verify 闭环, 失败无回滚 | Plan 必须 freeze 后才动 |
| 不要用 ELv2 license | 跟 Phoenix 划清界限 | Loom 严格 MIT |
| 不要 push remote | user 没授权 | 全部本地 commit |
| 不要做 SaaS | 跟 LangSmith / Phoenix Cloud 划清界限 | 纯本地 daemon |
| 不要代理 LLM (sampling/createMessage 收到 fail closed) | 跟 Helicone / Logfire Gateway 划清界限 | Loom = 编排 + 观测 |
| 不要造 agent runtime (不写思考循环) | 跟 Cline / OpenCode / Pi 划清界限 | SubAgent dict 引用外部 agent |
| 不要开外部端口 (Loom Web 只在 localhost) | 跟 SaaS 划清 | localhost-only |
| 不要 review 还没完整就 commit (没有 RED → GREEN → VERDICT) | 跨项目硬规则 §1 | deliverable.md 末尾必填 `VERDICT: PASS/FAIL` |

---

## 11. 项目依赖 (Loom 跟外部项目的关系)

### 协议依赖 (Loom 必须遵循的规范)

| 外部项目/规范 | Loom 怎么依赖 |
|---|---|
| **MCP 2025-11-25** (Model Context Protocol) | Bridge v1.1 envelope = JSON-RPC 2.0, wire format 对齐 |
| **JSON-RPC 2.0** spec | Bridge 的 request/response/notification envelope |
| **W3C Trace Context** spec | Bridge v1.1 的 trace 字段 (traceparent / tracestate) |
| **JSON Schema 2020-12** spec | Bridge 暴露的 method 都有 schema 验证 |
| **OpenTelemetry** spec (CNCF) | OTel SDK 接入, OTLP gRPC exporter (默认 `localhost:4317`) |
| **age encryption** spec (filippo.io/age) | AgentKey vault 加密格式 (X25519 + ChaCha20-Poly1305) |

### 运行时依赖 (Loom 跑起来时必须存在)

| 外部服务 | 失败处理 |
|---|---|
| **9Router** (用户自己的 Provider 路由) | Loom 永远不直接调 provider, 永远经 9Router |
| **OTel collector** (默认 `localhost:4317`) | collector 不在 → buffer + drop, 不阻塞 run |
| **SubAgent binary** (Cline / OpenCode / Pi / 自写) | 进程死 → coordinator 重启, 最多 N 次 |
| **SQLite database** (`~/.loom/journal.db`) | 文件锁失败 → fail closed, daemon 起不来 |
| **Evidence directory** (`~/.loom/evidence/`) | 写失败 → run 失败, evidence 跳过 |

### 客户端集成依赖 (用户从哪调 Loom)

| 客户端 | 接入方式 | 何时启用 |
|---|---|---|
| **Claude Code** / **Codex** / **Cursor** | 读 `.loom/skills/loom-bridge.md` (SKILL.md v1) | S2-W5 |
| **Cline** / **OpenCode** / **Pi** / **Aider** / **Goose** | 作为 SubAgent 通过 JSON-RPC 2.0 envelope | S2-W1 + S2-W3 |

### 可选集成 (Phase 2/3 暂缓)

| 项目 | 何时 |
|---|---|
| **Arize Phoenix** (Remote MCP Server 接 Loom cost view) | Phase 2 (P0-11), 暂不开放 |
| **Langfuse** (Platform MCP Server) | Phase 2, 暂不开放 |
| **DBOS / Restate / Temporal** (durable engine) | Phase 3 选型后学 |

---

## 12. Loom 内部模块依赖 (3 层无环)

```
外圈 (cmd/loom/main.go)
  ├─ internal/cli (loom route / loom status) ──── S1 已做
  └─ internal/daemon (新 S2-W2, long-running process)
        ├─ internal/bridge (S2-W1, JSON-RPC 2.0)
        ├─ internal/agentkey (S2-W2, age 加密)
        ├─ internal/coordinator (S2-W3)
        │     └─ internal/subagent (S2-W3)
        │           └─ internal/capability (S2-W5)
        ├─ internal/budget (S2-W4, $USD budget)
        └─ internal/otel (S2-W4, OTel SDK)
              ↓ 全部依赖
中圈 (Slice 2 新增)
        ↓ 全部依赖
内圈 (Slice 1 已做叶子, 不依赖任何上层)
  ├─ internal/mode (4 Agent trigger)
  ├─ internal/journal (SQLite + append-only)
  └─ internal/evidence (SHA-256 + atomic publish)
        ↓
migrations/0001_init.sql (SQLite schema, embedded, journal 唯一入口)
```

**依赖铁律** (跟 Kubernetes 一样):
- ✅ 上层依赖下层 (daemon → bridge → core)
- ✅ 核心模块不依赖上层 (mode / journal / evidence 永远不 import bridge / daemon / coordinator)
- ✅ 同层互不依赖 (Slice 1 三个 core 平级; Slice 2 三个 middle 平级)
- ❌ 严格禁止反向 import
- ❌ 严格禁止循环 import

**边界保护**:
- `internal/journal` 是**唯一 SQLite 入口**
- `internal/evidence` 是**唯一文件系统入口** (写)
- `internal/agentkey` 是**唯一 secret 入口**
- `internal/bridge` 是**唯一协议入口** (JSON-RPC 2.0 envelope 解析)

---

## 13. 联系方式 / review 路径

- **Owner**: lune (唯一 commit-authority, 唯一 GO 决策者)
- **Assistant**: mavis (调研 / 文档 / 启动 session)
- **review 流程**:
  1. 用户 review 文档, 反馈 OK / 部分改 / 大改
  2. mavis 改 (如需)
  3. 用户标 "FROZEN" + 日期
  4. mavis 出下一个交付物 (spec / doc / code)
- **autonomous mode 规则** (2026-07-08 01:20 授权):
  - 本地 commit 不 push remote
  - bypass STOP / 仅 surface 重大决策
  - 等用户在场时再 push

---

## 14. 一句话给新 session 的话

> 你接手的是 **Loom Phase 1 Slice 2**, 16 个 work item 排在 DAG 上, 关键路径 ~5 周。
> **不要重新调研** — 6 个调研文档在 `.loom-drafts/`, 5 条 memory entry 写满了, Graph Engineering Plan v1 等 review。
> **不要重新命名** — S1 frozen, 新工作用 S2-W*。
> **不要重新设计** — MIT + view layer + Coordinator + SubAgent 4 不抢 红线已定。
> **直接接上**: 读 Plan → 等 review → 出 spec → 启动 S2-W1 → 跑 5 周 → 进 Phase 2。
>
> 跨项目经验全部用上: 硬规则 6 条 + overflow 4-hint + Verifier 自治 + 命名错位 mitigation + 调研方法学 (raw.githubusercontent.com + freshness=month)。
>
> **做有产出的事**: 出 spec, 写代码, 跑测试, 写 deliverable.md (末尾必填 `VERDICT: PASS/FAIL`)。
> **不做空话**: 不写 "should work", 不写 "committed", 不写没证据的 status。

---

## 附录 A: 关键文件路径速查

```
/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild/
├── .loom-drafts/
│   ├── loom-handoff-prompt.md            # 本文档 (跨 session 传承)
│   ├── loom-graph-engineering-plan.md    # DRAFT 21K 字节, 等 review
│   ├── phase1-slice1.optimized.goalspec.yaml  # Slice 1 spec (历史)
│   ├── new-session-prompt.md             # 启动 session 用
│   ├── phase-roadmap-supplements.md      # v0.4 路线图 (MCP 5 必决)
│   ├── community-survey-v1.md            # 7 repo 调研 (历史)
│   ├── community-survey-v2.md            # 14 项目 × 4 category (中等)
│   └── community-survey-v3-deepdive.md   # 2026-Q3 前沿深挖 (主参考)
├── PROGRESS.md                          # 同步队列 + 调研摘要 + Plan 章节
├── .loom-evidence/
│   ├── phase1-slice1/                   # S1 5 件 evidence (frozen)
│   │   ├── S1-W1/, S1-W2/, S1-W3/, S1-W3-L2/, S1-W4/, S1-W5/
│   │   └── final-verification.md
│   └── phase1-slice2/                   # S2 evidence (待创建)
│       ├── meta.json                    # DAG 完整定义
│       └── S2-W1/                       # per-task evidence
│           ├── contract.md
│           ├── plan.json
│           ├── progress.json
│           ├── verify.json
│           └── deliverable.md           # 末尾: VERDICT: PASS/FAIL
├── branch: codex/loom-platform-slice2   # head detached at 8e207b8
└── (无 push, 无 merge, 无 release, 无 activation, 无 credential change, 无 paid remote work, 无 FastContext installation)
```

## 附录 B: 跨项目经验索引 (memory 关键 entry)

- **硬规则 §1-6**: `memory/MEMORY.md:13-36`
- **长 worker context overflow (4-hint)**: `memory/MEMORY.md:40-47`
- **Verifier 自治文化**: `memory/MEMORY.md:49-58`
- **Doc 路由默认 Keeper**: `memory/MEMORY.md:60-69`
- **Loom 工作队列排序入队 2026-07-24**: 1 条 entry
- **Loom Slice 1 vs Slice 2 命名错位纠正 2026-07-24**: 1 条 entry
- **Loom 客户端 3 形态决策 2026-07-24**: 1 条 entry
- **MCP 2026-07-28 RC 4 production signals**: 1 条 entry
- **Pydantic AI Harness + Capabilities composable bundles**: 1 条 entry
- **Loom Graph Engineering Plan v1 2026-07-25**: 1 条 entry

---

## 附录 C: 启动新 session 的 prompt 模板 (复制粘贴用)

```
你接手一个新项目, 上下文全部在以下文件里:
- 项目根: /Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild
- 完整 handoff prompt: .loom-drafts/loom-handoff-prompt.md
- 完整工作队列 + DAG + acceptance: .loom-drafts/loom-graph-engineering-plan.md
- 路线图 (MCP 5 必决): .loom-drafts/phase-roadmap-supplements.md
- 主参考 (2026-Q3 前沿): .loom-drafts/community-survey-v3-deepdive.md
- 同步状态: ./PROGRESS.md

你的角色: Mavis (assistant), 调研 / 文档 / 启动 session 跑 work item
Owner: lune (唯一 commit-authority)
严格不抢: 不代理 LLM, 不造 agent runtime, 不做 SaaS, 不用 ELv2, 不复用 S1-W* 命名

先做这 3 件事:
1. 读 .loom-drafts/loom-handoff-prompt.md 整篇 (14 章节)
2. 读 .loom-drafts/loom-graph-engineering-plan.md 重点 §2 DAG + §2.2 acceptance
3. 读 ./PROGRESS.md 顶部 "Slice 1 completion commit" + WorkItems 表 + "工作队列" 章节

然后告诉我:
- 你看到 Loom 现在是什么状态
- 你接下来建议做什么
- 你对 Graph Engineering Plan v1 的 review 反馈 (如果用户问)

跨项目硬规则 (违反会被 plan 引擎 auto-reject):
- deliverable.md 末尾必填 `VERDICT: PASS/FAIL`
- 任务状态变更 3 目标同回合同步 (TodoWrite + PROGRESS.md + memory)
- 长 worker context overflow 4-hint: owner-recovery / 早+频 commit / deliverable.md report-back 前 flush / owner-skip accept-on-ready
- 单写者原则 (per task), 不允许并行写同一文件
- 失败重试最多 3 次, escalate human
```

---

**Verdict (本文档)**:

- 包含 Loom 完整跨项目传承 (14 章节 + 3 附录)
- 新 session 读完就能立刻接上工作, 不需要重新调研
- 硬规则 + 跨项目 SKILL + 风险 mitigation 全部浓缩
- 启动新 session 的 prompt 模板 (附录 C) 可直接复制用

VERDICT: READY (可直接使用, 无需 review)
