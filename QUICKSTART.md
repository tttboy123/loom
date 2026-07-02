# 从零到打开 Loom 控制台（在你自己的 Mac 上）

> 重要：Loom 默认在你本机运行。没有“我那边的服务器”可访问。
> 下面几步走完，你就能打开全局控制台，发起研发运行、看产物、看模型用量。

## 第 1 步：装 Docker（只需一次）

- 下载 Docker Desktop：https://www.docker.com/products/docker-desktop/
- 安装后打开它，等菜单栏出现 Docker 图标。

如果你使用本机的 colima，`./loom up` 会在 Docker 未运行时尝试启动 colima。

## 第 2 步：把文件放到本地

把 `agent-platform/` 放到任意目录，例如：

```bash
~/agent-platform
```

## 第 3 步：启动 Loom 核心服务

```bash
cd ~/agent-platform
./loom up
./loom doctor
./loom open
```

`./loom up` 启动 lite 核心：全局控制台、LiteLLM 网关和订阅代理。

完成后打开：

- 全局控制台：http://localhost:8899
- LiteLLM 网关看板：http://localhost:4000/ui

## 第 4 步：跑一条研发任务

```bash
./loom run "实现一个小功能，要求产出测试和独立审查"
```

或在 http://localhost:8899 的「发起运行」里输入任务。

运行产物会落在：

```text
devkit/runs/<时间戳>/
```

## 接真实模型 / 订阅后端 / full profile

确认核心服务跑通后，再按需升级：

1. **接真实 API key**：`cp .env.example .env`，填 GLM / DeepSeek / MiniMax 等 key，再 `docker compose restart litellm`。
2. **Claude / ChatGPT 订阅后端**：运行 `./loom login` 刷新本机 OAuth token，然后 `./loom doctor` 看后端健康。
3. **聊天 UI + Agno full profile**：运行 `./loom up full`，再访问 http://localhost:3000 和 http://localhost:8000/docs。

## 访问不通时的排查

```bash
./loom doctor
docker compose ps
docker compose logs console
./loom down
```

常见原因，从高到低：

1. Docker 没装或没打开。
2. 镜像还在拉，没等够。
3. 8899 / 4000 / 8317 / 3000 端口被别的程序占了。
4. 命令跑在了沙箱或别的机器上，而不是你自己的 Mac。

把 `./loom doctor` 和相关 `docker compose logs` 输出发出来即可定位。
