# 首页快速开始清单：去掉孤立的 "1." 编号（2026-08-20）

## 用户视角问题
首页快速开始清单在"未选文件夹但 Team 已就绪"时显示
"1. Open Folder so work has a home" + "Agent Team ready" —— 只有孤立的
"1."，没有 "2."，新用户会以为清单不完整或顺序错乱。

## 根因
`quickStartGuide` 用 `"1. "` / `"2. "` 前缀硬编码在 pending 文案里；
当某一项已完成（Team 已就绪）时它换成 "Agent Team ready"（无编号），
编号只在未完成项上残留。

## 修复（Swift，LoomWorkspaceShell.swift）
- 新增纯函数 `quickStartStepLabel(ready:value:done:pending:)`：
  pending 一律是无编号祈使句；ready 显示完成文案，`%@` 可代入值
  （"Folder: X"）。
- `quickStartGuide` 两行改用该函数，去掉 "1." / "2." 前缀。
- 图标不变：未完成 folder/person.3，已完成 checkmark.circle.fill。

## 测试
- 新增 `testQuickStartStepLabelKeepsConsistentChecklistWording`（3 条断言）。
- 全量 Swift XCTest：`swift test --filter LoomLocalAppTests` →
  **267 用例，0 失败（1 视觉导出按设计跳过）**。

## 安装版 live 验证（LoomBuild38 → /Users/lune/Applications/Loom.app）
- 新建空会话（触发 quick start），OCR（截图 quickstart-after.png）：
  - "• Chat is ready with DeepSeek • deepseek.primary"
  - "• Open Folder so work has a home"（无孤立编号）
  - "• Agent Team ready"

## 回归门
`swift build --build-tests` 0 error；`go test ./... -p 1` 全绿；
`git diff --check` 干净；App 干净重启（LoomBuild38）daemon healthy。
