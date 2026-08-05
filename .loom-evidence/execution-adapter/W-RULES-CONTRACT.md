# Gate 1 Exit Contract — W-RULES 客户规则与 standing policy

**Status**: FROZEN — EXECUTION AUTHORIZED（Product Owner Goal 指令：完成 Phase 3 全部扩展任务 + Controller 冷读自查；flash 独立评审为开放项）
**Date**: 2026-08-06
**Baseline**: `8be5d8ab` on `codex/loom-platform-slice2`
**Risk**: STRICT — 新 Journal facts、规则状态机、预算计数、导入通道

## 1. 唯一垂直 WorkItem

```text
W-RULES Customer Rules and Standing Policy
```

包含：规则定义/修订/撤销/过期、effect 语义（require_approval/report_only/
reject）、预算计数与超限 fail-closed、`.loom/permissions.toml` 导入、
领域层 Evaluate 接线、跨客户端旅程。不包含：Autopilot 触发器（W-AUTONOMY）；
网络/第三方；自动批准绕过。

## 2. 产品结果

客户在 TUI/原生 app（或导入配置文件）定义规则：匹配条件 → 效果
（require_approval 暂停 / report_only 记录 / reject 拒绝）+ 预算 + 过期。
规则可随时撤销；预算超限后规则 fail-closed 拒绝并告警；重启/重放后规则、
预算、批准状态与 Journal 一致。

## 3. 精确语义

### 3.1 规则模型

```text
CustomerRule{rule_id, scope{kind,id}, condition{action?, tool?, path?, risk?},
  effect{require_approval|report_only|reject, approver_refs?, timeout?, on_timeout?},
  budget{unit, limit, period?}?, expires_at?}
```

### 3.2 生命周期（Journal 事实，流 `customer-rule/<rule_id>`）

- CustomerRuleDefined（含 digest）/ Revised / Revoked / Expired
- RuleMatched / BudgetConsumed / RuleDenied / ApprovalRequested 复用 A4

### 3.3 效果

- require_approval：复用 A4（Job/WorkItem waiting_approval，批准恢复）。
- report_only：记 RuleMatched，继续（零阻塞、零暂停）。
- reject：RuleDenied + 阻断（不执行、不暂停）。
- budget：BudgetConsumed 逐次追加；周期内累计达限 → fail-closed 拒绝 +
  RuleBudgetExhausted 事实；预算不可改写历史。

### 3.4 导入

`.loom/permissions.toml`（可提交）与 `.loom/permissions.local.toml` 仅由
human 授权命令导入为 CustomerRuleDefined 事实；未导入零效果；导入幂等
（同一文件 digest 重放不产生重复规则）。

## 4. 冻结符号

```go
// internal/customerrule（新包，或 internal/rules 扩展——冻结时二选一，默认扩展 internal/rules）
type CustomerRule struct{...}
func (a *Authority) DefineCustomerRule(ctx, CustomerRule, authorizedBy, operationID, journeyID) ([]journal.Event, error)
func (a *Authority) RevokeCustomerRule(ctx, ruleID, authorizedBy, operationID, journeyID) ([]journal.Event, error)
func (a *Authority) EvaluateCustomerRules(ctx, ActionContext) (CustomerDecision, error)
func (a *Authority) ConsumeBudget(ctx, ruleID, operationID, journeyID) ([]journal.Event, error) // 超限 fail-closed
func ImportPermissionsTOML(ctx, content []byte, authorizedBy, operationID, journeyID) ([]journal.Event, error)
```

## 5. RED-first 矩阵

1. define/revoke/expire 幂等（同 operationID 重放无双份）。
2. require_approval → 复用 A4 暂停；批准恢复；拒绝不恢复。
3. report_only → RuleMatched + 继续，零阻塞零暂停。
4. reject → RuleDenied + 阻断，零执行。
5. 预算超限 → BudgetConsumed 后 fail-closed 拒绝 + BudgetExhausted。
6. 预算撤销/替换不可改写历史（append-only）。
7. 导入未执行零效果；导入幂等；文件不直接评估。
8. 重启/重放后规则/预算/批准一致；未知事件拒绝。
9. 权限规则与客户规则分层：工具调用先权限管道，客户规则领域层裁决。
10. authorized_by 门禁（root/scope 写规则需 human 通道）。

## 6. 验证矩阵

Go full/race/vet/tidy/gofmt；真实跨客户端旅程（TUI 定义规则 + 导入 +
report_only/require_approval/reject 三路径 + 预算超限）+ flash 独立评审；
原子提交。

## 7. 非目标

Autopilot 触发器（W-AUTONOMY）；网络/第三方；自动批准绕过；第二套规则权威；
覆盖用户既有 Skill。

## 8. 停止条件

身份不一致、未评审 authority/schema 扩展、任何独立 Review FAIL、dirty 无法
隔离 ⇒ 停止 HUMAN_REQUIRED。


## 6.1 Exact owned files

### 新增

```text
internal/rules/customer_rule.go       // CustomerRule 模型 + Define/Revoke/Expire + Evaluate + ConsumeBudget
internal/rules/customer_rule_test.go  // RED 1-10
internal/rules/permissions_import.go  // .loom/permissions.toml 子集解析 + 导入为事实
internal/rules/permissions_import_test.go
internal/app/local_customer_rule.go   // 产品服务：规则 CRUD/导入/预算查询
internal/app/local_customer_rule_test.go
internal/api/local_customer_rule.go
internal/tui/customerrule.go          // ScreenCustomerRules（列表/定义/导入/预算）
internal/tui/customerrule_test.go
cmd/loomd/customer_rule_wire_test.go
scripts/verify-wrules-cross-client-journey.sh
.loom-evidence/execution-adapter/W-RULES-PROGRESS.md
```

### 修改（最小面）

```text
cmd/loomd/product_daemon.go       // 挂载 customer rule 服务 + IPC
internal/localipc/protocol.go     // customer_rule_snapshot/command 方法
internal/tui/model.go             // ScreenCustomerRules 路由
apps/macos/LocalIPCClient.swift   // 只读规则/预算状态
```

`internal/permissions`、`internal/execution` 只被消费；不重写其语义。
