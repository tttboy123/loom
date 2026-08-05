# ADR: W-BRIDGE 模型桥接接入执行适配器

**Status**: FROZEN — 已获 Product Owner Goal 指令授权（Controller 自查；flash 评审开放项）
**Date**: 2026-08-06
**Baseline**: `f69b49d9`（B-W1/C-W1 接线收尾）on `codex/loom-platform-slice2`
**Parent**: B-W1（执行适配器，已实现待验收）

## Context

B-W1 已提供执行面（Evaluate 门禁 + 沙箱执行器 + Journal 事实 + 恢复），但模型
桥接（`internal/runtime/piadapter/rpc_bridge_adapter.go`）仍以 `--no-tools` +
系统提示 `Use no tools` 关闭工具面：授权管道没有真实模型提议可消费。W-BRIDGE
把"模型提议工具调用 → 执行适配器 → 结果回模型"打通，是"Loom 像 Codex 一样
真实开发"的最后一公里。

## Decision

1. **桥接启用单一结构化工具调用协议**：移除 `--no-tools`；新系统提示声明
   模型只能通过严格 JSON 信封 `{"job_id","call":{"tool","command","path"}}`
   提议工具调用（作为唯一工具），隐藏推理/凭据/文件系统路径禁止入信封。
2. **模型提议、daemon 执行**：桥接只透传信封；daemon 绑定 Run/Job 的 profile
   digest + generation，调用 `execution.Adapter.Execute`：allow → 沙箱执行 +
   结果/证据回桥接（仅已批准调用的结果进 transcript audit）；ask → 走 A4
   批准（Job 暂停，批准后同一 call digest 恢复执行一次）；deny/error → 工具
   返回 typed denial，零执行。
3. **不变量 3 保持有界**：模型输出仍不是执行权威；只有 Evaluate=allow（或
   ask 经批准）且授权事实链完整的信封被执行适配器执行。
4. **桥接确定性**：桥接不含执行决策；信封解析失败/未知工具/越界 → 拒绝并
   返回错误，绝不执行。

## Alternatives considered

- 维持 Use no tools：执行适配器无消费方，B-W1 价值悬空，拒绝。
- 模型直连执行器：绕过 Evaluate/Journal 事实链，违反不变量 3/5，拒绝。
- 多工具自由协议：审计面失控；先冻结单一信封，后续经评审扩展。

## Consequences

- 正面：真实模型可经授权管道执行工具调用；每次执行可审计、可批准、可拒绝；
  ask/deny/error 零执行；桥接保持确定性。
- 代价：桥接需适配严格信封解析与工具结果回传；Pi 桥接的 toolcall 诊断事件
  首次真正启用，需要配套 RED 与旅程证据。
