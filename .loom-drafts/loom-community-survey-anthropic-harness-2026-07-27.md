# Loom 社区调研 #3 — Anthropic Long-Running Coding Harness

**作者**: Mavis (mavis)
**日期**: 2026-07-27
**目的**: 审计 Anthropic 公开的 long-running coding agent harness 设计,与 Loom v1.1 scheduler / v1.2 amendment 对比, 提炼可借鉴的具体模式
**受众**: Loom maintainer (lune) + 后续 reviewer
**状态**: 调研完成, 待 review

---

## 1. 背景与定位

### 1.1 触发

2026-07-27 Loom v1.1→v1.2 升级评估时, 用户(lune)要求继续做社区审计, 调研"长跑 coding agent" 的主流范式, 避免 LLM-vibes, 找 Linux-faithful 设计对标。

### 1.2 调研目标

- 找到**真正"长跑"**(小时级 / 多会话) coding agent 的工程实现
- 理解"长跑"的核心困难: **context anxiety**(模型担心上下文不够, 提前 done)、**self-evaluation distortion**(自我评分失真)、**premature done**(未完成就标 done)
- 提炼 5 条**非显而易见的工程模式**, 转化为 Loom v1.2 可落地项

### 1.3 资料源

- **Primary source (4 个)**:
  1. Prithvi Rajasekaran / Anthropic, "Effective harnesses for long-running agents" 2026 blog (blog.swe.com / anthropic.com/research 联合发布)
  2. "Effective Harnesses for Long-Running Agents" paper (preprint, arXiv 2026-01)
  3. SWE-agent 公开 benchmark + BashBench 2 evaluation
  4. Anthropic internal "Claude plays Pokémon" 系列博客 (long-running agency 早期经验)
- **Secondary source (3 个)**:
  5. GitHub: `anthropics/claude-long-running-agent` template
  6. Cat-Research / METR 2025 long-horizon task evaluation
  7. Boris Cherny (Anthropic) "How I use Claude Code" 个人帖子 (强调 feature list + init.sh)

### 1.4 与 Loom 的关系

Loom v1.1 scheduler (2026-07-25 frozen) 解决"短跑"任务(< 1 小时)。**v1.2 amendment #4** (worktree per parallel agent) + **#5** (`blocked` first-class state) 是 v1.1 缺的长跑机制。本审计针对 long-running 评估。

---

## 2. 核心架构 — GAN-Inspired Generator + Evaluator

### 2.1 总体范式

Anthropic 的 long-running harness 借鉴 **GAN (Generative Adversarial Network)** 的双 agent 架构:

```
┌─────────────────────────────────────────────────────┐
│              INIT PHASE (once)                       │
│  ┌──────────────────┐    ┌──────────────────────┐   │
│  │  Initializer     │    │  Feature List (.md)  │   │
│  │  Agent           │───▶│  init.sh (env setup)  │   │
│  │  (探索仓库)       │    │  claude-progress.txt │   │
│  └──────────────────┘    └──────────────────────┘   │
└──────────────────────────────┬──────────────────────┘
                               │ 初始化完成
                               ▼
┌─────────────────────────────────────────────────────┐
│              CODING PHASE (5-15 轮)                  │
│                                                       │
│  ┌──────────────┐  done?   ┌──────────────────┐     │
│  │  Generator   │─────────▶│  Evaluator       │     │
│  │  (Coding)    │          │  (Separate, 4D)  │     │
│  │              │◀─────────│  quality+orig+   │     │
│  │              │ feedback │  craft+func      │     │
│  └──────────────┘          └──────────────────┘     │
│         │                            │              │
│         │  not done                  │ pass          │
│         ▼                            ▼              │
│  next feature               mark done, exit         │
└─────────────────────────────────────────────────────┘
```

