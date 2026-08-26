# Mission Room Outcome 显示 blocked/failed 原因与下一步（2026-08-20）

## 问题
Mission 卡片已显示 blocked 原因，但打开 Mission Room 后 Outcome 只显示
milestone（如 "blocked"），用户仍看不到"为什么 + 接下来做什么"。

## 修复（Swift）
- `missionOutcomeBlock`：Outcome 卡片在 milestone 下，对 blocked/failed 任务
  显示 ⚠ 具体原因（如 Context Retrieval Denied）+ 下一步提示
  （"This Mission cannot proceed on its own. Open the Inspector for
  node-level failure details, or start a new Mission with the Team."）。

## 验证
- `swift build --build-tests` 0 error；`go build ./...` OK；
  `git diff --check` 干净；安装版（LoomBuild29）构建并安装成功。
- 数据来源为已收敛的 `block_reason`（20 个幻影 failed 任务均带
  runtime_process_failed，3 个 blocked 带 context_retrieval_denied/provider_http）。
