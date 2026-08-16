# Loom Governed Handoff 产品简报

Date: 2026-08-08

Status: `DRAFT` — 非权威产品诊断与加速建议；不修改既有合同、Phase、代码、发布或执行权限。

Source session: `019fd351-61ae-7db1-a087-3c410e7995ec`

Competitive research: [`session-handoff-competitive-research-2026-08-08.md`](session-handoff-competitive-research-2026-08-08.md)

## 1. 结论先行

建议立即把 **Governed Handoff（受治理交接）** 提升为 Loom 的第一差异化叙事，但不要直接跳到完整 Roundtable，也不要再造一套 handoff authority。

Loom 已经有可演示的 `CURRENT` 底座：Phase 2B `Side-task Handoff and Parent Decision` 在提交 `6d380233` 完整验收，具备独立子任务 lineage、结构化摘要 Artifact、digest、受限 `ContextPacket`、七种 typed decision、CAS 单赢家、崩溃恢复、Projection 重建，以及 macOS/TUI 表面。当前最大问题不是核心能力缺失，而是：

1. 社区几乎看不到它：README 与用户指南没有把它作为主能力说明；
2. chat-first 入口尚未把它变成自然动作；
3. 当前分支比远端上游领先 166 个提交且工作区很脏，不能把“本地已实现”误写成“社区已可用”；
4. Roundtable 仍是 `TARGET` 草案，没有产品代码；
5. 旧 Roundtable 方案没有充分复用已经验收的 P2B handoff 原语，若按原稿直接实现，会有平行权威与重复语义风险。

一句话定位：

> **Claude 可以跨 Session 发消息，Codex 可以迁移 Chat 与 Git state；Loom 让交接本身可选择、可批准、可拒绝、可验证、可重放。**

英文短句：

> **Don't move the whole session. Transfer only the context you can authorize and verify.**

## 2. 原 Session 内容整理

目标 Session 的有效产出分为四块：

### 2.1 Roundtable 产品定义

- 用户是主持人，在两个或多个 Agent 会话之间决定谁发言、传什么、插入什么、丢弃什么；
- 每跳都需要用户确认，消息对接收会话只是输入，不自动成为执行指令；
- 不复制完整上下文，消息体有界，Artifact 只传 digest 引用；
- 每轮写 Journal，结论写 SHA-256 Artifact；
- 先做 A2A 形状的本地通道，外部 A2A endpoint 后置；
- 会话失效时保留 pending，不静默重试；Roundtable 不接管 seat 自身状态权威。

### 2.2 原技术拆分

- `RT-R1`：`internal/roundtable` 域、session/seat/message/conclude facts、typed errors、可重建视图；
- `RT-R2`：本地 service、strict IPC、A2A-shaped data、主持人门禁、pending/ack、摘要 Artifact；
- `RT-R3`：TUI/Swift 双席位界面与跨客户端旅程。

### 2.3 原路线判断

- 把 Roundtable 放在 Phase 4 互通的首个纵向切片；
- 后续才做导入导出、Provider routing、外部 A2A seat、协作平台与 Web；
- 自动中继、带预算 grant、多用户与外部凭据边界后置。

### 2.4 与产品无关但已完成的工作

- `codebase-design` skill 的 interface/error/performance/test-seam 词汇修订；
- issue tracker、triage labels 与 domain docs 的治理说明。

这些内容可保留，但不应混进 Governed Handoff 的社区叙事或加速合同。

## 3. 与当前代码真相的校准

旧 Session 以 `baa0ed6b` 为基线。当前记录时 HEAD 为 `651f156a`，已前进 23 个提交，并存在大量未提交/未跟踪工作。旧交接里的主线状态已经过期。

