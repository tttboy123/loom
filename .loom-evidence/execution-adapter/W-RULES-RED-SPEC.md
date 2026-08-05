# W-RULES RED Spec

Date: 2026-08-06

| RED | 场景 | 断言 |
|---|---|---|
| 1 | define/revoke/expire 幂等 | 同 operationID 无双份事实 |
| 2 | require_approval 暂停/批准恢复 | 复用 A4；批准后恢复，拒绝不恢复 |
| 3 | report_only 零阻塞 | RuleMatched + 继续 |
| 4 | reject 阻断 | RuleDenied + 零执行 |
| 5 | 预算超限 fail-closed | BudgetConsumed 达限 → 拒绝 + BudgetExhausted |
| 6 | 预算 append-only | 撤销/替换不改写历史 |
| 7 | 导入零直接评估 + 幂等 | 未导入零效果；重复导入无双份 |
| 8 | 重启/重放一致 | 规则/预算/批准重放一致；未知事件拒绝 |
| 9 | 权限/客户规则分层 | 工具调用先权限管道，领域层客户规则 |
| 10 | authorized_by 门禁 | root/scope 写规则需 human 通道 |
