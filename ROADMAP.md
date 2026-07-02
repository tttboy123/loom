# Loom · 轻量化与迭代路线

> 现状实测（满栈）：≈ **2.5 GB 内存 / 4.6 GB 镜像**，瓶颈高度集中在 agent-ui 与 litellm。

## 占用画像（实测）

| 容器 | 内存 | 镜像 | 说明 |
|---|---|---|---|
| agent-ui | 1.23 GB | 1.38 GB | Next.js **dev 模式**常驻编译器 —— 最大头 |
| litellm | 1.02 GB | 1.71 GB | 网关核心，难裁 |
| agentos | 110 MB | 428 MB | 多 Agent 编排（聊天 UI 后端） |
| postgres | 45 MB | 663 MB | 用量/会话落库 |
| console | 24 MB | 203 MB | 极轻（纯标准库） |
| redis | 3 MB | 192 MB | 单机基本用不上（多副本才需要） |

---

## A. 轻量化（按性价比，逐项落地）

| # | 方向 | 预期收益 | 状态 |
|---|---|---|---|
| ① | **agent-ui 改生产模式**（`next build`+`start` 替代 `dev`） | 运行内存 1.23 GB → ~0.2 GB，省 ~1 GB；启动更快 | ✅ 已落地 |
| ② | **cliproxy 容器化 + `restart: always`** | 根治订阅代理「裸进程被杀」问题 | ✅ 已落地 |
| ③ | **lite 默认 / full 可选**（agent-ui+agentos 收进 `full` profile） | 默认栈 2.5 GB → ~1.1 GB | ✅ 已落地 |
| ④ | 缩 colima 内存 8 GB → 4 GB | 把内存还给宿主机 | ✅ 已降到 4 GiB（满栈占 ~1.3 GB，余量充足） |
| ⑤ | 预构建镜像、改 `pull` 不 `build` | 新人首启从「分钟级构建」→「秒级拉取」 | 🔜 |

落地原则：**默认轻、按需重**。`docker compose up -d` = 轻量核心；`--profile full` 叠加聊天 UI + 编排。

---

## B. 后续迭代（按价值）

### 🔴 让开发流程真正"能干活"
- **dev 阶段升级为带工具的 agent**：git worktree 内真实落文件 + 跑测试，把"草案"变"可运行+已测的改动"。
- **Eval Gate 落地**：devkit 自动跑 golden 集，用真实通过率决定 GO/NO-GO。
- **apply 前人类确认门**（对齐 RD-LOOP human gate）。

### 🟠 稳健性
- cliproxy 容器化（见 A②）+ 开机自启（`loom up` 脚本 / launchd）。
- colima/cliproxy 重启后自动恢复。

### 🟡 成本与可观测
- ✅ **控制台/台账显示每次运行 token + $ 成本**（按实际服务模型计，含降级；run-log 有逐阶段明细）。
- ✅ **网关调用打 `run:<ts>` / `stage:<key>` 标签** → LiteLLM 日志可按 devkit run/阶段过滤（实测落库）。
- LiteLLM budget 给每个角色设花费红线。

### 🟢 体验 / 低门槛
- ✅ **一键 `loom` CLI**（up/run/doctor/open/login/logs/down，自动拉起 colima）。
- ✅ **控制台运行流式进度**（发起后实时显示各阶段完成，替代轮询；console 控制 run-id）。
- ✅ **「✎ 改一改重跑」**（运行详情一键把任务回填到发起框）。
- ✅ **从 UI 改「角色→厂商」映射**（`/api/carriers` + `/api/remap` 写 config，重启网关生效；实测改 loom-tester→glm 生效再还原）。
- 控制台：产物 diff/摘要。

### ⚫ 安全
- ✅ **控制台访问 token**（`CONSOLE_TOKEN`，设了即全量鉴权；空=本地零摩擦）。
- `.env` 明文密钥 → 可接 macOS Keychain。

---

## 落地记录（本批 · 实测结果）

