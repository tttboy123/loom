# 调研报告：社区「多模型 + token 优化」项目，及接入 Loom 的可行性

> 双 agent 协同产出：**研究 agent**（联网调研社区项目/技术）+ **评审 agent**（对照 Loom 真实代码评估接入可行性）。评审 agent 在依据代码时**修正了研究 agent 的排序**。
> 日期：2026-06-25。

---

## 0. 背景：Loom 现有能力（避免重复造轮子）
LiteLLM 多厂商网关 + `loom-*` 角色载体路由 + 跨厂商降级 + 每运行 token/$ 与 `--budget` + 便宜模型摘要式 `compact_text` 压缩 + 并行多模型 ask + **golden 质量门**（`evals.run_golden`：expr/expect、`raises`、`cmd`、真机 `web`）。
关键架构事实（评审 agent 从代码读出，决定一切结论）：
1. **单一咽喉**：所有 LLM 调用都过 `rdloop.gateway_chat()` —— 在这里加东西自动全局生效。
2. **按角色静态路由**（改 YAML，不在调用时选模型），没有"按难度动态选模"的代码路径。
3. **Claude/Codex 经 CLIProxy 的 OpenAI 兼容垫片（订阅）**，且网关 `drop_params: true` —— 任何厂商原生缓存参数（`cache_control`）可能被吞；订阅后端 cost=$0，省的是**额度/延迟**不是钱。
4. **质量门已存在且可机器判定** —— 正好是 cascade 需要的"便宜验证器"。

---

## 1. 社区项目地形图（研究 agent，附来源）
| 项目 | 类别 | 关键 token/成本技术 | 来源 |
|---|---|---|---|
| LiteLLM | 多厂商网关（Loom 底座） | prompt-cache 感知路由（同前缀固定到持有缓存的 deployment） | docs.litellm.ai |
| OpenRouter | 网关/路由 | `:floor`/`:nitro` 一 token 路由修饰；auto-router 按 query 选模 | openrouter.ai |
| Portkey / Helicone / Cloudflare AI Gateway | 网关/可观测 | 语义缓存、成本可见性、边缘响应缓存 | 各官博 |
| **RouteLLM**（LMSYS, 5.1k★） | 学习型路由 | 一个 0-1 阈值拨"多少流量给强模"；自称 MT-Bench 上 ~85% 降本@~95% GPT-4 质量 | github.com/lm-sys/RouteLLM |
| **FrugalGPT**（Stanford 论文） | LLM 级联 | 便宜模型先答 + 学习型打分器判好坏，失败才升级；自称 ~98% 降本 | arxiv 2305.05176 |
| Not Diamond / Martian | 托管学习型路由 | 按 query 选"最佳 LLM" + 成本上限 | 各官 doc |
| **LLMLingua / -2**（微软, 6.4k★, MIT） | 提示压缩 | 蒸馏 BERT 丢低信息 token，最高 ~20x；CPU 可跑 | github.com/microsoft/LLMLingua |
| **GPTCache**（Zilliz, 8k★） / vCache | 语义+精确缓存 | 嵌入向量近似命中返缓存；vCache 加每条置信界控错误率 | github.com/zilliztech/gptcache；arxiv 2502.03771 |
| 厂商 prompt caching（Anthropic/OpenAI/DeepSeek） | KV/上下文缓存 | 缓存静态前缀：Anthropic 读便宜 ~90%，OpenAI/DeepSeek 自动 ~50% | platform.claude.com |
| TOON / 约束解码 | 省 token 的输出编码 | 紧凑表格编码比 JSON 省 ~40% token | arxiv 2603.03306 |

> 诚实警告（研究 agent）：RouteLLM/FrugalGPT 的"85–98% 降本"是它们自家聊天 benchmark 的数，**不会 1:1 迁移到 agentic 编码流量**，当方向别当承诺。语义缓存有 **1–15% 错误命中率**，对开发/代码工具不可接受——必须设门。

---

## 2. 接入 Loom 的可行性评审（评审 agent，对照代码）
| # | 技术 | 合 Loom 架构? | 工作量 | 价值 | 风险 | Loom 里的挂载点 |
|---|---|:--:|:--:|:--:|---|---|
| 1 | **级联+便宜验证器**（FrugalGPT） | 部分（缺升级路径，验证器已有） | M | Med（implement 处 Hi） | 中 | `run_loop` 迭代循环；复用 `_materialize_test`/`run_golden` |
| 2 | 启发式难度路由（RouteLLM） | 弱（与角色静态路由 + 审查跨厂商独立性冲突） | M | Lo–Med | 中 | 新 `route_carrier()`，**只限非对抗阶段，绝不动 review** |
| 3 | **prompt-cache 感知前缀结构** | 是，但受 CLIProxy/drop_params 约束 | S–M | Med（订阅处=额度/延迟） | 中 | `gateway_chat` payload；先做验证 spike |
| 4 | LLMLingua-2 压缩 | **否**（torch+BERT 重依赖；会破代码语法） | L | Med | 高 | 会取代 `compact_text` → **拒绝** |
| 5 | **精确响应缓存**（+ 受控语义） | 精确:完美；语义:否 | 精确 **S** | Med–Hi | 精确 Lo | 包住 `gateway_chat`，sha256(model+system+user) → sqlite |
| 6 | 上下文外置+记忆分层 | 部分（Loom 已有便宜版） | M | Med | 中 | 扩 `compacted{}` + upstream 构建 |
| 7 | 保前缀缓存的压缩（TokenPilot） | 否（依赖 #3 先成立） | L | Lo（当前） | 高 | **暂拒** |
| 8 | TOON/紧凑输出编码 | 窄（仅 contract 出结构化） | S–M | Lo | 中 | `contract._FMT`，别碰散文阶段 |
| 9 | 按模板/角色聚合成本（Helicone） | **已基本有** | S | Med（诊断） | Lo | `usage()` 加按 `stage:` tag 透视 |
| 10 | 调用方意图标签（:floor/:nitro） | 是（tags 管道已有） | S | Lo–Med | Lo | `gateway_chat(tags=)` + `route_carrier` 读 tag |

