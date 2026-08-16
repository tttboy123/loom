# Loom 执行手册 (2026-07-25)

> **作者**: mavis (主 session `mvs_3be0bd26bb64475dbc65181516dda9df`)
> **时间**: 2026-07-25 03:30 SGT
> **目的**: 用户问 "每个产品功能怎么落地 / 技术方案怎么执行 / FastContext 怎么用" 时的权威答案
> **数据基线**: docs/TECH-PLAN.md §14 Slice 1-5 + docs/architecture/c4-components-daemon.md 13 Component + Slice 1 已实施代码
> **状态**: 执行手册 (working document, 不 commit)

---

## 1. 产品功能 → 落地映射 (Phase 1 5 Slice)

### Slice 1: 入口与持久化 (✅ PASS at 5861f82)

| 产品功能 | 用户看到 | 落点 (C4 Component) | Go package | 命令 |
|---|---|---|---|---|
| 普通对话 vs Agent mode 区分 | `loom chat` (默认 plain) | Mode Router | `internal/mode/router.go` | — |
| 显式 Agent trigger (4 种) | 选 Agent / Use Agent / 显式要求 / 分派 | Mode Router | 同上 | — |
| Event Journal append | (无 UI, 内部) | Event Journal | `internal/journal/store.go` | — |
| Evidence Artifact Store | (无 UI, 内部) | Evidence Repository | `internal/evidence/store.go` | — |
| rebuildable projection | (无 UI, 内部) | Projection Builder | `internal/projection/projection.go` | — |
| 最小 CLI | `loom route`, `loom status` | Local API + cmd | `cmd/loom/main.go` | `loom route` / `loom status` |

**用户操作**:
```bash
# 普通对话
$ loom chat
> 你好
[Reply]

# 显式 Agent trigger
$ loom chat --use-agent
> Build a Go HTTP server
[Agent Mode + Team Draft]

# 查状态
$ loom status --json
{ "mode": "conversation", "events": 1247, "evidence": 32 }
```

---

### Slice 2: Agent 定义与团队草案 (⏸ S2-W1 contract frozen, 等授权)

| 产品功能 | 用户看到 | 落点 (C4 Component) | Go package | 状态 |
|---|---|---|---|---|
| AgentDefinition 加载 | `loom agent list` | Runtime Adapters | `internal/agents/definition.go` | S1 partial / S2-W1 frozen |
| RuntimeProfile 描述 | (内部) | Runtime Adapters | `internal/runtime/catalog.go` | S2-W1 frozen |
| RuntimeInstance 自动发现 | `loom runtime list` | Runtime Adapters | 同上 | S2-W1 frozen |
| 团队直接加载 (已保存) | `loom team load <name>` | Team Resolver | (待 S2-W2 实施) | 未开始 |
| Team Draft 生成 | 显式 trigger 时 Main Agent 提议 | Team Resolver + Main Agent Coordinator | (待 S2-W2 实施) | 未开始 |
| 用户确认 Team Draft | 弹窗: confirm / edit / reject | Team Resolver | (待 S2-W2 实施) | 未开始 |

**用户操作** (实施后):
```bash
# 列已定义 agent
$ loom agent list
[{"id": "codex-claude", "name": "Codex + Claude"}, ...]

# 列自动发现的 runtime
$ loom runtime list
[{"id": "codex@local", "version": "0.7.1", "models": ["claude-opus-4"]}]

# 显式触发 Team Draft
$ loom chat --use-agent
> Refactor internal/journal/store.go
[Agent Mode: Main Agent proposes Team Draft]
[User: confirm / edit / reject]
```

---

### Slice 3: 真实执行 (未开始)

