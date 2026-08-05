# W-BRIDGE Progress

Updated: 2026-08-06

- Gate 1 FROZEN（W-BRIDGE-CONTRACT.md / ADR / RED-SPEC / FREEZE-RECORD）。
- 实现（子代理越权实现 + Controller 复核收编）：
  - `internal/runtime/piadapter/toolcall.go`：ToolCallEnvelope 严格解码（未知键/
    重复键/尾随值/大小上限/白名单工具）+ ToolCallSystemPrompt（启用工具面）。
  - `rpc_bridge_adapter.go`：`piRPCSystemPrompt = ToolCallSystemPrompt()`；
    acceptToolCallEvent ToolCallEnd → piRPCToolCallEnvelope → DecodeToolCallEnvelope
    → toolHook.ExecuteToolCall（nil hook fail-closed）→ emitToolCallResult。
  - `cmd/loomd/wbridge_wiring.go`：bridgeExecutionHook 接 B-W1 execution.Adapter；
    daemon 注入 ToolHook。
  - execution 增强：pending-Allowed 无 terminal → ErrExecutionInterrupted（不重执行）；
    limit_exceeded 错误码；幂等键改用 executionID。
- Controller 修复：**ResolveEffectiveProfile 合并 profile 内嵌规则**（B-P1 §3.2
  "Job profile 规则"作用域缺失，模板/内嵌 allow 此前未生效）；deny wire 测试
  断言修正（deny 的 ExecutionID 是拒绝记录标识，零执行由事实断言保证）。
- 验证：RED W1-W5 + TestWBridgeHook Allow/Ask/Deny + TestProfileEmbeddedRulesApplyToJob
  全绿；Go 全矩阵 + Swift 98 tests 全 PASS。
- 待办：真实 Pi 桥接旅程（allow 执行 / ask 批准 / deny 零执行）+ flash 独立评审
  开放项 + 原子提交。
