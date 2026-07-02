# Loom 战略评估 + 两个技术方案（项目 Agent 的评审回复）

> 日期 2026-06-27 · 作者：负责当前项目的 Agent（即本仓库的开发者 Agent）
> 输入：① Control Plane 战略报告 ② OpenCode/OpenWork 增量评审 ③ OpenHands 竞品对标
> 用户要求：研究社区怎么做 → Agent Team 给"怎么做"的建议（开发者体验/演进/技术成长性/易用性/后续优化）→ **用户做最终决策**。
> 本文不替你拍板，只给"做/不做/排序"的工程判断和两份可落地设计。

---

## Part 0 — 先承认一件事：你今天碰到的 session limit 就是报告的核心论点

acceptance 子 agent 这次 0 token 直接撞限额。这正是战略报告反复说的痛点：**多 Agent + 大上下文 = token 烧得快、还容易在交接处丢信息**。
所以在所有宏大蓝图里，**真正紧迫、ROI 最高的只有两件**：
1. 上下文/记忆优化（方案 A）——让每次调用塞进窗口的都是"必要且不丢关键信息"的内容。
2. 模型即子 Agent 的开发流水线（方案 B）——把贵模型只用在刀刃上，开发交给性价比模型，验证交给 Codex。
这两件直接对冲"token 不够用"。控制面（Control Plane）是北极星，但**不该现在就铺开**。下面分别说。

---

## Part 1 — 对战略报告的评估（我作为项目 Agent 的判断）

### 1.1 方向判断：对，但要克制
报告把 Loom 从"多模型研发流水线"升级为"本地 Agent/AI Control Plane"，**定位是对的**——
LiteLLM 管请求、CC Switch 管配置、OpenWork 管桌面体验，没人管"任务能不能在我的额度内被可验证地完成"。Loom 占这个位是成立的。

但报告有 12 个大模块（Hub/Registry/Quota Wallet/Capacity Planner/Model Fitness/Asset/Radar/Migration/Cluster/Safety/Task Center/Learning）。**全做 = 必然烂尾**，而且违反你自己写进宪章的 anti-overengineering（也正是 PonyTail 的精神）。
> 评审红线：任何一轮只动一个模块，每个模块先出"最小可验证版"，按 确定性×价值÷工作量 排序。这跟我们一直跑的 Agent Team Loop 是同一套纪律。

### 1.2 哪些"已经有雏形"、哪些"是新工程"
| 报告模块 | Loom 现状 | 判断 |
|---|---|---|
| Agent/Pipeline Registry | `loom.roles.toml` 已是雏形（stage=role=carrier=executor） | **升级即可**，低成本，P0 |
| Model Fitness / Score | `devkit scores` + `stages`（#46）+ ratings.jsonl 已落 | **再补"按任务类型"分桶**即可 |
| Quota Wallet / Capacity | `devkit quota` 读 LiteLLM spend 已落一半 | 余额 adapter 是新工程，中等 |
| Asset / Radar / Migration / Cluster | 几乎全新 | **暂缓**，是"生态"不是"内核"，过早做会拖垮核心 |
| Safety Presets | sandbox + `--apply`/`--apply-git` human gate 已落 | **声明式权限分级**是增量，可后置 |
| Task Command Center | console 已有 run/diff/apply 面板 | 增量演进，不是重写 |

**结论**：报告里 70% 的"差异化"其实是 Loom 已有能力的**深化**，不需要推倒重来。真正的新内核只有"额度容量预估"和"结构化 Artifact 交接"，后者正是方案 B 的地基。

### 1.3 OpenHands 竞品对标（你点名要的差异化）
OpenHands（前 OpenDevin）是 Loom 最像的直接对手：开源、本地/云、sandbox runtime、CodeActAgent、agent 自己跑命令/测试、有 UI、长任务、支持多 LLM。**它强在"单 Agent 把活干完"的执行深度**。