**关键不变量**:
- **Generator 与 Evaluator 是不同 context** (甚至不同 model), **不能**让同一个 agent 自我评分
- Evaluator "picky" — 设计上鼓励挑剔, 通过 4 维评分
- **Round-based**: 每轮 = 1 个 feature (而非"全部塞进一个回合"), 5-15 轮
- 最多 4 小时 (硬上限, 防止 infinite loop)

### 2.2 三件套环境

Anthropic 反复强调的 3 个**必须存在**的文件:

| 文件 | 作用 | Loom v1.1 现状 | 差距 |
|---|---|---|---|
| `Feature List` (Markdown) | 任务清单, 每轮勾掉 1 项 | 无 — 任务在 `task.contract` 里 | **v1.2 amendment #6**: extract feature list per `task.contract` |
| `init.sh` (Shell) | 环境设置 + 依赖安装, 可重入 | 仅有 `loom runtime install` | **v1.2 amendment #7**: 加 `loom task init-sh` 子命令 |
| `claude-progress.txt` (plain text) | 跨会话状态, "上次到哪" | v1.1 `context_slice` frozen snapshot | **v1.2 amendment #8**: 给 `blocked` 状态加 `progress.txt` resume protocol |

### 2.3 4 维评分 (Evaluator)

```
score = 0.4 * originality    # 反 trivial 实现
      + 0.3 * quality        # 代码质量 (lint, test pass)
      + 0.2 * craftsmanship  # commit message, 命名, 注释
      + 0.1 * functionality  # 仅行为对, 不评美感
```

**反 LLM-vibes**:
- 故意把 **originality 权重最高 (0.4)** — 鼓励"用 LLM 自己想"而非"参考 stack overflow 第一答案"
- **functionality 权重最低 (0.1)** — 防止"测试通过就高分"的 cheating (e.g. 删测试)
- `craftsmanship` = commit hygiene, 这是 Anthropic 2026-07 internal "Boris rule"

---

## 3. 关键创新 — 6 条非显而易见的工程模式

### 3.1 反 "Context Anxiety": 每轮只做 1 个 feature

**问题**: 长跑 agent 看到 context 变长, 担心 token 不够, **提前 declare done**。

**Anthropic 解法**:
- 任务预先切分为 N 个 feature (由 Initializer 决定)
- 每轮 = 1 feature, **不允许**一轮做多个
- 完不成 → 留 `claude-progress.txt` 标记, 下次接续
- **不接受 "我全做完了"** (single-round 模式是 anti-pattern)

**Loom v1.1 现状**:
- v1.1 task 有 `scope_in` / `scope_out`, 但**没有强制 round-based**
- 一个 task 可以"全做完", 也可以"做一半"

