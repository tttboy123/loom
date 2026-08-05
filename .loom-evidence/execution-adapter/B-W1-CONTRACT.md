# Gate 1 Exit Contract — B-W1 Bounded Execution Adapter

**Status**: FROZEN — EXECUTION AUTHORIZED（Product Owner "达成生产可用"指令 +
Controller 冷读自查；flash 独立评审为开放项，见 REVIEW-NOTES）
**Date**: 2026-08-05
**Baseline**: `f5b7009d`（B-P1 ACCEPTED）on `codex/loom-platform-slice2`
**Risk**: STRICT — 执行面打开、新 Journal facts、Evidence、权威边界扩展、
不变量 3 有界修订（ADR）

## 1. 唯一垂直 WorkItem

```text
B-W1 Bounded Execution Adapter
```

不拆分薄 WorkItem（不建 adapter-only/evidence-only/probe-only 单元）。包含：
`internal/execution` 执行适配器（Evaluate 门禁 + 沙箱执行 + Journal 事实 +
Evidence）、确定性探针驱动、A4 批准后的执行恢复、跨客户端旅程。不包含：
模型客户端/Provider 路由/key/agent loop（未来桥接接入方）；网络执行；
自动批准绕过 human lane；间接危险调用沙箱（A13，记录不阻塞）。

## 2. 产品结果

普通用户旅程：Job/Candidate 运行时，模型桥接（或确定性探针）提议一次工具
调用 → daemon 裁决 allow/ask/deny → allow 在候选 worktree 内真实执行并产生
Journal 事实 + Evidence → ask 进 Attention，批准后执行 → 全程可在
Timeline/Evidence 查看；deny/error 零执行且给原因。重启/重放后同一执行
不重复、不丢失。

## 2.5 客户端表面

- IPC 新增只读 `execution_snapshot` 与写 `execution_command`
  （action: `propose`；批准后以同一 operation 幂等重放，无独立 resume
  action）。
- TUI 新增 `ScreenExecution`（"Execution"）：列出最近执行（job、tool、
  verdict、exit code、evidence digest、时间），`p` 以确定性探针发起一次
  propose，`d` 详情，`r` 刷新；Attention 批准后 `p` 重放同一 proposal
  完成执行。不新建其它屏。
- 生产 Swift 客户端只读新增 `executionSnapshot`（严格解码，与 Go JSON
  对应），不新增写方法；Journey probe 只读消费同一快照。
- Timeline/Evidence 屏直接展示执行事实与 Evidence 摘要，不重复建模。

## 3. 精确语义

### 3.1 门禁（执行前置）

```text
Proposal{jobID, call}
  → EffectiveProfile（binding digest+generation 校验，失败=deny 零执行）
  → Evaluate
      allow  → 执行
      ask    → A4 批准；ApprovalDecided(approved) 后以同一 call digest 重放执行
      deny/error → 不执行，ToolExecutionDenied + typed denial
```

`PermissionDecisionRecorded` 仅用于 ask（沿 B-P1 语义：validate_call 只对
ask 记录决策事实）；deny/error 的决策事实就是执行流内的
`ToolExecutionDenied`，不跨流重复写。

### 3.2 执行器约束（fail-closed）

- 编辑执行：仅限 profile OwnedPaths ∩ Candidate worktree；补丁原子应用；
  无 apply 则不落盘。
- 命令执行：cwd=候选 worktree；环境仅注入白名单变量；默认无网络
  （禁 `--network`/代理环境）；超时（默认 60s，可配置上限 600s）；
  无隐藏重试；执行器异常 → 失败事实 + 零副作用声明。
- 命令执行后对有界 worktree 做变更检测：仅记录 regular file 相对路径 +
  SHA-256，文件数（默认 ≤10,000）与总大小（默认 ≤64 MiB）超限即
  `ToolExecutionFailed(limit_exceeded)`，绝不扫描符号链接目标或越出
  worktree；变更集为空时 `changed_files_digest` 为固定空集摘要。
- 链式命令：只执行 Evaluate=allow 的整串；危险段检查已前置（B-P1 RED #18/#20）。
- 输出与产物：stdout/stderr 大小上限；Evidence 只存 digest 与元数据，
  不存 raw secret；changed-files 按候选 worktree 相对路径。

### 3.3 Journal 事实（唯一权威）

流 `execution/<job_id>/<execution_id>`：
- `ToolExecutionProposed`（job_id, execution_id, call_digest, tool, command?, path?, proposed_at, generation）
- `ToolExecutionAllowed`（execution_id, verdict, allowed_at）
- `ToolExecutionDenied`（execution_id, denial{reason, authorization_path, rule_ids}, denied_at）
- `ToolExecutionCompleted`（execution_id, exit_code, output_digest, changed_files_digest, duration_ms, completed_at）
- `ToolExecutionFailed`（execution_id, reason, error_code, failed_at）

