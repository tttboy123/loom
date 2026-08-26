# Mission Board "Hide completed" 开关（2026-08-20）

## 问题
Mission Board 固定显示 5 条横向车道；历史任务多时（本轮环境 Complete 车道
22 条）横向滚动拥挤，用户难以聚焦活跃工作。

## 修复（Swift）
- `MissionWorkspaceState` 新增 `boardHideCompleted`（+`updateBoardHideCompleted`），
  store 暴露 `updateMissionBoardHideCompleted`。
- Board 头部新增 "Hide completed" 复选框（紧邻 Filter Missions）：勾选后
  跳过 Complete 车道，聚焦 Proposed/Ready/Orchestrating/Review。

## 验证
- 单元测试 `testMissionWorkspaceBoardHideCompletedTogglesLane`（toggle 状态往返）。
- `swift build --build-tests` 0 error；`go build ./...` OK；
  `git diff --check` 干净；安装版（LoomBuild26）构建并安装成功。
- 注：盲 UI 自动化无法可靠点击导航到 Board 页面，UI 布局为代码审查确认；
  逻辑由 Core 层单测覆盖。

## 说明
此开关是"显示偏好"，不改变任何权威状态（board 状态仍来自投影快照）。
