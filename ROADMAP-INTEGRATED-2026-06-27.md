# Loom 整体落地路线（整合版 2026-06-27）

> 本文把三份输入合一：①GPT 的《Loom 完整评审报告》(13 大能力/8 Plane/26 对象) ②我先前的
> 《STRATEGY-2026-06-27 评估》(Part1 评估 / Part2 方案A 上下文记忆 / Part3 方案B 模型子Agent流水线)
> ③《DESIGN-P0-artifact-bus》(P0 细化稿) ④已完成的 #1–#46。
> 目标：给一条**依赖有序、防过度工程、含已有内容**的落地路线，而不是把 13 个模块平铺。

---

## 0. 一句话结论
GPT 报告的 13 个模块**选得对，但排序错**。正确顺序不靠拍脑袋——它**从"每个模块读哪份数据"自动落出来**。
所有 Plane 最终都读同一条东西:**结构化 Artifact 数据脊**(TaskPack→Plan→Patch→VerifyReport→ReviewReport→Score→CostReport)。
报告自己在 §20.1 写了"Agent 不互信、Artifact 互信"——但把它当**原则**列在最后。本路线把它**提升为第一个要建的东西(Phase 0)**,因为后面 Quota/Fitness/Task Center/Asset/Radar/Learning **全部读它**。先建脊,其余顺序自然成立。

> 而且 Phase 0 **双重紧迫**:它既是产品地基,又是你刚指出的"为什么 Loom 开发还全靠 Claude Code"的解——
> 有了可靠 Artifact 交接 + Codex 验证,implement 才能安全甩给便宜模型,Loom 才能开始被便宜模型开发。

---

## 1. 把 13 大能力按"读哪份数据"重排(这就是"怎么做"的依据)

| 能力(GPT报告) | 读/写什么 | 依赖 | 落点 | 现状 |
|---|---|---|---|---|
| **Artifact 总线 + 受保护字段**(方案A) | *定义*数据脊结构 | 无(是脊) | **P0** | 雏形:rdloop 现在拼字符串交接 |
| **Context Budgeter** 窗口自适应(方案A) | 读 Artifact + carrier 窗口 | Artifact | **P0** | 有 compact_text，缺窗口比例分配 |
| **模型即子Agent流水线**(方案B) | 跨阶段写/读 Artifact | Artifact | **P0** | roles+executor+#44 cascade 已能拼 |
| **Codex executor**(方案B) | 写 VerifyReport | executor 层(已有) | **P0** | 新工程,先 spike |
| **Agent Registry**(roles→agents/pipelines) | 定义 Agent/Pipeline 对象 | roles.toml(已有) | **P0→P1** | roles.toml 已是雏形 |
| **PonyTail 反过度工程门** | 读 Patch→ReviewReport | Artifact | **P1** | 新,接 review 阶段即可 |
| **Quota Preflight(估自历史)** | 读 `stage_report` 均成本 + `quota_report` 剩余 | **零** 脊字段(现成数据) | **P0** | 输入全有,只差一条 simulate |
| **Model Fitness 按任务分桶** | 读 run Score + `task_type` | Artifact(`task_type`) + #46 stages | **P1** | `stages` 按**阶段**已有;按**任务类型**分桶是新工作 |
| **Quota Wallet 余额 adapter** | provider 余额 API(分4级) | provider adapter(全新) | **P1** | 现 `quota` 的 remaining=静态配置算的,**provider 余额 API 未接** |
| **Task Command Center**(UI 深化/证据链) | 读 Run/Stage/Artifact/Verify | Artifact + console(已有) | **P2** | console 已有 run/diff/apply |
| **OpenCode/OpenHands/OpenWork 接入** | executor/backend adapter | executor 层 + Artifact | **P2** | executor 层已可扩展 |
| **Loom Asset + Importer** | 定义 Asset manifest | Asset schema | **P2** | 全新 |
| **Community Radar** | 扫描→Asset | Asset + Importer | **P3** | 全新 |
| **Workflow Migration 全家桶** | import 各工具→Agent/Asset | Registry + Asset | **P3** | 全新 |
| **Learning Sidecar**(只读、可审计) | 读 Run events→Suggestion | Artifact + MEMORY.md(已有) | **P3** | MEMORY.md 已是雏形 |
| **Safety Presets 声明式分级** | 权限→Agent/Asset | Registry + sandbox(已有) | **P3** | sandbox+human gate 已有 |
| **小白模式 / 专家双入口** | 包装以上全部 | 几乎全部 | **P4** | 全新 |

**读法**:一个模块不能先于它依赖的数据被建。脊(P0)没铺好,Quota 的"任务能否完成"没有成本样本可读、Fitness 没有评分样本、Task Center 没有结构化证据可展示、Learning 没有事件可复盘。所以**P0 不是"我偏好",是硬依赖**。

---

