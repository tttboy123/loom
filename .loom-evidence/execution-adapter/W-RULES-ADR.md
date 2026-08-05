# ADR: W-RULES 客户规则与 standing policy

**Status**: FROZEN — 已获 Product Owner Goal 指令授权（Controller 自查；flash 评审开放项）
**Date**: 2026-08-06
**Baseline**: `8be5d8ab`（W-BRIDGE）on `codex/loom-platform-slice2`
**Closes**: P2B 记录的 capability_gap（现有 Rules/Approval 事实无法表达
可撤销、可过期、带预算的 standing policy）

## Context

Loom 已有：B-P1 权限规则（工具调用级，Journal 事实）、A4 批准生命周期
（复用 internal/rules）、`internal/rules` 客户 RuleSet 权威（RuleSetActivated +
Evaluate + ApprovalRequested/ApprovalDecided）。但客户定义规则（TECH-PLAN §8/§9）
仍是 capability_gap：没有"仅记录/告警/拒绝"效果、没有规则可撤销/过期/预算语义、
没有 `.loom/permissions.toml` 导入通道。

## Decision

1. **规则生命周期为 Journal 事实**：CustomerRuleDefined/Revised/Revoked/Expired
   （流 `customer-rule/<rule_id>`）；规则带 scope（project/team/work_package/
   work_item）、匹配条件、effect（require_approval/report_only/reject）、
   budget（单位 + 上限 + 周期）、expires_at。
2. **效果语义**：
   - require_approval → 复用 A4 批准（Job/WorkItem 暂停，批准后恢复）；
   - report_only → 记录 RuleMatched 事实并继续（零阻塞）；
   - reject → 阻断 + RuleDenied 事实。
3. **预算为事实计数**：BudgetConsumed 逐次追加（append-only），超限 fail-closed
   拒绝；预算可 revoke/replace（不可改写历史）。
4. **导入通道**：`.loom/permissions.toml` 只由 human 授权命令导入为
   CustomerRuleDefined 事实（复用 B-P1 配置导入模式）；文件永不直接评估。
5. **权威边界**：不新建第二套规则权威；Evaluate 复用 internal/rules；
   权限规则（B-P1）与客户规则分层组合（工具调用先过权限管道，客户规则在
   领域层裁决）。

## Alternatives considered

- 维持 capability_gap：standing policy 与 Autopilot 无法解锁，拒绝。
- 把客户规则塞进 B-P1 权限规则：混淆工具级与领域级语义，拒绝。
- 第二套规则权威：违反单一权威，拒绝。

## Consequences

- 正面：客户可定义可撤销/过期/带预算的 standing policy；报告/告警/拒绝/
  暂停四种效果；全部可审计可重放。
- 代价：规则评估与预算计数新增 Journal 事实与状态机；需与 A4 批准、
  B-P1 权限管道分层组合。
