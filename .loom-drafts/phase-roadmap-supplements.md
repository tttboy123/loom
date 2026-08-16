# Loom Phase 1-4 路线图 · 工程实践吸纳与排除建议

> 草稿 v0.4 · 2026-07-24（v0.3 → v0.4：追加 MCP 协议预对齐调研 + 12 节 envelope diff + 5 个 Phase 1 前必须定的决策 + 推荐选项 C = Loom Bridge v1.1 对齐 JSON-RPC 2.0 + pin 2025-11-25）
> 目的：在不污染 `PRODUCT-PLAN.md` / `TECH-PLAN.md` 的前提下，整理从 Multica / Pi Agent / AgentScope 2.0 / grok-cli / ECC / OpenHands / Omnigent / Codex App / MCP 2025-11-25 / Claude Code Hooks / SKILL.md 等活跃工程中可借鉴与必须排除的实践。
> 写盘路径：`.loom-drafts/phase-roadmap-supplements.md`（不 commit，等 review）
> 状态：草案。条目仅作为 review 起点，不替代 `TECH-PLAN.md §14-§16` 的合同。

## 评估依据

| 工程 | License | Stars / Commits | 调研深度 | 关键摘要 |
|---|---|---|---|---|
| **Multica** | Apache 2.0 (modified) | 22.7k★ / 2.8k forks | 已有 `docs/architecture/multica-comparison.md` 完整对照 | Issue/Board + Agent 团队 + Skill 沉淀 + Runtime discovery |
| **Pi Agent (Earendil Works)** | MIT | 22k+ | 已定位 | coding agent CLI + 统一 LLM API + TUI + Web UI + 共享 session |
| **pi-coding-agent (Earendil Works)** | MIT | npm v0.78.1 (2026-06-06) | 已定位 | Pi Agent 底层 SDK，单独可作为 RuntimeProfile 候选 |
| **pi-agent-bus (kylebrodeur)** | 未明确 | 33 commits (2026-04→06) | 已定位 | MessageBus pub/sub 协调 Pi 生态；**不吸** |
| **AgentScope 2.0** | Apache 2.0 | 415 commits / 2026-07 集成 Daytona + K8s + OpenSandbox | 已定位 | Multi-tenancy + Sandbox 矩阵 + Extensible Middleware |
| **grok-cli (superagent-ai)** | MIT | 173 commits | 已定位 | Hooks 体系（PreToolUse/PostToolUse/SubagentStart/Stop 等） |
| **ECC (Everything Claude Code)** | MIT | 19万★ | 已定位 | SKILL.md 跨 IDE 通用，Skill 沉淀成熟实践 |
| **OpenHands** | MIT | 已 OpenDevin 改名 | 简要 | AI Safety Lab 合作，sandbox VM，--verify 工具 |
| **Agency Swarm** | MIT | 244 commits | 简要 | OpenAI Assistants API 依赖；**不吸** |
| **Paperclip** | MIT | 已定位 | 简要 | 学术研究 MCP 服务器，"虚拟公司" 治理；**不吸** |
| **CoPaw / OpenJarvis** | MIT | 已定位 | 简要 | 24/7 chat platform + cron；**不吸** |
| **PicoClaw** | 未明确 | 已定位 | 简要 | <10MB Go 嵌入式；**不吸** |
| **OpenManus / AutoGPT** | MIT | 已定位 | 简要 | fully autonomous；**不吸** |
| **TradingAgents / Dexter** | MIT | 6.3万★ / 已定位 | 简要 | 领域专用 multi-agent；**不吸** |
| **Omnigent** | 调研中 | 5.1k★ | 已定位 | meta-harness，把多个 Agent 当 Runtime 统一编排；Policies (ALLOW/DENY/ASK) + YAML Agent；alpha |
| **Codex App (OpenAI)** | 闭源 + 部分协议 | 2026-02-03 macOS 首发 | 已定位 | 多 Agent 并行 + Skills + 后台自动化；**Skills/Hooks 模式第三次被验证** |

---

## Agent Control Platform 通用设计模式（v0.2 追加）

agent control platform 不是一个项目，是一个**正在快速演化的类别**。把现有项目横向抽象一下，通常包含 5 层：

```text
┌─────────────────────────────────────────┐
│  5. Views/UI（TUI / Web / CLI）          │  ← Multica / Loom / Omnigent / Codex App
├─────────────────────────────────────────┤
│  4. Governance（Policy / Approval）      │  ← Omnigent Policies / Loom Rules
├─────────────────────────────────────────┤
│  3. Orchestration（调度 / 委派）         │  ← Multica Squad / AgentScope team tools
├─────────────────────────────────────────┤
│  2. Harness（meta-harness / 进程边界 /   │  ← Omnigent meta-harness / Pi / grok
│     hook）                                │     / Loom Bridge
├─────────────────────────────────────────┤
│  1. State Authority（Journal / DB /      │  ← Multica Postgres / Loom Event
│     Event Store）                          │     Journal / Omnigent ?
└─────────────────────────────────────────┘
```

### 各工程分层覆盖度

| 工程 | L1 State | L2 Harness | L3 Orchestration | L4 Governance | L5 Views |
|---|---|---|---|---|---|
| **Multica** | Postgres (mutable) | Runtime discovery | Squad | Issue workflow | Web + Daemon |
| **Pi Agent (Earendil)** | 进程内 state | CLI 自身 | 无 | 无 | CLI + TUI + Web |
| **Omnigent** | 未明 | meta-harness | 多 Agent YAML | Policies (3 态 × 3 层) | Web 工作台 |
| **Codex App (OpenAI)** | 本地 + 后台 queue | Codex 内核 | 多 Agent 并行 | 后台自动化 | macOS App |
| **AgentScope 2.0** | Toolkit + Service | Agent base + Middleware | team tools | tool permission | SDK + Web UI |
| **Loom (current plan)** | **Event Journal + Projection** | **Bridge stdio JSON** | Scheduler (planned) | **Customer Rule + require_approval** | TUI (Phase 2) |

### Loom 在 5 层的"特殊性"

