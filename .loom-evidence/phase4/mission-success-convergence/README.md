# Mission 用户视角收敛：从永久 running → succeeded（2026-08-20）

## 结论
安装版 App（LoomBuild18，重建于 HEAD + 本轮修复）上，真实 Mission
**完整收敛到 `succeeded`**：subagent → 独立 Verifier 验收 → 主节点 →
独立 Verifier 验收 → `TeamExecutionTerminal`。不再永久 running。

## 修复链（本轮，RED-first，全部有单测）
1. **context-read 有界拒绝指引**（`internal/contextcapsule/delivery.go` +
   `internal/runtime/nativeadapter/deepseek_adapter.go`）：拒绝消息不再误导模型
   去调用它没有的 web_search / 换一个 item，而是明确
   "Answer directly using the objective and instructions already provided"。
2. **context-read 耗尽不终结失败**（`deepseek_adapter.go`）：模型在单步内连续
   context-read 到上限时，先注入 exhaustion directive 让其直接作答；若仍调用
   工具则以已产生的最佳内容干净收尾——不再返回终态 `context_retrieval_denied`。
3. **第二步 context-read 内联拒绝**（`deepseek_adapter.go`）：native adapter
   的 context-read 是 Exclusive（每步至多一个 dispatch），模型同一轮再发
   context-read 时改为内联返回同一有界拒绝（不产生新 dispatch/fact），
   模型可恢复作答；step 保持可 FINAL 终结（所有已 dispatch 的 call 均已投递），
   不再触发 StepEnd conflict 或残留 undelivered call。
4. **失败 step 可终结**（`cmd/loomd` 既有 `02d4c9c8` 延续）：任何失败 attempt
   都能写出 StepEnded/TurnEnded，Mission 从"永久 running"变为可恢复/可治理。

## 安装版 live 验证（2026-08-20，Mission Verify 5，DeepSeek）
- 新建 Team → preflight（2 node，deepseek-chat，context_retrieval）→ start。
- Journal 完整成功链：
  - subagent：AttemptLoopStarted → context-read admit/dispatch/accept/deliver
    （有界拒绝）→ StepEnded/TurnEnded → RunTerminalCommitted →
    WorkItemReadyForReview → verifier → WorkItemVerificationCommitted →
    WorkItemDone → **TeamNodeAcceptanceCommitted**。
  - main：同一路径 → WorkItemVerificationCommitted → WorkItemDone →
    **TeamNodeAcceptanceCommitted** → **TeamExecutionTerminal**。
- 快照：mission status = `succeeded`，`last_milestone = "Mission completed"`，
  completed_node_count = 2，active = 0；4 分钟轮询保持 succeeded。
- 时间线：02:10:28 建队 → 02:10:43 start → 02:10:55 subagent 验收 →
  02:11:09 main 验收 + TeamExecutionTerminal（约 26 秒收敛）。

## 之前的状态（未修复时）
- 01:27（旧二进制）：Mission 停在 ToolDispatchCommitted，无 StepEnded →
  running 永久卡死。
- 01:43（修复1-2 后）：Mission 到 blocked（有 StepEnded，失败隔离生效），
  但模型连环 context-read 触发 Exclusive 冲突 → context_retrieval_denied。
- 01:58（修复3 前）：2nd context-read 失败 → step 关闭为 failed → blocked。
- 02:11（修复3 后）：内联拒绝让模型直接作答 → **succeeded**。

## 测试
- `go test ./... -p 1` 全绿（serial）。
- `go test -race` nativeadapter/contextcapsule/cmd/loomd 全绿。
- `go vet` 相关包 0 告警；`gofmt`（改动文件）0；`git diff --check` 干净。
- `swift test`（apps/macos）通过（本机工具链只发现 15 条 Swift 用例；
  本轮改动为 Go-only，不影响 Swift）。

## 已知边界（诚实）
- 环境仍含约 22 个历史卡死 Mission（journal append-only，无法删除）；
  它们显示 running/awaiting_recovery 占用 Mission Board，但不阻断 daemon 与
  新 Mission（daemon 韧性修复）。彻底清理需要后续治理路径。
- Swift 254 条全量在本机 `swift test` 不可复现（仅 15 条被发现），为环境
  工具链限制，非本轮回归。
