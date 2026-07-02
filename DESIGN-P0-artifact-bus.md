# P0 设计细化：结构化 Artifact 总线 + Context Budgeter + Codex executor

> 状态：**设计稿，未动代码**（用户决定"先别建，再细化方案"）。
> 落地时按严格 Agent Team Loop：独立 code-review 子 agent 先谈合同 → 建 → 真机验证 → 独立验收；子 agent 撞限额就停下等额度。
> 本稿全部锚定现有代码真实行号，避免空谈。

---

## 1. 现状（grounded，带行号）——为什么非改不可

**今天的跨阶段上下文传递**（`devkit/rdloop.py:408-414`）：
```python
upstream = "\n\n".join(
    f"### 上游产物：{a.title}（{a.role}）\n{compacted.get(a.key, txt[:3500])}"
    for a, txt in artifacts
) or "（无，这是第一步）"
mem = mem_ctx if st.key in ("brainstorm","plan") else ""
user = mem + f"## 开发任务\n{task}\n\n## 已有上游产物\n{upstream}\n\n请产出本阶段产物。"
```
compact 触发：`len(content) > COMPACT_THRESHOLD` 时把整段压成要点（`:480-488`）。
合同验收点注入 implement 的 `user`（`:440`）。迭代失败回灌见 `_build_feedback`（`while iterate ...:527`）。

**三个硬伤**：
1. **窗口无感**：`txt[:3500]` 是写死的字符截断。128k 模型上浪费了 ~96% 容量；32k 模型上若 5 个上游产物 ×3500 字 + 宪章 + 任务，仍可能爆窗。**同一个常数适配不了 32k–128k 全区间**——这正是用户要解决的核心。
2. **关键信息会被截掉**：截断取**前** 3500 字；而 acceptance criteria / 失败详情 / 报错栈常在产物**结尾**。compact_text 压散文时也可能把契约/`want=`/具体报错揉没。→ "丢信息导致下一轮解题失败"。
3. **无优先级**：所有上游产物一律全带、等权截断；既不按相关性取舍，也不保护"绝不能丢"的字段。

---

## 2. 目标设计

### 2.1 Artifact：从 `(stage, text)` 升级为带字段的产物
现在 `artifacts` 是 `[(st, content), ...]`（`:478`）。升级为一个轻量结构（**纯 dict，保持 stdlib、向后兼容**）：
```python
# devkit/artifact.py（新文件，~80 行）
def make(stage, role, title, body, fields=None):
    return {"key": stage, "role": role, "title": title,
            "body": body, "fields": fields or {}}
```
`fields` 装**受保护字段**：`contract`（验收点）、`failure`（上一轮 want=/报错）、`plan_decisions`（架构决定）、`patch_targets`（要改的文件清单）。
> 落地策略：**先不强制 worker 输出结构化字段**——由一个 `extract_fields(stage, body)` 从产物文本里抽（implement 抽 `# path` 文件名，review 抽 NO-GO 理由）。**首个受保护字段直接用已结构化的 `contract.json`**（`rdloop.py:437` 已写出 JSON，无需 scrape，零脆弱）。后续可演进成让 worker 直接产 JSON 字段。**第一版零改 prompt** 就能上。

#### 2.1.1 必须在 P0 就锁死的 schema 字段（评审 R2/§3：否则后面 Plane 全要回填迁移）
即使 P0 没有消费者读，也先在 artifact 上留这些槽：
- `stage / role / carrier`（现在在 tuple 里 → 提进 dict；Quota 与 Fitness 都按 carrier×stage 索引）。
- **`task_type`**（`backend-fix|test-gen|review|refactor|…`）——**最关键的缺失字段**。没它，Phase1 的 Model Fitness 按任务分桶 + Quota 按任务成本预测都得回填每条历史。v1 可用粗启发/用户打标，先让槽存在。
- `tokens / cost`（**每产物**，不只每 run-log 行）——否则 Quota Preflight 永远在 string-parse markdown 表。
- `verdict`（`GO|NO-GO|null`）+ `tests_passed`（`bool|null`）——VerifyReport/ReviewReport 形状。P0 锁死 → Codex(P0-b) 与 PonyTail(P1) 写同一字段，Task Center(P2) 读同一 schema。
- `window_used / budget_report`（budgeter 的 `{kept,compacted,dropped}` 做成结构化字段,不只 run-log 字符串)——Learning Sidecar(P3) 才能关联"丢了 X → 下阶段失败"而不必再解析日志。

