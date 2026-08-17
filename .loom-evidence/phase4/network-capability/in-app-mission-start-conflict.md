# Phase 4 · 剩余阻断：in-app Mission start 冲突（enrollment 绑定 team）

Status: `BLOCKED — 需专项调试`
Date: 2026-08-17

## 已闭环（live 证据）
- 安装版 daemon broker 开启（`LOOM_ENABLE_WEB_TOOLS=1`）+ `web_search` enrollment
  物化 + Team 就绪 + 撤销隔离：G5 gate `PASS`。
- 真实模型主动调 web 工具循环（DeepSeek → Loom toolbroker → 互联网 → 作答）：
  `PASS`。
- harness loom_context MCP 发布/解码/执行 web 工具、execution adapter 提交真实
  结果：均 `PASS`。

## 剩余阻断：Mission start `conflict`
复现（`cmd/loomd/live_web_mission_e2e_test.go`，
`LOOM_LIVE_WEB_MISSION=1 LOOM_LIVE_TEAM_E2E=1 LOOM_LIVE_NET=1`）：
- 2-agent team（deepseek main + deepseek subagent，subagent 绑定 `web_search`
  enrollment）：**preflight 成功、subagent ready**；
- `mission_execution start` 返回 `conflict`（重试 + 重新 preflight 后仍 conflict）。

## 诊断方向
- `internal/app/local_product_execution.go` `compileMissionExecution`：start 比
  preflight 多做 `persistContextCapsules=true`；冲突来自
  `binding.ViewVersion != command.ExpectedViewVersion` 或角色形状校验
  （`missionExecutionRoleBindings` 对 enrollment 绑定节点的映射）或
  start 路径的 fallback approval（`resolveFallbackApproval`）。
- 需要专项排查：enrollment 绑定 subagent 在 start 时 binding/view 一致性、
  或 role shape（route-sibling/aggregation/fallback）校验。

## 下一步
把该冲突作为独立任务调试（不改动已交付的能力与证据）；修好后重跑
`TestLiveMissionAgentCallsWebSearch` 断言 journal 出现 `"tool":"WebSearch"`
执行事实，才算 in-app attempt 全链路闭环。
