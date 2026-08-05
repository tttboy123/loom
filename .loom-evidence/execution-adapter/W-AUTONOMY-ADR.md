# ADR: W-AUTONOMY Standing Orders / Autopilot（默认关闭的有界形态）

**Status**: FROZEN — 已获 Product Owner Goal 指令授权（Controller 自查；flash
评审开放项，见 REVIEW-NOTES）
**Date**: 2026-08-06
**Baseline**: `cf9360aa` on `codex/loom-platform-slice2`
**Release target**: `v0.2.1 experimental`（默认关闭）

## Context

Phase 3 目标要求"standing orders / Autopilot 默认关闭的有界形态：持久授权/
预算/触发器/scope/stop-revoke/审计"。W-RULES 已提供规则生命周期、effect 与
ConsumeBudget；B-W1 已提供每次工具调用的 permissions.Evaluate 执行门禁；
Phase 3B 已提供沙箱策略门禁。W-AUTONOMY 需要在这些之上提供"跨多次调用的
持久、可撤销、有预算的自动继续"形态，同时绝不建立第二套执行权威。

## Decision

1. **Standing Order = 持久授权表面**（非执行权威）：由 Human（root
   policy/管理员或项目 owner）显式定义并激活；包含 trigger（复用 W-RULES
   Evaluate）、scope、预算（复用 W-RULES ConsumeBudget，append-only）、
   max_iterations、stop 条件、revoke 通道。模型输出/Agent 永远不能自行
   创建、激活、续期或撤销 Standing Order。
2. **Autopilot 默认关闭**：全局与 per-Job 的 Autopilot 标志均为 off；开启
   需显式 human 批准（A4 通道 + Journal 事实）。关闭时行为与现状完全一致
   （每次工具调用仍走 Execute 的 allow/ask/deny）。
3. **每次工具调用仍经 permissions.Evaluate**：Standing Order 至多提供有界
   grant（scope+tool+pattern+budget），不能绕过 deny 规则、admin lock、
   危险命令重询或沙箱 Required 门禁。
4. **有界停止**：budget 耗尽、max_iterations 达到、revoke、scope/generation
   失配、触发器不再匹配 → 停止后续 dispatch；in-flight 调用有界完成。
5. **全审计**：define/activate/deactivate/revoke/budget spend/stop 全部为
   Journal 事实；read model 可重建（无第二权威）。
6. **默认关闭 + 显式激活**：任何 standing order 定义后默认 inactive；
   Autopilot 开关与 order 激活均需 human 显式动作。

## Alternatives considered

- 让模型/Agent 自建自激活 standing orders：违反不变量 3/8（模型输出非执行
  权威、root policy 不被在线自演化修改），拒绝。
- 完全无自动继续：无法达成 Phase 3 目标（standing orders 是有界自动化的
  最小形态），拒绝。
- 用第二套批准/预算/规则引擎：违反单一 Journal 权威与"不建第二套规则"，
  拒绝；一律复用 W-RULES + B-W1 既有语义。

## Consequences

- 正面：获得可审计、可撤销、有预算的持久自动化表面；默认关闭零行为变更；
  复用既有权威链（规则/执行/沙箱），无第二套批准或执行权威。
- 代价：W-AUTONOMY 为 experimental 层（默认关闭），需要受控旅程验证
  define→activate→dispatch→budget→stop/revoke 全闭环。
