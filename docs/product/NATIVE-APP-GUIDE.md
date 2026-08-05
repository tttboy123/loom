# Loom 原生 app 使用说明（macOS）

原生 app（`Loom.app`，SwiftUI）与 TUI 共享同一 daemon、同一 Journal 投影：
它是产品的图形观察/操作面，走生产 Swift 客户端（`LocalIPCClient` /
`LocalProductStore`）经真实 socket 通信。

## 启动

```sh
# 显式连接某个 daemon socket（推荐，产品旅程用法）
open /path/to/Loom.app --args --socket /path/to/loomd.sock --journey-id <uuid>

# 或直接打开，使用默认 socket：~/Library/Application Support/Loom/run/loomd.sock
open /path/to/Loom.app
```

- `--socket`：真实 daemon 的 Unix socket 路径。
- `--journey-id`：旅程关联 UUID（仅关联，不是权威/代际；不带则普通使用）。
- app 启动时会在 IPC 日志留下 `loom-swift-*` 启动读（连接证明）。
- 连接不可用时 app 显示不可用状态（offline），不会伪造数据。

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

## 与 TUI 的关系

- 同一 daemon：TUI 与 app 可同时连接并观察同一状态（跨客户端旅程即
  用此验证）。
- 同一投影：app 不持有第二套 Scheduler/Journal/StateWriter/Projection。
- 操作约束：评审、集成、队列等权威边界与 TUI 一致（评审只读、单写
  集成、显式人工触发）。

## 局限

- app 是本地单用户产品：无 Web/共享/多用户。
- Provider 连接只做受控凭证管理与可用性显示；不执行 Provider 路由或
  自动激活。
- 默认窗口尺寸 1100×720（最小 900×580），不支持命令行环境之外的主题化。
