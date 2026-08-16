# PlanView — Loom 子能力方案

> **状态**：Draft v0.1 · 2026-07-28 · 待用户 review
> **作者**：mavis（agent）
> **触发**：用户 2026-07-27 提出"demo 的规划图 + Loop 部分做到精，作为 Loom 标准能力"
> **目标读者**：lune (Loom owner) + 后续 Slice reviewer

---

## 0. 摘要

把 `loopgraph-arena` demo 中的 **规划图 (Plan DAG)** 和 **Loop 时间线 (Loop Timeline)**
抽出，做成 Loom 平台的一个一等子能力 **`planview`**。定位：用户在 review TeamInstance
计划 / 观察执行进度时使用的可视化层。

**关键边界**：
- **只读**：planview 不写任何新 Event，不引入新 authority，不修改 SQLite schema。
- **可复用**：CLI 子能力 + 未来 App 端共享同一份 spec。
- **客户端动画**：通过浏览器跑 GSAP，Go 进程只起 localhost HTTP server + 喂 JSON/SSE。
- **不污染 daemon**：daemon 主体一行不改，只通过现有 Local API + team_execution_stream.go SSE 读。

---

## 1. 背景与动机

### 1.1 现状

- `loopgraph-arena`（`/Users/lune/Documents/Codex/demos/loopgraph-arena`）已经做到一个
 完整的 dev-path demo：plan DAG + Loop/Graph 对比 + 动效 + 真实 LLM 跑通。
- Demo 中的 **DAG 图** 和 **Loop 时间线** 两个组件，是用户认可要保留并打磨的部分。
- 用户的明确意图（2026-07-27 22:13 SGT）：这两个组件**作为标准能力放入 Loom 平台**，
 而非继续留在 demo。

### 1.2 痛点

- 现有 Loom 没有 plan 可视化层：用户在 CLI 里 `loom status` / `loom timeline` 只能看
 文本投影；TeamInstance 的 DAG 是隐式的（藏在 projection 里）。
- 团队执行时，用户无法直观看到"现在跑到第几个 task、哪个 task 卡了、整体进度如何"。
- 现有 `internal/api/team_execution_stream.go` 已经实现了 SSE-like event stream，但**没有
 消费者**。planview 是它的第一个消费方。

### 1.3 目标

让 Loom 用户可以：
1. **审 plan**：在 dispatch 之前，以可视化方式看 team 准备执行的任务图（静态）。
2. **看执行**：dispatch 之后，实时看到 Loop 时间线在跑、节点状态在变（live SSE）。
3. **用 CLI 启动**：`loom plan view <team-instance-id>` 一条命令拉起浏览器。
4. **跨平台复用**：未来 App 端 (loomapp / web console) 直接 import 同一份 `internal/planview/` 包。

---

## 2. 已锁定的决策（来自 grill-me 5 问访谈）

| # | 维度 | 决策 |
|---|---|---|
| Q1 | 能力形态 | **Loom 内部模块（Go 端 + 客户端 JS bundle）** |
| Q2 | 部署位置 | **CLI 和 App 端展示**——`loom plan view` 子命令启动，App 端未来直接复用 |
| Q3 | 数据源 | **live + static 混合**——规划启动前是 static（读 projection），执行中是 live（订阅 SSE） |
| Q4 | v0 范围 | **Standard**——DAG + Loop timeline + 状态卡 + 节点详情 + motion-reduce + 键盘导航 |
| Q5 | 落地阶段 | **Parallel**——用 git worktree + 后台 sub-agent 并行推 visual 和 Go 两路 |

**未在访谈中、需要在本方案中决定的**：
- Slice 设计：作为新 lineage（PV-W*）还是并入现有 S3？
- 数据契约 schema version 策略
- App 端复用的接口形状
- 离线/无 daemon 场景下的 fallback

（见 §11 开放问题）

---

## 3. 架构总览