## 2. 现状盘点:别重造已有的(防过度工程 = PonyTail 精神)
GPT 报告的 26 对象 / 8 Plane,Loom **已有约一半雏形**:
- **已成形**:Model/Provider/Carrier(config)、Run/Stage/Artifact(runs/*)、Sandbox、ApplyGate、Cost、Score(scores+#46 stages)、Memory(MEMORY.md)、Pipeline(roles.toml)、Executor(chat/hermes/openclaw)、Quota(spend 读取)、Health(#42)、Cache(#43)、Cascade(#44)。
- **需升级**:roles→Agent/Pipeline Registry;(stage,text)→typed Artifact;quota→Wallet+Capacity。
- **全新**:Asset/Radar/Migration/Learning Sidecar/Safety 分级/小白模式/OpenHands·OpenCode·OpenWork adapter。
> 评审红线:每个 Plane 先问"Loom 现在有没有雏形",有就深化、绝不推倒。一轮只动一个,先出最小可验证版,独立验收。

---

## 3. 分阶段落地路线

### Phase 0 — 内核地基(NOW)｜解 token 短缺 + 铺数据脊 + 让便宜模型能开发 Loom
**已细化**(见 DESIGN-P0-artifact-bus.md)。交付:
- `artifact.py`:typed 产物 + `extract_fields()`(第一版零改 prompt)。
- 受保护字段 `{contract, failure, patch_targets}`:永不压缩、放首尾。
- `budget.py`:Context Budgeter,按 carrier 窗口比例分配,32k–128k 同一套码;输出 `report`(带了什么/压了什么/丢了什么)写进 run-log。
- **Quota Preflight(估自历史)** ←评审 R1 提前到 P0:`loom quota simulate "<任务>"` = `sum(各阶段均成本 from stage_report) vs remaining_usd from quota_report → Safe/Risky/Insufficient/Unknown`。**输入今天全有,零脊字段依赖**,是订阅烧额度场景下最高 ROI 的一条,不该等 P1。(provider 余额 API adapter 仍留 P1。)
- **P0-b**:`codex` executor(先 spike:非交互 sandbox 跑测试回 VerifyReport;过了再集成,不过则 verify 回退现有 `run_tests`)+ **目标流水线落地** `Claude=plan+review / DeepSeek·GLM·MiniMax=implement(并行或#44级联) / Codex=verify`(依赖 Codex,故归 P0-b)。
**为什么第一**:见 §0/§1。**切分**:**P0-a**(artifact+受保护字段+budgeter+改 rdloop+Quota simulate,纯 stdlib、不依赖 Codex)先做并单独验收;**P0-b**(Codex executor+流水线)随后。

### Phase 1 — 可信 & 省钱｜读 P0 的数据脊
- **Recipes(命名预设)** ←评审建议提前:`loom run "<任务>" --recipe cheap-dev` 一键套"Claude设计→便宜开发→Codex验证→Claude审查"。这是"专家编 TOML"与"小白选模式"之间的**中间档**,缓解易用性断崖(见 §6),成本低,随 Registry 一起落。
- **Model Fitness 按任务分桶**:`scores` 升级成"backend-fix/review/test-gen… 各自榜"(注:`stages` 现按**阶段**聚合,**按任务类型分桶是新工作**,靠 P0 锁的 `task_type` 字段),指标 pass-rate / cost-pass / Codex-verify-pass / human-accept。喂路由。
- **Quota Wallet 余额 adapter**:provider 余额 API(分 4 级,DeepSeek 有余额 API、订阅型只能估)+ Drain Mode。(simulate 已在 P0 落。)
- **PonyTail 门**:review 阶段挂 `gate=ponytail`,diff 太大/乱加依赖→NO-GO 回 implement。先硬编码一个 PonyTail 风格 review prompt 验证价值,不必先做 Asset 全家桶。escalation 可借 AutoMix 思路:用便宜 Codex/PonyTail 自检信号先判,再决定是否升级到强模型(省最贵那一跳)。
- **Agent Registry 升级**:roles→`[[agents]]`+`[[pipelines]]`,Agent 带 model/executor/skills/permissions/fallback;`loom run @pipeline/cheap-backend-fix`。

### Phase 2 — 生态接入(兑现 harness 中立)｜读 P0 的 executor 接口
- **OpenCode executor / OpenHands backend / OpenWork bridge**:把它们当 executor/backend 接进来,**不重写**。`loom run --executor implement=opencode`、`loom backend add openhands`。这是对 §7 "OpenHands Agent Canvas 威胁"的正面回应:不抢 runtime,抢"额度+评分+验证+调度"层。
- **Loom Asset + Importer**:统一 manifest(`loom.asset.toml`),先支持 import skill/rule/MCP(PonyTail/Caveman 当首批试点)。
- **Task Command Center**:console 深化成"任务证据链"(谁做的/花多少/改哪些文件/测试过没/可不可信),读 P0 的 Artifact。

### Phase 3 — 规模化 & 治理｜读 Asset/Run 历史
- **Community Radar**:扫 GitHub/Awesome/MCP 列表→分类→安全扫描→生成 Asset→smoke bench→推荐。
- **Workflow Migration 全家桶**:`migrate claude-code/codex/cline/roo/aider`、`bridge openclaw/hermes`。
- **Learning Sidecar**:**默认只读、可审计**,读 run events→出"规则/模型/quota/golden"建议,以 diff 给用户 Accept/Reject。**不**默认给它改代码/装 skill/动配置的权限(§15.4)。
- **Safety Presets**:Level 0–6 声明式权限,外部 asset 默认 Level 0,审查后提升。

### Phase 4 — 受众扩展｜包装以上全部
- **小白模式**:任务卡片("帮我整理文件夹")、模式选择(省钱/稳妥/隐私/高质量,隐藏 model/carrier)、权限像手机 App、默认不动原文件、结果页回答"能不能信"。
- **专家控制台**:Models/Agents/Pipelines/Assets/Quota/Scores 全暴露。
- 渐进式复杂度:新手看卡片、进阶用 Recipe、专家编 TOML。

---

## 4. 竞品区隔(含 OpenHands Agent Canvas 新增)
- **OpenHands Agent Canvas**(最近的对手,已抢"developer control center"叙事 + ACP + automations):Loom 别只讲 control center,讲 **Quota-aware Agent Control Plane**——差异在"额度够不够 / 哪个模型最适合 / 哪个 workflow 通过率高 / Artifact 互信独立审查"。**接入它当 backend,不正面对抗**。
- **OpenCode**=执行器(接入当 executor);**OpenWork**=桌面 coworker(别做 clone,做管理它的控制面);**LiteLLM**=底座(继续用,差异在上层语义路由);**Agyn**=企业 K8s Infra(别走重平台路线,借鉴 spend caps/RBAC/audit);**Dify/Flowise**=app builder(别变通用 builder)。
- 一句话:*OpenHands 让 coding agents 跑起来;Loom 让所有 AI 资源被看见、被预算、被调度、被验证、被评分、被持续优化。*

## 5. 红线 / 不做清单
1. 不做 OpenWork/OpenHands clone、不做通用 app builder、不做 ChatGPT 客户端、不做又一个网关。
2. 不在内核(Phase 0–1)稳之前碰 Radar/Migration 全家桶/Cluster/桌面小白模式/Asset 市场。
3. 不走 K8s-native 重平台(那是 Agyn 的地)。
4. Free=平台免费/本地/自带 key,**不**承诺无限模型、不绕厂商限制、不刷额度。
5. 默认安全:外部 asset 不信任、新手只读/副本、真实改动必经 human gate。
6. 一轮一个模块、最小可验证、独立验收(避免 Ralph Wiggum 自评)。

## 6. 双视角评审(Agent Team 两个 lens,落地时由独立子 agent 复核)
- **Builder/可行性**:Phase 0 全部纯 stdlib、改动集中在 rdloop 组 user 那一段 + 两个新文件 + 一个 executor 分支,可控;Codex 先 spike 降风险;每阶段都有"退化不比现在差"的兜底。
- **Reviewer/易用性+后续演进**(独立子 agent 复核结论 APPROVE-WITH-CHANGES,已并入本稿):① 易用性——主要只改 roles.toml,`--context-mode auto` 兜底,run-log 显示带了什么(可调试);**易用性断崖真实存在**(P0–P2 全是"改 TOML",小白模式却远在 P4)→ 已把 **Recipes 提前到 P1** 作中间档;② 演进——Artifact 脊是后面所有 Plane 的共同地基,**前提是 P0 就锁死 `task_type/cost/verdict/tests_passed/window_used` 字段**(见 DESIGN §3),否则 Fitness/Quota/Learning 全要回填迁移;③ 风险——**char/token≈3.5 对中文是硬错(CJK≈1–1.7 字/token,会撑爆窗口)**,P0 改用网关 `/utils/token_counter` 或 CJK 取 1.5 下限,不准发 3.5;P3 检索留 retriever 插槽;Codex 非交互鉴权先 spike。

**社区/论文借鉴**(评审补充,直接改具体决策):
- Anthropic *Effective context engineering / context-editing + memory tool*(2025):P3/P4 长尾别预打包进 prompt,把上游产物做成**可取回(fetchable)**让 worker 按需拉——绕开长尾的 char/token 估算。
- Anthropic *多 Agent research 系统*(2025):lead 只传**目标+边界**不传 transcript;子 agent token 是主成本——印证"受保护字段 + 上下文隔离"应排第一。
- *AutoMix*(Madaan 2023):用便宜自检的 meta-verifier 决定是否升级,而非全测后才升级——喂 P1 的 cascade。
- *Aider architect/editor、RouteLLM/FrugalGPT、Lost-in-the-Middle、MemGPT、LLMLingua-2*(已在 STRATEGY Part2/3 引)。

## 7. 下一步
Phase 0-a(`artifact.py`+受保护字段+`budget.py`+改 rdloop 组 user)→ 我起草 Sprint Contract → 独立 code-review 子 agent 协商 → 建(implement 可试甩给便宜模型,我做 plan+review,测试门把关)→ 真机验证 → 独立验收。严格 Loop,子 agent 撞限额就停下等额度。