| 层 | Loom 选择 | 比同类的优势 | 比同类的弱势 |
|---|---|---|---|
| **L1** | Event Journal + 可重建 Projection | 比 mutable DB 强（Multica 是 mutable） | — |
| **L2** | Bridge stdio JSON，**不实现** Runtime 内核 | 简洁、跨语言、Phase 2 可集成 pi-coding-agent | 不直接执行命令，必须委托 Runtime |
| **L3** | Scheduler + WorkItem DAG | 显式 DAG + dependency | Phase 3 才完整，Multica 已 production |
| **L4** | Customer Rule + require_approval | 简单、显式、可审计 | 比 Omnigent Policies 简单但少 3 态 × 3 层 |
| **L5** | 显式触发 + TUI | 默认不打扰用户 | 现阶段 TUI 弱，Web UI 待 Phase 4 |

### 关键观察

1. **Skills 抽象已是行业共识**：Codex App + ECC + Multica + Omnigent 四家工程殊途同归，Loom Phase 3 的"Skill 库"不需再辩论，是必选
2. **Hooks 抽象已是行业共识**：grok-cli + Codex App + Claude Code 三家都走 hook 体系，Loom Phase 2 的 hook 增补是必选
3. **YAML Agent 定义是低摩擦入口**：Omnigent / Codex App 都在用，Loom 考虑在 Phase 3 把 AgentDefinition 写成 YAML 友好（不是必须，YAML 解析带来的复杂度要权衡）
4. **meta-harness 是 Phase 2 的核心定位**：Loom 不应再造 Runtime 内核，应做"已有 Agent 的统一接入层"——这跟 Omnigent 的 meta-harness 思路一致
5. **State Authority 是 Loom 的最大差异点**：Event Journal + Projection 是 Loom 比 Multica 强的设计，应作为产品宣传的护城河

### "agent control platform" 类别的 7 个不变量（从现有工程归纳）

虽然不写到产品不变量文档里，这里作为**设计参照清单**：

1. **多 Agent 入口** — 让用户用自然语言选/创建 Agent（Codex App / Omnigent 都做了）
2. **Session 持久化** — 重启后能恢复（Multica / Hermes 都做了）
3. **Skill 沉淀** — 把成功经验转化为可复用单元（Codex / ECC / Multica / Omnigent 四家都做了）
4. **Policy 治理** — 区分 ALLOW / DENY / ASK 三态（Omnigent / Codex App）
5. **Hook 体系** — 让 Tool 调用可拦截、记录、增强（grok-cli / Codex App / Claude Code）
6. **跨设备** — 终端 / 浏览器 / 手机 / 桌面（Omnigent / Hermes / Codex App 都有）
7. **可观测** — token、cost、timeline、attention（Multica / Omnigent / Loom View Layer 都有）

### Loom 在 7 个不变量上的覆盖率（自检）

| 不变量 | Loom 现状 | Loom 计划 |
|---|---|---|
| 多 Agent 入口 | Phase 2 显式 trigger | Phase 2 Runtime discovery |
| Session 持久化 | Phase 1 Event Journal 已覆盖 | 不变 |
| Skill 沉淀 | — | **Phase 3 增补**（见下文） |
| Policy 治理 | Customer Rule（简化版） | 评估是否升级为 3 态 × 3 层（Phase 3 决策） |
| Hook 体系 | Bridge stdio JSON 无 hook | **Phase 2 增补**（见 v0.1 增补 ①） |
| 跨设备 | 不在 Phase 1-3 范围 | Phase 4 评估 |
| 可观测 | TUI + Projection | 不变（已是 Loom 强项） |

**覆盖率自检结论**：Loom 在 7 个不变量中已有 4 项覆盖、2 项在 Phase 2-3 增补路径上、1 项（跨设备）Phase 4 评估。**没有空白缺口需要重新设计**。

---

## Phase 1：维持现状

**当前范围**：Go 工程骨架 / 显式模式路由 / SQLite Event Journal / Evidence Artifact Store / 可重建投影 / 最小只读 CLI。

### 维持

- Phase 1 不引入任何上层 agent 概念。
- Multica 已吸收的机制（Runtime discovery / claim-lease / task-scoped token / managed workspace）**在 Phase 2 实施，不在 Phase 1**。
- `docs/architecture/multica-comparison.md` 已划定的不复制边界（PostgreSQL / Squad routing / 网络 terminal callback）持续生效。

### 主动排除（写入 Phase 1 约束）

| 来源 | 不吸原因 |
|---|---|
| Agency Swarm 的 OpenAI Assistants API 依赖 | Loom 不绑 Provider |
| AutoGPT / BabyAGI / OpenManus 的 fully autonomous loop | 与"显式 Agent trigger"不变量冲突 |
| OpenClaw / Hermes 的 24/7 vibe coding daemon | Loom 是 view layer，不做 chat platform |
| pi-agent-bus 的 MessageBus pub/sub | Loom 用 Event Journal 事实权威 + 投影语义，pub/sub 不适合做"事实来源" |
| Multica 的 PostgreSQL / Next.js / WebSocket | Loom 走 SQLite + stdio + 未来 TUI |

### 新增文档（建议）

不新增。Phase 1 已有 `TECH-PLAN.md §14 Slice 1` + `docs/architecture/multica-comparison.md` 足够。

---

## Phase 2：建议增补 3+1 项

**当前范围**：Credential Broker / 3 种认证模式 / 第 2 种 Agent Runtime / 成本与 fallback / TUI。

### 增补 ①：Runtime Hook 协议

**来源**：grok-cli 的 hooks 体系 + Earendil Works Pi 的 session lifecycle。

**动机**：Loom Bridge 当前是 JSONL stdio，**没有 hooks 概念**。Phase 2 真实执行时这会变成盲区——例如"用 LLM 生成的 SQL 先过 SQL 注入检测"、"SubAgent 启动时记录到 Evidence 投影" 等需求无落点。

**建议实现**：
- 新增 `internal/bridge/hooks.go`，定义 6-8 个核心 hook 事件
- 候选事件：`PreToolUse` / `PostToolUse` / `SubagentStart` / `SubagentStop` / `TaskCreated` / `TaskCompleted` / `PreCompact` / `SessionEnd`
- Bridge JSONL 协议加 1 个 `event` 字段（向后兼容）区分 `request` / `response` / `event`
- AgentGrant 重新审视：是否需要在 hook 调用时也校验？

