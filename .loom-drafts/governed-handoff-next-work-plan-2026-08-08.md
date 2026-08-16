# Governed Handoff 下一步执行计划

Date: 2026-08-08

Status: `DRAFT` — 非权威任务拆分；只用于排序、依赖与验收讨论，不授权产品写入、Journey、发布、push 或 merge。

Inputs:

- [`session-handoff-differentiation-product-brief-2026-08-08.md`](session-handoff-differentiation-product-brief-2026-08-08.md)
- [`session-handoff-competitive-research-2026-08-08.md`](session-handoff-competitive-research-2026-08-08.md)
- [`PHASE-2C-REPAIR-AMENDMENT.md`](../.loom-evidence/phase2c/contracts/PHASE-2C-REPAIR-AMENDMENT.md)
- accepted P2B commit `6d380233`

## 1. 执行结论

接下来不是直接做 Roundtable。按以下顺序推进：

```text
Lane A — 当前主线（单 writer）
P2C candidate lock
  -> fresh deterministic matrix
  -> one-fixture J1-J10 + evidence
  -> independent reviews
  -> Product Owner sign-off + accepted local commit
  -> H1 chat-first Governed Handoff

Lane B — 可安全并行（只读/草稿）
G0 message pack
  -> P2B evidence map
  -> screenshot/video storyboard
  -> wait for accepted clean baseline
  -> apply reviewed public docs locally

After H1
  -> decide/package G1 public demo harness
  -> H2 task-to-task governed transfer
  -> RT1 moderated Roundtable ledger
  -> RT2/RT3 local seats and clients
  -> external adapters last
```

当前 P2C repair 已无已知 P0/P1 实现缺陷。真正阻塞 H1 的是 J1-J10、证据、独立评审与 Product Owner sign-off。H1 会写 `LoomWorkspaceShell.swift`、`LocalProductStore.swift`、`MissionWorkbench.swift` 和 `internal/tui/*`，与当前 P2C dirty ownership 重叠，因此不能抢跑。

## 2. Phase A — 先关闭 P2C Repair

### Task A1：冻结当前 Repair Candidate 身份与边界

**Description:** 在任何 Journey 前，把物理 cwd、Git top-level、HEAD、合同版本、Candidate 文件清单和所有排除项绑定到同一记录，保证后续证据不会落在漂移源码上。

**Acceptance criteria:**

- [ ] cwd、Git top-level、分支与 `docs/CURRENT.md` 指向同一仓库；
- [ ] P2C owned-path 清单覆盖当前 repair 源码、测试、合同与 evidence，且不包含 phase1、`.codex/**`、`.loom-drafts/**`、build 产物或其他用户改动；
- [ ] `CURRENT/PARTIAL/TARGET` 与 Repair Amendment 一致；Phase 2C 仍标记 `PARTIAL`；
- [ ] 已冻结的 Repair Amendment 与 Exit Contract 默认只读；若 hash/bytes 必须改变，A1 立即停止，先完成独立 Contract Amendment Review PASS，再重新开始 source lock；
- [ ] Candidate source hash/lock 在 Journey 前冻结，之后任何 drift 都使 Journey 失效。

**Verification:**

- [ ] `git status --short` 与 exact owned-path inventory 人工对照；
- [ ] `git diff --check`；
- [ ] source-lock 重算与记录值一致；
- [ ] frozen contract hashes 与最近一次适用 Review 绑定；
- [ ] 无 staging、commit 或 live action 混入 preflight。

**Dependencies:** None.

**Files likely touched:**

- `docs/CURRENT.md`
- 新的 P2C repair source inventory/lock evidence

`PHASE-2C-REPAIR-AMENDMENT.md` 与 `PHASE-2C-EXIT-CONTRACT.md` 在本任务中是 read-only inputs；如需修订，进入独立 contract-amendment 子流程，不算 A1 的普通状态整理。

**Estimated scope:** S — 合同/证据，不改产品行为。

### Task A2：Fresh deterministic matrix

**Description:** 对冻结源码重新运行完整确定性矩阵，不能把 checkpoint 中的历史 PASS 当作最终验收 PASS。

