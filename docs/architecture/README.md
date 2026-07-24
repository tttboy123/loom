# Loom 技术架构

本目录按 C4 模型拆分整体技术架构。每张图只表达一个抽象层级，避免把产品流程、进程部署和代码组件混在同一张图中。

| 文档 | 读者 | 回答 |
|---|---|---|
| [System Context](c4-context.md) | 所有人 | Loom 与用户、Agent Runtime、Provider 和外部协作系统的边界 |
| [Containers](c4-containers.md) | 架构与开发 | 哪些单元独立运行或持久化 |
| [Daemon Components](c4-components-daemon.md) | 开发者 | Go daemon 内部模块及调用方向 |
| [Deployment](c4-deployment.md) | 开发与运维 | 本地进程、SQLite、Agent 子进程和外部网络如何部署 |
| [Explicit Agent Flow](c4-dynamic-agent-mode.md) | 产品与开发 | 用户显式使用 Agent 后的完整动态链路 |
| [Trust Boundaries](trust-boundaries.md) | 安全与开发 | 哪些进程和数据可以相互信任 |

关键选择的背景、被否决方案和后果见 [Architecture Decision Records](../adr/README.md)。

## 架构结论

- Phase 1 使用本地模块化单体 daemon，不拆微服务。
- CLI/TUI 是独立客户端，不持有任务状态权威。
- 外部 Agent CLI 作为受监督子进程运行，不进入 Loom 可信进程。
- Event Journal 与 Read Model 共用一个 SQLite 文件，但保持逻辑分层。
- 大型 Evidence 使用 daemon 独占的本地内容寻址 Artifact Store。
- Credential Broker 位于 daemon 内，真实 Provider 凭证不进入 Agent 进程。
- Evolution Sidecar 是独立本地进程，只通过 daemon 的受限 API 访问脱敏终态和候选。
- 普通对话不创建团队；显式 Agent 意图才进入团队解析。

## 运行与故障边界

| 故障位置 | 预期行为 |
|---|---|
| Client 退出 | Daemon 和 Run 继续；重连后从 Projection 恢复 UI |
| Daemon 退出 | 子进程组被清理或在重启时对账；Event Journal 重建 Projection |
| Agent Runtime 退出 | 对应 Run 写 terminal failure；不会把 WorkItem 标记为 Done |
| Provider/Router 失败 | 记录认证、限流、模型、网络或服务失败；是否 fallback 由 RuntimeProfile 和客户规则决定 |
| SQLite 写失败 | 停止接收新的状态转移，不回退到内存权威 |
| Artifact digest 不匹配 | 拒绝 Evidence，不追加有效 Evidence 引用 |
| Sidecar 退出 | 不影响当前 Team、Run 或批准；下次空闲时重试候选生成 |

## Phase 1 容量模型

- 单个 daemon、单个 SQLite writer；
- 查询使用独立只读连接，写入通过有界队列串行化；
- 每个 RuntimeProfile 和 TeamInstance 都有并发上限；
- Bridge、Event 和 Evidence staging 都有大小与时间上限；
- 达到容量时排队或拒绝新 Run，不丢弃已有 terminal、Evidence 或批准记录。
