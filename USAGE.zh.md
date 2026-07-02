# Loom 开发套件 · 使用文档

> 🌐 English version: [USAGE.en.md](USAGE.en.md)
> 本地部署的「多 Agent + 多厂商模型」研发套件。轻量、可本地跑、上手门槛低。

---

## 这是什么

让**不同的模型分担研发的不同角色**——产品、编排、开发、测试、审查——并用一个统一网关调度、一个全局控制台观测。

```
你（浏览器） →  全局控制台 :8899  →  LiteLLM 网关 :4000  →  5 个模型后端
                （发起/看产物/看用量）   （统一·自动降级）     Claude订阅 / Codex订阅 / GLM / DeepSeek / MiniMax
```

---

## 30 秒上手（零基础也能跑）

1. **启动**（最简单：一键 `loom`，缺 Docker 引擎会自动拉起 colima）：
   ```bash
   cd agent-platform
   ./loom up            # 轻量核心（控制台+网关，≈730MB）
   ./loom up full       # 满栈（+聊天 UI + 编排，≈980MB）
   ./loom doctor        # 体检：服务 + 端点
   # 或用原生 compose：
   docker compose up -d                 # = lite
   docker compose --profile full up -d  # = full
   ```
2. **打开控制台**：浏览器访问 **http://localhost:8899**
3. 在「**发起运行**」框里用一句话描述一个开发任务 → 点「**运行 ▶**」
4. 几十秒后，在「**运行记录**」里点开那一行 → 看产品判断 / 计划 / 代码 / 测试 / 审查五份产物

> 第一次跑什么 key 都不用：默认勾选「订阅替代」，用你的 Claude / ChatGPT 订阅就能跑通（审查仍是跨厂商）。

---

## 我想…（一张表看怎么做）

| 我想做的 | 怎么做 | 在哪 |
|---|---|---|
| 跑一条完整研发流程 | 控制台输入任务点「运行」 | http://localhost:8899 |
| 看每次运行的产物 | 控制台「运行记录」点开任一行 | 同上 / 文件 `devkit/runs/<时间戳>/` |
| 跟某个角色单独对话 | 选角色 Agent 聊天 | Agent UI http://localhost:3000 |
| 看调用量 / 花费 | 控制台「网关用量」面板 | 同上 / LiteLLM http://localhost:4000/ui |
| 换某个角色用的模型 | 改一个配置文件 | 见下方「换模型」 |

---

## 角色 = 你自己定义（不写死）

**默认**给一套开箱即用的 5 角色流水线：

| 角色 | 默认模型 | 作用 |
|---|---|---|
| 产品逻辑 | GPT-5.4 | 把需求变成产品判断与取舍 |
| 编排 | GPT-5.4 | 拆任务、定计划、分派 |
| 开发 | MiniMax-M3 | TDD 写代码 |
| 测试 | MiniMax-M3 | 验证、eval |
| 审查 | GPT-5.4 | 独立审查，抓“看似完成实则没完成” |

> 核心理念：**审查用和开发不同厂商的模型**，盲区不同，才能真正挑出问题。

但这套**不是写死的**——每个角色（阶段）的名字、顺序、用哪个模型、系统提示，都由你一个文件说了算：

```bash
devkit roles init        # 从默认生成可编辑的 loom.roles.toml（参考 loom.roles.example.toml）
devkit roles list        # 看当前生效的角色流水线
devkit roles path        # 看用的哪个文件（没有则=内置默认）
./loom roles             # = devkit roles list
```

`loom.roles.toml` 里每个 `[[stages]]` = 一个角色（一个文件就能完整描述「这个 Agent 用什么模型、什么 harness、给多少 token、什么职责」）：
```toml
[[stages]]
key = "spec"            # 阶段键
role = "产品"            # 角色名（文档用）
title = "需求拆解"
carrier = "deepseek"    # 指向哪个模型 ← 见下
executor = "chat"       # 执行器：chat（默认）| hermes | openclaw（带工具的 agent）
max_tokens = 600        # 可选：本阶段单独的 token 上限（不写则用 run 级默认）
system = """
你是产品角色，把需求拆成清晰验收点。不要写代码。
"""
```
> 运行时 `--executor stage=...` / `--max-tokens` 仍可临时覆盖文件里的设置。

