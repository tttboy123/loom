# ADR: Phase 3B 受治理沙箱（SandboxBackend）

**Status**: FROZEN — 已获 Product Owner Goal 指令授权（Controller 自查；flash 评审开放项）
**Date**: 2026-08-06
**Baseline**: `46d522ff`（W-RULES 旅程）on `codex/loom-platform-slice2`
**Release target**: `v0.2.1 experimental`（默认关闭）

## Context

B-W1 执行器当前用本机 `SandboxExecutor`（cwd=worktree、消毒环境、禁网、超时）。
"间接危险调用"（A13）与远端隔离需要供应商中立的沙箱能力。TECH-PLAN §12.4
冻结最小能力：`Create/Exec/Cancel/Pause/Resume/Destroy/InspectCapabilities`，
默认关闭、单一后端、policy 要求时 fail-closed、后端状态非权威。

## Decision

1. **供应商中立接口**：`internal/sandbox.SandboxBackend`（7 个能力方法），
   实现可选（AgentENV/CubeSandbox/E2B 经受控 spike 选一，同一版本最多一个）。
2. **接入 B-W1**：`execution.Adapter` 增加沙箱 policy 门禁——profile/Job 可带
   `SandboxPolicy{Required bool, Backend string}`；policy 要求沙箱而后端不可用
   → 拒绝执行（fail-closed），绝不静默回退本机执行。
3. **状态权威不变**：后端 scheduler/checkpoint/session/callback 不是权威；
   Loom 从 Journal + accepted artifact facts 重建、reconcile、取消、清理；
   后端实例绑定 Job/Run generation（fencing）；不传 Provider 原始凭证、
   raw Grant、隐藏推理或超出当前 Run 的 workspace。
4. **默认关闭**：无 policy 时走本机 SandboxExecutor；Phase 3B 状态保持
   EXPERIMENTAL 直到受控 canary（隔离执行、取消、重启恢复、无凭证泄漏）通过。

## Alternatives considered

- 只做本机沙箱：A13/远端隔离不落地，拒绝。
- 同时承诺多后端：接口相似也禁止双后端（TECH-PLAN），拒绝。
- 后端作为第二权威：违反单一 Journal 权威，拒绝。

## Consequences

- 正面：工具执行获得可选纵深隔离；policy fail-closed；后端可替换；
  无凭证/推理泄漏面。
- 代价：沙箱后端为实验层（默认关闭），需要受控 spike 选型与 canary 证据。