**对应文档**：新增 `docs/architecture/bridge-hooks.md`（草稿后续补）

### 增补 ②：Sandbox 后端选型

**来源**：AgentScope 2.0 (Apache 2.0, 2026-07) 暴露的沙箱矩阵：Daytona / K8s / OpenSandbox / E2B / Docker。

**动机**：Phase 2 第 2 种 Agent Runtime 的实际部署需要沙箱隔离。AgentScope 2.0 已经做过 production 验证。

**建议实现**：
- 新增 `docs/integrations/sandbox-backends.md` 候选矩阵
- 5 个候选：本地 namespace / Docker / Firecracker / macOS sandbox-exec / Wasm
- **不在 Phase 2 定死**——只选型 + 留 adapter port
- 与 `internal/runtime/adapter/` 包结构对齐

**对应文档**：新增 `docs/integrations/sandbox-backends.md`（草稿后续补）

### 增补 ③：Provider routing 边界再明确

**来源**：Earendil Works Pi Agent 的 "unified multi-provider LLM API"。

**动机**：Earendil Pi Agent、grok-cli、Multica 都内置了 Provider 抽象。Loom 走 9Router。**这条要在 Phase 2 文档里再明确一次**，否则新人 onboarding 时会问"为什么 Loom 不直接做 Provider 抽象"。

**建议实现**：
- `TECH-PLAN.md §11 AgentGrant` 段落末尾加 1 句："Provider 路由由 9Router / Multica 处理；Loom 只见 Bridge 接口，不持有 Provider API key"
- 与 `docs/architecture/AGENT-KEY-VAULT.md`（如果存在）交叉引用

**对应位置**：`TECH-PLAN.md §11` 末段（不是新文件）

### 增补 ④：第 2 种 RuntimeProfile 候选 = pi-coding-agent

**来源**：Earendil Works `pi-coding-agent` (MIT, npm v0.78.1)。

**动机**：Pi Agent 的底层 SDK 已 production-ready、多 Provider 抽象、写好了 agent loop。**Loom 不需要自己写第二个 Runtime**——可以集成 `pi-coding-agent` 作为可选 RuntimeProfile，类似 Multica 把 Claude Code / Codex 当 Runtime 集成。

**建议实现**：
- 新增 `internal/runtime/adapter/pi/` 包，只做 **进程边界 + JSONL Bridge 适配**
- 不引入 TypeScript 依赖（Loom 是 Go），通过 `node` 子进程调用 `npx pi-coding-agent`
- Adapter 必须被 Loom 严格封装（`internal/runtime` 是 port，adapter 是实现），保持 Loom 不绑 Provider 的立场
- Phase 2 不强制启用，写为 `experimental` RuntimeProfile

**对应文档**：新增 `docs/integrations/pi-coding-agent.md`（草稿后续补）

### Phase 2 主动排除

| 来源 | 不吸原因 |
|---|---|
| Earendil Pi 的 "unified multi-provider LLM API" | 9Router / Multica 的工作，Loom 不重复 |
| Earendil Pi 的 "sharing real-world AI coding sessions" 平台 | Sidecar Phase 3 离线评测可引用其数据（带脱敏），不复制其平台 |
| Multica 的 Workspace / Issue / Board | Loom 用 Task Board（§14 Slice 5）但不做 Issue tracker |
| CoPaw / OpenJarvis 的 cron + heartbeat | Loom Phase 2 不做 24/7 后台 |
| AgentScope 2.0 的 multi-tenancy 服务 | Phase 4 再说 |

---

## Phase 3：建议增补 1 项

**当前范围**：Agent 库 / Sidecar 记忆与候选 / 离线评测 / 激活/回滚 / 多 WorkPackage。

### 增补 ①：Agent 库拆成 Agent + Skill 两层

**来源**：ECC (19万★) + Multica + AgentScope 2.0 的共同实践。

**动机**：3 个工程殊途同归地走了 SKILL.md / skill.yaml 路线。Loom Phase 3 的"Agent 库"如果只放 AgentDefinition，**会错过经验沉淀的最优单位是 Skill，不是 Agent**。

**建议实现**：
- **Agent 库** = AgentDefinition + RuntimeProfile 复用（已有）
- **Skill 库** = 可重用的 prompt 模板 + 工具配置 + 验收脚本（新增）
- Sidecar 不只评测 Agent，还评测 Skill
- 新增 `internal/skill/` 包；目录布局参考 `.loom-drafts/skill-layout.md`（后续补）

**对应文档**：修订 `PRODUCT-PLAN.md` §"Agent 库" + 新增 `docs/architecture/skill-registry.md`

### Phase 3 主动排除

| 来源 | 不吸原因 |
|---|---|
| Multica 的 Skills "团队共享"语义 | Loom 走单用户显式共享，团队共享留 Phase 4 |
| OpenHands 的 sandbox VM per session | 已在 Phase 2 sandbox 选型覆盖 |
| Hermes Agent 的 procedural memory 自动生成 | Sidecar 离线评测是人工/半自动，不做自动沉淀 |

---

## Phase 4：建议收敛而非扩展

**当前范围**：Provider 路由后端 / 协作平台适配 / 可选 Web UI / 导入导出与共享合同。

### 维持 + 主动排除（写入 §17 / 新章节）

| 来源 | 不吸原因 |
|---|---|
| Paperclip 的 "虚拟公司" 治理 | v1.0 前不要碰 |
| OpenHands 的 "24/7 cloud runtime" | Loom 是 local-first |
| Multica 的 Next.js Web UI | "可选" = 用户自部署、与 daemon 解耦、不引入云依赖 |
| AgentScope 2.0 的 multi-tenancy | Phase 4 写"协作平台适配"时**只**取 session 隔离 |
| Earendil Pi 的 web UI components | 不复制代码，**不**与 daemon 强绑定 |
| CoPaw / OpenJarvis 的 24/7 chat platform + cron | Loom 不做 chat platform |
| PicoClaw 的 <10MB Go 嵌入式 | Loom 面向 desktop，不需要 RISC-V 极致优化 |
| OpenManus / AutoGPT 的 fully autonomous | 与显式 Agent 不变量冲突 |
| ECC 的 19万星生态 | Loom 是产品不是 harness，ECC 可作为 client 集成层 |
| TradingAgents / Dexter 的领域专用 multi-agent | Loom 走通用 WorkPackage，不做行业垂直 |

