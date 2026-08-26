# awaiting_recovery 只读收敛 + 首页计数一致（2026-08-20）

## 问题
1. pre-fix 残留的 awaiting_recovery Mission（retry 时间已过但从未执行新
   attempt）永久显示 awaiting_recovery，且之前只读收敛只处理 running。
2. 首页 "0 active · 7 need you" 矛盾：attention 含历史/归档团队的项。

## 修复
- Go `reconciledMissionStatus`：base status 为 awaiting_recovery 且 running/
  awaiting_recovery 节点的当前 attempt Run 已终态、无未来 retry 时，反映终态
  （failed/cancelled）；board 节点同步。block-reason 也覆盖 awaiting_recovery。
- Swift `countActiveAttention`：首页 "need you" 只算可执行团队且 Mission
  未完成（active）的 attention，与 activeMissionCount 一致。

## 安装版 live（LoomBuild34）
- awaiting_recovery 3 个 → 全部 failed（provider_http）；missions 变为
  {failed:23, blocked:3, succeeded:3}。
- 首页：**0 active · 0 need you**（之前 6/7）；Local service ready。
- 环境彻底干净：无幻影 active Mission。

## 测试
- `testCountActiveAttentionExcludesHistoricalAndArchivedTeams`（1/3 计）。
- `TestLocalProductMissionBlockReasonSurfacesTerminalFailure` 扩展
  awaiting_recovery 分支。
- `go test ./internal/api/` 全绿；`swift build --build-tests` 0 error；
  `git diff --check` 干净。
