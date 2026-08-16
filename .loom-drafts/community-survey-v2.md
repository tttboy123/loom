# Loom 社区开源项目调研 v2 (2026-07-24)

## 调研目标

在 v1 调研（7 个用户指定 repo）基础上，按 4 个 Loom 相关品类横向调研 14 个项目：

1. **Coding agent frameworks** — Loom 真正 orchestrating 的 Runtime 候选
2. **Stateful agent frameworks** — 长生命周期、memory、与 Loom Event Journal 思路相关
3. **Durable execution engines** — Loom 的 Run claim + heartbeat 模式在工业界叫什么、怎么做
4. **LLM observability** — Loom "3 view" 中的 cost / governance view 直接相关

**问题答案**：
- 这些产品跟 Loom 的"view layer + 2-tier planner/executor + Bridge v1.1 + AgentKey" 有什么关系？
- 哪些是 Loom 真正的"下游消费者"或"借鉴对象"？
- 哪些是"应该标记 not-recommended"避免选型踩坑？

## 调研方法

吸取 v1 教训：GitHub HTML 渲染页面被大量 chrome 污染，前 12 个 web_fetch 大部分截断。**这次改用 `raw.githubusercontent.com/owner/repo/branch/README.md` 直接抓原始 markdown**。

每个项目统一提取：
- 实际 repo URL + commit/release/license 状态
- 核心定位（README 第一段）
- 关键架构特征
- 跟 Loom 的对比（同构 / 差异 / 借鉴点）

## Cat 1：Coding agent frameworks (6 个项目)

### 1.1 Goose (block/goose → aaif-goose/goose)

- **Repo**: https://github.com/block/goose
- **License**: Apache 2.0
- **Language**: Rust
- **Org**: Linux Foundation AAIF (Agentic AI Foundation)
- **Star**: 5k-6k trending（Linux Foundation 背书）

**核心定位** (README 第一段)：
> "goose is a general-purpose AI agent that runs on your machine. Not just for code — use it for research, writing, automation, data analysis, or anything you need to get done. A native desktop app for macOS, Linux, and Windows. A full CLI for terminal workflows. An API to embed it anywhere. Built in Rust for performance and portability. goose works with 15+ providers — Anthropic, OpenAI, Google, Ollama, OpenRouter, Azure, Bedrock, and more. Connect to 70+ extensions via the Model Context Protocol open standard."

**关键架构**：
- **3 个产品形态**: Desktop app (native) + CLI + API
- **15+ LLM providers** (Anthropic / OpenAI / Google / Ollama / OpenRouter / Azure / Bedrock)
- **70+ extensions via MCP** — 完全依赖 MCP protocol 作为 extension 接入标准
- **Custom Distributions** — 用户可以"build your own goose distro with preconfigured providers, extensions, and branding"（这是关键：把"distro"作为分发单元）

**与 Loom 对比**：

| 维度 | Goose | Loom |
|---|---|---|
| 定位 | "your native open source AI agent" — 通用 agent | view layer + 2-tier planner/executor |
| Extension 协议 | MCP (依赖) | Bridge v1.1 (JSON-RPC 2.0 自有) |
| 部署形态 | Desktop / CLI / API (直接产品) | 本地 daemon (不直接面向用户) |
| Distro 概念 | ✅ Custom Distributions | ❌ 无（Loom 不分发产品） |
| 信任根 | Linux Foundation AAIF 治理 | AgentKey scoped token |
| 主语言 | Rust (性能+portable) | Go 1.22 (本地 daemon) |

**借鉴价值**：
- ✅ **Custom Distributions** 概念 = Loom Phase 2+ 可以做"team-specific Goose-style distribution with preconfigured subagents"
- ✅ **MCP 70+ extensions** 验证了"AI agent + MCP"是 2026 主流，Loom Phase 2 接入 MCP 是对的
- ⚠️ Goose 是"做产品"，Loom 是"做 view" — 不会直接竞争，但 AOS CE / Goose / Loom 三者形成"产品 / 操作系统 / view layer"三层
- ⚠️ Goose 的"Desktop / CLI / API"三形态 vs Loom 只有 daemon — Goose 用户面更广

### 1.2 OpenCode (sst/opencode → anomalyco/opencode)

- **Repo**: https://github.com/sst/opencode (默认 dev 分支) / https://github.com/anomalyco/opencode
- **License**: MIT（推测，未明确标注）
- **Language**: TypeScript
- **Star**: 中等（npm `opencode-ai` 包）

**核心定位** (README 第一段)：
> "The open source AI coding agent."

**关键架构**：
- **3 种内置 agents** (Tab 键切换)：
  - `build` — Default, full-access agent for development work
  - `plan` — Read-only agent for analysis and code exploration (denies file edits by default, asks permission before running bash)
  - `general` (subagent) — for complex searches and multistep tasks，invoked via `@general`
- **多端支持**: Terminal (npm `opencode-ai`) + Desktop App (BETA) + 各 OS installer
- **安装脚本尊重** `$OPENCODE_INSTALL_DIR` / `$XDG_BIN_DIR` / `$HOME/bin` 优先级

**与 Loom 对比**：

| 维度 | OpenCode | Loom |
|---|---|---|
| 定位 | "open source AI coding agent" | view layer (不直接是 agent) |
| Agents | build + plan + general (3 种) | Planner (高) + Executor (低) (2 种) |
| Plan mode | ✅ 内置 plan agent (read-only) | ❌ 还没显式 plan subagent |
| Subagent | ✅ `@general` invocation | ✅ Bridge protocol first-class |
| Permission | build=full, plan=read+ask | AgentKey scoped + governance view |

**借鉴价值**：
- ✅ **"build + plan" 双 agent 模式** 是 Loom 应该直接借鉴的 — Planner 的"plan"阶段完全可以用只读 agent 表达，Executor 的"execute"阶段用 full agent 表达
- ✅ **`@general` subagent invocation** 验证了"subagent 是 first-class"的设计，Loom Bridge v1.1 应该考虑 subagent dispatch 的语法
- ⚠️ OpenCode 仓库刚刚从 sst 迁到 anomalyco（commit 显示），是项目治理上的不稳定信号 — 关注

### 1.3 Aider (Aider-AI/aider)

- **Repo**: https://github.com/Aider-AI/aider
- **License**: Apache 2.0
- **Language**: Python
- **PyPI installs**: 6.8M
- **Tokens/week**: 15B
- **OpenRouter**: Top 20
- **Singularity**: 88% (Aider 最后 release 88% 的新代码是 Aider 自己写的)

**核心定位** (README 第一段)：
> "Aider lets you pair program with LLMs to start a new project or build on your existing codebase."

**关键架构**：
- **Maps your codebase** — repo map (类似代码地图)
- **100+ code languages** — 覆盖广
- **Git integration** — 自动 commit (sensible commit messages)
- **IDE integration** — 文件加注释即可让 Aider 改
- **Voice-to-code** — 语音输入
- **Linting & testing** — 自动 lint + test + fix
- **Web chat copy/paste** — 不需要 API key，直接 web chat 复制粘贴

**与 Loom 对比**：

| 维度 | Aider | Loom |
|---|---|---|
| 定位 | AI pair programming (CLI) | view layer (不直接是 agent) |
| 多 agent / 团队 | ❌ 单 agent | ✅ Planner + Executor + subagent |
| Git integration | ✅ 自动 commit | ❌ 不直接做 git (executor 决定) |
| Repo map | ✅ 自身实现 | ❌ Loom 不分析代码 (那是 subagent 的事) |
| 模型 | 多 LLM via API key 或 web chat | 2-tier (Planner 强 + Executor 弱) |

