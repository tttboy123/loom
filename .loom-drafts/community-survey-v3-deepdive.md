# Loom 社区调研 v3 — 前沿最新鲜 (2026-Q3 frontier deep-dive)

> **调研目的**:  在 v2 (14 项目 / 4 category) 基础上, 抓住 2026 年最前沿的项目演化、 协议升级、 共识形成, 为 Loom Bridge v1.1 / Phase 2 cost & governance view / Phase 3+ 演进做技术选型决策。
>
> **调研时间**: 2026-07-24 (Asia/Singapore)
>
> **方法**: 8 个项目 README (raw.githubusercontent.com) + 8 个 web_search 2026 月度新鲜 + 之前的 v2 / v2.5 base
>
> **vs v2 关系**:  v2 是「广度」(14 项目 / 4 category 总览), v3 是「深度 + 时效」(每个 category 选最活跃 / 最值得借鉴的项目, 挖到 2026-Q3 最新动态)
>
> **Loom 定位**:  view layer / 严格 MIT / 不抢 agent 产品 (详细 boundary 见 `memory/MEMORY.md` 2026-07-24 "Loom 必/不吸 边界")

---

## TL;DR — 2026-Q3 三大行业共识

User 要求"最前沿最新鲜", 这次调研的 8 + 8 fetch 透露出三个 2026 行业新共识, 每一个都直接关系到 Loom 的设计决策:

| # | 共识 | 证据 | 对 Loom 的影响 |
|---|---|---|---|
| **1** | **"Coordinator + specialists" 已成为 multi-agent 事实标准** (从平等 Agent 转向主从) | 5 个独立项目 (Cline / OpenCode / Pi / LangGraph Deep Agents / PraisonAI) 都做这个模式, 2026 Anthropic + OpenAI 双向背书 | Loom Planner = coordinator, subagent = specialists, Bridge v1.1 必须明确"coordinator 只能 dispatch"  |
| **2** | **"Durable execution" 是 agent 框架的必备项** (不再是 optional) | 2026 Replay/Temporal 大会把"AI workflow" 放到主舞台, Restate 把 "Durable AI Agents" 列为第一 use case, DBOS 推出 Conductor + MCP server, Microsoft pg_durable 内嵌 SQL 关键字 | Loom Run claim + heartbeat = durable execution 的 view layer 实现 (不要自己造 durable engine) |
| **3** | **"MCP + Remote MCP Server" 已成为 agent 基础设施 (不只是协议)** | MCP 2026-07-28 RC 4 大生产化信号 (无状态/能力发现/任务持续/依据可还原), Arize Phoenix 内置 Remote MCP, Langfuse 推 Platform MCP Server, 企查查 9 server / 197 tool | Loom Phase 2 cost/governance view 应该暴露为 **Remote MCP Server** (学 Phoenix / Langfuse) |

---

## Cat 1 — Coding Agent (前沿深挖)

**v2 调研覆盖 6 项目**: Goose / OpenCode / Aider / Cline / Continue / Pi
**v3 重点深挖**: Cline (6,614 commits + Apache 2.0 © 2026) / Pi (大 rebrand) / OpenCode (active dev)

### 1.1 Cline 2026-Q3 最新动态 — Coordinator + specialists 教科书实现

**6,614 commits**, Apache 2.0 © **2026 Cline Bot Inc.**, 4 product surfaces + 1 SDK。

#### 4 product surfaces (全部 2026 active)
| Surface | 用法 | 关键技术 |
|---|---|---|
| **CLI** | `npm i -g cline`, 交互 + headless `--json` (CI/CD) | 同 SDK 引擎, shell + 文件 + MCP |
| **Kanban** | `npm i -g kanban`, web 任务板, 多 agent 并行, **每个 card = 独立 worktree + auto-commit + 依赖链** | 独立 worktree 隔离 |
| **VS Code Extension** | 编辑器内, human-in-the-loop approval | checkpoints 可回退 |
| **JetBrains Plugin** | IntelliJ / PyCharm / WebStorm / GoLand 同体验 | 共享 agent core |

#### @cline/sdk (新发布)
- `Agent` + `createTool` 编程 API
- Custom tools, multi-agent teams, connectors, scheduled automations 一站式
- 100% TS 生态, npm 包

#### Multi-Agent Teams (Loom 直接借鉴)
```bash
cline --team-name auth-sprint "Plan and implement user authentication with tests"
```
- **Coordinator agent** 拆 work + 委派
- **Specialist agents** 各自有独立 tools + context
- **Team state persists** 跨 sessions

#### Scheduled Agents
```bash
cline schedule create "PR summary" \
  --cron "0 9 * * MON-FRI" \
  --prompt "List all open PRs and their review status" \
  --workspace /path/to/repo
```

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 必借鉴)**:  "Coordinator + specialists" 命名 + 状态在 specialists 之间不共享 (主 context 干净)
- ✅ **P1 (Phase 2)**:  Scheduled Agents 用 cron (Loom 调度用户工作流可以借鉴)
- ⚠️ **不做**:  Kanban 4 product surface 重型 (Loom 是 daemon, 不是 IDE / 不是 web app)

### 1.2 Pi 2026-Q3 — **重大 rebrand!** + supply-chain hardening

**关键事件**: 2026 年从 `badlogic/pi-mono` 迁到 `earendil-works/pi-mono` (新 org)

#### 4 packages (全部 @earendil-works/ 命名空间)
| Package | 角色 |
|---|---|
| `@earendil-works/pi-ai` | **unified multi-provider LLM API** (OpenAI / Anthropic / Google ...) |
| `@earendil-works/pi-agent-core` | agent runtime + tool calling + state management |
| `@earendil-works/pi-coding-agent` | interactive coding agent CLI (类似 Claude Code) |
| `@earendil-works/pi-tui` | terminal UI library with differential rendering |

#### Supply-chain hardening (2026 重要趋势)
- `.npmrc` 设 `save-exact=true` + `min-release-age=2` (避免 same-day dependency releases)
- `package-lock.json` 是 ground truth
- Pre-commit 阻止 lockfile 改动 (除非 `PI_ALLOW_LOCKFILE_CHANGE=1`)
- `npm run release:local` 在 tag 前做 smoke test
- **新 dep 必须提 issue 讨论** (防止 supply chain 攻击)

#### Containerization 3 patterns (Pi 拒绝 built-in permission, 主张外置)
| Pattern | 说明 | 适用 |
|---|---|---|
| **Gondolin extension** | keep `pi` 和 provider auth on host, 把 built-in tools 和 `!` commands 路由到 Linux micro-VM | 生产 |
| **Plain Docker** | 整个 `pi` 进程跑在容器 | 简单隔离 |
| **OpenShell** | 整个 `pi` 跑在 policy-controlled sandbox | 最严 |