| 产品功能 | 用户看到 | 落点 (C4 Component) | Go package | 依赖 |
|---|---|---|---|---|
| 1 个真实 Runtime Adapter | 调 Codex / Claude / Pi | Runtime Adapters | `internal/runtime/codex/adapter.go` 等 | S2-W1 (catalog) |
| JSONL Bridge | (内部 wire format) | Runtime Adapters | `internal/bridge/jsonl.go` | S2-W1 (envelope) |
| AgentGrant 颁发 | (内部凭证) | AgentGrant Authorizer | `internal/agentgrant/authorizer.go` | Slice 2 完 |
| claim generation | (内部状态) | Scheduler | `internal/scheduler/claim.go` | S2-W1 |
| prepare lease | (内部状态) | Scheduler | `internal/scheduler/lease.go` | 同上 |
| 受管 workspace | 每个 Run 独立 worktree | Scheduler + Workspace Supervisor | `internal/scheduler/workspace.go` | S2-W2 (daemon) |
| WorkItem DAG | (内部依赖图) | Main Agent Coordinator | `internal/coordinator/dag.go` | S2-W3 |
| cancel / timeout / terminal | 强制终止 Run | Scheduler | `internal/scheduler/control.go` | S2-W2 |

**用户操作** (实施后):
```bash
# 启动 Run (普通用户看不见内部)
$ loom run --agent codex-claude "Refactor journal/store.go"
[Run ID: r-7f8a9b]
[Status: pending → claim → prepare → execute → done]
[实时 stream: tool calls + evidence + bridge messages]

# 取消
$ loom run cancel r-7f8a9b
[Status: cancelled, reason: user_request]
```

---

### Slice 4: 规则与验收 (未开始)

| 产品功能 | 用户看到 | 落点 (C4 Component) | Go package | 依赖 |
|---|---|---|---|---|
| 客户 Rule | `loom rule list` | Rule + Approval Engine | `internal/rule/engine.go` | Slice 3 完 |
| `require_approval` 持久暂停 | 审批弹窗: allow/deny | Rule + Approval Engine | `internal/rule/approval.go` | 同上 |
| Evidence 收集 | (内部, 跟 Slice 1 同) | Evidence Repository | `internal/evidence/store.go` | 已有 |
| 确定性验收 | `go test ./...` 跑通 | Acceptance + Verifier Router | `internal/acceptance/runner.go` | Slice 3 完 |
| 风险级 Verifier 路由 | Standard / Strict / High 三档 | Acceptance + Verifier Router | `internal/acceptance/verifier.go` | 同上 |

**用户操作** (实施后):
```bash
# 配 Rule (用户写)
$ loom rule add --pattern "delete_*" --action require_approval

# Run 触发 approval
$ loom run --agent xxx "delete old logs"
[Run paused, waiting for approval]
$ loom run approve r-xxx
[Run resumed]

# Verdict
$ loom run status r-xxx
{"verdict": "PASS", "reviewer": "verifier-strict-1", "evidence_digest": "abc123"}
```

---

### Slice 5: 看板与 Demo (未开始, TUI 推到 Phase 2)

| 产品功能 | 用户看到 | 落点 (C4 Component) | Go package | 依赖 |
|---|---|---|---|---|
| Team / Task / Observation / Cost / Governance CLI 视图 | `loom board` | Local API | `cmd/loom/board.go` | Slice 1-4 完 |
| 对话 + 实时 Draft | 终端实时刷新 | Local API (stream) | `internal/localapi/stream.go` | Slice 2 完 |
| 执行期 timeline / Attention 投影 | 实时流 | Local API (stream) | 同上 | Slice 3 完 |
| 1 个 Coding WorkPackage | 模板 | (内置) | `templates/coding/` | Slice 3 完 |
| 1 个知识工作 WorkPackage | 模板 | (内置) | `templates/research/` | Slice 3 完 |
| 真实任务 Demo | README + 录屏 | (内部) | — | 全部完 |
| crash/restart 状态恢复 | 自动从 Event Journal 恢复 | Projection Builder | `internal/projection/recover.go` | 已有基础 |

**用户操作** (实施后):
```bash
# 看板
$ loom board
[Team: codex-claude | Task: r-xxx | Cost: $0.42 | Verdict: pending]

# 实时 stream
$ loom run --agent xxx "..." --stream
[10:00:01] tool_call: read_file(internal/journal/store.go)
[10:00:02] tool_call: write_file(internal/journal/store.go)
[10:00:05] evidence: sha256:abc123
[10:00:10] done: verdict=PASS
```

---

## 2. 技术方案执行流 (13 Component 协作)

按 `docs/architecture/c4-components-daemon.md` 的协作流,**用户一次 Run 走完 13 Component**:

```
[1] User → CLI/TUI Client (cmd/loom) 提交命令
   ↓
[2] Local API (internal/localapi) 验证 envelope (JSON-RPC 2.0)
   ↓
[3] Mode Router (internal/mode) 区分 conversation / Agent mode
   ↓ (Agent mode 触发)
[4] Team Resolver (internal/team) 加载已保存 team OR 启动 Team Draft
   ↓
[5] Main Agent Coordinator (internal/coordinator) 编译 task graph
   ↓
[6] Scheduler (internal/scheduler) 接收 WorkItem 提交
   ↓
[7] Rule Engine (internal/rule) 评估 action (allow/deny/ask)
   ↓ (allow)
[8] AgentGrant Authorizer (internal/agentgrant) 颁发 generation-bound grant
   ↓
[9] Scheduler 创建 worktree + 验 source digest
   ↓
[10] Runtime Adapters (internal/runtime) 选 compatible RuntimeInstance + 启动 Run
    ↓
[11] Adapter 调 Agent Runtime (Codex/Claude/Pi) via JSONL/stdio
    ↓
[12] Runtime Adapters 提交 evidence streams → Evidence Repository
    ↓
[13] Event Journal 追加 idempotent facts
    ↓
[14] Projection Builder 更新 read models
    ↓
[15] Scheduler 请求 Acceptance 跑确定性验收
    ↓
[16] Acceptance 选 Verifier (Standard/Strict/High)
    ↓
[17] Verifier 出 verdict (PASS/FAIL) → emit event
    ↓
[18] Projection + Local API 把 verdict / status 推回 Client
```

**每步都有 evidence** (per evidence lifecycle), 用户可查 `.loom-evidence/phase1-slice*/<id>/deliverable.md`。

---

## 3. FastContext 怎么用起来 (User Flow, Phase 2 授权后)

### 前置 (你必须显式授权)

1. **Phase 2 governance 重 freeze** (覆盖 Phase 1 那 5 处 `fastcontext_download_or_install: false`)
2. **Spike gate 授权** (per ADR-0006 + CURRENT.md EXPERIMENTAL)
3. **FCS-1 contract 状态**: CONTRACT_DRAFT → CONTRACT_FROZEN (你审完)

### User Flow (授权后 ~2 周)

#### Step 1: 装 (1 次)

```bash
# 装 community FastContext SFT 到 loopback-only local runtime
$ loom fastcontext install
[INFO] Downloading pinned community FastContext SFT...
[INFO] Verifying SHA-256...
[INFO] Setting up loopback-only runtime (localhost:0)
[INFO] Writing Loom-owned read-only adapter config
[DONE] FastContext ready at http://127.0.0.1:<random>
```

**实际落点**: `cmd/loom/fastcontext/install.go` + `internal/codeanalyst/adapter.go` (新建)

#### Step 2: 状态查询

```bash
$ loom fastcontext status
{"installed": true, "version": "v0.4.2", "runtime": "loopback", 
 "model": "fastcontext-sft-q4", "port": "127.0.0.1:54321", 
 "adapter": "loom-owned-readonly"}
```

#### Step 3: 单次查询 (code analysis sidecar)

```bash
# FastContext 跑 code analysis,不参与 Run claim
$ loom fastcontext query "find all functions in internal/journal/store.go"
[INFO] Calling Loom-owned adapter
[INFO] Adapter validates path (root containment OK)
[INFO] Calling FastContext: READ + GLOB + GREP
[INFO] Trajectory stored outside source repo: /tmp/loom-fc/<run-id>.jsonl
[RESULT] {"matches": [...], "cited_lines": [...], "confidence": 0.87}
```

**实际落点**: `cmd/loom/fastcontext/query.go`

**重要**: **不抢 agent runtime** — 这只是 code analysis sidecar, 跟 `loom run` 的 Run claim 完全独立。

#### Step 4: 跑评估 set (Loom-specific recall)