**最低接入成本**：`carrier` 直接写网关**已知的后端名**（`deepseek` / `glm` / `minimax` / `claude-code-sub` / `codex-sub`）→ **零额外网关配置、免重启**。想要"换厂商在控制台点一下"的体验，再用 `loom-*` 语义载体（见下）。查找顺序：`$LOOM_ROLES` → 当前目录 → 项目根 → `devkit/` → `~/.loom/`。

**也能在控制台在线编辑**：http://localhost:8899 「角色 → 载体 → 后端」下方的 **✎ 编辑角色流水线** —— 改字段、加/删阶段、上下移、保存。控制台保存到 `devkit/loom.roles.toml`，**和 `devkit run`/`devkit roles` 用同一份**（CLI 与 UI 不漂移）；「恢复默认」删掉该文件即回到内置默认。

---

## 配置（按需，可跳过）

- **只想用 API 三家（GLM / DeepSeek / MiniMax）**：把 key 填进 `.env` 的三行，然后 `docker compose restart litellm`。
- **想用订阅（Claude / ChatGPT）**：在宿主机跑一次浏览器登录（只需一次）：
  ```bash
  ./cli-proxy-api --claude-login     # 登录你的 Claude
  ./cli-proxy-api --codex-login      # 登录你的 ChatGPT
  ./cli-proxy-api --config ./config.yaml   # 启动，监听 :8317
  ```
- **换某个角色的厂商**：编辑 `litellm/config.full.yaml` 里对应的 `loom-*` 那一段（改 `model` 即可），然后 `docker compose restart litellm`。**不用改任何流程代码。**

> ⚠️ 把 Claude / ChatGPT 订阅当 API 用可能违反其服务条款、有限流/封号风险，请自行权衡。

---

## 开发者用法（CLI）

控制台只是壳，核心是命令行套件 `devkit`，纯标准库、零依赖：

```bash
cd agent-platform
python3 -m devkit "实现一个小功能，要求有测试和审查"          # 全 5 阶段
python3 -m devkit "..." --stages brainstorm,plan,implement  # 只跑某几阶段
python3 -m devkit "..." --carrier review=codex-sub          # 临时给某阶段换载体
python3 -m devkit "..." --executor implement=hermes         # 某阶段交给 hermes agent 跑（沙箱内）
python3 -m devkit "..." --golden cases.json                 # Eval Gate：golden 质量回归，不过即 NO-GO
python3 -m devkit "..." --executor implement=hermes --sandbox  # OS 级沙箱跑 agent（写盘受限）
python3 -m devkit "..." --budget 0.05                       # 软预算护栏：累计花费超 $0.05 即停剩余阶段并 NO-GO
python3 -m devkit "..." --no-compact                        # 关上下文压缩（默认开，用便宜模型把长上游产物压成要点）
```

**迭代循环（借鉴 Anthropic「Planner→Generator→Evaluator」长跑 harness）**：默认是一次性流水线；加 `--iterate N` 就变成**循环**——评判（测试/Eval/审查）NO-GO 时，把失败明细（含 Golden 的 `want=` 期望值 + 审查 critique）**回灌给构建者**重做，重测重评，最多 N 轮直到通过：
```bash
python3 -m devkit "实现 X，给 x.py" --golden cases.json --iterate 3
```
收尾会报**收敛/未收敛 + 迭代花费**（呼应文章的"单位接受变更成本"——低 accept rate = 空烧）。控制台发起框也有「迭代N」输入。已通过则 0 轮、不浪费。