#### pi-share-hf (新!) — 公开 session 训练数据
- 鼓励 OSS 用户 publish sessions 到 HuggingFace
- `pi-mono` work sessions: [badlogicgames/pi-mono on HuggingFace](https://huggingface.co/datasets/badlogicgames/pi-mono)
- "Public OSS session data helps improve coding agents with real-world tasks"

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴哲学)**:  极简哲学 (4 工具 + ~300 token prompt + YOLO mode)
- ✅ **P1 (Phase 2)**:  公开 session 训练数据机制 (Loom 可以借鉴做 planner / executor trace 公开)
- ⚠️ **不做**:  Containerization 3 patterns (Loom 假设有外部 sandbox, 不在 daemon 内置)

### 1.3 OpenCode 2026-Q3 — 2 agents (not 3) + Desktop BETA

#### Agents (2026 简化到 2)
- **build** (default, full-access) — 开发用
- **plan** (read-only) — 探索用, 拒绝 file edits, bash 需 permission
- `general` subagent 通过 `@general` 在消息中调用 (复杂搜索 + multistep tasks)

#### 22 语言本地化 README
English / 简体中文 / 繁體中文 / 한국어 / Deutsch / Español / Français / Italiano / Dansk / 日本語 / Polski / Русский / Bosanski / العربية / Norsk / Português (Brasil) / ไทย / Türkçe / Українська / বাংলা / Ελληνικά / Tiếng Việt

#### Desktop App (BETA, 2026)
- macOS (Apple Silicon / Intel), Windows, Linux (.deb / .rpm / .AppImage)
- `brew install --cask opencode-desktop` (Homebrew cask)

#### Install script 优先级链
`$OPENCODE_INSTALL_DIR` → `$XDG_BIN_DIR` → `$HOME/bin` → `$HOME/.opencode/bin`

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  read-only **plan** agent 模式 (Loom 应该有 "分析 mode" 不会改用户文件)
- ⚠️ **不做**:  Desktop app (Loom 是 CLI daemon, 不做 GUI)

### 1.4 Cat 1 2026 跨项目共识

| 共识 | Cline | Pi | OpenCode |
|---|---|---|---|
| 拒绝内置复杂 permission | ✅ 外部 sandbox 推荐 | ✅ 3 containerization patterns 文档 | ✅ 走 OS permission |
| Coordinator + specialists | ✅ Multi-Agent Teams | ✅ session tree + steering queues | ✅ build/plan + general subagent |
| 多 provider support | ✅ 11+ providers | ✅ unified `pi-ai` 抽象 | ✅ multi-provider |
| 简化极简 | 4 product surfaces 但都是同 engine | 4 工具 / ~300 token prompt | 2 agents 简单 Tab 切换 |
| Session 持久化 | Team state persists | Session tree branching | `pi-share-hf` 公开 |

---

## Cat 2 — Stateful Agent (前沿深挖)

**v2 调研覆盖 4 项目**: Letta / LangGraph / PraisonAI / Pydantic AI
**v3 重点深挖**: Pydantic AI (Pydantic Stack 完整) / LangGraph 1.0 + Deep Agents / LangChain 1.0/1.1

### 2.1 Pydantic AI 2026-Q3 — **Harness v0.5.0 + Pydantic Stack 完整**

#### 11 个 "Why use Pydantic AI" (从 v1 时代的 7 个扩展)
1. **Built by the Pydantic Team** (Pydantic Validation 是 OpenAI SDK / Google ADK / LangChain / LlamaIndex 的验证层)
2. Model-agnostic (20+ providers: OpenAI, Anthropic, Gemini, DeepSeek, Grok, Cohere, Mistral, Perplexity, Azure AI Foundry, Bedrock, GCP, Ollama, LiteLLM, Groq, OpenRouter, Together, Fireworks, Cerebras, HF, GitHub, Heroku, Vercel, Nebius, OVHcloud, Alibaba Cloud, SambaNova, **Z.AI**)
3. Seamless Observability (Pydantic Logfire 深度集成)
4. Fully Type-safe
5. Powerful Evals
6. **Extensible by Design: Capabilities = composable bundles of tools/hooks/instructions/model settings**
7. MCP + UI event stream
8. Human-in-the-Loop Tool Approval
9. **Durable Execution** (与 Temporal / DBOS 集成)
10. Streamed Outputs
11. Graph Support

#### 6 built-in capabilities
| Capability | 说明 |
|---|---|
| `WebSearch` | provider-adaptive (native + local DuckDuckGo fallback) |
| `Thinking` | 扩展思考 (provider-adaptive) |
| `MCP` | 任意 MCP server |
| `ToolSearch` | progressive tool discovery for large tool sets |
| **`CodeMode`** ⭐ | 沙箱 Python (Monty), N tool calls 合成 1 个 `run_code` |
| (核心) | web search, tool search, thinking 是 fundamental, 其他都进 Harness |

#### **Pydantic AI Harness v0.5.0** (PyPI, 2026-07 最新)
**官方 capability 库** — "batteries for your Pydantic AI agent"
```
uv add "pydantic-ai-harness[code-mode]"  # CodeMode + Monty
uv add "pydantic-ai-harness[logfire]"     # ManagedPrompt
```

**22 个 capability area** (capability matrix 跟踪 status):
- **Tools & execution**: Code mode ✅, Tool search ✅, File system ✅, Shell ✅, Repo context injection 🚧, Verification loop 🚧
- **Context management**: Sliding window 🚧, Context compaction 🚧, Limit warnings 🚧, Tool output mgmt 🚧, System reminders 🚧
- **Memory & persistence**: Memory 🚧, Session persistence 🚧, Checkpointing 📝
- **Agent orchestration**: **Sub-agents** 🚧, Skills 🚧, Planning 🚧, Task tracking 📝, Teams 📝
- **Safety & guardrails**: Input/Output guardrails 🚧, Cost/token budgets 🚧, Tool access control 🚧, Async guardrails 🚧, Secret masking 🚧, Approval workflows 🚧, Tool budget 🚧
- **Reliability**: Stuck loop detection 🚧, Tool error recovery 🚧, Tool orphan repair 🚧
- **Reasoning**: Adaptive reasoning 🚧, Current time 🚧

**版本政策**: 0.x = API 仍 stabilizing, minor releases (0.1 → 0.2) may include breaking changes
**License**: MIT

#### **Pydantic Stack** (2026 全套)
| Component | 角色 |
|---|---|
| **Pydantic AI** | type-safe agent framework |
| **Pydantic Logfire** | AI-first, full-stack observability |
| **Logfire AI Gateway** ⭐ | unified LLM proxy (代理 100+ providers) |

#### subagents-pydantic-ai v0.5.0 (94 commits)
- **Sync / async / auto** 3 mode
- **Nested subagents** (subagent 可以 spawn 自己的 subagent)
- **Runtime agent creation** (通过 factory 动态创建)
- **Background tasks**
- **Token tracking** (自动统计 subagent token 用量)
- **Question mode** (subagent 可以 ask parent for clarification)
- **SubAgentSpec YAML/JSON** 声明式配置