### 2.2 受保护字段机制（治"丢信息"）
一个全局集合 + 一条铁律：
```python
PROTECTED = {"contract", "failure", "patch_targets"}  # 永不压缩、永不截断、放首尾
```
- Budgeter 组装 `user` 时，PROTECTED 字段**原文**放在最前（任务之后）和最后各复述一次关键锚点（呼应 *Lost in the Middle*：关键信息放首尾）。
- compact_text 只作用于 `body` 散文（P4），**绝不碰 fields 里的 PROTECTED 项**。
- 迭代回灌（`_build_feedback`）的 `failure` 走 PROTECTED 通道——把 #44 已有的"回灌 want=+critique"从临时拼接**提升为一等机制**。

### 2.3 Context Budgeter（窗口自适应，32k–128k 同一套码）
```python
# devkit/budget.py（新文件，~120 行，新增代码纯 stdlib）
def build_user(carrier, task, mem, artifacts, contract_block, *, window=None, mode="auto"):
    win = window or carrier_window(carrier)        # 见 2.4，缺省 32768 保守
    # ⚠ 评审硬错修正：char/token≈3.5 只对英文成立；中文 CJK≈1–1.7 字/token，
    #   用 3.5 会把预算高估 ~2-3 倍 → 撑爆窗口（正是 budgeter 要防的）。
    #   v1：调网关 /utils/token_counter 真数 token（网关已在跑，新代码仍 stdlib）；
    #   取不到时按内容 CJK 占比在 [1.5(中) … 3.5(英)] 间插值，宁可保守。
    budget_tokens = int(win * 0.6)                 # 0.6 留输出余量
    fit = lambda s: count_tokens(carrier, s) <= remaining  # 用真 token 数裁剪，不用字符
    # 优先级分配（高→低；超预算从低开始压/丢）：
    #   P0 PROTECTED: contract + 最近 failure + patch_targets        —— 永不动
    #   P1 task + 本阶段宪章 system（稳定前缀，已在 :451 拼好）       —— 永不动
    #   P2 直接上游（上一阶段）body：优先原文，超预算→compact 摘要    —— 可摘要不可丢字段
    #   P3 更早阶段：按相关性取 top-k（先关键词/标题匹配，stdlib）   —— 窗口小先砍
    #   P4 mem 教训散文                                              —— 最先 compact
    ...
    return user_str, report  # report: {kept:[...], compacted:[...], dropped:[...]}
```
- **小窗口（32k）**：P3 只留 top-1、P4 丢、P2 摘要化。
- **大窗口（128k）**：P2 给原文、P3 给 top-5、P4 原样。
- `mode=tight|rich` 给手动挡；`auto` 按窗口自动。
- `report` 写进 run-log（"本次输入：保留 X / 压缩 Y / 丢弃 Z"），**让人看得见带了什么**——易用性 & 可调试。

### 2.4 carrier → 窗口大小的来源
优先级：roles.toml 的 `context_window`（新增可选字段）> `litellm/config.full.yaml` 里该 model 的声明 > 内置保守表（claude/codex/glm≈128k、deepseek≈64k、minimax≈32k，可调）> 兜底 32768。
**保守兜底原则**：拿不准就当小窗口，宁可多压也不爆窗。

