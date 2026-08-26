# Board 状态文案：消除 "Complete · failed" 自相矛盾（2026-08-20）

## 用户视角问题
Mission Board 治理面板里，已终结（历史）Mission 显示为 "Complete · failed"
（如 "UI Mission Team, Complete · failed"）。"Complete" 车道 + "failed"
状态并排读起来自相矛盾，用户分不清是完成了还是失败了。

## 根因
`boardPanel` 直接拼接 `"\(mission.lane) · \(mission.status)"`。对已达
Complete 车道的 Mission，原始 status 是 `failed`/`blocked`/`succeeded`，
于是渲染成 "Complete · failed"。

## 修复（Swift，LoomWorkspaceShell.swift）
- 新增纯函数 `missionBoardDetailText(lane:status:)`：
  - lane == "Complete" → 只显示人性化结果（"Failed" / "Blocked" /
    "Succeeded"），不再并列矛盾的车道词；
  - 活跃车道 → "Orchestrating · Running" / "Review · Blocked"；
  - 空值兜底不产生多余分隔符。
- `boardPanel` 改用该函数。

## 测试
- 新增 `testMissionBoardDetailTextIsNotContradictory`（7 条断言）。
- 顺带修复两个潜伏失败：
  1. `countActiveAttention` 测试的 attention fixture 缺 `schema_version`
     （strict decoder 要求该字段）→ 补齐。
  2. 该测试期望值 1 与当前语义（按可执行团队 + 非 Complete 团队计数，
     attention 不带 mission_id）不符；重写 fixture 为
     exec(活跃)/historical(仅 Complete)/archived(不可执行) 三团队，
     真实覆盖 "historical + archived 排除"，期望 1。
- 全量 Swift XCTest 套件：`swift test --filter LoomLocalAppTests` →
  **265 个用例，0 失败（1 个视觉导出按设计跳过）**。

## 安装版 live 验证（LoomBuild36 → /Users/lune/Applications/Loom.app）
- AX 树：Board 面板行标签从 "UI Mission Team, Complete · failed" 变为
  **"UI Mission Team, Failed"**；全部历史团队一致（截图 board-inspector-after.png）。
- OCR 确认面板显示团队名 + "Failed"，无 "Complete · failed"。

## 回归门
`swift build --build-tests` 0 error；`go test ./... -p 1` 全绿；
`git diff --check` 干净；App 干净重启（LoomBuild36）daemon healthy。