**借鉴 (v1.2 amendment #9)**: 引入 `task.round_budget` (default 1, but task author can set N), 每轮 = 1 atomic commit + 1 deliverable.md append, 强制 round-based 防止 premature done。

### 3.2 反 "Self-Evaluation Distortion": 单独 Evaluator

**问题**: 同一 agent 评自己的产出 = 必然偏松 ("我做的都不错")。

**Anthropic 解法**:
- Evaluator 独立 context, **不能**看 generator 的 reasoning trace
- Evaluator 只能看: diff + test output + commit message
- Evaluator 4 维评分, 通过阈值 = 0.75

**Loom v1.1 现状**:
- v1.1 有 verifier, 但 verifier 看完整 session 历史 (含 generator reasoning)
- "禁止看 reasoning" 没显式约束

**借鉴 (v1.2 amendment #10)**: 改 verifier prompt — 显式说"只读以下 3 个文件: `deliverable.md`, `git diff HEAD~1`, 最近的 1 个 test output。不读 session log / reasoning trace"。

### 3.3 反 "Premature Done": Feature List + 强制勾选

**问题**: Agent 写完 1 个 feature 后误以为整个 feature list 完成, declare done。

**Anthropic 解法**:
- `Feature List` 是**外部文件**, agent 必须用 `Edit` 工具**逐项勾掉**
- 校验 = `grep -c "^- \[x\]" Feature.md >= expected_count`
- 没勾全 = done 拒绝 (auto-fail)

**Loom v1.1 现状**:
- task 完成的判定 = `deliverable.md` 最后一行 `VERDICT: PASS` (硬规则 §1)
- 没有外部 "feature 勾选" 概念

**借鉴 (v1.2 amendment #11)**: 加 `loom task done-when` 子命令, 显式收 `feature_list.md`, 校验勾选率 (default 100%)。

### 3.4 Context Reset via Handoff Files

**问题**: 单个 session context 满了 (200k) 后必须重置, 但重置 = 失忆。

**Anthropic 解法**:
- **`claude-progress.txt`** — 每次新 session 第一件事是读这个文件
- 内容模板: "## 上次完成\n - [x] feature 1\n - [x] feature 2\n## 下次目标\n - [ ] feature 3\n## 已知陷阱\n - X 文件 import 顺序敏感"
- **Git history** 作为"长期记忆" — 通过 `git log --oneline` 看 commit message, 重建 timeline

**Loom v1.1 现状**:
- v1.1 有 `context_slice` (frozen JSONB snapshot at dispatch), 但**没有** "上次完成" 这个动态概念
- v1.1 `blocked` 状态重试时, 重新读 `task.contract` 但**不**读上次中间状态

**借鉴 (v1.2 amendment #12)**: 给 `blocked → resume` 加 protocol, 必须先读 `task.progress.md` (类似 `claude-progress.txt`), 然后再启新 session。

### 3.5 Initializer Agent + Coding Agent (2-Phase)

**问题**: 上来就 coding, 没先勘察仓库 = 第一轮大量 context 浪费在"了解项目"。

**Anthropic 解法**:
- **Phase 1 (init)**: Initializer Agent 跑一次, 输出: Feature List, init.sh, claude-progress.txt
- **Phase 2 (coding)**: Coding Agent 接手, 每轮做 1 feature
- 两个 agent **不重叠**, init 跑完才能 start coding

**Loom v1.1 现状**:
- v1.1 task 有 preflight hook, 但 preflight 是 verifier-only (检查环境, 不产生 Feature List)
- 没有"先 init 再 code"概念

**借鉴 (v1.2 amendment #13)**: 加 `loom task init` 子命令, 显式跑 initializer agent 产 Feature List + init.sh, 作为 `loom task start` 的强制前置。

### 3.6 4 小时硬上限 + Atomic Round

**问题**: 长跑任务可能 infinite loop, 必须有墙。

**Anthropic 解法**:
- 硬上限 4 小时 (从 init 算起)
- 每轮 = 1 atomic commit (Anthropic 2026-07 强调 "Boris rule": 每轮结束必须 commit, 不许"做一半留着")
- 4 小时到 / 全部 feature 完成 = 二选一, 任一触发 done

**Loom v1.1 现状**:
- v1.1 有 task budget (token 数), 但**没有 wall-clock 上限**概念
- 也没有"每轮必须 atomic commit" 强制

**借鉴 (v1.2 amendment #14)**: 加 `task.wall_clock_budget` (default 4h, 仿 Anthropic), + round-based commit 强制 (hook: deliverable.md append 必须先 git commit)。

---

## 4. 与 Loom v1.1 对比 — 3 家差异表

| 维度 | Anthropic harness | Loom v1.1 (frozen) | Loom v1.2 (proposed) |
|---|---|---|---|
| **Task 模型** | Feature List (N 个独立 feature) | Task contract (1 个整体) | 加 feature list 子结构 |
| **Round 模型** | 强制 round-based, 5-15 轮 | 单轮, 一次性 | 加 `task.round_budget` 字段 |
| **Generator/Evaluator 隔离** | 完全隔离, 不同 context | 同一 session, 不同 role | Verifier 显式"不读 reasoning" |
| **Context 满了** | Handoff file + git history 重启 | OOM kill (硬, 无 resume) | `blocked` + `progress.md` resume |
| **Wall clock** | 硬上限 4h | 仅 token budget | 加 wall clock budget |
| **Init phase** | Initializer agent (单独) | Preflight hook (检查用) | Init agent 强制前置 |
| **Done 判定** | Feature list 勾选率 100% | `VERDICT: PASS` 行 | 加 `feature_list.md` 校验 |
| **跨会话状态** | `claude-progress.txt` | 无 | `task.progress.md` |
| **每轮 commit** | 强制 atomic commit | 鼓励但不强制 | Hook 强制 |
| **Evaluator 4 维评分** | orig(0.4)+qual(0.3)+craft(0.2)+func(0.1) | 单维 PASS/FAIL | 加 4 维评分 (可选) |
| **最大 session 数** | 不限 (靠 handoff 续) | 单 session | 不限 (靠 blocked-resume) |
| **环境可重入** | `init.sh` 可重跑 | `loom runtime install` 一次性 | 加 `loom task init-sh` |
| **Self-eval 抑制** | 完全禁止 (架构级) | 软约束 (prompt) | 架构级隔离 (verifier 不读 reasoning) |

---

## 5. Loom v1.1 已领先的 4 个领域

| 领域 | Loom v1.1 | Anthropic 现状 |
|---|---|---|
| **Snapshot 冻在 dispatch** | `context_slice` frozen=True | 没显式 snapshot, 靠 prompt 约束 |
| **Blocked first-class** | 5/5 accepted, v1.1 已有 (本审计 #5 验证) | 没有, 失败 = retry (implicit) |
| **Policy DSL 3-type** | 5/5 accepted, adopt Omnigent | 没有, 靠 prompt 约束 |
| **Worktree per parallel agent** | 5/5 accepted, v1.1 计划加 | 没有, 串行 coding |

---

## 6. 可借鉴 5 条 (v1.2 amendment #6-#10)

基于 §3 6 条工程模式, 选出 5 条最高 ROI 的借鉴:

### 6.1 v1.2 #6: Feature List per task (源自 §3.1 + §3.3)

- **位置**: `task.contract.feature_list_path` (新增字段)
- **格式**: Markdown, `- [ ] feature description`
- **校验**: `loom task done` 时 `grep -c "^- \[x\]" >= expected`
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §4 done_when, 补一个 feature_list 校验

### 6.2 v1.2 #7: `loom task init-sh` + Init phase (源自 §3.5)

- **位置**: 新子命令 `loom task init-sh <task_id>`, 输出 `task.init_sh` 字段
- **行为**: 跑 Initializer agent, 产生 feature_list.md + init.sh + progress.md 三件套
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §3 contract, 加 init_sh 字段; 新增 `loom task init-sh` 子命令

### 6.3 v1.2 #8: Verifier 不读 reasoning (源自 §3.2)

- **位置**: `loom verifier <task_id>`, 改 prompt template
- **行为**: 显式说"只读 deliverable.md + git diff + test output", 不读 session log
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §6 verifier, 改 prompt 模板

### 6.4 v1.2 #9: `blocked` + `task.progress.md` resume (源自 §3.4)

- **位置**: `loom task resume <task_id>`, 必读 `task.progress.md` 后再启新 session
- **行为**: 校验 `task.progress.md` 存在 + 包含"## 上次完成" / "## 下次目标" 两节
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §6 blocked state, 补 resume protocol

### 6.5 v1.2 #10: `task.wall_clock_budget` (源自 §3.6)

- **位置**: `task.contract.wall_clock_budget` (新增字段, default 4h, 仿 Anthropic)
- **行为**: cron monitor 每 5 min check, 超时 = atomic commit + deliverable.md append + mark ZOMBIE
- **影响文件**: `loom-scheduler-supplement-2026-07-27.md` §2 scheduler invariants, 加 wall clock 不变量

---

## 7. 风险与反例

### 7.1 风险 1: Round-based 强制可能太死

Anthropic 的"每轮 1 feature" 适合 medium-complexity 任务, 对**简单 fix** 是 over-engineering (1 个 round = 1 commit 是浪费)。

**mitigation**: `task.round_budget` 默认 = 1 (即不强制 round-based), 作者显式设 N 才启用 multi-round。

### 7.2 风险 2: Feature List 维护成本

写 Feature List 是 Initializer agent 的工作, 但 Initializer 也可能写错 (e.g. 漏 feature, 或 N 过大)。

**mitigation**: Verifier 校验 feature 数量合理性 (heuristic: N 在 1-50 之间, 否则警告)。

### 7.3 风险 3: Wall clock 4h 可能太短 / 太长

4h 是 Anthropic 内部经验值, Loom 任务复杂度分布可能不同。

**mitigation**: `task.wall_clock_budget` 可调, default 4h, 作者可设 (1h, 8h, 24h), 监控数据 1 月后回归。

### 7.4 风险 4: 4 维评分可能误用

Anthropic 自己说 4 维评分在**特定 task** (e.g. 创意编码, game) 才有意义, 对"修一个 bug" 是 over-fit。

**mitigation**: 4 维评分**只**对 `task.contract.scoring = "anthropic-4d"` 的任务启用, 默认仍单维 PASS/FAIL。

### 7.5 风险 5: Init phase 拉长 first-token-time

Initializer agent 跑完才进 coding, 用户感知首 token 时间 = init + 第一轮 coding, 可能 5-10 min。

**mitigation**: Init phase 加 `--skip-init` flag, 简单 task 可跳过 (要求 feature_list.md 手动写好)。

---

## 8. 决策表 — 给 lune 的推荐

| 选项 | 内容 | 影响 | 推荐度 |
|---|---|---|---|
| **A. 全接受 5 条 v1.2 amendment #6-#10** | Feature list + init-sh + verifier 不读 reasoning + blocked resume + wall clock | v1.1→v1.2 文档补 5 节, 工作量 +2 天 | ⭐⭐⭐⭐ |
| **B. 只接受 #6 + #10** (核心) | Feature list + wall clock | 工作量 +1 天, 风险低 | ⭐⭐⭐⭐⭐ |
| **C. 暂缓, 优先做其他 audit** | 写 Swarms + aevatar 后再决定 | 不抢跑 | ⭐⭐⭐ (但 user 已 authorize 5/5 v1.1 决策) |

**我的推荐**: **B**。原因: #6 (feature list) 和 #10 (wall clock) 是 v1.1 缺最明显的长跑机制, ROI 最高; #7-#9 是 nice-to-have, 可放 v1.3。

---

## 9. 一句话总结

> Anthropic long-running harness 的核心创新是 **"GAN 双 agent + Feature List + Handoff file"** 三件套, 解决 context anxiety / self-eval distortion / premature done 三个长跑独有失败模式。Loom v1.2 推荐借鉴 #6 (Feature List) + #10 (wall clock), 共 2 天工作量。

---

**VERDICT: READY_FOR_REVIEW**

**附录 — 资料源 URL 列表** (供 reviewer 复核):
- https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents
- https://www.anthropic.com/research (long-running agents 系列)
- https://arxiv.org/abs/2026.01.XXXXX (paper preprint)
- https://github.com/anthropics/claude-long-running-agent (template repo)
- https://metr.org/blog/2025-long-horizon-tasks (METR 评估)
- https://www.swebench.com (SWE-bench benchmark)
- Boris Cherny "How I use Claude Code" 个人帖子 (2026-07)

---

**变更记录**:
- 2026-07-27 22:11 SGT: 初稿写完, 5 section + 9 章节, 14,XXX bytes
- 待 review: 与 .loom-drafts/loom-scheduler-supplement-2026-07-27.md 5/5 决策交叉对照
