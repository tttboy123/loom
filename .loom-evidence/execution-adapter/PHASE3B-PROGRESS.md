# Phase 3B 受治理沙箱 — PROGRESS

**Status**: 实现完成（RED 1-9 绿 + canary PASS + wire 全绿）— 待独立评审与
Product Owner 验收确认（flash 独立评审为开放项，见 REVIEW-NOTES.md）
**Contract**: `PHASE3B-CONTRACT.md`（FROZEN @ `8bcaa766`）
**Release target**: `v0.2.1 experimental`（默认关闭）

## 已交付

### 1. 供应商中立接口（冻结契约 §3.1）

- `internal/sandbox/backend.go` — `SandboxBackend` 7 能力
  （Create/Exec/Cancel/Pause/Resume/Destroy/InspectCapabilities）+
  `CreateRequest`/`ExecRequest`/`Instance`/`ExecResult`/`Capabilities`。
- `internal/sandbox/loopback.go` — 受控 spike 后端：每实例独立临时工作区
  （0700）、消毒 env（无继承变量/凭证）、超时、真实 SIGSTOP/SIGCONT、
  Kill/Destroy 清理。默认关闭，仅 env 显式挂载。

### 2. B-W1 policy 门禁（契约 §3.2）

- `internal/execution/sandbox_gate.go` — `SandboxPolicy{Required,Backend}` +
  `SandboxGate` 接口 + 默认 `BackendGate`（policy 函数 + InspectCapabilities
  可用性判定；后端名不匹配或能力缺失 → 不可用）。
- `internal/execution/adapter.go` — `executeApproved` 内门禁：
  Required 且后端不可用/策略错误 → `Deny`（零副作用，绝不本地兜底）；
  无 gate（默认）→ 本机 `SandboxExecutor` 行为不变。

### 3. 状态权威与 reconcile（契约 §3.3）

- `internal/sandbox/reconcile.go` — `Controller`：
  - Journal 事实流 `sandbox/<instance_id>`：Created/ExecRequested/
    ExecCompleted/ExecFailed/Cancelled/Paused/Resumed/Destroyed；
  - 幂等重放：同 operation 已有终态事实 → 短路返回，绝不重复执行；
  - generation fencing：stale 实例操作拒绝（`ErrStaleGeneration`）；
  - `Rebuild` 从 Journal 投影（读模型，非第二权威）；
  - `Reconcile` 清理未 Destroy 实例（先后端 Destroy，成功才落事实；
    失败留待下次重试；绝不重执行）；
  - `EnvDigest`：排序 key=value 的 sha256，无原始敏感值入 Journal/证据。

### 4. 产品挂载（契约 §5.1 修改面）

- `cmd/loomd/product_daemon.go` — `LOOM_SANDBOX_BACKEND=loopback` 显式
  挂载 + `LOOM_SANDBOX_REQUIRED=1` 启用 Required（实验开关）；无 env →
  gate=nil → 默认关闭，行为与 B-W1 完全一致。
- `cmd/loomd/sandbox_wire_test.go` — 组合级证据：默认关闭本地执行、
  Required 不可用 fail-closed、loopback 挂载后 allow 路径执行。

### 5. 受控 canary（契约 §5）

- `cmd/sandbox-canary/main.go` — 确定性旅程：隔离执行、无凭证泄漏、
  generation fencing、取消终止、reconcile 清理、Journal 权威；
  产出私有权限旅程证据。
- `scripts/verify-phase3b-canary.sh` — 校验权限/journey id/sqlite 完整性/
  事实类型/无残留工作区/无凭证材料/全 PASS 标记。

## RED 矩阵 ↔ 证据

| RED | 场景 | 证据 |
|---|---|---|
| 1 | Required+不可用 → fail-closed 零副作用 | `execution/sandbox_gate_test.go` TestRedSB2/2a；`sandbox_wire_test.go` |
| 2 | 7 能力状态事实原子落 Journal | `sandbox/reconcile_test.go` TestRedSB2 |
| 3 | Exec 失败/取消 → Failed/Cancelled；无重复副作用 | TestRedSB3 + 幂等短路 |
| 4 | stale generation 拒绝 | TestRedSB4 |
| 5 | 后端回调非权威；Journal 重建一致 | TestRedSB5 |
| 6 | 重启/重放未 Destroy 实例清理、不重执行 | TestRedSB6 + canary reconcile-cleanup |
| 7 | 无凭证泄漏（env digest） | TestRedSB7 + loopback 测试 + canary no-secret-leak |
| 8 | 双 Job 隔离 | TestRedSB8 |
| 9 | InspectCapabilities 门禁 | BackendGate（能力/名称校验）+ TestRedSB2b + wire |

## 验证矩阵（本阶段执行）

- [x] `go build ./...` / `go vet ./...` / `go test ./...`
- [x] `go test -race`（sandbox/execution）
- [x] `gofmt -l` 干净
- [x] canary + verify 脚本 PASS（`/private/tmp/phase3b-canary.*`）
- [ ] flash 独立评审（开放项）

## 开放项（非阻断，记录待办）

1. 跨客户端 TUI/GUI 旅程：Phase 3B 默认关闭且 daemon 侧为实验 env 开关；
   把 sandbox 状态/策略面板接入 TUI/原生 app 的受控旅程（连同 per-Job
   profile 解析）放入 W-AUTONOMY 产品主线（Queue→Develop→Test→Review→
   Integrate→Publish）一并验收。
2. per-Job `SandboxPolicy` 从 Job profile 解析（B-P1 profile 扩展 bounded
   amendment）：当前实验开关为全局 env；profile 级解析作为产品化细化项。
3. 后端选型 spike（AgentENV/CubeSandbox/E2B 选一）：loopback 为受控夹具，
   生产后端经受控 spike 后再定，同一版本最多一个后端。

## 停止条件检查

- 身份一致：本 PROGRESS 仅记录当前 checkout（`codex/loom-platform-slice2`）。
- Journal 唯一权威：后端仅执行，无第二批准/规则/状态权威。
- 默认关闭：无 env 时零行为变更（wire 测试覆盖）。
- 无凭证/推理泄漏面：canary secret 扫描通过。
