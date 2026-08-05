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
- 旅程（本更新）：`cmd/wbridge-journey` 用真实 piadapter 桥接（fake-pi 夹具）
  + bridgeExecutionHook + B-W1 Adapter + Journal 组合驱动三段：
  - allow：`printf wbridge-ok` → allow → 执行一次 → ToolExecutionCompleted；
  - ask：`curl` → ask 零执行 → A4 ApprovalRequested/ApprovalDecided →
    同信封重放 → 恢复执行恰好一次；
  - deny：`rm -rf /`（deny rule）→ deny 零执行 → 原因回桥接。
  - 旅程发现并修复真实缺陷：piadapter 构造 ToolCallBinding 时未传播
    JourneyID，导致桥接 ask 路径的批准 CorrelationID 为空而失败
    （hook 级测试用 nil approvals 掩盖了该路径）；修复为从 dispatch
    帧 CorrelationID 派生（owned file 内最小变更）。
  - `scripts/verify-wbridge-cross-client-journey.sh` PASS：
    `/private/tmp/wbridge-journey.*`（journey_id 零漂移、事实集、
    ApprovalDecided、audit 只含摘要、权限卫生、postflight）。
- 开放项：daemon 传输层（Pi 本地模型服务器需真实模型二进制）与 TUI/GUI
  面板的桥接显示为受限旅程；执行/批准表面已由 B-W1/C-W1 跨客户端旅程覆盖，
  桥接侧以 bridge-runtime 双客户端（bridge + gui observe）证据闭环（bounded
  amendment，已记录 REVIEW-NOTES）。flash 独立评审开放项。
