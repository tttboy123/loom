# Loom 长时自治 Agent — 落地计划 + 工作清单 + 派发提示词

> 目标：让 Agent 自己迭代项目（长时自治）。两层、共用一个引擎。
> 本文 = 设计 + 工作清单 + 第一批可直接派发的 spec/golden（Agent Team 执行用）。

## 0. 两层自治（务必分清）
- **Layer A：开发 Loom 自己时的自治**（放进你的开发 Loop）。目标=Loom 仓库；backlog=Loom 路线图；把关人=你；独有风险=自我修改（改坏约束自己的 harness）。
- **Layer B：Loom 被使用时的自治**（产品能力 `devkit auto "<愿景>"`）。目标=用户项目沙箱；把关人=终端用户；自我修改风险=0。
- **同一个引擎，换目标 + 换护栏。先造引擎 → 在 B 练（零自改风险）→ 指向 A（开自改护栏）。**

## 1. 核心架构原则：模块/接线分离
每个任务拆成两半，**严格分工**：
- **(a) 纯核心模块**（greenfield、单文件、可 golden 测）→ **便宜模型建**（spec+golden 派发）。
- **(b) 热路径接线**（接进 rdloop / apply 门 / 驱动器）→ **Claude 自己做**（有隐藏不变量，便宜模型搞不定）。

> 教训：reasoning 模型派发必须 `--max-tokens 8000`；并发各带唯一 `--run-id`；"往已有文件追加"便宜模型必失败 → 核心模块都设计成**新独立文件**。

## 2. 完整工作清单

| ID | 任务 | (a)便宜模型建的纯模块 | (b)Claude 接线 | 工作量 | 依赖 |
|---|---|---|---|---|---|
| **T2** | 物理证据门（默认失败契约） | `evidence.py`：gate(record)→GO/NO-GO | 接进 rdloop verdict | M | — |
| **T6** | Test Ratchet（测试只增不减） | `ratchet.py`：is_weakened(old,new) | 接进 apply-git 门 | S | — |
| **T4** | AGENT_STOP / 死循环检测 | `stopcheck.py`：should_stop(sigs) | 接进 iterate/driver | S | — |
| **T7** | 锁 4 类文件走人类 apply | `applylock.py`：requires_human(path) | 接进 apply 通道 | S | — |
| **T1** | 自治驱动 loop `devkit auto` | `autoloop.py`：pick_next/状态推进（纯逻辑） | 接 feature/task + 主循环 | M | T2,T4 |
| **T3** | Resume/Checkpoint | `resume.py`：done_stages(run_dir 内容) | 接 run_loop 重入 | S-M | — |
| **T5** | STEER 转向口 | （太小，Claude 直接做） | 循环间隙读文件注入 | S | T1 |
| **T8** | 热路径自动升级 Opus 审查 | （用 applylock 判定 + 路由） | Claude | S | T7 |
| **T9** | Loom 自我发现（内部信号） | `discover.py`：从 fitness/learn 找缺口候选 | 接 backlog 生成 | M | T1 |
| **T11** | 价值评估器（必须引真信号） | `valuer.py`：score(候选,证据)→分+理由 | — | M | T9 |
| **T12** | 人类优先级门 | （CLI 交互） | Claude | S | T11 |
| **T10** | Community Radar（外部信号） | hermes 工具调研 | Claude/hermes | L | — |
| **T13** | Layer B 产品模式 | （T1 换目标=沙箱+用户门） | Claude | M | T1,T7 |
| **T15/16** | 盲审 Evaluator + 物理验证 | review 干净上下文 + Playwright | Claude | S/M | — |
| **T0** | `git init`（前提：worktree/apply-git/回滚/checkpoint 都靠它） | — | Claude（一次性） | S | — |
| **T3b** | 多 worktree 隔离执行（并发不撞文件） | `worktree.py`：为任务开/清 worktree 的纯路径逻辑 | 接 auto 驱动 + apply | M | T0 |
| **T4b** | 全局预算 + 熔断 + 恢复（无人值守多小时） | `guard.py`：全局累计成本/失败率→continue/stop | 接 auto 主循环 | M | T4 |

**最短关键路径到"Loom 自迭代"**：`T2 + T6 + T4 + T7`（4 个纯模块并发建）→ Claude 接线 → `T1` 驱动 → `T3` 续命 → 开 Layer A 护栏。
**并发/无人值守扩展**：`T0`(git init) → `T3b`(worktree 隔离) + `T4b`(全局预算/熔断/恢复)。

## 2.5 并发 + 无人值守层（源自 Kimi 13h / GLM 长时 Agent 的评估）
Kimi/GLM 是**云端多机**系统；Loom 是**本地单机**。取模式、用单机右尺寸实现，别照搬分布式基建（避免 Agyn/K8s 过度工程）。