```
                    ┌──────────────────┐
                    │   loomd (daemon) │
                    │  - projection    │
                    │  - journal       │
                    │  - team_execution│
                    │    _stream.go    │
                    └─────┬────────────┘
                          │ (1) 读 team state (Local API)
                          │ (2) SSE 订阅 events
                          ▼
              ┌──────────────────────────────┐
              │  cmd/loom/planview (subcommand)│
              │  - 起 localhost HTTP server   │
              │  - 调 open 打开浏览器          │
              │  - 提供 /api/plan + /api/events│
              └──────────┬───────────────────┘
                         │ (3) HTTP + SSE
                         ▼
              ┌──────────────────────────────┐
              │  浏览器 (Chrome/Safari/...)   │
              │  ┌─────────────────────────┐ │
              │  │ React + GSAP + Vite     │ │
              │  │ - DAG diagram (SVG)     │ │
              │  │ - Loop timeline (Gantt) │ │
              │  │ - Status cards          │ │
              │  │ - Node detail panel     │ │
              │  └─────────────────────────┘ │
              └──────────────────────────────┘
```

**数据流**：
1. CLI subcommand 通过 Loom Local API 读 team state（plan DAG + status 投影）。
2. CLI subcommand 通过 `team_execution_stream.go` 的 SSE 端点订阅实时 events。
3. CLI subcommand 把 1+2 串成 `/api/plan` + `/api/events` 两个 endpoint，起一个 loopback-only HTTP server。
4. 浏览器拉取 `/api/plan` 渲染静态 DAG；订阅 `/api/events` 推动 GSAP 动画。

**为什么这样分**：
- daemon 不被污染（line count: 0 改动）。
- CLI 是天然的 launcher（已有 `loom status` / `loom timeline` 模式）。
- 浏览器跑动画（GSAP 在浏览器原生，vs Go 端纯 HTML 无动画）。
- 未来 App 直接复用 `internal/planview/web/` bundle + spec，不需要重写。

---

## 4. 模块结构

### 4.1 新增文件清单

```
loom-pi-rebuild/
├── internal/planview/                        # 新 Go 包
│   ├── types.go                              # PlanDAG / PlanNode / Edge / ExecutionEvent
│   ├── reader.go                             # PlanReader interface + LocalAPI 适配器
│   ├── server.go                             # localhost HTTP server (loopback only)
│   ├── sample.go                             # 离线 fallback 用的样本数据
│   ├── types_test.go                         # JSON 序列化 round-trip 测试
│   ├── server_test.go                        # HTTP handler 测试
│   └── web/                                  # 嵌入的 web bundle (//go:embed)
│       ├── index.html                        # 入口（GSAP 引入 + app 挂载）
│       ├── app.jsx / app.tsx                 # React 根 + 路由
│       ├── components/                       # DAG, LoopTimeline, StatusCards, NodeDetail
│       ├── lib/gsap-timeline.ts              # GSAP 时间线封装
│       ├── styles/                           # design tokens + 模块样式
│       └── vite.config.ts                    # build 产物到 ../web_bundle (供 embed)
│
├── cmd/loom/
│   ├── planview.go                           # 新 subcommand: loom plan view <team-id>
│   └── planview_test.go
│
└── docs/
    └── planview.md                           # 用户文档（用法 + 配置 + 故障排除）
```

### 4.2 包边界与依赖

```
internal/planview/
  ↓ imports (读 only)
  internal/projection     ← 读 team state
  internal/journal        ← 不直接读，由 Local API 包装
  internal/api            ← 调用 team_execution_stream.go 已有 SSE 端点
  internal/app            ← 调用 LocalAPI 命令
  protocol/bridge/v1      ← 复用消息类型

  ✗ NOT imports:
    internal/state         ← 不写 Event
    internal/agents        ← 不创建 Agent
    internal/teams         ← 不创建 Team
```

**约束**：`internal/planview/` **只** import `app`, `api`, `projection`, `protocol/bridge`。
任何触碰 `state`, `agents`, `teams` 的依赖都是 review blocker。

### 4.3 cmd/loom/planview.go 责任