### 2.5 Codex executor（方案 B 唯一真新工程）
现有 executor 分发在 `executors.run(executor, prompt, carrier, sandbox, ...)`（`rdloop.py:461`，支持 chat/hermes/openclaw）。新增一支：
```python
# devkit/executors.py 里加 "codex" 分支
# 接口与现有一致：run("codex", system+user, carrier, sandbox, ...) -> (ok, content, meta)
# 行为：在 sandbox 内非交互调用 Codex CLI，让它跑真实测试/复现，回 VerifyReport（结构化）
#   VerifyReport.fields = {tests_passed: bool, failing: [...], repro: "...", verdict: "GO|NO-GO"}
```
- Codex"持整机操作流"→ 它当 **verify** 阶段执行器最合适：跑 pytest/ruff、真实复现、回报。
- 与现有 `apply.run_tests`（sandbox 内 pytest，`:508`）**并存不冲突**：run_tests 是确定性快测，Codex verify 是 agentic 深测（能读报错、试修、判 GO/NO-GO）。可二选一或叠加。
- 非交互鉴权沿用 openclaw 的思路（已有先例 #20）。**风险**：Codex CLI 的非交互/sandbox 行为需先做一个 spike 验证（像 #45 那样先验证再决定），不直接重度集成。

---

## 3. 集成点（落地时精确改哪里）
1. `rdloop.py:408-414` → 换成 `user, rpt = budget.build_user(carrier, task, mem, artifacts, contract_block, window=..., mode=...)`。**carrier 在 :446 才算出**，需把窗口解析提到组 user 之前（小重构）。
2. `artifacts.append((st, content))`（`:478`）→ 仍存 tuple 以**向后兼容** diff/score/stages 的现有解析；另存一份 `artifact.make(...)` 供 budgeter。或让 budgeter 接受 tuple + 旁路 `fields_map`。择一，合同里定。
3. compact 段（`:480-488`）→ 改为"只压 body，不碰 PROTECTED"。
4. `--context-mode auto|tight|rich` 加到 `__main__.py` argparse（仿 `--no-cache`/`--cascade`）。
5. roles.toml schema 加可选 `context_window`（`roles.py` 解析，缺省 None）。
6. Codex：`executors.py` 加分支 + `loom.roles.example.toml` 给 verify 阶段示例 `executor="codex"`。

## 4. 向后兼容 & 降级
- 无 `context_window`、无 fields → budgeter 退化成"接近当前行为"（按兜底 32k 截断），**绝不比现在差**。
- Codex 未配/spike 没过 → verify 阶段回退到现有 `run_tests`，不阻塞主流程。
- 现有 **67** 项测试（`unittest discover` 实测）必须继续全绿；新增 budgeter/artifact/codex 各自单测。

## 5. 风险 & 待定（合同协商时让独立评审拍）
- **char/token 比例**（评审定性为硬错,非小风险）：中文 CJK≈1–1.7 字/token,3.5 会高估预算 2-3 倍撑爆窗口。**决议**:v1 直接调网关 `/utils/token_counter` 真数 token;不可用时按 CJK 占比在 [1.5…3.5] 插值,取保守下限。**不发 3.5 常量。**
- **相关性排序 P3**：第一版关键词/标题匹配够不够？还是直接"只带上一阶段 + 合同"最稳？倾向后者（最小可用），P3 检索留插槽。
- **Artifact 字段抽取** vs **让 worker 直接产 JSON**：第一版用抽取（零改 prompt），合同里写清。
- **Codex executor** 必须先 spike（非交互能否在 sandbox 跑测试并回结构化结果），spike 不过则本轮只做 A（budgeter+artifact），B 的 Codex 顺延。

## 6. 建议的落地切分（每块独立走 Loop）
- **P0-a**：`artifact.py` + 受保护字段 + Budgeter（窗口自适应）→ 改 `rdloop` 组 user。**先做这个**，直接解决 token/丢信息，且不依赖 Codex。
- **P0-b**：Codex executor（先 spike 再集成）。
- P1：implement best-of-N/cascade 接 PonyTail 审查门（复用 #44）。

> 待用户 greenlight 某一块 → 我起草该块 Sprint Contract → 独立 code-review 子 agent 协商 → 再建。