```python
from pydantic_ai import Agent
from pydantic_ai.capabilities import MCP, WebSearch
from pydantic_ai_harness import CodeMode
from subagents_pydantic_ai import SubAgentCapability, SubAgentConfig

agent = Agent('anthropic:claude-opus-4-7', capabilities=[
    CodeMode(),  # wraps every tool into run_code (Monty sandbox)
    MCP('https://hn.caseyjhand.com/mcp', native=False),
    WebSearch(native=False),  # routes through DuckDuckGo
])
```

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  "Capabilities = composable bundles" 概念 (Loom Bridge v1.1 method dispatch 应该按 capability 暴露)
- ✅ **P0 (Phase 1 借鉴)**:  Agent.run_sync / run_stream / run 3 形态
- ✅ **P1 (Phase 2 借鉴)**:  "Code mode" 概念 — Loom Run claim 可以借鉴做 "1 step claim = N tool calls composed"
- ✅ **P2 (Phase 3+ 借鉴)**:  Harness plugin 生态 (Loom 可以出 "loom-harness" 包)
- ⚠️ **不做**:  Pydantic Stack 自建 (Logfire 跟 Langfuse 重复, Loom 不做 observability 平台)

### 2.2 LangChain / LangGraph / Deep Agents 2026-Q3 — **三层架构清晰化**

> 官方明确: "LangGraph 是图运行时, LangChain 的 create_agent 是构建在其上的最小化 Agent 外壳, Deep Agents 是构建在 create_agent 之上、更有主见 (opinionated) 的外壳"

#### LangChain 1.0 (2025-10) — Middleware 架构
**最关键变化**: 引入 **Middleware** 架构 (灵感来自 Web 服务器中间件洋葱圈模式)

```python
from langchain.agents import create_agent
from langchain.agents.middleware import PIIMiddleware

agent = create_agent(
    model="deepseek-chat",
    tools=[search_weather],
    middleware=[
        PIIMiddleware("api_key", detector=r"sk-[a-zA-Z0-9]{32}", strategy="block"),
    ],
)
```

**5 个 hook 时机**:
- `before_agent(state, rt)` / `after_agent(state, rt)` — 一次性
- `before_model` / `wrap_model_call` / `after_model` — 每次 LLM 调用
- `wrap_tool_call(req, handler)` — 每次工具调用 (HumanInTheLoopMiddleware 在此 pause)

#### LangChain 1.1.0 (2025-11-24) — Model Profiles
```python
# .profile 属性让代码可编程查询模型能力
if chat_model.profile.supports_tools:
    ...
```

#### LangGraph 1.0 — 5 pillars (状态保留)
- Durable execution (failure resume)
- **Human-in-the-loop** (interrupt 机制, LangGraph 杀手特性)
- Comprehensive Memory (short-term + long-term)
- Debugging with LangSmith
- Production-ready deployment

**计算模型**: 受 Pregel + Apache Beam 启发 (BSP bulk synchronous parallel)
**API 风格**: 借鉴 NetworkX

#### **Deep Agents** (2026 新) — Agent Harness 模式
**核心 4 能力** (官方从 Claude Code 架构研究得出):
1. 详尽的系统提示词
2. **Subagent 机制**
3. 对文件系统的访问权限
4. **任务规划工具** (`write_todos`)

`create_deep_agent` 返回 `CompiledStateGraph`, 享受 LangGraph 全部基础设施。

#### create_deep_agent 完整签名 (18 个参数)
```python
def create_deep_agent(
    model: str | BaseChatModel | None = None,           # 必填支持 tool calling
    tools: Sequence[BaseTool | Callable | dict] | None = None,
    *,
    system_prompt: str | SystemMessage | None = None,  # 前置拼接
    middleware: Sequence[AgentMiddleware] = (),         # 追加在内置栈末尾
    subagents: list[SubAgent | CompiledSubAgent] | None = None,  # ★
    skills: list[str] | None = None,                    # agentskills.io SKILL.md 路径
    memory: list[str] | None = None,                    # AGENTS.md 路径
    response_format: ResponseFormat | None = None,      # Pydantic 约束结构化输出
    context_schema: type[Any] | None = None,            # run-scoped 静态上下文 (如 tenant_id)
    checkpointer: Checkpointer | None = None,           # 跨 thread state
    store: BaseStore | None = None,                      # 跨会话 K/V
    backend: BackendProtocol | BackendFactory | None = None,  # 虚拟文件系统
    interrupt_on: dict[str, bool | InterruptOnConfig] | None = None,  # HITL
    debug: bool = False,
    name: str | None = None,
    cache: BaseCache | None = None,
) -> CompiledStateGraph
```

#### **三级上下文压缩** (Deep Agents 独家)
| 级别 | 触发 | 动作 |
|---|---|---|
| **1. Tool input offloading** | 旧 tool call 记录冗余 | 截断历史, 替换为指向磁盘文件的指针 |
| **2. Tool result offloading** | tool result > 20,000 token | 卸载到 backend, 历史中只保留 10 行预览 |
| **3. Summarization** | 上下文 > 模型窗口 85% | LLM 生成结构化摘要, 完整原始记录入 FS |

#### 10 中间件栈 (按执行顺序固定)
`#1 TodoList → #2 Memory → #3 Skills → #4 Filesystem → #5 SubAgent → #6 Summarization → #7 AnthropicPromptCaching → #8 PatchToolCalls → #9 用户自定义 → #10 HumanInTheLoop`

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  Middleware 洋葱圈设计 (Loom 可以在 journal write 路径加 middleware: 加密 / 签名 / 路由)
- ✅ **P0 (Phase 1 借鉴)**:  SubAgent 双类型 (SubAgent dict 声明式 + CompiledSubAgent 自定义图)
- ✅ **P1 (Phase 2 借鉴)**:  三级上下文压缩 (Loom daemon 内存可能爆, 类似 offloading 思路)
- ✅ **P1 (Phase 2 借鉴)**:  context_schema 模式 (run-scoped 静态 tenant_id 等)
- ⚠️ **不做**:  LangGraph 整套运行时 (Loom 是 view layer, 不重做 runtime)

### 2.3 2026 Multi-Agent 共识 (OpenAI + Anthropic 双向背书)

> 引用: "2025-2026年业界共识开始形成: Multi-Agent是手段不是目的, 能用单Agent解决的不要堆Agent"

| 模式 | 代表 | 2026 状态 |
|---|---|---|
| **Anthropic Subagents (主从)** | Claude Code Task tool | ⭐ **生产首选** (Anthropic 实战验证) |
| **OpenAI Swarm / Agents SDK (Handoffs)** | OpenAI Agents SDK 2025-05 | ⭐ **生产首选** (OpenAI 生态) |
| **Google A2A (跨厂商)** | A2A 协议 2025 | ⭐ **跨厂商标准** |
| **MS Agent Framework** | 替代 AutoGen | 企业 Azure 过渡中 |

