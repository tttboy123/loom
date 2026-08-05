# Gate 1 Exit Contract — B-P1 Execution Permission Pipeline

**Date**: 2026-08-05
**Status**: FROZEN — EXECUTION AUTHORIZED（Product Owner 目标指令 + Controller 评审；
独立 fresh Contract Review 为开放项，见 FREEZE-RECORD.md）
**Authority**: Product Owner 方向确认（移植 grok-build/主流 agent-platform 权限模型
为基线 + Loom 自研能力 + 用户体验不降级）；配套 ADR：
`.loom-evidence/execution-permissions/PERMISSION-ADR.md`
**Risk**: STRICT — 新 Journal facts、执行层授权路径、状态权威扩展、strict IPC、
TUI/原生 app 表面
**Baseline**: `a6806751` (`docs(product)`) on `codex/loom-platform-slice2`

## 1. 唯一垂直 WorkItem

```text
B-P1 Execution Permission Pipeline
```

这是权限管道唯一的垂直 WorkItem：模式基线（5 种精确语义）、规则合并
（`deny > ask > allow` 跨作用域）、只读白名单、危险命令表、per-Job 权限画像
与 generation 围栏、确定性 fail-closed 校验器、管理员锁、授权事实进 Journal、
Permission Explorer 与 Attention Inbox 客户端表面、跨客户端旅程。

本契约不包含：B（有界执行适配器——模型真实调工具的执行桥接）与 C（生产化
落地：常驻 daemon、launchd、`~/Library/Application Support/Loom` 真实配置、
显式激活）。它们是后续独立垂直 WorkItem，消费本管道。本契约不拆分薄 WorkItem
（不建 B-P1a/b、纯 writer/adapter/投影/IPC/UI 单元）。

## 2. 产品结果（用户旅程）

```text
定义/修订/退役 permission profile（TUI 或原生 app）
  -> 绑定 profile 到 Queue Job（自动继承 + 可预览收窄）
  -> 对 Job 提议一次工具调用：只读零提示；画像外 ask/deny
  -> ask 命中 -> ApprovalRequest 进 Attention Inbox，Job 停在 waiting_approval
  -> 单键批准/拒绝/记住（grant 按项目或 Job 作用域持久化）
  -> Permission Explorer 查看有效权限、命中规则来源、被拦原因、授权路径
  -> 激活/授权前 permission diff 可见；危险命令每次重提示
  -> 重启/重连后 profile、grant、ApprovalRequest 语义保持（Journal 重放）
```

客户端约束与仓库现有约定一致：Native GUI 与 TUI 走同一 Go application
service + strict local IPC；客户端不执行 Runtime、不写 SQLite/Artifact、
不创建 ID、不重构 Grant、不决定 policy、不声称完成。

## 3. 权限模型（移植基线 + 精确语义）

### 3.1 Modes（冻结 5 种，精确裁决语义）

| Mode | 裁决语义 |
|---|---|
| `default` | 只读白名单 → `allow`；其余 → `ask`（规则优先于模式） |
| `accept_edits` | 白名单 → `allow`；`Edit` 路径在 profile `OwnedPaths` 内 → `allow`；其他 `Edit`/`Bash` → `ask` |
| `auto` | 白名单 → `allow`；`Edit` 在 OwnedPaths 内 → `allow`；无危险段的 `Bash` → `allow`；网络工具/`MCPTool` → `ask` 除非显式 `allow` 规则 |
| `dont_ask` | 白名单 + 显式 `allow` 规则/grants → `allow`；其余 → `deny`（严格白名单） |
| `bypass_permissions` | 白名单 → `allow`；`deny` 规则、校验器、管理员锁仍然生效；危险段仍 `ask`；其余 → `allow` |

`always-approve`（bypass）不绕过 `deny`、确定性校验器与管理员锁。EffectiveMode
优先级（管理员锁检查在前）：Job profile mode > scope activation
（`personal` > `project` > `root`）> daemon 默认（`default`）。
`bypass_permissions` 只有在存在对应 `PermissionActivationActivated` 事实且由用户
显式操作（`authorized_by` 非空）时才生效。只读白名单是全模式便捷层；
`go test`/`go build`/`gofmt`/`go vet` 等平台常规开发命令不在白名单，但默认项目
profile 模板显式 include 这些命令的 `allow` 规则（见 §3.2），保证测试/构建旅程
零提示，不降低用户体验。

