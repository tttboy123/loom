# W-AUTONOMY — PROGRESS

**Status**: 实现完成（RED 1-10 绿 + 旅程 PASS）— 待独立评审与 Product Owner
验收确认（flash 独立评审为开放项，见 REVIEW-NOTES）
**Contract**: `W-AUTONOMY-CONTRACT.md`（FROZEN @ `89ec751e`）
**Release target**: `v0.2.1 experimental`（默认关闭）

## 已交付

### 1. Standing Order 领域模型（契约 §3.1）

- `internal/rules/standing_order.go`：
  - `StandingOrder`（OrderID/Scope/ScopeID/TriggerRuleID/Tool/Pattern/
    BudgetRuleID/MaxIterations/Active/RevokedAt/ExpiresAt/AuthorizedBy/Digest）；
  - 定义默认 inactive；激活需显式 human actor（空 actor 拒绝）；
  - 全生命周期 Journal 事实（流 `autonomy/<order_id>`）：
    Defined/Activated/Deactivated/Revoked/Expired/Dispatch/Blocked；
  - `CheckDispatch` 无副作用 fail-closed 门（active/revoke/expire/
    generation/scope/trigger 规则状态/budget 只读可用性/max_iterations）；
  - `ConsumeDispatch` 复用 W-RULES `ConsumeBudget`（append-only）+ 迭代事实；
    幂等重放不重复消耗/不重复计数；
  - `Rebuild` 从 Journal 投影（读模型，非第二权威）。

### 2. 复用既有权威（契约 §3.2/3.3）

- 触发器 = W-RULES 客户规则状态（active 才可 dispatch）+ Tool/Pattern 匹配；
- 预算 = W-RULES `ConsumeBudget`（只读检查不消耗，dispatch 后原子消耗）；
- Autopilot 默认关闭：无 order/不激活时零行为变更；
- 每次工具调用仍经 B-W1 `execution.Adapter.Execute`（deny/admin lock/危险
  命令/沙箱门禁优先级不变）——旅程中 2 次真实执行均走完整权限管道。

### 3. 受控旅程（契约 §5）

- `cmd/wautonomy-journey/main.go`：define(默认 inactive 阻断) → human 激活 →
  预算内 2 次真实 dispatch → 第 3 次 budget 耗尽停止 → revoke 阻断；
  Journal 事实精确（Dispatch=2、BudgetConsumed=2、Completed=2）。
- `scripts/verify-wautonomy-cross-client-journey.sh` PASS：
  `/private/tmp/wautonomy-journey.*`（journey 零漂移、事实集、双客户端
  autonomy+gui observe、权限卫生、postflight）。

## RED 矩阵 ↔ 证据

| RED | 场景 | 证据 |
|---|---|---|
| 1 | 定义默认 inactive 零 dispatch | TestRedSA1 + 旅程 define |
| 2 | Autopilot 默认 off；显式 human 激活 | TestRedSA2 + 旅程 activate |
| 3 | 触发器/scope/budget/max 全满足；任一不满足 fail-closed | TestRedSA3 + 旅程 dispatch_3 |
| 4 | deny/admin lock/危险/沙箱优先 | 旅程每次经 Execute；契约不变量 |
| 5 | budget 耗尽 stop；append-only | TestRedSA5 + 旅程 |
| 6 | max_iterations stop 不超发 | TestRedSA6 |
| 7 | revoke 拒绝；in-flight 不重复 | TestRedSA7 + 旅程 revoke |
| 8 | generation/scope 失配拒绝 | TestRedSA8 |
| 9 | 全生命周期事实可重建 | TestRedSA9 |
| 10 | 双 order 隔离 + 幂等重放 | TestRedSA10 |

## 验证矩阵（本阶段执行）

- [x] `go build ./...` / `go vet ./...` / `go test ./...`
- [x] `go test -race`（rules standing orders）
- [x] `gofmt -l` 干净
- [x] 旅程 + verify 脚本 PASS
- [ ] flash 独立评审（开放项）

## 开放项（非阻断，记录待办）

1. ~~daemon 挂载与 TUI Autopilot 屏~~（已交付：`standing_order_snapshot/command`
   IPC + `internal/app`/`internal/api` 服务 + `internal/tui/autonomy.go`
   Autopilot 屏 + `cmd/loomd/wautonomy_wire_test.go` 组合测试）。
2. per-Job 触发器的 ActionContext 绑定细化（当前触发器状态检查 + 调用模式
   匹配；Action/Risk 语义由 W-RULES 规则承载）。
3. flash 独立评审开放项。

## 产品面接线（2026-08-06 更新）

- `internal/app/local_standing_order.go`：LocalStandingOrderService
  （Snapshot 只读 + Command define/activate/revoke，human actor 强制）。
- `internal/api/local_standing_order.go`：IPC 薄封装。
- `internal/localipc/protocol.go`：`standing_order_snapshot` /
  `standing_order_command` 方法注册。
- `cmd/loomd/product_daemon.go`：服务组装 + handler 分发 + nil 守卫
  （第 15 参 `standingOrderService`，全部既有 wire 测试调用点同步更新）。
- `cmd/loomd/wautonomy_wire_test.go`：组合级 IPC 测试 PASS
  （define→snapshot inactive→activate(human)→snapshot active→revoke→
  snapshot revoked；空 actor 拒绝）。
- `internal/tui/autonomy.go` + `model.go`：Autopilot 屏（默认 off 显示、
  K 激活 / L 撤销 / r 刷新），`autonomy_test.go` PASS；屏幕导航测试同步
  更新（新增第 17 屏，selections 扩容）。
