# OpenCode 角色在 App Team 构建器中的正确命名（2026-08-20 · LoomBuild47）

## 背景
上一提交让 OpenCode 成为可选的 Team 角色（目录发布 opencode 原生选项）。
App Team 构建器主/子代理角色菜单里该选项的 Provider 名显示为 "Opencode"
（`providerName` 的 default 分支把 "opencode" 首字母大写），应为 "OpenCode"。

## 修复（Swift，InteractionContinuity.swift）
- `providerName` 增加 `case "opencode": return "OpenCode"`（并去掉
  `fileprivate`，使纯展示函数可测）。
- 角色菜单项现为：
  "Coordinate bounded work and review with OpenCode · OpenCode ·
   opencode/deepseek-v4-flash-free"。

## 测试
- 新增 `testProviderNameUsesCanonicalDisplayNames`（opencode/deepseek/minimax
  三个规范名断言）。
- 全量 Swift XCTest：`swift test --filter LoomLocalAppTests` →
  **268 用例，0 失败（1 视觉导出按设计跳过）**。

## 安装版 live 验证（LoomBuild47 → /Users/lune/Applications/Loom.app）
- 打开 Team 构建器 → Main Agent role 菜单（截图 builder-role-menu.png）：
  - "Coordinate bounded work and review with DeepSeek · deepseek.primary · deepseek-chat"
  - "Coordinate bounded work and review with **OpenCode** · **OpenCode** · opencode/deepseek-v4-flash-free"

## 回归门
`swift build --build-tests` 0 error；`go test ./... -p 1` 全绿；
`git diff --check` 干净；App 干净重启（LoomBuild47）daemon healthy。
