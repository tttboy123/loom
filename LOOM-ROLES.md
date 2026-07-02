# Loom 角色「模型载体」层

目标：让你**后续只编排角色，不用每次重建编排系统**。角色与厂商解耦——
角色名稳定，背后用哪个模型在一个配置文件里调。

## 三层结构

```
你的 Loop / harness（LangGraph、Codex、脚本…）
        │  只引用稳定角色名（model = "loom-dev"）
        ▼
LiteLLM 网关 :4000   ← 角色「模型载体」+ 跨厂商降级 + 统一用量/成本日志
        │  litellm/config.full.yaml 的 loom-* 条目决定每个角色用哪个厂商
        ▼
5 个厂商后端（Claude订阅 / Codex订阅 / GLM / MiniMax / DeepSeek）
```

平台内还另外提供了**现成的角色 Agent + 编排 Team**（Agno），在 full profile 下可用，见下文「方式 B」。

## 角色 → 载体 → 厂商（默认映射）

| 角色 key | 载体名 (model) | 默认厂商 | 降级链 |
| --- | --- | --- | --- |
| product | `loom-product` | `codex-sub` | MiniMax-M3 → MiniMax-M2.7-highspeed → MiniMax-M2.7 → GLM → DeepSeek |
| orchestrator | `loom-orchestrator` | `codex-sub` | MiniMax-M3 → MiniMax-M2.7-highspeed → MiniMax-M2.7 → GLM → DeepSeek |
| dev | `loom-dev` | MiniMax-M3 | MiniMax-M2.7-highspeed → MiniMax-M2.7 → GLM → DeepSeek |
| tester | `loom-tester` | MiniMax-M3 | MiniMax-M2.7-highspeed → MiniMax-M2.7 → GLM → DeepSeek |
| reviewer | `loom-reviewer` | `codex-sub` | MiniMax-M3 → MiniMax-M2.7-highspeed → MiniMax-M2.7 → GLM → DeepSeek |

> 当前默认口径是：**控制面走 `codex-sub`，执行面走 `MiniMax-M3`，`MiniMax-M2.7` 系列负责 fallback**。
> 如果某次任务要强调 reviewer 的额外独立盲区，可临时把 `loom-reviewer` remap 到 `glm` 或 `codex-sub`。

### MiniMax 请求口径

- `litellm/config.full.yaml` 里的 MiniMax 载体统一使用 provider-native 模型名：`minimax/MiniMax-M3`
- Loom 在 `devkit/rdloop.py` 内会把 MiniMax 请求的 `max_tokens` 自动改写为 `max_completion_tokens`
- 同时固定发送 `thinking={"type":"disabled"}` 和 `reasoning_split=true`

这三项要一起成立。否则小预算请求容易出现 `reasoning_content` 有值但 `content=""` 的空答复。

## 怎么用

### 方式 A：任意框架，把角色名当模型名（最通用，框架无关）

```bash
KEY=$(grep '^LITELLM_MASTER_KEY' .env | cut -d= -f2)
curl -s http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"model":"loom-dev","messages":[{"role":"user","content":"实现 X，先写失败测试"}]}'
```

LangGraph 节点 / Codex 工具 / 任意 OpenAI SDK 同理：`base_url=http://localhost:4000`，
`model` 填 `loom-product` / `loom-tester` / … 即可。换厂商不用改这里。

### 方式 B：平台内现成的角色 Agent / 编排 Team（Agno，可选 full profile）

- 先启动 full profile：`./loom up full`。
- Agent UI http://localhost:3000 的 Agent 下拉里直接有 5 个角色 + `LoomRDLoop` 团队。
- 或走 AgentOS API（:8000 / :7777）：`GET /agents`、`GET /teams`，
  对单个角色或整个 `LoomRDLoop` 发任务，由 orchestrator 自动按阶段分派。

日常发起研发运行仍推荐走全局控制台 http://localhost:8899 或 `./loom run "任务"`；它们会统一产出 run 账本、artifact、gate 和成本记录。

## 改一个角色的厂商（唯一要动的地方）

编辑 `litellm/config.full.yaml` 里对应的 `loom-*` 条目，例如确认审查走 `codex-sub`：

```yaml
  - model_name: loom-reviewer
    litellm_params:
      model: openai/gpt-5.5
      api_base: http://host.docker.internal:8317/v1
      api_key: sk-cliproxy-local
```

然后 `docker compose restart litellm`。**Loop 代码、角色注册表都不用改。**

## 新增一个角色

1. `app/roles.py` 的 `ROLES` 加一条（key / name / model=`loom-xxx` / 职责 / 指令）。
2. `litellm/config.full.yaml` 加一个同名 `loom-xxx` 载体 + 一条 fallback。
3. `docker compose up -d --build agentos && docker compose restart litellm`。

## 配套提醒（来自 Loom 宪章）

- **Eval Gate**：每次给角色换载体/厂商，按你的 golden 集重跑一遍再采用。
- **可观测**：每次角色调用都过 LiteLLM，`:4000/ui` 有用量/成本/降级日志——
  这是成本台账、trace-cost schema 和后续 cockpit 的单一收口。
- **降级会换厂商**：记录每次运行**实际**由哪个模型服务（LiteLLM 会记），别假设永远是默认厂商。