```go
// 简化伪代码
func runPlanView(ctx context.Context, args []string, ...) error {
    teamID := args[0]
    mode := "static"  // 或 "live"（自动从 projection state 推断）

    // 1. 拉 plan（从 Local API 读 team state）
    reader := planview.NewLocalAPIReader(loomClient)
    plan, err := reader.ReadPlan(ctx, teamID)
    if err != nil { return err }

    // 2. 推断 mode
    if plan.Status == "running" || plan.Status == "paused" {
        mode = "live"
    }

    // 3. 起 localhost HTTP server
    srv := planview.NewServer(planview.ServerConfig{
        Plan: plan,
        Mode: mode,
        EventSource: reader.SubscribeEvents,  // SSE func
    })
    port, err := srv.Start(ctx)  // random loopback port
    if err != nil { return err }

    // 4. 打开浏览器
    url := fmt.Sprintf("http://127.0.0.1:%d/?team=%s", port, teamID)
    if err := openBrowser(url); err != nil {
        fmt.Fprintf(stderr, "Open this URL in your browser: %s\n", url)
    }

    // 5. 阻塞直到用户 Ctrl-C
    <-ctx.Done()
    return nil
}
```

---

## 5. 合约（Data Contract）

### 5.1 Go types（`internal/planview/types.go`）

```go
// PlanDAG — 整个 team 的任务图 + 静态元数据
type PlanDAG struct {
    SchemaVersion  int       `json:"schema_version"`  // 当前 1
    TeamInstanceID string    `json:"team_instance_id"`
    PlanID         string    `json:"plan_id"`
    Title          string    `json:"title"`
    Status         string    `json:"status"`  // draft | queued | running | paused | completed | failed
    Mode           string    `json:"mode"`    // loop | graph
    Nodes          []Node    `json:"nodes"`
    Edges          []Edge    `json:"edges"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}

type Node struct {
    ID         string            `json:"id"`
    Title      string            `json:"title"`
    Kind       string            `json:"kind"`  // task | decision | barrier
    Status     string            `json:"status"`  // queued | running | done | failed | blocked
    AssignedTo string            `json:"assigned_to,omitempty"`  // agent instance id
    EstimateMS int64             `json:"estimate_ms,omitempty"`
    Metadata   map[string]string `json:"metadata,omitempty"`
}

type Edge struct {
    From string `json:"from"`
    To   string `json:"to"`
    Kind string `json:"kind"`  // dependency | dataflow
}

// ExecutionEvent — 来自 team_execution_stream.go 的事件
type ExecutionEvent struct {
    SchemaVersion int       `json:"schema_version"`  // 当前 1
    TeamID        string    `json:"team_id"`
    Sequence      int64     `json:"sequence"`
    EmittedAt     time.Time `json:"emitted_at"`
    Kind          string    `json:"kind"`  // node_started | node_progress | node_finished | ...
    NodeID        string    `json:"node_id,omitempty"`
    Payload       json.RawMessage `json:"payload,omitempty"`
}
```

### 5.2 端点合约

| 端点 | 方法 | 响应 | 用途 |
|---|---|---|---|
| `/api/plan` | GET | `PlanDAG` | 浏览器初始化用 |
| `/api/events` | GET (SSE) | `text/event-stream` of `ExecutionEvent` | live 模式推流 |
| `/api/mode` | GET | `{"mode": "static" \| "live"}` | UI 决定渲染策略 |
| `/api/health` | GET | `{"ok": true}` | 用于浏览器健康检查 |

**SSE 格式**（沿用现有 `team_execution_stream.go` 风格）：
```
event: node_started
id: 42
data: {"schema_version":1,"team_id":"...","sequence":42,"kind":"node_started","node_id":"T3",...}

