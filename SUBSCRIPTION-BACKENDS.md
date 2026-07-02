# 同时使用订阅产品（Claude Code 非 API / Codex）+ API

## 这是什么 / 为什么能行

Claude Code、Codex 这类产品是用**订阅额度**（你的 Claude / ChatGPT 账号 OAuth 登录），
不是按 token 计费的 API。要把它们当成多 Agent 系统的「模型后端」，需要一个中间件，
把它们的本地 CLI / OAuth 会话**包装成 OpenAI 兼容接口**，然后让上层（LiteLLM → Agno）
像调普通 API 一样调它们。

社区里做这件事最成熟的开源项目是 **CLIProxyAPI**（Go，~24k star，活跃维护）：
一个本地代理，把 ChatGPT Codex、Claude Code、Gemini、Qwen 等的 OAuth 登录
统一暴露成 OpenAI/Claude/Gemini 兼容端点，支持多账号轮询和失败切换。

接进来后，你的栈变成：

```
Agno 多 Agent ──► LiteLLM 网关 ──┬─► CLIProxyAPI(:8317) ─► Claude 订阅 (非API)
  每个 Agent 配模型   统一降级     │                       └─► Codex 订阅  (非API)
                                  └─► 厂商 API ─► Claude / GPT / GLM / DeepSeek
```

订阅额度优先用（不额外花钱），耗尽或被限流时，LiteLLM 自动降级到付费 API。

---

## ⚠️ 先读这一段（重要、诚实提醒）

- **可能违反产品服务条款。** 用第三方代理把订阅产品当 API 用，通常不在 Claude / ChatGPT
  的官方授权范围内，可能触发**限流甚至封号**。是否使用、用谁的账号，请你自己权衡。
- **Anthropic 已在主动封堵。** 2026 年 4 月起 Anthropic 限制了第三方 harness 使用
  Claude Max 订阅额度；这类工具与官方处于持续的「猫鼠博弈」，**随时可能失效**。
  Codex / ChatGPT 侧风险类似。
- **稳妥的生产做法仍是用 API**（前面 `litellm/config.yaml` 那套）。订阅后端适合
  个人 / 本地 / 试验性使用，别把它放进对稳定性有要求的生产链路。
- 正因为以上，我把订阅后端做成**可选叠加层**：API 那套照常能独立跑。

---

## 本地落地步骤（在你自己的 Mac 上，非 mock）

### 1. 装 CLIProxyAPI

二进制（最简单，免依赖）：去 Releases 下对应 macOS 架构的包，解压得到 `cli-proxy-api`。
- https://github.com/router-for-me/CLIProxyAPI/releases

或用 Docker（见仓库 `docker-compose.yml`）。OAuth 登录用二进制更省事，推荐先用二进制。

把本仓库的 `cliproxy/config.yaml` 放到 CLIProxyAPI 目录（或用它自带的 config.example.yaml 改）。

### 2. 登录你的订阅账号（这一步只有你能做，结果就在这里产生）

```bash
./cli-proxy-api --claude-login     # 浏览器弹出 → 登录你的 Claude 账号
./cli-proxy-api --codex-login      # 浏览器弹出 → 登录你的 ChatGPT 账号
# token 会写到 ~/.cli-proxy-api/ 下的 json
```

### 3. 启动代理并确认它真的在服务

```bash
./cli-proxy-api --config ./config.yaml
# 监听 http://localhost:8317

# 看看它暴露了哪些模型名（用来核对 LiteLLM 配置里的 model 字段）
curl -s http://localhost:8317/v1/models \
  -H "Authorization: Bearer sk-cliproxy-local" | python3 -m json.tool

# 直接打一发，验证「订阅后端」真能出 token（非 mock）：
curl -s http://localhost:8317/v1/chat/completions \
  -H "Authorization: Bearer sk-cliproxy-local" -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"说一句话证明你在工作"}]}'
```

> 若第 2 步返回的模型名和 `litellm/config.with-subscriptions.yaml` 里写的不一致
> （比如 codex 那行的 `gpt-5.1-codex`），按 `/v1/models` 的真实结果改 LiteLLM 配置即可。

### 4. 让 LiteLLM + Agno 接上订阅后端

```bash
cd agent-platform
# 用带订阅的 LiteLLM 配置启动网关
docker compose up -d                      # 已挂载，详见下方 compose 说明
# 或原生：litellm --config litellm/config.with-subscriptions.yaml --port 4000
```

通过网关验证「订阅 + API 混合降级」（仍然非 mock）：

```bash
KEY=$(grep LITELLM_MASTER_KEY .env | cut -d= -f2)
curl -s http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"model":"claude-code-sub","messages":[{"role":"user","content":"hi"}]}'
# 正常 → 用的是你的 Claude 订阅；把订阅停掉/退登 → 自动降级到 Claude API
```

Agno 侧已经给了示例：`app/agents_subscription.py` 里两个 Agent 分别绑
`claude-code-sub` 和 `codex-sub`，把它们加进 `app/server.py` 的 agents 列表即可在
AgentOS 控制台里直接对话。

---

## docker-compose 接法

把 LiteLLM 服务的配置换成订阅版，并让它能访问宿主机上的 CLIProxyAPI：

```yaml
# docker-compose.override.yml （放在 agent-platform/ 下，compose 会自动合并）
services:
  litellm:
    volumes:
      - ./litellm/config.with-subscriptions.yaml:/app/config.yaml:ro
    extra_hosts:
      - "host.docker.internal:host-gateway"   # 容器内可访问宿主机 :8317
```

CLIProxyAPI 因为要走浏览器 OAuth 登录，建议**原生跑在宿主机**（上面第 2-3 步），
不放进 compose；LiteLLM 用 `host.docker.internal:8317` 连它。

---

## 一句话总结

技术上完全可行，开源工具就是 **CLIProxyAPI**，本仓库已把它和 LiteLLM + Agno 接好。
唯一你必须亲手做的是「用你的账号 OAuth 登录」那一步——也正因如此我没法替你在这里跑出结果。
跑之前请认真看上面的服务条款 / 封号风险提醒。