- **① agent-ui 生产模式**（Dockerfile.ui：`next build`+`start`）：运行内存 **1.23 GB → 136 MiB**（省 ~1.1 GB），仍正常 serve。
- **② cliproxy 容器化**（Dockerfile.cliproxy + compose `cliproxy`，`restart: always`）：订阅 token 由宿主机 `~/.cli-proxy-api` 挂载，linux 二进制读取无碍；网关改指 `cliproxy:8317`；订阅推理实测通过。**裸进程被杀问题根治。**
- **③ lite/full profile**（agent-ui+agentos 标 `profiles:[full]`）：
  - 默认 `docker compose up -d` = **lite ≈ 730 MiB**（cliproxy/console/litellm/postgres/redis）
  - `docker compose --profile full up -d` = **full ≈ 980 MiB**（+ 聊天 UI + 编排）
  - 对比改造前满栈 ~2.5 GB。

> 重新登录订阅（少见，token 会自动刷新）：在宿主机跑一次 `./cli-proxy-api --claude-login`（或 `--codex-login`）刷新 `~/.cli-proxy-api`，容器自动读到。

## 落地记录（第二批 · DX / 可观测 / 安全）

- **A 成本/用量可观测**（devkit/rdloop.py + console）：每阶段记 tokens + $（取 LiteLLM `x-litellm-response-cost`，按**实际服务模型**计，降级也准）；run-log 有逐阶段明细 + 合计，`RUNS.md` 台账加「用量」列，控制台运行表加「用量」列。合同测试同步更新为 6 列并通过。
- **B 控制台访问 token**（console/server.py）：`CONSOLE_TOKEN` 设了即对所有页面/接口鉴权（`?token=` 写 cookie / `X-Console-Token` 头均可）；不设则本地零摩擦。实测 401/200 均正确。
- **D 一键 `loom` CLI**（`agent-platform/loom`）：`up [full] / run / doctor / open / login / logs / down`，缺引擎自动 `colima start`。`loom doctor` 一眼看服务 + 端点。

## 对标 Wayland（github.com/ferroxlabs/wayland · 参考借鉴）

Wayland 是更成熟的本地优先 Agent 指挥中心（Electron + Rust 核 + 认知记忆分区 + 自组装团队 + OS 级沙箱）。我们走「轻量 / 易接入 / 与框架解耦」路线，已借鉴其最契合的一项，并标注后续可借鉴方向：

| Wayland 模式 | Loom 现状 | 行动 |
|---|---|---|
| **Constitution Framework**（平台层强制的明文规则） | ✅ **已落地** `CONSTITUTION.md`，注入每个角色（host + 容器内 devkit），控制台可查看 | done |
| 认知记忆分区（working/episodic/semantic… 跨会话演化） | ✅ **轻量 learnings 记忆**：`devkit/MEMORY.md` 记任务/Gate/审查要点，自动回注 brainstorm/plan（控制台 🧠 可看） | done |
| OS 级沙箱执行（Landlock / sandbox-exec） | ✅ **opt-in `--sandbox`**：agentic 执行器经 macOS sandbox-exec，写盘限沙箱+状态目录（实测越界写被拒、hermes 仍正常） | done |
| 自组装团队 + 黑板协作 | 固定 5 阶段流水线 + Agno Team | 🟡 可选：动态分派 / 共享黑板 |
| MCP / ACP 接外部工具与 CLI | 经 LiteLLM 走模型；未接 MCP | 🟡 给 dev/编排接 MCP 工具 |

> 定位差异：Wayland 重「桌面指挥中心 + 自治」，Loom 重「**研发流程套件 + 与任意 harness 解耦的模型载体**」——角色名当模型名用，换厂商改一处，门槛更低、更轻。

## 执行器层（embed hermes + openclaw）✅ 架构落地

devkit 现在支持**按阶段挑执行器**，可在一次运行里**同时嵌入**多种 agent harness：

```bash
python3 -m devkit "任务" \
  --executor implement=hermes \
  --executor review=openclaw      # 其余阶段默认 chat
```