**Sprint Contract `--contract N`**：implement 之前，**评判者（独立载体）先约定 ~N 条可机器验证的验收点**（golden 格式），既注入构建者（照此实现接口/文件名），又作为 Eval Gate + 迭代反馈。无需手写 golden：
```bash
python3 -m devkit "实现 X" --contract 6 --iterate 3        # 先立约 → 实现 → 自动验收 → 不过就迭代
```
golden 现支持 `"raises":"ValueError"`（非法输入应抛异常，非 happy-path 可表达且可满足）。加 `--contract-rounds N` 让**构建者再 N 轮收紧/修正**评判者拟的验收点（带反削弱地板：最终条数与 raises 用例数不得少于原合同，评判者有最终否决权）。

**Initializer + 单特性增量**（长跑增量构建一个项目）：
```bash
devkit backlog "一个命令行待办清单库"      # Initializer：拆成特性清单（backlog.json + progress.md + 项目目录）
devkit feature --count 3                   # 逐个增量构建：读当前代码库 → 实现下一个特性 → 跑测试 → 绿了才标 done
devkit backlog --status                    # 看进度 done/total
```
状态全在项目目录（backlog.json + progress.md + 代码），下次接着干；测试绿才算数（强制增量/测试/干净交接）。

**真机 Web 验证**（golden 的 `web` 用例，纯标准库、无浏览器依赖）：启动应用 → 真实 HTTP 请求 → 校验 status/body。
```json
{"name":"主页200","web":{"start":["python","app.py"],"port":8000,"path":"/","status":200,"expect_contains":"<html"}}
```
装了 playwright 才跑的真浏览器用例：`{"playwright":"check.py","web":{...start...}}`（脚本用 `LOOM_BASE_URL` 拿地址；没装则**跳过**不拉低 gate）。

**两个子命令（与控制台同源，CLI 也能用）**：
```bash
# @ask-model：临时问一个/多个载体一句（多个则并行对比），不开整条 loop
python3 -m devkit ask "用一句话讲依赖倒置" --models deepseek,glm,loom-reviewer
./loom ask "..." --models deepseek,glm        # loom 快捷方式

# diff：对比两次运行的 build/ 产物（默认自动对上一次带 build 的运行）
python3 -m devkit diff 20260623-1730 [--against 20260623-1700]
./loom diff 20260623-1730                      # loom 快捷方式
```

**额度薅羊毛 🐑 + 模型评分 ⭐**（控制台也有同名面板）：
```bash
devkit quota         # 各后端：已用$ / 免费额度 / 剩余，建议优先薅哪个（订阅/免费优先）
devkit scores        # 模型评分：实际使用(成功率/延迟/成本) + 你的👍/👎 + 官网评测 → 综合
devkit rate deepseek up --note "够用又便宜"   # 记一次实际体验，计入综合分
./loom quota | ./loom scores | ./loom rate <后端> up|down
```
- **额度薅羊毛**：`cp loom.quota.example.toml loom.quota.toml` 声明每个后端的免费额度/是否订阅；Loom 用**真实花费**（经本网关）算剩余、按"薅价值"排序（订阅 > 剩余免费额度多 > 付费）。*只统计经本网关花掉的部分，面向未来薅，不是账单级精确值。*
- **模型评分**：实际使用分全来自 Loom 真跑的日志；官网评测分**不杜撰**——`cp loom.scores.example.toml loom.scores.toml` 自己按各家榜单填（0-100，标来源）。综合 = 实际 0.5 + 用户 0.2 + 官网 0.3（按存在项归一化）。

**上下文压缩（compact 指针）**：默认开启——某阶段产物过长时，先用便宜模型（默认 `deepseek`，`--compact-model` 可换）压成要点再喂下游，省 token、保关键信号（结论/接口/约束/风险），替代粗暴截断。每件产物只压一次，压缩成本计入总账。