**与 Loom 已有的对账**：#4 与 `compact_text` **冗余且更差**（拒）；#9 与 `usage()/score_report` **已有，只差按阶段透视**（小升级）；#2 与角色载体**部分冲突**（审查独立性不能破）；#1 其实是 `--iterate` 的**升级**（循环机器已在，只差"失败升级到更强模"）；#5 与 #3 **是全新的**（Loom 没有任何缓存层）。

---

## 3. 推荐接入顺序（评审 agent，按 确定性×价值÷工作量；**与研究 agent 排序不同并说明**）
研究 agent 把 #3 放第一；**评审 agent 依据代码改为 #5-精确第一**——因为 #3 的收益**被外部黑盒（CLIProxy 是否转发 `cache_control`、`drop_params` 是否吞掉）挟持**，而 #5/#1 由 Loom 自己的代码保证。

1. **精确响应缓存（#5-精确）· S · 先做。** 在 `gateway_chat` 顶部查 `sha256(model+system+user+max_tokens)`，命中返 `(content, served, tokens=0, cost=0, cached=True)`，存 stdlib `sqlite3`（`devkit/` 下）。加 `--no-cache` + TTL；**跳过 liveness 探针**（它要真打）。键里含 `served` 防 remap 串味。零依赖、零错误命中、idempotent 阶段（plan/brainstorm 重跑、dev 迭代）天天逐字重复。
2. **级联升级 = `--iterate` 的一个模式（#1）· M。** 现在迭代失败重跑同一载体；改成 `escalate` 阶梯（如 implement: `deepseek→glm→claude`），第 0 轮用最便宜的，**只在 golden 门真失败时升级**。验证器已现成（`_materialize_test`+`run_golden`）。风险=便宜模型"过了门但暗错"——保留跨厂商 `review` 作第二道强制门。
3. **prompt-cache 前缀结构（#3）· S–M，但先做 30 分钟验证 spike。** 给 Anthropic 系载体标稳定前缀（宪章+角色 system）可缓存、重试固定到同 deployment。**必须先验证**：发两次带标记的相同前缀请求，看第二次 usage 是否出现 cache read——若 CLIProxy 吞掉，则该法在订阅链路上死，只对直连 API 的 DeepSeek/GLM 有效（且它们本就自动 ~50% 无需标记）。订阅处省的是额度/延迟非钱。
4. （可选）**按阶段成本透视（#9 升级）· S。** `usage()` 加 `by_stage`（已有 `stage:` tag），让 #1/#2 的决策数据驱动。纯读侧、零风险。

---

## 4. 明确**不接 / 设为可选**（不合 Loom 轻量·本地·stdlib）
- **#4 LLMLingua-2 —— 拒**：需 torch+BERT 重依赖；与 `compact_text` 冗余；会破代码语法（对 TDD 流水线危险）。
- **#5 语义缓存半部 —— 拒**：需嵌入模型+向量库（重依赖）+ 1–15% 错误命中，对"产物会进 apply"的开发环不可接受。真要的话严格 opt-in 且**每次命中仍过 golden 门**才信。精确缓存已拿走大部分收益、零风险。
- **#7 TokenPilot 保前缀压缩 —— 暂拒**：只有 #3 经 CLIProxy 验证成立后才有意义。
- **#2 RouteLLM 完整训练路由 —— 设可选、绝不全局**：训练型路由是重依赖且越界；启发式版与"审查跨厂商独立"冲突，若用只限非对抗阶段、加 flag。
- **#8 TOON —— 保持窄**：仅 contract 出结构化（已是稳健 JSON）；散文阶段强压会掉推理质量。

---

## 5. 一句话结论
Loom 的**单一 `gateway_chat` 咽喉 + 已有的 golden 验证门**，让两个最高确定性的赢面——**精确响应缓存**和**把 `--iterate` 升级成 cascade-escalate**——都是低工作量、零新依赖、完全在 stdlib/本地约束内。prompt-cache 前缀结构值得一个验证 spike，但收益受 CLIProxy 边界挟持，故排在研究 agent 的 #1 之后。