### 可参考（不复制）

- **Earendil Pi 的 "sharing real-world AI coding sessions"** — Sidecar 离线评测数据来源（带隐私脱敏），不是新功能
- **AgentScope 2.0 的 Extensible Middleware System** — Phase 1/2 verification pipeline 已参考；Phase 4 不再扩展
- **Multica 的 WebSocket realtime** — Phase 4 TUI 实时投影可参考，**不**用 PostgreSQL NOTIFY

---

## Hook / Skill 标准实证（v0.3 追加）

> 这一节是 v0.2 "Hook 体系" 和 "Skill 沉淀" 行业共识的**实证展开**，给 Phase 2 增补 ① 和 Phase 3 增补 ① 提供**可直接对齐的 schema**。
> 调研依据：Claude Code 官方文档（code.claude.com/docs/en/hooks、/skills）+ Anthropic Agent Skills 开放标准（agentskills.io / docs.claude.com/en/docs/agents-and-tools/agent-skills/overview）+ Codex CLI 0.65+ / Codex App 2026-02+ 实践。

### 1. Hook 协议实证（Claude Code v2.1+ 标准）

**22 个生命周期事件**（按时间顺序）：

| 事件 | 触发时机 | 4 种类型都支持？ | 关键响应 |
|---|---|---|---|
| `SessionStart` | 会话开始/恢复/clear/compact | cmd / http | output_to_claude |
| `UserPromptSubmit` | 用户提交输入 | cmd / http | output_to_claude / decision |
| `PreToolUse` | 工具调用前 | cmd / http / prompt | **permissionDecision**: allow/deny/ask/defer |
| `PermissionRequest` | 权限对话框出现 | cmd | decision: approve/block |
| `PermissionDenied` | 自动模式拒绝 | cmd | — |
| `PostToolUse` | 工具调用成功 | cmd / http | output_to_claude |
| `PostToolUseFailure` | 工具调用失败 | cmd | — |
| `Notification` | Claude 发通知 | cmd / http | — |
| `SubagentStart` | 子代理启动 | cmd / http | — |
| `SubagentStop` | 子代理完成 | cmd / http / **prompt** | — |
| `TaskCreated` | 任务创建 | cmd / http | — |
| `TaskCompleted` | 任务标记完成 | cmd / http | — |
| `Stop` | Claude 完成响应 | cmd / http / **prompt** | decision: continue/stop |
| `StopFailure` | API 错误结束 | cmd | — |
| `PreCompact` | 压缩前 | cmd / http | — |
| `PostCompact` | 压缩后 | cmd / http | output_to_claude |
| `SessionEnd` | 会话终止 | cmd / http | — |
| `ConfigChange` | 配置文件变更 | cmd | — |
| `FileChanged` | 监视文件变更 | cmd | — |
| `WorktreeCreate / WorktreeRemove` | worktree 生命周期 | cmd | — |
| `Setup` | 仓库初始化 | cmd | — |
| `Elicitation / ElicitationResult` | MCP 输入请求 | cmd | — |

**4 种 Hook 类型**：

```text
type: "command"  →  shell 命令，stdin 接收 JSON
type: "http"     →  POST JSON 到 URL
type: "prompt"   →  注入 LLM 提示（仅 Stop / SubagentStop 有效）
type: "agent"    →  启动带工具的子代理
```

**PreToolUse 的关键响应字段**（最重要，决定 allow/deny）：

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow" | "deny" | "ask" | "defer",
    "permissionDecisionReason": "Human-readable reason"
  }
}
```

**退出码语义**：

- `0` = 成功，继续
- `2` = 阻塞（仅 `PreToolUse` 有效），stderr 反馈给 Claude
- 其他 = 非阻塞错误，记录但不中断

**Hook 配置位置**（4 级作用域）：

```text
~/.claude/settings.json                       # 用户级
{project}/.claude/settings.json               # 项目级
{project}/.claude/settings.local.json         # 项目本地（不提交）
{plugin}/hooks/hooks.json                     # 插件级
```

**Loom Phase 2 hook 增补的最小可行集（6-8 事件）**：

| 事件 | 用途 | 来源 |
|---|---|---|
| `PreToolUse` | 拦截危险/越权 Tool 调用 | grok-cli + Claude Code |
| `PostToolUse` | 自动格式化、记录审计 | grok-cli + Claude Code |
| `SubagentStart / SubagentStop` | 跟踪 SubAgent 生命周期 | Claude Code |
| `TaskCreated / TaskCompleted` | WorkItem 状态投影 | Claude Code |
| `Stop` | 完成前最终质量门禁 | Claude Code + grok-cli |
| `SessionEnd` | 清理 + 写 Evidence | Claude Code |

**与 Loom Bridge 的对接**：

- 当前 Bridge 是 `JSONL stdio`
- 新增 hook 事件用同一个 JSONL channel 加 `event` 字段区分
- **不**复制 Claude Code 22 事件全部 — 选 6-8 个核心，文档明确"Loom 实现的 hook 子集"
- **不**支持 `prompt` / `agent` 类型（Phase 2）— 只 `command` / `http`

### 2. SKILL.md 开放标准（Anthropic 2025-10-16 推出，2025-12-18 标准化）

**目录结构**（最小到完整）：

```text
my-skill/
├── SKILL.md          # 必需：YAML frontmatter + Markdown body
├── scripts/          # 可选：Python/Bash 脚本（确定性强）
├── references/       # 可选：长文档、API 手册
├── assets/           # 可选：模板、静态资源
└── examples/         # 可选：few-shot 示例
```

**SKILL.md 完整 frontmatter 规范**：

```yaml
# 必需
name: my-skill                      # kebab-case，1-64 字符
description: Use when [触发条件] to [目标] by [核心] while [限制]  # ≤1024 字符