**Acceptance criteria:**

- [ ] Go full/race/vet/tidy/gofmt/diff 全部通过；
- [ ] Swift full/TSAN/Release 全部通过，记录测试数、skip 和既有 warning；
- [ ] configured conversation adapter、shared local model、zero-Journal chat、TUI thread identity、socket single-winner 重点回归通过；
- [ ] 输出与 exact source lock 绑定。

**Verification:**

- [ ] `go test ./... -count=1`
- [ ] 相关 package 的 fresh `go test -race`
- [ ] `go vet ./...`
- [ ] `go mod tidy -diff`
- [ ] `gofmt -l` 对 Candidate Go files 无输出
- [ ] `swift test --package-path apps/macos`
- [ ] Swift TSAN suite
- [ ] `swift build --package-path apps/macos -c release`

**Dependencies:** A1.

**Files likely touched:** 只新增 verification evidence；测试失败时停止并回到原 W1/W2/W3 repair ownership。

**Estimated scope:** M — 一次完整验证 session。

### Task A3：执行单一 clean fixture 的 J1–J10

**Description:** Native macOS 与真实 PTY TUI 必须共享同一个干净 daemon fixture，覆盖 fresh launch、folder、offline/reconnect、conversation、governance、runtime route、restart、宽度与键盘旅程。

**Acceptance criteria:**

- [ ] J1–J10 全部在同一 source lock 与 clean daemon root 下完成；
- [ ] daemon restart 前后 conversation、event/projection 与 panel state 符合合同；
- [ ] native 与 TUI 看到同一服务状态和权威视图；
- [ ] 普通 chat 仍为 tentative output，产生零 Journal authority facts；
- [ ] 没有 inert button、placeholder tab、raw transport error 或重复 service-state presenter。

**Verification/evidence:**

- [ ] `.loom-evidence/phase2c/journeys/JOURNEY-MANIFEST.md` 全项闭合；
- [ ] light/dark、900/1080/1440pt screenshots；
- [ ] PTY transcripts；
- [ ] redacted IPC summaries；
- [ ] restart/projection comparison；
- [ ] keyboard/accessibility traversal；Computer Use 继续跳过，除非另获授权，manual VoiceOver 可作为合同允许的证据；
- [ ] postflight 无遗留 socket、lock、daemon、model process 或临时凭据。

**Dependencies:** A2.

**Files likely touched:** `.loom-evidence/phase2c/journeys/**`、review evidence；不应再改产品代码。若 Journey 暴露产品缺陷，立即停止，回到对应 W1/W2/W3 repair，修复后从 A1 重新锁定。

**Estimated scope:** M/L — 不拆成多个独立 fixture，避免证据不连续。

### Task A4：独立评审、Phase 接受与本地原子提交

**Description:** 对 exact Candidate 与 Journey evidence 完成全部独立门禁，再请求 Product Owner sign-off。

**Acceptance criteria:**

- [ ] W1/W2/W3 implementation review、dual-Result review、whole-WorkItem review 全部 PASS；
- [ ] Phase contract/implementation/dual-Result/whole-Phase review 全部 PASS；
- [ ] 历史失败和 superseded evidence 保留，不改写为 PASS；
- [ ] Product Owner 明确签署后，Phase 2C/ADR-0015 才能标记 accepted；
- [ ] exact-path staging inspection 后形成一个本地原子 Candidate commit；不 push/merge。

**Verification:**

- [ ] Reviewer 重算 source/evidence hashes；
- [ ] Reviewer 独立复现合同要求的核心矩阵；
- [ ] staged path 与 owned-path 完全一致，无排除项或 secret sentinel；
- [ ] commit 后工作区只保留原有排除项。

**Dependencies:** A3.

**Estimated scope:** M — 多个只读 reviewer 可并行，唯一 writer 仍串行。

## 3. Phase B — 与 A 并行准备 G0，但不碰产品代码

### Task B1：形成 Governed Handoff 社区 message pack

**Description:** 把现有产品简报压缩成社区能复述的定位、对比、限制和一个演示故事。先写在 `.loom-drafts`，不要修改当前 dirty README。

**Acceptance criteria:**