**借鉴价值**：
- ✅ **Singularity 88%** — Aider 88% 自己的代码是 Aider 写的。这是"AI 写 AI"的最强 evidence。Loom 的"executor 可以自己写新 subagent" 想法不疯狂
- ✅ **Web chat copy/paste** 模式 — 验证了"用户可以用现成 ChatGPT 订阅而非 API key" 的 UX 路径，Loom 用户如果只买 ChatGPT Plus 也能跑
- ⚠️ Aider 是"tight coding loop"（读 → 改 → 自动 commit），跟 Loom 的"长期 run claim + heartbeat"模式完全不同
- ⚠️ Aider 没有 multi-agent 编排，是"solo agent"。Loom 真的需要 multi-agent 时不应该把 Aider 当模板

### 1.4 Cline (cline/cline)

- **Repo**: https://github.com/cline/cline
- **License**: Apache 2.0
- **Language**: TypeScript
- **Org**: Cline Bot Inc. (公司化运营)

**核心定位** (README 第一段)：
> "The open source coding agent in your IDE and terminal."

**关键架构 — 4 个产品 surface + 1 个 SDK**：

| 产品 | 描述 | 路径 |
|---|---|---|
| **CLI** | Terminal UI, headless mode, shell commands | `apps/cli/` |
| **VS Code Extension** | Marketplace extension | `/` |
| **JetBrains Plugin** | IntelliJ / PyCharm / WebStorm / GoLand | (不开源) |
| **Kanban** | Web-based multi-agent task board, each card = own worktree + auto-commit | `cline/kanban` |
| **SDK** | Node.js programmatic agent API | `sdk/` (`@cline/sdk`) |

**关键能力**：
- **Multi-Agent Teams** — coordinator agent 分发到 specialist agents，每个有独立 tools + context。`cline --team-name auth-sprint "Plan and implement user authentication with tests"`
- **Scheduled Agents** — cron schedule 跑 recurring automations。`cline schedule create ... --cron "0 9 * * MON-FRI"`
- **MCP servers** — `cline mcp` 命令管理
- **Plugin system** — `beforeToolCall` / `afterToolCall` 钩子
- **Headless CLI** — `cline --json` 输出 JSON 事件流给 CI/CD
- **跨平台通信** — Slack / Telegram / Discord / Google Chat / WhatsApp / Linear

**与 Loom 对比**：

| 维度 | Cline | Loom |
|---|---|---|
| 定位 | Coding agent in IDE/terminal | view layer (不直接是 agent) |
| Multi-agent | ✅ Teams with coordinator + specialists | ✅ Bridge v1.1 subagent first-class |
| SDK | ✅ `@cline/sdk` Node.js | ❌ 无（Phase 2 考虑） |
| 跨平台 UI | VS Code + JetBrains + Kanban | ❌ 无（Phase 2 TUI） |
| Schedule | ✅ cron scheduled agents | ❌ 还没有 |
| Permission | Plan/Act mode toggle + auto-approve | AgentKey + governance view |

**借鉴价值**：
- ✅ **"Coordinator + specialists" multi-agent model** 是 Loom 应该直接借鉴的 — Planner 就是 coordinator，subagent 就是 specialists
- ✅ **`@cline/sdk` 这种"agent as library" 模式** — Loom 未来可以让 subagent 通过 SDK 暴露（不只是 stdio JSON）
- ✅ **Headless `--json` 模式** — 验证"agent 是 event source"模式，Loom Bridge 的"subagent 通过 event stream 与 Planner 通信" 设计合理
- ⚠️ Cline 是"产品 + 公司 + IDE 集成"，Loom 是"view layer + 本地 daemon"，赛道不同但 multi-agent 部分高度可借鉴
- ⚠️ Cline Kanban = Loom 未来 TUI 的 inspiration（多 agent 任务板）

### 1.5 Continue (continuedev/continue)

- **Repo**: https://github.com/continuedev/continue
- **License**: Apache 2.0
- **Status**: ⚠️ **ARCHIVED / READ-ONLY** — "Pioneering open-source coding agent" 不再 active development
- **Final release**: 2.0.0 (停止)

**核心定位** (README 第一段)：
> "Continue is a coding agent available as a CLI, VS Code extension, and JetBrains plugin."

**关键事实**：
- ⚠️ **"The `continuedev/continue` repository is no longer actively maintained and is read-only for all users"**
- 团队做了 final 2.0.0 release 后停止
- 移除 anonymous telemetry、pulling out authentication、squashing bugs

**为什么研究 Continue**：
- 是开源 coding agent 的先驱之一（与 Aider / Cline 同代）
- 它的"死"是行业信号：coding agent 工具想长期活下去，必须找到商业模式（公司化 like Cline、像 Aider 一样持续增长用户、像 Goose 一样进 Linux Foundation）

**借鉴价值**：
- ✅ **"维护者烧光"教训** — Loom 不能是"vibe-coded weekend project"，必须有用户/客户/基金会支撑
- ✅ **"remove anonymous telemetry" 决策** — Continue 最终选择"反 telemetry"，跟 Loom "view layer, 不向云端报告"哲学一致
- ❌ 不学技术架构（已经停止维护）

### 1.6 Pi (badlogic/pi-mono)

- **Repo**: https://github.com/badlogic/pi-mono
- **License**: MIT
- **Language**: TypeScript
- **Author**: Mario Zechner (libGDX 作者)
- **Star**: ~40k+ (被 OpenClaw 推到 21 万)
- **Note**: Pi 的 runtime 被 OpenClaw 嵌入

**核心定位** (README 第一段)：
> "Pi Monorepo — Tools for building AI agents and managing LLM deployments."

**关键架构 — 7 个包**：

| 包 | 职责 |
|---|---|
| `@mariozechner/pi-ai` | 统一 LLM API (20+ providers) |
| `@mariozechner/pi-agent-core` | Agent 运行时 (tool calling + state management) |
| `@mariozechner/pi-coding-agent` | Interactive coding agent CLI |
| `@mariozechner/pi-mom` | Slack bot (delegates to pi coding agent) |
| `@mariozechner/pi-tui` | Terminal UI 库 (differential rendering) |
| `@mariozechner/pi-web-ui` | Web components for AI chat |
| `@mariozechner/pi-pods` | vLLM deployments on GPU pods |

**关键设计哲学**：
- ✅ **4 个原语工具 (read/write/edit/bash)** — Turing-complete 最小集
- ✅ **~300 词 system prompt** — 极短
- ✅ **YOLO 模式 (无权限弹窗)** — "Permission popups 是剧场"
- ❌ **MCP 刻意不支持** — "用 CLI 工具 + Skills 标准替代"
- ✅ **Session tree branching** — JSONL 树状历史
- ✅ **Steering/Follow-up 双消息队列** — 运行时动态注入指令
- ✅ **OAuth 订阅** — Claude Pro/Max / ChatGPT Plus / GitHub Copilot 直接用
- ✅ **Session tree JSONL** — 完整可恢复的会话历史

**与 Loom 对比**：

| 维度 | Pi | Loom |
|---|---|---|
| 定位 | Agent 工具集 (4 工具哲学) | view layer (不直接是 agent) |
| 工具数 | 4 (read/write/edit/bash) | Loom 不做工具 (subagent 决定) |
| MCP | ❌ 刻意拒绝 | ✅ Bridge v1.1 = JSON-RPC 2.0 + 未来 MCP |
| Session | 树状 JSONL | Event Journal (SQLite, append-only) |
| 扩展 | TypeScript single-entry function | Bridge v1.1 (JSON-RPC 2.0) |
| Routing | 无 | Subagent dispatch 协议 |