| 执行器 | 说明 | 现状 |
|---|---|---|
| `chat` | 默认：经 LiteLLM 网关一次扁平对话（用 loom 载体，含 token/$ 计费） | ✅ 可用 |
| `hermes` | Nous Hermes Agent（`hermes -z`，带工具）；用 hermes **原生 provider** + 复用我们 `.env` 的厂商 key（默认 `deepseek/deepseek-chat`） | ✅ **实测产出真实 TDD 草案**，在 sandbox 里跑 |
| `openclaw` | OpenClaw `agent --local`（开源，同源于 hermes） | ✅ 已安装 + **非交互配好 deepseek**（`onboard --non-interactive` → `models auth paste-api-key` → `models set deepseek/deepseek-chat`），**实测在 loop 里出活** |

机制（对齐 Constitution）：
- agentic 执行器在**每阶段独立 sandbox 目录**（`runs/<ts>/sandbox-<stage>/`）里跑，不碰真实仓库；**apply 是人类门**。
- hermes 走**原生 provider**（经我们网关会触发 hermes「no final response」），key 复用我们 `.env`（deepseek/glm/minimax），`-m provider/model` 指定，缺省 deepseek。
- `loom doctor` 列出执行器可用性；`devkit/executors.py` 是适配器层，新增 harness 只加一个 `run_xxx`。

## 🔴 dev-as-agent 闭环（apply + test）✅ 落地

`devkit/apply.py` + run_loop：implement 产出 → **物化成文件** → **沙箱里跑 unittest** → 折进 Gate（测试失败 = NO-GO）→ **`--apply DIR`**（人类门）复制产物到目标。

```bash
python3 -m devkit "实现 X，给 x.py + test_x.py" \
  --executor implement=hermes --apply ./out   # 测试绿才 apply
```

实测：hermes 实现 `reverse.py` + `test_reverse.py` → 物化 → **unittest 7 通过** → Gate GO → apply 到目标目录（真实可运行、已测代码）。这一步把流程从「出草案」变「**出可运行 + 已测的改动**」。

**已扩展（第三批）**：
- **多文件 / 子目录**：materialize 支持 `src/x.py`、`tests/test_x.py` 等相对路径（拒绝 `..`/绝对路径）；unittest 递归发现；有 `requirements.txt` 尽力 `pip install --target _deps`。实测多文件项目跨目录 import + 测试通过。
- **控制台一键 apply**：运行详情显示 `build/` 产物 + 测试结论；**测试绿**才显示「⬇ 一键 apply」按钮（人类门），POST `/api/apply` 复制到 `devkit/applied/<ts>/`（宿主机可见，保目录结构，跳过 `__pycache__`）。测试未过则禁用 apply。
- **三执行器全部 live**：`loom doctor` = chat✓ / hermes✓ / openclaw✓。

**已扩展（第四批）**：
- **跨运行学习记忆**（`devkit/memory.py` + `MEMORY.md`）：每次运行记录任务/Gate/审查要点；下次自动把最近若干条作为「过往教训」注入 brainstorm/plan，让流程**越跑越聪明**。控制台侧栏「🧠 学习记忆」可看。
- **apply-to-git**（`--apply-git REPO [--branch NAME]`）：测试通过时用 **git worktree** 在 REPO **新建分支 + commit 产物（不 push）**，**不动用户当前 checkout**（人类门）。实测分支/commit/文件正确，main 保持干净。
- **测试运行器加固**：优先 **pytest**（兼容 unittest 与 pytest 风格用例），缺则按需 `pip install --target _deps`（绕过 PEP 668），再回退 unittest。修掉「模型产 pytest 风格用例 → ImportError」。

**已扩展（第五批）**：
- **Eval Gate**（`devkit/evals.py` + `--golden FILE`）：golden 用例（python 表达式/期望 或 子进程/stdin/包含）跑在 build 上，**任一不过即 Gate NO-GO**（即使单测绿）。实测：正确 golden→GO；故意错 golden→NO-GO。
- **OS 级沙箱**（`--sandbox`）：agentic 执行器经 macOS `sandbox-exec`，**写盘只许沙箱 + agent 状态目录 + tmp**；实测越界写被拒、hermes 仍正常。
- **控制台「✎ 改一改重跑」**：运行详情一键回填任务到发起框。
- **控制台「角色→厂商」热改**：`/api/carriers` 列出 loom-* 当前后端，下拉改成 5 个基础后端之一 → 写 `config.full.yaml`（console 现以 rw 挂载）→ `docker compose restart litellm` 生效。
- **colima 8G→4G**：满栈占 ~1.3 GB，余量充足。

