# Loom 产品文档（正式版）

Loom 是一个 local-first 的 Agent 团队编排与治理平台：单一 Event Journal
作为状态权威，Scheduler 常驻，短生命周期 Worker Pool 并发执行，单写
Integrator 发布版本化 Release，Timeline/Attention 仅是可重建投影。本
目录是面向用户与开发者的正式产品文档，覆盖当前已验收成果（v0.4.1
whole-slice 通过，分支 `codex/loom-platform-slice2`）。

## 文档地图

| 文档 | 内容 |
|---|---|
| [能力矩阵](CAPABILITY-MATRIX.md) | 当前产品能力全表：能力、权威归属（WorkItem/commit）、状态、验证与客户端表面 |
| [Runbook 全集](RUNBOOKS.md) | 全部跨客户端旅程 runbook 的索引、场景、验证脚本与已消费证据 |
| [TUI 使用说明](TUI-GUIDE.md) | 终端 TUI 的启动、12 屏导航、全部按键与输入模式 |
| [原生 app 使用说明](NATIVE-APP-GUIDE.md) | macOS 原生 app 的启动、界面区域与 daemon 连接 |

配套架构与治理文档：

- 当前状态快照：[`../CURRENT.md`](../CURRENT.md)
- 架构总览：[`../ARCHITECTURE.md`](../ARCHITECTURE.md)、[`../architecture/`](../architecture/)
- 决策记录（ADR）：[`../adr/`](../adr/)（含 ADR-0014 Agent Scheduling Framework）
- 开发流程：[`../DEVELOPMENT.md`](../DEVELOPMENT.md)
- 验收证据：`.loom-evidence/agent-scheduling/`（v0.4.0/v0.4.1 全部评审与
  source-lock）、`.loom-evidence/phase3a/`、`.loom-evidence/phase2a/`

## 快速开始（本地产品三件套）

```sh
# 1. 构建
go build -o /tmp/loom ./cmd/loom
go build -o /tmp/loomd ./cmd/loomd
scripts/build-loom-local-app.sh --output /tmp/Loom.app

# 2. 启动 daemon（PTY 会话；runtime-dir/probe-id/local-model 三件套）
/tmp/loomd --state /tmp/loom/state/loom.db \
  --isolation-root /tmp/loom/isolation \
  --probe-id demo --socket /tmp/loom/loomd.sock \
  --runtime-dir "/Users/<you>/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/.bin" \
  --instance-id runtime.pi.earendil-works.0.82.1 --device-id local-mac \
  --display-name "Pi 0.82.1" --interval 1s --process-timeout 10s

# 3. 连接客户端
/tmp/loom app --socket /tmp/loom/loomd.sock          # 真实 PTY TUI
open /tmp/Loom.app --args --socket /tmp/loom/loomd.sock   # 原生 app
```

> 说明：本地模型（`--local-model-private-root` /
> `--local-model-executable` / `--local-model-path` 三参数必须同时给出）
> 仅用于受控离线 canary 与本地模型目录绑定；不带这三参数时 daemon 仍可
> 运行，只是不会发布本地模型目录。

## 边界声明

以下能力**不在**当前已验收范围内：Web/Marketplace/共享/多用户、
Phase 3B 风格路由、Provider fallback/checkpoint、自动批准绕过人工通道、
隐藏推理或逐 token 写入 Journal、自动激活、联网/付费/真实用户配置、
远端 push/merge。当前产品是 local-first、离线、确定性验证的开发者
平台；任何 Agent 执行都要求显式人工触发与确认。
