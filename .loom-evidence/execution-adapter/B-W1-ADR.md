# ADR: Bounded Execution Adapter（不变量 3 的有界修订）

**Status**: PROPOSED — 待 flash 独立评审 + Product Owner 冻结
**Date**: 2026-08-05
**Baseline**: `f5b7009d`（B-P1 ACCEPTED）on `codex/loom-platform-slice2`

## Context

B-P1 已交付工具调用级授权管道（mode/规则/白名单/危险表/grants/管理员锁/
per-Job profile + generation 围栏 + A4 批准生命周期），但模型桥接仍是
`Use no tools`：授权能力没有任何执行面可挂。B-W1 要打通"模型提议工具调用 →
Evaluate → allow 才执行"的通道，这是对不变量 3（模型输出永远不是执行权威）
的**有界修订**。

## Decision

### 1. 修订不变量 3（有界）

原文：Agent/Harness/Sidecar/模型输出是 Proposal 或 Candidate，永远不是执行
权威。

修订（唯一例外通道）：模型输出仍不是执行权威；**只有**经过
`permissions.Evaluate` 返回 `allow` 且存在完整授权事实链（profile binding +
generation + 规则/授权 + 必要批准）的提议，才由**执行适配器**执行。执行
适配器是 daemon 内的确定性组件，不是模型；每次执行都是 Journal 事实 +
Evidence。`ask`/`deny`/error 一律不执行。

### 2. 架构（单权威，无第二执行器）

```text
模型桥接/探针 → ToolProposal{jobID, call}
  → permissions.ResolveEffectiveProfile(projection, jobID)  // generation 围栏
  → permissions.Evaluate(effective, call)
       allow → 执行适配器执行（candidate worktree / 沙箱）
       ask   → A4 批准生命周期（批准后 allow 才执行）
       deny/error → 不执行，typed denial + 决策事实
  → ToolExecutionProposed/Allowed/Denied/Completed/Failed Journal 事实
  → Evidence（命令 digest、exit code、输出 digest、changed-files digest，无 secret）
```

约束：
- 执行适配器（`internal/execution`）不含模型客户端、Provider 路由、key 或
  agent loop；模型桥接是未来接入方，本 WorkItem 用确定性探针驱动。
- 每次执行绑定 Job/Candidate 的 profile digest + generation；旧 generation
  或撤销的 profile 在下一调用拒绝（复用 fencing）。
- 编辑类执行仅限 Candidate OwnedPaths；命令执行限定 cwd=候选 worktree、
  环境消毒、默认禁网、超时、无隐藏重试。
- 全部事实进 Event Journal（append-only），Evidence 进 Artifact Store；
  无第二套权威。

## Alternatives considered

- **维持 Use no tools**：B 无法成立，拒绝。
- **模型直连执行器**：绕过 Evaluate 与 Journal 事实链，违反不变量 3/5，拒绝。
- **第二套执行权威**：违反"单一 Journal/权威"，拒绝。

## Consequences

- 正面：授权管道获得真实执行面；每次执行可审计、可重放、可拒绝；并行
  Candidate 仍隔离。
- 代价：执行面打开带来真实副作用；B-W1 必须证明"deny/ask/error 零执行"
  与"allow 恰好执行一次"（幂等），并接受沙箱仍有限（间接危险留 A13）。