# 推荐
when_to_use: "用户提到 X 时使用本 skill"   # 自然语言补充
paths: ["src/**/*.ts"]                  # glob 路径过滤
allowed-tools: "Read, Bash(git:*)"      # 工具白名单

# 高级
model: opus                           # 覆盖模型
context: fork                         # 在子代理隔离上下文运行
agent: Explore                        # 子代理类型
user-invocable: false                 # 仅 Claude 可调用
disable-model-invocation: true       # 仅手动 /skill-name 触发
shell: bash                           # 跨平台 (bash | zsh | powershell)
license: MIT
compatibility: "gpt-4, claude-3"
```

**渐进式披露（Progressive Disclosure）架构**：

```text
L1（常驻）: name + description       ~100 tokens / skill
L2（触发）: SKILL.md body            <500 行
L3（按需）: references/scripts/assets/  不计入常驻
```

**description + when_to_use 预算**：1536 字符 / skill（超出 → 触发失败）。

**跨平台兼容**（同一份 SKILL.md 不修改）：

| 平台 | 目录 |
|---|---|
| Claude Code | `~/.claude/skills/` 或项目级 `.claude/skills/` |
| Codex CLI | `~/.codex/skills/` 或 `~/.agents/skills/` |
| Codex App | 同上 + 插件市场 |
| Gemini CLI | 兼容 SKILL.md |
| Cursor | 兼容 SKILL.md（部分功能受限） |
| GitHub Copilot | 兼容 SKILL.md |

**Loom Phase 3 Skill 库映射**：

| Anthropic 字段 | Loom 内部表示 | 说明 |
|---|---|---|
| `name` | `Skill.Name` | kebab-case 唯一 |
| `description` | `Skill.Description` | 必须含触发关键词 |
| `allowed-tools` | `Skill.AllowedTools` | 字符串白名单 |
| `paths` | `Skill.PathFilter` | glob 数组 |
| `scripts/` | `Skill.ScriptsDir` | daemon 加载路径 |
| `references/` | `Skill.ReferencesDir` | 按需加载 |
| `model` | SkillBinding.Model | Runtime 绑定 |
| `agent` | SkillBinding.Agent | SubAgent 类型 |

**关键设计决策（Phase 3 实施前必须定）**：

- **是否要求 Skill 跨 Loom 内部 + Claude Code/Codex 都可读？** 推荐是（**Loom Skill = SKILL.md 兼容的子集**）
- **是否要 Loom 自己的 Skill 仓库（registry）？** Phase 4 才考虑，Phase 3 先用文件系统 + 显式 import
- **Skill 评估怎么打分？** 跟 AgentScope 2.0 的 Skill 评估对齐（用 LLM-as-a-Judge 测 description 触发准确率）

### 3. Codex 2026 新发现：Record & Replay

Codex 2026 推出 `Record & Replay` 功能：
- 用户演示一遍操作 → Codex 自动录制 → 封装为 Skill
- 极大降低 Skill 创建门槛（"演示即写"）
- 实际效果：剪视频、修报销、做报告等都能"看一遍就会"

**对 Loom 的影响**：

- Phase 3 计划可加 "Skill 录制器" — 用户演示 Loom 操作 → 自动生成 Skill YAML
- 但**不要**现在就做 — Phase 3 先把基础 Skill 库落地，录制器是 Phase 4+
- 这个功能让"Skill 沉淀"变得更民主化，Loom 跟上的话**门槛更低 = 护城河更深**

### 4. v0.3 总结

**新增的可对齐标准**：

1. **Hook 协议**：Claude Code 22 事件（Phase 2 用 6-8 个）
2. **SKILL.md**：Anthropic 开放标准，**Loom 必须兼容**（不只是参考）
3. **Cross-platform**：Skill 在 Claude Code / Codex / Cursor / Gemini CLI 通用 — Loom 走兼容路径
4. **渐进式披露**：3 层 token 预算分配 — Loom Skill 库的设计原则
5. **Record & Replay**：Codex 2026 录屏生成 Skill — Loom 未来功能（Phase 4+）

**Phase 2 增补 ① 修订建议**：

把"Runtime Hook 协议"细化为以下具体承诺：

```text
事件: PreToolUse, PostToolUse, SubagentStart, SubagentStop,
      TaskCreated, TaskCompleted, Stop, SessionEnd
类型: command (Phase 2) / + http (Phase 3)
不实现: prompt, agent (留给 Phase 4+)
配置:  ~/.loom/hooks/ (用户级) + {project}/.loom/hooks/ (项目级)
文档:  docs/architecture/bridge-hooks.md (新增)
```

**Phase 3 增补 ① 修订建议**：

```text
Skill schema: SKILL.md 兼容子集（必选字段 name/description）
目录:      {project}/.loom/skills/（项目级优先）
跨平台:    同时写一份到 ~/.claude/skills/ 目录（如果项目级）
评估:      description 触发准确率（Phase 3 末评估）
录制器:    Phase 4+（不进入 Slice 1）
```

---

## MCP 协议预对齐调研（v0.4 追加，2026-07-24）

> 背景：Loom Bridge Phase 1 已确定走 JSONL over stdio（`TECH-PLAN.md §7`）。
> 问题是：MCP 2024-11-05 → 2026-07-28-v2 已成为 AI 工具协议事实标准（Anthropic
> / OpenAI / Google / Microsoft 全员支持）。Phase 1 Bridge 现在要不要
> 预对齐 MCP 的 wire format？Phase 2 怎么"接入" MCP 生态？
> 结论先给：**Phase 1 envelope 加 `jsonrpc: "2.0"` 字段成本极低、未来收益极高**；
> Phase 2 走"Loom as MCP Host + Loom exposes MCP Server"双角色。

### 1. MCP 版本时间线（截至 2026-07-24）

| 版本 | 日期 | 关键变化 | 行业采用状态 |
|---|---|---|---|
| **2024-11-05** | 2024-11-25 | v1 首发，tools + resources + prompts + sampling | 起点 |
| **2025-03-26** | 2025-03-26 | Tool annotations（readOnlyHint / destructiveHint / idempotentHint / openWorldHint） | OpenAI 宣布支持 |
| **2025-06-18** | 2025-06-18 | Structured content（outputSchema + structuredContent）、audio content、tools.title | Google DeepMind + Microsoft 加入 |
| **2025-11-25** | 2025-11-25 | icons metadata、incremental scope consent (SEP-835)、tasks (实验性, SEP-1686)、JSON Schema 2020-12 默认 | 现行稳定版 |
| **2026-07-28** | 2026-07-28 (beta) | v2 SDK + 新 spec（已合并 schema 重构） | v1.x 仍为生产推荐，未来 6 个月继续修 bug + 安全更新 |

**关键观察**：

- v1 仍是被广泛生产采用的稳定线；v2 SDK beta 阶段，**v1.x 至少维护到 2026-12 之后**
- Loom Phase 1（2026 H2）应该 pin **v1 (2025-11-25)**
- v2 升级是 Phase 4 之后再考虑的事

### 2. MCP wire format 精确 schema（2025-11-25 现行）

**物理层**：newline-delimited JSON (JSONL) over stdio（每行一个完整 JSON 对象）

**消息层** — 三种 JSON-RPC 2.0 消息：

```typescript
// Request (MUST have id, MUST NOT be null)
{ "jsonrpc": "2.0", "id": "string|number", "method": "string", "params"?: { ... } }

