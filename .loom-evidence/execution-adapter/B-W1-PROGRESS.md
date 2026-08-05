# B-W1 Progress

Updated: 2026-08-05

- Gate 1 FROZEN（B-W1-CONTRACT.md；flash 独立评审开放项）。
- 核心实现提交 `3d750465`：internal/execution 门禁/沙箱执行器/事实/恢复 +
  daemon/app/api/TUI/Swift 表面。
- Controller 修复：approved-resume partial-batch 冲突、approved-ask resume
  语义、executor worktree 自动预置（0700）。
- 跨客户端旅程 **PASS**：`/private/tmp/bw1-journey-final`
  （journey_id `7794d968-c8a8-46ba-9c1b-1a2e5bae81c7`）——真实执行
  （propose→ask→A4 批准→resume 执行），ToolExecutionProposed/Allowed/
  Completed + ApprovalDecided 事实齐全，`verify-bw1-cross-client-journey.sh`
  PASS。
- 全矩阵：Go build/vet/test 全 PASS；Swift 98 tests PASS。
