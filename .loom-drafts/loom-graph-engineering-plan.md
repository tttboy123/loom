# Loom Graph Engineering Plan (Phase 1 Slice 2)

**Status**: DRAFT (awaiting review)
**Created**: 2026-07-25 02:00 SGT
**Owner**: mavis (assistant)
**Reviewer**: lune (customer #0)
**Branch**: `codex/loom-platform-slice2` (head detached at `8e207b8`)
**Slice 1 commit**: `5861f82` (frozen, 5 WorkItem all PASS)
**Next gate**: freeze this plan → emit `phase1-slice2.goalspec.yaml` → start S2-W1

---

## 1. Overview

用 Graph Engineering (DAG 任务调度 + 状态机 + plan-execute-verify 闭环) 来管理 **Loom Phase 1 Slice 2** 的 16 个 work item。这套方法把任务画成有向无环图, 每组并行, 严格按状态机走 PDCA 闭环, 出问题可回滚到 checkpoint。

**关键不变量** (从 Loom Constitution 继承):
- Slice 1 commit `5861f82` frozen, 新工作用 `S2-W*` 命名
- `phase1-slice1.optimized.goalspec.yaml` 是历史, Slice 2 需要自己的 spec
- 借鉴优先级 P0 必在 S1-W 期间出, P1 在 Phase 2 准备合桶, P2 长期暂缓
- 单写者原则 (per task) + 频繁 atomic commit + deliverable.md 在 report-back 前 flush

---

## 2. DAG (16 tasks × 12 parallel groups)

### 2.1 ASCII DAG 图

```
                   ┌──────────────────┐
                   │  G0 [1] 认错纠正  │ ✅ done
                   └────────┬─────────┘
                            ▼
                   ┌──────────────────┐
                   │  G1 [2] Slice 2  │ 1-2d 必经
                   │  spec (go)       │
                   └────────┬─────────┘
                            ▼
              ┌─────────────────────────────┐
              │ G2 [3] S2-W1: Bridge v1.1  │ 4-5d 必经
              │  JSON-RPC 2.0 + W3C +       │
              │  JSON Schema + Coord        │
              └────┬─────────────────┬─────┘
        (并行)     │                 │
              ┌────▼─────┐    ┌──────▼────────────┐
              │ G3a[5]doc│    │ G3b [4] S2-W2      │ 3-4d 必经
              │ bridge-  │    │  Daemon + age +    │
              │ v1.1-    │    │  LICENSE + gov.    │
              │ arch.md  │    └────────┬───────────┘
              └──────────┘             ▼
                        ┌──────────────────────────┐
              ┌────────▼──┐    ┌─────────────────┐
              │G4a[7] doc │    │G4b [6] S2-W3    │ 4-5d 必经
              │subagent-  │    │ Coordinator +   │
              │ spec.md   │    │ SubAgent 双类型  │
              └───────────┘    └────────┬────────┘
                                        ▼
                                ┌────────────────────┐
                                │ G5 [8] S2-W4      │ 2-3d 必经
                                │  $USD budget +    │
                                │  OTel SDK         │
                                └────────┬──────────┘
                                         ▼
              ┌──────────┐    ┌────────────────────┐
              │G6a[10]doc│    │G6b [9] S2-W5      │ 3-4d 必经
              │SKILL.md  │    │ Capabilities +    │
              │v1        │    │ SKILL.md + chaos  │
              └──────────┘    └────────┬──────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ G7 [11] Loom Console │ 2-3d
                            │  TUI 最小版 (新加)    │ Slice 2 末尾
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ G8 [12] Phase 2 P0   │ 1-2w
                            │  4 项合桶             │ Phase 2 入口
                            │  (.agents/skills +    │
                            │   Remote MCP + 5 层   │
                            │   + TUI 完整版)       │
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ G9 [13] Phase 2 P1   │ 1-2w
                            │  4 项合桶             │ Phase 2 中
                            │  (三级压缩 + ctx +    │
                            │   exactly-once +      │
                            │   OAuth 2.1)          │
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ G10 [14] Phase 2 docs│ 1w
                            │  3 future docs 合桶  │ Phase 2 末
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ G11 [15] Phase 3+    │ 不定
                            │  5+1 项暂缓           │ 长期
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ G12 [16] 长期监控    │ 月度
                            └──────────────────────┘
```

### 2.2 Task Table (16 项)

| ID | Group | Name | Risk | Parallel-Safe-With | Est. | Blocked-By | Acceptance |
|---|---|---|---|---|---|---|---|
| [1] | G0 | 认错纠正 | — | — | ✅ done | — | TodoWrite + PROGRESS.md + memory 三目标同步 |
| [2] | G1 | Slice 2 spec | Standard | — | 1-2d | [1] | `phase1-slice2.goalspec.yaml` 220+ 行, 含完整 DAG + plan schema |
| [3] | G2 | S2-W1: Bridge v1.1 | Strict | — | 4-5d | [2] | JSON-RPC 2.0 envelope + W3C Trace + JSON Schema 2020-12 + Coordinator dispatch 4 件套 + 测试 |
| [4] | G3b | S2-W2: Daemon | Strict | [5] | 3-4d | [3] | Daemon 长驻 + age 加密 + LICENSE (MIT) + governance 红线 + 测试 |
| [5] | G3a | doc: bridge-v1.1-arch | Standard | [4] | 1d | [3] | `docs/bridge/loom-bridge-v1.1-architecture.md` 详细 spec |
| [6] | G4b | S2-W3: Coordinator | Strict | [7] | 4-5d | [4] | Coordinator + SubAgent 双类型 + Cline Teams 模式 + 测试 |
| [7] | G4a | doc: subagent-spec | Standard | [6] | 1d | [6] | `docs/architecture/subagent-spec.md` SubAgent dict + CompiledSubAgent |
| [8] | G5 | S2-W4: budget+OTel | Standard | — | 2-3d | [6] | $USD budget middleware + OTel SDK (默认 `localhost:4317`) + W3C trace + 测试 |
| [9] | G6b | S2-W5: Capabilities | Standard | [10] | 3-4d | [8] | Capabilities bundles + SKILL.md v1 + 1h chaos test |
| [10] | G6a | doc: SKILL.md v1 | Standard | [9] | 1d | [9] | `docs/skills/loom-coding-agents.md` v1, 3 editor 同步示例 |
| [11] | G7 | Loom Console TUI 最小版 | Standard | — | 2-3d | [10] | bubbletea + lipgloss 实时 journal/evidence/cost 观察器, 取代 sqlite3+jq |
| [12] | G8 | Phase 2 P0 准备 | Standard | — | 1-2w | [11] | 4 项合桶: `.agents/skills/` + Remote MCP 框架 + 5 层能力矩阵 + TUI 完整版 |
| [13] | G9 | Phase 2 P1 准备 | Standard | — | 1-2w | [12] | 4 项合桶: 三级压缩 + context_schema + exactly-once + OAuth 2.1 |
| [14] | G10 | Phase 2 docs | Standard | — | 1w | [13] | 3 future docs: mcp-2026-07-28-rc.md + dbos-vs-restate-vs-temporal.md + loom-coding-agents.md v2 |
| [15] | G11 | Phase 3+ 长期 | Cancelled | — | 不定 | [14] | 5+1 项: P1-1/4/6/9/10 跳过/暂缓 + Loom Web Phase 3+ 等 TUI 跑稳 |
| [16] | G12 | 长期监控 | Standard | — | 月度 | [15] | 每月看 DBOS / pg_durable / MCP minor / Temporal Replay 2027 |

### 2.3 关键路径 + 时间线

**关键路径 (必经, ~5 周)**: G1 → G2 → G3 → G4 → G5 → G6 → G7
- G1: 1-2d
- G2: 4-5d (核心, JSON-RPC + W3C + Schema)
- G3: 3-4d (Daemon + age + LICENSE, 跟 [5] doc 并行)
- G4: 4-5d (Coordinator + SubAgent, 跟 [7] doc 并行)
- G5: 2-3d (budget + OTel)
- G6: 3-4d (Capabilities + SKILL + chaos, 跟 [10] doc 并行)
- G7: 2-3d (Loom Console TUI 最小版)
- **总: ~25 天 ≈ 5 周**

**Phase 2 准备**: 1-2w (G8) + 1-2w (G9) + 1w (G10) ≈ 3-5 周
**Phase 3+ 长期**: 不定 (G11) + 月度 (G12)

---

## 3. plan.json Schema (per task)

每个 task 启动时, 写一份 `plan.json` 到 `.loom-evidence/phase1-slice2/<task-id>/plan.json`:

```json
{
  "task_id": "S2-W1",
  "task_name": "Bridge v1.1 envelope",
  "parallel_group": 2,
  "risk_level": "strict",
  "estimated_days": "4-5",
  "dependencies": ["phase1-slice2.goalspec.yaml"],
  "blocked_by": ["[2]"],
  "blocks": ["[4]", "[5]"],
  "acceptance_criteria": [
    "JSON-RPC 2.0 envelope implemented (jsonrpc: 2.0, id, method, params/result/error)",
    "W3C Trace Context propagation (traceparent / tracestate headers)",
    "Complete JSON Schema 2020-12 for all exposed methods",
    "Coordinator dispatch via loom.dispatch method",
    "go test ./... -count=1 PASS",
    "go test -race ./... -count=1 PASS",
    "go vet ./... PASS",
    "git diff --check PASS",
    "Fresh reviewer session returns VERDICT: PASS"
  ],
  "deliverable_artifacts": [
    "internal/bridge/  Go package with envelope.go, trace.go, schema.go, dispatch.go",
    "internal/bridge/  Tests (unit + race + integration)",
    ".loom-evidence/phase1-slice2/S2-W1/contract.md",
    ".loom-evidence/phase1-slice2/S2-W1/deliverable.md (末尾: VERDICT: PASS)"
  ],
  "atomic_commit_targets": [
    "internal/bridge/envelope.go + envelope_test.go",
    "internal/bridge/trace.go + trace_test.go",
    "internal/bridge/schema.go + schema_test.go",
    "internal/bridge/dispatch.go + dispatch_test.go"
  ],
  "rollout_strategy": "single-session, one writer",
  "recovery_checkpoint": "atomic commit after each atomic_commit_target"
}
```

---

## 4. plan-execute-verify 闭环 (PDCA)

### 4.1 状态机

每个 task 走严格状态机 (LangGraph 风格 + Reducer):

```
       plan.json 写好
            │
            ▼
   ┌────────────────┐
   │  pending       │ 启动 session
   └───────┬────────┘
           │ start
           ▼
   ┌────────────────┐
   │  in_progress   │ 写代码, 频繁 commit
   └───────┬────────┘
           │ RED + GREEN 走完
           ▼
   ┌────────────────┐
   │  ready_for_    │ deliverable.md flush
   │  review        │ 
   └───────┬────────┘
           │ 独立 reviewer session 跑
           ▼
      ┌─────────┐
      │ VERDICT?│
      └──┬───┬──┘
       PASS  FAIL
         │    │
         ▼    ▼
       [done] ┌────────────────────────────┐
              │ repair (bounded, ≤3 次)     │
              │  - 分析失败原因              │
              │  - 限定修复范围              │
              │  - 重新 EXECUTE + VERIFY    │
              │  - 3 次还失败 → escalate    │
              │    human review            │
              └────────────────────────────┘
```

### 4.2 文件结构 (per task)

```
.loom-evidence/phase1-slice2/
└── S2-W1/
    ├── contract.md        # 启动前写, 边界 + acceptance
    ├── plan.json          # DAG schema (见 §3)
    ├── progress.json      # 实时进度, 阶段 1-5
    ├── verify.json        # 验证结果 (test/race/vet/reviewer)
    ├── deliverable.md     # (末尾必填 VERDICT: PASS/FAIL)
    ├── repair-1-contract.md (可选, 失败时)
    ├── repair-2-contract.md (可选, 失败时)
    └── review-scope.md    # reviewer 范围
```

### 4.3 Reducer (LangGraph 风格)

```python
# 状态机 reducer
from typing import Annotated, Literal
from operator import add

TaskStatus = Literal["pending", "in_progress", "ready_for_review", "done", "failed", "repair"]

# 不可变状态
class TaskState(TypedDict):
    task_id: str
    status: Annotated[TaskStatus, transition]  # 只能通过 transition 函数改
    progress: Annotated[Dict[str, Any], deep_merge]  # 子字段 deep merge
    attempts: Annotated[int, add]  # 重试次数累加
    verifier_verdicts: Annotated[List[Literal["PASS", "FAIL"]], add]  # 历次 verdict
    
# transition 函数 (白名单)
ALLOWED_TRANSITIONS = {
    "pending": ["in_progress"],
    "in_progress": ["ready_for_review", "repair"],
    "ready_for_review": ["done", "repair"],
    "repair": ["in_progress"],
    "done": [],
    "failed": ["repair"],
}
```

---

## 5. State Management (借鉴 graph engineering 状态机)

### 5.1 5 类状态文件

| 文件 | 位置 | 内容 | 写入时机 |
|---|---|---|---|
| `meta.json` | `.loom-evidence/phase1-slice2/meta.json` | 整个 DAG (16 tasks + 12 groups + 依赖边) | 启动时一次 |
| `plan.json` | per task (见 §3) | 单 task 计划 | task 启动时 |
| `progress.json` | per task | 实时进度 (started / sub_steps / %) | 持续 |
| `verify.json` | per task | 验证结果 (test/race/vet/verdict) | EXECUTE 完 |
| `deliverable.md` | per task | (末尾 `VERDICT: PASS/FAIL`) | EXECUTE 完 + flush 前 |

### 5.2 Reducer 模式 (LangGraph)

每个 task 状态用 `Annotated[Status, transition]`:
- 不可变快照
- 只能通过 transition 函数改
- attempts + verdicts 用 add reducer 累加

### 5.3 不可变 + 声明式

```go
// Go 实现风格 (跟 S1 模式对齐)
type TaskState struct {
    TaskID    string
    Status    TaskStatus  // pending / in_progress / ready_for_review / done / failed / repair
    Attempts  int
    Verdicts  []string    // ["PASS", "FAIL", ...]
    Snapshot  []byte      // 不可变 JSON 快照
}

// 只能通过 transition 函数改
func (s *TaskState) Transition(to TaskStatus) error {
    if !allowed(s.Status, to) {
        return fmt.Errorf("illegal transition: %s -> %s", s.Status, to)
    }
    s.Snapshot = marshal(s)  // 不可变快照
    s.Status = to
    return nil
}
```

---

## 6. Checkpoint 策略 (崩溃可恢复)

### 6.1 4 级 checkpoint

| 级别 | 触发 | 内容 | 恢复点 |
|---|---|---|---|
| **L1 atomic commit** | 每完成 1 个 atomic_commit_target | 1-2 个 Go 文件 + test | 该子步 done |
| **L2 task commit** | 1 个 task 全部 atomic commit 完成 | 1 个 task 的所有 Go 文件 | 该 task done |
| **L3 group commit** | 1 个 parallel group 全部 task 完成 | 1 个 group 的所有 task 产物 + tag | 该 group done |
| **L4 slice milestone** | Slice 2 关键里程碑完成 (G2 / G6 / G7 done) | 整个 Slice 2 中间状态 + tag | 该 milestone done |

### 6.2 atomic commit 频率 (硬规则, 跨项目经验)

**长 worker context overflow (~237k tokens)** 的对策:
- 每个 atomic_commit_target 完成后立即 commit, **不要攒**
- deliverable.md 在 report-back **前** flush (避免最后一步 overflow 丢 work)
- 早 + 频繁 commit 是反 overflow 的关键

### 6.3 恢复策略

| 失败点 | 恢复方法 |
|---|---|
| atomic commit 之间 | `git checkout -- <file>` + 重写该子步 |
| task 内 | `git log` 找上一个 atomic commit, 重新跑后续 sub-steps |
| task 之间 | `git checkout <last-task-tag>` + 重新跑该 task |
| group 之间 | `git checkout <last-group-tag>` + 重新跑该 group |
| slice 中间 | 重新跑那个 milestone 的所有 task |

---

## 7. 风险 Mitigation (跨项目经验)

### 7.1 长 worker context overflow (~237k tokens)

| 风险 | 缓解 |
|---|---|
| 长 implement worker 在 deliverable-flush / report-back 步骤 deterministic overflow | 4-hint 对策 (跨项目 SKILL): <br>1. **owner-recovery**: overflow 时 owner 从 atomic commit 接手 <br>2. **早期+频繁 atomic commits**: 每子步 commit, 不攒 <br>3. **deliverable.md 在 report-back 前 flush**: 避免最后一步丢 <br>4. **owner-skip accept-on-ready**: owner 看完 evidence 直接 accept, 不等 worker 报告 |

### 7.2 失败重试无限循环

| 风险 | 缓解 |
|---|---|
| 同一个 task 失败 10 次还不收敛 | **最多 3 次 fix, 失败 escalate human** (硬规则) |
| 失败原因 unclear | 用 verifier 文化: claim 前 grep + check 时间戳 + 不 speculation 填 gap |
| 跨 task 互相干扰 | 单写者原则, 不允许并行写同一文件 |

### 7.3 Verifier 自治文化 (跨项目 SKILL)

| 规则 | 含义 |
|---|---|
| **claim "pre-existing" 前** | `git log --all -- <path>` 验证 |
| **claim "sibling caused X" 前** | check 时间戳+ls-files |
| **claim 错了** | 明确认错, 不用 speculation 填 gap |
| **VERDICT 必填** | deliverable.md 末尾必须有 `VERDICT: PASS/FAIL`, 缺失 → auto-reject → consecutive_failures++ → cancel |

### 7.4 跨 Slice 命名错位 (已发生 1 次, 2026-07-24)

| 风险 | 缓解 |
|---|---|
| S1 frozen 但命名复用, 错位到 S2 | S1 commit `5861f82` 起, S1-W* 命名 frozen; 新工作强制 S2-W* |
| Slice 1 spec 误用给 Slice 2 | Slice 2 必须出新的 `phase1-slice2.goalspec.yaml` (本 plan freeze 后) |
| 任意多 Slice 项目启动前 | 必读 "完成状态 + 当前 Slice" + "已完成的 Slice 不要复用命名" |

### 7.5 Git 状态污染

| 风险 | 缓解 |
|---|---|
| 调研产物污染 working tree | 放 `.loom-drafts/`, untracked, 不 commit |
| Slice 2 启动时 dirty tree | 启动前 `git status` 必须 clean, 调研进 `.loom-drafts/` |
| Zombie 文件 (plan cancel 后) | `mavis session ls <agent>` + `git status --porcelain \| grep "^??"` |

---

## 8. 借鉴优先级时间表 (按 S2 排)

| P0 借鉴 | 时间 | 落点 |
|---|---|---|
| **P0-1** Coordinator + specialists 命名 | S2-W3 (G4b) | Bridge v1.1 `loom.dispatch` |
| **P0-2/3/4** JSON-RPC 2.0 + W3C Trace + JSON Schema 2020-12 | S2-W1 (G2) | Bridge v1.1 envelope |
| **P0-5** Capabilities = composable bundles | S2-W5 (G6b) | Bridge v1.1 method dispatch |
| **P0-6** Middleware 洋葱圈 | S2-W2 / S2-W4 | daemon 中间件栈 |
| **P0-7** SubAgent 双类型 | S2-W3 (G4b) | subagent spec |
| **P0-8** $USD budget per run | S2-W4 (G5) | Run claim 必填 |
| **P0-9** OTel spans trace 出口 | S2-W4 (G5) | daemon observability |
| **P0-10** 多 editor skill 同步 | Phase 2 准备 (G8) | `.loom/skills/` |
| **P0-11** Remote MCP Server | Phase 2 (G8) | cost view |
| **P0-12** 5 层能力矩阵 | Phase 2 (G8) | cost view |
| **P0-13** MIT + view layer 严格 fail closed | S2-W2 (G3b) | LICENSE + governance |
| **P0-14** view layer 不可妥协 | S2-W2 (G3b) | governance/non-negotiables.md |

| P1 (Phase 2) | 时间 |
|---|---|
| **P1-2** 三级上下文压缩 | Phase 2 (G9) |
| **P1-3** context_schema | Phase 2 (G9) |
| **P1-11** exactly-once | Slice 2 顺便 (S2-W4/S2-W5) |
| **P1-12** OAuth 2.1 | Phase 2 末 (G9) |

| P2 (Phase 3+ 暂缓) | 处理 |
|---|---|
| **P1-1** Code mode (Pydantic) | **跳过** — Loom 不是 user 写代码场景 |
| **P1-4** DBOS Conductor MCP | Phase 3 选型后学 |
| **P1-6** Postgres | Phase 2 multi-user 才需要 |
| **P1-9** loom-insight (PXI) | 3-6 月数据后 |
| **P1-10** 7h chaos | Phase 3 才上 (S2 跑 1h, P2 跑 4h) |

---

## 9. Loom 客户端 3 形态 (用户 2026-07-24 16:36 SGT 决策)

| 形态 | 何时 | 队列项 | 作用 |
|---|---|---|---|
| **SKILL.md v1** (3 editor 触发) | **S2-W5 (G6b 内含)** | 项 #9 | Claude Code / Codex / Cursor 3 editor 调 Loom (走 daemon JSON-RPC 2.0) |
| **Loom Console TUI 最小版** (bubbletea + lipgloss) | **Slice 2 末尾先出 1 个 (G7)** | 项 #11 | journal/evidence/cost 实时观察器 (纯 view, 不思考不执行), 取代 sqlite3+jq |
| **Loom Console TUI 完整版** (cost dashboard + trace 串联) | **Phase 2 准备 (G8)** | 项 #12 合桶 | TUI 加 cost + trace view |
| **Loom Web** (本地静态, `localhost:7432`) | **Phase 3+ 长期 (G11)** | 项 #15 合桶 | 时间线 + 树状 + 验证 + dashboard, 等 TUI 跑稳 |

**Loom 客户端严格不抢**:
- ❌ 不做 "前台 agent CLI" 跟 Cline / OpenCode / Pi 抢
- ❌ 不做 IDE 跟 Cursor 抢
- ❌ 不做完整 observability SaaS 跟 Langfuse / Phoenix 抢
- ❌ 不开外部端口 (Loom Web 只在 localhost)

---

## 10. 实施下一步 (推荐路径)

### 10.1 选项 B 的输出

本文档 (`docs/loom-graph-engineering-plan.md`, 放 `.loom-drafts/`) 包含:
- ✅ 完整 DAG 图 (16 tasks × 12 parallel groups)
- ✅ 每个 task 的 acceptance criteria (16 项)
- ✅ plan.json schema
- ✅ plan-execute-verify 闭环
- ✅ State Management (5 文件 + Reducer 模式)
- ✅ Checkpoint 策略 (4 级)
- ✅ 风险 Mitigation (5 大风险)
- ✅ 借鉴优先级时间表
- ✅ Loom 客户端 3 形态

### 10.2 用户 review 流程

1. **lune review** 整篇文档, 重点看:
   - §2 DAG 是否合理 (有漏掉 / 错位的 task 吗)
   - §2.2 acceptance criteria 是否够严格
   - §4.1 状态机是否需要加状态
   - §6 checkpoint 频率是否合适
   - §7 风险是否漏了关键
2. **lune 反馈**: OK / 部分改 / 大改
3. **mavis 改**: 重新出 v2 (如有改动)
4. **lune freeze**: 标 "FROZEN" + 标日期
5. **mavis 出 spec**: `phase1-slice2.goalspec.yaml` 写出来 (本 plan 是骨架, spec 是 contract)
6. **启动 [3] S2-W1**: 独立 session 跑 Bridge v1.1 envelope

### 10.3 时间表 (估算)

| 步骤 | 估时 | 责任 |
|---|---|---|
| 用户 review 本 plan | 1-2 天 | lune |
| 反馈 + 改 v2 (如需) | 0.5-1 天 | mavis |
| Freeze 本 plan | 0 | lune |
| 出 `phase1-slice2.goalspec.yaml` | 1-2 天 | mavis |
| Freeze spec | 0 | lune |
| 启动 S2-W1 独立 session | 4-5 天 | worker |
| S2-W1 收尾 + doc | 1 天 | worker |
| 启动 S2-W2 (跟 S2-W1 收尾并行) | 3-4 天 | worker |
| ... 按 DAG 推 |  |  |
| Slice 2 完整 | **~5 周** |  |

---

## 11. 验收标准 (本 plan freeze 后)

- [ ] 用户 review 完毕, 标 "FROZEN" + 日期
- [ ] `phase1-slice2.goalspec.yaml` 出, 含完整 DAG + 16 task acceptance
- [ ] TodoWrite 16 项都已 approved, 准备启动
- [ ] `.loom-evidence/phase1-slice2/` 目录创建好
- [ ] `meta.json` (DAG 16 tasks + 12 groups) 写好
- [ ] branch `codex/loom-platform-slice2` 状态 clean (working tree 没有 dirty 文件除了 `.loom-drafts/`)
- [ ] S1 commit `5861f82` 不动
- [ ] 启动 S2-W1 独立 session, 从 `phase1-slice2.goalspec.yaml` 读 spec

---

## 12. 一句话总结

**Loom Slice 2 = 16 tasks × 12 parallel groups DAG × plan-execute-verify 闭环 × 4 级 checkpoint × 5 文件 state 管理**, 关键路径 ~5 周, 崩溃可恢复, 跨项目经验 (overflow 4-hint + verifier 文化 + 命名错位 mitigation) 全部用上。

---

## 附录 A: 文件结构总览

```
loom-pi-rebuild/
├── .loom-drafts/                              # untracked, 调研 + 计划
│   ├── phase1-slice1.optimized.goalspec.yaml   # Slice 1 (历史)
│   ├── new-session-prompt.md
│   ├── phase-roadmap-supplements.md            # v0.4
│   ├── community-survey-v1.md
│   ├── community-survey-v2.md
│   ├── community-survey-v3-deepdive.md
│   └── loom-graph-engineering-plan.md          # 本文档 (待 freeze)
├── PROGRESS.md                                # 同步队列 + 调研摘要
├── .loom-evidence/                            # Slice 1 evidence (frozen)
│   ├── phase1-slice1/                         # commit 5861f82
│   │   ├── S1-W1/, S1-W2/, S1-W3/, S1-W3-L2/, S1-W4/, S1-W5/
│   │   └── final-verification.md
│   └── phase1-slice2/                         # Slice 2 evidence (待创建)
│       ├── meta.json                          # DAG 完整定义
│       ├── S2-W1/                             # per-task evidence
│       │   ├── contract.md
│       │   ├── plan.json
│       │   ├── progress.json
│       │   ├── verify.json
│       │   └── deliverable.md                 # 末尾: VERDICT: PASS/FAIL
│       ├── S2-W2/...
│       └── ... (16 个 task 目录)
└── branch: codex/loom-platform-slice2          # head detached at 8e207b8
```

## 附录 B: 跨项目经验索引

- **硬规则 §1** (VERDICT 必填): memory `memory/MEMORY.md:13-14`
- **硬规则 §2** (prompt 术语先 grep): `memory/MEMORY.md:16-17`
- **硬规则 §6** (3 目标同步): `memory/MEMORY.md:28-36`
- **跨项目模式 — 长 worker context overflow (4-hint)**: `memory/MEMORY.md:40-47`
- **跨项目模式 — Verifier 自治文化**: `memory/MEMORY.md:49-58`
- **Doc 路由 — 默认 Keeper**: `memory/MEMORY.md:60-69`
- **Loom 工作队列排序入队 2026-07-24**: 1 条 entry
- **Loom Slice 1 vs Slice 2 命名错位纠正 (2026-07-24)**: 1 条 entry
- **Loom 客户端 3 形态决策 (2026-07-24)**: 1 条 entry
- **MCP 2026-07-28 RC 4 production signals**: 1 条 entry
- **Pydantic AI Harness + Capabilities composable bundles**: 1 条 entry

---

**Verdict (本 plan)**:
- 包含 16 task 完整 acceptance + DAG + checkpoint + state mgmt
- 跨项目经验全部用上 (overflow + verifier + 命名错位 mitigation)
- 用户 review 后 freeze → 出 spec → 启动 S2-W1

VERDICT: DRAFT (待用户 review)
