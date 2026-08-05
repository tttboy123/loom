# Gate 1 Exit Contract — Phase 3B 受治理沙箱

**Status**: DRAFT — 待 flash 独立评审 PASS 后冻结；冻结前不改产品代码
**Date**: 2026-08-06
**Baseline**: `46d522ff` on `codex/loom-platform-slice2`
**Risk**: STRICT — 执行隔离面、policy fail-closed、后端 reconcile

## 1. 唯一垂直 WorkItem

```text
Phase 3B Governed Sandbox Backend
```

包含：`internal/sandbox` 供应商中立接口、B-W1 policy 门禁（SandboxPolicy
Required→fail-closed）、受控 spike 选一后端适配、Journal reconcile 与
generation fencing、canary。不包含：双后端；网络/付费；自动批准绕过；
后端作为状态权威。

## 2. 产品结果

Job/Candidate 可声明"需要沙箱"：执行时若后端不可用则拒绝（fail-closed，
绝不用本机兜底）；后端执行可取消/暂停/恢复/销毁；Loom 从 Journal 重建并
清理后端实例；重启/重放无重复副作用；无凭证泄漏。

## 3. 精确语义

### 3.1 SandboxBackend 接口（冻结）

```go
// internal/sandbox（新包）
type SandboxBackend interface {
	Create(context.Context, CreateRequest) (Instance, error)
	Exec(context.Context, ExecRequest) (ExecResult, error)
	Cancel(context.Context, string) error
	Pause(context.Context, string) error
	Resume(context.Context, string) error
	Destroy(context.Context, string) error
	InspectCapabilities(context.Context) (Capabilities, error)
}
type CreateRequest struct{ JobID, RunID string; Generation int64; WorkspaceDigest string }
type ExecRequest struct{ InstanceID string; Command string; Timeout time.Duration; EnvDigest string }
```

### 3.2 Policy 门禁（接入 B-W1）

- `SandboxPolicy{Required bool, Backend string}`（Job profile 可携带，B-P1
  profile 扩展 bounded amendment）。
- Required=true 且后端不可用/身份校验失败 → 拒绝执行（fail-closed，无副作用）。
- Required=false → 本机 SandboxExecutor（默认）。

### 3.3 权威与 reconcile

- Journal 事实：SandboxInstanceCreated/ExecRequested/ExecCompleted/ExecFailed/
  Cancelled/Paused/Resumed/Destroyed（流 `sandbox/<instance_id>`）。
- 后端回调/checkpoint/session 不是权威；Loom 从 Journal 重建并 reconcile。
- 实例绑定 Job/Run generation；stale 拒绝。
- 不传 Provider 原始凭证、raw Grant、隐藏推理、超出 Run 的 workspace。

## 4. RED-first 矩阵

1. Required 且后端不可用 → 拒绝执行，零副作用。
2. Create/Exec/Cancel/Pause/Resume/Destroy 各状态事实原子落 Journal。
3. Exec 失败/取消 → Failed/Cancelled 事实；无重复副作用。
4. stale generation 的实例操作拒绝。
5. 后端回调不写权威；Journal 重建后 reconcile 一致。
6. 重启/重放：未 Destroy 实例按 Journal 标记并清理（不重复执行）。
7. 无凭证泄漏：env digest 不含敏感值；workspace 仅绑定 Run。
8. 双 Job 隔离（实例绑定 generation/workspace）。
9. InspectCapabilities 决定 policy 可行性（Required 时不可用→拒绝）。

## 5. 验证矩阵

Go full/race/vet/tidy/gofmt；受控 spike 后端（loopback 夹具）canary：
隔离执行、取消、重启恢复、无凭证泄漏；跨客户端旅程 + flash 独立评审；
原子提交。默认关闭（无 policy 时不改变本机执行）。

## 6. 非目标

双后端；网络/付费；自动批准绕过；后端为状态权威；覆盖用户 Skill。

## 7. 停止条件

身份不一致、未评审 authority/schema 扩展、任何独立 Review FAIL、dirty 无法
隔离 ⇒ 停止 HUMAN_REQUIRED。