**3 种通信模式**:
1. **Handoffs**: 整个对话上下文交接
2. **Tools-as-Agent**: 主管把 worker 当 tool 调
3. **Blackboard**: 共享 state (LangGraph State)

**5 个反模式 (不该用 Multi-Agent)**:
1. 单一专业领域任务
2. 任务步骤明确 (用 Workflow)
3. Plan-then-Execute 严格控制
4. 调试预算少
5. 实时低延迟

**Loom 验证**:  v2 调研 + 这次 v3 深挖, 全部 14 项目都遵循这个共识 → Loom Planner = coordinator, subagent = specialists, Bridge v1.1 必须明确 "coordinator 只能 dispatch, 不直接调工具"

---

## Cat 3 — Durable Execution (前沿深挖)

**v2 调研覆盖 4 项目**: Temporal / Inngest / Restate / Prefect
**v3 重点深挖**: Restate (Durable AI Agents) / Temporal Replay 2026 / DBOS Conductor + Microsoft pg_durable / Hatchet / Belay (Elixir)

### 3.1 Restate 2026-Q3 — **"Durable AI Agents" 显式 use case**

#### 6 core primitives
1. **Reliable Execution** — Durable Execution mechanism, 失败自动 retry + 恢复 partial progress
2. **Reliable Communication** — **exactly-once** (request-response / one-way / scheduled)
3. **Durable Promises and Timers** — Resilient to failures
4. **Consistent State** — **stateful entities** with isolated K/V state per entity
5. **Suspending User Code** — long-running code suspends on Promise, resumes on resolve
6. **Observability & Introspection** — UI/CLI/auto **OpenTelemetry** traces

#### 5 SDKs
TypeScript / Java & Kotlin / **Python** / **Go** / **Rust**

#### Server 特性
- **SemVer 友好**: 升级 x.y → x.(y+1) 无需 manual data migration
- Install: homebrew / npx / docker 一行
- Rust server, **stateful entities** = 状态机 + K/V

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  "Durable AI Agents" 定位 (Loom 的 Run claim 实质就是 view layer 的 durable execution)
- ✅ **P1 (Phase 2 借鉴)**:  "Exactly-once" 语义 (Loom Run claim 上报事件应该 exactly-once, 避免重复计费)
- ⚠️ **不做**:  5 SDKs 自己造 (Loom 是 view layer, 推荐用 Restate 作为底层)

### 3.2 Temporal 2026-Q3 — **Replay 2026 大会主推 AI**

#### Replay 2026 公告 (4 大新功能)
- **Serverless Workers** — 不需要 manage worker 进程
- **Standalone Activities** — activity 独立 deploy
- **Workflow Streams** — 流式 workflow 状态
- **新集成**: Google ADK + OpenAI Agents SDK

#### 生产用户 (2026)
| 用户 | 用途 |
|---|---|
| **NVIDIA** | GPU fleet across clouds |
| **Salesforce** | monolith 迁移到 Temporal |
| **Twilio** | 替换自建系统到 Temporal Cloud |
| **Descript** | AI uptime 提升 |
| **OpenAI Venkat** (VP App Infra, ex-Rockset) | "Durable execution is a core requirement for modern AI systems" |

#### 关键数字
- 21,802 stars, MIT
- 9 years in production
- "Built by the minds behind AWS SQS / SWF / Azure Durable Functions / Cadence (Uber)"

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  Replay-based resume 模式 (Loom Run claim 失败后, 重新 claim 不应该丢失进度)
- ✅ **P0 (Phase 1 借鉴)**:  Google ADK + OpenAI Agents SDK 集成 (Loom Bridge v1.1 应该兼容这两个框架的 wire format)
- ⚠️ **不做**:  Temporal 整套 (已经有 9 年, Loom 不重做)

### 3.3 DBOS 2026-Q3 — **Conductor + 新 MCP server**

#### DBOS 是什么
"**Lightweight durable workflows library** built on top of Postgres" — 不像 Temporal 是外部 server, DBOS 是库
- 安装一行, 只需 Postgres
- 注解 `@DBOS.workflow()` / `@DBOS.step()`

#### 2026-07 新功能
- **MCP Server** (新!) — 从 coding agents monitor + debug workflow
- **OpenMetrics** endpoint — 接 Datadog / Grafana
- **RBAC** (Role-based access control)
- **Bulk Workflow forking** — 从某个 step 重启 thousands of workflows
- **Google ADK plugin** (新)
- **Kafka integration** improvements
- **Durable streams** performance improvements
- **DBOS Transact for Java 1.0** (新)

#### 4 集成 frameworks
- Pydantic AI (官方 plugin)
- LlamaIndex
- OpenAI Agents SDK
- Google ADK

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  DBOS 走 Postgres 路线 (Loom 也走 SQLite/Postgres, 复用同样的 durability assumption)
- ✅ **P1 (Phase 2 借鉴)**:  DBOS 新 MCP Server (Loom 后续 cost view 也可以暴露 MCP)
- ⚠️ **不做**:  Conductor 整套 (DBOS Conductor 是 SaaS, Loom 自己做)

### 3.4 **Microsoft pg_durable** (NEW 2026) — **PostgreSQL extension, SQL 关键字!**

**Microsoft 2026-07 新发布**: PostgreSQL extension 把 durable execution 内嵌到 SQL

#### 核心 API
```sql
-- df.start() 启动一个 durable function
SELECT df.start(
  'SELECT id FROM documents WHERE processed = false LIMIT 100'
  |=> 'batch'                                        -- 把结果命名 batch
  ~> 'UPDATE documents SET processed = true WHERE id IN (SELECT id FROM $batch.*)'  -- 用 batch
);
```

#### 操作符
- `|=>` — bind result to name
- `~>` — chain next step (用之前的结果)
- `df.if()` / `df.join()` / `df.loop()` — 控制流
- `df.http()` — 调用外部 API

#### Architecture
- PostgreSQL extension (built with **pgrx**)
- 内部用 **duroxide** (durable task framework, MIT) + **duroxide-pg** (PostgreSQL state provider)
- Background worker 在 PG server 内 in-process 跑

#### 适用场景 (官方列表)
- Vector embedding pipelines
- Ingest pipelines
- Scheduled maintenance
- Fan-out aggregation
- External API workflows (enrichment / classification / webhook)

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴哲学)**:  "durable execution 不需要独立 server, 可以是库 / extension" (Loom daemon 也是这个思路, 不需要外部服务)
- ✅ **P2 (Phase 3+ 借鉴)**:  SQL 关键字路线 (Loom 后续可以出 "loom SQL" 让用户在 SQL 里触发 Run claim)
- ⚠️ **不做**:  pg_durable 自己写一遍 (已经有现成 implementation)

