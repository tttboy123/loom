# Loom 产品文档

Loom 是一个 Conversation-first、local-first 的 Agent Team 编排与治理产品。
用户先在熟悉的对话流中与 Agent 结对；当工作需要多个 Agent、明确工作流、
权限、预算、Evidence 或恢复边界时，再把目标升级为 Mission。

当前发布候选为 `v0.5.3-rc.1`，对应已安装验证的 `0.5.3` Build 188。
Phase 2D 在既定范围内为 `ACCEPTED / COMPLETE`。精确状态和延期项以
[CURRENT](../CURRENT.md) 为准。

## 用户心智模型

| 概念 | 用户看到的含义 |
|---|---|
| **Conversation** | 默认入口和结对编程对话流；普通对话不会隐式创建 Team。 |
| **Mission** | 与 Conversation 可关联的一次有标题工作流；点开后查看该次执行的 Team、过程、结果和恢复。 |
| **Agent Team** | 可复用的 Agent 编排；每个 Agent 可独立选择 Harness、Provider Account 和模型。 |
| **Runtime & Providers** | 管理 Harness 检测、Provider Account、Credential Vault、模型与诊断。 |
| **RoundTable** | 用于受治理协商和交接；不是普通任务的默认入口。 |

Mission Board 只投影 Mission 生命周期，不把 Provider、Runtime、Agent 和内部
WorkItem 全部罗列成同层卡片。Mission 标题是入口，Mission Room 是一次工作流
的完整现场。

## 第一次使用

1. 直接打开 `Loom.app`。App 会启动并管理内置 `loomd`，无需终端。
2. 在中间 Conversation 输入消息。
3. 需要其他执行路线时，在输入框附近选择 Harness、Provider Account、模型和
   推理强度。
4. 需要 Team 或治理执行时，选择 **Run as Mission**。
5. 检查 Agent Team 的逐 Agent 绑定，确认后启动。
6. 在 Mission Room 查看 Agent conversation、节点状态、结果和 Incident。
7. Mission 被阻塞时，在 **Continue this Mission** 输入指导并开启新的受审计
   Attempt。

没有可用路线时再进入 **Runtime & Providers**。目录中出现一个 Provider 仅代表
系统理解其配置合同，不代表本机已有凭证、兼容 Runtime 或通过真实请求验证。

## 文档地图

| 文档 | 内容 | 状态 |
|---|---|---|
| [根 README](../../README.md) | 产品心智模型、当前能力、安装和开发入口 | CURRENT |
| [原生 App 使用说明](NATIVE-APP-GUIDE.md) | macOS App 的真实交互表面 | CURRENT |
| [当前状态](../CURRENT.md) | 最新 Build、验收边界和延期项 | CURRENT |
| [Changelog](../../CHANGELOG.md) | 版本级用户影响和已知限制 | CURRENT |
| [能力矩阵](CAPABILITY-MATRIX.md) | 历史 WorkItem 到产品能力的追踪表 | HISTORICAL + CURRENT REFERENCES |
| [Runbook 全集](RUNBOOKS.md) | 开发和跨客户端验证入口 | DEVELOPER |
| [TUI 使用说明](TUI-GUIDE.md) | 终端客户端和运维验证 | DEVELOPER |

## Runtime、Provider 与 Route

三者不可混为一谈：

```text
Runtime / Harness: Codex, Claude Code, OpenCode, Loom Native, Pi
Provider Account: OpenAI, Anthropic, DeepSeek, Kimi, MiniMax, ...
Route: Harness + Provider Account + Credential Revision + Model + Limits
```

OpenCode 是 Runtime，不是 DeepSeek 或 MiniMax 的重复 Provider。用户可以看到
`OpenCode · DeepSeek` 这样的执行路线，但 Provider 管理页只应出现独立的
DeepSeek Account。

## 当前边界

`v0.5.3-rc.1` 包含 App-managed daemon、Conversation Route Segments、Context
Capsules、Loom Credential Vault、逐 Agent Execution Binding、Mission 实时过程、
阻塞干预、Incident diagnostics、显式 fallback、账户 accounting 和治理 UI。

以下三项已明确延期，不属于该 RC 的完成声明：

- 四个真实 Provider Account 同时组成一个安装版四 Agent Team；
- 撤销或限流一个真实账户后的完整外部故障隔离矩阵；
- 新导入自定义端点后完成一次真实 Conversation。

旧版本、历史 Evidence 和研究文档可能描述当时的手动 daemon 或旧导航方式。
它们是历史记录，不是当前用户说明。
