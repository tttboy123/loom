# 2026-07-01 Parallel Agent Team Iteration

本文件记录 2026-07-01 启动的 3 个并行 Agent Team 对 Loom 下一轮产品和开发需求的自治发现结果。

## Runs

| Team | Run | Focus | Gate |
| --- | --- | --- | --- |
| Product Discovery | `devkit/runs/parallel-discover-product-20260701` | 产品方向、需求发现、自动迭代闭环 | `REQUEST-CHANGES` |
| Development Discovery | `devkit/runs/parallel-discover-dev-20260701` | 下一轮可进入 backlog 的开发任务 | `REQUEST-CHANGES` |
| Project Autoloop Design | `devkit/runs/parallel-discover-project-20260701` | 项目级多轮自动迭代编排 | `REQUEST-CHANGES` |

三个 team 的方向判断一致：Loom 下一阶段不应继续优先清理零散 utility，而应补齐“发现需求 -> 评分 -> 入队 -> 执行 -> 验证 -> 审查 -> 失败回流”的项目级自治闭环。

`REQUEST-CHANGES` 不是方向失败，而是审查要求把上游产物整理成 backlog-ready 结构，避免“摘要看似完整、实际不可执行”的假完成。

后续架构落点见 `docs/loom-stable-agent-runtime-blueprint.md`。该蓝图把这里的并行
发现、验证来源、lease/recovery 和失败回流要求收敛成 Loom-only 的稳定 Agent
Runtime 目标，并把 `Agentic MapReduce` 定义为 `cluster` 模式下的一个有界并行策略。

## Consolidated Product Requirements

### R1. Formal Discover Member

- **Benefit**: 让 Loom 从“执行给定任务”进化为“能发现下一轮项目发展需求”。
- **Scope**: 为 `discover` 定义正式输入/输出契约，覆盖 vision、roadmap、run history、external requests、failure signals。
- **Acceptance**:
  - 输出 3-10 条结构化候选需求。
  - 每条候选包含 `title`、`problem`、`benefit`、`scope`、`non_goals`、`dependencies`、`risks`、`acceptance_criteria`、`source_type`、`evidence_refs`。
  - 缺少输入源时显式输出 `missing_inputs` 或 `degraded=true`，不得静默成功。
- **Risk**: discover 产出抽象口号，污染 backlog。
- **Order**: 1.

### R2. Candidate Valuation And Backlog Governance

- **Benefit**: 把 discover 输出变成可解释的 `accept / defer / reject / split` 决策，避免需求池膨胀和跑偏。
- **Scope**: 在 `devkit/valuer.py` 外层补充证据组装、评分理由、决策记录和去重规则。
- **Acceptance**:
  - 每个候选都有用户价值、自治增益、实施成本、验证难度、风险和依赖评分。
  - `split`、`defer`、`reject` 的触发条件明确。
  - 入队项可追溯到来源和评分理由。
- **Risk**: 伪精确打分掩盖真实产品判断。
- **Order**: 2.

### R3. Project-Level Autoloop

- **Benefit**: 让一个项目目录可以多轮自动迭代，而不是只执行一次 feature。
- **Scope**: 扩展现有 `devkit auto` 项目模式，复用 `backlog.json`、`progress.md`、`devkit feature`、`runs/`。
- **Acceptance**:
  - 支持 `dry-run` 只输出本轮计划。
  - 能在 backlog 不足时触发 discover/refill。
  - 每轮写入状态、证据、停止原因。
  - MVP 明确串行执行；并行只用于只读发现/评分/审查，避免同目录写冲突。
- **Risk**: 声称并行执行，但实际会污染同一个项目目录。
- **Order**: 3.

### R4. Stable Verification Surface

- **Benefit**: 外部项目使用 Loom 时，能区分 sandbox 结果、物化后 repo 真相和未知来源。
- **Scope**: 为验证结果增加 `source`、`evidence_paths`、`verification_confidence`、`mismatch_detected`。
- **Acceptance**:
  - `source` 只允许 `inner_sandbox`、`materialized_repo`、`unknown`。
  - 仅 sandbox 通过时不得标记为 repo truth。
  - 无法确认来源时必须是 `unknown`。
- **Risk**: shadow-build 通过被误报成真实仓库通过。
- **Order**: 4.

### R5. Failure Feedback Into Discover And Valuer