### 3.2 规则（deny > ask > allow 跨作用域合并）

- 动作：`deny` > `ask` > `allow`；任意作用域的 `deny` 全局优先（root `deny`
  不可被项目 `allow` 覆盖）。
- 非 `deny` 冲突：作用域由窄到宽优先（`job` > `project` > `personal` > `root`）；
  同作用域内 `ask` > `allow`（更保守优先）。
- 作用域来源：Job profile 规则（`job`）> 项目 `.loom/permissions.toml`
  （`project`）> 个人 `.loom/permissions.local.toml`（`personal`）>
  daemon/root policy（`root`，管理员锁管辖）。
- 配置文件是**导入输入，不是直接评估源**：v1 中规则只能通过 UI/TUI/daemon
  命令写入 Journal 事实；`.loom/permissions.toml` / `.loom/permissions.local.toml`
  的导入命令由 human-authorized 通道执行，读取后以 `PermissionRuleAdded` 事实
  写入，之后与其它规则同样重放合并。未导入的配置文件零效果（RED #16）。
- `ScopeRoot` 规则与管理员锁一样，只能由 `authorized_by` 非空（human 通道）
  写入（RED #17）。
- 匹配：`Read`/`Edit`/`Grep` 按路径 glob（`*` 不跨 `/`，`**` 跨层级）；
  `Bash` 按命令段（空白/`&&`/`;`/`|` 分段）前缀或整串匹配——`deny`/`ask`
  命中任一段即整条被拒/待批，`allow` 只按整串匹配。

非 `deny` 冲突采用作用域窄者优先（`job` > `project` > `personal` > `root`），
同作用域内 `ask` > `allow`——这是相对 grok-build 仅按严重度合并的**有意偏离**
（更具体的作用域更保守，同时允许 Job profile 精确收窄）。

### 3.3 只读白名单（便捷层，非安全边界）

- 只读工具：`Read`、`Grep`、`WebSearch`。
- 只读 shell：`ls`、`cat`、`pwd`、`head`、`tail`、`wc`、`grep`、`rg`、
  `git status`、`git log`、`git diff`、`git show`、`git rev-parse`。
- `tee`、`go test`、`cargo check`、编译/写盘命令**不在**白名单。
- 白名单只影响无规则命中时的默认裁决；`deny`/`ask` 规则与校验器始终优先。

### 3.4 危险命令表（每次重提示，不参与前缀记忆）

冻结初始表：`rm`、`chmod`、`chown`、`chattr`、`pkill`、`kill`、`killall`、
`dd`、`mkfs`、`shutdown`、`reboot`、`git push`、`git reset --hard`、
`git clean -f`。命中危险段的调用即使已有匹配 grant/`bypass_permissions`
仍返回 `ask`；校验器异常时返回 `deny`（fail-closed）。

### 3.5 记住的 grants 与一次性授权

- grants 按 `project`/`job` 作用域持久化（Journal 事实），跨作用域不生效；
  危险命令不参与 grant 记忆。
- "整个 Candidate 内允许编辑"是一次性批量授权（Job 级 grant，pattern 为该
  Job OwnedPaths 的 `**`），不隐式提升 `Bash`/`MCPTool`/网络权限。
- grant 撤销后，in-flight 调用在下一校验点拒绝（Evaluate 以撤销后的投影为准）。
- 单次 resolve `allow` 只放行该次调用；`grant-always` 才生成持久 grant
  （非危险 pattern）。`IssueGrant` 拒绝覆盖危险段的 pattern（RED #17）。

### 3.6 确定性 fail-closed 校验器（Loom 自研，替代 fail-open hooks）

```text
ProposedCall 进入 Evaluate
  -> 绑定 Job 的 effective profile（binding digest + generation 校验）
  -> 规则裁决 deny > ask > allow（跨作用域合并）
  -> 只读白名单
  -> 危险段检查（ask）
  -> 模式提示策略
  -> allow / ask / deny（typed Denial：原因 + 授权路径 + 命中 RuleIDs）
```

`Evaluate` 是编译进 daemon 的确定性纯函数：profile 缺失/退役、binding digest
或 generation 与最新投影不符、工具名未知、命令解析失败、管理员锁开启且模式为
`bypass_permissions` ⇒ 返回 error；调用方必须把 error 视为 `deny` 且无副作用
（fail-closed）。外部 shell 脚本 hooks 本契约不引入。