```

### 5.3 Schema 版本策略

- `schema_version` 字段强制存在，向后兼容。
- v1 → v2 升级：bump version，新增字段为 optional；解析器忽略未知字段。
- 在 v0 阶段固定 v1，避免早期 churn。

---

## 6. UI/UX 规约

### 6.1 屏幕布局（Standard 范围）

```
┌────────────────────────────────────────────────────────────┐
│  planview · team_id: ti-abc123                            │  ← 顶栏 (48px)
├────────────────────────────────────────────────────────────┤
│                                                            │
│              DAG Diagram (SVG, 70% 宽)                    │  ← Section 1
│              - 10 nodes, 14 edges                          │     静态 + 高亮
│              - 节点: queued/active/done/failed 配色        │
│                                                            │
├────────────────────────────────────────────────────────────┤
│  Loop Timeline (Gantt-style, 全宽)                        │  ← Section 2
│  - 横轴: 时间 (ms, auto-scale)                            │     静态画所有节点
│  - Loop 模式: 串行 1 节点 1 时刻                          │     live 时 playhead 推进
│  - playhead: 1px 竖线 + 顶部时刻标签                       │
│                                                            │
├──────────────────────────────────┬─────────────────────────┤
│  Status Cards (4 张, 各 1/4 宽) │  Node Detail Panel       │  ← Section 3
│  - Loop wall: 49s                │  (点击节点时滑出)         │
│  - Active tasks: 2/10            │  - 节点 ID + title        │
│  - Done: 5                       │  - 当前 status           │
│  - Elapsed: 23.4s                │  - Assigned agent        │
│                                  │  - Recent events (5)     │
└──────────────────────────────────┴─────────────────────────┘
```

### 6.2 交互与动画

| 元素 | static 模式 | live 模式 |
|---|---|---|
| DAG 节点 | 全显示，颜色按 final status | 按当前 status 渐变 |
| DAG 边 | 全显示，灰色 | active 路径渐变成 active 蓝 |
| Loop timeline | 完整画出所有节点，playhead 在末尾 | playhead 从 0 推进，推进过程 GSAP tween |
| Status cards | 显示 final 数字 | 数字 tween 动画更新 |
| 节点点击 | 打开 detail panel（readonly） | 打开 detail panel，实时刷新 events |
| 键盘 | `Tab` 切节点 / `Enter` 打开 / `Esc` 关闭 | 同左 + `Space` 暂停/恢复 |
| motion-reduce | 全部动画降为瞬时切换 | 同左 |

### 6.3 设计 tokens（与 ui-ux-pro-max 风格对齐）

```ts
// styles/tokens.ts
export const tokens = {
  color: {
    bg:        '#07090e',   // OLED 暗色
    surface:   '#0d1117',
    border:    '#1c2128',
    text:      '#e6edf3',
    textDim:   '#8b95aa',   // WCAG AA 5.4:1
    queued:    '#8b95aa',
    running:   '#58a6ff',   // 蓝色
    done:      '#3fb950',   // 绿色
    failed:    '#f85149',   // 红色
    blocked:   '#d29922',   // 黄色
    accent:    '#58a6ff',
  },
  motion: {
    fast:  '120ms',
    base:  '200ms',
    slow:  '320ms',
    ease:  'cubic-bezier(0.16, 1, 0.3, 1)',
  },
  font: {
    mono:  '"JetBrains Mono", "SF Mono", monospace',
    sans:  'Inter, system-ui, sans-serif',
    size:  { xs: 11, sm: 12, md: 13, lg: 15, xl: 18 },
  },
  space: { 0: 0, 1: 4, 2: 8, 3: 12, 4: 16, 6: 24, 8: 32 },
  radius: { sm: 4, md: 6, lg: 8 },
}
```

### 6.4 客户端技术栈

- **Vite 5** + **React 18** + **TypeScript 5**
- **GSAP 3.12**（含 `@gsap/react`）—— 节点进入、playhead 推进、status tween
- **零路由**：单页，单 team-instance 视图（多 team 在外层由 CLI 处理）
- **零 state mgmt lib**：local state + zustand（轻量）；如不需要可只用 useState
- **零 CSS 框架**：纯 CSS modules + tokens
- **a11y**：ARIA roles, keyboard nav, `prefers-reduced-motion` 适配

**为什么不用 Tailwind / shadcn**：Loom 视觉是 engineer 风格（GitHub 暗色调），design tokens 足够，
引入 utility CSS 反而稀释控制力。

### 6.5 不用 GSAP 也能跑

- 没网环境（部分 CI / 离线容器）：GSAP 静态文件本地化（不依赖 CDN）
- motion-reduce 用户：GSAP 跳过 tween，直接 set 终态
- 老浏览器：检测 `gsap` 不存在时降级到 CSS transition

---

## 7. Daemon 集成策略

### 7.1 复用现有接口

| 需要的能力 | 现有接口 | 备注 |
|---|---|---|
| 读 team state / plan DAG | `LocalAPI` 命令 + `projection.GlobalReadView` | S2-W16 已 accepted |
| 订阅 team events | `internal/api/team_execution_stream.go` | 已实现 SSE |
| 浏览器 spawn | `open` / `xdg-open` / `open -a` | 标准库 `os/exec` |
| Loopback HTTP server | 标准库 `net/http` | 不引入新依赖 |

### 7.2 需要检查的接口完整性

**阻塞问题**（实施前需确认）：
1. `LocalAPI` 是否暴露 `GetTeamPlan(teamID) → PlanDAG`？还是只有 raw projection？
   - 如果只有 raw projection：planview reader 自己做 domain → PlanDAG 映射。
2. `team_execution_stream.go` 的 SSE 端点现在接的是 daemon 内部调用，还是有 external
   client 调用方？
   - 如果只有内部：planview 通过 `http://127.0.0.1:<daemon-port>/internal-stream/...` 走 loopback。