流内顺序为 Proposed → Allowed|Denied → Completed|Failed（ask 时 Proposed →
PermissionDecisionRecorded(ask) + ApprovalRequested；批准后以同一
execution_id 重放 Proposed/Allowed/Completed，绝不产生第二个执行）。
证据 Artifact 只发布 JSON 元数据（exit code、duration_ms、output_digest、
changed_files_digest、evidence_id），不发布 raw stdout/stderr。

幂等：execution_id 由 (job_id, call_digest, operation_id) 确定性生成；重放
同 operation 不产生重复执行；崩溃（CAS 前后）由既有 lease/generation 恢复
（执行前写 Proposed+Allowed，执行后写 Completed/Failed；重启后
`ReplayPending` 把 allowed-无-terminal 与 proposed-无-terminal 一律无副作用地
标记 `ToolExecutionFailed(interrupted)` 并附原因，绝不重执行）。

### 3.4 与 A4 批准组合

ask → `PermissionDecisionRecorded` + A4 `ApprovalRequested`；批准
（ApprovalDecided approved）→ 允许以同一 call digest 的 `ToolExecutionProposed`
执行一次；拒绝 → `ToolExecutionDenied`。批准本身不执行（仍需 allow 门禁）。
同一 operation 在 ask 未决议时重复 propose：幂等返回既有批准（approval_id），
不新建批准请求、不执行、不写重复事实。批准后 resume 必须满足三条件才执行：
① ApprovalDecided=approved 且 approval 的 call digest 与 proposal 一致；
② 重新 Evaluate 仍为 allow（profile/generation/mode/规则未变或仍放行）；
③ execution_id 尚无 terminal 事实。任一不满足 → 零执行 +
`ToolExecutionDenied`/`ToolExecutionFailed`（有因）。

## 4. 冻结 Go 符号（`internal/execution`，新包）

```go
type Proposal struct { JobID string; Call permissions.ProposedCall; OperationID, JourneyID string }
type ExecutionResult struct { ExecutionID, Verdict, ExitCode, OutputDigest, ChangedFilesDigest, EvidenceID string }
type Executor interface {
	Edit(context.Context, EditRequest) (EditResult, error)   // 原子补丁，OwnedPaths 内
	Run(context.Context, RunRequest) (RunResult, error)      // cwd=worktree，消毒环境，超时
}
type Adapter struct { store *journal.Store; projection func() *permissions.Projection; evidence *evidence.Store; executor Executor; now func() time.Time }
type ApprovalRequester interface {   // app 层注入：复用既有 rules 批准生命周期
	RequestPermissionApproval(context.Context, rules.PermissionApprovalInput) (rules.ApprovalRequestRecord, error)
}
type DecisionRecorder interface {    // app 层注入：ask 决策事实（B-P1 RecordDecision 语义）
	RecordDecision(context.Context, string, permissions.ProposedCall, permissions.Verdict, permissions.Denial, string, string, string) error
}
func NewAdapter(...) (*Adapter, error)
func (a *Adapter) Execute(ctx, Proposal) (ExecutionResult, error)
func (a *Adapter) ReplayPending(ctx) error                     // 崩溃恢复：pending allowed → failed(no re-exec)
func ReplaySnapshot(events []journal.Event) (ExecutionSnapshot, error) // 只读投影：未知 execution 事件拒绝
```

工作区解析由 daemon 注入（本 WorkItem 不实现全局 projection 扩展）：

```go
type WorktreeResolver interface {
	Resolve(context.Context, string) (string, error) // jobID → 最新 AttemptClaimed 的 candidate_worktree
}
```

执行器实现（`internal/execution/executor.go`）直接以
`EditRequest{Worktree, RelativePath, NewContentDigest, NewContent}` 与
`RunRequest{Worktree, Command, Timeout, OutputLimit}` 为冻结输入；
`NewAdapter` 接受 `WorktreeResolver` 端口，失败（无 attempt / worktree
越界/不存在）→ 零执行 + `ToolExecutionFailed`。
`NewAdapter` 同时接受 `ApprovalRequester` 与 `DecisionRecorder` 端口（ask 时
先写决策事实再创建批准；二者任一为 nil = forward-only，仍写 Proposed、
不创建批准）。

执行面：Adapter 只执行 `ToolEdit`/`ToolBash`；其它工具（Read/Grep/
WebSearch/MCPTool/WebFetch）由未来桥接自行处理，propose 即拒绝
（`ToolExecutionDenied(tool_not_executable)`），不产生副作用。

执行发生在 Evaluate=allow 后；deny/ask/error 由 `Execute` 返回 typed 结果且
零副作用。`ReplayPending` 只写事实，不执行。

