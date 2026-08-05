# B-P1 Amendment 2 — 批准生命周期接线（A4）

**Status**: IMPLEMENTED — 已获 Product Owner 授权（2026-08-05）；RED A4-1…A4-8
全绿，旅程 final11（journey_id `78cd45d2-15c2-4259-a0bb-fcaabe8d66c7`）含真实
批准段并 verify PASS
**Date**: 2026-08-05

## 目标

闭环契约 §2/§8 的 "approve 全流程"：`validate_call` 命中 ask → 创建批准请求
（复用既有 `internal/rules` 批准生命周期，不新建第二套）→ Job 进入
`waiting_approval` → 用户单键批准/拒绝 → 决议事实生效、attention 按 resolved
过滤 → Evaluate 随投影变化。

## 设计决策

1. **扩展 `internal/rules`，不新造批准权威**。新增权限批准的入口方法，
  复用同一 `ApprovalRequested`/`ApprovalDecided` 与
  `WorkItemApprovalPaused`/`WorkItemApprovalResolved` 事件、同一
  `internal/projection/approval.go` 投影与 `waiting_approval` 状态机。
  不引入第二套 ApprovalRequest 事实类型。
2. **语义映射**：权限 ask 是工具调用级批准，不是"启动 Run 的客户规则
   require_approval"。因此新增方法不要求客户 RuleSet 评估
   （permission 层已裁决 ask），但批准决议仍是 human 门禁（TUI a/x 由
   `resolved_by` 用户传入，daemon 只接受用户命令路由的决议）。
3. **WorkItemID = Queue JobID**：rules 的 work-item 流以 Queue Job ID 作为
   WorkItemID，`WorkItemApprovalPaused` 即 Job 的 `waiting_approval` 事实；
   权限投影经 `ApprovalView` 消费既有 approval 投影判定 pending/resolved。
4. **幂等与重放**：沿用 rules 的 deterministic event ID + CAS + idempotency；
   同一 approval（同一 job+call digest）只产生一个 pending；resolve 后不可覆盖。

## 新增/修改面（owned files bounded amendment）

### internal/rules

- `authority.go`：新增
  `RequestPermissionApproval(ctx, input PermissionApprovalInput) (ApprovalRequestRecord, error)`
  与
  `DecidePermissionApproval(ctx, approvalID, approvalDigest, decision, resolvedBy, correlationID) (ApprovalRequestRecord, error)`
  （decision 限 allow/deny；resolvedBy 非空即 human 通道）。
  `PermissionApprovalInput{JobID, CallDigest, Tool, Command, Path, Reason, RequestedAt, CorrelationID}`。
  复用 `approvalStream`/`workItemStream`/`approvalRequestedPayloadFor` 等既有
  私有构造，事件类型不变。
- `authority_test.go`：RED A4-1…A4-8。

### internal/permissions

- `ApprovalView` 接线：`Replay` 保持只读视图；产品服务把既有 approval 投影
  （rules 域）合并进 `PermissionAttention`，pending 才展示，resolved 过滤。

### internal/app / daemon / TUI / Swift

- `local_permission.go`：`validate_call` ask → 先写 `PermissionDecisionRecorded`，
  再经注入的 `ApprovalPort`（daemon 适配 rules 权威）创建批准请求；
  `resolve_approval` → 转发 rules `DecidePermissionApproval`；
  `PermissionAttention` 合并 pending 批准 + 决策事实。
- daemon：构造 `ApprovalPort` 适配器（持有 rules 权威），注入权限服务。
- TUI：`a`/`x` 走真实批准（带 approvalID/digest），`g` grant-always 不变；
  Attention 屏展示 pending 批准行。
- Swift：attention 模型同步 approval 字段（可选）。

## RED 矩阵（A4-1…A4-8，先红后绿）

1. ask → 产生批准请求事实（ApprovalRequested + WorkItemApprovalPaused，
   WorkItemID=jobID，状态 waiting_approval），且权限层只写决策事实。
   ✅ `TestA41…`（rules）+ projection 兼容测试
2. 同一 job+call 重复 ask → 同一 pending 批准（幂等，无双份）。
   ✅ `TestA42…` / `TestA46…`（重启）
3. resolve allow → ApprovalDecided(resolved, allow) + WorkItemApprovalResumed；
   投影 pending→resolved；attention 不再展示。✅ `TestA43…` + A4-8 服务层
4. resolve deny → ApprovalDecided(resolved, deny)；attention 不再展示；
   Evaluate 对同一 call 仍 ask（决策事实保留）。✅ `TestA44…`
5. 空 resolvedBy / 非法决议 → error；resolved 后再次 resolve → error。
   ✅ `TestA45…`
6. 重启重放：pending 批准不丢失不重复；resolved 不可回退。
   ✅ `TestA46…`
7. 权限层不写 Approval* 事件（复用 rules 事件为唯一批准权威）。
   ✅ `TestA41…` 断言 + daemon 接线 `TestBp1DaemonApprovalPortWiredEndToEnd`