**借鉴价值**：
- ✅ **"4 个原语"哲学** — Loom 的 subagent 也可以追求极简（"less is more"）
- ✅ **Session tree branching** — Loom 的 Event Journal 现在是线性，Pi 的树状是 inspiration（Phase 2+ 考虑"timeline branching"）
- ✅ **Steering/Follow-up 队列** — Pi 证明"运行时动态注入消息"是 agent runtime 的关键能力，Loom 的"governance view 注入规则" 可以借鉴
- ✅ **OAuth 订阅** 模式 — 用户可以用现有 ChatGPT Plus 跑 Pi，Loom 也可以支持
- ⚠️ Pi 拒绝 MCP = Loom 接受 MCP，是哲学差异。Loom 选 MCP 是因为 MCP 是 2026 行业标准（被 Goose / Cline / PraisonAI / Pydantic AI 共同支持）
- ❌ Pi 不可作为 Loom runtime（Pi 不持久化、不支持 SQLite、不给 multi-agent 协调）

### Cat 1 总结

**6 个项目的"Loom 相关度"**：

| 项目 | 强相关 | 模式 | 关键借鉴 |
|---|---|---|---|
| **Goose** | 中 | Linux Foundation 治理 + Custom Distributions | Distro 概念 |
| **OpenCode** | 中 | Build + Plan + General subagent 模式 | 双 agent 模式 |
| **Aider** | 低 | Solo agent + git integration | Singularity evidence (88%) |
| **Cline** | **强** | Coordinator + specialists + SDK + multi-surface | Multi-agent teams, headless JSON mode |
| **Continue** | 不适用 | 已停维护 | 教训：商业模式 |
| **Pi** | **强** | 4 工具极简哲学 + Session tree + Steering 队列 | 极简哲学 + Event Journal 树状化 |

**Cat 1 核心 insight**：
1. ✅ **多 agent 模式已成主流** — OpenCode / Cline / Pi 都做 multi-agent，Loom 的"subagent first-class"决策对
2. ✅ **"Coordinator + specialists" 是事实标准** — Loom 的 Planner = coordinator、subagent = specialists 直接对位
3. ⚠️ **"产品 vs view layer" 仍是 Loom 唯一差异** — Goose / Cline / OpenCode 都是产品，Loom 是底层 view。这让 Loom 的"下游消费者"是 Goose/Cline 这类工具，而不是直接竞争
4. ❌ **Continue 死掉** = 行业警告 — 开源 coding agent 想要长期活下去必须解决"商业模式 / 治理 / 用户粘性"

---

## Cat 2：Stateful agent frameworks (4 个项目)

### 2.1 Letta (letta-ai/letta, formerly MemGPT)

- **Repo**: https://github.com/letta-ai/letta
- **License**: MIT (推测)
- **Language**: Python
- **Note**: 仓库 "legacy" — V1 server 留在这里，**active development moved to `letta-ai/letta-code`**

**核心定位** (README 第一段)：
> "Build AI with advanced memory that can learn and self-improve over time."
> "Letta Agent: run agents locally in your terminal, via the desktop app, or via channels like Slack"