Loom 不该比"谁的单 Agent 更能干"——那是 OpenHands/OpenCode 的主场。Loom 的差异化打三点：
1. **多厂商额度感知调度**：OpenHands 一个任务通常一个模型跑到底；Loom 把"贵模型做设计、便宜模型做开发、Codex 做验证"做成一等公民，且**任务开始前能预估额度够不够**。这是 OpenHands 没有的。
2. **Artifact 互信、Agent 互不信**：OpenHands 是 agent 内部记忆驱动；Loom 用结构化产物（Plan/Patch/VerifyReport）做跨厂商交接 + 独立审查门（reviewer 与 dev 不同厂），抗"自评自洽"的盲区。
3. **harness 中立**：OpenHands 想做你的 runtime；Loom 把 OpenHands/OpenCode/Codex/Claude Code 都当 **executor** 接进来，不抢 runtime，抢"调度+评分+验证+额度"这层。
> 一句话差异：OpenHands 让一个 Agent 把活干完；Loom 让**一队不同厂商的模型在你的额度预算内、可验证地协作把活干完**。

### 1.4 该做 / 不该做（给你决策用）
**该做（近期）**：方案 A（上下文/记忆）、方案 B（模型子 Agent 流水线）、Registry 升级、Score 按任务分桶。
**该做（中期）**：Quota 余额 adapter + Capacity Preflight、Safety 声明式分级、Codex executor。
**先别做**：Radar、Migration 全家桶、Cluster Profiles、桌面小白模式、Asset marketplace。理由：它们是"生态/增长"功能，在内核（A+B）稳之前做，只会摊薄注意力、撞 OpenWork/OpenHands 的主场。等 A+B 让 Loom 真的"省 token 且结果可信"，再谈生态拉新。

---

## Part 2 — 方案 A：上下文传递 + 记忆管理（窗口 32k–128k 自适应）

### 2.1 社区怎么做（调研 + 论文）
**工程实践**
- **Anthropic context editing + memory tool**（Claude Agent SDK）：自动清除陈旧 tool result，把长期信息写到外部文件/内存，按需读回。→ Loom 的 MEMORY.md + compact_text 是同思路，但缺"自动按窗口比例触发"。
- **子 Agent 上下文隔离**（Claude Code subagents / OpenHands delegation）：子 Agent 有独立窗口，父只收**最终结果**，不回灌中间 transcript。**这是最大的省 token 杠杆**，也是今天 acceptance 子 agent 本该省钱的机制。
- **MemGPT / Letta**：分层记忆（主上下文=内存，外部存储=磁盘），LLM 自己分页换入换出。
- **Cline/Roo 的 condense**：窗口快满时把历史压成结构化摘要再继续。
- **结构化交接**：不传聊天记录，传 Plan/Patch/Diff/TestLog 这类 typed artifact（CrewAI/AutoGen/LangGraph 的 handoff 都走这条）。
- **LLMLingua-2**：prompt 压缩；**RAG/Self-RAG**：只检索相关历史而非全量塞入。

**论文（按对本设计的影响排序）**
- *Lost in the Middle*（Liu 2023）：长上下文中部信息被忽略 → **关键字段放首尾**，别埋在中间。
- *MemGPT*（Packer 2023）：OS 式分层记忆与分页。
- *StreamingLLM / attention sinks*（Xiao 2023）、*H2O*（Zhang 2023）：哪些 token 可丢而不崩。
- *RAG*（Lewis 2020）、*Self-RAG*（Asai 2023）：检索式记忆。
- *LLMLingua-2*（Pan 2024）：压缩比/保真权衡。

### 2.2 核心设计：一个"窗口自适应预算器" + "受保护字段"
痛点不是"压缩"，是**压缩时把关键信息（acceptance criteria、失败详情、契约）一起压没了，导致下一轮解题失败**。所以设计两件事：

**(1) Context Budgeter（按当前载体窗口分配预算）**
读取当前 carrier 的 `context_window`（32k…128k，从 LiteLLM model 配置或 roles.toml 声明），把输入切成带优先级的块，按窗口比例分配：