## 对标 Kode（shareAI-lab/Kode-CLI · 参考借鉴）

Kode 是多模型协作的终端 coding agent。多数能力 Loom 已有对应（Model Pointers≈loom 角色载体、/cost≈每运行 token/$、Plan/Safe≈report-only+人类门、sandbox≈`--sandbox`、可分享 profile≈config）。已借鉴其最有特色的一项：

- ✅ **@ask-model 专家咨询**（`/api/ask` + 控制台「快速咨询」卡片）：选任一载体/后端临时问一句，出答案 + token/$，**不开整条 loop**（容器内可用，复用网关计费）。实测 deepseek/订阅载体均可。
- ✅ **上下文压缩（compact 指针）**：长上游产物先用便宜模型（默认 deepseek）压成要点再喂下游，替代粗暴截断。每件只压一次、成本计入总账。`--compact-model` / `--no-compact`。实测 4150→415 字、保留结论/接口/约束/风险。
- ✅ **并行多模型对比**（并行子 agent 的轻量形态）：`/api/ask` 支持逗号多模型 → 线程池**并行**问、并排比较；控制台「⇉ 对比后端」一键并行问 5 个基础后端。实测 3 后端并行 ~7s（非串行累加）。
- ✅ **软预算护栏**：`--budget $X`（控制台发起框也有「预算$」），本次累计花费超线即停掉剩余阶段并 NO-GO。实测越线即停。
- ✅ **改动预览（diff）**：控制台运行详情「⇄ 改动预览」对比上一次带 build 的运行，彩色 unified diff（配合「改一改重跑」看改了什么）。实测自动选基线 + 正确 diff。

- ✅ **角色×载体不再写死（用户自定义）**：角色/阶段从用户文件读取（`loom.roles.toml`/`.json`，TOML 多行提示友好，回退内置默认），`devkit/roles.py` 单一加载器；CLI `devkit roles init|list|path`（`./loom roles`）+ `loom.roles.example.toml`。**最低接入成本**：`carrier` 直接写网关已知后端名（deepseek/glm/...）→ 零额外配置、免重启；也可用 `loom-*` 载体享受热改。控制台「角色」面板同源显示。实测：自定义 `spec→build→audit` 流水线端到端驱动 loop、产物用自定义阶段名；校验（缺 system / key 重复）报错清晰。
- ✅ **角色文件完整自描述（executor + max_tokens 入文件）**：`Stage` 加 `executor`（chat/hermes/openclaw）与 `max_tokens`（每阶段 token 上限）；run_loop honor 之，运行时 `--executor`/`--max-tokens` 仍可覆盖。一个角色文件即可定义「用什么模型 + 什么 harness + 多少 token + 什么职责」。CLI `roles list`、控制台编辑器、脚手架、JSON 都带这两字段；校验非法 executor/负 max_tokens。实测：文件里 build=hermes，不传 `--executor` 也自动 `via hermes`。
- ✅ **控制台在线编辑角色**：「✎ 编辑角色流水线」卡片 —— 改字段（含 executor 下拉 + max_tokens 数字框）/ 加删阶段 / 上下移 / 保存（`/api/roles/full|save|reset`）。控制台写入 `devkit/loom.roles.toml`（容器可写 + 宿主机共享，find 顺序加了 `ROOT/devkit`），**与 CLI `devkit run`/`roles` 同一份不漂移**；保存先校验过不了不落盘，「恢复默认」删文件回内置。实测：缺 system 被拒、合法保存后宿主机 CLI 与控制台都读到同一份、reset 回默认。
- ✅ **测试硬化**：`devkit/test_features.py` 用打桩网关 + 临时文件单测了压缩回退 / 并行咨询保序聚合 / diff 状态判定（new/changed/deleted/same + 自动基线）/ 角色加载与校验（TOML+JSON+回退）；`./loom test` 一条命令跑合同 + 单测（纯标准库、无需 live 网关，CI 友好）。共 14 个用例全绿。
- ✅ **CLI ↔ 控制台对齐**：把 @ask-model 与 diff 抽成 `devkit/ask.py`、`devkit/diff.py` 单一实现，控制台 `/api/ask`、`/api/diff` 改为委托——**同一份逻辑两处用，不漂移**。CLI 侧新增子命令：`devkit ask <prompt> --models a,b`（单个/多个并行对比）、`devkit diff <ts> [--against <ts>]`（彩色 unified diff，tty 才上色）；`./loom ask` / `./loom diff` 快捷方式。`devkit "任务"` 默认仍是跑 loop（向后兼容）。

