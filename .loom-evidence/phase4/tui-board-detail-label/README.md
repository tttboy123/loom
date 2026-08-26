# TUI Board 状态文案：与 App 对齐，去掉 "Complete · Failed"（2026-08-20）

## 用户视角问题
App 端 Board 已改为人性化结果（"Failed"），但 TUI（`loom app`）的 Board
仍显示 "UI Mission Team · Complete · Failed" —— 同一产品两个界面对同一
Mission 呈现矛盾文案。

## 根因
`internal/tui/model.go` ScreenBoard 行渲染 `title · Lane · Status`，
未做 Complete 车道的结果优先处理。

## 修复（Go，internal/tui/model.go）
- 新增 `missionBoardRowStatus(lane, status)`：与 App 端
  `missionBoardDetailText` 同语义 —— lane == "Complete" 只显示人性化结果
  （Failed/Blocked/Succeeded）；活跃车道显示 "Orchestrating · Running"。
- ScreenBoard 行改为 `title · <rowStatus>`（不再并列 lane+status）。

## 测试
- 新增 `TestMissionBoardRowStatusLeadsWithHumanizedOutcome`（7 条断言）。
- `go test ./internal/tui/` 全绿；`go test ./... -p 1` 全绿。

## TUI live 验证（PTY 连接安装版 daemon）
- Board 屏实测输出：
  - `UI Mission Team · Failed`（原 Complete · Failed）
  - `Mission Verify 4 · Orchestrating · Blocked`（活跃车道保留）
  - `Clean Accept · Succeeded`
- 与 App 端（LoomBuild36+）口径完全一致。