3. Daemon 监听哪个端口？CLI 怎么发现？
   - 当前 Loom 的 CLI 怎么找到 daemon？（需要 grep 现有代码确认）

**降级方案**：如果 daemon 暂未对外暴露 plan view 接口，v0 允许 planview reader 直接读
daemon 的 SQLite journal（read-only）作为 fallback；后续在 Slice 中正式化 LocalAPI。

### 7.3 不允许触碰

- `internal/state/`（schema/Event 写权限）
- `internal/journal/`（Event 写权限）
- `internal/agents/`、`internal/teams/`（领域写权限）
- `cmd/loomd/`（daemon 主体）

**只读路径**：
- `internal/api/` 暴露的 read endpoints
- `internal/projection/` 的 `GlobalReadView`（已 S2-W16 接受）
- `internal/app/` 的 LocalAPI 命令

---

## 8. Slice 设计提议

### 8.1 lineage 划分

**推荐**：作为**新 lineage** `PV-W*`（PlanView WorkItem），独立于 S2-W* 和 S3-W*。

**理由**：
- S2-W* 是 Slice 2（Runtime discovery 链），已 frozen + 接受。
- S3-W* 是 Slice 3 Phase 1 final live gate（看现有文件名结构），不相关。
- PlanView 是新的"展示"能力，逻辑独立，强行并入任何 Slice 都会拉长依赖链。

### 8.2 提议的 WorkItem 拆解（最小集）

| WorkItem | 范围 | 风险 | 估时 |
|---|---|---|---|
| `PV-W1` | 合约冻结：`types.go` + JSON schema + 样本 fixtures | Standard | 0.5d |
| `PV-W2` | reader：`PlanReader` interface + LocalAPI 适配器 + sample 降级 | Standard | 1d |
| `PV-W3` | server：loopback HTTP + 4 个 endpoint + SSE relay | Strict | 1.5d |
| `PV-W4` | web bundle：DAG + Loop timeline + status cards + node detail | Standard | 2d（**主要由 sub-agent 推**） |
| `PV-W5` | CLI subcommand：`loom plan view` + 浏览器启动 + lifecycle | Standard | 1d |
| `PV-W6` | 整合 + E2E + screenshot + docs | Strict | 1d |
| **合计** | | | **7d** |

**注**：以上是 Slice 流程下的拆分。如果用户希望"先 ship 后 freeze"，
可合并为 PV-W1 (合约+reader+server+CLI 骨架) + PV-W2 (web bundle + 整合)。

### 8.3 与现有 lineage 的关系

```
codex/loom-platform-slice2  (c7cabee, S2-W* accepted up to W38, S3-W* in flight)
  └─ codex/loom-planview    (新 worktree, 基于 c7cabee)
      ├─ PV-W1 contract
      ├─ PV-W2 reader
      ├─ PV-W3 server
      ├─ PV-W4 web bundle
      ├─ PV-W5 CLI subcommand
      └─ PV-W6 integration + E2E
```