- ✅ **额度薅羊毛 + 模型评分**（`devkit/insight.py`，CLI `devkit quota|scores|rate` + 控制台面板，共用同一份）：
  - **薅羊毛**：`loom.quota.toml` 声明各后端免费额度/订阅；用 LiteLLM `/spend/logs` **真实花费**算剩余，按"薅价值"排序（订阅 > 剩余免费多 > 付费），默认 claude/codex=订阅。诚实边界：只统计经本网关花掉的部分。
  - **评分**：实际使用分来自 run-log（成功率/延迟/成本，loom-* 载体经 config 解析回后端）+ 用户 👍/👎（`devkit/ratings.jsonl`）；官网评测分**不杜撰**，来自用户维护的 `loom.scores.toml`。综合 = 实际 0.5 + 用户 0.2 + 官网 0.3（按存在项归一化）。实测：scores 读真实历史（claude 27 次 96%）、quota 默认正确、rate 落库进综合。单测 5 个（归一/解析/评分/额度/综合）。

## 对标 Anthropic「Planner→Generator→Evaluator」长跑 harness（AI Engineer workshop / Prithvi 工程博客）

文章论点："赢家拥有最好的循环"。三角色对抗（GAN 灵感）+ Sprint Contract + 真机验证 + 状态管理 + "单位接受变更成本"指标。Loom 已有三角色 + 跨厂商独立评判 + 外部 gate + 记忆/git 状态；**缺的正是"循环"本身**（一次性 pipeline）。本批补上：

- ✅ **迭代循环 `--iterate N`**（run_loop）：评判（测试/Eval/审查）NO-GO → 把失败明细回灌构建者（implement）重做 → 重物化重测重评 → 直到通过或达 N 轮。runtime/角色文件的 executor、carrier、max_tokens 全程沿用。收尾报**收敛/未收敛 + 迭代花费**（= 文章的 accept rate / 单位接受成本；未收敛即"空烧"）。迭代模式下评判 REQUEST-CHANGES 也进 gate。控制台发起框有「迭代N」。`iterate=0` 默认 = 原行为，向后兼容。
- ✅ **反馈质量两处加固**（让循环真能收敛）：① Golden 评测异常分支也带 `want=` 期望值；② 迭代反馈纳入 Golden 明细（含 want=）+ 审查 critique，而非仅单测 stdout。实测：round0 过 → 0 轮（不浪费）；round0 错（大写 Fizz）→ 反馈带 `want=[...'fizz'...]` → 1 轮收敛 GO；模型坚持己见 → 达上限 NO-GO + 空烧提示。
- ✅ **Sprint Contract `--contract N`**（`devkit/contract.py`）：implement 前评判者（独立载体）先产出 ~N 条**可机器验证**的验收点（golden 格式），注入构建者（照此实现接口/文件名）+ 作 Eval Gate + 迭代反馈。借此还修了 golden：**新增 `raises` 断言**（非 happy-path 可表达且可满足），并让异常分支也带 `want=`。实测：评判者约定 5 条（含 raises 错误用例）→ 实现一次过 → 0 轮收敛 GO。
- ✅ **Initializer + 单特性增量**（`devkit/backlog.py`，`devkit backlog`/`feature`）：Initializer 把想法拆成特性清单（backlog.json + progress.md + 项目目录）；`feature` 取优先级最高的 todo，读当前代码库增量实现一个特性、跑测试、**绿了才标 done** 并落进度（强制增量/测试/干净交接，状态可恢复）。借此修了两个**全局受益**的真 bug：materialize 漏进文件的 markdown 围栏（致 SyntaxError 污染整批测试）、0 文件误判 done。
- ✅ **真机 Web 验证**（`evals.py` 新增 `web` 用例）：启动应用 → 真实 HTTP 请求 → 校验 status/body，**纯标准库、无浏览器依赖**。另有可选 `playwright` 用例（装了才跑真浏览器，脚本用 `LOOM_BASE_URL`；没装则 `⏭` 跳过、不拉低 gate）。诚实取舍：Playwright+Chromium 是重依赖、与 Loom 轻量化相悖，故默认走 HTTP 真机验证，浏览器验证留作可选。
- 🟡 仍可借：合同谈判做成构建者↔评判者多轮往返；feature backlog 接 apply-to-git 每特性一 commit。

