# B-P1 RED Spec（规划：RED 1-17 → 测试函数映射）

Date: 2026-08-05

本文件是规划文档，不是产品代码。RED 阶段按此映射先写失败测试（编译目标见
契约 §4），再实现 `internal/permissions`。每个 RED 项必须先在当前基线证明
失败（缺失符号编译失败或断言失败），实现后转绿；17/17 全绿且独立评审 PASS
后进入旅程与验收。

| RED | 场景 | 测试函数（authority_test.go 除非注明） | 断言要点 |
|---|---|---|---|
| 1 | deny 覆盖 allow（跨作用域；root deny 不可被项目 allow 覆盖） | `TestRed01_DenyBeatsAllowAcrossScopes` | 项目 allow + root deny 同 pattern → deny |
| 2 | deny 覆盖 always-approve | `TestRed02_DenyBeatsBypass` | bypass 有效 + deny 规则 → deny，无副作用 |
| 3 | ask 命中 → 不执行 + 既有 rules ApprovalRequest + 权限层只写决策事实 | `TestRed03_AskCreatesDecisionAndReusesApproval` | Evaluate=ask；Authority.RecordDecision 写事实；approval 由 rules 权威创建，权限层无批准事件 |
| 4 | 只读白名单零提示；`tee`/`go test` 不在白名单；默认模板 allow 使测试旅程零提示 | `TestRed04_ReadOnlyWhitelistZeroPrompt`（evaluate_test.go）+ `TestRed04_DefaultTemplateCoversDevCommands`（compile_test.go） | default 下白名单 allow；tee/go test 无规则时 ask；模板含 go test/go build/gofmt/go vet |
| 5 | 危险命令在 grant 与 bypass 下仍 ask；`rm` 链式段命中整条 | `TestRed05_DangerousReasksEvenWithGrantAndBypass` | 已 grant + bypass → rm 仍 ask |
| 6 | 链式命令：一个 deny 段拒整条；allow 只按整串匹配 | `TestRed06_ChainedCommandDenyWholeAllowWhole` | `git status && rm -rf x`：git allow 不生效、rm 段 deny → deny |
| 7 | 旧 generation / 退役 profile / digest 不符 → error（fail-closed） | `TestRed07_StaleGenerationAndRetiredProfileError` | 三类输入 Evaluate/EffectiveProfile 返回 error，零写入 |
| 8 | 管理员锁：ActivateMode(bypass) 错误；Evaluate 对 bypass deny；SetAdminLock 需 authorized_by | `TestRed08_AdminLockBlocksBypass` | lock on → activate error、evaluate deny；空 authorized_by → error |
| 9 | 策略缺失/未知工具/解析失败 → error | `TestRed09_UnknownToolAndParseFailureError` | 未知 ToolKind、空 pattern、命令解析失败 → error |
| 10 | Journal 重放重建相同有效权限集；无效/未知事件 → Replay error | `TestRed10_ReplayRebuildsAndRejectsUnknown`（replay_test.go） | 两组事件重放投影相等；未知类型 error |
| 11 | 撤销 grant 后 in-flight 下一校验点拒绝；重启后 Approval 不丢失不重复；同 operationID 幂等 | `TestRed11_GrantRevocationRestartIdempotency` | revoke 后 Evaluate deny；重放后 pending 保留；同 operationID 重放无双份 |
| 12 | 两个并行 Job profile 隔离 | `TestRed12_ParallelJobIsolation` | A allow 不适用 B；B deny 不影响 A；binding 独立 |
| 13 | bypass 激活是 Journal 事实 + authorized_by 非空 | `TestRed13_BypassActivationRequiresExplicitUser` | 空 authorized_by → error；成功激活产生 Activation 事实 |
| 14 | snapshot/attention 只读，不写 Journal | `TestRed14_SnapshotIsReadOnly`（local_permission_test.go） | 只读服务调用零事件；无 Journal 写入 |
| 15 | Projection 失败保留旧视图、零写入 | `TestRed15_ProjectionFailureKeepsOldView`（replay_test.go） | Replay error 后旧投影不变，无副作用 |
| 16 | 配置文件不直接评估；导入才产生 PermissionRuleAdded | `TestRed16_ConfigFileImportOnly`（authority_test.go） | 未导入文件零效果；导入后重放生效 |
| 17 | ScopeRoot 写入与管理员锁需 authorized_by；IssueGrant 拒绝危险 pattern | `TestRed17_RootHumanGateAndDangerousGrantRejected` | 空 authorized_by → error；危险 pattern grant → error |
| 18 | allow 规则/模板/grants/bypass 均不能放行危险链式命令（危险段检查前置于 allow/ask 与 grants） | `TestRed18_AllowRuleCannotPassDangerousChain` | 模板 allow + `go test … && rm -rf …` → ask；`git *` allow + `git status && rm -rf …` → ask；bypass 仍 ask；干净模板命令仍 allow |
| 19 | personal/project 作用域规则应用于 Job（单项目 daemon） | `TestRed19_PersonalAndProjectScopeApplyToJob` | personal allow `go run *` → allow；project deny `rm *` → deny；未被 deny 覆盖的危险命令仍 ask |
| 20 | allow/grants 不能放行非危险链式命令；确定性输出 | `TestRed20_AllowRuleCannotPassNonDangerousChain`、`TestRed20_DenialRuleIDsAreDeterministicallySorted`、`TestRed20_EffectiveModeDeterministicAcrossActivations`、`TestRed20_ActivationStreamMismatchRejected` | `go test ./... && curl …` → ask；显式链式 allow 仍精确放行；RuleIDs 有序；同层多激活按 scopeID 字典序确定性选择；激活/管理员锁事件流错位 → Replay error |

旅程验证（契约 §8）：
- `scripts/verify-bp1-cross-client-journey.sh`：TUI define/bind/validate/approve
  全流程 + 原生 app（生产 Swift 客户端）读到相同 explorer/attention 状态；
  每段唯一 journey_id；GUI 用已批准替代方法，不用 Computer Use。
