# Claude Code Deployment Notes

> 这份文件现在只作为“让 Claude Code 帮你在本机部署 Loom”的执行说明。当前推荐入口是 `./loom` CLI，而不是旧的 `bash deploy.sh full` 主路径。

## 目标

在本机启动 Loom 的自治 Agent Team：

- 控制台：http://localhost:8899
- LiteLLM 网关：http://localhost:4000
- 订阅代理：http://localhost:8317
- full profile 可选 UI：http://localhost:3000

当前默认模型分工：

| 角色 | carrier | 模型 |
| --- | --- | --- |
| 产品判断 | `loom-product` | GPT-5.4 |
| 编排 | `loom-orchestrator` | GPT-5.4 |
| 开发 | `loom-dev` | MiniMax-M3 |
| 测试 | `loom-tester` | MiniMax-M3 |
| 审查 | `loom-reviewer` | GPT-5.4 |

## 风险提示

Claude / ChatGPT 订阅代理经 CLIProxyAPI 暴露成本地 OpenAI-compatible API，可能违反对应产品服务条款，也可能触发限流或失效。执行 `./loom login` 前必须让用户确认是否接受这个风险。

只使用 API key 后端时，可以跳过订阅登录；但 GPT-5.4 角色载体当前走 `cliproxy`，需要可用的 ChatGPT/Codex 订阅代理或对应兼容后端。

## 部署步骤

### 1. 检查 Docker

```bash
docker info >/dev/null 2>&1 && echo "docker ok" || echo "需要先安装并打开 Docker Desktop"
```

如果本机使用 colima，`./loom up` 会在 Docker 引擎未运行时尝试启动 colima。

### 2. 准备 `.env`

```bash
cp -n .env.example .env
```

至少确认这些值：

- `LITELLM_MASTER_KEY`
- `MINIMAX_API_KEY`
- `ZHIPU_API_KEY`
- `DEEPSEEK_API_KEY`

MiniMax-M3 是开发/测试主力；GLM/DeepSeek 是 fallback。

### 3. 启动 lite 核心

```bash
./loom up
./loom doctor
```

lite profile 启动：

- console
- litellm
- cliproxy
- postgres / redis 等基础服务

### 4. 登录订阅后端

仅当用户确认接受订阅代理风险时执行：

```bash
./loom login
./loom doctor
```

`./loom login` 会调用宿主机 `../cliproxy/cli-proxy-api`，依次刷新 Claude 与 Codex/ChatGPT OAuth token，然后重启 `cliproxy`。

### 5. 启动 full profile

只有需要 Agent UI / AgentOS API 时才启动：

```bash
./loom up full
```

访问：

- http://localhost:3000
- http://localhost:8000/docs

### 6. 验证 Agent Team

```bash
./loom doctor
python3 -m devkit roles list
python3 -m devkit auto --backlog devkit/backlog.json --dry-run
```

跑一条小任务：

```bash
./loom run "用最小改动实现一个纯函数，并给出测试和独立审查"
```

产物位置：

```text
devkit/runs/<run-id>/
devkit/RUNS.md
```

## 常见问题

- `claude-code-sub` 或 `codex-sub` 显示过期：运行 `./loom login`。
- `loom-dev` 或 `loom-tester` 失败：检查 `MINIMAX_API_KEY` 和 LiteLLM 日志。
- `loom-product` / `loom-orchestrator` / `loom-reviewer` 失败：检查 cliproxy 的 Codex/ChatGPT 订阅代理是否可用。
- 端口冲突：查看 `docker compose ps` 和 `docker compose logs <service>`。

## 验证命令

```bash
./loom doctor
ruby -e 'require "yaml"; YAML.load_file("litellm/config.full.yaml"); puts "yaml ok"'
```