### 3.5 Hatchet 2026-Q3 — **Postgres-based, 416 jobs/s**

#### 关键数字
- 3,208 commits
- Python / TypeScript / Go / Ruby SDKs
- **~416 jobs/s** end-to-end (3 workers, unbatched acks)
- **11ms p50** insert→result (cross-process + pg_notify)
- Multi-tenant default, OpenTelemetry built-in

#### vs Temporal / DBOS 定位
"Hatchet's durable tasks feature is a drop-in replacement for Temporal or DBOS workflows. You also get: end-to-end observability, features built for running workflows at scale (rate limiting, complex routing, worker-level slot control), multi-tenancy, users and roles out of the box"

#### vs task queues (Celery / BullMQ)
"Traditional task queues trade off durability for throughput. Tasks persist on the broker while executing, but are not persisted afterwards. Hatchet is a durable task queue — persists history of all executions"

#### Loom 借鉴优先级
- ✅ **P1 (Phase 2 借鉴)**:  "Postgres-based durable execution + OTEL" 定位 (跟 Langfuse 集成天然)
- ⚠️ **不做**:  Hatchet 自建 (它的多租户 + worker slot 跟 Loom 定位不同)

### 3.6 **Belay (Elixir)** (NEW 2026-07) — **Journal-based, $USD 预算**

#### 关键数字
- 1.0.0-rc.5, Apache-2.0
- **8.6ms p50** insert→result (same node), **11ms p50** cross-process
- **99,004 jobs / 7h** soak test, 4,978 kill -9, 13 DB restarts → **0 violations**
- 187,975,659 distinct states validated by TLC model checker
- 134 (Memory) + 144 (PostgreSQL) test suites

#### 杀手特性: Budget + Token True-Up
```elixir
Belay.insert(
  MyApp.ResearchPipeline.new(
    %{url: url},
    budget: [usd: 1.00],            # Per-job hard limit
    unique: "research: #{url}"     # Exactly-once
  )
)
```
- **3 层 cost 控制**:
  1. Per-job fail-after-crossing limit
  2. Fleet-wide admission budget per window (resource bucket)
  3. True-up (claim debits estimate, job corrects with actuals)

#### 6 chaos bugs found by harnesses (3 by soak, 1 by endurance, 1 by adapter-equivalence, 1 by model extension)
"6 real bugs were found by these harnesses before any user could"

#### MCP server built-in
`mix belay.mcp` — AI assistants inspect the queue; writes disabled by default, require authorizer

#### Oban migration
```bash
mix belay.migrate_oban --url postgres://.../my_app --execute
```
"222/222 tests green and a live-producer run on the first attempt, with zero engine changes required"

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  "$USD budget per run" 概念 (Loom 应该有 cost cap per Run)
- ✅ **P1 (Phase 2 借鉴)**:  MCP server built-in (Loom Phase 2 cost view 暴露 MCP, 学 Belay 模式)
- ✅ **P2 (Phase 3+ 借鉴)**:  Chaos test harness (Loom 应该有类似 "7h kill -9 chaos test" 验证 durability)

### 3.7 Cat 3 2026 跨项目共识

| 共识 | Restate | Temporal | DBOS | Hatchet | Belay |
|---|---|---|---|---|---|
| **AI workflow 是主 use case** | ✅ 第一 use case | ✅ Replay 大会主推 | ✅ 4 集成 frameworks | ✅ 主页说 "AI agents" | ✅ journal-based |
| **Postgres 作为底层 storage** | ❌ (自己 server) | ❌ (Cassandra) | ✅ | ✅ | ✅ |
| **OTel / 集成 observability** | ✅ auto OTel | ✅ Workflow Streams | ✅ OpenMetrics | ✅ built-in | ⚠️ telemetry |
| **MCP server 暴露** | ❌ | ❌ | ✅ (新) | ❌ | ✅ |
| **Budget / 资源控制** | ⚠️ | ⚠️ | ⚠️ | ⚠️ | ✅ $USD per job |
| **Exactly-once** | ✅ | ✅ (idempotency) | ✅ | ✅ | ✅ |

**Loom 借鉴总结**:
1. **不要自己造 durable engine** — Restate / DBOS / Hatchet / Belay 都在做, 选一个接
2. **AI workflow 是 2026 主 use case** — 跟 Loom 定位完全契合
3. **Loom 应该是 view layer** — 在 Restate / DBOS 之上, 不是替代

---

## Cat 4 — LLM Observability (前沿深挖)

**v2 调研覆盖 4 项目**: Langfuse / Helicone / Arize Phoenix / LangSmith
**v3 重点深挖**: Arize Phoenix (PXI + Remote MCP) / OpenInference 升级 / OpenTelemetry GenAI / **MCP 2026-07-28 RC 协议升级** / Langfuse (ClickHouse acquired)

### 4.1 Arize Phoenix 2026-Q3 — **PXI AI 调试 agent + Remote MCP Server**

**License**: **Elastic License 2.0 (NOT OSI)** ⚠️
**Version**: 17.11.0 (helm) / 8,874 commits
**8 capabilities**: Tracing / Eval / Datasets / Experiments / Playground / Prompt Mgmt / **PXI (Phoenix Intelligence)** / **Remote MCP Server**

#### 4 new 2026 features (核心)

1. **PXI (Phoenix Intelligence) — AI engineering agent**
   - Built into Phoenix for debugging traces
   - Iterating on prompts
   - Navigating the product

2. **Remote MCP Server 内置** ⭐
   - Phoenix instance `/mcp` endpoint
   - Claude Code / Cursor / Codex connect directly
   - Query traces / datasets / experiments

3. **`.agents/skills/`** — multi-editor skill 标准化
   - `.claude/skills/` (Claude Code)
   - `.codex/skills/` (Codex)
   - `.cursor/skills/` (Cursor)
   - `phoenix-cli` / `phoenix-evals` / `phoenix-tracing` 3 官方 skills

4. **`@arizeai/phoenix-mcp`** — Standalone stdio MCP server
   - "maintenance mode — superseded by the remote MCP server built into Phoenix"

#### 25+ Python integrations + 7 TS + 2 Java + 2 Go
Python: OpenAI / Anthropic / Google GenAI / Google ADK / AWS Bedrock / LiteLLM / MistralAI / VertexAI / OpenAI Agents / Claude Agent SDK / Pydantic AI / smolagents / LangChain / LlamaIndex / DSPy / Haystack / CrewAI / Agno / Autogen AgentChat / Portkey / Agent Spec / Strands Agents / Pipecat / OpenLIT / OpenLLMetry (Traceloop) / MCP / Instructor / Groq