未绑定 profile 的 Job 不是 error：`EffectiveProfile` 返回 root policy +
daemon 默认模式（`default`）的保守合并（只读 + ask），仅在 binding 指向
缺失/退役 profile 时才返回 error。

### 3.7 管理员锁（不变量 8 的实现）

root policy 由 human-authorized writer 独占管理。`PermissionAdminLockEnabled`
只能由 `authorized_by` 非空（human 通道）写入；锁开启时 `ActivateMode
(bypass_permissions)` 返回错误、`Evaluate` 对 `bypass_permissions` 有效模式
返回 `deny`。在线自演化（Agent/Harness/Sidecar/模型输出）不得修改 root
policy、不得关闭管理员锁、不得提升自身 profile。

### 3.8 默认项目 profile 模板（冻结常量）

`DefaultProjectProfileTemplate()` 返回项目新建时绑定的默认 profile，显式
include 以下 `allow` 规则（`project` 作用域），保证平台常规旅程零提示：

```text
Bash(go test *)   Bash(go build *)  Bash(gofmt *)  Bash(go vet *)
Bash(git status)  Bash(git log *)   Bash(git diff)  Bash(git show *)
Bash(rg *)        Bash(grep *)      Bash(ls *)      Bash(cat *)
Read(**)          Grep
```

模板不含网络、`MCPTool`、`WebFetch`/`WebSearch`、危险命令、删除/覆盖类
pattern；这些在 `default` 模式下仍 `ask`。模板随实现冻结为常量并随 RED #4
覆盖。

## 4. 精确冻结 Go 符号（RED 编译失败目标）

包 `internal/permissions`（新增；纯域 + 投影 + 状态写入权威，不含 os/exec）：