// Response (same id as request, exactly one of result/error)
{ "jsonrpc": "2.0", "id": "string|number", "result"?: { ... }, "error"?: { code: number, message: string, data?: any } }

// Notification (MUST NOT have id)
{ "jsonrpc": "2.0", "method": "string", "params"?: { ... } }
```

**标准错误码**：

```text
-32700  Parse error          JSON 解析失败
-32600  Invalid Request      消息不符合 RPC 格式
-32601  Method not found     方法不存在
-32602  Invalid params       参数校验失败
-32603  Internal error      处理器异常
-32000 ~ -32099  Server error  业务级错误
```

**生命周期方法**（必实现）：

```text
initialize        request  → 客户端宣告 protocolVersion + capabilities
initialized       notification  → 客户端确认握手完成
shutdown          request  → 优雅关闭
exit              notification  → 进程退出
tools/list        request  → 列出可用工具
tools/call        request  → 调用工具
resources/list    request  → 列出资源
resources/read    request  → 读资源
resources/subscribe  request  → 订阅资源变化
prompts/list      request  → 列出 prompt 模板
prompts/get       request  → 获取 prompt
```

**传输层**：

- **stdio**（本地）：子进程 stdio pipe，每行一个 JSON
- **Streamable HTTP**（远程）：HTTP POST + SSE 流（SSE 已被官方弃用，转 streamable HTTP）
- HTTP 传输 MUST 走 OAuth 2.1（PKCE + RFC 8707 resource indicators）
- stdio 传输 SHOULD NOT 走 HTTP auth，凭据从环境取

### 3. Loom Bridge Phase 1 envelope vs MCP — 精确 diff

**Loom 当前 envelope**（`TECH-PLAN.md §7`）：

```json
{
  "protocol_version": "loom.bridge.v1",
  "message_id": "uuid",
  "correlation_id": "uuid",
  "work_item_id": "work-123",
  "run_id": "run-456",
  "claim_generation": 2,
  "runtime_instance_id": "runtime-local-codex",
  "sender_agent_instance_id": "agent-789",
  "seq": 7,
  "type": "dispatch | event | evidence | result | cancel | ack | heartbeat",
  "emitted_at": "2026-07-24T00:00:00Z",
  "payload": {}
}
```

**逐字段对照**：

| 维度 | Loom Bridge v1 | MCP v1 (2025-11-25) | 对齐 / 冲突 |
|---|---|---|---|
| 物理层 | JSONL over stdio | JSONL over stdio | ✅ 完全一致 |
| 协议层 | 自定义 envelope | JSON-RPC 2.0 envelope | ⚠️ 不兼容（但可桥接）|
| 消息 ID | `message_id` (uuid) | `id` (string\|number) | 🔁 rename |
| 关联追踪 | `correlation_id` | （无；自己塞 params 里） | 🆕 Loom 多了 |
| Run 标识 | `run_id` + `claim_generation` + `runtime_instance_id` | （无；用 session） | 🆕 Loom 多了 |
| 消息 kind | `type` 字段（dispatch/event/evidence/result/cancel/ack/heartbeat） | `method` 字段（动态字符串） | 🔁 概念重叠 |
| Payload | `payload: {}` 自由 | `params` / `result` / `error` 三选一 | 🔁 schema 不严格 |
| 错误码 | 散在 `payload` 里（推测） | `error.code: number` 标准码 | ⚠️ 没对齐 |
| 协议版本 | `protocol_version: "loom.bridge.v1"` | `protocolVersion: "2025-11-25"` | 🔁 字段名不同 |
| 生命周期 | Run claim 流程 | `initialize` / `initialized` / `shutdown` | ⚠️ 概念不同 |
| 通知（无 ID） | `type: event` + 没有 message_id 对应响应 | `notification`（无 id 字段） | ⚠️ 隐式 vs 显式 |

### 4. 三个预对齐选项 + 推荐

#### 选项 A：完全保留 Loom 自定义 envelope（不做对齐）

- 现状。Phase 2 接入 MCP 需要在 Bridge Adapter 加双向转换层
- 优点：保持 Loom 独特性（Run claim generation、evidence digest、ack）
- 缺点：MCP 生态 Server 不能直接对接 Loom，需要包装；外部 Client 不能直接消费 Loom Agent

#### 选项 B：双层 envelope（Loom 在外层，MCP 在内层 payload）

```json
{
  "jsonrpc": "2.0",
  "id": "loom-msg-uuid-7",
  "method": "loom.dispatch",  // Loom 保留 type 语义
  "params": {
    "loom": {                  // Loom 专属字段
      "protocol_version": "loom.bridge.v1",
      "run_id": "run-456",
      "claim_generation": 2,
      "runtime_instance_id": "runtime-local-codex",
      "work_item_id": "work-123"
    },
    "mcp": {                   // MCP 兼容字段
      "method": "tools/call",
      "params": { "name": "read_file", "arguments": { "path": "/foo" } }
    }
  }
}
```

- 优点：JSON-RPC 2.0 兼容；Phase 2 可以直接用现成 MCP SDK；Loom 独有字段仍在
- 缺点：envelope 体积变大；需要写明确的 schema 文档

#### 选项 C（**推荐**）：Loom Bridge v1.1 — JSON-RPC 2.0 envelope + 保留 Loom 字段

- 继承选项 B，但**重命名**核心字段对齐 MCP 习惯：
  - `message_id` → `id`（同时支持 string UUID 和 number）
  - `type: "ack"` → `result: {...}`（带同样 `id`）
  - `type: "dispatch"` → `method: "loom.dispatch"` + `params.loom = {...}`
  - `type: "event"`（单向）→ `method: "loom.event"`（无 `id`）= MCP notification
  - `type: "cancel"` → `method: "loom.cancel"` + 标准 `error.code: -32001`
  - `protocol_version` → 顶层 `jsonrpc: "2.0"`（固定）+ 业务字段保留 `loom.protocol_version`
- 优点：JSON-RPC 2.0 严格兼容；现有 MCP SDK（TS / Python / Java / Kotlin / C# / Swift）能直接 parse；Phase 2 零迁移
- 缺点：v1.0 已发的 Loom Agent 需要小幅兼容 shim

**建议落地路径**：

1. **Phase 1 Slice 1 之前**：在 `protocol/bridge/v1/` 加 `schema.json`，定义 v1.1
2. **Phase 1 Slice 1 实施时**：写入 S1-Wx contract 之前先冻结 v1.1 schema
3. **Phase 1 Bridge 实现**：先实现 v1.0 兼容 shim（在 v1.1 envelope 上去掉 `jsonrpc` 字段就是 v1.0 行为）
4. **Phase 2**：正式发布 v1.1，document 标注 "MCP 2025-11-25 compatible subset"

### 5. Phase 2 接入 MCP 生态的 3 种策略

#### 策略 1：Loom as MCP Host（消费 MCP Server）

- Loom 内部用 MCP Client SDK（TypeScript SDK `@modelcontextprotocol/client`）
- 每个 Agent Runtime 启动时按 `loom.toml` 配置 spawn 多个 MCP Client
- Runtime 收到 Loom dispatch 后，转成 MCP `tools/call` 调用外部 Server
- 优点：复用现成生态（filesystem / github / postgres / sqlite / fetch 都是现成 MCP Server）
- 缺点：依赖用户自己装 MCP Server；Runtime 启动开销

#### 策略 2：Loom exposes MCP Server（被外部 Client 消费）

- Loom 自带 MCP Server 端点（`http://localhost:port/mcp` over streamable HTTP）
- 把 Loom Agent / Skill / Evidence / Cost view 暴露成 MCP Tools / Resources / Prompts
- 外部 Cursor / Claude Desktop / VS Code Copilot 都能直接消费 Loom
- 优点：Loom 价值显性化（让用户在熟悉的 IDE 里用 Loom）
- 缺点：需要写 MCP Server 实现 + OAuth 2.1（不能走 stdio auth）