- **Benefit**: 失败不只停在日志里，而是影响下一轮发现和排序，减少坏候选反复入队。
- **Scope**: 将已有 failure classification 扩展为 structured failure packet，并回流到 discover/valuer。
- **Acceptance**:
  - 区分 bad candidate、test failure、verification mismatch、infra/provider failure、review failure。
  - `failure_kind` 与 `next_action_hint` 一致。
  - 单次模型异常不得系统性降权一个产品方向。
- **Risk**: 把执行失败误判为产品方向失败。
- **Order**: 5.

## Backlog-Ready Development Tasks

### P0-1. Discover Contract And Structured Candidate Output

- **Type**: `feature / contract`
- **Dependencies**: none
- **Minimal scope**: 定义 discover candidate schema，并让最小输入源能输出可机读候选。
- **Acceptance tests**:
  - minimal inputs return valid structured candidates.
  - each candidate includes title, value, deps, minimal_scope, acceptance_tests, source.
  - missing sources are reported, not hidden.
  - valuer can consume output without product-specific patching.
- **Implementation model**: MiniMax-M3.
- **Review model**: GPT-5.4 required.

### P0-2. Discover To Valuer To Backlog Refill Wiring

- **Type**: `feature / orchestration`
- **Dependencies**: P0-1
- **Minimal scope**: 将 discover 输出接到 valuer 和 `_refill_backlog()`，只把 `accept` 候选写入 `pending` backlog。
- **Acceptance tests**:
  - accepted candidates become backlog items.
  - split candidates do not enter backlog before being split.
  - defer/reject decisions are logged with reasons.
  - duplicates are not inserted twice in one refill.
- **Implementation model**: MiniMax-M3.
- **Review model**: GPT-5.4 required.

### P0-3. External Verification Surface Contract

- **Type**: `feature / contract`
- **Dependencies**: materialization/apply contract from `loom-upstream-002`
- **Minimal scope**: 给验证输出补 source-of-truth classification。
- **Acceptance tests**:
  - sandbox result is `inner_sandbox`.
  - materialized target repo result is `materialized_repo`.
  - unknown source remains `unknown`.
  - result includes at least one relevant evidence path, command, or reason.
- **Implementation model**: MiniMax-M3.
- **Review model**: GPT-5.4 required.

### P0-4. Queue Lease, Heartbeat, And Stale Reclaim

- **Type**: `reliability`
- **Dependencies**: P0-3 recommended
- **Minimal scope**: 为 external queue running state 增加 lease、heartbeat、stale 判定和 reclaim 审计。
- **Acceptance tests**:
  - acquiring a task creates lease metadata.
  - active lease cannot be acquired twice.
  - heartbeat refresh prevents stale reclaim.
  - stale task can be reclaimed with trace.
  - reclaim limit prevents infinite cycling.
- **Implementation model**: MiniMax-M3.
- **Review model**: GPT-5.4 required.

### P1-1. Candidate Drift Gate And Failure Packet

- **Type**: `quality gate`
- **Dependencies**: P0-3
- **Minimal scope**: 在验证/外部执行链路输出 structured failure packet，区分 candidate drift 与真实产品失败。
- **Acceptance tests**:
  - missing expected outputs classify as candidate drift.
  - unrelated file changes do not count as task success.
  - drift, repo test failure, infra failure use distinct `failure_kind`.
  - packet contains only task-relevant trace.
- **Implementation model**: MiniMax-M3.
- **Review model**: GPT-5.4 required.

### P1-2. Actual Provider Fallback Observability

- **Type**: `observability`
- **Dependencies**: none
- **Minimal scope**: 在 run log / CLI status 中区分 requested carrier/model 与 actual carrier/model。
- **Acceptance tests**:
  - primary model hit records requested and actual.
  - fallback records original target and actual provider.
  - provider down is visible, not silently swallowed.
  - unknown cost is not fabricated.
- **Implementation model**: MiniMax-M3.
- **Review model**: GPT-5.4 recommended.

### P1-3. Retry Policy Wiring Into Auto Loop

- **Type**: `reliability`
- **Dependencies**: P0-4
- **Minimal scope**: 将已有 retry policy 接入 backlog auto loop 和 external queue recovery。
- **Acceptance tests**:
  - failed tasks below limit can re-enter ready state.
  - non-failed tasks are not retried.
  - attempts increment and preserve failure trace.
  - max attempts moves task to final failed/blocked state.
  - stale reclaim and retry cannot form an infinite loop.