```go
type Mode string
const (
    ModeDefault         Mode = "default"
    ModeAcceptEdits     Mode = "accept_edits"
    ModeAuto            Mode = "auto"
    ModeDontAsk         Mode = "dont_ask"
    ModeBypassPermissions Mode = "bypass_permissions"
)

type ToolKind string
const (
    ToolBash      ToolKind = "Bash"
    ToolRead      ToolKind = "Read"
    ToolEdit      ToolKind = "Edit"
    ToolGrep      ToolKind = "Grep"
    ToolMCPTool   ToolKind = "MCPTool"
    ToolWebFetch  ToolKind = "WebFetch"
    ToolWebSearch ToolKind = "WebSearch"
)

type RuleAction string
const ( ActionAllow RuleAction = "allow"; ActionAsk RuleAction = "ask"; ActionDeny RuleAction = "deny" )

type ScopeKind string
const ( ScopeJob ScopeKind = "job"; ScopeProject ScopeKind = "project"; ScopePersonal ScopeKind = "personal"; ScopeRoot ScopeKind = "root" )

type Rule struct {
    RuleID string `json:"rule_id"`
    Scope  ScopeKind `json:"scope"`
    ScopeID string `json:"scope_id"`
    Action RuleAction `json:"action"`
    Tool   ToolKind `json:"tool"`
    Pattern string `json:"pattern"`
}

type ProfileInput struct {
    ProfileID string `json:"profile_id"`
    Mode      Mode   `json:"mode"`
    Rules     []Rule `json:"rules"`
    OwnedPaths []string `json:"owned_paths"`
}

type PermissionProfile struct {
    ProfileID  string `json:"profile_id"`
    Generation int64  `json:"generation"`
    Digest     string `json:"digest"`
    Mode       Mode   `json:"mode"`
    Rules      []Rule `json:"rules"`
    OwnedPaths []string `json:"owned_paths"`
}

type ProposedCall struct {
    Tool    ToolKind `json:"tool"`
    Command string   `json:"command"`
    Path    string   `json:"path"`
}

// EffectiveProfile 是 Evaluate 的唯一输入：Job 绑定 profile 合并全部作用域
// 规则、active grants、模式与管理员锁后的最终裁决上下文。
type EffectiveProfile struct {
    Profile     PermissionProfile
    Mode        Mode
    MergedRules []Rule
    Grants      []Grant
    AdminLock   bool
}

type Verdict string
const ( VerdictAllow Verdict = "allow"; VerdictAsk Verdict = "ask"; VerdictDeny Verdict = "deny" )

type Denial struct {
    Reason            string   `json:"reason"`
    AuthorizationPath string   `json:"authorization_path"`
    RuleIDs           []string `json:"rule_ids"`
}

type JobBinding struct {
    JobID            string `json:"job_id"`
    ProfileID        string `json:"profile_id"`
    ProfileDigest    string `json:"profile_digest"`
    ProfileGeneration int64 `json:"profile_generation"`
    BoundAt          string `json:"bound_at"`
}

type Grant struct {
    GrantID  string `json:"grant_id"`
    Scope    ScopeKind `json:"scope"`
    ScopeID  string `json:"scope_id"`
    Tool     ToolKind `json:"tool"`
    Pattern  string `json:"pattern"`
    IssuedAt string `json:"issued_at"`
    RevokedAt string `json:"revoked_at"`
}

// Approval 不是本层新事件，而是既有 rules/approval 权威（ApprovalRequested/
// ApprovalResolved，见 internal/rules 与 internal/projection/approval.go）
// 在权限投影中的只读视图；权限层只记录 ask 决策事实。
type Approval struct {
    ApprovalID string `json:"approval_id"`
    JobID      string `json:"job_id"`
    Tool       ToolKind `json:"tool"`
    Command    string `json:"command"`
    Path       string `json:"path"`
    AskedAt    string `json:"asked_at"`
    ExpiresAt  string `json:"expires_at"`
    Status     string `json:"status"`     // pending | resolved
    Resolution string `json:"resolution"` // allow | deny (resolved only)
    ResolvedAt string `json:"resolved_at"`
    ResolvedBy string `json:"resolved_by"`
}

type Activation struct {
    Mode          Mode      `json:"mode"`
    Scope         ScopeKind `json:"scope"`
    ScopeID       string    `json:"scope_id"`
    ActivatedAt   string    `json:"activated_at"`
    DeactivatedAt string    `json:"deactivated_at"`
}

type Projection struct {
    Profiles    map[string]PermissionProfile
    Bindings    map[string]JobBinding
    Rules       map[string]Rule
    Grants      map[string]Grant
    Activations map[string]Activation // key: scope + "/" + scopeID
    AdminLock   bool
    // ApprovalView 只读引用既有 approval 投影；本层不写批准事件。
    ApprovalView map[string]Approval
}

func ValidMode(value string) bool
func ValidToolKind(value string) bool
func CompileProfile(input ProfileInput) (PermissionProfile, error)
func Evaluate(effective EffectiveProfile, call ProposedCall) (Verdict, Denial, error)
func IsReadOnlyTool(kind ToolKind) bool
func IsReadOnlyCommand(command string) bool
func DangerousSegments(command string) []string
func Replay(events []journal.Event) (*Projection, error)
func ResolveEffectiveProfile(projection *Projection, jobID string) (EffectiveProfile, error)
func EffectiveMode(projection *Projection, jobID string) (Mode, error)
func ActiveGrants(projection *Projection, scope ScopeKind, scopeID string) []Grant

type Authority struct { /* store + now */ }
func NewAuthority(store *journal.Store, now func() time.Time) (*Authority, error)
func (a *Authority) DefineProfile(ctx context.Context, input ProfileInput, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) ReviseProfile(ctx context.Context, profileID string, input ProfileInput, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) RetireProfile(ctx context.Context, profileID, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) AddRule(ctx context.Context, rule Rule, authorizedBy, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) RevokeRule(ctx context.Context, ruleID, authorizedBy, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) BindJob(ctx context.Context, jobID, profileID, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) IssueGrant(ctx context.Context, grant Grant, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) RevokeGrant(ctx context.Context, grantID, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) ActivateMode(ctx context.Context, mode Mode, scope ScopeKind, scopeID, authorizedBy, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) SetAdminLock(ctx context.Context, enabled bool, authorizedBy, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) ResolveApproval(ctx context.Context, approvalID, resolution, resolvedBy, operationID, journeyID string) ([]journal.Event, error)
func (a *Authority) RecordDecision(ctx context.Context, jobID string, call ProposedCall, verdict Verdict, denial Denial, approvalID, operationID, journeyID string) ([]journal.Event, error)
```

