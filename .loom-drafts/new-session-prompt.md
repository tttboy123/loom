# Loom Phase 1 Slice 1 · 新 Session 启动 Prompt

> 用途：给新 Codex session 启动 Phase 1 Slice 1 工作的输入 prompt。
> 配套 GoalSpec：`.loom-drafts/phase1-slice1.optimized.goalspec.yaml`
> 不重复 GoalSpec 内部（work_items / acceptance / governance），让新 session 自己读 YAML。

## 身份与工作区
你是 Loom Platform Phase 1 Slice 1 的 primary Codex Controller session。
- 工作区: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- 分支: `codex/loom-platform`（HEAD `0ccb31b`，implementation-not-started）
- 执行模式: `codex_loop_engineering` = `docs/DEVELOPMENT.md §4.1`

## 必读（按顺序，不重复读）
1. `.loom-drafts/phase1-slice1.optimized.goalspec.yaml` — 你的工作合同（work_items / acceptance / evidence / governance 全在这里）
2. `AGENTS.md` — root invariants
3. `docs/AGENTS.md` — doc 规则
4. `docs/DEVELOPMENT.md` — 重点 §1.2 / §3 / §4.1 / §5 / §6 / §7
5. `docs/CURRENT.md` — volatile status

按需读（不进 active WorkItem 不必读）：
TECH-PLAN §13.1 / §14 Slice 1 / §15 / §16；
ADR-0002 / ADR-0006；
`.codex/agents/*.toml`。

## 跨项目硬规则（你的 memory，不在 GoalSpec 里）
- 每个 WorkItem 完成后 `deliverable.md` 末尾必须有 `VERDICT: PASS|FAIL` 行（最后非空行）。缺它 = auto-reject。
- failure code / status / enum 写到 prompt 前先 `grep -rn` 实际代码，不要让 worker 改 enum。
- 任务状态变更 3 目标同步：TodoWrite + `<workspace>/PROGRESS.md` + memory。
- 真实 test terminal > model 文字判断。"Should work" / "committed" 不算 evidence。

## Preflight（5 条）
cd /Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild
pwd && git rev-parse --abbrev-ref HEAD && git rev-parse --short HEAD
git status --short | head -50
ls .codex/agents/ AGENTS.md docs/AGENTS.md docs/DEVELOPMENT.md docs/CURRENT.md .loom-drafts/
go version 2>/dev/null || echo "no go"

## 工作流（DEVELOPMENT.md §4.1 别名）
1. 读 CURRENT.md + active slice。
2. 必要时 Code Analyst 给出 bounded evidence（`code_analyst.toml`，不是 `code-analyst.toml`）。
3. Controller 冻结一个 WorkItem contract：id / risk / owned files / acceptance / checks / depends_on / **corresponds_to**。
4. Developer 加最小 RED test → 最小实现 → focused GREEN → impact union。
5. Fresh Reviewer 检 contract + diff + test evidence。
6. 同 lineage 修复；二次同类失败升级 problem_analyst。
7. 更新 CURRENT / PARTIAL / TARGET 文档。
8. **不** commit / push / merge / 装 FastContext / 切工作树。

## WorkItem 顺序（按 depends_on）
S1-W1 (standard) → S1-W2 (strict) → S1-W3 (strict) → S1-W4 (strict) → S1-W5 (standard)
前一个 GREEN + Reviewer verdict 才能开下一个。

## 完成报告（DEVELOPMENT.md §7 + memory 硬规则 §1）
1. 改了什么
2. 成功与否 + 精确 evidence（test 输出、run log、失败码、spec 行号）
3. 影响的文件 / 产品行为
4. 剩余风险 / 未验证 live boundary
5. 下一可执行步骤
6. **末尾硬写 `VERDICT: PASS|FAIL` 行**

## 硬边界（来自 GoalSpec governance）
- `terminal_boundary: phase1_slice1_only` — 不进 Slice 2
- `fastcontext_download_or_install: false` — 不装
- `max_product_repairs_per_lineage: 3` — 第三次失败 = HUMAN_REQUIRED
- `preserve_existing_worktree_changes: true`
- 不切到旧 `agent-platform` 工作树开发产品代码

## 启动
preflight → 读 GoalSpec → 冻结 S1-W1 contract → RED → GREEN → Reviewer → 下一 WI。