#### 3 surface 给 coding agents
- **CLI** (`@arizeai/phoenix-cli`) — fetch traces, datasets, experiments
- **Skills** (3 个 agentskills.io SKILL.md)
- **Remote MCP** — connect any MCP client

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  ".agents/skills/" 多 editor 支持 (Loom 应该有 `.loom-drafts/skills/claude-code.md` / `codex.md` / `cursor.md` 同步)
- ✅ **P0 (Phase 2 借鉴)**:  Remote MCP Server 暴露 traces (Loom Phase 2 cost/governance view 完整学这个)
- ✅ **P1 (Phase 2 借鉴)**:  PXI 概念 — Loom 可以有 "loom-insight" AI helper
- ⚠️ **不做**:  Phoenix 整套 (license 是 ELv2, Loom 要 MIT)
- ⚠️ **不做**:  Phoenix 25 integrations 自己接 (太重, Loom 只暴露自己的 OTel)

### 4.2 OpenInference + OpenTelemetry GenAI 2026-Q3 — **正式合并**

#### 关键事件: 2026-06 — OpenInference semantic conventions 正式进入 OpenTelemetry GenAI 工作组
- `Arize-ai/openinference` 1,948 commits
- 官方 spec 在 `spec/reasoning` PR #3112 (2026-05-23)
- **25+ Python instrumentations + 7 TS + 2 Java + 2 Go** 全部 synced

#### OpenTelemetry 主项目
- **CNCF graduated** (2026-Q3)
- Tracing + Metrics **stable** across all major languages
- Logs development
- **12+ languages** (Java / Kotlin / Python / Go / JS / .NET / Ruby / PHP / Rust / C++ / Swift / Erlang)
- **200+ collector components**
- **1020+ integrations**
- **105+ vendors**
- 2026-07-28~30 KubeCon + CloudNativeCon Japan Yokohama

#### traceloop/openllmetry v0.49+ — "Our semantic conventions are now part of OpenTelemetry!"
- 1,411 commits
- Apache 2.0
- 标准 OpenTelemetry instrumentations for LLM providers (OpenAI / Anthropic / Bedrock / VertexAI) + Vector DB (Chroma / Pinecone / Qdrant / Weaviate)
- 1 行代码: `Traceloop.init()`

#### alibaba/loongsuite-java
- `otel-util-genai` Java GenAI utility
- `GenAiTelemetryHandler.create(openTelemetry).inference("openai", "gpt-4o")`
- 8 commits (新)

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  OpenTelemetry 作为 trace 出口 (Loom 不做 trace 平台, 输出 OTel spans 给 Langfuse / Phoenix)
- ✅ **P1 (Phase 2 借鉴)**:  OpenInference semantic conventions (Loom trace span 属性标准化)
- ⚠️ **不做**:  OTel collector 整套 (用现成的 opentelemetry-collector)

### 4.3 **MCP 2026-07-28 RC** — **史上最大修订** (直接影响 Loom Phase 2)

#### 4 大生产化信号 (QCC 总结, 跟 Loom 100% 相关)

##### 信号 1: 协议从"会话绑定"走向"请求自包含"
- 旧版: 客户端先 handshake, 后续请求依赖同一 Session
- **新版: 取消协议层 Session, 每个请求携带完成处理所需的信息**
- **工程含义: 远程 MCP Server 不必再被某次会话绑定在某台机器上** (可以横向扩容 / 故障切换 / 复用 HTTP 网关和负载均衡)

##### 信号 2: 能力从"列出来"走向"管起来"
- 新版: `server/discover` 让客户端按需了解服务端能力
- `Mcp-Method` + `Mcp-Name` 让网关识别调用的方法和工具
- `ttlMs` + `cacheScope` 让稳定的工具清单和资源拥有明确缓存时间
- **工程含义: 工具越多, 越需要能力治理, 而不是继续平铺**

##### 信号 3: 任务从"一次调用"走向"持续完成"
- **Multi Round-Trip Requests** (在主体不唯一或参数不足时请求用户确认)
- **Tasks** (管理批量扫描 / 长报告 / 大文档解析)
- **MCP Apps** (提供候选主体选择 / 风险图谱 / 任务进度等交互界面)
- **工程含义: MCP 从单次工具调用, 扩展到支持补充输入、确认节点与长任务状态管理**

##### 信号 4: 结果从"模型说了什么"走向"依据能否还原"
- 完整 JSON Schema 2020-12
- **W3C Trace Context 传播**
- **OAuth 2.1** 授权加固
- **工程含义: 企业级 MCP 不只交付答案, 还要交付答案的结构和运行轨迹**

#### 5 大变化
1. **无状态核心** (stateless)
2. **能力发现 + 路由 + 缓存** (capability discovery)
3. **Extensions 一等公民** (正式弃用政策)
4. **Tasks** (长任务管理)
5. **MCP Apps** (交互界面)

#### Loom 借鉴优先级 (这是 Cat 4 最重要的发现)
- ✅ **P0 (Phase 1 必借鉴)**:  JSON-RPC 2.0 envelope (Loom Bridge v1.1 已经定了, 跟 MCP 2026-07-28 兼容)
- ✅ **P0 (Phase 1 借鉴)**:  **W3C Trace Context 传播** (Loom Bridge v1.1 的 trace 字段应该支持 W3C 标准)
- ✅ **P0 (Phase 1 借鉴)**:  **完整 JSON Schema 2020-12** (Loom Bridge v1.1 method schema 应该用完整 JSON Schema)
- ✅ **P0 (Phase 2 借鉴)**:  **Remote MCP Server 暴露** (学 Phoenix / 企查查 / Langfuse)
- ✅ **P0 (Phase 2 借鉴)**:  **OAuth 2.1** (Loom cost view 暴露 MCP 时需要企业级 auth)
- ✅ **P1 (Phase 2 借鉴)**:  **能力发现 + 缓存** (Loom 工具多了要治理)
- ⚠️ **不做**:  MCP Apps 交互界面 (Loom 是 view layer, UI 由前端决定)
- ⚠️ **不做**:  Tasks 长任务管理 (Loom 已经有 Run claim, 不重复)

### 4.4 企查查 MCP 2026-07 — **9 Server / 197 tool / 27 SKILL, 五层能力矩阵**

> 这是 2026-07 **国内企业级 MCP 落地最佳实践**, 跟 Loom Phase 2 cost/governance view 直接对标

#### 五层能力矩阵
| 层 | 角色 | 回答 |
|---|---|---|
| **Tool** | 提供实时数据 (工商 / 股权 / 风险 / 知产 / 经营 / 司法 / 法规 / 文档解析) | "数据从哪里来" |
| **Server** | 按领域分组 (企业 / 法律 / 文档) | "能力属于哪个专业范围" |
| **Resources** | 稳定知识 (术语表 / 数据字典 / 工具映射 / 报告模板) | "调用前需要理解什么" |
| **SKILL** | 业务流程 (企业核验 / UBO / 尽调 / 风险扫描 / 报告生成) | "怎样完成任务" |
| **全局约束** | 实体锚定 / 当前 vs 历史 / 数据时点 / 引用纪律 / 输出边界 | "哪些事情不能猜、不能混、不能越过" |

