# B-P1 Amendment 1（实现期评审修复）

Date: 2026-08-05

## 背景

flash 独立 Implementation Review（bp1_impl_review_v9）返回 FAIL：P0×1、
P1×3、P2×4。本 Amendment 记录修复与开放项。所有修复均通过确定性矩阵与
跨客户端旅程验证。

## A1（P0 修复，已实现）

`evaluate.go`：危险段检查前置于 allow/ask 规则与 grants（deny 仍最优先）。
allow 规则/模板/grants/bypass 均不能放行含危险段的链式命令。新增 RED #18
（`TestRed18_AllowRuleCannotPassDangerousChain`）。契约 RED 矩阵 17→18。

## A2（P1-3 修复，已实现）

`RecordDecision` 签名携带 `call ProposedCall`；`PermissionDecisionRecorded`
事实现含 `tool/command/path`，授权审计链完整。契约 §4/§5.1 同步；App
attention 视图、Swift `PermissionDecisionView`、TUI grant 派生同步
（TUI grant pattern 现取决策命令首词，`issued_at` 用真实时间，空命令拒绝）。

## A3（P1-2 修复，已实现）

`rulesForJob`/`grantsForJob`/`EffectiveMode` 补齐 personal/project 作用域
（单项目 daemon 假设：project/personal 规则与授权作用于全部 Job；
多项目 job→project 映射为后续 bounded amendment）。新增 RED #19。

## P2 修复（已实现）

- P2-1：契约 §5.1 `PermissionGrantIssued` 载荷移除已废弃的 `danger` 字段。
- P2-2：TUI `permissionGrantAlways` 改用决策命令首词 + 真实 `issued_at` +
  空命令拒绝（无 panic）。
- P2-4：source-lock 在全部修改后重新生成（本文件同批提交）。

## A4（P1-1，OPEN — 需独立评审后实施）

批准生命周期接线：`validate_call` 命中 ask 时创建既有 `internal/rules`
ApprovalRequest（经适配器实现 `CustomerAuthorizer`，授权有界的权限 ask），
Job 进入 `waiting_approval`；`ResolveApproval` 转发 rules 权威；attention
按 resolved 状态过滤（P2-3 一并关闭）；TUI a/x/g 完成真实批准。

设计要点：不新建第二套批准权威；rules 事件与 permission 决策事实共存；
generation/幂等沿用现有语义。因涉及 authority 扩展，按治理约定本项
**不静默实施**，待独立评审 + Product Owner 确认后作为 Amendment 2 落地。

## 验证

- Go build/vet/test 全 PASS；Swift build + 98 tests PASS。
- 新旅程证据根（final9）经 `verify-bp1-cross-client-journey.sh` PASS。
- RED 18/19 通过；既有 RED 1-17 无回归。
