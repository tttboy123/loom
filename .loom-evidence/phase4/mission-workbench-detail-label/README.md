# Mission 工作台工具栏：消除 "Complete · Failed"（2026-08-20 · LoomBuild39）

## 用户视角问题
打开一个已失败的历史 Mission 时，工作台工具栏副标题仍显示
"Complete · Failed"（自相矛盾），而 Board 面板与 TUI 已修复为 "Failed"。

## 根因
`MissionWorkbench.swift` 工具栏副标题用 `"\($0.lane) · \(humanStatus($0.status))"`
拼接，未复用 `missionBoardDetailText` 的结果优先逻辑。

## 修复（Swift，MissionWorkbench.swift）
- 工具栏副标题改用 `missionBoardDetailText(lane:status:)`（Complete 车道只
  显示 Failed/Blocked/Succeeded；活跃车道显示 "Lane · State"）。
- Mission 卡片 accessibilityLabel 同步改为
  `missionBoardDetailText`（"UI Mission Team, Failed"）。

## 测试
`missionBoardDetailText` 已有单测覆盖（7 条断言）；本改动为纯复用调用。
全量 Swift XCTest：`swift test --filter LoomLocalAppTests` →
**267 用例，0 失败（1 视觉导出按设计跳过）**。

## 安装版 live 验证（LoomBuild39 → /Users/lune/Applications/Loom.app）
- 打开 "UI Mission Team" 的已失败 Mission 工作台：AX 树工具栏副标题为
  **"Failed"**（原 "Complete · Failed"）；outcome 区 "Runtime Process Failed"
  正常（截图 workbench-after.png）。

## 回归门
`swift build --build-tests` 0 error；`go test ./... -p 1` 全绿；
`git diff --check` 干净；App 干净重启（LoomBuild39）daemon healthy。