- **Implementation model**: MiniMax-M3.
- **Review model**: GPT-5.4 recommended.

## Project Autoloop MVP Decision

为避免假并行，本轮建议明确：

- 项目级 MVP 默认串行执行 feature。
- `--parallel K` 先只用于只读阶段：discover、score、plan、review。
- 对同一项目目录的实际物化和测试保持串行。
- 真正并行写盘需要独立 worktree / temp project copy / merge gate，作为后续版本。

建议入口：

```bash
python3 -m devkit auto --project <dir> --dry-run
python3 -m devkit auto --project <dir> --yes --rounds 1
python3 -m devkit auto --project <dir> --yes --loop --ready-floor 3
```

建议新增项目状态文件：

- `autoloop.json`: project loop snapshot, stop reason, round, budget, failure streak.
- `task_state.json`: per-feature status, attempts, cooldown, evidence runs.
- `project_runs.jsonl`: per-round concise evidence records.

## Next Execution Recommendation

下一轮应先执行 Batch A：

1. P0-1 Discover Contract And Structured Candidate Output.
2. P0-2 Discover To Valuer To Backlog Refill Wiring.
3. P0-3 External Verification Surface Contract.

这三项直接回答“需求从哪里来、怎么排序、怎么证明执行结果可信”。P0-4 lease/recovery 可以紧随其后，但不应早于 verification source-of-truth。

## Execution Update

已将 Batch A 写入 `devkit/backlog.json`：

| Task | Status | Notes |
| --- | --- | --- |
| `discover-contract` | `failed` | 自动执行已跑完，测试通过但 GPT-5.4 审查 `REQUEST-CHANGES`。 |
| `discover-valuer-refill-wire` | `pending` | 依赖 `discover-contract`，当前不会被选中。 |
| `external-verification-surface` | `failed` | 自动执行已跑完，构建测试失败并触发 `AGENT_STOP`。 |

### `discover-contract` NO-GO Summary

Run: `devkit/runs/auto-20260701-222134`

结果：

- `implement`: MiniMax-M3 OK.
- `verify`: MiniMax-M3 OK.
- `review`: GPT-5.4 `REQUEST-CHANGES`.
- Build tests: `7 passed`.
- Iteration: 2/2 rounds, still `REQUEST-CHANGES`.
- Final gate: `NO-GO`.

主要审查阻塞：

1. 没有证明 discover 输出可被 `valuer` 直接消费。
2. focused unittest 没覆盖 discover -> valuer 的最小端到端消费。
3. `missing_inputs` 的允许值集合未冻结。
4. candidate 字段类型和最小语义未稳定。
5. 未知输入边界测试不足，包括 unknown source kind、`payload=None`、非字符串 text、空 sources。

下一次修复 `discover-contract` 应先补：

- `DiscoverInput / Source / Candidate / DiscoverOutput` 的真实契约代码。
- happy path、缺输入、malformed 输入、未知输入、valuer 消费测试。
- `Candidate.to_dict()` 稳定 schema 和字段类型断言。
- 一条最小 discover -> valuer 消费 trace，证明不需要产品专用 patch。

### `external-verification-surface` NO-GO Summary

Run: `devkit/runs/auto-20260701-222542`

结果：

- `implement`: MiniMax-M3 OK.
- `verify`: MiniMax-M3 OK.
- `review`: GPT-5.4 OK at stage level, but downstream build/test failed.
- Build tests: `1 failed, 10 passed`.
- Iteration: 2/2 rounds, same error repeated, `AGENT_STOP`.
- Final gate: `NO-GO`.

失败点：

```text
tests/test_verify_result_contract.py expected tests/test_features.py to exist,
but the sandbox build only contained the generated build files.
```

这个失败本身是有价值的信号：生成的验证面测试混淆了 sandbox build 与真实 repo root，正好说明该任务必须先稳定 `source` 与 evidence root 语义，不能在 build sandbox 中假设真实 repo 文件存在。

下一次修复 `external-verification-surface` 应先补：

- 明确 sandbox build root 与 materialized repo root 的区别。
- 测试不得通过查找真实仓库的 `tests/test_features.py` 来证明“现有测试不回退”。
- contract test 应只验证当前 build 内的 verify result schema；真实 repo 回归应由外层 harness 执行并记录为 `materialized_repo` 或 `unknown`。
- 对 `unknown` source 增加保护测试，防止无证据时伪装为 repo truth。
