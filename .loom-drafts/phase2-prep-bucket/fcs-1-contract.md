# FCS-1 Frozen WorkItem Contract (FastContext Local Spike)

- **ID**: `FCS-1`
- **Title**: FastContext Local Spike — Code Analysis Sidecar
- **Risk**: Strict (license + path escape + 召回率)
- **Status**: `CONTRACT_DRAFT` (blocked by Phase 2 governance 解锁)
- **Depends on**:
  1. S1 PASS at commit `5861f82`
  2. **显式 Spike gate 授权** (per ADR-0006 + CURRENT.md EXPERIMENTAL)
  3. **Phase 2 governance 重 freeze**, 覆盖 Phase 1 那 5 处 `fastcontext_download_or_install: false` 硬声明
- **Corresponds to**:
  - `docs/adr/0006-fastcontext-compatible-code-analysis-sidecar.md` (accepted 2026-07-24)
  - `docs/integrations/fastcontext.md` (209 行集成规范)
  - `docs/CURRENT.md` PARTIAL + EXPERIMENTAL 状态
- **Frozen branch/head**: `codex/loom-platform-slice2` at `f0820be` (假设不变)
- **WorkItem 路径**: `.loom-evidence/phase2/fastcontext-spike/FCS-1/`

## Owned files (本 spike 期间可写)

- `docs/integrations/fastcontext.md` (现有, 受 contract 约束)
- `cmd/loom/fastcontext/` (新建, Loom-owned read-only adapter)
- `internal/codeanalyst/` (新建, sidecar glue)
- `eval/fastcontext/` (新建, Loom-specific 评估 set)
- `.loom-evidence/phase2/fastcontext-spike/FCS-1/*` (evidence 路径, 跟 S1/S2 一致)

## Objective

1. **装 community FastContext SFT** (loopback-only local runtime, 不接外部)
2. **写 Loom-owned read-only adapter**, 暴露 `READ` / `GLOB` / `GREP` 3 个 tool, 验 path + 引用, 轨迹外存
3. **跑 Loom-specific 评估 set** (recall on Loom 真实 codebase, 5+ 个 query 场景)
4. **跑 1-2 周 spike**, 输出决策: ADR-0006 升 `ACCEPTED` 修订 / 保持 `EXPERIMENTAL` / 回退

## Frozen domain boundary (硬边界)

### In scope

- `internal/codeanalyst/` Go package (sidecar glue)
- `cmd/loom/fastcontext/` CLI 命令 (启动 / 停止 / 状态)
- `eval/fastcontext/` 评估 set 脚本
- Loom-owned adapter 实现 (READ/GLOB/GREP + 引用验证)

### Out of scope (严禁越界)

- ❌ **不抢 agent runtime** — 不参与 Run claim dispatch, 不接 Coordinator
- ❌ **不写 Loom State Store** — output 是 Candidate evidence, 不入 Event Journal
- ❌ **不生产化** — per ADR-0006 Decision 段
- ❌ **不推 remote** — loopback-only, 不开外部端口
- ❌ **不装 Microsoft FastContext** — 官方 repo 不可用, 论文 withdrawn (per ADR-0006 Context)
- ❌ **不替代 Loom deterministic acceptance** — 评估 set 失败不能拒 Loom CI
- ❌ **不抢 path containment** — root containment + secret denial 强约束

## Risk mitigation

| 风险 | 缓解 |
|---|---|
| **license** (community SFT 可能非 OSI) | 评估 license 兼容性, 不兼容 Loom MIT 就回退到 "FastContext 仅作为调研, 不实施" |
| **path escape** | 严格 root containment, secret denial, 零 source mutation outside model instructions (per ADR-0006 Consequences) |
| **召回率虚高** (token reduction 隐藏 recall) | Loom-specific 评估 set, 不靠 community benchmark (per ADR-0006) |
| **成本失控** | bounded by `cost_cap_usd`, 超 cap 自动 cancel (跟 P0-13 同步) |
| **prompt injection** | 拒绝 secrets / 拒绝 model instructions 之外的源变更 |
| **token reduction 抢主 context** | 评估 set 单独跑, 不入主 daemon context |

## Hard governance (Phase 1 期间不变量, Phase 2 启动时重审)

- `fastcontext_download_or_install: false` (Phase 1 期间) — **本 contract 实施前置: 必须先在 Phase 2 governance 重 freeze 时显式 unlock**
- `push: false` / `merge: false` / `release: false` / `production_activation: false` (per Phase 1 spec governance)
- `paid_remote_work: false`
- `credential_read_or_change: false`
- `source_history_rewrite: false`

## Acceptance (FCS-1 完成的硬证据)

- [ ] `cmd/loom/fastcontext/` 安装脚本可执行, 在 `localhost` 启动 community SFT
- [ ] `internal/codeanalyst/` Go package 通过 `go test ./...` + `-race` + `go vet`
- [ ] 5+ 个 Loom-specific 评估 query 跑通, 召回率 >= 80% (可调)
- [ ] 1-2 周 spike 期间 0 path escape + 0 secret leak + 0 source mutation
- [ ] 评估 set 输出报告 `.loom-evidence/phase2/fastcontext-spike/FCS-1/eval-report.md`
- [ ] 决策推荐落 `.loom-evidence/phase2/fastcontext-spike/FCS-1/adr-revision.md` (升 accepted / 保持 experimental / 回退)
- [ ] fresh read-only Reviewer 对 deliverable.md 出 `VERDICT: PASS`

## 不实施 = OK 的退出条件

spike 跑出**任意** 1 个问题:
- license 不可调和 → 立刻退出, ADR-0006 修订 "rejected: license conflict", 调研归档
- 召回率 < 60% → 退出, ADR-0006 修订 "rejected: recall insufficient"
- path escape / secret leak / source mutation 任 1 → 立刻退出, ADR-0006 修订 "rejected: safety violation"

**退出 ≠ 失败**, 是**显式决策**, 写进 ADR 修订。

## 估时 (授权后)

- 1: spike 启动 + 装 community SFT + adapter 写完 (3-4 天)
- 2: 评估 set 跑 1 周 (5 个工作日)
- 3: 报告 + ADR 修订 + Reviewer (1-2 天)
- **总计: ~2 周** (跟 P0-13 chaos test harness 类似)

## 同步规则 (3 目标, 跨项目硬规则 §6)

- 实施前置: 更新 PROGRESS.md 加 FCS-1 work item entry, TodoWrite 加 1 项 in_progress
- 实施中: 每 atomic commit 同步 TodoWrite + PROGRESS.md
- 实施完成: deliverable.md 末行 `VERDICT: PASS|FAIL` 必填