**重要**：planview worktree 期间，**不**做 S2/S3 的 repair / amendment。
S2/S3 的工作流与 PV 完全隔离。

### 8.4 与 Final Live Gate 的关系

`phase1-final-live-gate/` 已经在跑（看 `git status`）。planview 不应被纳入
final live gate（那是为了 Slice 1→2→3 阶段交付的）。planview 是 Slice 1 完成后的**追加**能力。

---

## 9. 分阶段交付

### 9.1 阶段 A：方案冻结（**当前**）

- ✅ grill-me 5 问访谈完成
- ⏳ 本方案文档 review（**等你 OK**）
- ⏳ 决定 lineage 命名（PV-W* vs 其他）
- ⏳ 创建 `codex/loom-planview` worktree（基于 c7cabee）

### 9.2 阶段 B：合约 + 骨架（1.5d）

主线程执行，不开 sub-agent：
- PV-W1: `types.go` + JSON schema
- PV-W2: `reader.go`（含 sample 降级）
- PV-W3: `server.go`（最小 endpoint，先不接真 SSE）
- 单元测试：types round-trip, server handler

**验收**：`go test ./internal/planview/...` 全绿；`loom plan view --sample` 能起 server
并返回 200。

### 9.3 阶段 C：并行实现（3-4d）

两个后台 sub-agent 并行：

**Stream A (visual sub-agent, coder agent)**：
- 工作目录：`internal/planview/web/`
- 任务：搭建 Vite+React+TS+GSAP 项目；实现 DAG、LoopTimeline、StatusCards、NodeDetail
  四个组件；接 sample data mode 跑通
- 必须 skills：`/ui-ux-pro-max` + `/gsap-react` + `/design-taste-frontend`
- 交付：`npm run build` 产出 `internal/planview/web_bundle/`
- 验收：screenshot 4 个状态（idle / running / completed / failed）

**Stream B (Go wire sub-agent, general agent)**：
- 工作目录：`internal/planview/` + `cmd/loom/planview.go`
- 任务：补完 server 接 SSE；CLI subcommand；浏览器启动；浏览器 fallback
- 交付：`go build ./...` + `go test ./...` 全绿
- 验收：`loom plan view <team-id>` 在测试 harness 下能起 server 并 open browser

### 9.4 阶段 D：整合（1d）

主线程：
- 把 Stream A 产出的 `web_bundle/` 接到 Stream B 的 //go:embed
- E2E：起 loomd 模拟环境 + loom plan view，验证 static → live 切换
- Screenshot 5 个核心状态
- 写 `docs/planview.md`（用法、配置、故障排除）

### 9.5 阶段 E：review + 提交（1d）

- 本地 atomic commit
- 写 `deliverable.md`（含 VERDICT: PASS 行——硬规则 §1）
- 跑最终 strict 矩阵：`go test -race ./...`, `go vet`, `gofmt`, `git diff --check`
- 等用户 sign-off

---

## 10. 验收标准

### 10.1 functional

- [ ] `loom plan view <team-id>` 起 server，浏览器自动打开到正确 URL
- [ ] static 模式：浏览器显示完整 DAG + Loop timeline，节点按 final status 配色
- [ ] live 模式：浏览器订阅 SSE 收到 events，节点状态实时变化，timeline playhead 推进
- [ ] 点击节点 → detail panel 滑出，显示该节点最近 5 条 events
- [ ] motion-reduce 用户：所有动画降为瞬时切换
- [ ] 键盘导航：`Tab` 切节点 / `Enter` 打开 / `Esc` 关闭 / `Space` 暂停（live）
- [ ] fallback：无 daemon 时 `loom plan view --sample` 跑样本数据

### 10.2 non-functional

- [ ] **零 daemon 改动**：`git diff codex/loom-platform-slice2..codex/loom-planview -- internal/api internal/state internal/journal cmd/loomd` 为空
- [ ] **零新 Event**：`grep -r "Event" internal/planview/` 只引用 read API，不创建
- [ ] **零 schema 变更**：`internal/state/migrations/` 无新文件
- [ ] **包边界干净**：`internal/planview/` 不 import 写权限包
- [ ] **测试覆盖**：`go test -cover ./internal/planview/...` ≥ 80%
- [ ] **build 体积**：embed bundle ≤ 500KB（gzip）
- [ ] **浏览器兼容**：Chrome ≥ 100, Safari ≥ 15, Firefox ≥ 100
- [ ] **a11y**：axe-core 自动检测无 critical 违规

