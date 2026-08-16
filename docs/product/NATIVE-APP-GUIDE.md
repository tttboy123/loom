# Loom 原生 app 使用说明（macOS）

原生 app（`Loom.app`，SwiftUI）与 TUI 共享同一 daemon、同一 Journal 投影：
它是产品的图形观察/操作面，走生产 Swift 客户端（`LocalIPCClient` /
`LocalProductStore`）经真实 socket 通信。

## 启动

```sh
# 正常使用：直接打开即可
open /path/to/Loom.app
```

- `Loom.app` 内含匹配版本的 `loomd`。App 启动时先通过 macOS
  `SMAppService` 注册并启动服务；本地 ad-hoc 签名包无法注册后台项目时，
  使用仅随当前 App 生命周期存在的同包 helper。
- 两种路径都使用默认 socket：
  `~/Library/Application Support/Loom/run/loomd.sock`。
- App 会等待真实 socket 并做有界重连。只有服务确实无法建立时才显示
  unavailable；注册 API 返回成功不等于服务健康。
- 用户不需要单独安装、启动或理解 `loomd`，也不需要打开终端。
- 若本机 Codex 已登录，Loom 会自动将其作为普通对话的受控 responder；
  用户不需要先打开 Runtime & Providers。未登录时才需要显式 Connect。
- Developer ID 签名公开版由 `launchd` 管理常驻生命周期；本地 ad-hoc
  版关闭 App 后 helper 会自动结束。

## 主界面

左侧导航栏（rail）+ 主内容区（`HSplitView`）。

### 导航（rail）

| 项目 | 作用 |
|---|---|
| New Mission | 打开新建 Mission 面板 |
| My Missions | 任务看板（orchestration board） |
| Teams | 团队工作区 |
| Needs You | 注意力工作区（待你处理的可执行事实） |
| Library | 演化资产库（Evolution Assets） |
| Runtime & Providers | 运行时与 Provider 连接管理 |
| Roundtable | 受治理交接账本：双席位旅程与 AlignmentSummary |

底部连接脚注显示 daemon/投影状态（online/partial/preserved/unavailable
等），语义与投影保持一致。

### 主内容区

- **Orchestration Board**：Mission 列表与状态。
- **Mission Room**：目标 + 已确认 Team + Work Package；新建侧任务
  （purpose/mode/title/request → Review proposal → Confirm and run）；
  决策卡（decision + 可用决策 + CTA，含 Review Gate）；取消 Mission。
- **Team Builder**：一次一问题 + 精确预览 + 确认（Candidate 边界）。
- **Teams Workspace / Attention Workspace / Library Workspace**：团队、
  注意力与演化资产。

### Mission 检查器（Inspector）

右侧检查器四个页签，全部来自 Journal 权威投影：

- **Team Pulse**：各角色状态与 Attempt 编号；
- **Plan**：逻辑节点拓扑与状态；
- **Changes**：ready_for_review / verification / acceptance 记录；
- **Evidence**：Evidence 引用列表（完整权威活动加载后才显示）。

### Roundtable（受治理交接）

- 左侧导航 **Roundtable** 打开受治理交接工作台：
  1. 填会话 ID 与标题，**Create session**（Moderator 席位自动创建）；
  2. **Run full journey** 或逐步执行：加 Writer/Target 席位 → 开轮 →
     propose（writer）→ relay（moderator）→ acknowledge（target）→
     insert（moderator）→ conclude（moderator）；
  3. 会话状态实时展示席位、轮次、消息状态与 body digest；
- 结论生成 digest-bound `AlignmentSummary` 工件；会话一旦 concluded，
  后续写入会被拒绝（状态里显示 Concluded）。
- 模型层仍然按 Provider → Model → 推理强度三层选择，凭据未验证的模型
  会被禁用并给出可操作提示。

## 与 TUI 的关系

- 同一 daemon：TUI 与 app 可同时连接并观察同一状态（跨客户端旅程即
  用此验证）。
- 同一投影：app 不持有第二套 Scheduler/Journal/StateWriter/Projection。
- 操作约束：评审、集成、队列等权威边界与 TUI 一致（评审只读、单写
  集成、显式人工触发）。

## 局限

- app 是本地单用户产品：无 Web/共享/多用户。
- 普通 Codex 对话使用临时、只读、非权威路径；Agent Team 的 Provider
  路由、写操作与执行仍需显式治理和授权，不会由聊天自动激活。
- 默认窗口尺寸 1100×720（最小 900×580），不支持命令行环境之外的主题化。
