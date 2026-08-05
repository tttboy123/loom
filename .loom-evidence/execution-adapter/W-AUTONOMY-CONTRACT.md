# Gate 1 Exit Contract — W-AUTONOMY Standing Orders / Autopilot

**Status**: FROZEN — EXECUTION AUTHORIZED（Product Owner Goal 指令：完成
Phase 3 全部扩展任务 + Controller 冷读自查；flash 独立评审为开放项）
**Date**: 2026-08-06
**Baseline**: `cf9360aa` on `codex/loom-platform-slice2`
**Risk**: STRICT — 持久自动化面打开；authority 边界（复用 W-RULES/B-W1，
不建第二套权威）

## 1. 唯一垂直 WorkItem

```text
W-AUTONOMY Standing Orders / Autopilot（默认关闭的有界形态）
```

包含：Standing Order 领域模型（定义/激活/撤销/过期/预算/触发器/scope/
max_iterations/stop）、Autopilot 默认关闭开关、复用 W-RULES Evaluate +
ConsumeBudget 与 B-W1 Execute 门禁的调度判定、全审计事实、受控旅程。
不包含：模型自建/自激活 orders；绕过 deny/ask/沙箱；第二套批准/规则/预算
引擎；网络/付费执行。

## 2. 产品结果

Human 可以定义一条 standing order（"当触发器匹配且预算充足时，在 scope 内
自动继续最多 N 次"）并随时停止/撤销；Autopilot 默认关闭，开启与激活均需
显式 human 动作；每次工具调用仍走 permissions.Evaluate + 沙箱门禁；所有
生命周期动作（定义/激活/预算/停止/撤销）都是 Journal 事实，可从 Journal
重建。

## 3. 精确语义

### 3.1 StandingOrder 领域模型（冻结）

```go
// internal/rules（消费 W-RULES 既有语义）
type StandingOrder struct {
    OrderID      string            // 稳定 ID（human 定义）
    Scope        ScopeKind         // job | project | personal（复用）
    ScopeID      string
    Trigger      RuleID            // 复用 W-RULES 规则的匹配语义
    Tool         permissions.ToolKind
    Pattern      string            // 命令/路径模式（复用 rule match）
    BudgetID     string            // 复用 W-RULES ConsumeBudget
    MaxIterations int              // >0；达到即 stop
    Active       bool              // 默认 false
    RevokedAt    string            // "" | RFC3339
    ExpiresAt    string            // "" | RFC3339（可选）
}
```

- 定义后默认 inactive；激活/撤销/过期均为显式 human 动作（A4 通道 +
  Journal 事实）。
- 模型输出/Agent 提议只是 Proposal；standing order 本身是授权表面，不执行。

### 3.2 调度判定（复用既有权威，不新建）

```text
order.Active && !revoked && !expired &&
trigger.Evaluate(call) 匹配 &&
scope 匹配 (job/project/personal) &&
budget.Consume(order.BudgetID) 成功 &&
iterations < order.MaxIterations
  → 允许将 call 作为 Proposal 交给 B-W1 execution.Adapter.Execute
其余任何不满足 → 不 dispatch（fail-closed，不静默放行）
```

- 每次调用仍经 permissions.Evaluate：deny 规则、admin lock、危险命令重询、
  沙箱 Required 门禁优先级不变。
- Standing order 至多提供有界 grant（scope+tool+pattern+budget）；不绕过
  ask（ask 仍走 A4 批准）。

### 3.3 Autopilot 开关（默认关闭）

- 全局与 per-Job Autopilot 标志默认 off；on 需要显式 human 批准（A4 +
  Journal 事实）。
- Autopilot=off 且无 active order → 行为与当前完全一致。

### 3.4 停止与撤销（有界）

- budget 耗尽 / max_iterations 达到 / revoke / scope|generation 失配 /
  触发器不再匹配 → 停止后续 dispatch（Journal stop 事实）。
- in-flight 调用有界完成（不 kill 已授权执行，除非沙箱 Cancel 显式调用）。
- revoke 立即生效：后续 dispatch 拒绝。

### 3.5 审计

- define/activate/deactivate/revoke/expire/budget_spend/stop 全部为
  Journal 事实（流 `autonomy/<order_id>` 与预算/执行既有流）；read model
  可重建，无第二权威。

## 4. RED-first 矩阵

1. 定义 order → 默认 inactive + Journal 事实；无 dispatch。
2. Autopilot 默认 off；显式激活（human 批准）后才可 dispatch。
3. 触发器匹配 + scope 匹配 + budget 足够 + 未达 max → dispatch 一次；
   每项不满足 → 不 dispatch（fail-closed）。
4. 每次 dispatch 仍经 permissions.Evaluate；deny/admin lock/危险命令/
   沙箱 Required 优先级不变（order 不绕过）。
5. budget 消耗 append-only；耗尽 → stop 事实 + 不再 dispatch。
6. max_iterations 达到 → stop；不超发。
7. revoke → 后续 dispatch 拒绝；in-flight 不重复执行。
8. scope/generation 失配或触发器失配 → 拒绝。
9. 全生命周期事实可重建；无第二权威。
10. 双 order 并行隔离；幂等重放不重复消耗 budget/不重复执行。

## 5. 验证矩阵

Go full/race/vet/tidy/gofmt；受控旅程：define(默认 off) → activate(human
批准) → dispatch(预算内多次) → budget 耗尽/max 停止 → revoke 拒绝；
permissions 卫生 + Journal 事实断言；跨客户端旅程（bridge/tui + gui
observe，沿用 W-BRIDGE schema）；flash 独立评审；原子提交。默认关闭
（无 order/off 时不改变现有行为）。

## 6. 非目标

模型自建/自激活 orders；绕过 deny/ask/沙箱；第二套批准/规则/预算引擎；
网络/付费；覆盖用户 Skill；push/merge 远端。

## 7. 停止条件

身份不一致、未评审 authority/schema 扩展、任何独立 Review FAIL、dirty
无法隔离、越出 owned files ⇒ 停止 HUMAN_REQUIRED。

## 7.1 Exact owned files

### 新增

```text
internal/rules/standing_order.go          // 模型 + define/activate/revoke/expire/evaluate-dispatch 判定
internal/rules/standing_order_test.go     // RED 1-10
internal/app/local_standing_order.go      // 产品表面（只读 + human 动作）
internal/api/local_standing_order.go
internal/tui/autonomy.go                  // TUI Autopilot 屏（默认 off 显示）
cmd/loomd/wautonomy_wiring.go             // daemon 接线（order→dispatch 探针）
cmd/loomd/wautonomy_wire_test.go
cmd/wautonomy-journey/main.go             // 受控旅程 harness
scripts/verify-wautonomy-cross-client-journey.sh
.loom-evidence/execution-adapter/W-AUTONOMY-PROGRESS.md
```

### 修改（最小面）

```text
internal/rules/customer_rule.go  // 仅消费（trigger/budget 复用，不改语义）
internal/execution/adapter.go    // 仅消费（dispatch 探针经 Execute，默认 nil 不启用）
cmd/loomd/product_daemon.go      // 挂载 autonomy 服务（默认 off）
```

不触碰：permissions 评估语义、B-W1 执行门禁、Phase 3B 沙箱门禁、rules 批准
权威（仅消费）；模型客户端/Provider 路由。