**✅ 现在做（单机就需要、成本低、解真问题）：**
- **T3b 多 worktree 隔离**：每个并发任务在独立 git worktree 里建+测+apply，物理上不可能撞文件——比 `--run-id` 打补丁干净。（前提 T0 `git init`。）
- **T4b 全局预算/熔断/恢复**：所有并发任务累计预算上限 + 总失败率/成本突刺熔断 + 崩了从 checkpoint 续（T3 resume 升到全局作用域）。

**🟡 暂缓（云端多机才回本，先用单机右尺寸版顶 80%）：**
- lease/TTL 并发领取 → 单机版 = `backlog.json` 条目加 `claimed`+时间戳，超时未 done 自动释放（穷人版 lease）。
- worker registry/heartbeat → 单机版 = PID 存活检查。
- REQUEST-CHANGES→repair task 跨任务 → 有队列才划算；现在用环内 `--iterate` 回灌。
> 触发升级到真·分布式的条件：并发 worker 多到单机扛不住 / 需要多机。在那之前，提前做 = 烧钱建用不上的基建。

**⚠️ 作用域升级（多项目共享账号）→ 见 `DESIGN-SCHEDULER.md`**：当**同机多个项目 + Loom 自举共享同一批模型账号**时，并发/队列的作用域从"单项目"提升到"**整台开发机、跨项目**"，且引入**优先级编排**（项目优先级 + 任务优先级 + 防饿死，借 K8s 调度器心智但单机实现）。分两层：**Tier 1 每厂商并发闸（hold-not-reject，先行止血）** → **Tier 2 中央 broker + 优先级调度器（`~/.loom/queue.db`）**。**具体实现由 Agent Team 按 DESIGN-SCHEDULER.md §6 的开放问题深度讨论后再定。**

## 3. 第一批派发（4 个安全核心纯模块，并发，零依赖）

> 这一批是自治引擎的"安全底座"。都是 greenfield 纯函数，便宜模型最稳。
> spec/golden 已写到 `devkit/auto-*.task.md` / `devkit/auto-*.golden.json`。

派发命令（Agent Team 执行；各唯一 run-id，reasoning 模型 8000 token）：
```bash
cd agent-platform
# T2 物理证据门
python3 -m devkit --task-file devkit/auto-evidence.task.md --stages implement \
  --golden devkit/auto-evidence.golden.json --cascade glm,minimax --run-id auto-evidence --max-tokens 8000 --no-cache
# T6 Test Ratchet
python3 -m devkit --task-file devkit/auto-ratchet.task.md --stages implement \
  --golden devkit/auto-ratchet.golden.json --cascade glm,minimax --run-id auto-ratchet --max-tokens 8000 --no-cache
# T4 死循环检测
python3 -m devkit --task-file devkit/auto-stopcheck.task.md --stages implement \
  --golden devkit/auto-stopcheck.golden.json --cascade glm,minimax --run-id auto-stopcheck --max-tokens 8000 --no-cache
# T7 文件锁分类
python3 -m devkit --task-file devkit/auto-applylock.task.md --stages implement \
  --golden devkit/auto-applylock.golden.json --cascade glm,minimax --run-id auto-applylock --max-tokens 8000 --no-cache
```
每个跑完：读 `devkit/runs/<run-id>/build/<模块>.py`，Claude 审查（抓 spec 违规/多余测试文件）→ 应用到 `devkit/` → 给 `test_features.py` 加单测 → 全套绿。**不要 `--apply`。**

## 4. Claude 接线清单（第一批模块建好后，Claude 自己做的热路径）
1. `evidence.py` → 接进 `rdloop.py` 的 verdict 计算：build/test 后，verdict 默认 NO-GO，只有 `evidence.gate(record)` 返回 GO 才 GO。
2. `ratchet.py` → 接进 `--apply-git` 门：apply 前对比新旧 golden，`is_weakened` 为真则拒绝 apply。
3. `stopcheck.py` → 接进 `--iterate` 循环 + T1 驱动：每轮记 error 签名，`should_stop` 为真则建 `AGENT_STOP` + commit + 挂起。
4. `applylock.py` → 接进自动 apply 通道：`requires_human(path)` 为真的文件不自动 apply，挂人类门 + 升级 Opus 审查。

## 5. Layer A 自我修改护栏（指向 Loom 自己时必开）
- Test Ratchet（T6）对 `test_features.py` + 所有 `*.golden.json` 生效。
- applylock（T7）把 `rdloop.py / evals.py / autoloop.py / evidence.py / ratchet.py / stopcheck.py / applylock.py / test_*.py / *.golden.json` 列为 human-only。
- 这两条 = 自治体不能拆自己的刹车。
