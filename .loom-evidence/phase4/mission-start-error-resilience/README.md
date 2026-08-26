# Mission Start: error resilience + daemon brick fix（用户视角 live 验证）

日期：2026-08-19 · 分支 codex/loom-platform-slice2 · commit 473995d4

## 背景（用户视角复验发现的两个真实缺陷）

用安装版 App 真实走"New Mission → 填目标 → 确认 → Review preflight →
Start Mission"全链时发现：

1. **Start Mission 失败提示不可读**：Team 已在运行另一 Mission 时，daemon
   返回的具体错误（busy/conflict）被 Swift 客户端折叠成
   `Mission could not proceed: unavailable`，用户完全不知道原因。
2. **单个不可恢复 Mission 会砖掉整个 daemon**：早前 IPC 测试产生的
   "running" 但 run 已 terminal（runtime_process_failed）的 Mission，
   重启后 `ResumeProjectedMissions` 因 binding/plan 漂移无法重建 → 整个
   daemon 起不来 → App 永远停在 "Local service unavailable / Try Again"。

## 修复（RED-first，全部有测试）

- `LocalProductStore.closedClientReason`：新增对 `LocalIPCRemoteError` 的映射
  （返回 daemon 真实 code，如 busy/conflict），不再全部折叠为 unavailable。
  新增 2 个 store 测试（remote error code 保留；未知传输错误仍回落 unavailable）。
- `MissionWorkbench`：`.failed(reason:)` 按已知 code 渲染可操作文案，并在
  busy/conflict 时提供 **Open Mission Board** 恢复按钮。
- `ResumeProjectedMissions`：单个投影 Mission 无法重建时跳过（不再
  return ErrMissionExecutionConflict 砖掉启动），其余产品正常服务。
  更新既有 unknown status 断言 + 新增 unresumable 跳过测试。

## 安装版 live 验证（真实 socket + 真实 UI）

- daemon 在存在卡死 Mission 的 DB 上正常启动（loomd.sock 出现，
  "Local service ready"）。
- 用户视角：欢迎页 Start Mission → New Mission sheet（Team 已预选）→
  填目标 → 勾选确认 → Review preflight → 2 Agent ready →
  点 Start Mission → 显示：
  `The Mission state changed (for example this Team is already running).
  Review preflight, then retry.`
  并出现 **Review preflight** + **Open Mission Board** 两个按钮。
- 点 Open Mission Board → sheet 关闭 → Mission Board 打开，显示
  UI Mission Team（Orchestrating / Running / Attempt 0 · 0/2）。

## 截图
- 01-actionable-busy-error.png：可读错误 + Open Mission Board 按钮
- 02-open-mission-board.png：Mission Board 打开并显示运行中 Mission
- 03-new-mission-sheet-preselected-team.png：New Mission sheet 预选 Team

## 测试
- Swift：254 tests（1 视觉导出按设计跳过）全绿。
- Go：internal/app、internal/work、internal/supervisor、
  internal/runtime/nativeadapter、internal/toolbroker 全绿；
  go vet + go build ./... + gofmt + git diff --check 干净。

## 已知边界（诚实）
- 被跳过的卡死 Mission 在 Mission Board 上仍显示 running（Team 被占用），
  不阻断 daemon/其它功能；彻底终结/清理该状态需后续治理路径。
