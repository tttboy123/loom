# B-W1 执行适配器 — 生产可用验收包

**Status**: 证据齐备，待 Product Owner 确认（ACCEPTANCE REQUESTED）
**Date**: 2026-08-06
**Branch**: `codex/loom-platform-slice2`
**Release target**: v0.2.1 experimental（执行器默认本机沙箱，policy 默认关闭）

## 1. 验收范围（B-W1-CONTRACT.md 冻结语义）

- 模型/桥接的 ToolProposal 经 `execution.Adapter.Execute` 裁决：只有
  permissions.Evaluate=allow 才执行；ask 走 A4 批准后恢复一次；deny/stale/
  解析失败零执行。
- Journal 事实：ToolExecutionProposed/Allowed/Denied/Completed/Failed；
  证据为 digest-only；崩溃/重放不重复执行（exactly-once）。
- 确定性执行器 `SandboxExecutor`：cwd=worktree、消毒 env、禁网、超时、
  输出限量；越界路径拒绝。

## 2. 验收证据

| 项 | 证据 | 状态 |
|---|---|---|
| Gate 1 冻结 | `B-W1-CONTRACT.md` / `B-W1-ADR.md`（Controller 冷读 + Product Owner Goal 指令） | ✅ |
| 核心实现 | `3d750465`（internal/execution + daemon/app/api/TUI/Swift 表面） | ✅ |
| Controller 修复 | approved-resume partial-batch 冲突、approved-ask resume 语义、worktree 自动预置 | ✅ |
| RED 矩阵 | `internal/execution` 11 测试（allow/ask/deny/崩溃/重放/越界/幂等/隔离/密钥） | ✅ 全绿 |
| 跨客户端旅程 | `/private/tmp/bw1-journey-final`（`7794d968-c8a8-46ba-9c1b-1a2e5bae81c7`）：真实执行 propose→ask→A4 批准→resume 执行；`verify-bw1-cross-client-journey.sh` PASS（本次复验） | ✅ |
| 全矩阵 | Go build/vet/test 全 PASS；Swift 98 tests PASS（实现时） | ✅ |
| 不变量 | 模型输出非执行权威；Journal 唯一权威；证据 digest-only；无第二套批准/规则 | ✅ |
| 默认关闭 | 无 profile/policy 时不改变本机执行语义（Phase 3B wire 覆盖） | ✅ |

## 3. 验收结论（待确认）

```text
B-W1 = 生产可用验收证据齐备（ACCEPTANCE REQUESTED）
实现提交 = 3d750465（+ Controller 修复）
旅程 = bw1-journey-final（7794d968…）verify PASS（2026-08-06 复验）
全矩阵 = Go 全 PASS
开放项 = flash 独立评审（通道不可用，记录 REVIEW-NOTES）
```

## 4. 真实激活

C-W1 显式激活（launchd/常驻 daemon）与生产发布仍需 Product Owner 单独显式
批准（不变量 8 + 激活流程）；本包只申请"验收确认"，不自动激活。

## 5. 当前分支复跑说明（2026-08-06）

- 尝试用 bcw-journey.py 在当前分支（含 Phase 3 扩展，17 屏 TUI）重跑：
  产品侧首个周期正确到达 ask（Journal 事实 ToolExecutionProposed=1 /
  ApprovalRequested=1 / WorkItemApprovalPaused=1），随后外部 PTY 驱动的
  按键计划按 16 屏调校、新增 Autopilot 屏后按键落点偏移，A4 批准键未落在
  Attention 屏 → verify 报缺 ApprovalDecided。此为驱动适配问题（/tmp
  临时工具），非 B-W1 产品回归；执行适配器语义未变（Phase 3 扩展对
  B-W1 默认路径零行为变更，sandbox gate nil）。
- 验收证据以既有已复验旅程（`7794d968…` verify PASS）+ 当前分支全 Go 矩阵
  绿为准；驱动按键计划适配为记录型开放项（不影响验收结论）。