#### 策略 3（**推荐**）：双角色，Phase 2 早期策略 1，Phase 2 中后期加策略 2

- 复用 FastMCP / TypeScript SDK / Spring AI MCP starter
- 阶段 1（Phase 2 早期）：让 Loom Agent Runtime 可调用任意 MCP Server（策略 1）
- 阶段 2（Phase 2 中后期）：让 Loom 自己作为 MCP Server 暴露能力（策略 2）
- 阶段 3（Phase 3）：在 Loom Skill 库里允许写"Loom Skill 同时也是 MCP Server 端点"

### 6. Tool annotations 直接映射 Loom risk levels

| MCP annotation (2025-03-26+) | Loom Risk Level (Slice 1) | 实际意义 |
|---|---|---|
| `readOnlyHint: true` | STANDARD | 只读，安全自动执行 |
| `readOnlyHint: false` + `destructiveHint: false` | STANDARD | 写入但可逆 |
| `destructiveHint: true` + `idempotentHint: true` | STRICT | 破坏性但幂等，retry 友好 |
| `destructiveHint: true` + `idempotentHint: false` | CRITICAL | 破坏性 + 非幂等，需要客户审批 |
| `openWorldHint: true` | （叠加）| 与外部世界交互，需要审计 |

**对 Loom 增强**：

- `loomspec/skill/loomskill.schema.json` 可以加 `loom:risk` 字段直接对应 MCP annotations
- `loom-rules.yaml` 写策略时直接引用 annotation 名称（更直观）
- Phase 3 Skill 库可以让用户用 MCP tool annotations 生成 Loom Risk 字段

### 7. Loom Subagent ≠ MCP Sampling（明确区分）

| 概念 | MCP Sampling | Loom Subagent |
|---|---|---|
| 方向 | Server → Client（请求 Client 调用 LLM）| Planner → Planner（自己 spawn 自己的子任务）|
| 目的 | 让 Server 委托 LLM 推理 | 让 Planner 委派子任务给 Executor |
| 生命周期 | 一次性 LLM 调用 | 长生命周期 Run（含 claim/heartbeat/terminal）|
| 输出 | LLM tokens | Evidence + Run terminal |
| 权限 | 由 Host 设定 sampling 审批 | 由 AgentGrant 限定 Loom 本地能力 |
| 实现基础 | MCP `sampling/createMessage` 方法 | Loom Run 认领 + claim_generation |

**结论**：Loom Subagent 是 Planner 自己的子 Run（不通过 Bridge 出 Loom），跟 MCP Sampling 是正交概念。Bridge 协议中**不需要**实现 Sampling 原语；Phase 2 MCP 嵌入时，Loom 收到 `sampling/createMessage` 请求应该**直接拒绝**（"Loom 拒绝代理 LLM"）以保持 Loom = 编排 + 观测，Provider 路由不沾手。

### 8. Structured content（2025-06-18）对 Loom 的影响

