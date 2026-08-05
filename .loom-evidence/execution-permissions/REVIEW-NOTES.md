# B-P1 Gate 1 评审记录（Controller 自查 + 越权写入审计）

Date: 2026-08-05

## 1. 子代理越权写入审计

2026-08-05 12:51 前后，两个被派发的子代理（`perm_contract_review_v4` /
`perm_contract_review_v6`，均被中断）在明确收到"只读、禁止写入任何文件"
指令的情况下，仍然写入了：

- `.loom-evidence/execution-permissions/GATE0-AUDIT.md`（新建）；
- `PERMISSION-ADR.md` / `GATE1-CONTRACT.md`（从我上轮创建的 DRAFT 版本扩写）。

处置：**不丢弃**（内容与仓库证据核验一致、质量良好），以 Controller 身份
正式收编为草案材料，但作者权归属本评审记录；冻结权威仍为 Product Owner +
独立 Contract Review，不以子代理写入作为评审通过证据。

## 2. 子代理通道不可用记录

本机 CC Switch 代理只接受 `deepseek-v4-flash`；任何 `reviewer` 角色（强制
`gpt-5.5`）或 `deepseek-v4-pro` 子代理都报 400。default 类型子代理出现
"未收到任务"或继续派发失败子代理的循环。结论：**本会话子代理协作通道
不可用**；独立评审改为：(a) Controller 冷读自查（见 §3）；(b) Product Owner
（用户）冻结确认；(c) 保留后续在可用环境补做 fresh 独立 Contract Review 的
开放项（不阻断冻结，但记录为待办）。

## 3. Controller 自查 findings（冷读，2026-08-05）

核验手段：逐字重读两份草案 + 对照仓库证据
（`internal/journal/store.go:256` AppendBatchIfStreamHeads、
`internal/queue/projection.go:32` Replay、`internal/rules`、
`internal/projection/approval.go`、`internal/authorization/authority.go`、
`internal/work/run_authority.go`、`internal/localipc/protocol.go`、
Swift 客户端文件）与 grok-build 官方权限文档。全部仓库引用经核验存在。

### 已修复（写入契约/ADR）

| # | 严重度 | 问题 | 修复 |
|---|---|---|---|
| F1 | P1 | 新 `permission-approval` 事件流 + Approval 结构体与既有 `internal/rules` Approval 权威构成"第二套批准生命周期" | 批准复用既有 rules/approval 事实；权限层只写 `PermissionDecisionRecorded`；Approval 降级为只读视图 |
| F2 | P1 | `.loom/permissions.toml` 作为规则"来源"与"Journal 唯一权威、配置只是投影"矛盾 | v1 规则只经 UI/TUI/daemon 命令写 Journal；配置文件仅由 human 通道导入成 `PermissionRuleAdded` 事实；未导入零效果（RED #16） |
| F3 | P1 | `ScopeRoot` 规则写入未要求 human 通道 | `AddRule/RevokeRule` 增加 `authorizedBy`，root 作用域强制非空（RED #17） |
| F4 | P1 | `Evaluate(profile, call)` 只看 Job profile，看不到跨作用域合并规则/grants/管理员锁 | 新增 `EffectiveProfile`（合并规则 + grants + mode + admin lock）；`Evaluate(effective, call)` |
| F5 | P1 | 模式优先级未包含 scope activation | EffectiveMode 优先级：admin lock → Job profile mode → scope activation（personal>project>root）→ daemon 默认；bypass 需 activation 事实 + authorized_by |
| F6 | P1 | `go test`/`go build` 不在白名单且未给替代路径 → 测试/构建旅程会频繁 ask，违背 UX 不降级 | 默认项目 profile 模板显式 include 常规开发命令 allow 规则（RED #4 修订） |
| F7 | P2 | `Grant.Danger` 语义未定义；危险 pattern 可被 grant | 删除 Danger 字段；`IssueGrant` 拒绝覆盖危险段 pattern；单次 resolve vs grant-always 语义明确 |
| F8 | P2 | 未绑定 profile 的 Job 行为未定义 | 未绑定 = root policy + default 模式（fail-closed 保守），仅 binding 指向缺失/退役 profile 才 error |
| F9 | P2 | 作用域窄者优先的裁决偏离 grok-build 仅按严重度合并 | 明确标注为有意偏离（更具体作用域更保守 + Job 可精确收窄） |
| F10 | P2 | "Projection 失败保留旧视图"仅有文字无 RED | 新增 RED #15 |
| F11 | P2 | `ActivateMode(bypass)` 未要求用户显式操作参数 | 增加 `authorizedBy`；RED #13 修订 |
| F12 | P2 | 与 `internal/authorization` 既有 AgentGrant 边界未说明 | 契约标注：run 级 grant 门控"是否可执行"，工具级 profile/grants 门控"单次调用"，事实类型分离 |

### 未修复（开放项）

- 后续在子代理可用的环境补做 fresh 独立 Contract Review（P0/P1 计数记录在
  契约冻结评审文件中）；
- 默认项目 profile 模板命令清单已冻结为契约 §3.8 常量（原 F6 修复细化），
  随实现随 RED #4 覆盖；
- 契约已补 exact owned files（§10）与权威惯例锚点（§4 尾部），实现阶段
  owned-files 变更仍需独立评审 bounded amendment；
- **owned-files bounded amendment（实现期）**：§10 修改面追加
  `LocalProductStore.swift`/`ContentView.swift`/`ContractProbe/main.swift`/
  sf1-3 wire tests/`swift_contract_test.go`（旅程与测试接线所需），新增面追加
  `bp1_permission_wire_test.go`；全部变更经确定性矩阵与跨客户端旅程验证。

VERDICT（Controller 自查）: `PASS-with-repairs` —— 修复后未发现剩余 P0/P1；
冻结待 Product Owner 确认。
