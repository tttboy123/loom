# W-RULES Progress

Updated: 2026-08-06

- Gate 1 FROZEN（baa0ed6b；flash 评审开放项）。
- 核心实现（RED WR1/WR3/WR4/WR5/WR7 全绿，rules 全包无回归）：
  - `internal/rules/customer_rule.go`：CustomerRule 模型（scope/action/risk/
    effect/approver/timeout/budget/expires）、Define/Revoke/Expire（Journal
    事实、幂等、root 门禁）、EvaluateCustomerRules（scope+action+risk 匹配、
    效果优先 reject>require_approval>report_only、预算超限 fail-closed）、
    ConsumeBudget（append-only 计数）、ImportPermissionsTOML（有界子集解析、
    human 通道、幂等、零直接评估）。
  - require_approval 默认 on_timeout=reject；预算窗口按周期锚定规则定义时间。
- 表面层完成：app 服务（Snapshot/Command：define/revoke/expire/import/
  evaluate/consume_budget）、api 包装、daemon IPC（customer_rule_snapshot/
  customer_rule_command）、protocol 白名单、wire 测试（define→snapshot→
  evaluate require_approval）全绿。
- 待办：TUI ScreenCustomerRules + 跨客户端旅程（report_only/require_approval/
  reject/预算超限）+ require_approval→A4 暂停/恢复的 workItem→Job 映射接线
  + flash 评审开放项。