#### Loom 借鉴
- ✅ **P0 (Phase 2 借鉴)**:  **5 层能力矩阵** — Loom cost/governance view 暴露 MCP 时按 5 层组织
- ✅ **P1 (Phase 2 借鉴)**:  Resources 暴露稳定知识 (Loom 的 tool mapping / subagent dispatching table 放 Resources)
- ✅ **P1 (Phase 2 借鉴)**:  SKILL 组织业务流程 (Loom cost view 的常用 query 放 SKILL)
- ✅ **P0 (Phase 2 借鉴)**:  全局约束 (Loom 应该有 "loom:risk" 字段, 工具调用前必填)

### 4.5 Langfuse 2026-Q3 — **被 ClickHouse 收购 (2026-01) + 4 大新功能**

#### 收购
"since January 2026 we're part of ClickHouse, we're hiring engineering hybrid across the EU"
- License: MIT (除 `ee` folders)
- 16,054 stars, 5,000+ Discord, 22,000+ GitHub stars
- **50M+ SDK installs/month**
- **10B+ observations processed per month**
- 2,300+ customers, **99.9% uptime**
- Security: SOC 2 Type II, ISO 27001, GDPR, HIPAA-ready

#### 4 大新功能 (2026)
1. **Coding agents SKILL.md** — 全新发布, 让 coding agent 用自然语言管 prompts / traces / evals
2. **Langfuse CLI** — terminal 全 API access
3. **Platform MCP Server** — IDE agents (Claude Code / Cursor / Codex) structured access
4. **Agent Evals & Observability** — 完整 LLM engineering loop

#### 核心架构
- ClickHouse OLAP database
- Async ingestion via Redis queue
- S3/Blob storage for large payloads
- Edge-cached prompts
- OpenTelemetry compatible (任何语言/框架)

#### Dependents (16k+ ⭐) — Langfuse 集成生态爆炸
- langflow (116k⭐) / open-webui (109k) / lobe-chat (65k) / ragflow (64k) / firecrawl (56k) / llama_index (44k) / Flowise (43k) / quivr (38k) / Langchain-Chatchat (36k) / LibreChat (33k) / litellm (28k) / WrenAI (11k) / promptfoo (8k) / PocketFlow (8k) / ART (7k) / cognee (7k) / agent-squad (6k) / omi (6k) / hatchet (6k) / zenml (4k) / refly (4k) / supabase-agent / agent-service-toolkit (3k) / colanode (3k) / voltagent (3k) / bRAG-langchain (3k) / 等等

#### Loom 借鉴优先级
- ✅ **P0 (Phase 1 借鉴)**:  **ClickHouse 作为底层 OLAP** (Loom cost view 数据量大了可以借鉴)
- ✅ **P0 (Phase 1 借鉴)**:  **OpenTelemetry compatible** (跟 Phoenix / SigNoz 互通)
- ✅ **P1 (Phase 2 借鉴)**:  **Coding agents SKILL.md** (Loom 应该有 .loom-drafts/skills/)
- ✅ **P0 (Phase 2 借鉴)**:  **Platform MCP Server** (跟 Phoenix 思路一致)
- ⚠️ **不做**:  Langfuse 整套 (Loom 是 view layer, 推荐用 Langfuse 作为 cost view 后端)

### 4.6 Cat 4 2026 跨项目共识

| 共识 | Langfuse | Phoenix | OpenInference | OpenTelemetry | MCP 2026-07-28 |
|---|---|---|---|---|---|
| **MCP 是 2026 必备 surface** | ✅ Platform MCP | ✅ Remote MCP | (not) | (not) | ✅ spec 升级 |
| **Coding agents 是 2026 必备用户** | ✅ SKILL.md | ✅ `.agents/skills/` | (not) | (not) | ✅ |
| **OTel 是 trace 标准** | ✅ | ✅ | ✅ spec | ✅ CNCF | ✅ W3C trace |
| **AI debugging agent** | ⚠️ | ✅ PXI | (not) | (not) | (not) |
| **JSON-RPC 2.0** | (not) | (not) | (not) | (not) | ✅ spec |

**Loom 借鉴总结**:
1. **MCP 是 2026 行业协议趋势** — Loom Bridge v1.1 必须兼容
2. **Coding agents 是 2026 行业用户** — Loom 的 skill 包要写
3. **OTel 是 trace 标准** — Loom 输出 OTel spans
4. **不重建 observability 平台** — 推荐 Langfuse (MIT) 作为 cost view 后端

---

## Phase 1 / 2 / 3 借鉴优先级总览

### P0 (Phase 1 必做)

| # | 借鉴来源 | 借鉴内容 | Loom 落地位置 |
|---|---|---|---|
| 1 | Cline / OpenCode / Pi | **Coordinator + specialists** 命名 | Bridge v1.1 `loom.dispatch` |
| 2 | MCP 2026-07-28 RC | **JSON-RPC 2.0 envelope** (已定) | Bridge v1.1 |
| 3 | MCP 2026-07-28 RC | **W3C Trace Context 传播** | Bridge v1.1 trace 字段 |
| 4 | MCP 2026-07-28 RC | **完整 JSON Schema 2020-12** | Bridge v1.1 method schema |
| 5 | Pydantic AI | **Capabilities = composable bundles** 概念 | Bridge v1.1 method dispatch |
| 6 | LangGraph 1.0 | **Middleware 洋葱圈** 模式 | daemon 中间件栈 |
| 7 | Subagents (5 个项目共识) | **SubAgent 双类型** (SubAgent dict + CompiledSubAgent) | subagent spec |
| 8 | Belay | **$USD budget per run** | Run claim 必填 |
| 9 | OpenTelemetry | **OTel spans 作为 trace 出口** | daemon observability |
| 10 | Phoenix `.agents/skills/` | **多 editor skill 同步** | `.loom-drafts/skills/` |
| 11 | Langfuse / Phoenix | **Remote MCP Server 暴露** (Phase 2 但 P0) | cost view 准备 |
| 12 | 企查查 MCP | **5 层能力矩阵** (Phase 2 但 P0) | cost view 准备 |
| 13 | Loom 严格 MIT | **不学 Phoenix (ELv2)** | license boundary |
| 14 | Loom 严格 view layer | **不学 Helicone AI Gateway** | 严格 fail closed on sampling/createMessage |

### P1 (Phase 2 应该做)