- [ ] 主定位是 governed context transfer，不是 session sync；
- [ ] Claude relay、Claude teleport、Codex relocation、Loom governed handoff 清楚分开；
- [ ] 所有能力标为 `CURRENT/PARTIAL/TARGET/EXPERIMENTAL`；
- [ ] 明示 P2B 只支持 parent/side-task，不声称 generic task-to-task 或 Roundtable 已实现；
- [ ] 不含发布、性能、安装或 production activation 的未经验证承诺。

**Verification:**

- [ ] 每条竞品声明链接官方一手来源；
- [ ] 每条 Loom CURRENT 声明链接 `6d380233` 或 P2B evidence；
- [ ] negative claim scan 无 “Roundtable implemented”“generic session handoff”“stable install”。

**Dependencies:** None; 可与 A1–A4 并行。

**Files likely touched:** `.loom-drafts/governed-handoff-launch-copy-2026-08-08.md`。

**Estimated scope:** S.

### Task B2：P2B evidence map 与 storyboard

**Description:** 使用已验收、已 consumed 的 P2B evidence 讲清 proposal → confirm → child lineage → summary Artifact → typed decision → ContextPacket → restart。不得重跑或替换 `canary-001`。

**Acceptance criteria:**

- [ ] 索引 `controlled-offline-canary.md`、`result-review.md`、`final-candidate-lock.json`、source locks 与 visual-only result；
- [ ] 记录 296 Events、零 duplicate、exact-one ContextPacket/continuation/effect/decision、18 Artifacts 与 secret sentinel 结果；
- [ ] screenshot/AX 只按 retained hash 与 visual-only 身份使用，不冒充第二 authority canary；
- [ ] 处理 input Artifact schema 文字差异：保留历史 contract，不重写；用 clarification 指向 accepted amendment 与当前 v2 code truth。

**Verification:**

- [ ] 重算 repo 内 P2B evidence/lock hashes；
- [ ] retained screenshot/AX 若仍存在，校验其 SHA-256；若不存在则如实标记 unavailable，不伪造；
- [ ] 所有相对链接有效。

**Dependencies:** None; 可与 A1–A4 并行。

**Files likely touched:** `.loom-drafts/governed-handoff-evidence-map-2026-08-08.md` 与 storyboard 草稿。

**Estimated scope:** S.

### Checkpoint B：等待 clean accepted baseline

- [ ] B1/B2 已独立文档复核；
- [ ] A4 完成，得到 accepted P2C commit；
- [ ] 从 accepted commit 创建干净隔离 worktree 的动作另获实施授权；
- [ ] 当前脏 branch 不直接用于 release、tag 或 push。

### Task B3：在干净 baseline 应用 G0 docs

**Description:** P2C 接受后，把已复核 message/evidence 内容应用到公开文档；这是 docs-only Candidate，不包含 demo harness 或产品代码。

**Acceptance criteria:**

- [ ] README 首屏准确展示 Governed Handoff 与 Development Preview 限制；
- [ ] 新增 `docs/product/GOVERNED-HANDOFF-GUIDE.md`；
- [ ] `docs/product/README.md`、Capability Matrix 与 Native/TUI guides 链接一致；
- [ ] 现有 Side-task 操作键和 native flow 与代码一致；
- [ ] 不改 P2B 历史合同，不重跑 consumed canary，不发布。

**Verification:**

- [ ] docs-only path inspection；
- [ ] Markdown link check、`git diff --check`；
- [ ] CURRENT/TARGET negative claim scan；
- [ ] independent docs/claim review PASS。

**Dependencies:** A4 + Checkpoint B.

**Files likely touched:**

- `README.md`
- `docs/product/GOVERNED-HANDOFF-GUIDE.md`
- `docs/product/README.md`
- `docs/product/CAPABILITY-MATRIX.md`
- `docs/product/NATIVE-APP-GUIDE.md`
- `docs/product/TUI-GUIDE.md`

**Estimated scope:** M — 一个独立 docs commit，不 push。

## 4. Phase C — H1 Chat-first Governed Handoff

