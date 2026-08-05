# Loom 能力矩阵

本矩阵记录当前已验收的产品能力。状态词沿用仓库约定：

- `CURRENT` — 已验收、可用的产品行为；
- `PARTIAL` — 有界实现但未达完整形态；
- `EXPERIMENTAL` — 仅评估/研究，不构成执行权威。

权威归属列给出验收 WorkItem 与原子提交；验证列给出矩阵/旅程证据；客户端
表面列说明用户可从哪里触达该能力。

## 1. 基础域（Phase 1 / Slice 1，提交 `5861f82`）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| Agent Mode Router（对话 vs Agent 触发） | S1-W1 | CURRENT | 单元 + 全仓矩阵 | CLI `loom route` |
| SQLite Event Journal：append-only、迁移、事件不可变 | S1-W2 | CURRENT | 焦点/race/仓库检查 | daemon 内部 + `loom status` |
| Evidence Artifact Store：SHA-256、原子发布、权限 0700/0600 | S1-W3 | CURRENT | 焦点 + 严格 Reviewer | 投影/Artifact 引用 |
| 可重建投影 + GlobalReadView（Journal 唯一权威） | S1-W4 | CURRENT | 重放/幂等/失败保留 | TUI/原生 app/CLI |
| 只读 CLI：`loom status`、`loom timeline` | S1-W5 | CURRENT | 确定性 JSON 校验 | CLI |

## 2. 域模型与团队（Slice 2，S2-W1…S2-W38 逐个原子提交）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| AgentDefinition / Runtime 目录（稳定 ID、能力/模型/权限） | S2-W1/W2 | CURRENT | RED-first + 焦点矩阵 | TUI Team Builder、原生 app |
| Team Draft 与未决问题（每次一个、目录重校验） | S2-W3…W11 | CURRENT | 焦点/50-run race/仓库矩阵 | TUI Team Builder |
| 保存团队实例（Main 种子 + SubAgent 休眠记录） | S2-W12/W13/W15/W16 | CURRENT | 焦点/race/仓库矩阵 | TUI Team Builder、原生 app |
| Journal 原子批追加（`AppendBatchIfStreamHeads`） | S2-W14 | CURRENT | 事务/冲突/重放测试 | daemon 内部 |

## 3. 运行时发现（S2-W17…S2-W31）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| Pi 元数据探测核心（版本/模型/能力，受控输入输出） | S2-W17 | CURRENT | 严格解析/不可变映射测试 | daemon 观察周期 |
| 隔离本地 Pi 进程 runner（固定环境、超时、清理） | S2-W18 | CURRENT | 真实 child-process fixture | daemon 内部 |
| 配置化固定名定位器（不搜索 PATH） | S2-W19 | CURRENT | no-PATH/no-process 测试 | daemon 内部 |
| 确定性发现 + canonical Runtime 发现/状态 writer/projection | S2-W20…W31 | CURRENT | 真实 SQLite 发现→重发现→状态链 | 原生 app Runtime & Providers |

## 4. 观察周期（S2-W32…S2-W38）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| 投影感知的单次发现/状态观察周期（write priority、同投影前后刷新） | S2-W32…W38 | CURRENT | 直接 none/discovery/status 证明 | daemon 常驻周期 |

## 5. 本地产品体验（Phase 2A，`7a27149b`，PX-01…PX-18 全 DONE）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| 原生 app 外壳 + 真实 daemon 私有 socket（免终端配置） | P2A-W1 | CURRENT | 安装/构建 fixture + root006 旅程 | 原生 app |
| 只读产品投影：Board/Teams/Runs/Compare/Evidence/Attention/Timeline | P2A-W1 | CURRENT | 严格 IPC + 原生/TUI 视图 | TUI、原生 app |
| Mission/Team 决策路径 + 时间线分页绑定 | P2A-W2/W3 | CURRENT | cursor 重连/gap/重复/代际围栏 | 原生 app、TUI |
| 一次一问题的 Team Builder（编辑/保存/确认，Candidate 边界） | P2A-W2 | CURRENT | 决策生命周期 RED + 旅程 | 原生 app、TUI |
| Credential Broker + Keychain（configure/replace/revoke、不泄露） | P2A-W2 | CURRENT | broker/Keychain 测试 + 消费证据 | TUI `g`、原生 app |
| 受控执行：preflight + Start + 有界恢复 + 单一 succeeded 终端 | P2A-W3 | CURRENT | Pi-006 真实旅程 + 权威验收 | 原生 app、TUI |
| 安装/更新/回滚/原子性/私有模式 | P2A-W1 | CURRENT | dry-run/install/rollback fixture | `scripts/install-*` |
| 本地模型目录绑定（离线 llama-server + GGUF，127.0.0.1，digest 绑定） | P2A-W2 `71101c26`+`8c0338ca` | CURRENT | isolated catalog RED + 真实解析器/漂移测试 | canary、材料化、daemon 观察 |