```
预算 = 窗口 × 安全系数(0.6)  # 给输出留 40%
分配优先级（高→低，超预算从低开始压/丢）：
  P0 受保护：acceptance criteria / 契约 / 上一轮失败详情(want=/报错)   ← 永不压缩，永远放首尾
  P1 任务本体：task + 当前阶段 system(宪章稳定前缀)
  P2 直接上游产物：上一阶段 Plan/Patch（结构化，可摘要不可丢字段）
  P3 检索记忆：从 runs/ + MEMORY.md 取 top-k 相关（不是全量）
  P4 远端历史：更早阶段的散文 → 最先被 compact_text 压成要点
```
窗口小（32k）→ P3 只取 top-1、P4 直接丢、P2 摘要化；窗口大（128k）→ P2 给原文、P3 给 top-5。**同一套代码适配全区间**，靠的是"按比例"而非"写死行数"。

**(2) 受保护字段（解决"丢信息导致解题失败"）**
compaction 只压 P4 散文，**P0 一字不动**。这正是我们 #44 cascade 已经在做的雏形（把 golden `want=` + review 批评回灌给下一轮 implement）——把它**提升为通用机制**：任何阶段交接，failure detail 与 contract 走"受保护通道"，不进压缩器。

### 2.3 落到 Loom（最小改造，复用现有件）
- 已有 `compact_text`（rdloop.py）→ 包一层 `budget_context(carrier, blocks)`，按 2.2 分配；compact 只作用于 P4。
- roles.toml 每个 stage 增 `context_window`（可选，缺省按 carrier 在 config 里的值/保守 32k）。
- 阶段交接产物从"拼字符串"升级为 typed dict：`{contract, plan, patch, failure}`，budgeter 认字段优先级。
- 子 Agent 调用强制"只回结果不回 transcript"（今天就该这么省）。
- 可观测：run-log 记录"本次输入：保留 X 块/压缩 Y 块/丢弃 Z 块"，让人能看见到底带了什么。

### 2.4 审查视角（易用性 + 后续优化）
- **易用性**：用户只需在 roles.toml 写 `context_window`（甚至不写，自动探测），其余自动。一个 `--context-mode auto|tight|rich` 兜底手动挡。
- **后续优化点**：P3 检索器先用关键词/TF-IDF（纯 stdlib），留 `retriever` 插槽，将来可换 embedding/语义；压缩器留 `compactor` 插槽，将来可接 LLMLingua-2。**先 stdlib 可用，再谈先进**——符合宪章。

---

## Part 3 — 方案 B：模型即子 Agent 的开发流水线

### 3.1 目标固定映射（你给的分工）
```
Claude Code  → 架构/方案制定(plan) + 最终审查(review)   ← 只在刀刃，省订阅额度
DeepSeek/GLM/MiniMax → 开发(implement)                  ← 性价比，可并行best-of-N 或 级联cascade
Codex        → 测试 + 验证(verify)                       ← 它持有整机操作流，跑真测试最合适
```
关键难点是你点出的：**任务正确传递 + 上下文正确传递**。答案就是 Part 2 的"结构化 Artifact + 受保护字段"。**Agent 互不信任，Artifact 互相信任**——交接不靠聊天记忆，靠 typed 产物。

### 3.2 社区怎么做（调研）
- **Aider 的 architect/editor 双模型**：强模型出方案、便宜模型落编辑。**与你的"Claude 设计 + 便宜模型开发"几乎一一对应**，是最直接的先例，证明这条路可行且省钱。
- **CrewAI / AutoGen / LangGraph**：role agent + 结构化 handoff + 状态机；LangGraph 的"状态对象在节点间流动"= 我们的 Artifact 总线。
- **Claude Code subagents**：上下文隔离 + 只回结果（省 token）。
- **OpenHands delegation / CodeActAgent**：主 agent 委派子任务。
- **RouteLLM / FrugalGPT**：按难度路由/级联——**Loom 的 `--cascade` 已落**（#44），便宜模型先上，过不了再升级。
- **PonyTail**：作为 **review gate**（不是普通 prompt），专治便宜模型"为完成任务而过度膨胀 diff/乱加依赖"。放在 worker patch → reviewer → **PonyTail 门** → Codex verify → apply。不过则打回 worker 要求更小 patch。

