# Mission Board 展示 blocked 原因（2026-08-20）

## 用户问题
"用户不知道错误原因是什么"。Mission 现在能干净收敛到 blocked（失败隔离），
但 Mission 卡片只显示 "Blocked" / milestone "blocked"，用户看不到为什么
（例如 context_retrieval_denied / provider_http），要点击进 inspector 才看得到。

## 修复（Go + Swift，strict-IPC 兼容）
- `internal/api/local_product_mission.go`：`LocalProductMissionSummary` 新增
  `block_reason`（omitempty），由 `localProductMissionBlockReason` 推导：
  优先 blocked 节点的 InitialBlockReason → 当前 attempt 的 TerminalReason →
  任一 attempt 的 TerminalReason。
- Swift `LocalProductMissionSummary` 新增 `blockReason`（decodeIfPresent +
  rejectUnknownKeys allowlist 加入 `block_reason`，避免严格解码拒收新字段）。
- `MissionWorkbench.missionCard`：blocked 任务卡片在状态下方显示
  `⚠ Block reason`（人类化文案，如 "Context Retrieval Denied"），
  accessibility label 同步。

## 安装版 live 验证
- 安装版 daemon snapshot 的 blocked 任务现在携带
  `block_reason`：context_retrieval_denied（×2）/ provider_http。
- App 正常渲染（snapshot 解码无报错，首页正常显示）。

## 测试
- Go 新增 `TestLocalProductMissionBlockReasonSurfacesTerminalFailure`：
  优先 InitialBlockReason、回退 attempt TerminalReason、无 blocked 为空。
- Swift 新增 `testMissionSummaryDecodesBlockReason`（strict allowlist 含
  block_reason，缺失时 nil）。
- `go test ./... -p 1` 全绿；`go test -race ./internal/api/` 绿；
  `swift build --build-tests` 0 error；`gofmt`/`git diff --check` 干净。