## 5. RED-first 矩阵

1. allow → 恰好执行一次（幂等重放不重复）；执行前写 Proposed+Allowed。
2. ask → 零执行；批准后同一 call digest 执行一次；拒绝 → Denied 零执行；
   ask 未决议时重复 propose 幂等返回同一批准、零执行、零重复事实。
3. deny/error/stale generation/退役 profile → 零执行 + 决策事实。
4. 编辑越出 OwnedPaths/Candidate worktree → 拒绝，零落盘。
5. 命令 cwd 越出 worktree / 环境含敏感变量 / 超时 → failed 事实 + 零副作用声明。
6. 崩溃 CAS 前：无 Allowed 则重启不执行；CAS 后：Allowed 无 Completed → 标记
   failed 且不重执行。
7. 证据只含 digest/元数据，无 secret；输出超限截断并记录。
8. 双 Job 并行执行隔离（profile/owned paths 互不越界）。
9. 批准决议不执行（仍需 allow 门禁）。
10. 全量 Journal 重放重建执行投影；未知事件拒绝。

## 6. 验证矩阵

- Go full/race/vet/tidy/gofmt；STRICT fail-closed 与重启/重放证明。
- 确定性探针驱动真实执行（编辑 + 命令）跨客户端旅程，GUI 用已批准替代方法。
- Evidence/Journal 摘要、0700/0600、journey_id 贯穿、无 secret。
- Implementation / dual-Result / Whole-Candidate Review 独立 PASS
  （P0=P1=P2=0），flash 评审循环按既定方式执行。

## 6.5 Exact owned files（冻结范围）

### 新增

```text
internal/execution/model.go          // Proposal/ExecutionResult/EditRequest/RunRequest/EditResult/RunResult/Journal 载荷/WorktreeResolver
internal/execution/adapter.go        // Adapter.Execute/ReplayPending + 全部 Journal 事实 + 幂等/CAS + ask 批准组合
internal/execution/executor.go       // 沙箱 Executor：原子编辑 + 命令执行（cwd/环境/超时/输出上限/变更检测）
internal/execution/replay.go         // ReplaySnapshot：全量重放重建执行只读投影，未知事件拒绝
internal/execution/adapter_test.go   // RED 1-10 + A4 组合 + 崩溃恢复
internal/execution/executor_test.go  // 沙箱约束 RED（越界/消毒/超时/变更检测）
internal/execution/replay_test.go    // RED 10
internal/app/local_execution.go      // 产品服务：execution_snapshot/execution_command + worktree 解析 + Evidence 元数据
internal/app/local_execution_test.go
internal/api/local_execution.go      // API 别名（沿 local_permission.go 模式）
internal/tui/execution.go            // ScreenExecution 只读列表 + 确定性探针 propose
internal/tui/execution_test.go
apps/macos/Sources/LoomLocalAppCore/LocalExecutionModels.swift
apps/macos/Sources/LoomLocalAppCore/LocalExecutionViews.swift
apps/macos/Tests/LoomLocalAppTests/LocalExecutionModelsTests.swift
scripts/verify-bw1-cross-client-journey.sh
cmd/loomd/bw1_execution_wire_test.go
.loom-evidence/execution-adapter/B-W1-ADR.md
.loom-evidence/execution-adapter/B-W1-CONTRACT.md
.loom-evidence/execution-adapter/B-W1-PROGRESS.md（证据根）
```

### 修改（最小面）

```text
cmd/loomd/product_daemon.go           // 挂载 execution 服务 + handler 路由
internal/localipc/protocol.go         // execution_snapshot/execution_command 方法白名单
internal/tui/model.go                 // ScreenExecution 路由
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift  // executionSnapshot 只读
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift  // refreshExecutions 只读
apps/macos/Sources/LoomLocalAppUI/ContentView.swift        // Execution 只读视图挂载
apps/macos/Sources/LoomLocalAppContractProbe/main.swift    // 只读 execution probe 动作
internal/localipc/swift_contract_test.go // swiftc 探测文件清单补新模型文件
```

修改面之外的任何产品路径一律不动；`internal/permissions`、
`internal/rules`、`internal/work`、`internal/queue`、`internal/projection`
既有权威只被**消费/转发**，不重写其事件语义。owned files 变更必须走独立
评审的 bounded amendment。

## 7. 非目标

模型客户端/Provider/key/agent loop；网络执行；自动批准绕过；A13 间接危险
沙箱；覆盖用户 Skill；push/merge 远端。

## 8. 停止条件

身份不一致、未评审 authority/schema 扩展、任何独立 Review FAIL、dirty 无法
隔离、越出 owned files ⇒ 停止 HUMAN_REQUIRED。
