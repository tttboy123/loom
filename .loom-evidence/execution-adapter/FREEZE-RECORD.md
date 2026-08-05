# B-W1 / C-W1 Gate 1 Freeze Record

Date: 2026-08-05
Baseline: `f5b7009d`（B-P1 ACCEPTED）on `codex/loom-platform-slice2`

## Authority

Product Owner 方向指令："按照B和C的下一步，达成生产可用"（本会话原始
消息：`按照B和C的下一步，达成生产可恶用`），即授权推进 B-W1（有界执行
适配器）与 C-W1（生产化落地）至生产可用，并按 Gate 节奏冻结契约、RED-first
实现、验证与评审。

## Frozen documents

- `.loom-evidence/execution-adapter/B-W1-ADR.md`（不变量 3 有界修订）
- `.loom-evidence/execution-adapter/B-W1-CONTRACT.md`（含 owned files /
  RED 1-10 / 停止条件）
- `.loom-evidence/execution-adapter/C-W1-CONTRACT.md`（含 owned files /
  RED 1-8 / 停止条件）

## Controller contract review（独立评审开放项）

外部 fresh flash 评审通道在本会话再次失效：两次 spawn + 两次 follow-up
均收到空载荷（子代理回复"未收到任务"），与 B-P1 期间记录的同一故障模式
一致。Controller 以代码为据完成自查，发现并修复 3 处必须冻结前修正的缺陷：

1. B-W1 §3.1 deny 语义与 B-P1 冲突（原写 PermissionDecisionRecorded，实际
   B-P1 validate_call 只对 ask 记录决策事实）→ 改为 ToolExecutionDenied。
2. B-W1 §3.4 缺少 ask 未决议时重复 propose 的幂等语义（会重复创建批准）→
   冻结"幂等返回既有批准"；批准后 resume 三条件（call digest 一致 +
   重新 Evaluate=allow + 无 terminal 事实）。
3. C-W1 停用流程缺少 preview digest/authorized_by 门禁 → 拆为
   deactivation_preview / deactivation_confirm。

其余 P1/P2 级自查项（ReplayPending 对 proposed-无-terminal 也标记 failed、
RED 10 需 replay.go、Adapter 需 ApprovalRequester 端口）已一并写入契约。

开放项（不阻塞实现，通道恢复后补做）：fresh 独立 Contract Review PASS
记录。

## 冻结后行为

- 产品代码零改动直到 RED 阶段开始。
- B-W1 先行实现与提交，C-W1 随后；各自独立 RED + 验证矩阵 + 跨客户端
  旅程 + 独立实现评审（通道恢复后补） + 原子提交。
- 真实 `~/Library/Application Support/Loom` 与用户级 launchd 写入仅在
  C-W1 最终激活时由用户对 diff 显式确认后进行；测试与旅程一律使用注入的
  沙箱 root。
