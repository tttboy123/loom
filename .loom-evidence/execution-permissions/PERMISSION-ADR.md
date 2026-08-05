# ADR: Execution Permission Pipeline（工具调用级授权）

**Status**: PROPOSED — Gate 1 冻结候选；待独立 Contract Review PASS 后 ACCEPTED；
未冻结前不进入产品代码
**Date**: 2026-08-05
**Applies to**: 下一阶段 B（有界本地执行适配器）与 C（生产化落地）的 Gate 1 前置
**Type**: Architecture Decision Record

## Context

当前 Loom 的执行边界是"模型桥接系统提示硬编码 `Use no tools`"
（`internal/runtime/piadapter/rpc_bridge_adapter.go:46`）。这意味着执行面被整体关闭，
而不是被授权。B 要像 Codex 一样真实改代码，就必须打开工具调用；打开后，
当前领域层治理（claim/lease/generation fencing、`waiting_approval`、七类失败、
只读 Reviewer、单写 Integrator）能管理 WorkItem/Attempt 级生命周期，但无法回答：
一次工具调用（读文件、写文件、跑命令、访问网络）是否被允许。

grok-build（以及 Claude Code、Codex 等主流 agent-platform）采用同构的权限模型：
permission mode 基线 + `deny > ask > allow` 规则 + hooks + 只读白名单 +
记住的 grants + 管理员锁，规则按全局/项目/个人分层。这套模型已被生态验证，
用户明确要求移植它作为 Loom 的权限基线，再叠加 Loom 自研能力，且不以牺牲
用户体验为代价。

## Decision

### 1. 移植 grok-build 权限模型作为行业标准基线

Loom 执行器获得与主流 agent-platform 同构的工具调用授权管道，语义对齐
（移植设计语义，不复制代码）：

- **Permission modes**：`default(ask)`、`acceptEdits`、`auto`、`dontAsk`、
  `bypassPermissions(always-approve)`。模式只设基线；规则永远在模式之上生效。
- **规则动作**：`deny` > `ask` > `allow`，任意来源合并后按严重度裁决，
  `deny` 跨作用域优先，不可被项目 `allow` 覆盖。
- **工具类别**：`Bash`、`Read`、`Edit`、`Grep`、`MCPTool`、`WebFetch`、
  `WebSearch`；路径 glob（`*`/`**`）、命令前缀与分段解析、危险命令表
  （`rm`、`chmod`、`git push` 等强制重提示）。
- **只读白名单**：只读工具与只读 shell 命令（`ls`/`cat`/`git status|log|diff`
  等）默认自动放行；`tee`、编译型检查等不在白名单。
- **记住的 grants**：per-project/per-candidate 作用域；危险命令不参与前缀记忆。
- **管理员锁**：root 级 `disable_bypass_permissions_mode`，普通配置不可覆盖。

### 2. Loom 自研增强（不变量约束下的改造）

1. **授权事实进 Event Journal，配置文件只是投影。** PermissionProfile、
   PermissionRule、PermissionGrant、PermissionActivation、ApprovalRequest
   全部是 versioned Journal facts；`.loom/permissions.toml` 与 daemon 落盘
   配置是由 Journal 可重建的 materialized 投影，不是第二权威（不变量 5）。
2. **Per-Job/Candidate 权限画像。** 每个 Queue Job/Candidate 绑定一个
   permission profile（继承项目 policy 后收窄）；profile 带 digest + generation，
   复用现有 lease/fencing：旧 generation 或已撤销 profile 的调用一律拒绝。
3. **确定性 fail-closed 校验器，不抄 fail-open hooks。** grok-build 的
   PreToolUse hook 失败默认放行，作为安全边界是弱点。Loom 在 daemon 内用
   编译进去的确定性 Go 校验器做 pre-tool-use 检查：策略缺失 → 只读兜底或拒绝，
   校验器异常 → 拒绝，绝不放行。
4. **管理员锁 = 不变量 8 的实现。** root policy 的 writer 只能由 human-authorized
   通道持有；在线自演化（Agent/Harness/Sidecar 输出）不得修改 root policy。
5. **与现有规则引擎组合，不新建第二套。** `ask`/`require_approval` 类命中复用
   既有 `waiting_approval` 暂停与 ApprovalRequest 生命周期（TECH-PLAN §8.2/§9、
   `internal/rules`、`internal/projection/approval.go`、
   `internal/work/run_authority.go`）；权限层只记录 ask 决策事实，**不新建第二套
   Approval 生命周期**；工具层裁决是领域层规则引擎之下的前置闸门。
6. **OS 沙箱作为独立兜底层。** 权限管"允许请求什么"，沙箱管"进程实际能做什么"
   （Phase 3B 实验后端），两者独立叠加，不互相替代。

### 3. 用户体验是一等约束（不降级 + 新产品能力）

- 常见只读旅程（看板/队列/状态/时间线）零提示；ask 只出现在画像外动作。
- ask 提示支持单键 approve / reject / always-allow；"整个 Candidate 内允许编辑"
  的一次性批量授权；危险命令每次都重提示。
- 激活/授权前展示精确 permission diff（将改变什么、维持什么、最保守默认）；
  被拒动作给出原因与可执行的授权路径，而不是冷拒绝。
- 新增产品能力：Permission Explorer（某个 Job/Candidate 的有效权限、为什么被
  拦、如何授予）、Attention Inbox 聚合待批项不阻塞、策略变更 canary、
  授权摘要进 Timeline/Evidence。

## Alternatives considered

- **维持 `Use no tools` 不变**：B 无法成立，拒绝。
- **仅 WorkItem 粒度批准**：太粗，表达不了路径/命令/工具级边界，并行
  Candidate 无法独立收窄权限，拒绝。
- **直接抄 grok-build 静态配置文件 + fail-open hooks**：引入第二权威与
  fail-open 信任面，违反不变量 5/8，拒绝。
- **自研全新权限模型**：与生态不兼容、学习成本高、验证成本高；用户明确要求
  移植行业标准基线，拒绝。

## Consequences

- 正面：工具调用面在动作发生前被裁决；并行 Candidate 各自持最小权限；
  全量授权事实可重放重建；管理员锁满足不变量 8；与主流 agent-platform
  心智模型一致。
- 代价：执行器接入需要新增一条 pre-tool-use 校验路径与一组 Journal 事实；
  B 的执行适配器必须携带 profile 且通过校验器才能调工具；C 的 daemon
  激活流程必须展示 permission diff 并保留管理员锁。
- 边界：本 ADR 只冻结权限管道本身；B（有界执行适配器）与 C（生产化落地）
  各自仍是后续垂直 WorkItem，消费本管道。
