# 交接上下文：Loom 圆桌（Roundtable）设计 + codebase-design 技能修订

## 0. 仓库与当前主线状态

- 仓库：`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`，分支 `codex/loom-platform-slice2`
- 主线程 HEAD：`baa0ed6b`（docs(w-rules): freeze Gate 1）
- 主线进度（按提交）：
  - B-P1 权限管道：**已验收**（whole-slice ACCEPTED）
  - B-W1 执行适配器 / C-W1 生产化落地：已实现 + 跨客户端旅程 PASS（`3d750465`/`d4a1b9ee`/`38dbf3ec`），**待生产可用验收**（演示曾被暂停）
  - W-BRIDGE 模型桥接接入执行管道：Gate 1 已冻结（`f57107ab`/`8be5d8ab`）
  - W-RULES 客户规则：Gate 1 已冻结（`9b6faf8a`/`baa0ed6b`）
  - **Phase 4（含圆桌）排在这些之后，尚未开始执行**
- 治理惯例（必须遵守）：Event Journal 唯一权威 + `AppendBatchIfStreamHeads` CAS；投影可重建；strict IPC + journey_id；TUI/原生 app 共用同一 Go service；Evidence 0700/0600；不变量 1-8；flash 独立评审通道不稳定（多次空载荷/卡死，记为开放项，通道恢复后补做）。

## 1. 圆桌（Roundtable）产品定义

- **主持人制中继对话**：用户在两个/多个 Agent 会话之间作主理人，逐条决定"谁发言、传什么、是否插入/丢弃"，产生有界多轮 QA + 对齐摘要。
- 交互入口：拖拽连接两个 Agent = 创建会话（显式触发，非普通对话建 Team）。
- **不传完整上下文**：每条消息是有界载荷（body ≤ 8 KiB、artifact_refs 只引用 digest），与人和 Agent 对话同构——用户决定对方能看到什么。
- **安全底线**：中继内容对对方会话只是**输入，不是指令**；不自动执行、不自动转发、每跳经主持人确认。
- **留痕**：每轮进 Journal + 对齐摘要 SHA-256 Artifact；存储长期迁移 Memory（P3，短期就地）。
- **A2A 方向**：用 A2A 作协议形态（Message/Task/Part、role 语义，主持人=A2A client role=user，席位=A2A agent）；先做 **A2A 形状的本地通道**（进程内/loopback，不强行 HTTP），外部端点后置。
- **故障语义**：
  - Agent 会话失效 → 席位标记 `seat_unavailable`（事实），pending 消息保留不丢、不自动重试；恢复由主持人决定；留痕不依赖会话存活（摘要来自 Journal/Artifact）。
  - 圆桌异常 → 事实持久、投影重建、CAS 冲突 typed；圆桌**只通知不接管**席位（席位状态权威在席位自身）。

## 2. 技术方案（R1-R3，已含 codebase-design 评审修正）

### R1 圆桌域 + 留痕（`internal/roundtable`）
- Authority 小接口：`CreateSession / JoinSeat / LeaveSeat / Relay / Insert / Drop / Conclude`
- 事实：`RoundtableCreated / SeatJoined / SeatLeft / RoundtableMessageRelayed / ModeratorInserted / MessageDropped / RoundtableConcluded`
- **Typed errors（现在就定死）**：`ErrSessionNotFound / ErrAlreadyConcluded / ErrRoundCapExceeded / ErrNotModerator / ErrSeatUnavailable / ErrPayloadTooLarge / ErrDigestMismatch / ErrSeatAlreadyJoined`
- 隐式契约显式化：create→join→relay/insert→conclude；conclude 后拒绝一切写入；仅主持人可 relay/insert/drop。
- 返回领域视图（深拷贝），不泄漏 journal.Event；投影可重建。

### R2 服务 + IPC + A2A 形状数据（`local_roundtable`）
- **不做 Transport seam**（当前只有一个本地投递实现）；只建 A2A 形状 data types（Message/Task/Part、role、message_id/digest）。
- `SeatSession` 接口**只因测试 fake 而存在**（fake+本地=两个 adapter）；P2 外部端点出现才是真实 seam。
- `local_roundtable` 独占：①主持人门禁（无确认不投递）②轮次上限+有界载荷校验 ③投递 ack/pending 语义（按 message_id/digest 幂等重投）④摘要 Artifact 发布。
- IPC：`roundtable_snapshot`（读）/ `roundtable_command`（create/join/relay/insert/drop/conclude），strict 白名单 + journey_id。
- authority 不碰 seat 投递与 Evidence（防 god module）。