- MCP `tools/call` 现在支持 `structuredContent`（JSON）+ `outputSchema`（JSON Schema 2020-12）
- Loom Evidence 已经是结构化（digest + content type + bytes）
- **建议**：Loom Bridge 的 `result` payload 也加 `structuredContent` 字段（即使 v1.0 没用到），给 Phase 2 MCP 兼容预留扩展点
- 工具描述应该带 `outputSchema`（类似 Multica 的 Skill schema）方便 Host 做自动校验

### 9. Experimental Tasks（2025-11-25 SEP-1686）— Phase 4 候选

- MCP 2025-11-25 引入实验性 Tasks：跟踪 durable 请求 + 轮询 + 延迟结果检索
- 这跟 Loom 的"Run 长时间任务"语义高度重合
- **建议**：Phase 4 之后研究 Loom 是否需要支持"轮询式 long-running task"，还是用现有 heartbeat 机制就够
- **不进入 Phase 2/3**：Tasks 还是 experimental，v1 spec 还不稳定

### 10. 来源 + 版本 pin

| 资源 | URL | 调研时间 |
|---|---|---|
| MCP 官方规范 (2025-11-25) | https://modelcontextprotocol.io/specification/2025-11-25/basic | 2026-07-24 |
| MCP TS SDK (v1.x 稳定) | https://github.com/modelcontextprotocol/typescript-sdk (1,578 commits) | 2026-07-24 |
| MCP v2 SDK beta | @modelcontextprotocol/server@2 + @modelcontextprotocol/client@2 | 2026-07-24 |
| MCP 2025-11-25 changelog | https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/specification/2025-11-25/changelog.mdx | 2026-07-24 |
| MCP 官方注册中心 | https://registry.modelcontextprotocol.io/ | 2026-07-24 |
| FastMCP (Python 高层封装) | github.com/jlowin/fastmcp | 2026-07-24 |
| Spring AI MCP starter | https://docs.spring.io/spring-ai/reference/api/mcp/mcp-overview.html | 2026-07-24 |

**Loom 版本 pin 建议**：

- Phase 1（2026 H2）：Bridge v1.1 兼容 MCP **2025-11-25** v1
- Phase 2（2027 H1）：继续 v1.1 + 监控 v2 SDK GA
- Phase 3+：v2 SDK 评估升级（破坏性变更需要 Loom Bridge v2.0）
- v2 spec（2026-07-28）beta 出来后**不立即跟进**，等 GA + 6 个月生产验证

### 11. 关键决策（Phase 1 实施前必须定）

1. **Loom Bridge v1.1 是否对齐 JSON-RPC 2.0？** 强烈推荐**是**（选项 C）
2. **Phase 2 MCP 嵌入策略？** 强烈推荐**策略 3**（双角色：先消费后暴露）
3. **MCP version pin？** 选 **2025-11-25**（v1 现行稳定 + Anthropic / OpenAI / Google / MS 全员支持）
4. **Loom 拒绝代理 LLM？** 强烈推荐**是**（`sampling/createMessage` 必须 fail closed，保持 Loom = 编排 + 观测）
5. **Tool annotations ↔ Loom Risk 字段？** 强烈推荐**加**（`loom:risk` 字段直接对应 MCP annotations）

### 12. 未来 Loom 文档待补

- `docs/architecture/bridge-envelope-v1.1.md`（推荐新增，对应选项 C 落地）
- `docs/integrations/mcp-host.md`（Phase 2 策略 1）
- `docs/integrations/mcp-server.md`（Phase 2 策略 2）
- `docs/integrations/mcp-version-pin.md`（版本 pin 政策）
- `TECH-PLAN.md §7`（修订：v1.0 → v1.1 + JSON-RPC 2.0 envelope）
- `TECH-PLAN.md §11`（追加：MCP 2025-11-25 alignment 声明）

---

## 跨 Phase 决策

### 永久不吸清单（v1.0 前不动）

- **CoPaw / OpenJarvis** 的 24/7 chat platform 模式
- **PicoClaw** 的 <10MB 嵌入式
- **OpenManus / AutoGPT / BabyAGI** 的 fully autonomous
- **Paperclip** 的虚拟公司治理
- **OpenClaw / Hermes Agent** 的 24/7 vibe coding daemon
- **Pi Agent Bus 的 MessageBus pub/sub**（与 Event Journal 冲突）

### 永久借鉴清单

- **Multica** 的 Runtime discovery / claim-lease / task-scoped token / managed workspace
- **ECC / Multica / AgentScope** 的 SKILL.md + Toolkit 抽象
- **AgentScope 2.0** 的 sandbox 矩阵（参考但不绑定）
- **grok-cli** 的 hooks 生命周期（参考协议设计）
- **Earendil Pi** 的底层 SDK 作为可选 RuntimeProfile（不绑 Provider）
- **OpenHands** 的"prove it works" 工具（已被 Loom deterministic acceptance 覆盖）

---

## 待补的草稿

- [ ] `docs/architecture/bridge-hooks.md`（Phase 2 增补 ①）
- [ ] `docs/integrations/sandbox-backends.md`（Phase 2 增补 ②）
- [ ] `docs/integrations/pi-coding-agent.md`（Phase 2 增补 ④）
- [ ] `docs/architecture/skill-registry.md`（Phase 3 增补 ①）
- [ ] `TECH-PLAN.md §11` 末段修订（Phase 2 增补 ③）
- [ ] `PRODUCT-PLAN.md` §Agent 库 拆分（Phase 3 增补 ①）
- [ ] `TECH-PLAN.md §17`（新章节：Phase 4 不吸清单）

---

## 来源链接

- Multica: https://github.com/multica-ai/multica
- Pi Agent (Earendil Works): https://earendil-works/pi-coding-agent (npm)
- pi-agent-bus: https://github.com/kylebrodeur/pi-agent-bus
- AgentScope 2.0: https://github.com/agentscope-ai/agentscope
- grok-cli: https://github.com/superagent-ai/grok-cli
- ECC: https://github.com/.../ecc（占位）
- OpenHands: https://github.com/All-Hands-AI/OpenHands
- Agency Swarm: https://github.com/devtechdigital/agency-swarm
- Paperclip: https://github.com/.../paperclip