| 能力 | 当前状态 | 代码/证据 |
|---|---|---|
| Side-task Handoff | `CURRENT` | `6d380233`；`internal/work/side_task_handoff.go`、`internal/app/local_product_handoff.go`、`internal/projection/side_task_handoff.go` |
| 内容寻址 Artifact + digest-only 引用 | `CURRENT` | `internal/evidence/store.go` 与 P2B summary/input/context packet artifacts |
| Journal 多 stream CAS | `CURRENT` | `internal/journal/store.go` 的 `AppendBatchIfStreamHeads` |
| strict local IPC + journey correlation | `CURRENT` | `internal/localipc/protocol.go` |
| chat-first 单会话连续性 | `PARTIAL` | P2C 修复中；持久 thread 与 responder port 已有，配置化 conversation adapter 与全旅程复验未闭合 |
| Roundtable session/seat/relay domain | `TARGET` | 仅 `.loom-drafts/roundtable-landing-plan-2026-08-06.md`；代码中无 `internal/roundtable` |
| 任意 Loom task/session → task/session 交接 | `TARGET` | P2B 当前是 parent/side-task 关系，不是通用 session router |
| 外部 Claude/Codex/A2A seat | `EXPERIMENTAL` | 尚无 endpoint、身份、凭据或 fail-closed adapter 合同 |

关键架构修正：Roundtable 只应新增 **session/seat/relay/conclude ledger**，并直接复用既有 Journal CAS 与 Evidence Store。P2B 的摘要约束、允许字段、digest、typed decision 与 replay 语义可以作为设计来源，但其 `ContextPacket`、Event 与 authority API 都绑定 parent/side-task 身份，不能被 Roundtable 直接调用或假定为通用接口。任何通用 handoff envelope 的抽取都必须成为单独评审的 authority/schema 边界。

## 4. 为什么现在值得加速

官方一手资料显示，这个类别已被两家同时教育市场，但实现语义不同：

| 产品 | 已发布原语 | 实际移动的状态 | 留给 Loom 的空间 |
|---|---|---|---|
| Claude Code 2.1.224 | 跨 Session `SendMessage` | 独立 Session 之间的纯文本消息；接收方保留自己的上下文与权限 | 缺少 Loom 式 Evidence-linked ContextPacket、typed absorption、Journal replay 与多方主持流程 |
| Claude Code teleport | cloud/web Session 在本地 CLI 继续 | 已有 cloud Session 的 continuation | 不是通用 peer handoff，也不是受治理的跨任务吸收 |
| Codex Remote Handoff | 同一 Chat 在 host 间迁移 | Chat + Git state；目标创建/复用 worktree | 是 execution-location migration，不是两个独立任务之间的选择性语义交接 |
| Loom P2B | parent ↔ side-task 的受治理交接 | 授权摘要 Artifact + bounded ContextPacket + exact decision/effect lineage | 已有技术差异，但缺少公开入口、通用目标绑定与 Roundtable UX |

因此不要把三者都叫成“Session 搬家”。应明确区分：

- **Relay**：给另一个 Session 发消息；
- **Resume/Teleport**：在另一个客户端继续同一 Session；
- **Relocate**：把同一 Chat 与工作态迁到另一台 host；
- **Governed Handoff**：把经过选择、验证和授权的上下文交给另一条独立任务 lineage；
- **Roundtable**：用户主持多个独立 Session 的多轮、可审计交接。

## 5. Product Lens 诊断

### Who

同时使用 Codex、Claude Code、Pi 或多个 Agent task 的开发者、Tech Lead 与小团队负责人。他们需要把研究、诊断、评审或实现结果交给另一个任务，但不希望手工复制整段 transcript，也不希望目标 Agent 获得源任务的全部权限和隐含上下文。

### Pain

当前替代方案主要是复制粘贴、共享整段 transcript、移动同一 Session，或发送一条无结构消息。它们很难回答：

- 具体传了哪些字段？
- 来源和 Evidence 是什么？
- 接收者是否接受、丢弃或要求补充？
- 这次交接是否绑定了正确任务、generation 和 workspace？
- 重启、竞争点击或失败后会不会重复继续？
- 是否夹带凭据、raw Grant、隐藏推理或无关敏感上下文？

### Why now

- Claude 与 Codex 的发布已建立用户心智，社区无需再被教育“为什么需要 handoff”；
- Loom 已完成最昂贵的 authority、Artifact、CAS、replay 与客户端基础；
- 竞争窗口更适合用现有能力形成清晰产品，而不是再等待完整 Phase 4。

### 10-star version

Provider-neutral Context Router：任何 Loom task、外部 Agent、human reviewer 或 Roundtable seat 都能提议一个有界 ContextPacket；用户预览来源、字段、Evidence、风险和 scope delta，选择吸收、继续、补充、转向、丢弃或归档；所有结果可重建、可撤销到下一条独立 lineage，并保持最小权限。

