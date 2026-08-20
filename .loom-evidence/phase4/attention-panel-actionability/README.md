# Attention 面板：可操作项与历史分离 + 文案人性化（2026-08-20）

## 用户视角问题
"Items needing attention" 面板把所有 attention 原样列出，但其中大量条目
来自已归档团队或 Mission 已 Complete 的历史团队——首页 "need you" 已经是
0，面板却显示 7 条 "inspect_failure · Critical"，用户看到互相矛盾的数字，
且条目标题是原始 code（inspect_failure），不是人话。

## 根因
`attentionPanel` 直接遍历 `snapshot.attention`，标题用
`SafeText.sanitize(item.actionRequired)`（原始 code），没有任何可操作性
过滤；"needs you" 计数（`countActiveAttention`）却只算可执行团队 + 非
Complete 团队，两处口径不一致。

## 修复（Swift，LoomWorkspaceShell.swift）
- 新增纯函数：
  - `actionableAttention(teams:missions:attention:)` —— 可操作项（口径与
    `countActiveAttention` 一致）；
  - `historicalAttention(...)` —— 其余为历史（已归档 / Mission 已
    Complete）；
  - `attentionActionTitle(actionRequired:kind:)` —— 人话标题
    （"inspect_failure" → "Inspect failure"，空则回退 kind，再回退
    "Needs your attention"）。
- `attentionPanel` 重构：可操作项优先展示；0 项时显示
  "Nothing needs your attention"；历史单独一节
  "History (N) — archived teams or completed Missions"，置灰 0.62，
  超过 6 条显示 "+N more in Mission history"。
- `countActiveAttention` 复用 `actionableAttention(...).count`，口径单一来源。

## 测试
- 新增 `testActionableAttentionSplitsActionableFromHistory`：exec 活跃团队
  a1 进可操作、archived 团队 a2 进历史；标题人性化三档断言。
- 全量 Swift XCTest：`swift test --filter LoomLocalAppTests` →
  **266 用例，0 失败（1 视觉导出按设计跳过）**。

## 安装版 live 验证（LoomBuild37 → /Users/lune/Applications/Loom.app）
- AX/OCR（截图 attention-panel-after.png）：
  - "Nothing needs your attention"（可操作 = 0，与首页 need you = 0 一致）
  - "History (7) — archived teams or completed Missions"
  - 行标题 "Inspect failure · Critical"（人话化），"+1 more in Mission history"

## 回归门
`swift build --build-tests` 0 error；`go test ./... -p 1` 全绿；
`git diff --check` 干净；App 干净重启（LoomBuild37）daemon healthy。