每个权威方法在写入前重建最新投影并做 CAS（`AppendBatchIfStreamHeads`），
与 `internal/authorization`、`internal/rules` 现有权威模式一致。
事件 ID 与幂等键沿用 `internal/rules` 的 `deterministicEventID` +
`StreamHeadExpectation` + `IdempotencyKey` 惯例（§5.2 公式）；`bind_job` 的
`jobID` 引用 `internal/queue` 的 `QueueJob.JobID`（同一队列身份，不新建 Job
权威）。

## 5. 精确 Journal 事实（唯一权威，不建第二库）

### 5.1 流与事件

| 流 | 事件 | 载荷字段 |
|---|---|---|
| `permission-profile/<profile_id>` | `PermissionProfileDefined` / `PermissionProfileRevised` | `profile_id, generation, digest, mode, rules:[{rule_id,scope,scope_id,action,tool,pattern}], owned_paths` |
| `permission-profile/<profile_id>` | `PermissionProfileRetired` | `profile_id, retired_at` |
| `permission-rule/<rule_id>` | `PermissionRuleAdded` / `PermissionRuleRevoked` | 前者 `{rule_id,scope,scope_id,action,tool,pattern}`；后者 `{rule_id,revoked_at}` |
| `permission-binding/<job_id>` | `JobPermissionBound` | `job_id, profile_id, profile_digest, profile_generation, bound_at` |
| `permission-grant/<grant_id>` | `PermissionGrantIssued` / `PermissionGrantRevoked` | 前者 `{grant_id,scope,scope_id,tool,pattern,issued_at}`；后者 `{grant_id,revoked_at}` |
| `permission-activation` | `PermissionActivationActivated` / `PermissionActivationDeactivated` | `{mode,scope,scope_id,activated_at|deactivated_at, authorized_by}` |
| `permission-admin-lock` | `PermissionAdminLockEnabled` / `PermissionAdminLockDisabled` | `{enabled_at|disabled_at, authorized_by}` |
| `permission-decision/<job_id>` | `PermissionDecisionRecorded` | `{job_id,approval_id?,verdict,tool,command?,path?,reason,authorization_path,recorded_at}` |

工具级 ask 的**批准生命周期不新建第二套**：`validate_call` 命中 `ask` 时，
权限层写入 `PermissionDecisionRecorded(ask)` 决策事实，并调用既有
`internal/rules` Approval 权威创建 ApprovalRequest（复用其
`ApprovalRequested`/`ApprovalResolved` 事实与 `waiting_approval` 状态机）；
权限投影经 `ApprovalView` 只读消费既有 approval 投影判定 pending/resolved。
`ResolveApproval` 是产品服务对既有 approval 权威的转发，不在权限层重复写
批准事件。

### 5.2 幂等与事件 ID 公式（精确）

```text
eventID    = "perm1-" + hex(sha256("perm1\n" + eventType + "\n" + stream + "\n" + operationID))[:32]
idempotencyKey = "perm1/" + eventType + "/" + operationID
seq        = per-stream，由 append 阶段 AppendBatchIfStreamHeads 分配（读 head + 批量计数）
schemaVersion = 1；EmittedAt = service.now().UTC()；CorrelationID = journeyID
```

eventID 与 idempotencyKey 只由 operationID 决定（不含 seq），保证同一逻辑操作
重放产生同一事件、幂等返回既有事实（与 `internal/rules` 的
`deterministicEventID` + `IdempotencyKey: id` 语义一致）；同一 operationID
用于不同内容时由 store 返回 `ErrIdempotencyConflict`（RED #11）。

### 5.3 重放语义

`Replay` 严格按事件顺序重建：Profile 最新 generation 生效、Retired 退役、
规则/授权追加与撤销、binding 后写覆盖、admin lock 以最新事件为准、
Approval pending/resolved 状态机（resolved 后不得覆盖）。Projection 只读、
失败返回 error（调用方保留旧视图，不 swap）。

## 6. IPC 与客户端表面（精确）

### 6.1 local IPC 方法（加入白名单）

```text
permissions_snapshot   (读 Permission Explorer / 有效权限视图)
permissions_attention  (读 Attention Inbox 待批项)
permissions_command    (写：下述 actions)
```

### 6.2 permissions_command actions

```text
define_profile | revise_profile | retire_profile
add_rule | revoke_rule
bind_job
issue_grant | revoke_grant
activate_mode | set_admin_lock
resolve_approval
validate_call   (对 Job 提议一次工具调用：返回 allow/ask/deny + typed denial；
                 ask -> 决策事实 + 既有 rules ApprovalRequest（复用 waiting_approval）；
                 bypass 激活需 authorized_by 非空)
```