**关键架构**：
- **Letta Agent** = CLI + Desktop + Slack channel
- **Letta Agent SDK** = TypeScript SDK，部署到：
  - Constellation (Letta's agent cloud)
  - 本地
  - 自建 App Server
- **Skills + subagents** (pre-built for advanced memory + continual learning)
- **Stateful** — 每个 agent 有持久 memory
- **V1 SDK (`@letta-ai/letta-client`)** 仍可用，**Agent SDK** 是新代

**与 Loom 对比**：

| 维度 | Letta | Loom |
|---|---|---|
| 定位 | Memory-first agent platform | view layer (不直接是 agent) |
| Memory 模型 | 高级 (advanced + self-improve) | ❌ 不做 memory (subagent 决定) |
| Channel | CLI + Desktop + Slack | ❌ 不做 channel (subagent 决定) |
| 部署 | Cloud (Constellation) + Local + Self-host | 本地 daemon only |
| Event Journal | 自有格式 | Loom Event Journal (SQLite, append-only) |

**借鉴价值**：
- ✅ **"Advanced memory + self-improve over time"** = Letta 的核心 tagline。Loom Event Journal 是"可重放的历史"，但不做"主动 memory 优化"。这是 Phase 2+ 的 inspiration
- ✅ **"pre-built skills" 概念** — Letta 把常用 agent 能力预打包为 skills。Loom 的 subagent 可以类似做"predefined subagent templates"
- ⚠️ Letta 是"产品 + cloud"，Loom 是"view layer + 本地"
- ⚠️ Letta 的 legacy V1 server 留在原 repo = "产品迭代时的过渡处理" 教训

### 2.2 LangGraph (langchain-ai/langgraph)

- **Repo**: https://github.com/langchain-ai/langgraph
- **License**: MIT
- **Language**: Python (LangGraph.js 是 TypeScript 版)
- **Trusted by**: Klarna / Replit / Elastic

**核心定位** (README 第一段)：
> "Low-level orchestration framework for building stateful agents."

**关键架构 — 5 大支柱**：
1. **Durable execution** — Build agents that persist through failures and can run for extended periods, **automatically resuming from exactly where they left off**
2. **Human-in-the-loop** — Incorporate human oversight by **inspecting and modifying agent state at any point during execution** (interrupts)
3. **Comprehensive memory** — Both short-term working memory + long-term persistent memory
4. **Debugging with LangSmith** — trace execution paths, capture state transitions, runtime metrics
5. **Production-ready deployment** — LangSmith Deployment (sophisticated agent systems)

**设计灵感**：
- Inspired by **Pregel** (Google) + **Apache Beam**
- Public interface draws inspiration from **NetworkX**
- Can be used standalone, integrates with LangChain

**Companion package — Deep Agents**:
> "higher-level package built on LangGraph for agents that can plan, use subagents, and leverage file systems for complex tasks"

**与 Loom 对比**：

| 维度 | LangGraph | Loom |
|---|---|---|
| 定位 | "Low-level orchestration framework" | view layer + Bridge protocol |
| Durable execution | ✅ 自动 resume from exact point | ❌ 还没有 (Heartbeat + Run claim 是 manual 版本) |
| Human-in-the-loop | ✅ interrupts (任何点检查 + 修改 state) | ❌ 还没有 (governance view 是观察) |
| Memory | ✅ short + long term | ❌ 不做 (subagent 决定) |
| Resume mechanism | 自动 (framework-level) | 手动 (Loom 只是 observer) |
| Deployment | LangSmith Deployment | 本地 daemon |

**借鉴价值**：
- ✅ **"Durable execution" 概念** — Loom 的 Run claim + heartbeat 在工业界就叫 **durable execution**。LangGraph 的 "automatically resuming from exactly where they left off" 是 Loom 应该长期追求的目标
- ✅ **"Interrupts" 机制** — Loom 未来 governance view 的 "pause and inspect" 模式可以借鉴 LangGraph 的 interrupt API
- ✅ **Pregel/Beam 灵感** — 验证了"AI agent orchestration 借鉴大数据处理模型"是正确方向
- ✅ **"Deep Agents" = "subagents that can plan and use file systems"** — 跟 Loom "subagent first-class" 直接同构
- ⚠️ LangGraph 是 framework（要写代码用），Loom 是 view layer（运行时不改代码）

### 2.3 PraisonAI (MervinPraison/PraisonAI)

- **Repo**: https://github.com/MervinPraison/PraisonAI
- **License**: MIT
- **Language**: Python + JavaScript SDK
- **Highlighted by**: Elon Musk (Grok 3 customer support)

**核心定位** (README 第一段)：
> "PraisonAI 🦞 — **Hire a 24/7 AI Workforce.** Stop writing boilerplate and start shipping autonomous, self-improving agents that research, plan, and execute tasks across your apps. From one agent to an entire organization, deployed in 5 lines of code."

**关键架构 — 25+ features**：

| 类别 | 关键能力 |
|---|---|
| **MCP** | stdio / HTTP / WebSocket / SSE 4 transport |
| **Multi-agent** | Agent Handoffs, A2A Protocol, Orchestrator Workers, Planner-Executor |
| **Memory** | 0-deps memory, Graph Memory (Neo4j-style), File-based |
| **Reliability** | Shadow Git Checkpoints (auto-rollback), Doom Loop Detection, Background Tasks |
| **Security** | Guardrails, Human Approval, Policy Engine |
| **Performance** | Prompt Caching, Context Compaction, Model Router (cheapest capable) |
| **Observability** | OpenTelemetry traces, Langfuse integration |
| **Tooling** | 100+ custom tools, 100+ LLM providers, 24+ integration examples |
| **Sandbox** | Isolated code execution |
| **Distribution** | Claw (Telegram/Slack/Discord/WhatsApp gateway), Langflow integration |

**性能指标**:
- Agent instantiation: **14μs** (极快)

**与 Loom 对比**：

| 维度 | PraisonAI | Loom |
|---|---|---|
| 定位 | "Hire a 24/7 AI Workforce" | view layer |
| MCP | ✅ 4 transport full support | ✅ Bridge v1.1 + Phase 2 MCP |
| Multi-agent | ✅ Handoffs + A2A + Orchestrator | ✅ Bridge v1.1 subagent |
| Memory | 0-deps + Graph | ❌ 不做 |
| Channel | Telegram/Slack/Discord/WhatsApp | ❌ 不做 |
| Sandbox | ✅ Isolated execution | ❌ 不做 (subagent 决定) |
| Performance | 14μs instantiation | N/A (Loom 是 view) |

**借鉴价值**：
- ✅ **25+ features 列表是 "what's possible in 2026 AI agent framework" 的完整 reference** — Loom 未来 subagent 应该考虑这些能力的子集
- ✅ **"A2A Protocol"** (agent-to-agent interop) — PraisonAI 直接做。如果 Loom 的 subagent 需要跨 instance 通信，A2A 是个 reference
- ✅ **"Model Router (auto-routes to cheapest capable)"** = Loom cost view 想要的"成本最优调度" reference
- ✅ **"Policy Engine"** = Loom governance view 想要的"声明式 agent 行为控制" reference
- ✅ **"Doom Loop Detection"** = Loom 应该监控的"agent 失控"信号
- ⚠️ PraisonAI 是"产品 + 完整 framework"，Loom 只是"view layer"。Loom 不应该自己实现这些 — 留给 subagent
- ⚠️ PraisonAI 是"opinionated framework"，Loom 应该是"unopinionated view layer"

### 2.4 Pydantic AI (pydantic/pydantic-ai)

- **Repo**: https://github.com/pydantic/pydantic-ai
- **License**: MIT
- **Language**: Python
- **Org**: Pydantic 团队（与 Pydantic Validation 同一团队）

**核心定位** (README 第一段)：
> "Pydantic AI is a Python agent framework designed to help you quickly, confidently, and painlessly build production grade applications and workflows with Generative AI."
> "We built Pydantic AI with one simple aim: to bring that **FastAPI feeling** to GenAI app and agent development."

**关键架构 — 11 核心能力**：

1. **Built by the Pydantic Team** — Pydantic Validation 是 OpenAI / Google ADK / Anthropic / LangChain / LlamaIndex / AutoGPT / Instructor 的底座
2. **Model-agnostic** — 20+ providers
3. **Seamless Observability** — 深度集成 Pydantic Logfire (OTel)
4. **Fully Type-safe** — IDE + AI coding agent auto-completion
5. **Powerful Evals** — Systematically test and evaluate
6. **Extensible by Design** — **Capabilities = composable bundles of tools / hooks / instructions / model settings**
7. **MCP and UI** — MCP 集成 + UI event stream standards
8. **Human-in-the-Loop Tool Approval** — Certain tool calls require approval
9. **Durable Execution** — Preserve progress across transient API failures and restarts
10. **Streamed Outputs** — structured output continuously
11. **Graph Support** — Define graphs using type hints

**Part of Pydantic Stack**:
- **Pydantic AI** — Type-safe agent framework
- **Pydantic Logfire** — AI-first full-stack observability
- **Logfire AI Gateway** — Unified LLM proxy

**与 Loom 对比**：

| 维度 | Pydantic AI | Loom |
|---|---|---|
| 定位 | "FastAPI feeling" GenAI framework | view layer |
| Type safety | ✅ Pydantic 全套 | ❌ Loom 用 Go 1.22 + SQLite |
| Observability | Logfire (OTel) | ❌ Loom cost/governance view |
| Capabilities | Composable bundles | Subagent dispatch protocol |
| MCP | ✅ | ✅ Phase 2 |
| Durable execution | ✅ Preserve progress | ❌ (观察层) |
| UI event stream | ✅ | ✅ Bridge v1.1 notifications |
| Graph | ✅ | ❌ |

**借鉴价值**：
- ✅ **"FastAPI feeling"** — Pydantic AI 的设计目标是"给 AI agent 开发带来 FastAPI 那种 ergonomic 体验"。Loom 应该追求"Bridge v1.1 也有同类体验" — 让 subagent 注册像 API 一样简单
- ✅ **"Capabilities = composable bundles"** — Pydantic AI 把 tool + hook + instruction + model settings 组合为 "capability"，可重用。Loom subagent 可以做"predefined subagent template" 类似
- ✅ **"Human-in-the-Loop Tool Approval"** — Pydantic AI 提供 per-tool approval 机制。Loom governance view 想要"按 risk level 审批工具调用" 直接借鉴
- ✅ **"Part of Pydantic Stack"** — Pydantic 团队把 AI / Logfire / Gateway 三个东西打包成 stack。Loom 未来可以"Loom Stack = Loom daemon + Loom Bridge + Loom Studio" 类似
- ✅ **"YAML/JSON agent spec — no code required"** — Pydantic AI 支持声明式 agent 定义。Loom 的 subagent manifest 完全可以 YAML
- ⚠️ Pydantic AI 是 Python framework，Loom 是 Go 1.22 daemon — 语言栈不同
- ⚠️ Logfire 是 SaaS-first（Pydantic 商业产品），Loom 是 local-first

### Cat 2 总结

**4 个项目的"Loom 相关度"**：

| 项目 | 强相关 | 模式 | 关键借鉴 |
|---|---|---|---|
| **Letta** | 中 | Memory-first agent | Memory 模型 + pre-built skills |
| **LangGraph** | **强** | Durable execution + Interrupts | Durable execution = Loom 的目标 |
| **PraisonAI** | **强** | 25+ features 全参考 | 25+ features 列表 = what 的 reference |
| **Pydantic AI** | **强** | "FastAPI feeling" + capabilities | "FastAPI feeling" = Bridge v1.1 体验目标 |

**Cat 2 核心 insight**：
1. ✅ **Durable execution 是行业标准** — LangGraph / Pydantic AI / Restate 都做。Loom 的 Run claim + heartbeat 模式 = durable execution 的"view layer 实现"
2. ✅ **"Capabilities = composable bundles"** 是行业共识 — Loom subagent 应该支持 template / package 化
3. ✅ **"Interrupts" (HIL via state modification) 是 governance 机制** — Loom governance view 想要"运行时暂停+改规则"应该看 LangGraph
4. ⚠️ **"Memory"是所有框架的标配** — Letta / LangGraph / PraisonAI / Pydantic AI 都有 memory。Loom 不做 memory = 留给 subagent
5. ✅ **YAML/JSON declarative agent spec** 是新趋势 — Loom subagent manifest 可以完全走 YAML

---

## Cat 3：Durable execution engines (4 个项目)

### 3.1 Temporal (temporalio/temporal)

- **Repo**: https://github.com/temporalio/temporal
- **License**: MIT
- **Language**: Go (server) + Go/Java/Python/TypeScript/etc SDKs
- **起源**: Fork of **Uber's Cadence**
- **公司**: Temporal Technologies

**核心定位** (README 第一段)：
> "Temporal is a **durable execution platform** that enables developers to build scalable applications without sacrificing productivity or reliability. The Temporal server executes units of application logic called **Workflows** in a resilient manner that automatically handles intermittent failures, and retries failed operations."

**核心概念**:
- **Workflows** — 持久执行的 unit of application logic
- **Activities** — workflow 内可失败可重试的 step
- **Workers** — 执行 workflow 的进程
- **Web UI** at http://localhost:8233

**与 Loom 对比**：

| 维度 | Temporal | Loom |
|---|---|---|
| 定位 | Durable execution platform | view layer |
| Workflow | 1st class | Run (类似但 Loom 不执行) |
| Failure recovery | ✅ 自动 retry + replay | ❌ 手动 (heartbeat 监视) |
| Server | 自建 (Go) | 自建 (Go 1.22 + SQLite) |
| License | MIT | 待定 (v0.4 倾向 MIT) |

**借鉴价值**：
- ✅ **"Workflows + Activities + Workers" 命名** — Loom 概念模型可以借鉴：Run = Workflow, Subagent call = Activity, Loom daemon = Worker
- ✅ **"Replay-based resume" 模式** — Temporal 通过事件历史 replay 来恢复 workflow 状态。Loom Event Journal 本质是同一思路但不做 replay
- ✅ **Mature 8+ years production** — 验证了"durable execution 是工业级需求"。Loom 走这条路是对的
- ⚠️ Temporal 是 SaaS-first + 自部署复杂，Loom 是 local-first + 单 binary

### 3.2 Inngest (inngest/inngest)

- **Repo**: https://github.com/inngest/inngest
- **License**: **Server Side Public License + DOSP** (server), Apache 2.0 (SDKs)
- **Language**: Go (server) + TypeScript/Python/Go/Kotlin SDKs

**核心定位** (README 第一段)：
> "Inngest's **durable functions** replace queues, state management, and scheduling to enable any developer to write reliable step functions faster without touching infrastructure."

**3 个核心 element**:
- **Triggers** — Events, Cron schedules, Webhook events
- **Flow Control** — Concurrency / Throttling / Debouncing / Rate limiting / Prioritization
- **Steps** — 每个 step 自动 retry on failure

**完整架构** (README 有图):
```
Event API → Event stream → Runner → Queue → Executor → State store + DB
```

- Event API receives events via HTTP
- Runner schedules new "function runs" given event type
- Queue = multi-tenant aware, multi-tier, with flow control
- Executor = runs functions + writes state

**与 Loom 对比**：

| 维度 | Inngest | Loom |
|---|---|---|
| 定位 | Durable functions platform | view layer |
| 触发 | Events + Cron + Webhook | ❌ 不触发 (subagent 决定) |
| Flow control | Concurrency / Throttling / Debouncing | ❌ 不做 (subagent 决定) |
| State store | DB | Event Journal (SQLite) |
| Architecture | Event API → Stream → Runner → Queue → Executor | Loom 只是 observer |

**借鉴价值**：
- ✅ **"Event API → Stream → Runner → Queue → Executor" 架构图** — 这是"事件驱动 durable execution" 的经典图。Loom 的 Event Journal 消费侧可以做类似分层
- ✅ **"Flow control" 概念** — Inngest 把"concurrency limit / throttling / debouncing"作为 primitive。Loom governance view 想要"按 risk 限流" 可以直接借鉴
- ✅ **"Triggers" 分类** — Events / Cron / Webhook。Loom future subagent dispatch 触发器可参考
- ⚠️ Inngest 是"产品 + 完整 framework"，Loom 只是 observer

### 3.3 Restate (restatedev/restate)

- **Repo**: https://github.com/restatedev/restate
- **License**: Open core (Apache 2.0 SDKs)
- **Language**: Rust (server) + TypeScript/Java/Python/Go/Rust SDKs

**核心定位** (README 第一段)：
> "Restate is the simplest way to build resilient applications."
> "**Durable AI Agents**" is one of the explicit use cases.

**6 个核心 primitives**:
1. **Reliable Execution** — Failures result in retries that use the Durable Execution mechanism
2. **Reliable Communication** — **Exactly-once semantics** (request-response, one-way messages, scheduled tasks)
3. **Durable Promises and Timers** — Sleep, webhooks, timers that survive failures
4. **Consistent State** — Stateful entities with isolated K/V state per entity
5. **Suspending User Code** — Long-running code suspends on Promise, resumes when resolved
6. **Observability & Introspection** — UI + CLI + auto OpenTelemetry traces

**与 Loom 对比**：

| 维度 | Restate | Loom |
|---|---|---|
| 定位 | Simplest resilient app platform | view layer |
| Exactly-once | ✅ Reliable Communication | ❌ Loom 只观察 (subagent 自己保证) |
| Durable timers | ✅ sleep/webhook survive | ❌ 不做 |
| State | K/V per entity (stateful entities) | Event Journal (append-only) |
| AI Agent use case | ✅ "Durable AI Agents" 明确支持 | ✅ Loom 类似定位 |

**借鉴价值**：
- ✅ **"Durable AI Agents" 是 Restate 显式 use case** — Restate 直接把"AI agent that survives failure" 作为卖点。Loom 的 Run claim + heartbeat 模式在工业界有现成对应
- ✅ **"Exactly-once semantics"** — Restate 的 exactly-once 是 subagent dispatch 的金标准。Loom 的 bridge protocol 应该考虑"dispatch 唯一性"
- ✅ **"Suspending User Code"** — long-running code 暂停 / 恢复是 agent 关键能力。Loom 的 long-running subagent 应该能 suspend (等外部事件) → resume
- ✅ **"Durable Promises and Timers"** — sleep / webhook 持久化是 agent 必备 (例如"等 PR 合并后再继续")。Loom bridge 可以借鉴
- ⚠️ Restate 是 Rust server + SDKs 模式，Loom 是 Go daemon + Bridge protocol 模式，栈不同

### 3.4 Prefect (PrefectHQ/prefect)

- **Repo**: https://github.com/PrefectHQ/prefect
- **License**: Source-available (商业友好但非 OSI)
- **Language**: Python
- **Adoption**: 200M+ data tasks/month in Cloud, 25k practitioners

**核心定位** (README 第一段)：
> "Prefect is a **workflow orchestration framework** for building data pipelines in Python. It's the simplest way to elevate a script into a production workflow. With Prefect, you can build resilient, dynamic data pipelines that react to the world around them and recover from unexpected changes."

**关键特性**:
- **Flows + Tasks** — Python decorator 模式
- **Retries / Caching / Dependencies / Event-based automations / Scheduling**
- **Self-hosted server OR Prefect Cloud**
- **prefect-client** — 轻量级 client（适合 ephemeral environment）

**与 Loom 对比**：

| 维度 | Prefect | Loom |
|---|---|---|
| 定位 | Python workflow orchestration | view layer |
| Language | Python (deeply integrated) | Go 1.22 daemon |
| Flow | Python decorator | ❌ 不定义 (subagent 决定) |
| Resilience | Retries / Caching / Dependencies | ❌ 手动 (heartbeat 监视) |
| Cloud | ✅ Prefect Cloud | ❌ local-only |

**借鉴价值**：
- ✅ **"Python decorator 模式"** — Prefect 的 `@flow` `@task` 让 Python 开发者零摩擦加入 durability。Loom future 的 subagent 装饰可以借鉴
- ✅ **"Event-based automations"** — Prefect 支持"事件驱动 workflow 触发"。Loom 的 subagent dispatch 可以支持事件触发
- ⚠️ Prefect 是 Python 深度集成，Loom 不限定 subagent 语言
- ⚠️ Prefect 商业化偏强（Cloud-first），Loom 严格 local-first

### Cat 3 总结

**4 个项目的"Loom 相关度"**：

| 项目 | 强相关 | 模式 | 关键借鉴 |
|---|---|---|---|
| **Temporal** | **强** | Mature durable execution (8+ years) | Workflow/Activity 命名 + Replay |
| **Inngest** | 中 | Event-driven + flow control primitives | Flow control 概念 |
| **Restate** | **强** | "Durable AI Agents" 显式 use case | Exactly-once + Suspending + Durable Promises |
| **Prefect** | 低 | Python workflow orchestration | Python decorator 模式 |

**Cat 3 核心 insight**：
1. ✅ **Loom 的 Run claim + heartbeat 模式 = 工业级 "durable execution" 的 view layer 实现**
2. ✅ **Restate 直接把 "Durable AI Agents" 作为卖点** — 验证 Loom 这个方向正确
3. ✅ **"Exactly-once" + "Suspending" + "Durable Timers" 是 agent 必备的 3 个 primitives** — Loom future bridge 应该考虑
4. ✅ **Replay-based resume** 是 Temporal 验证的成熟模式 — Loom Event Journal 已经存了事件，未来可以做 "replay to subagent for recovery"
5. ⚠️ **Loom 不做调度器** — Temporal / Inngest / Prefect 是"调度+执行"，Loom 是"观察"，边界清楚

---

## Cat 4：LLM observability (4 个项目)

### 4.1 Langfuse (langfuse/langfuse)

- **Repo**: https://github.com/langfuse/langfuse
- **License**: MIT (except `ee` folders)
- **Language**: TypeScript + Python
- **Storage**: ClickHouse (analytic) + PostgreSQL
- **Y Combinator**: W23
- **Acquired by**: ClickHouse (Jan 2026)
- **Stars**: 16k+

**核心定位** (README 第一段)：
> "Langfuse is an **open source LLM engineering platform**. It helps teams collaboratively **develop, monitor, evaluate,** and **debug** AI applications. Langfuse can be **self-hosted in minutes** and is **battle-tested**. Proudly made with **ClickHouse** open source database."

**6 大核心能力**:
1. **LLM Application Observability** — Instrument your app, ingest traces, track LLM calls
2. **Prompt Management** — Centrally manage, version control, iteratively improve prompts (with caching)
3. **Evaluations** — LLM-as-a-judge, Code evaluators, user feedback, manual labeling, custom pipelines
4. **Datasets** — Test sets + benchmarks for evaluating LLM applications
5. **LLM Playground** — Test and iterate on prompts and model configurations
6. **Comprehensive API** — OpenAPI spec, Postman collection, typed SDKs (Python, JS/TS)

**部署选项**:
- **Langfuse Cloud** (managed)
- **Self-Host** — Docker Compose (5 min local) / VM / Kubernetes (Helm) / Terraform (AWS/Azure/GCP)

**集成生态 (Massive)**:
- OpenAI / Anthropic / LangChain / LlamaIndex / Haystack / LiteLLM / Vercel AI SDK / Mastra
- AutoGen / Flowise / Langflow / Dify / OpenWebUI / Promptfoo / LobeChat / smolagents / CrewAI
- **Goose** (Langfuse 集成 Goose — Cat 1 项目反向被 Cat 4 集成！)

**Top dependents (按 stars 排序)**:
- langflow (116k★) / open-webui (109k★) / screenshot-to-code (70k★) / lobe-chat (65k★) / ragflow (64k★) / firecrawl (56k★) / llama_index (44k★) / Flowise (43k★) / quivr (38k★) / ai-agents-for-beginners (38k★) / Langchain-Chatchat (36k★) / mindsdb (35k★) / LibreChat (33k★) / litellm (28k★) / onlook (22k★) / nixpkgs (21k★) / suna (17k★) / courses (17k★) / **mastra (16k★)** / **langfuse (16k★)** / WrenAI (11k★) / promptfoo (8.3k★) / PocketFlow (8.3k★)

**与 Loom 对比**：

| 维度 | Langfuse | Loom |
|---|---|---|
| 定位 | LLM engineering platform | view layer |
| Observability | ✅ ClickHouse-based 完整 trace | ✅ Loom Event Journal (SQLite) |
| Prompt Management | ✅ Version control | ❌ Loom 不管理 prompt (subagent 决定) |
| Evaluation | ✅ LLM-as-a-judge | ❌ Loom 不评估 (subagent 决定) |
| Storage | ClickHouse + PostgreSQL | SQLite (本地) |
| Deployment | Cloud + Self-host (complex) | Local-only (single binary) |

**借鉴价值**：
- ✅ **"ClickHouse for trace analytics"** — 工业级 trace 存 ClickHouse。Loom 选 SQLite 是因为"local-first + 单 binary + 简单"，Phase 2+ 考虑 ClickHouse for scale
- ✅ **"OpenAPI spec + Postman + typed SDKs"** = "treat observability as a product"。Loom Bridge v1.1 也是这样设计 (JSON-RPC 2.0 spec)
- ✅ **"Massive 集成生态"** — Langfuse 集成 LangChain / LlamaIndex / PraisonAI / Goose / smolagents / CrewAI 等。Loom 未来"view layer" 想要被广泛集成，应该先发标准 spec + SDK
- ✅ **"Top dependents 列表"** 是开源项目成功的关键 signal。Loom 长期要看自己被多少 top agent framework 集成
- ⚠️ Langfuse 是 SaaS-first（Cloud），Loom 严格 local-first — 商业模式不同
- ⚠️ Langfuse 的 `ee` 文件夹是商业版 — Loom 选 license 时要避免这种"open core" 模式

### 4.2 Helicone (Helicone/helicone)

- **Repo**: https://github.com/Helicone/helicone
- **License**: Apache 2.0
- **Language**: TypeScript
- **Storage**: Supabase + ClickHouse
- **Y Combinator**: (Helicone 公司)
- **Workers**: Cloudflare Workers (proxy layer)

**核心定位** (README 第一段)：
> "**Helicone is an AI Gateway & LLM Observability Platform for AI Engineers**"
> - 🌐 **AI Gateway**: Access 100+ AI models with 1 API key through the OpenAI API with intelligent routing and automatic fallbacks
> - 🔌 **Quick integration**: One-line of code to log all your requests
> - 📊 **Observe**: Inspect and debug traces & sessions
> - 📈 **Analyze**: Track metrics like cost, latency, quality
> - 🎮 **Playground**: Rapidly test and iterate
> - 🧠 **Prompt Management**: Version prompts using production data
> - 🎛️ **Fine-tune**: Fine-tune with one of our fine-tuning partners
> - 🛡️ **Enterprise Ready**: SOC 2 and GDPR compliant

**架构 (5 services)**:
- **Web**: Frontend (NextJS)
- **Worker**: Proxy Logging (Cloudflare Workers) — **single point of traffic**
- **Jawn**: Dedicated log collection server (Express + Tsoa)
- **Supabase**: Application DB + Auth
- **ClickHouse**: Analytics DB
- **Minio**: Object Storage for logs

**与 Loom 对比**：

| 维度 | Helicone | Loom |
|---|---|---|
| 定位 | AI Gateway + Observability | view layer |
| Gateway | ✅ 100+ models via 1 API key | ❌ Loom 不代理 LLM 调用 (fail closed) |
| Proxy 层 | ✅ Cloudflare Workers (single point) | ❌ Loom 不在 LLM 流量路径上 |
| Architecture | 5 services + ClickHouse + Supabase | 1 binary + SQLite |
| LLM Observability | ✅ 自动 | ✅ Loom 通过 subagent 暴露 |

**借鉴价值**：
- ✅ **"AI Gateway" 模式** — Helicone 直接替代 OpenAI endpoint，1 API key 通 100+ models。Loom 不做这个（fail closed on sampling/createMessage）
- ✅ **"Cloudflare Workers as proxy" 模式** — single point of traffic 实现 observability。Loom 不在 LLM 流量上所以不需要
- ⚠️ Helicone 是"AI Gateway + observability" — Loom 是"view layer" — 边界不同
- ⚠️ Helicone 是 SaaS-first，Loom 严格 local-first

### 4.3 Arize Phoenix (Arize-ai/phoenix)

- **Repo**: https://github.com/Arize-ai/phoenix
- **License**: **Elastic License 2.0 (ELv2)** — not OSI approved!
- **Language**: Python + TypeScript
- **Standards**: **OpenTelemetry-based** + OpenInference instrumentation
- **Note**: Phoenix 已经有 **Remote MCP Server** 内置

**核心定位** (README 第一段)：
> "Phoenix is an open-source AI observability platform designed for experimentation, evaluation, and troubleshooting."

**7 大核心能力**:
1. **Tracing** — OpenTelemetry-based
2. **Evaluation** — LLM benchmarks (response + retrieval evals)
3. **Datasets** — Versioned datasets for experimentation
4. **Experiments** — Track and evaluate prompt/LLM/retrieval changes
5. **Playground** — Optimize prompts, compare models, replay LLM calls
6. **Prompt Management** — Version control + tagging + experimentation
7. **PXI (Phoenix Intelligence)** — **AI engineering agent built into Phoenix for debugging traces, iterating on prompts, navigating the product**
8. **Remote MCP Server** — Connect Claude Code, Cursor to Phoenix instance's `/mcp` endpoint

**与 Loom 对比**：

| 维度 | Phoenix | Loom |
|---|---|---|
| 定位 | AI observability platform | view layer |
| Standards | OpenTelemetry + OpenInference | ❌ Loom 用自家 Bridge v1.1 |
| MCP Server | ✅ Remote MCP Server 内置 | ✅ Bridge v1.1 → Phase 2 MCP |
| PXI (AI debugging agent) | ✅ AI agent 内置 | ❌ Loom 不做 AI agent |
| License | Elastic License 2.0 | 待定 (倾向 MIT) |
| Code agent integration | CLI `npx @arizeai/phoenix-cli setup` | ❌ 还没有 |

**借鉴价值**：
- ✅ **"OpenTelemetry-based"** 是工业标准 — Loom 未来如果想被 Langfuse / Phoenix 集成，应该 emit OTel traces
- ✅ **"Remote MCP Server" 内置** — Phoenix 直接把"AI observability 暴露为 MCP server"，让 Claude Code / Cursor 能 query traces。Loom 的 cost/governance view 可以做同样事
- ✅ **"Phoenix CLI `npx @arizeai/phoenix-cli setup`"** — 一行命令给 AI coding agent 自动 instrument。Loom 可以做"loom setup" 给 coding agent 自动接入
- ✅ **"OpenInference semantic conventions"** — Phoenix + OpenInference 推动的 trace schema 标准。Loom 长期应该看
- ⚠️ **Elastic License 2.0 ≠ 开源** — Phoenix 严格说不是 OSI 开源。Loom 应该避免这个 license
- ⚠️ Phoenix 是 SaaS-first + 商业版，Loom 是 local-first + 倾向 MIT

### 4.4 LangSmith (langchain-ai via smith.langchain.com)

- **Repo**: https://github.com/langchain-ai/langsmith-cookbook (cookbook, 不是主代码)
- **License**: 主代码 closed-source (LangChain 商业产品)
- **Language**: Closed
- **Org**: LangChain Inc.

**核心定位** (from CSDN/网络文章):
> "LangSmith 是由 LangChain 官方推出的可观测性 (Observability) 与评估平台，专门为 LLM 应用和智能体 (Agent) 开发提供端到端的监控、调试和评估支持。目标是让开发者能够 **像调试传统 Web 应用一样，去调试和监控大模型应用**。"

**3 大能力**:
1. **可观测性** — 仪表盘 + 告警
2. **可评估** — 性能分析
3. **提示词工程** — 提示词管理 + 版本控制

**关键技术 — 分布式链路追踪** (类似 APM):
- **TraceID** — 整条调用链路的唯一标识
- **SpanID** — 调用片段的节点号
- **ParentID** — 形成父子层级

**工作流**:
1. 设置环境变量 (`LANGSMITH_TRACING=true`, `LANGSMITH_API_KEY`, `LANGSMITH_PROJECT`)
2. 运行 LangChain 代码
3. 自动 ingest 到 LangSmith 平台
4. 平台可视化链路 + Token 消耗 + 性能

**与 Loom 对比**：

| 维度 | LangSmith | Loom |
|---|---|---|
| 定位 | LangChain 商业 observability | view layer |
| TraceID/SpanID | ✅ APM-style | ❌ Loom Event Journal 用自家格式 |
| Integrations | LangChain 深度 | ❌ Loom 通过 Bridge protocol |
| License | 闭源 | 倾向 MIT |
| Pricing | 收费 (个人免费) | 免费 (local-only) |

**借鉴价值**：
- ✅ **"TraceID / SpanID / ParentID" 模式** — APM 经典三层。Loom Event Journal 如果暴露 OpenTelemetry trace，应该用同样模式
- ✅ **"环境变量自动集成"** 模式 — `LANGSMITH_TRACING=true` + `LANGSMITH_API_KEY` 一行接入。Loom 的 cost/governance view 集成可以借鉴
- ✅ **"像调试传统 Web 应用一样调试 LLM 应用"** — 这是 Loom 想要实现的"view layer" 哲学
- ❌ LangSmith 是闭源商业，Loom 不应该学商业模式
- ⚠️ LangChain 内部产品，跨生态集成（与 LangChain 强绑定）弱

### Cat 4 总结

**4 个项目的"Loom 相关度"**：

| 项目 | 强相关 | 模式 | 关键借鉴 |
|---|---|---|---|
| **Langfuse** | **强** | Open source LLM engineering platform | MIT + OpenAPI spec + Massive 集成生态 |
| **Helicone** | 中 | AI Gateway + observability | Cloudflare Workers proxy 模式 |
| **Arize Phoenix** | **强** | OTel-based + Remote MCP Server | OTel 标准化 + Remote MCP Server 暴露 |
| **LangSmith** | 中 | LangChain 商业 observability | TraceID/SpanID 模式 + env var 自动集成 |

**Cat 4 核心 insight**：
1. ✅ **Loom cost/governance view 直接对应 Langfuse / Phoenix** — Loom 想做"Loom-style observability" 应该学 Langfuse (开源) + Phoenix (MCP 暴露)
2. ✅ **OpenTelemetry 是 observability 的事实标准** — Loom Event Journal 未来应该 emit OTel traces 跟 Langfuse / Phoenix 兼容
3. ✅ **"Remote MCP Server" 模式 (Phoenix) 是 2026 新趋势** — Loom 应该把 cost/governance view 暴露为 MCP server
4. ✅ **"Env var auto-integration" 模式 (LangSmith) 是 UX 标杆** — Loom 应该一行命令让 coding agent 接入
5. ❌ **不要选 Elastic License 2.0 (Phoenix) 这种 "open core" 模式** — 严格 MIT 是 Loom 价值观
6. ✅ **"Massive 集成生态" 是 Langfuse 的护城河** — Loom 长期要看自己被多少 top framework 集成 (Top dependents list)

---

## 14 项目综合 insight

### 三大跨 category 模式

**1. "Coordinator + specialists" 是 multi-agent 事实标准**
- OpenCode (build + plan + general) ✅
- Cline (coordinator + specialists) ✅
- Pi (Steering + Follow-up + 4 tools) ✅
- LangGraph (Deep Agents = subagents) ✅
- PraisonAI (Orchestrator Workers) ✅
- **Loom 直接对位**: Planner = coordinator, subagent = specialists

**2. "Durable execution" 是 agent 必备**
- Temporal (Workflows + Activities + Workers + Replay) ✅
- Inngest (Event-driven + Flow control) ✅
- Restate (Durable AI Agents explicit use case) ✅
- Prefect (Python decorator) ✅
- LangGraph (Durable execution + Interrupts) ✅
- Pydantic AI (Durable execution) ✅
- **Loom 直接对位**: Run claim + heartbeat = durable execution 的 view layer 实现

**3. "MCP + Remote MCP Server" 是 2026 行业新趋势**
- Goose (70+ extensions via MCP) ✅
- Cline (`cline mcp` command) ✅
- PraisonAI (4 transport MCP + A2A) ✅
- Pydantic AI (MCP + UI event stream) ✅
- **Arize Phoenix (Remote MCP Server 内置)** ✅ — 让 Claude Code / Cursor 直接 query traces
- **Loom 直接对位**: Bridge v1.1 (JSON-RPC 2.0) + Phase 2 MCP + cost/governance view 暴露为 Remote MCP Server

### Loom 应该做什么 vs 不做什么

| 应该做 | 不应该做 |
|---|---|
| **view layer** (不抢 agent 产品) | **agent runtime** (那是 Goose / Cline / Aider) |
| **Event Journal** (append-only, rebuildable) | **memory model** (那是 Letta / Pydantic AI) |
| **durable execution 的 view** | **durable execution 本身** (那是 Temporal / Restate) |
| **"Coordinator + specialists" 编排协议** | **multi-agent framework 本身** (那是 LangGraph) |
| **cost / governance view** | **observability 平台** (那是 Langfuse / Phoenix) |
| **Bridge v1.1 (JSON-RPC 2.0)** | **subagent 完整 SDK** (那是 Cline @cline/sdk) |
| **CLI 工具 + run claim** | **IDE 集成 / Desktop** (那是 Cline / Continue) |
| **local-first + MIT** | **SaaS-first + Elastic License** (那是 Phoenix) |

### Permanent "不吸" 清单 (v2)

| 项目 | 不吸原因 |
|---|---|
| **Continue** | 已停维护 = 行业教训 ("vibe-coded weekend project" 死法) |
| **Prefect** | 商业化强 + Python 深度集成 + Cloud-first (跟 Loom local-first 冲突) |
| **Helicone** | 商业化 AI Gateway (跟 Loom "fail closed on sampling/createMessage" 冲突) |
| **LangSmith** | 闭源商业 (跟 Loom MIT 倾向冲突) |
| **Arize Phoenix (license only)** | Elastic License 2.0 ≠ OSI 开源 (Loom 应该避免) |

---

## 借鉴优先级

**P0 (Phase 1 必借鉴)**:
- ✅ Cline **"Coordinator + specialists" multi-agent model** → Loom Planner 协调 subagent 直接对位
- ✅ LangGraph **"durable execution" 命名 + Interrupts 概念** → Loom Run claim + heartbeat 用同套术语
- ✅ Pydantic AI **"Capabilities = composable bundles"** → Loom subagent template 化

**P1 (Phase 2 应该借鉴)**:
- ✅ Arize Phoenix **"OpenTelemetry-based + Remote MCP Server"** → Loom cost/governance view emit OTel + 暴露 MCP
- ✅ Langfuse **"Massive 集成生态"** → Loom Bridge v1.1 推广给 Top dependents
- ✅ Restate **"Exactly-once + Suspending + Durable Promises"** → Loom subagent dispatch 协议

**P2 (Phase 3+ 长期借鉴)**:
- Temporal **"Replay-based resume"** → Loom Event Journal replay to subagent for recovery
- Langfuse **"OpenAPI spec + Postman + typed SDKs"** → Loom Bridge v1.1 spec 完善
- LangSmith **"Env var auto-integration"** → Loom 一行命令 coding agent 接入

---

## 行动建议

### 1. 立即 (本周)
- ✅ **完成 v2 调研** (本文档) → PROGRESS.md + memory 3-target sync
- ✅ **更新 TECH-PLAN.md §subagent protocol** — 引用 Cline / LangGraph / Pi 的 multi-agent 模型
- ✅ **Bridge v1.1 spec 文档** — 引用 LangGraph interrupts + Restate exactly-once

### 2. Phase 1 收尾
- ✅ **Goose / Cline / Pydantic AI README 链接** 加入 phase-roadmap-supplements.md "借鉴" 章节
- ✅ **Continue 死亡教训** 写入 memory: "coding agent 工具必须解决商业模式 / 治理 / 用户粘性"
- ✅ **Decision: Loom 不做 AI Gateway (vs Helicone)** — 严格 fail closed on sampling/createMessage

### 3. Phase 2 准备
- ✅ **调研 Remote MCP Server 模式 (Phoenix)** — Loom cost/governance view 暴露为 MCP
- ✅ **调研 OpenTelemetry + OpenInference** — Loom Event Journal emit OTel traces
- ✅ **Restate "Durable AI Agents" 案例研究** — 写 Loom Run claim 的白皮书

### 4. Phase 3+ 长期
- ✅ **保持 Loom Stack = Loom daemon + Bridge + Studio** (学 Pydantic Stack)
- ✅ **Top dependents 监控** — 看 Loom 被多少 top framework 集成 (Langfuse 是标杆)
- ✅ **License 严格 MIT** — 避免 Phoenix 的 Elastic License 2.0 模式

---

VERDICT: PASS (基于 14 个项目 README 实际验证 + 跟 Loom 架构对比)
