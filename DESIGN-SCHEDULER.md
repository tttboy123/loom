# 全局调度层设计（整机 · 跨项目 · 共享账号）

> 目的：多个项目（含 Loom 自举）在**同一台开发机**上并行自动化开发、**共享同一批模型账号**时，
> 如何编排稀缺的厂商并发，避免超订→超时→长重试→互踩→吞吐反降。
> **本文是设计简报（问题 / 原则 / 优先级模型 / 开放问题）——具体实现交给 Agent Team 深度讨论。**
> 概念上借 K8s 调度器心智；**实现上单机右尺寸（SQLite + 本地 daemon），绝不上 Redis/etcd/K8s。**

## 1. 问题
- 多项目 A/B/C + **Loom 自举** 同机并行，都经 Loom 调模型。
- 稀缺资源 = **每厂商并发**（MiniMax=2、GLM=k…），**按账号（同 key）算 → 全局共享**。
- 无编排 → 各项目各自开火 → 合计超账号上限 → 429/超时 → **长重试还占着并发 → 雪上加霜**。
- 根因：**稀缺资源是全局的，消费者却各自为政。**

## 2. 原则
1. **一个中央治理者独占"每厂商并发预算"**，谁都不许直接开火，先申请名额。
2. **超限 hold 排队，不 reject 重试**（消掉超时风暴）。
3. **溢出填闲置厂商**（多厂商总容量 > 任何单厂商）。
4. **优先级 + 公平调度**（见 §4）。
5. 总吞吐目标 = Σ(各厂商容量)，而非卡在 MiniMax 的 2。

## 3. 资源模型（借 K8s 概念）
| K8s | 本系统对应 |
|---|---|
| Node 的 CPU/内存 | **每厂商并发 slot**（MiniMax=2、GLM=k…）；可扩展到 per-vendor RPM/TPM |
| ResourceQuota | 每厂商月 token 预算 + 每项目用量上限（防单项目吃干） |
| Namespace | **项目**（A/B/C/Loom-self），各带 priority/weight + min-share |
| Pod / Job | **任务**（一次 Loom run / 一次派发），带 PriorityClass |
| Scheduler | **中央 broker 的调度器**（空位释放时按优先级+公平选下一个） |

## 4. 优先级模型（你的 K8s 直觉，两级 + 防饿死）
- **项目优先级**（namespace weight）：如 Buddys=high、Loom-self=medium、实验项目=low。
- **任务优先级**（PriorityClass）：如 人触发 > 修复(NO-GO repair) > 普通特性 > 后台自我发现。
- **有效优先级 = f(项目权重, 任务类)**：某厂商空出一个 slot 时，按有效优先级挑下一个待跑任务。
- **防饿死（关键，否则低优永远跑不上）**：
  - 每项目 **min-share 保底**（像 ResourceQuota 的 request，至少给一点）。
  - **aging**：任务等越久有效优先级越高（避免低优无限延后）。
- **抢占（preemption）**：LLM chat 调用是秒级，通常**队列插队即可，无需 kill 运行中**；
  但长 agentic 任务（codex/hermes 分钟级）可能需要"高优等待时不再接纳新低优"——留给 Agent Team 定。

## 5. 两层落地
### Tier 1 — 并发闸（先行，止血，无优先级）
- 每厂商一个信号量：`minimax=2, glm=k, deepseek=m`。
- **超限 hold 排队，不报错重试**；配合现有 fallback 链溢出到闲置厂商。
- 各项目照旧各跑各的 Loom，只是都指向这个"闸"。**先消掉超时+重试雪崩。**

### Tier 2 — 中央 broker + 优先级调度（结构性，优先级住这里）
- **全局任务队列**（`~/.loom/queue.db`，SQLite/WAL，跨进程锁）。
- 所有项目 + Loom 自举**投递任务**进队列。
- **调度器**：持每厂商信号量 + §4 优先级/公平/防饿死 + 溢出到闲置厂商。
- 超容量的任务**在队列里等**（不超时、不重试），有空位按有效优先级领。

## 6. 留给 Agent Team 深度讨论的开放问题
1. **调度粒度**：per model-call（细、对项目透明）vs per task/run（粗、易做优先级与公平）vs 混合（任务准入 + 每调用信号量）？
2. **优先级算法**：严格优先 + aging？加权公平队列(WFQ)？多资源（并发×token 预算）要不要 DRF(dominant resource fairness)？
3. **长 agentic 任务抢占**：要不要？只做"高优等待时不接纳新低优"够不够？
4. **治理者位置**：网关前置准入代理 / 任务 broker / 两者叠加？
5. **提交接口**：共享 SQLite 表（cooperative claim）vs 本地 HTTP daemon(`loomd`) vs unix socket？
6. **配置 schema**：项目/任务怎么声明优先级？（`loom.priority.toml`？PriorityClass 注册表？项目根一个 weight？）
7. **lease/claim 语义**：领任务标 `claimed`+时间戳，超时未 done 自动释放（穷人版 lease/TTL）——TTL 取多少？
8. **QoS 分级**：要不要 guaranteed/burstable/best-effort 三档项目？
9. **可观测**：队列深度、每厂商占用率、各项目等待时长、饿死告警。

## 7. 与 AUTONOMY-PLAN 的关系
- 这层把 AUTONOMY-PLAN 的 `T3b`/单机队列的**作用域从"单项目内部"提升到"整台开发机、跨项目"**。
- **Loom 自举 = broker 的一个普通客户**：Loom 开发自己的任务、A/B/C 的任务，全进同一队列、同一套优先级体系、同一并发闸。统一、公平、不超订。
- 不是新机制，是把并发/队列/优先级的**作用域放大到整机**。

## 8. 单机右尺寸红线（防 Agent Team 过度工程）
- ✅ SQLite（WAL）+ 一个本地 `loomd` 调度进程（或纯 cooperative-claim 无 daemon）。
- ❌ 不 Redis / etcd / 消息队列中间件 / K8s。**概念借 K8s 调度器，实现别抄 K8s。**
- 触发升级到真·分布式的条件：多机 / 并发规模单机扛不住——在那之前一律单机。