### 6.3 TUI（`internal/tui/permissions.go`）

- 新增 `ScreenPermissions`（"Permissions"）作为 Permission Explorer：
  `j` Job 选择、`p` profile 详情（有效权限 + 规则来源 + 被拦原因 + 授权路径）、
  `d` permission diff 预览、`q` 返回。
- **复用既有 `ScreenAttention`** 聚合权限待批项（不新建 Attention 屏幕）：
  `a` approve / `r` reject / `g` grant-always（非危险）/ `v` 查看命令全文与
  影响面。
- 只读旅程（看板/Queue/Workers/Integration/Timeline/状态）零新增提示。

### 6.4 原生 app（Swift）

- 生产 Swift 客户端新增只读方法 `permissions_snapshot` / `permissions_attention`
  （`LocalIPCClient.swift` 白名单 + `LocalPermissionModels.swift` Codable
  闭包解码），与 TUI 读同一 projection；不新增写权限。
- 跨客户端旅程：TUI 完成 define/bind/validate/approve 全流程，原生 app 读
  到相同 explorer/attention 状态。

## 7. RED-first 矩阵（必覆盖，全部先红后绿）

1. deny 覆盖 allow（跨作用域）；root deny 不被项目 allow 覆盖。
2. deny 覆盖 always-approve（bypass 下 deny 仍 deny）。
3. ask 命中 → 调用不执行，产生决策事实 + 既有 rules ApprovalRequest；
   resolve allow/deny 后 Evaluate 结果随投影变化；权限层不重复写批准事件。
4. 只读白名单命令/工具零提示（default 下直接 allow）；`tee`/`go test` 不在
   白名单，但默认项目 profile 模板的 `allow` 规则使常规测试/构建旅程零提示。
5. 危险命令在已记住 grant 与 bypass 下仍 ask；`rm` 链式段命中整条被拒/待批。
6. 链式命令：一个 deny 段拒绝整条；allow 只按整串匹配。
7. 旧 generation / 已退役 profile / digest 不符的 binding → Evaluate error（fail-closed，无副作用）。
8. 管理员锁开启时 `ActivateMode(bypass_permissions)` 错误；`Evaluate` 对 bypass 返回 deny；
   `SetAdminLock(true)` 需要非空 `authorized_by`。
9. 策略缺失/未知工具/解析失败 → error（调用方视为 deny）。
10. Journal 重放重建相同有效权限集；无效事件/未知事件类型 → Replay error。
11. 撤销 grant 后 in-flight 下一校验点拒绝；重启后 ApprovalRequest 不丢失、不重复；
    同一 operationID 重放不产生双份事实（idempotency）。
12. 两个并行 Job 的 profile 相互隔离（A 的 allow 不适用于 B；B 的 deny 不影响 A）。
13. `activate_mode(bypass_permissions)` 激活本身是 Journal 事实且需用户显式
    操作（`authorized_by` 非空）。
14. `permissions_snapshot`/`permissions_attention` 是只读查询，不写 Journal。
15. Projection 重放/评估失败时保留旧视图，零写入、无副作用。
16. `.loom/permissions.toml` / `.loom/permissions.local.toml` 不直接评估：
    只有导入命令产生 `PermissionRuleAdded` 事实后才生效；未导入文件零效果。
17. `ScopeRoot` 规则写入与管理员锁写入必须 `authorized_by` 非空；
    `IssueGrant` 拒绝覆盖危险段的 pattern。
18. allow 规则/模板/grants/bypass 均不能放行含危险段的链式命令
    （危险段检查前置于 allow/ask 规则与 grants；RED #5/#18 共同覆盖）。

## 8. 验证矩阵

- Go full/race/vet/tidy/gofmt；STRICT 路径加 fail-closed 与重启/重放证明。
- 真实 PTY TUI 旅程 + 原生 app（生产 Swift 客户端 + strict IPC）跨客户端旅程，
  每段唯一 journey_id；GUI 验证采用已批准的替代方法（真实原生窗口/PTY 操作 +
  冻结证据 schema），不依赖 Computer Use 自动化。
- Permission Explorer 只读查询、授权摘要进 Timeline/Evidence。
- 全量 repository/race 矩阵在集成 checkpoint 运行。
- Implementation / dual-Result / Whole-Candidate Review 独立 PASS
  （P0=P1=P2=0）。

