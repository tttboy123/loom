# TUI Attention/Overview：与 App 口径统一（2026-08-20）

## 用户视角问题
App 端已把 Attention 拆成"可操作 + 历史"并人话化标题，但 TUI 仍是：
- Overview "Needs you · 7"（把全部 attention 算作需要处理，实际可操作 = 0）；
- Attention 屏列原始 "inspect_failure"，且全部混在一起。

## 根因
`internal/tui/model.go` Overview 用 `len(model.snapshot.Attention)`；
Attention 屏原样输出 `ActionRequired`，无可操作性过滤。

## 修复（Go，internal/tui/model.go）
- 新增 `actionableAttentionItems` / `historicalAttentionItems` /
  `countActionableAttention`（与 App `actionableAttention` /
  `countActiveAttention` 同语义：可执行团队 + 非 Complete Mission）。
- 新增 `attentionActionTitle(actionRequired, kind)`（与 App 同文案逻辑）。
- Overview "Needs you · N" 改用 `countActionableAttention`。
- Attention 屏：可操作项优先；0 可操作时显示
  "Nothing needs you · history (N)"；有可操作项时另起
  "History (N) · archived teams or completed Missions" 小节；标题人话化。

## 测试
- 新增 `TestCountActionableAttentionMatchesAppSemantics`（exec 活跃 1 /
  historical / archived 排除）。
- `TestAttentionActionTitleHumanizesRawCodes`（action_required / kind /
  兜底三档）。
- `TestMissionBoardRowStatusLeadsWithHumanizedOutcome`（沿用上轮 Board 修复）。
- 更新 `TestModelRendersEveryBoundedScreen...`：fixture 的 attention 位于
  不可执行 + Complete 团队 → 断言 Attention 屏渲染 "history" 小节。
- `go test ./internal/tui/` 全绿；`gofmt` 干净。

## TUI live 验证（PTY 连接安装版 daemon）
- Governance Overview："Active missions · 3 / **Needs you · 0** / Teams · 30 /
  Runtimes · 3"（此前 Needs you = 7）。
- Attention 屏：
  ```
  Needs your attention
  Nothing needs you · history (7)
  • Inspect failure ×7
  ```
- 与 App（LoomBuild37+）口径、文案完全一致。

## 回归门
`go test ./... -p 1` 全绿；`gofmt -l` 干净；`git diff --check` 干净。