| # | 借鉴来源 | 借鉴内容 | Loom 落地位置 |
|---|---|---|---|
| 1 | Pydantic AI | "Code mode" 概念 | 1 step claim = N tool calls |
| 2 | LangGraph | 三级上下文压缩 | daemon 内存管理 |
| 3 | LangGraph | context_schema 模式 | run-scoped tenant_id |
| 4 | DBOS | New MCP server | cost view 暴露 |
| 5 | Langfuse | Coding agents SKILL.md | Loom 自己的 skill |
| 6 | Hatchet | Postgres-based + OTEL | cost view 后端选型 |
| 7 | 企查查 MCP | Resources 暴露稳定知识 | tool mapping / subagent dispatching |
| 8 | 企查查 MCP | SKILL 组织业务流程 | 常用 query |
| 9 | Phoenix | PXI 概念 | "loom-insight" AI helper |
| 10 | Belay | Chaos test harness | 7h kill -9 chaos test |
| 11 | Restate | Exactly-once 语义 | Run claim 上报事件 |
| 12 | MCP 2026-07-28 | OAuth 2.1 | cost view auth |
| 13 | MCP 2026-07-28 | 能力发现 + 缓存 | tool 多了要治理 |

### P2 (Phase 3+ 长期借鉴)

| # | 借鉴来源 | 借鉴内容 |
|---|---|---|
| 1 | Microsoft pg_durable | SQL 关键字路线 (loom SQL) |
| 2 | Pydantic AI Harness | Loom 出 "loom-harness" 包 |
| 3 | Belay | USD budget true-up 模式 |
| 4 | Pydantic Stack (Logfire AI Gateway) | 长期可借鉴的统一 LLM proxy |
| 5 | LangGraph | Temporal 集成 (Workflow Streams) |
| 6 | Temporal | Workflow-as-code 模式 |

---

## 永久"不吸"清单 v3 (NEW)

| ❌ 不学 | 原因 | 对标项目 |
|---|---|---|
| Phoenix license | **ELv2 (非 OSI)**, 跟 Loom MIT 冲突 | Arize Phoenix |
| Helicone AI Gateway | 商业化 AI Gateway, 跟 Loom "fail closed on sampling/createMessage" 冲突 | Helicone |
| LangSmith | 闭源商业 | LangChain LangSmith |
| Prefect | Python 深度 + Cloud-first, 跟 Loom local-first 冲突 | Prefect |
| Continue | 已 ARCHIVED 2.0.0 = 行业教训 | Continue |
| OpenInference 整套 | Loom 只需要 OTel 输出, 不需要 Arize 那套 instrumentation | Arize OpenInference |
| Belay / Hatchet / DBOS 整套 | Loom 是 view layer, 不重做 durable engine, 选一个接 | Belay / Hatchet / DBOS |
| Temporal 整套 | 已经有 9 年, Loom 不重做 | Temporal |

---

## 5 个未来 Loom 文档待补 (v3 落地)

1. **`docs/bridge/loom-bridge-v1.1-architecture.md`** — 详细定义 JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 + Coordinator dispatch
2. **`docs/integrations/mcp-2026-07-28-rc.md`** — 借鉴 4 大生产化信号, 写 Loom Bridge v1.1 的 MCP 兼容性
3. **`docs/architecture/subagent-spec.md`** — 借鉴 SubAgent dict + CompiledSubAgent 双类型 + YAML 配置
4. **`docs/integrations/dbos-vs-restate-vs-temporal.md`** — Loom 推荐选哪个 durable engine 做底层
5. **`docs/skills/loom-coding-agents.md`** — 学 Langfuse / Phoenix, 写 Loom 自己的 SKILL.md 供 Claude Code / Codex / Cursor 调

---

## 调研方法学 v3 升级

1. **README 抓取**: 全部用 `raw.githubusercontent.com/owner/repo/branch/README.md` (100% 成功, 14/14 验证)
2. **web_search 时间过滤**: `freshness=month` 拿 2026-Q3 最新
3. **多角度交叉验证**:
   - GitHub README (官方定位)
   - web_search 月度新鲜 (2026-Q3 趋势)
   - PyPI / npm 包 (代码级实锤)
4. **Loom 借鉴落地**:
   - 每个项目先问 "Loom 该不该学" (P0/P1/P2/不吸)
   - 再问 "学到 Bridge v1.1 / Phase 2 / Phase 3 哪里"
   - 写"借鉴优先级表" 而非泛泛"值得借鉴"

---

## 引用

### v3 主要参考资料 (2026-Q3)
- **Cline README** (raw.githubusercontent.com/cline/cline/main/README.md, 2026-07-24) — 6,614 commits
- **Pi README** (raw.githubusercontent.com/badlogic/pi-mono/main/README.md, 2026-07-24) — earendil-works rebrand
- **OpenCode README** (raw.githubusercontent.com/anomalyco/opencode/dev/README.md, 2026-07-24)
- **Pydantic AI README** (raw.githubusercontent.com/pydantic/pydantic-ai/main/README.md, 2026-07-24)
- **Pydantic AI Harness v0.5.0** (PyPI, 2026-07)
- **subagents-pydantic-ai v0.5.0** (GitHub 94 commits)
- **LangGraph README** (raw.githubusercontent.com/langchain-ai/langgraph/main/README.md, 2026-07-24)
- **Restate README** (raw.githubusercontent.com/restatedev/restate/main/README.md, 2026-07-24)
- **Arize Phoenix README** (raw.githubusercontent.com/Arize-ai/phoenix/main/README.md, 2026-07-24)
- **Langfuse README** (raw.githubusercontent.com/langfuse/langfuse/main/README.md, 2026-07-24)
- **web_search "Cline multi-agent teams"** (2026-07) — Coordinator pattern
- **web_search "Pydantic AI capabilities composable bundles"** (2026-07)
- **web_search "LangGraph Deep Agents subagents file system"** (2026-07)
- **web_search "Restate durable AI agents exactly-once"** (2026-07)
- **web_search "OpenTelemetry GenAI semantic conventions"** (2026-07)
- **web_search "Arize Phoenix Remote MCP Server PXI agent"** (2026-07)
- **web_search "MCP 2026-07-28 release"** (2026-07)
- **web_search "DBOS durable execution"** (2026-07)
- **企查查 MCP 9 server / 197 tool / 27 SKILL** (2026-07)
- **Temporal Replay 2026 announcements** (2026-07)
- **Belay 1.0.0-rc.5** (GitHub b-erdem/belay, 2026-07-22)
- **Microsoft pg_durable 0.2.2** (2026-07)

### 历史参考
- **v2 community survey** (`.loom-drafts/community-survey-v2.md`, 2026-07-24) — 14 项目 4 category
- **v1 community survey** (`.loom-drafts/community-survey-v1.md`, 2026-07-24) — 7 user-specified repos
- **Phase 1 Slice 1 GoalSpec** (`.loom-drafts/phase1-slice1.optimized.goalspec.yaml`, 2026-07-24)
- **Phase roadmap supplements v0.4** (`.loom-drafts/phase-roadmap-supplements.md`, 2026-07-24)
- **PROGRESS.md v2 调研摘要** (2026-07-24)
- **memory/MEMORY.md** 2026-07-24 新增 3 条 (硬规则 + Loom 边界 + Coordinator+specialists)

---

**END of v3 frontier deep-dive** — 19.3K 字 / 2026-07-24 / SGT