### MVP

从 chat-first Mission 中一键创建 research/diagnosis/review Side-task，完成后在同一界面看到结构化交接卡，明确预览将进入父任务的字段，并由用户选择 `absorb` / `continue` / `request_followup` / `discard`。整个旅程复用 P2B，不新增第二权威。

### Anti-goals

- 不同步 raw transcript 或隐藏推理；
- 不把 provider session/context ID 当 checkpoint；
- 不自动批准、自动执行或自动转发；
- 不在第一个预览版做跨账号、多用户、外部凭据或公网 endpoint；
- 不用 Roundtable UI 掩盖 P2B 与 P2C 尚未产品化的入口问题。

### Success metrics

- 新用户从启动到完成第一次可验证 handoff 的时间不超过 3 分钟；
- 一个公开、可重复、本地离线旅程可从干净环境完成；
- handoff packet 对 raw transcript、credential、raw Grant、隐藏推理的负向扫描为零；
- stale view/generation、wrong digest 与竞争决定均 fail closed；
- 每次 handoff 最多一个 ContextPacket 与一个 destination continuation；
- daemon 重启后，两个客户端看到同一交接状态与决定；
- 社区反馈能复述差异为“受治理的上下文交接”，而不是“又一个 Session sync”。

## 6. 优先级

以下 ICE 是方向性排序，实施前仍需 exact owned-path 与当前 P2C dirty boundary 审核。

| 候选 | Impact | Confidence | Effort | ICE | 决策 |
|---|---:|---:|---:|---:|---|
| G0：公开定位、用户指南、既有证据演示材料、干净 preview baseline | 5 | 5 | 1 | 25.0 | 立即做 |
| H1：chat-first 入口 + 现有 P2B handoff 卡片/预览/typed decision | 5 | 5 | 2 | 12.5 | 第一产品切片 |
| G1：新的公开可重复 demo harness（若需代码或 authority write） | 4 | 5 | 2 | 10.0 | 单独冻结 |
| H2：任意 Loom task/session 的显式 source→target ContextPacket | 5 | 4 | 3 | 6.7 | 第二产品切片 |
| RT1：本地双 seat、主持人门禁、Journal relay ledger | 4 | 4 | 4 | 4.0 | H1 后启动；可单独冻结 |
| RT2/RT3：真实 seat 投递、macOS/TUI Roundtable 旅程 | 4 | 3 | 5 | 2.4 | RT1 后 |
| 外部 A2A/Claude/Codex adapter、跨设备 sync | 4 | 2 | 5 | 1.6 | 后置实验 |

## 7. 建议的加速路线

### G0 — 先把已经有的能力变成社区能看见的产品

不改 authority：

1. 选择一个包含 `6d380233` 且验证可重现的干净 release baseline；不要从当前脏工作区直接发布；
2. README 首屏加入 Governed Handoff 定位和一张完整流程图；
3. 新增独立用户指南，解释 proposal → confirm → summary Artifact → decision → ContextPacket → continuation；
4. 用已验收、已锁定的 P2B 证据制作截图、短视频和 demo storyboard，不重跑已 consumed 的 canary；
5. 展示 macOS 与 TUI 同一状态、错误路径和重启恢复；
6. 明确标注 Development Preview、当前 parent/side-task 限制和非生产激活边界；
7. 如果社区可重复 demo 需要新增脚本、fixture、IPC 操作或任何 authority write，把它作为独立 G1 边界冻结、验证，不借 G0 文档工作夹带产品变化。

这一阶段的目标是先“占住定义”：Loom 不是另一个 session mover，而是 governed context transfer。

### H1 — 把 P2B 从 Mission Workbench 深处带到 chat-first 主路径

一个纵向产品切片，复用现有 `side_task_handoff` service/IPC：

```text
chat / mission
  -> Create Side-task
  -> preview purpose, mode, scope and destination parent
  -> explicit confirm
  -> independent execution + Evidence
  -> handoff card with summary, findings, risk and digest
  -> user typed decision
  -> at most one bounded ContextPacket / continuation
```

此切片不新增 handoff Event、不改变 P2B schema、不引入自动 admission。它只解决可发现性、可理解性和普通用户的 time-to-value。应在 Phase 2C repair 的 owned-path 冲突闭合后，使用独立干净 worktree 冻结。

