# Loom 原生 App 使用说明（macOS）

状态：`CURRENT / v0.5.3-rc.1 / BUILD 188`

Loom 的默认体验是 Conversation-first。原生 App、内置 daemon、Credential
Vault、Mission 状态和治理投影组成一个产品，不要求用户分别启动或理解后台组件。

## 启动

```bash
open "$HOME/Applications/Loom.app"
```

- `Loom.app` 内含匹配版本的 `loomd`。
- App 启动时自动拉起服务并等待私有 Socket 健康。
- 本地 ad-hoc 包使用随 App 生命周期存在的受管 helper；Developer ID 包可使用
  macOS 后台服务生命周期。
- 用户不需要手动运行 `loomd`、传入 `--socket` 或打开终端。
- 服务启动失败时，App 显示具体阶段、恢复动作和 Incident ID，而不是笼统的
  “offline”。

## 界面结构

主窗口由左侧导航、中间 Conversation 和可开合的右侧治理区组成。

### 左侧导航

| 入口 | 作用 |
|---|---|
| **New Mission** | 从当前 Conversation 的目标开始一次受治理工作流。 |
| **Chat** | 返回 Conversation。 |
| **Work** | 查看 Mission Board 和 Mission Room。 |
| **Teams** | 创建、编辑、归档和恢复 Agent Team。 |
| **Roundtable** | 打开受治理协商与交接工作台。 |
| **Attention** | 查看真正需要用户处理的活跃事项。 |
| **Library** | 查看可治理资产。 |
| **Runtimes** | 查看 Runtime、Provider Account、Vault 和诊断。 |

导航下方分别列出 Conversations 和可执行 Agent Teams。归档 Team 保留在治理页，
不会继续作为可执行快捷入口。

### 中间 Conversation

中间区域是默认主体验：

- 顶部切换、新建或复制 Conversation；
- 时间线显示用户与 Loom 的对话；
- 输入框附近选择执行 Route、模型和推理强度；
- 发送后按钮变为 Stop，可停止当前 Response；
- 提案可以通过 **Run as Mission** 升级为 Mission。

切换 Provider、账号、模型、Harness 或凭证版本时，Loom 创建新的 Conversation
Segment。若信任域或披露范围变化，用户需要选择 Continue with context、Summary
only 或 Start clean，并确认必要的信任边界。

### 右侧治理区

右侧用于查看与当前 Conversation 或 Mission 有关的治理信息，不取代对话本身：

- 关联 Mission 和 Agent Team；
- Context Capsule 容量、遗漏和 disclosure receipt；
- Provider Account、模型、凭证版本和 Route 状态；
- Evidence、accounting、Incident 和恢复动作。

## Mission 使用方式

Mission 是一个标题对应的一次工作流。它可以来源于 Conversation，也可以通过
New Mission 直接创建。

1. 输入标题和目标，选择或创建 Team。
2. 检查每个 Agent 的 Harness、Provider Account、模型、预算和超时。
3. Review preflight 后显式启动。
4. 在 Mission Room 查看每个 Agent 的状态和可见过程输出。
5. 查看结果、Evidence、accounting 和失败原因。

Mission Room 的 **Agent conversation** 在工作进行时显示有界增量。内容在终态
Evidence 接受前标记为 tentative；Prompt、密钥、Provider 原始响应和隐藏推理不会
进入该流。

### Mission 被阻塞

Blocked、Failed 或 Cancelled Mission 显示 **Continue this Mission**：

1. 输入对下一次执行的指导。
2. 打开预填的 Continue Mission review。
3. 再次确认上下文、Team 和 preflight。
4. 显式启动新的 Attempt。

继续执行不会篡改旧 Attempt。旧输出、失败、Incident 和 Evidence 保持可追溯。

## Agent Teams

Team Builder 中每个 Agent 行独立显示并编辑：

- Harness；
- Provider Account；
- Model；
- 推理强度；
- 预算与超时；
- 显式 fallback。

Team 顶层默认值只用于新 Agent 的初始值，不覆盖已有独立绑定。一个账户失败只
阻塞依赖该账户的 Agent。

## Runtime & Providers

该页面将三个维度分开：

- **Runtime**：本机检测到的 Codex、Claude Code、OpenCode、Loom Native 或 Pi；
- **Provider Account**：Provider endpoint、Credential Vault reference 和状态；
- **Route**：实际可执行的 Runtime + Account + Model 组合。

因此 `OpenCode · DeepSeek` 是 Route，不是名为 `opencode-deepseek` 的 Provider。
Provider 目录中的项目也不等于已连接；以 Account、Vault 和 Route 的实际状态为准。

## RoundTable

RoundTable 用于需要明确参与者、轮次和交接确认的协作。用户可以把可用 Agent
拖入席位，启动轮次，查看 propose、relay、acknowledge、insert/drop 和 conclude
过程。结论发布为 digest-bound `AlignmentSummary` Evidence。

普通聊天和普通 Mission 不需要先创建 RoundTable。

## 故障处理

错误面显示：

- 非秘密操作阶段；
- 是否可以重试；
- 建议恢复动作；
- Incident ID；
- Retry、View diagnostics 或 Copy incident ID。

Vault locked、Provider rejected、rate limit、transport timeout 和 profile conflict
拥有不同恢复路径，不会统一显示为 `Unavailable`。

## 当前限制

- 本地单用户产品，无云同步和多人共享。
- Provider 目录比当前安装环境的真实可执行 Route 更广。
- Tentative Agent 增量在重启后不保证重建；已接受的终态 Evidence 会保留。
- 四真实 Provider Team、真实账户 revoke/rate-limit 矩阵和自定义端点真实对话已
  延期到后续 Phase。

开发构建、安装和完整验证命令见[根 README](../../README.md)。精确验收状态见
[CURRENT](../CURRENT.md)。
