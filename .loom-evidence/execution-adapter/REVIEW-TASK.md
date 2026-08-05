# Contract Review Task (B-W1 / C-W1) — READ-ONLY

你是 Loom 项目的独立 Contract Reviewer（flash 评审）。

## 任务

对以下三份 Gate 1 文档做 fresh 独立评审（只读）：

1. `.loom-evidence/execution-adapter/B-W1-ADR.md`
2. `.loom-evidence/execution-adapter/B-W1-CONTRACT.md`
3. `.loom-evidence/execution-adapter/C-W1-CONTRACT.md`

## 背景

B-P1（execution permission pipeline）已验收（commit `f5b7009d`）。
B-W1 是"有界执行适配器"：模型提议工具调用 → `permissions.Evaluate` →
allow 才执行；Journal 事实 + Evidence；A4 批准组合；不变量 3 有界修订。
C-W1 是"生产化落地"：显式激活 preview/confirm/deactivate、
launchd/Application Support 原子安装回滚、fail-closed 恢复降级。

## 硬性要求

- 只读！绝对禁止修改、创建、删除任何文件，禁止 git add/commit/push，
  禁止运行任何写入命令。你只输出评审结论。
- 可阅读仓库代码核验契约引用：`internal/permissions`（model.go/evaluate.go/
  authority.go/replay.go）、`internal/rules`（authority.go 的
  PermissionApprovalInput/ApprovalDecided）、`internal/journal/store.go`
  （AppendBatchIfStreamHeads）、`internal/evidence/store.go`（Publish）、
  `internal/app/local_permission.go`、`internal/work/worker_execution.go`
  （CandidateWorktree/AttemptClaimed）、`cmd/loomd/product_daemon.go`
  （handler 路由模式）、`internal/localipc/protocol.go`（方法白名单）、
  `internal/tui/model.go`（Screen 路由）、
  `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`。

## 检查维度

- 契约一致性：ADR vs 契约 vs 现有代码。
- 安全性：fail-closed、deny/ask 零执行、幂等、越界、secret 卫生。
- 完整性：Journal 事实流、RED 矩阵可测性、owned files 闭合、停止条件。
- 可实施性：冻结 Go 符号与现有类型/模式是否吻合。
- UX 不降级约束。

## 输出格式

第一行：`VERDICT: PASS` 或 `VERDICT: FAIL`。
然后分 P0（必须修复才能冻结）/ P1（应修复）/ P2（建议）列出 findings，
每条给出文件 + 行号/位置 + 理由。
最后给"冻结建议"。

评审要严格（STRICT 契约评审）：不要放水，也不要编造不存在的问题；
所有引用必须是你在仓库里实际核验到的。