8. TUI a/x 真实批准路径在跨客户端旅程中可执行并留下决议事实。
   ✅ final11 旅程（ApprovalRequested/ApprovalDecided 事实 + transcript）

## 实现要点（与设计一致）

- 权限 ask = 临时 `require_approval` RuleSet（`work_item/<jobID>`，approverRefs
  `approver:permission-owner`，24h，onTimeout reject）经既有 `ActivateRuleSet`
  激活；`Evaluate` 产出决策；批准请求/决议沿用既有
  ApprovalRequested/ApprovalDecided/WorkItemApprovalPaused/Resolved 事件与
  `internal/projection/approval.go` 投影（兼容测试 PASS）。
- Queue Job 的 rules work-item 记录首次 ask 时引导（WorkItemCreated+Assigned），
  之后复用 assigned 状态。
- `RequestApproval` 的 RequestedAt==operationTime 强校验与实时时钟冲突，
  故激活规则集后由 `RequestPermissionApproval` 直接写批准事件（单一
  operationTime），决议走既有 `DecideApproval`（authorizer 绑定
  permission-owner，human 门禁由 resolvedBy 非空 + daemon 命令路由保证）。
- daemon 以 `permissionAuthorizer` 实现 `CustomerAuthorizer`（仅 daemon 使用
  rules），`permission_approvals.go` 适配 ApprovalPort 注入权限服务。

## 停止条件

未评审 authority 扩展、任何独立 Review FAIL、owned files 越界、dirty 无法
隔离 ⇒ 停止 HUMAN_REQUIRED。实现走 RED-first + 全矩阵 + 新旅程（final10，
含真实批准段）+ flash 独立评审。

## A5（Controller 复核修复，已实现 — 2026-08-05）

授权后 Controller 冷读复核发现**跨流泄漏**：`PermissionAttention` 的
`pendingApprovals` 扫描全 Journal 的 `ApprovalRequested` 事实，未过滤"仅
permission ask 产生的批准"；且 `DecideApproval` 只对 `approved` 决议校验
actor 归属（`local-owner` vs `approver:permission-owner`），`rejected` 决议
无 actor 校验——权限 Attention 可能展示并"拒绝"其它 rules 流程（如受控
Mission 授权）的待批项。

修复（RED-first，A4-9/A4-10 先红后绿）：

- `internal/rules/authority.go`：新增稳定判别标记
  `PermissionApprovalProjectID = "permission"`（权限批准上下文四元组
  project/team/workpackage 均为此值）；`DecidePermissionApproval` 在转发
  `DecideApproval` 前重放批准流并校验 `context.ProjectID()`，非权限批准一律
  返回 `ErrCustomerAuthorizationDenied`（approved 与 rejected 均拦截）。
- `internal/app/local_permission.go`：`pendingApprovals` 解析
  `context.project_id`，仅收录 `PermissionApprovalProjectID` 的批准。
- 测试：`TestA49DecidePermissionApprovalRejectsForeignApproval`（rules 层，
  构造真实 `local-owner` 待批批准，approved/rejected 均拒绝且零决议事实）；
  `TestA410AttentionAndResolveArePermissionScoped`（app 层，同 Journal 并存
  权限批准 + 外来批准，Attention 只展示权限项，`resolve_approval` 拒绝外来
  批准 ID）。

全矩阵（Go build/vet/test、Swift build + 98 tests）保持 PASS；旅程 final12
（含真实批准段）verify PASS。

## A6（Controller 复核修复，已实现 — 2026-08-05）

继续冷读发现**跨会话复用缺陷**：permission RuleSet 按 Job 作用域且对同一 Job
的所有 ask 内容相同，但 `RequestPermissionApproval` 无条件调用
`ActivateRuleSet`（幂等仅在 correlation 相同时生效）。首个批准 resolved
（work item 回到 assigned）后，同一 Job 换 call digest + 新会话 correlation
再次 ask 会命中 `ErrRuleAuthorityConflict`——阻塞"多次 ask / 新会话续用同一
Job"的真实旅程。

修复（RED-first，A4-11 先红后绿）：

- `RequestPermissionApproval` 在激活前经 `readRuleSet` 检查该 Job 的
  RuleSet 流；同 digest 已激活则复用（不新增 `RuleSetActivated` 事实），
  digest 不同则 `ErrRuleAuthorityConflict`，未激活才 `ActivateRuleSet`。
  崩溃窗口（激活已落盘、批准事件未落盘）与跨会话二次 ask 均恢复。
- 测试 `TestA411SecondAskSameJobDifferentCorrelationReusesActivation`：
  同一 Job 两次不同 call + 不同 correlation，第二次必须成功且产生新
  pending 批准，`RuleSetActivated` 事实恒为 1。

全矩阵保持 PASS；旅程 final13（含真实批准段）verify PASS。