H1 只能在 A4 之后冻结。它是一个纵向 WorkItem，下面是同一 WorkItem 内的实现任务，不得拆成多个平行 authority。

### Task C1：冻结 H1 合同与 Mandatory RED

**Description:** 只解决入口和呈现：从 chat-first Mission 显式创建现有 P2B Side-task，在 timeline/inspector 显示 handoff 卡，并调用既有 typed decisions。不得新增 Event、schema、policy auto-admission 或第二 handoff authority。

**Acceptance criteria:**

- [ ] exact owned paths 与 P2C accepted source lock 绑定；
- [ ] 普通 chat 零 Side-task lifecycle write；
- [ ] explicit action 先生成 zero-write proposal，二次确认才 create；
- [ ] UI 只调用既有 `side_task_handoff` propose/create/read/decide；
- [ ] 如果实现需要修改 `internal/work/**`、P2B Event/schema 或 generic ContextPacket，合同停止并重新规划，不在 H1 扩项。

**Verification:** Mandatory RED 只因缺少 H1 UI behavior 失败，既有 P2B domain/API tests 保持绿。

**Dependencies:** A4.

**Estimated scope:** M — contract + RED。

### Task C2：Native 纵向路径

**Description:** 在 chat timeline/composer 暴露 `Create Side-task`，显示 proposal preview、explicit confirm、result card 和 typed decision；详情仍可进入 governance inspector。

**Acceptance criteria:**

- [ ] purpose/mode/scope/parent identity 在确认前可见；
- [ ] summary、finding、risk、uncertainty、scope delta、Evidence/digest 与可用决定在结果卡可见；
- [ ] stale/digest/identity/recoverable error 使用现有 sanitized state；
- [ ] conversation 与 panel state 在 sheet/decision/restart 后保留。

**Likely files:**