```bash
$ loom fastcontext eval --set eval/fastcontext/recall-set.json
[INFO] Loading 5 Loom-specific queries...
[INFO] Running FastContext on each...
[QUERY 1/5] "find journal write boundary" → recall=85% (PASS)
[QUERY 2/5] "trace evidence atomic publish" → recall=92% (PASS)
[QUERY 3/5] "locate projection rebuild entry" → recall=78% (PASS)
[QUERY 4/5] "find all 13 daemon components" → recall=88% (PASS)
[QUERY 5/5] "trace slice 2 contract freeze" → recall=70% (FAIL)
[AGGREGATE] recall=82.6% (≥80% threshold, PASS)
[INFO] Eval report: .loom-evidence/phase2/fastcontext-spike/FCS-1/eval-report.md
```

**实际落点**: `eval/fastcontext/runner.go` + `.loom-evidence/phase2/fastcontext-spike/FCS-1/eval-report.md`

#### Step 5: 安全审计 (每次 spike 期间)

```bash
$ loom fastcontext audit
[SAFETY] path escape attempts: 0
[SAFETY] secret leak attempts: 0
[SAFETY] source mutation outside model instructions: 0
[COST] $0.42 / $5.00 cap (8.4% used)
[VERDICT] safe
```

**实际落点**: `internal/codeanalyst/audit.go`

#### Step 6: 决策 (跑完 1-2 周 spike 后)

输出 ADR-0006 修订决策:

```bash
# 选项 A: 升 accepted
$ loom fastcontext promote --reason "recall 82.6% + 0 safety violation"
[INFO] ADR-0006 revised: accepted
[INFO] FastContext promoted to view layer code analysis sidecar

# 选项 B: 保持 experimental
$ loom fastcontext hold --reason "recall borderline 80%, need more queries"
[INFO] ADR-0006 stays: experimental

# 选项 C: 回退
$ loom fastcontext reject --reason "license conflict" | "recall < 60%" | "safety violation"
[INFO] ADR-0006 revised: rejected
[INFO] FastContext archived to research-only
```

**实际落点**: `.loom-evidence/phase2/fastcontext-spike/FCS-1/adr-revision.md`

### FastContext 跟主路径的关系 (关键)

```
普通 `loom chat` / `loom run` 路径:
  User → Mode Router → Team Resolver → Coordinator → Scheduler → Runtime Adapters
  (完全不走 FastContext)

FastContext 路径 (独立 sidecar):
  User → `loom fastcontext query` → Loom-owned read-only adapter → community FastContext SFT
  (不参与 Run claim, output 是 Candidate evidence, 不入 Event Journal)
```

**两条路互不干扰**, FastContext 是**辅助 code analysis**, 不是 Agent runtime。

### 当前能用的 (Phase 1 期间, 边界 ON)

**❌ 全部不能跑** (硬边界 `fastcontext_download_or_install: false`, 5 处声明):
- `loom fastcontext install` → 拒绝
- `loom fastcontext query` → 拒绝
- `loom fastcontext eval` → 拒绝
- 任何 SFT install / 任何 spike 评估 → 拒绝

**✅ 当前能做的** (替代):
- `loom chat` + 显式 trigger (走 S1-W1 Mode Router)
- `loom status --json` (查 Slice 1 实施状态)
- 读 `docs/CURRENT.md` + `docs/adr/0006` + `docs/integrations/fastcontext.md` (了解设计)
- 出 FastContext contract DRAFT (FCS-1 已落盘)

---

## 4. 当前 status 摘要 (2026-07-25 03:30 SGT)

| Slice | 状态 | 用户能跑 |
|---|---|---|
| Slice 1 (S1-W1~W5) | ✅ PASS at 5861f82 | `loom chat` / `loom status` / `loom route` |
| Slice 2 (S2-W1) | ⏸ contract frozen + reviewer PASS, 等授权 | (实施后) `loom agent list` / `loom runtime list` / `loom chat --use-agent` |
| Slice 3 | 未开始 | (未来) `loom run` |
| Slice 4 | 未开始 | (未来) `loom rule add` / `loom run approve` |
| Slice 5 | 未开始 (TUI 推 Phase 2) | (未来) `loom board` / `loom run --stream` |
| **FastContext** | ❌ Phase 1 期间硬边界 ON, 5 处声明 | **当前 0 可跑**, 需 Phase 2 启动 + Spike gate 授权 |

---

**VERDICT: PASS** (执行手册完成, 4 节全数, 2026-07-25 03:30 SGT)
