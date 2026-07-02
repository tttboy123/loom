# 本地部署 / 跑起来看效果

四条路径，从“启动 Loom 核心控制台”到“只验证网关 fallback / 调试底座”，按需选。

## 路径 A：Loom 核心控制台（推荐先做）

```bash
cd agent-platform
./loom up
./loom doctor
./loom open
```

你应该能打开：

- 全局控制台：http://localhost:8899
- LiteLLM 看板：http://localhost:4000/ui
- 订阅代理：http://localhost:8317

然后跑一条任务：

```bash
./loom run "实现一个小功能，要求有测试和审查"
```

运行产物在 `devkit/runs/<时间戳>/`。

## 路径 B：零 key 网关冒烟（只验证 LiteLLM fallback）

不需要任何厂商 key、不花钱，用 mock 看到「网关正常返回」和「主模型坏掉→自动降级」。

```bash
cd agent-platform
pip install "litellm[proxy]"

# 终端 1：起网关（mock 配置）
litellm --config litellm/config.mock.yaml --port 4000

# 终端 2：跑冒烟测试
bash scripts/smoke_test.sh
```

你应该看到类似输出：

```text
1) 正常主力模型 claude-sonnet:
【mock claude-sonnet】我是主力模型，正常返回。

2) 故意调坏掉的 glm-flagship（应自动降级到 gpt-flagship）:
【mock gpt-flagship】我是降级后的备用模型，已成功兜底。
```

第 2 条能返回内容、而不是报错，就证明跨厂商降级链路是通的。

## 路径 C：Docker Compose 接真实 key

```bash
cd agent-platform
cp .env.example .env
# 编辑 .env，填你有的 key；没有的留空，对应模型会在降级链里被自动跳过

docker compose up -d
docker compose ps
```

访问：

- 全局控制台：http://localhost:8899
- LiteLLM 网关 / 用量看板：http://localhost:4000/ui
- full profile 的 AgentOS API：http://localhost:8000/docs
- full profile 的 Agent UI：http://localhost:3000

用真实 key 验证降级：

```bash
KEY=$(grep LITELLM_MASTER_KEY .env | cut -d= -f2) bash scripts/smoke_test.sh
```

需要聊天 UI / Agno 服务：

```bash
docker compose --profile full up -d
```

需要 CrewAI 子服务：

```bash
docker compose --profile crew up -d
```

停止：

```bash
docker compose down
```

加 `-v` 会连数据库卷一起清掉。

## 路径 D：不用 Docker，纯 Python 调试底座

适合想直接 debug Agno / AgentOS 代码的情况。需要本机有 Postgres，或临时改用 SQLite。

```bash
cd agent-platform
python3.12 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
pip install "litellm[proxy]"

export $(grep -v '^#' .env | xargs)

# 终端 1：起网关
litellm --config litellm/config.yaml --port 4000

# 终端 2：起 AgentOS
python -m app.server
# → http://localhost:8000/docs
```

> 没有本地 Postgres？把 `app/settings.py` 里 `DATABASE_URL` 默认值改成
> `sqlite:///./agno.db` 即可单机先跑通。生产仍建议 Postgres。

## 常见问题

- `litellm: command not found`：装的是 `litellm` 而不是 `litellm[proxy]`，重装后者。
- 降级没触发、直接报错：检查 `config.yaml` 里 `fallbacks` 的 `model_name` 和 `model_list` 名字是否完全一致。
- 某厂商一直失败：单独测它，先确认 key 和 `api_base`。
- 想换角色用什么模型：优先改 `loom.roles.toml` 或控制台角色映射；底层厂商映射在 `litellm/config.full.yaml`。
