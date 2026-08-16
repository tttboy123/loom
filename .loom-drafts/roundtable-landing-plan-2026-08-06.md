# Roundtable（圆桌）落地规划与 Phase 插入

Date: 2026-08-06
Status: 规划草案（未冻结；不含 Gate/RED 形式主义，落地按 R1→R2→R3 顺序执行）
定位：主持人制中继对话 —— 用户在两个（或多个）Agent 会话间作为主理人，逐条转达/插入/丢弃消息，产生有界多轮 QA 与对齐摘要；每轮留痕，存储长期迁移 Memory（短期 Journal + Artifact 就地）。

## 1. 落地切片（codebase-design 评审修正后）

### R1 圆桌域 + 留痕（internal/roundtable）

- **Authority**（深模块，小接口）：
  `CreateSession / JoinSeat / LeaveSeat / Relay / Insert / Drop / Conclude`
  - 全部走 `AppendBatchIfStreamHeads` CAS，事实：`RoundtableCreated / SeatJoined / SeatLeft / RoundtableMessageRelayed / ModeratorInserted / MessageDropped / RoundtableConcluded`
  - 返回领域视图（深拷贝），不泄漏 journal.Event；重建投影保持重启一致。
- **Typed errors（现在就定死，R2 不返工）**：
  `ErrSessionNotFound / ErrAlreadyConcluded / ErrRoundCapExceeded / ErrNotModerator / ErrSeatUnavailable / ErrPayloadTooLarge / ErrDigestMismatch / ErrSeatAlreadyJoined`
- **隐式契约显式化**（写入接口文档）：调用顺序 create→join→relay/insert→conclude；conclude 后任何 relay/insert 拒绝；只有主持人可 relay/insert/drop。
- 留痕：每轮 `RoundtableMessageRelayed` 事实 + 对齐摘要 SHA-256 Artifact（事实只存 digest）。
- 测试：经接口测（真实 store），覆盖顺序约束、轮次上限、幂等重投（message_id/digest）、双席位隔离、重启重放。

### R2 服务 + IPC + A2A 形状数据（local_roundtable）

- **不做 Transport seam**（当前只有本地投递一个实现，抽象 seam=假设性 seam）。
- **只定义 A2A 形状数据模型**（Message/Task/Part、role 语义、message_id/digest）——是 data types，不是协议抽象；AgentCard/task 生命周期/JSON-RPC 留到 P2。
- **`SeatSession` 接口只因测试 fake 而存在**（fake + 本地实现 = 两个 adapter，测试 seam 成立）；P2 外部端点出现时才成为真实 seam。
- **`local_roundtable` 独占四件行为（深模块，避免透传）**：
  1. 主持人门禁：无用户确认的 relay 一律不投递；
  2. 轮次上限 + 有界载荷校验（body ≤ 8 KiB、artifact_refs 白名单）；
  3. seat 投递与 ack/pending 语义：未送达=pending，按 message_id/digest 幂等重投，失败不静默；
  4. 摘要 Artifact 发布（SHA-256 + 事实存 digest）。
- IPC：`roundtable_snapshot`（读）/ `roundtable_command`（create/join/relay/insert/drop/conclude），strict IPC 白名单 + journey_id。
- authority 不碰 seat 投递与 Evidence（防 god module）；拆出新模块只发生在第二个 adapter 出现时。

### R3 客户端表面 + 旅程

- TUI `ScreenRoundtable`：双席位视图 + 消息卡片；`j/k` 选中、`t` 传给对方、`i` 插入、`d` 丢弃、`c` 总结；拖拽连接=创建会话入口。
- Swift 只读模型（LocalRoundtableModels）+ strict IPC 只读方法。
- `scripts/verify-roundtable-cross-client-journey.sh`：真实 PTY TUI + 原生 app，两个 Loom 内 Agent 席位完成多轮 QA + 对齐摘要，留痕与摘要 Artifact 齐全。

## 2. 任务队列 Phase 插入规划

### 插入点