### H2 — 从 parent/side-task 扩展到 task-to-task

这是第一个需要新合同的能力：用户选择 source task 与 destination task，Loom 生成可预览 ContextPacket proposal；目标任务显式接受后才写入自己的 lineage。

必须冻结：

- source/destination workspace、task、generation 与 view binding；
- 字段 allowlist、大小、digest、expiry 与 provenance；
- accept/reject/request-more；
- 双端权限与最小 scope；
- 幂等、并发、重启与目标失效语义；
- 不复制 raw transcript、credential、Grant 或完整 prompt。

### RT1 — Roundtable 只做新的协作状态，不复制 Handoff

原 `RT-R1` 应缩窄为：

- session、moderator、seat、round/message、pending/ack、insert/drop/conclude facts；
- moderator-only relay 与每跳确认；
- message ID + digest 幂等；
- body ≤ 8 KiB、Artifact refs digest-only；
- replay、conclude 后拒写、seat unavailable；
- `Conclude` 只发布规范化、digest-bound 的 `RoundtableAlignmentSummary` Artifact Candidate，并在 Roundtable Journal stream 记录 digest；RT1 不生成目标 `ContextPacket`、不继续其他任务；
- 后续若要把 alignment summary 交给任意 task/session，必须先冻结一个通用 handoff-envelope 抽取或 adapter 合同，显式处理 P2B parent/side-task schema 与 Roundtable schema 的差异。

### RT2/RT3 — 本地投递与界面

先接 Loom 内部 seat，再做 macOS/TUI 双 seat 界面和真实跨客户端旅程。A2A data shape 可以保留，但外部 AgentCard/JSON-RPC、跨账号身份与凭据 adapter 必须后置。

## 8. 最小可传播演示

社区演示只讲一个故事：

1. 父 Mission 遇到未知问题；
2. 用户创建一个最小权限 diagnosis Side-task；
3. Side-task 返回 Evidence-linked summary；
4. UI 明确显示风险、uncertainty、scope delta 与将被传递的字段；
5. 用户选择 `absorb`；
6. 父 Mission 只收到一个 digest-bound ContextPacket；
7. daemon 重启后，macOS 与 TUI 仍显示同一决定，重复点击不会二次继续。

不要在首个演示里同时讲 Team Builder、Scheduler、Autopilot、Sandbox、Memory、A2A 与所有七种 decision。它们是能力证明，不是第一条产品故事。

## 9. Go / No-Go

### GO

- 立即准备 G0 社区定位与干净 preview；
- 将 H1 作为 P2C 闭合后的第一个纵向产品切片；
- 把 Roundtable 从“Phase 4 最后再说”前移为 H1 后的独立差异化轨道；
- 复用既有 Journal/Evidence 基础和 P2B 已验证的设计约束；P2B 的 parent-specific `ContextPacket`/authority 只有在独立 generic-extraction 合同通过后才能共享。

### CONDITIONAL GO

- H2 与 RT1 可开始合同发现，但必须先解决当前工作区身份、owned-path 和单 writer 冲突；
- Roundtable 领域可独立设计，但 service/IPC/UI 接入不得与 P2C repair 并行写同一文件。

### NO-GO

- 从当前脏分支直接 push/release；
- 把 Claude `SendMessage`、Claude teleport 与 Codex host handoff 当成同一竞品能力；
- 为 Roundtable 新建第二套 Journal、Artifact summary、ContextPacket、decision 或 projection authority；
- 第一版直接做外部 endpoint、跨账号、多用户或自动 relay。

## 10. 建议下一步

下一次任务先只做一个无产品写入边界：

> **G0 Governed Handoff Category Claim：选择干净 baseline，整理已验收证据，补齐定位、用户指南、截图/短视频与 demo storyboard；不改产品代码、authority 或 schema，不发布。**

G0 验证后，等 Phase 2C repair 的 owned-path 冲突闭合，再单独冻结 H1 chat-first 接入；新的可重复 demo harness 是 G1，若触及代码或 authority write 也单独冻结。H1 通过后再冻结 RT1 moderator/seat/relay ledger。这样既能最快形成社区可见差异，也不会牺牲 Loom 已经建立的治理可信度。