- `apps/macos/Sources/LoomLocalAppUI/LoomWorkspaceShell.swift`
- `apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift`
- `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
- 对应 Swift tests

**Dependencies:** C1.

**Estimated scope:** M — 3–5 product/test files。

### Task C3：TUI 等价路径

**Description:** 在 `ScreenHome` 的 chat-first flow 提供相同 proposal/confirm/result/decision journey，不要求用户先切到旧 Mission screen。

**Acceptance criteria:**

- [ ] 宽/窄 terminal 都能完成完整 handoff；
- [ ] visible keys 全部有真实行为；
- [ ] selected proposal、result 与 decision 在 resize/reconnect 后保留；
- [ ] native 与 TUI 读取同一 Go service/Projection。

**Likely files:**

- `internal/tui/model.go`
- `internal/tui/model_test.go`
- `internal/tui/style.go`（仅确有需要）

**Dependencies:** C1；可在 C2 的 H1 schema/UI vocabulary 冻结后实现，避免两端命名漂移。

**Estimated scope:** M。

### Task C4：H1 Cross-client journey 与接受

**Description:** 用真实 daemon、native window、生产 Swift client 与 PTY TUI 验证同一 handoff，并完成独立评审。

**Acceptance criteria:**

- [ ] ordinary chat 不创建 Side-task；
- [ ] proposal zero-write，confirm exactly-one admit；
- [ ] child summary Artifact 与 Evidence 完整；
- [ ] `absorb` exactly-one ContextPacket/continuation；
- [ ] `discard` 零 parent disclosure；
- [ ] duplicate/stale/wrong-digest 失败且零重复 effect；
- [ ] daemon restart 后两客户端状态一致；
- [ ] Go full/race/vet/tidy/gofmt/diff 与 Swift full/TSAN/Release 对 exact H1 source lock 全部 PASS；
- [ ] independent implementation/result/whole-Candidate reviews PASS；
- [ ] Product Owner 明确 sign-off 后才将 H1 标记 accepted 并执行 exact-path staging；
- [ ] sign-off 后形成一个本地原子 commit，不 push。

**Verification matrix:**

- [ ] `go test ./... -count=1`
- [ ] H1/P2B/UI 影响 package 的 fresh `go test -race`
- [ ] `go vet ./...`
- [ ] `go mod tidy -diff`
- [ ] Candidate Go files `gofmt -l` 无输出
- [ ] `git diff --check`
- [ ] `swift test --package-path apps/macos`
- [ ] Swift TSAN suite
- [ ] `swift build --package-path apps/macos -c release`
- [ ] Reviewer 对 source/evidence hashes、non-disclosure、exactly-once 与 restart evidence 独立复验

**Dependencies:** C2 + C3.

**Likely files:** 新 H1 journey script/evidence；是否可作为公共 G1 demo 由下一个 checkpoint 决定。

**Estimated scope:** M。

## 5. H1 后的决策顺序

### Checkpoint C：是否需要 G1 public demo harness

优先复用 C4 的已评审 Journey。只有当社区无法在干净 checkout 重复它时，才冻结独立 G1：私有 temp root、离线/无凭据、确定性 fixture、exact Event counts、restart/reconnect、secret scan 与 cleanup。不得复用 `canary-001`，不得借 demo 扩 Event/schema。

### H2：任意 task-to-task Governed Transfer

H2 是下一项需要新 authority 合同的能力。它必须定义 source/destination workspace、task、generation、view、expiry、allowlist、digest、accept/reject/request-more、双端 scope 与失败恢复。P2B parent-specific `ContextPacket` 不能直接当通用接口；需要独立 generic envelope/adapter 评审。

### RT1：Moderated Roundtable Ledger

RT1 在 H2/generic-envelope 方向明确后再冻结，只新增 moderator/seat/round/message/pending/ack/insert/drop/conclude facts。`Conclude` 只生成 digest-bound `RoundtableAlignmentSummary` Artifact Candidate，不继续其他任务，不自动 relay，不新建第二 Journal/Evidence/ContextPacket authority。

### RT2/RT3 与外部 adapters

先做 Loom 内 local seats 与 macOS/TUI 旅程；A2A AgentCard/JSON-RPC、Claude/Codex adapters、跨账号、多用户与公网 endpoint 最后，且分别冻结身份、凭据、fail-closed 与撤销边界。

## 6. 并行规则

| 可并行 | 必须串行 |
|---|---|
| A lane 的只读 reviewer | A1 → A2 → A3 → A4 |
| B1/B2 草稿与 A lane | P2C accepted → B3 public docs |
| 独立竞品/文档核查 | P2C accepted → C1 → C2/C3 → C4 |
| C2/C3 在 vocabulary 冻结后的独立实现探索 | 所有 shared service/schema/authority 写入 |
| H2/RT1 的只读 contract discovery | 目标 branch 的 integration 与 release |

同一 branch/Candidate 始终只有一个 writer。任何会改 `LoomWorkspaceShell.swift`、`LocalProductStore.swift`、`MissionWorkbench.swift`、`internal/tui/*`、daemon 或 strict IPC 的任务，在 P2C A4 前都不得启动。

## 7. Stop conditions

- J1–J10 暴露产品缺陷：停止 review，回到对应 P2C repair ownership，从 A1 重新锁定；
- source/evidence digest drift：现有 Journey/Review 不再适用；
- G0 需要新 script、fixture、IPC 或 authority write：移到独立 G1；
- H1 需要改 P2B Event/schema/authority：停止 H1，另立合同；
- retained screenshot/AX 不存在或 hash 不符：如实标记 unavailable，不补造证据；
- 需要 Computer Use、production activation、push、merge、发布或远端资源变化：先取得明确授权；
- 工作区/HEAD/GoalSpec 身份不一致：所有写入和 dispatch 停止。

## 8. 现在就做什么

1. 主 writer 执行 **A1 → A4**，先把 Phase 2C 接受掉；
2. 另一个只读/草稿 lane 同时完成 **B1 + B2**；
3. A4 通过后，在干净 accepted baseline 做 **B3**；
4. 随后只冻结一个产品 WorkItem：**C1–C4 / H1 Chat-first Governed Handoff**；
5. H1 接受后再决定 G1，随后进入 H2，最后 RT1。

最重要的近期里程碑不是“Roundtable 代码出现”，而是：

> **一个新用户能从 chat-first 入口完成一次 Evidence-linked、explicit-confirmed、exactly-once、restart-safe 的 Governed Handoff。**