## 自举：开发 Loom 的 agent 也跑这套 harness（dogfood）

不只把"最好的循环"做进产品，开发 Loom 本身也按它来：**先立可测验收合同 → 增量做 → 真机验证 → 交独立评判者（spawn 的 code-review 子 agent，不自评）→ 不过就修再评 → APPROVE + 绿才收**。

- ✅ **后端真活性探针**（`devkit/insight.py` `liveness/probe_one` + `loom doctor` 段 + 控制台 `/api/liveness`，60s 缓存）：对 5 个基础后端**关降级**（`disable_fallbacks`）做最小推理探测 + **核对服务模型确为探测目标**（防降级假阳性），并发探测。补上了"挂了被降级悄悄兜住"的盲区——实测如实标出 claude/glm/minimax DOWN（带真实报错）。
  - 这一条是**用新立的 dev-harness 做的首个示范**：独立评判子 agent 第一轮 **REQUEST-CHANGES**（抓到 `fallbacks:[]` 并不能关降级、应 `disable_fallbacks:true` 的 [blocker]）→ 修复（开关 + 服务模型核对 + 并发 + 缓存）→ 第二轮 **APPROVE**。自评根本抓不到这个 blocker。
- ✅ **控制台「后端真活性」面板**（#38，懒加载、点刷新才探测、不烧额度）+ ✅ **`devkit feature --commit`**（#36，每特性绿了就地一 commit、幂等 init、.gitignore 排除构建产物、headless 身份、fail-open）。两者都用**完整 agent-team loop**做：构建者=我、评判者=独立子 agent，**编码前协商合同 + 编码后独立验收**。协商当场各拦下一个我会发的真 bug（面板会在开页时烧 5 次计费推理；commit 用错了 worktree 模型），验收各 APPROVE。这就解决了"设计完不验收"——设计者 ≠ 验收者。UI 改动还接了 Playwright 真机验证 `./loom verify-ui`。
- ✅ **`--contract-rounds N`**（#37，合同多轮协商：评判者拟好后构建者再 N 轮收紧/修正，带**反削弱地板**——最终条数与 raises 用例数不得少于评判者原稿，评判者有最终否决权）。协商当场拦下我漏掉的削弱风险（自利构建者会删掉 raises 错误用例、把合同改弱），验收 APPROVE（含一个 should-fix：独立性测试要测真实载体路由 + 同载体告警，已补）。
- ✅ **测试/物化稳健性**（#39/#40，apply.py）：① run_tests 把 pytest 装到**共享缓存** `~/.loom/pydeps`（一次、全局复用，`_addpath` 幂等），消除"每次重装/首次偶发失败掉回 unittest → 假未过"；② materialize **0 文件兜底**：模型没标文件名时，从代码块推断（实现名取测试的 `from X import`，支持点路径 `pkg.x→pkg/x.py`），避免白跑。独立验收 APPROVE（dotted-import should-fix 已补）。这两条直接修了"盘上其实通过却报失败"与小模型常见的"0 文件"。
- 诚实记录：跑到这里，小模型 feature 收敛主要卡在 **deepseek 是当前唯一可用的快后端**（自家 liveness 探针实测 claude/glm/minimax 全 DOWN）——属后端可用性问题（需你 `./loom login` / 换 MiniMax key / GLM 充值），非 Loom bug。
- ✅ **MiniMax 修复**：LiteLLM 原生 `minimax/` provider 报 `invalid api key 2049`（端点/构造不对），改 OpenAI 兼容 `openai/MiniMax-Text-01` @ `api.minimaxi.com/v1` 后通（key 本身没问题）。
- ✅ **K8s 探针式后端健康**（#42，`insight.health()`）：把"真活性 liveness"与"凭据就绪 readiness（读 `~/.cli-proxy-api/*.json` 的 `expired`）"合成 `serving/expired/down` 三态——Claude 订阅 OAuth token 过期不再表现成莫名的"cooling down"，而是 **⏰ 过期需重登 → run ./loom login**。完整决策表（disabled/过期且打不通/过期但在服务/真挂/服务中）、ISO 容错（Z 与 +08:00、不可解析不误判）、多 token 文件取最新、控制台读不到 token 时可见降级 + 可选只读挂载。独立验收 APPROVE，51 测试绿。
- ✅ **精确响应缓存**（#43，`devkit/cache.py` + `gateway_chat` 内包缓存）：键=sha256(model+system+user+max_tokens+config-mtime+schema)，刻意排除 tags/timeout/served；命中返 `(True,content,served,0,0.0)` 免费、台账/预算仍对；缓存逻辑在 `gateway_chat` 内（HTTP 移到 `_uncached_gateway_chat`）→ 现有 monkeypatch 单测照常过。线程安全（每次新连接 + WAL）、坏库降级不崩、schema 自愈、TTL 惰性清理 + 硬上限淘汰最旧。`--no-cache` / `LOOM_NO_CACHE=1` 关。实测 `ask` 同问第二次 0tok $0；61 测试绿。（独立验收抓出 2 个 should-fix：坏库连接泄漏、非 dict 行崩调用方——均已修+加测；因 session 反复限流未再 spawn 复核，这两处以测试自证。）
- ✅ **cascade-escalate**（#44，`--cascade c1,c2,...`，借鉴 FrugalGPT）：implement 用 cheap→strong 阶梯，初始最便宜档，**golden/审查 NO-GO 才升级下一档**，蕴含 iterate；与 `--carrier implement=` 互斥（CLI 报错）；坏档自动升级、末档坏即 NO-GO；run-log 记阶梯与实走路径。实测 `deepseek→glm`：deepseek 过不了 fizzbuzz golden → 升级 glm → 收敛。独立验收 APPROVE（2 个单档退化路径 should-fix 已修），63 测试绿。
- ✅ **prompt-cache 前缀结构（#45，验证 spike → 结论：不需要建）**：实测 DeepSeek 自动前缀缓存**经网关已生效**——同前缀不同后缀第二次请求 `prompt_cache_hit_tokens=3200/3211`（约 10% 价），LiteLLM 透传折扣到 cost。原因：Loom 的 prompt **本就稳定前缀在前**（system=宪章+角色，user=任务+上游），直连 API 后端（deepseek/glm）自动缓存白拿。Anthropic 需显式 `cache_control` 的那条路被 CLIProxy + `drop_params:true` 挡住（且 claude 当前过期，测不了），不确定。**故不建显式标记，只需保持"稳定前缀在前"别破坏。** token 优化批收官：缓存(#43) + 级联(#44) + 前缀缓存(自动，#45 验证) 三件已落，剩可选的 usage 按阶段透视。

**有意不做 / 已被覆盖**（避免重复造轮子）：
- **chat 侧接 MCP**：**冗余** —— hermes/openclaw 执行器原生支持 MCP/工具，要工具就 `--executor implement=hermes`，没必要把扁平 chat 复杂化。
- **预构建镜像 pull**：需要你自己的镜像仓库账号与推送授权，留作可选（届时把 compose 的 `build` 换 `pull` 即可）。
- **.env → Keychain**：当前 `.env` 明文可用；属体验增强非能力缺口，按需再接。
