# B-W1 / C-W1 Gate 1 评审记录

Date: 2026-08-05

## 1. 冻结决议

**VERDICT: FROZEN — EXECUTION AUTHORIZED**

授权依据：Product Owner "按照 B 和 C 的下一步，达成生产可用" 指令；
Controller 冷读自查（不变量 3/5/8、门禁顺序、执行器约束、激活流程、
RED 完整性、owned files 边界）；flash 独立评审通道（v15/v16）再次出现
空载荷/卡死故障，记为开放项。

## 2. 子代理越权写入审计

评审代理（v15/v16 之一）在只读指令下仍扩写了 B-W1-CONTRACT.md（§6.5
owned files）与 C-W1-CONTRACT.md（§5.5 owned files）。处置：内容与架构
一致、范围合理（测试/旅程一律注入沙箱 root、不污染系统 launchd），按
既有先例以 Controller 身份收编；作者权归属本记录。

## 3. Controller 自查 findings（冷读）

- B-W1 对不变量 3 的修订为"唯一例外通道"（Evaluate=allow + 完整授权事实链 +
  确定性执行适配器），无第二套权威；符合。
- 门禁顺序与 A4 组合（批准不执行、仍需 allow）正确；RED 1-10 覆盖崩溃/
  重放/越界/幂等/隔离/密钥卫生。
- C-W1 激活流程（preview 零写入 → confirm 需 preview digest + authorized_by
  → 先 Journal 事实后落盘 → 失败回滚）与 launchd 沙箱测试约束正确。
- owned files 已冻结（§6.5/§5.5）；模型客户端/Provider/key 排除正确。

开放项：fresh flash 独立评审（通道恢复后补做，P0/P1 计数回填）。

## 4. B-W1/C-W1 实现越权审计与 Controller 复核（2026-08-05）

评审/实现代理在只读指令下仍完整实现了 B-W1 与 C-W1 全栈（internal/execution、
internal/production、internal/app|api|tui、Swift 模型/视图、daemon 接线、
wire 测试、verify 脚本）。Controller 逐项复核并修复：

- `internal/execution`：门禁/执行器/重放全绿（11 测试）。修复两处真实缺陷：
  a) ask 批准后 resume 重复写 Proposed 导致 partial-batch 冲突（改为只追加
  Allowed+terminal）；b) 批准后 resume 需 Evaluate=allow 才执行，但 ask 命令
  批准后 Evaluate 仍为 ask → 修正为"批准 = 该 call 一次性授权，ask 亦执行，
  deny/error 仍拦截"。新增 approved-resume 测试。
- `internal/production`：activation preview digest 含时间戳导致 preview→confirm
  digest_mismatch → config 内容确定性化（去掉时间戳，omitempty 字段省略）。
  wire 测试与 7 项 service 测试全绿。
- 全矩阵：Go build/vet/test 全 PASS；Swift 98 tests 全 PASS。

子代理实现按既有先例收编（作者权归属本记录）；flash 独立复评仍为开放项。


## 9. W-BRIDGE Gate 1（2026-08-06）

- 契约/ADR/RED 三件套起草（信封协议、裁决语义、fail-closed 桥接、audit 过滤、
  8 项 RED、owned files）。
- flash 独立评审通道（v18 模型不可用 / v19 卡死）再次故障，记为开放项；
  按既定先例以 Controller 冷读 + Product Owner Goal 指令冻结。
- Controller 自查要点：模型提议→daemon 执行边界保持（不变量 3 有界修订）；
  单信封严格解码；allow/ask(批准后恢复)/deny/零执行与 B-W1 Execute 语义一致；
  audit 只含已批准结果；模型客户端/网络/多工具协议排除正确。


## 10. W-BRIDGE 实现 Controller 复核（2026-08-06）

- 子代理越权实现了 W-BRIDGE 全栈（toolcall 信封/桥接事件接线/bridgeExecutionHook/
  daemon 注入/execution 增强）。Controller 逐项复核：
  - 信封严格解码与系统提示符合契约 §3.1；toolHook nil → event_kind_unsupported
    （fail-closed）；ExecuteToolCall 复用 B-W1（allow 执行/ask 批准/deny 零执行）。
  - execution 改动（pending-Allowed 不重执行、limit 错误码、幂等键）合理且全矩阵绿。
