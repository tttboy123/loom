# Gate 1 Exit Contract — W-BRIDGE 模型桥接接入执行适配器

**Status**: FROZEN — EXECUTION AUTHORIZED（Product Owner Goal 指令：完成 Phase 3 全部扩展任务 + Controller 冷读自查；flash 独立评审为开放项）
**Date**: 2026-08-06
**Baseline**: `f69b49d9` on `codex/loom-platform-slice2`
**Risk**: STRICT — 桥接执行面打开、新协议、authority 边界（复用 B-W1）、
transcript audit 扩展

## 1. 唯一垂直 WorkItem

```text
W-BRIDGE Model Bridge Tool-Call Adapter
```

不拆分薄 WorkItem。包含：桥接系统提示/协议启用、信封解析与校验、daemon 接线
（bridge→execution.Adapter）、工具结果/denial 回传、transcript audit 扩展、
跨客户端旅程。不包含：模型客户端/Provider 路由/key；多工具自由协议；网络执行；
自动批准绕过。

## 2. 产品结果

真实模型（Pi Runtime）在受控旅程中提议一次工具调用：daemon 按权限管道裁决
allow/ask/deny——allow 真实执行并回传结果，ask 暂停走批准后恢复，deny 回传
原因且零执行。全程 Journal 事实 + Evidence；transcript 只含已批准调用的结果。

## 3. 精确语义

### 3.1 信封协议（冻结）

```text
ProposedToolCall = {"job_id": string, "call": {"tool": "Bash"|"Edit"|"Read"|"Grep",
                     "command"?: string, "path"?: string}}
```
- 单信封 = 单工具调用；禁止嵌套、脚本注入、多余字段（严格解码拒绝未知键）。
- 模型输出中 `toolcall_start/delta/end` 事件按既有诊断流解析；只有完整信封
  进入执行裁决。
- 系统提示明确：信封之外不得包含隐藏推理/凭据/文件系统路径；结果由 daemon
  回传，模型不得自证执行。

### 3.2 裁决与执行（复用 B-W1）

```text
信封 → 绑定 Run/Job generation → execution.Adapter.Execute
  allow → 沙箱执行 → 结果 + Evidence 回桥接（transcript audit 仅存已批准结果）
  ask   → A4 批准（Job waiting_approval）；ApprovalDecided(approved) 后
          同一 call digest 恢复执行一次；rejected → denial 回模型
  deny/error/stale → typed denial 回模型，零执行
```

### 3.3 桥接约束（fail-closed）

- 信封解析失败/未知工具/字段非法 → 拒绝回错误，不执行、不落执行事实。
- 桥接不含执行决策；模型提议只是 Proposal。
- transcript audit：只记录已批准执行的结果摘要与 Evidence digest；ask/deny
  只记 decision 事实，不把完整 denial 泄露给未授权观察者之外。
- generation 围栏：桥接绑定 Run/Attempt generation；stale 拒绝。

## 4. 冻结符号

```go
// internal/runtime/piadapter 扩展
type ToolCallEnvelope struct { JobID string; Call permissions.ProposedCall }
func DecodeToolCallEnvelope(line []byte) (ToolCallEnvelope, error)  // 严格解码+未知键拒绝
func ToolCallSystemPrompt() string                                  // 新系统提示（冻结常量）
// cmd/loomd 接线
type bridgeExecutionHook struct { adapter *execution.Adapter }      // 桥接事件→Execute
```

## 4.1 Owned Files（冻结）

- 修改：`internal/runtime/piadapter/rpc_bridge_adapter.go`、
  `internal/runtime/piadapter/rpc_bridge_adapter_test.go`（启用工具面需更新
  既有 `--no-tools`/system-prompt 断言；行为变更本身受 RED 5 覆盖）。
- 新增：`internal/runtime/piadapter/toolcall.go`、
  `internal/runtime/piadapter/toolcall_test.go`（信封解析/系统提示/RED 1-8
  单元测试）。
- 修改：`cmd/loomd/execution_wiring.go`（新增 bridgeExecutionHook 接线与
  daemon 组装），新增 `cmd/loomd/wbridge_toolcall_wire_test.go`。
- 新增：`scripts/verify-wbridge-cross-client-journey.sh`（跨客户端旅程证据
  门，遵循 B-P1/B-W1 同款 schema：journey_id 零漂移、双客户端 IPC、Journal
  事实集断言、权限卫生、postflight 清理）。
- 证据：`.loom-evidence/execution-adapter/W-BRIDGE-*`（FREEZE-RECORD、
  RED-SPEC、PROGRESS、REVIEW-NOTES、ACCEPTANCE）。
- 不触碰：`internal/permissions`、`internal/execution` 现有语义（仅消费）；
  `internal/rules` 批准权威（仅经既有 A4 路径转发）；TUI/Swift 现有 UI 面
  （Execution/Attention 屏已覆盖执行与批准展示，仅当旅程证据需要时做只读
  微调并记录）。

## 5. RED-first 矩阵

1. 启用工具面后，完整信封 → Evaluate=allow → 沙箱执行一次 + 结果回桥接。
2. 信封 ask → 零执行 + A4 批准；批准后同一 call digest 恢复执行一次；
   rejected → denial 回模型零执行。
3. deny/stale/未知工具/解析失败 → denial/error 回模型，零执行零事实。
4. 多余字段/嵌套/脚本注入信封 → 拒绝（严格解码）。
5. 系统提示与信封协议一致性（无 hidden reasoning/credential 指令存在）。
6. transcript audit 只含已批准结果；ask/deny 不写完整内容。
7. 桥接绑定 generation；stale 拒绝不执行。
8. 双 Job 并行桥接隔离；幂等重放不重复执行。

## 6. 验证矩阵

Go full/race/vet/tidy/gofmt；真实 Pi 桥接（受控夹具）驱动真实执行旅程 +
A4 批准 + denial 路径；跨客户端旅程 + flash 独立评审；原子提交。

## 7. 非目标

模型客户端/Provider/key/agent loop；网络执行；多工具自由协议；自动批准绕过；
覆盖用户 Skill；push/merge 远端。

## 8. 停止条件

身份不一致、未评审 authority/schema 扩展、任何独立 Review FAIL、dirty 无法
隔离、越出 owned files ⇒ 停止 HUMAN_REQUIRED。


## 6.1 Exact owned files

### 新增

```text
internal/runtime/piadapter/toolcall.go        // ToolCallEnvelope/DecodeToolCallEnvelope/ToolCallSystemPrompt
internal/runtime/piadapter/toolcall_test.go   // RED 1-8（严格解码/注入/未知键）
cmd/loomd/bridge_execution.go                 // bridgeExecutionHook：桥接事件→execution.Adapter
cmd/loomd/bridge_execution_test.go            // daemon 接线 + 旅程探针
scripts/verify-wbridge-cross-client-journey.sh
.loom-evidence/execution-adapter/W-BRIDGE-PROGRESS.md
```

### 修改（最小面）

```text
internal/runtime/piadapter/rpc_bridge_adapter.go  // 启用工具面：新系统提示 + toolcall 事件接线
internal/runtime/piadapter/rpc_bridge_adapter_test.go
cmd/loomd/product_daemon.go                       // 注入 bridgeExecutionHook
internal/localipc/protocol.go                     // 如需 bridge 状态只读方法
apps/macos/LocalIPCClient.swift                   // 只读 bridge/execution 状态（如旅程需要）
```

`internal/execution`、`internal/permissions`、`internal/rules` 只被消费；
不重写其语义。owned files 变更走独立评审 bounded amendment。