**软预算护栏**：`--budget $X`（控制台发起框也有「预算$」输入），本次运行累计花费超过该美元数就停掉剩余阶段、按 NO-GO 收尾——给自动化跑批一道成本红线。

**执行器（可同时嵌入多种 agent harness）**：每个阶段可选 `chat`（默认，扁平对话）/ `hermes`（Nous Hermes agent，带工具）/ `openclaw`（开源同类）。可在一次运行里同时用：`--executor implement=hermes --executor review=openclaw`。agentic 执行器在 `runs/<ts>/sandbox-<stage>/` 隔离目录里跑。`./loom doctor` 看哪些执行器可用。

**dev-as-agent 闭环（出可运行+已测的改动）**：implement 产出会被**物化成文件 → 沙箱里跑 unittest**，测试失败即 NO-GO；加 `--apply DIR` 则**测试绿时**把产物复制到 DIR（**apply 是人类门，默认不做**）：
```bash
python3 -m devkit "实现 X，给 x.py + test_x.py" --executor implement=hermes --apply ./out
```
支持**多文件/子目录**（`src/x.py`、`tests/test_x.py`）与 `requirements.txt`（尽力安装）。测试运行器优先 **pytest**（兼容两种风格），缺则按需装。也可在**控制台运行详情**里看 `build/` 产物 + 测试结论，**测试绿**时点「⬇ 一键 apply」→ 落到 `devkit/applied/<ts>/`。

**apply 进 git 仓库**（人类门，测试绿才做，**不 push**，用 worktree 不动你当前分支）：
```bash
python3 -m devkit "..." --executor implement=hermes --apply-git /path/to/repo --branch loom/feat-x
```

**跨运行学习记忆**：每次运行的任务/Gate/审查要点会记到 `devkit/MEMORY.md`，并自动作为「过往教训」注入下次 brainstorm/plan（控制台侧栏「🧠 学习记忆」可看）——流程越跑越聪明。

产物落在 `devkit/runs/<时间戳>/`：`00-task` + 各阶段 `.md` + `run-log.md`；总台账在 `devkit/RUNS.md`。

**加一个新角色**：在 `app/roles.py` 加一行 + 在 `litellm/config.full.yaml` 加一个同名 `loom-*` 载体，重建即可。

**跑测试**：`./loom test`（纯标准库、无需 live 网关，CI 友好）—— 跑台账合同测试 + 新能力单测（压缩回退 / 并行咨询聚合 / diff 状态）。

---

## 启停 / 排错

| 情况 | 处理 |
|---|---|
| 停止（保留数据） | `docker compose down` |
| 重启电脑后 | 先 `colima start`（若用 colima），再 `docker compose up -d` |
| 控制台打不开 | 看 `docker compose ps`，`docker compose logs console` |
| 测试/审查 401 | 对应 API key 或订阅代理不可用；填 `.env` 后 `docker compose restart litellm`，或运行 `./loom login` 刷新订阅 |
| 端口被占（3000/4000/8899…） | 改 `docker-compose.yml` 里的端口映射 |

---

## 轻量说明

- **默认即轻量**：`docker compose up -d` 只起 lite 核心（控制台 + 网关 + 库 + 订阅代理），**≈730 MiB**。发起运行 / 看产物 / 看用量全都在；聊天 UI 与编排服务收进 `--profile full`（**≈980 MiB**）。
- 控制台本体极轻（纯 Python 标准库，无三方依赖，~17 MiB）。
- 订阅代理 cliproxy 已容器化（`restart: always`），不会再「掉线」。
- 全部本地运行，数据不出本机。详细优化路线见 [ROADMAP.md](ROADMAP.md)。

---

## 链接速查

| 入口 | 地址 |
|---|---|
| 全局控制台（中枢） | http://localhost:8899 |
| 聊天界面 Agent UI | http://localhost:3000 |
| 网关看板 LiteLLM | http://localhost:4000/ui |
| AgentOS API 文档 | http://localhost:8000/docs |