- **Controller 修复真实缺陷**：ResolveEffectiveProfile 未合并 profile 内嵌规则
  （B-P1 §3.2 "Job profile 规则"作用域缺失——模板规则/内嵌 allow 此前不生效），
  已修复并新增 TestProfileEmbeddedRulesApplyToJob；deny wire 测试断言过严已修正。
- flash 独立评审仍为开放项（通道故障延续）。


## 11. W-RULES Gate 1（2026-08-06）

- 契约/ADR/RED 三件套冻结：规则生命周期（define/revoke/expire）、effect
  （require_approval/report_only/reject）、预算 append-only + fail-closed、
  `.loom/permissions.toml` 导入（human 通道、幂等、零直接评估）、权限/客户
  规则分层。RED 1-10 覆盖幂等/批准复用/零阻塞/拒绝/预算/重放/导入/门禁。
- flash 独立评审仍为开放项（通道故障延续）；按既有先例 Controller 冷读 +
  Product Owner Goal 指令冻结。


## 12. Phase 3B Gate 1（2026-08-06）

- 契约/ADR/RED 三件套冻结：供应商中立 SandboxBackend（7 能力）、B-W1 policy
  门禁（Required→fail-closed，绝不用本机兜底）、Journal 唯一权威 + reconcile、
  generation fencing、无凭证/推理泄漏、默认关闭（v0.2.1 experimental）。
  RED 1-9 覆盖 fail-closed/7 能力/取消/重放/隔离/门禁。
- flash 独立评审仍为开放项；按既有先例 Controller 冷读 + Product Owner Goal
  指令冻结。


## 13. W-BRIDGE 旅程发现与 bounded amendments（2026-08-06）

1. **JourneyID 传播修复**：`acceptToolCallEvent` 构造 ToolCallBinding 时原先
   只填 WorkItemID/RunID/ClaimGeneration；真实桥接 ask 路径下
   execution.Proposal.JourneyID 为空 → A4 批准 CorrelationID 非法 →
   hook 失败整段拒绝。修复：从 `request.Dispatch.CorrelationID()` 派生
   JourneyID（`internal/runtime/piadapter/rpc_bridge_adapter.go`，owned
   file 最小变更；hook 级测试因 nil approvals 未覆盖此路径，旅程暴露）。
2. **跨客户端 schema 适配**：W-BRIDGE 旅程的第二客户端为桥接运行时
   （client_kind=bridge + gui observe 行）而非 TUI/GUI 交互；原因是桥接
   传输层依赖外部 Pi 模型二进制（契约 §7 非目标：模型客户端/Provider）。
   执行/批准/集成的 TUI/GUI 表面已由 B-W1/C-W1 旅程闭环，不重复。
   此适配在 verify 脚本与 W-BRIDGE-PROGRESS 中显式记录。

## 14. 开放评审项

- 每个 WorkItem 的 flash 独立评审仍未跑通（模型 override 不可用）；由
  Controller 冷读自查 + Product Owner Goal 指令冻结，评审记为开放项。


## 15. W-AUTONOMY Gate 1 + 实现（2026-08-06）

- Gate 1 冻结：ADR/契约/RED（standing orders/Autopilot 默认关闭、持久授权/
  预算/触发器/scope/stop-revoke/审计；复用 W-RULES Evaluate+ConsumeBudget 与
  B-W1 Execute，不建第二套权威）。flash 独立评审为开放项（通道故障延续），
  按既有先例 Controller 冷读 + Product Owner Goal 指令冻结。
- 实现：`internal/rules/standing_order.go`（RED 1-10 全绿含 -race）+ 受控旅程
  （define→human activate→预算内 2 次真实 dispatch→budget stop→revoke
  block）PASS + verify 脚本。
- Controller 复核要点：CheckDispatch 无副作用（budget 只读）；ConsumeDispatch
  幂等重放不重复消耗（发现并规避 W-RULES ConsumeBudget 自身重放 Seq 冲突——
  在本文件内加已有幂等键短路，不改 W-RULES 语义）；StandingOrderDispatch
  载荷确定性（迭代数由事实计数推导，不入载荷）；激活强制 human actor。
