# 幻影 running Mission 只读收敛（2026-08-20）

## 问题
20 个 Mission 在投影里永久显示 "running"，但它们的 Run 其实全部已终态
（RunTerminalCommitted failed/succeeded），`TeamNodeAttemptTerminal` 事件
缺失（如 daemon 在 Run 终态提交与协调器记录之间被中断）。首页显示
"26 active missions"，Board 的 Orchestrating 车道被占满，且这些 Mission
永远不会自己结束。

## 修复（只读投影层，不写 journal）
- `reconciledMissionStatus(view, execution)`：当投影状态为 running 时，若每个
  running 节点的当前 attempt Run 已终态（failed/cancelled）且无重试调度，
  反映真实终态；succeeded-run 但无协调器记录时保守保持 running（验收链未完）。
- `localProductMissionBlockReason` 增加 view 回退：从 Run.TerminalReason 取原因。
- Board 节点与 board.status 同步收敛（`buildTeamBoard`）。

## 安装版 live 验证（LoomBuild24）
- 首页 active：26 → **6**（3 blocked + 3 awaiting_recovery，均需用户关注）。
- 20 个幻影 → 全部 **failed / Complete 车道**，原因 runtime_process_failed。
- Board：status failed，节点 failed + runtime_process_failed（原 running / 空原因）。
- 重启一致性：重启前后状态一致。

## 测试
- `TestLocalProductMissionBlockReasonSurfacesTerminalFailure` 更新为新契约
  （running 节点 + 终态 Run → 显示终态原因）。
- `go test ./... -p 1` 全绿；`gofmt`/`git diff --check` 干净；
  `swift build --build-tests` 0 error。