## 6. 治理侧任务交接（Phase 2B，`6d380233`）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| 显式确认型 side-task handoff：每次 policy reference 返回零写 `capability_gap` | P2B-W1 | CURRENT | 确定性离线 canary（296 Events 零重复）+ IPC→Swift 旅程 | TUI Mission 屏、原生 app |

## 7. 演化资产与材料化（Phase 3A，`7d5f0b01`）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| Versioned Evolution Assets：Skill/Team/WorkPackage/Recovery 模板、精确修订血统 | P3A-W1 | CURRENT | 8 场景跨客户端旅程全 PASS | TUI Evolution Assets 屏、原生 app Library |
| Run 绑定材料化（私有 skill 目录、原子发布、fail-closed 清理） | P3A-W1 | CURRENT | 材料化一致性 + 冲突/清理测试 | daemon 执行路径 |
| journey_id 贯穿 client/IPC/daemon/Journal/投影/证据（仅关联，非权威） | P3A-W1 | CURRENT | 跨客户端证据模式 | 所有客户端 |

## 8. Agent Scheduling Framework（v0.4.0，`7e24ec29`）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| Queue 状态/投影：admission/eligibility、DAG 环、重复活跃、冲突仲裁 | SF-W1 `1abf033c` | CURRENT | 双 Scheduler CAS 单赢家 RED + 旅程 | TUI Queue 屏、原生 app |
| 短暂 Worker Pool：一次一 claim、lease/generation fencing、崩溃回收 | SF-W2 `36b6c4b8` | CURRENT | before/after-CAS 崩溃回收 + 重启重建 | TUI Worker Pools 屏 |
| 有界恢复：七类失败路由、repair aging/fairness、无隐藏无限重试 | SF-W2 | CURRENT | 失败分类/饥饿 RED + 旅程 | TUI、Swift probe |
| 单写 Integrator：版本化 Release、later-Run adoption、rollback | SF-W3 `8e888565`+`ad9726a5` | CURRENT | 竞争集成 CAS 单赢家 + 旅程 | TUI Integration 屏 |
| 受控 self-host canary：一次性、幂等 CAS、不超卖、旧视图保留 | SF-W3 | CURRENT | canary 双跑拒绝 + 投影故障保留 + 修复 | TUI/Swift probe、W3 runbook |
| Timeline/Attention 投影：授权 + 代际绑定的节点流式输出 | SF-W3 | CURRENT | 未授权/陈旧/畸形帧零发布 | TUI Attention/Timeline 屏 |

## 9. 产品主线闭环（v0.4.1，`073ed862`）

| 能力 | 权威归属 | 状态 | 验证 | 客户端表面 |
|---|---|---|---|---|
| Queue 屏真实操作：`n` 入队 Job（生产 `queue_command create_job`） | W1 `b607957f` | CURRENT | v0.4.1-W1 旅程 + verify 脚本 | TUI Queue 屏 |
| Workers 屏真实操作：`c` claim、`t` 测试成功、`v` 评审 PASS | W1 | CURRENT | 同上 | TUI Worker Pools 屏 |
| Integration 屏真实操作：`i` 集成、`a` 采纳、`b` 回滚 | W1 | CURRENT | 同上 | TUI Integration 屏 |
| integration snapshot 空集合规范化（Swift 精确解码） | W1 | CURRENT | 双客户端读回 + Swift 96+4 | 原生 app/Swift probe |
| 框架驱动的两个真实并行开发 Candidate（最低验收第 1 条最强形态） | W2 `27ef27b5` | CURRENT | v0.4.1-W2 旅程（并行 claim/七类路由/单写） | TUI + Swift probe |
| 可重复受控 self-host canary runbook/脚本 | W3 `1fe5f8e8` | CURRENT | v0.4.1-W3 旅程（幂等/旧视图/崩溃恢复） | Swift probe、runbook |

## 状态汇总

```text
V0.4.1 = ACCEPTED / WHOLE-SLICE REVIEW PASS（12/12）
SLICE 1/2 = ACCEPTED   PHASE 2A = ACCEPTED（PX-01…18 DONE）
PHASE 2B = ACCEPTED    PHASE 3A = ACCEPTED（8/8 旅程）
V0.4.0 = ACCEPTED（SF-W1/W2/W3）  V0.4.1 = ACCEPTED（W1/W2/W3）
EXCLUDED = Phase 3B 路由 / Web / 多用户 / 自动批准 / push-merge 远端
```