### 10.3 review 用证据

- 5 张核心状态 screenshot（idle / static-loaded / live-mid / live-completed / failed）
- `loom plan view --sample` 跑通的录屏 / 截图
- `go test -race ./...` 输出
- `git diff` 摘要（新增文件清单 + 行数）

---

## 11. 风险与缓解

| 风险 | 等级 | 缓解 |
|---|---|---|
| Daemon LocalAPI 未暴露 plan view 端点 | 中 | v0 用 SQLite journal read-only 作 fallback；下个 Slice 加 endpoint |
| GSAP 在内网环境拉不到 CDN | 低 | 把 gsap.min.js 打进 bundle（不依赖外部） |
| team_execution_stream.go 现有 SSE 不够 | 中 | v0 自己接 journal 查 events；下个 Slice 与现有 stream 合并 |
| 用户在 macOS / Linux / Windows 浏览器启动差异 | 中 | 显式分支 `open` / `xdg-open` / `cmd /c start` |
| web bundle 体积膨胀（>500KB） | 低 | Vite tree-shake + 路由懒加载 + 字体 subset |
| Sub-agent 产出的代码质量不一致 | 中 | 主线程整合时做 code review + visual screenshot 验收 |
| 与 S3-W* 后续工作冲突 | 低 | planview 是新 lineage，新 worktree，git 自然隔离 |
| 客户端 XSS / 注入 | 中 | bundle 纯 React + 不解析 HTML；Node title 等用 textContent |

---

## 12. 开放问题（需用户决策）

| # | 问题 | 推荐答案 |
|---|---|---|
| OQ-1 | Lineage 命名 | **`PV-W*`**（新 lineage，不并入 S3） |
| OQ-2 | Sample data 是 v0 必需吗？ | **是**——demo 演示 + 测试都靠它 |
| OQ-3 | v0 是否需要 Replay 模式？ | **否**——live + static 够用，replay 是 v2 增强 |
| OQ-4 | 是否要在 v0 加 `--export-png` 截图导出？ | **否**——浏览器自带截图；v2 再加 |
| OQ-5 | web bundle 走 Vite 还是 Snowpack/Vite | **Vite**（生态最成熟） |
| OQ-6 | App 端的接口形态 | **先不定**——v0 只为 CLI 写；App 复用 spec 时再开设计 |
| OQ-7 | 是否纳入 Loom final live gate？ | **否**——是 Slice 1 后续追加能力，独立 review |

---

## 13. 关联文档

- 当前 demo 源：`/Users/lune/Documents/Codex/demos/loopgraph-arena/src/web/views/DevPathDemo.tsx`
- Loom 架构：`loom-pi-rebuild/docs/architecture/c4-containers.md` + `c4-components-daemon.md`
- Loom 团队事件流：`loom-pi-rebuild/internal/api/team_execution_stream.go`
- Loom 投影：`loom-pi-rebuild/internal/projection/`
- Slice 2 现状：`loom-pi-rebuild/docs/CURRENT.md`
- Slice 3 草稿：`loom-pi-rebuild/.loom-evidence/phase1-slice3/`

---

## 14. 决策记录

| 日期 | 决策 | 来源 |
|---|---|---|
| 2026-07-27 | 5 问访谈结果 | user picks |
| 2026-07-28 | 切成 plan-only 模式 | user "让你给个方案，而不是让你实现" |
| 2026-07-28 | 架构：内部包 + CLI subcommand + //go:embed | mavis 推荐 |
| 2026-07-28 | 数据模式：static + live 双模 | user Q3 |
| 2026-07-28 | 范围：Standard | user Q4 |
| 2026-07-28 | 阶段：Parallel | user Q5 |

---

**VERDICT: DRAFT v0.1** — 待用户 review。