当前基线：B-P1（权限管道）已验收；B-W1/C-W1（执行适配器/生产化）已实现且旅程 PASS，**待生产可用验收**。圆桌 R1-R3 插入在 B/C 验收之后，归属 **Phase 4（互通）** 的第一个纵向切片（4.1 圆桌）。

### Queue 三 Job（复用现有调度框架）

| Job | 能力 | 依赖 | owned files | lane | 退出条件 |
|---|---|---|---|---|---|
| RT-R1 | roundtable-domain | — | `internal/roundtable/**` | development | internal/roundtable 测试 + go vet 绿；R1 原子提交 |
| RT-R2 | roundtable-service | RT-R1 | `internal/app/local_roundtable.go`、`internal/api/local_roundtable.go`、`cmd/loomd/**`、`internal/localipc/protocol.go` | development | wire 测试 + 全矩阵绿；R2 原子提交 |
| RT-R3 | roundtable-client | RT-R2 | `internal/tui/roundtable.go`、`internal/tui/model.go`、Swift 模型/只读方法、`scripts/verify-roundtable-cross-client-journey.sh` | development | 跨客户端旅程 verify PASS；R3 原子提交 |

约束：每 lineage 单 writer；R2/R3 串行依赖 RT-R1；R1 可与 B/C 验收收尾并行（只读探索）。

### Phase 序列（圆桌并入 Phase 4）

```
[当前] B-P1 ACCEPTED ─ B-W1/C-W1 已实现+旅程 PASS（待生产可用验收）
   ↓ 验收后插入
[ v0.3 / Phase 4 ]
  4.1 圆桌 R1 → R2 → R3（域+留痕 → 服务+IPC+A2A 形状数据 → TUI/Swift+旅程）
  4.2 导入/导出/共享合同（含对齐摘要、资产与 Evidence 的契约化互操作）
  4.3 可替换 Provider 路由后端（Runtime 互通基建）
  4.4 圆桌外部席位（A2A 协议层/AgentCard/JSON-RPC）+ Runtime 桥接自动投递
  4.5 协作平台适配（Multica 等）与可选 Web UI（表面互通）
  后置：standing orders/Autopilot（依赖客户规则）、多用户权限、
        团队共享资产目录、外部通知
```

圆桌专属后置（不占 Phase 4 主线）：Memory 留痕迁移（事件留 digest/指针，
transcript 落 Memory，重放不受影响）可在 R1 后随时并行；自动摘要与带预算
中继 grant 依赖客户规则能力（4.5 之后）。

### 依赖与复用

- 复用 B-P1 权限管道：`artifact_refs` 引用白名单按 profile 规则收窄。
- 复用 A4 批准生命周期：中继 grant（P4）需要时直接挂 rules。
- 复用 C-W1 Evidence/留痕基建；Memory（P3）只做存储层替换，不改事件语义。
- 每切片独立验收（确定性矩阵 + 跨客户端旅程），评审沿用既有惯例（flash 独立评审为开放项，通道恢复后补做）。

## 3. 验收门（轻量）

1. R1：Go 焦点/race + 领域测试绿（含幂等重投、顺序约束、双席位隔离）；
2. R2：wire 测试 + 全矩阵绿；主持人门禁测试（无确认不投递）在服务层而非 UI；
3. R3：真实跨客户端旅程 verify PASS + 证据包；
4. 每切片一个原子本地提交；不 push/merge。

## 4. Phase 4 内容评估（摘要）

Phase 4 主题一致（互通：Runtime 互通/组织互通/数据互通/Agent 互通），但范围
偏宽、依赖差异大。建议按"契约先行 → Agent 互通 → 基建 → 表面"排序：4.1/4.2
（圆桌 + 导入导出合同）是低依赖高价值切片，先落；4.3 路由后端独立并行；
4.4 外部席位依赖 4.2 + 4.3；4.5 平台适配与 Web UI 最后。缺口：多用户权限与
外部适配器的凭据/fail-closed 边界需在各自契约里显式冻结；对齐摘要应纳入
导入/导出合同的覆盖范围。