## 9. 非目标（排除）

B 执行适配器本体、C 生产化落地（常驻 daemon/launchd/Application Support/
显式激活）、网络/付费/真实用户配置、push/merge 远端、第二套 Journal/权威/
规则数据库、工具级批准第二套生命周期（复用既有 rules/approval）、逐 token 或
隐藏推理写入 Journal、fail-open hooks、自动批准绕过 human lane、自演化修改
root policy、覆盖用户 Skill、Computer Use 自动化验证。

## 10. Exact owned files（冻结范围）

### 新增

```text
internal/permissions/model.go          // Mode/ToolKind/RuleAction/ScopeKind/Rule/Profile/EffectiveProfile/ProposedCall/Verdict/Denial/Binding/Grant/Approval(视图)/Activation/Projection
internal/permissions/evaluate.go       // Evaluate/IsReadOnlyTool/IsReadOnlyCommand/DangerousSegments
internal/permissions/compile.go        // CompileProfile + DefaultProjectProfileTemplate
internal/permissions/replay.go         // Replay/EffectiveProfile/EffectiveMode/ActiveGrants
internal/permissions/authority.go      // Authority + 全部权威方法（CAS append）
internal/permissions/authority_test.go // RED-first 矩阵
internal/permissions/evaluate_test.go  // Evaluate/白名单/危险表/链式命令 RED
internal/permissions/replay_test.go    // 重放/隔离/撤销/失败保留旧视图 RED
internal/tui/permissions.go            // Permission Explorer + Attention Inbox 屏
internal/tui/permissions_test.go
internal/app/local_permission.go       // 产品服务：投影读取 + 命令路由 + 既有 rules Approval 转发
internal/app/local_permission_test.go
apps/macos/Sources/LoomLocalAppCore/LocalPermissionModels.swift
apps/macos/Sources/LoomLocalAppCore/LocalPermissionViews.swift
apps/macos/Tests/LoomLocalAppTests/LocalPermissionModelsTests.swift
scripts/verify-bp1-cross-client-journey.sh
.loom-evidence/execution-permissions/B-P1-PROGRESS.md（证据根）
```

### 修改（最小面）

```text
cmd/loomd/product_daemon.go              // 挂载 permissions 服务 + IPC 方法白名单
cmd/loomd/product_daemon_test.go
cmd/loomd/bp1_permission_wire_test.go    // handler wiring 进程内验证
cmd/loomd/sf1_queue_wire_test.go         // 组合 handler 参数补齐
cmd/loomd/sf2_workers_wire_test.go       // 组合 handler 参数补齐
cmd/loomd/sf3_integration_wire_test.go   // 组合 handler 参数补齐
internal/localipc/protocol.go            // 方法常量（permissions_snapshot/attention/command）
internal/localipc/swift_contract_test.go // swiftc 探测文件清单补 LocalPermissionModels
internal/tui/model.go                    // 新屏路由（已有 Queue/Workers/Integration 旁挂载）
apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift  // 只读方法白名单
apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift  // refreshPermissions 只读加载
apps/macos/Sources/LoomLocalAppUI/ContentView.swift          // 启动时加载权限 explorer/attention
apps/macos/Sources/LoomLocalAppContractProbe/main.swift      // 只读权限 probe 动作
```

修改面之外的任何产品路径一律不动；`internal/rules`、`internal/work`、
`internal/queue`、`internal/projection` 既有权威只被**消费/转发**，不重写其
事件语义。owned files 变更必须走独立评审的 bounded amendment。

## 11. 关卡与停止条件

1. 确定性矩阵全绿（Go full/race/vet/tidy/gofmt；Swift full/TSAN/Release）；
2. B-P1 真实跨客户端旅程冻结并验证 PASS；
3. RED 矩阵 18/18 通过；独立 Contract/Implementation/dual-Result/Whole-Candidate
   Review 均 PASS（P0=P1=P2=0）；
4. exact staging per source-lock；单个原子本地 commit；不 push/merge。

身份不一致、未评审 authority/schema/credential 扩展、任何独立 Review FAIL、
dirty 无法隔离、或越出本节边界 ⇒ 停止 `HUMAN_REQUIRED`。

VERDICT: `FROZEN — EXECUTION AUTHORIZED`
