# B-P1 Execution Permission Pipeline — Progress

Updated: 2026-08-05（Amendment 3 后）

## Status

- Gate 0：PASS（GATE0-AUDIT.md）。
- Gate 1：FROZEN（FREEZE-RECORD.md；外部 fresh Contract Review 为开放项，
  通道故障记录于 REVIEW-NOTES.md）。
- RED-first：RED 1-20 全绿（18/19 由 Amendment 1 增加，20 由 Amendment 3
  增加）；`internal/permissions` 单元测试、app/daemon/localipc/rules 集成
  测试全 PASS。
- A4/A5/A6（Amendment 2）与 A7-A13（Amendment 3）均已实施并验证。
- 全仓 `go build ./...`、`go vet ./...`、`go test ./...` 全绿；
  `swift build` + `swift test`（98 tests + 4 Swift Testing）全绿。

## 已完成清单

- [x] `internal/permissions/` 五文件（model/compile/evaluate/replay/authority）
  + 三个测试文件（RED 1-20 覆盖）。
- [x] `internal/app/local_permission.go` + `internal/api/local_permission.go`
  （snapshot/attention 只读 + 命令路由 + validate_call + resolve_approval）。
- [x] `internal/rules` 批准生命周期接线（A4/A5/A6）：
  `RequestPermissionApproval`/`DecidePermissionApproval` +
  `PermissionApprovalProjectID` 跨流隔离 + 跨会话激活复用。
- [x] daemon 挂载与 IPC 白名单（`cmd/loomd/product_daemon.go`、
  `internal/localipc/protocol.go`、`cmd/loomd/permission_approvals.go`）。
- [x] TUI：`ScreenPermissions` Explorer + Attention 合并（a/x/g/v）。
- [x] Swift 原生 app：只读 snapshot/attention（含 approvals）、
  `PermissionApprovalView`、probe 输出；真实 IPC 契约测试
  `TestBp1SwiftProbeDecodesRealPermissionAttentionWithApprovals`。
- [x] `scripts/verify-bp1-cross-client-journey.sh` 冻结验证门。
- [x] 跨客户端旅程证据：`/private/tmp/bp1-journey-final13`
  （journey_id `7266f17f-…`，含真实批准段）verify PASS；
  final8/final9/final10/final11/final12 为阶段证据。
- [x] B/C 阻塞点评估：`B-C-BLOCKER-ASSESSMENT.md`。

## 确定性矩阵（当前全绿）

- `go build ./...`、`go vet ./...`、`go test ./...` 全 PASS。
- `internal/permissions` RED 1-20 全绿（含 Amendment 3 新增四项）。
- `internal/localipc` 新增 Swift 真实 IPC 契约测试 PASS。
- `swift build` + `swift test`（98 tests，1 skipped，0 failure）+
  4 Swift Testing PASS。
- `verify-bp1-cross-client-journey.sh /private/tmp/bp1-journey-final13` PASS。

## 实现期发现并修复的缺陷

1. 共享 Journal 非权限域事件曾导致 `Replay` 报错 → 只处理权限流，权限流内
   未知类型仍 fail-closed（RED #10/#15）。
2. TUI Attention 合并决策后渲染计数错误 → 修正。
3. Amendment 1：allow 放行危险链式命令（P0-1）、决策事实缺 call、作用域
   缺失（P1-2/3）、grant 载荷/模板等 P2。
4. Amendment 2：跨流批准泄漏（A5）、跨会话激活冲突（A6）。
5. Amendment 3：allow 放行非危险链式命令（A7 P0）、Swift attention 缺
   approvals + approval_id 解码失败（A8 P1）、EffectiveMode 非确定（A9 P1）、
   RuleIDs 非确定（A10）、激活/管理员锁流校验缺失（A11）。

## 开放项

- fresh 独立 Contract/Implementation 复评：子代理协作通道本机不可用
  （CC Switch 仅 deepseek-v4-flash 可用且消息载荷持续为空）；flash 冷读
  复评（v9 FAIL 基线 + v14 本轮）作为替代评审通道，通道恢复后补做。
- 新旅程 final14（原生 app 读取 pending 批准）推荐补充证据；等价路径已由
  真实 IPC 契约测试覆盖。
- decision 事实 `approval_id` 直接关联（A12）与间接危险调用沙箱（A13）：
  留待 B-W1 执行适配器落地。
