# Failed step 可终结修复（2026-08-20 · commit 02d4c9c8）

## 根因（分阶段 debug 定位）
- 安装版/调试版 daemon 的 Mission 停在第一个 context-read ToolDispatchCommitted：
  - 有界拒绝 payload 已写入 + ToolResultAccepted（fact 已 accept）；
  - 但 fact 从未 ToolResultDelivered（adapter 的 ack 未完成）；
  - 该步若以 final 或 failed 终结，EndStep 都因"已派发未投递的 tool call"
    返回 Attempt loop conflict → step 永久 open → Mission 永久 running。
- debug 证据：WRAP2-DEBUG delegate returned err=nil → EndStep failed =
  Attempt loop conflict；ENDSTEP-DEBUG call found=false。

## 修复（RED-first）
- EndStep 在 Outcome == AttemptStepFailed 时容忍 admitted-but-undispatched /
  dispatched-but-undelivered 的 tool call，使失败 attempt 可终结并让 Mission
  推进到 recovery/failure；成功 step 仍要求每个已派发 call 有 delivered fact。
- 新测试：TestAttemptLoopAuthorityAllowsFailedStepWithUndeliveredToolCall。

## 测试
Go work/app/contextcapsule/nativeadapter 全绿。
