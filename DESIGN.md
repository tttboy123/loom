# Loom 底座架构：多厂商多 Agent 平台

> 状态：历史/底座参考。
> 本文记录 Loom 早期的 Agno + LiteLLM + CrewAI 可选子服务架构。当前产品总览以 [README.md](./README.md)、[USAGE.zh.md](./USAGE.zh.md)、[VISION.md](./VISION.md) 和 `docs/pending-decisions/` 为准。
> 当前主路径是 `devkit` 研发闭环 + `:8899` 全局控制台；Agno / AgentOS / Agent UI 是 full profile 下的可选协作与聊天层。

## 1. 目标与核心判断

诉求：同时使用多 Agent 系统和多厂商模型（Claude / GLM / OpenAI / DeepSeek / Kimi），可为每个 Agent 配置模型，并为每个 Agent 定义降级（fallback）策略。云上部署，关注弹性与高可用。

核心设计原则是**分层解耦**，把问题切成互不耦合的两层：

- **网关层（LiteLLM）** 负责“接哪些厂商、挂了切哪个”——多厂商接入、跨厂商降级、重试、冷却、限流、计费、用量。
- **编排层（Agno）** 负责“有哪些 Agent、谁用哪个模型、怎么协作”——per-agent 模型配置、Team 协作、session、tracing、RBAC、调度。

关键决策：**框架层不做降级，降级全部下沉到网关层。** 这样业务代码只声明“用哪个逻辑模型”，运维只改一个 YAML 就能调整全平台的厂商接入与降级链，二者独立演进。

只选一个主编排框架（Agno）。CrewAI 不做框架嵌套，而是作为**可选的独立子服务**，通过 HTTP/Tool 被主系统调用——服务边界混用，不共享内存/路由/状态机，避免“两套一切”的调试地狱。

## 2. 架构总览

```
            ┌────────────────────────────────────────────┐
   用户/API │              AgentOS (Agno)                  │  ← 主编排层
            │  Agents:  Researcher / Planner / Writer /    │    per-agent 模型
            │           LongContextReader                  │    Team 协作
            │  Team:    ResearchTeam                        │    session/tracing/RBAC
            │  每个 Agent 只引用“逻辑模型名”               │
            └───────────────┬──────────────────────────────┘
                            │ OpenAI 兼容协议（model = 逻辑名）
                            ▼
            ┌────────────────────────────────────────────┐
            │           LiteLLM Gateway                    │  ← 网关层
            │  逻辑名 → 厂商模型映射                        │    多厂商接入
            │  fallbacks: 跨厂商降级链                      │    重试/冷却/限流
            │  context_window / content_policy fallbacks   │    计费/用量
            └───┬─────────┬─────────┬─────────┬─────────┬──┘
                ▼         ▼         ▼         ▼         ▼
             Claude     GPT      GLM     DeepSeek    Kimi
                            （各厂商真实 API）

  可选：某些 crew 式深度协作任务
            AgentOS 的一个 Agent ──HTTP Tool──► CrewAI 子服务（独立部署，也走 LiteLLM）
```

## 3. 两层职责对照

| 关注点 | 放在哪层 | 怎么实现 |
|---|---|---|
| 每个 Agent 用什么模型 | 编排层 Agno | `Agent(model=gw("claude-sonnet"))`，引用逻辑名 |
| 每个 Agent 怎么降级 | 网关层 LiteLLM | `litellm_settings.fallbacks` 按逻辑名定义跨厂商链 |
| 长上下文超限降级 | 网关层 | `context_window_fallbacks` |
| 内容审查降级 | 网关层 | `content_policy_fallbacks` |
| 多 Agent 协作 | 编排层 | Agno `Team` |
| session/历史/审计 | 编排层 | AgentOS + Postgres |
| 限流/计费/用量 | 网关层 | LiteLLM + Postgres + Redis |
| crew 式子任务（可选） | 独立子服务 | CrewAI as HTTP Tool |

## 4. 降级（fallback）策略设计

降级在 LiteLLM 的 `model_name`（逻辑名）粒度触发，主模型遇到错误/限流/超时/服务不可用时，按列表顺序切换，可跨厂商。本方案默认链路：

| 逻辑名（用途） | 主模型 | 降级链 |
|---|---|---|
| `claude-sonnet`（高质量主力） | Claude Sonnet | → GPT 旗舰 → GLM 旗舰 |
| `deepseek-reasoner`（推理） | DeepSeek-Reasoner | → Claude Sonnet → GLM 旗舰 |
| `glm-air`（便宜/后台） | GLM-Air | → DeepSeek-Chat → Kimi |
| `claude-haiku`（轻量总结） | Claude Haiku | → GLM-Air → DeepSeek-Chat |
| `gpt-flagship`（长上下文） | GPT 旗舰 | → Claude → GLM（含 context 超限专链） |

设计要点：降级链尽量**跨厂商**（一家挂了/限流，立刻换另一家），且尽量**同档质量或同档成本**，避免降级后质量或费用断崖。某厂商没填 key 时，它在链里自然被跳过。

## 5. 目录结构

```
agent-platform/
├── DESIGN.md                  # 本文档
├── README.md                  # 快速上手
├── litellm/config.yaml        # ★ 多厂商接入 + 降级链（运维主战场）
├── app/                       # ★ Agno 主编排
│   ├── settings.py            #   逻辑模型名常量、网关地址
│   ├── agents.py              #   每个 Agent 用什么模型
│   ├── teams.py               #   多 Agent 协作
│   ├── tools/crew_tool.py     #   把 CrewAI 子服务包成 Tool
│   └── server.py              #   AgentOS 入口（FastAPI）
├── crew_service/crew_app.py   # 可选的 CrewAI 子服务
├── docker-compose.yml         # 本地/单机一键起栈
├── Dockerfile / Dockerfile.crew
├── requirements*.txt
└── k8s/platform.yaml          # 云上部署 + HPA 弹性扩展
```

## 6. 云上部署要点

- **无状态层（AgentOS、LiteLLM、CrewAI）** 跑多副本 + HPA 自动扩缩；状态全部外置。
- **Postgres**：AgentOS 的 session/审计 + LiteLLM 的计费/key 管理，建议用云托管实例。
- **Redis**：LiteLLM 多副本必需，用于跨副本共享限流计数与路由冷却状态。
- **密钥**：厂商 key 走 K8s Secret（`model-keys`），LiteLLM 配置走 ConfigMap，二者解耦。
- **可观测**：AgentOS 自带 OpenTelemetry，可接 Langfuse / Phoenix / Logfire；LiteLLM 自带用量与成本看板。
- **CrewAI 子服务默认不部署**（compose 里用 profile 关掉，k8s 里默认不 apply），需要 crew 模式时再开。

## 7. 演进建议（重要）

先用**纯 Agno + LiteLLM 单栈**跑通——它已覆盖 per-agent 模型、跨厂商降级、Team 协作、平台化管理这四件核心诉求。CrewAI 留接口（`crew_tool.py` + `crew_service/`）不留依赖，等真的遇到 Agno Team 表达不了的 crew 协作场景，再把那块子任务切到 CrewAI 服务。避免一开始就背两套框架的复杂度。

> 注：Agno API 以 v2.x 为准；若版本有差异，主要是 import 路径与个别参数名，逻辑分层不变。