### R3 客户端 + 旅程
- TUI `ScreenRoundtable`（双席位视图 + 消息卡片；`j/k` 选择、`t` 传、`i` 插入、`d` 丢弃、`c` 总结；拖拽连接入口）。
- Swift 只读模型 + strict IPC 只读方法。
- `scripts/verify-roundtable-cross-client-journey.sh`：真实 PTY + 原生 app，两个 Loom 内 Agent 席位多轮 QA + 对齐摘要。

### 队列三 Job
- `RT-R1`（域，无依赖）→ `RT-R2`（服务，依赖 R1）→ `RT-R3`（客户端，依赖 R2）；各带 owned files、lane=development、独立原子提交；R1 可与 B/C 验收收尾并行。

## 3. Phase 4 插入与评估

- 圆桌并入 **Phase 4（互通）**，排序：`4.1 圆桌 R1-R3 → 4.2 导入/导出/共享合同（含对齐摘要）→ 4.3 可替换 Provider 路由后端 → 4.4 圆桌外部席位（A2A 协议层/AgentCard）+ 自动投递桥接 → 4.5 协作平台适配（Multica）+ 可选 Web UI`。
- 后置：standing orders/Autopilot（依赖客户规则）、多用户权限、团队共享资产目录、外部通知。
- 依赖：4.4 依赖 4.2+4.3；Memory 留痕迁移不占主线（R1 后可并行）；带预算中继 grant 依赖客户规则（capability_gap 未闭合前不立项）。
- 评估缺口：多用户权限需在 4.4 契约里冻结身份边界；外部适配器凭据/fail-closed 边界；对齐摘要纳入 4.2 合同；成本/时延治理。

## 4. codebase-design 技能（本对话第二产物，已完成修订）

- 文件：`/Users/lune/.agents/skills/codebase-design/SKILL.md`、`DEEPENING.md`、`DESIGN-IT-TWICE.md`
- 评审 1 → 修订 SKILL.md：explicit/implicit interface、errors part of interface、AI-navigability、depth has costs、separate decisions from effects、one external interface、test doubles count、vocabulary scope、test surface softened。
- 评审 2 → 修复：either/both 逻辑 bug、DEEPENING 测试策略对齐（internal-seam 豁免、不无脑删旧测试）、隐喻非词条、新增 "Performance is part of the interface"。
- 现状：三文件自洽，可直接使用该词汇评审模块设计。

## 5. 本对话改动的文件（均未提交）

- `/Users/lune/.agents/skills/codebase-design/SKILL.md`、`DEEPENING.md`（修订）
- `AGENTS.md`（追加 `## Agent skills` 块：Issue tracker=GitHub `tttboy123/loom`、Triage labels=五默认标签、Domain docs=单上下文；**治理主文件，未提交**）
- `docs/agents/issue-tracker.md`、`docs/agents/triage-labels.md`、`docs/agents/domain.md`（新建，来自 setup-matt-pocock-skills 种子模板）
- `.loom-drafts/roundtable-landing-plan-2026-08-06.md`（新建，96 行：R1-R3 落地 + Phase 4 插入 + 评估）

## 6. 开放项 / 新 session 建议首件事

1. 主线：确认 B-W1/C-W1 生产可用验收（证据根 `/private/tmp/bw1-journey-final`、`/private/tmp/cw1-journey-final2` 已 verify PASS）；继续 W-BRIDGE/W-RULES 实施。
2. 圆桌：**R1 技术草稿尚未写**（接口签名 + 事件载荷 schema + IPC 签名 + 错误清单），4.2-4.5 技术方案也未写——Phase 4 立项时逐个写。
3. flash 独立评审为全局开放项（通道恢复后补做）。
4. 所有未提交改动（第 5 节）需你确认后再决定是否提交。

---

> 本文件由 2026-08-06 交接会话落盘（双保险副本）。是否提交由用户确认。