### 3.3 落到 Loom（复用 roles + executor + cascade）
Loom 已有的三件正好拼成这条流水线：
1. **roles.toml**：stage→carrier→executor 已可自由绑定。直接写：
```toml
[[stages]]
key="plan"      carrier="claude-sub"  executor="chat"
[[stages]]
key="implement" carrier="deepseek"    executor="chat"   # 或 cascade=[deepseek,glm,minimax]
[[stages]]
key="verify"    executor="codex"      # ← 需新增 codex executor
[[stages]]
key="review"    carrier="claude-sub"  executor="chat"   # 与 dev 不同厂，独立审查
```
2. **executor 层**（chat/hermes/openclaw）→ **新增 `codex` executor**：把 Codex CLI 当 agentic 验证器（在 sandbox 里跑测试/复现/回报 VerifyReport）。这是方案 B 唯一的"真新工程"，其余是配置。
3. **`--cascade`（#44）+ 并行 ask（已落）**：implement 阶段可"3 个便宜模型并行出 patch → 取测试通过且 PonyTail 最瘦的"（best-of-N），或"级联"（便宜的先，过不了升级）。
4. **Artifact 契约**：plan 产 `Plan`，implement 产 `Patch`，codex 产 `VerifyReport`，review 产 `ReviewReport`——都带受保护的 contract/failure 字段（Part 2）。

### 3.4 三个角度（你要的评审维度）
- **开发者体验**：用户只改 roles.toml 就能换"谁做哪段"；一条 `loom run "任务" --recipe cheap-dev` 直接套用"Claude设计→便宜开发→Codex验证→Claude审查"配方。失败时 run-log 显示卡在哪个 Artifact、哪个字段没传到。
- **后续演进**：executor 接口稳定后，OpenCode/OpenHands/Aider 都能作为 implement/verify executor 接入（harness 中立的兑现）。Score 按"哪个便宜模型在 backend-fix 上通过率最高"反哺路由。
- **技术成长性**：结构化 Artifact 总线是日后 Quota Capacity（按 Artifact 预估各阶段成本）、Learning Sidecar（学你常接受谁的 patch）、Workflow Score 的共同地基。**先把这条总线铺对，后面所有模块都站在它上面**——这是最高杠杆的一块基建。

### 3.5 PonyTail 接入点（你点名的优化）
作为 Loom Asset 的第一个试点：`loom.roles.toml` 给 review 阶段挂一个 `gate="ponytail"`，审查不过（diff 太大/新增依赖/多余抽象）→ NO-GO 回 implement，要求"用更小 patch 满足 acceptance criteria"。这把"便宜模型膨胀 diff"的长期维护成本摁住。先硬编码一个 PonyTail 风格的 review system prompt 即可验证价值，不必先做 Asset Importer 全家桶。

---

## Part 4 — 推荐排序（按 确定性×价值÷工作量），你来拍板

| 优先 | 项 | 为什么 | 工作量 |
|---|---|---|---|
| **P0** | 方案 B 的 **Artifact 契约总线** + Codex executor | 直接解 token 短缺 + 是后续一切的地基；分工你已定 | 中 |
| **P0** | 方案 A 的 **Context Budgeter + 受保护字段** | 解"丢信息导致解题失败"，32k–128k 自适应 | 中 |
| P1 | implement 的 **best-of-N / cascade** 接 PonyTail 审查门 | 复用 #44 + roles，价值高 | 低 |
| P1 | Registry 升级（roles→agents/pipelines）+ Score 按任务分桶 | 已有雏形，深化即可 | 低-中 |
| P2 | Quota 余额 adapter + Capacity Preflight | 真新工程，但价值高 | 中-高 |
| 暂缓 | Radar / Migration 全家桶 / Cluster / 桌面小白模式 / Asset 市场 | 生态功能，内核稳前别碰 | 高 |

**我的单条建议**：先做 **P0 两件**（A 和 B 共用"结构化 Artifact + 受保护字段"这块地基，应一起做），用 Agent Team Loop 一次一件、独立审查、真机验证。其余等这两件让 Loom 真的"省 token 且结果可信"后再谈。
