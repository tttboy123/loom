# Loom 对标参考：cc-switch 与 Kun Agent

Date: `2026-08-04`

Status: `REFERENCE — NON-EXECUTIONAL ROADMAP INPUT`

来源：侧会话讨论整理（避免会话丢失的持久化笔记）。本文件不是合同、
Amendment、ADR 或执行授权；它只记录产品/架构对标结论，供主线程在
Phase 3A 完成后的安全检查点按治理顺序消费（作为 queued roadmap input）。

结论先行：

- **cc-switch** 是「工具配置层的 profile 管理器」，与 Loom 不同属；只借鉴
  「Provider/Model Profile 切换器」这一个 UX 概念，机制（改写外部工具配置、
  请求层热切换、持有 Provider key）一律不借鉴。
- **Kun Agent** 与 Loom 同属：本地运行时 + GUI/TUI 对等客户端 + DAG 编排 +
  治理/资产生命周期。Kun 应替代 cc-switch 成为 Loom 下一阶段的主要对标物，
  并补进 community survey（当前仓库调研里没有它）。

---

## 1. cc-switch 演进分析

### 1.1 项目定位

开源跨平台桌面应用，用于管理和一键切换 Claude Code / Codex / Gemini CLI /
OpenCode / OpenClaw / Grok Build / Hermes Agent 等工具的 API 供应商配置。
核心体验：填一个 API Key，点一下切换。本质是「配置中台」：把散落在各工具
配置目录里的供应商数据收拢成统一管理，切换时写回各工具的 live 配置。

### 1.2 版本演进时间线

| 版本/时间 | 关键变化 |
|---|---|
| 2024-08 v1.0 | 只有 Claude Code 基础供应商切换，Electron；由 Jason 开发 |
| v2.x | 仍是单工具切换器，增加预设（如 v2.0.3 DeepSeek v3.1） |
| v3.0.1 | Electron → Tauri 2.0 重构（体积 80MB → 6-7MB，启动零点几秒） |
| v3.1.0 | 加入 Codex 支持，管理 `~/.codex/auth.json` + `config.toml` |
| v3.2.0 | 架构转折：SSOT（单一事实源）、一次性迁移、归档、系统托盘、内置更新器、原子写入与回滚 |
| v3.3–v3.5 | 通用配置片段、VS Code 同步、WSL、MCP 管理（v3.5.0）、导入导出、备份轮转、延迟测试 |
| v3.14.0 | Hermes Agent 支持、Gemini 原生 API 代理、用量/会话/Skills/窗口控制 |
| v3.15.0 | Claude Desktop 一等管理对象、反向代理、角色模型映射、Codex OAuth 模型发现、用量看板 |
| v3.16.0 | Codex 第三方 Chat Completions 路由（本地代理协议转换）、模型映射表、reasoning 适配、统一 custom provider 桶 + 历史会话迁移 |
| v3.16.1 | Codex 官方 OAuth 保留（默认关闭开关）、按 app 串行切换锁、接管状态判定、模型目录修复 |
| v3.18.0 | Grok Build 成为第 8 个受管应用；代理接管 + 路由/failover/计费；用量看板 |
| v3.19.0 | 安全加固（zip-slip、深链导入确认、SQL 备份导入 authorizer、通用配置合并、终端转义）；代理媒体回传修复（图片 token 膨胀）；models.dev 自动定价同步 |
| v3.19.1 | DeepSeek/火山方舟/混元直连原生 Responses；官方厂商模型目录镜像；切回官方 Codex 401 修复；首个删除量大于新增量的版本（DB schema v16 不变） |

### 1.3 配置策略演化（反复后稳定）

1. 全量替换：切换时直接覆盖整个配置文件 → 覆盖用户自定义配置。
2. 引入「通用配置」片段缓解覆盖问题。
3. 重构为关键字段替换（只换 API Key 和 base URL）→ 无法传递自定义字段。
4. 最终回到「全量替换 + 通用配置」组合。

教训：配置类工具的核心矛盾是「不覆盖用户自定义」与「不丢失自定义字段」，
需要显式的通用片段/合并语义，而不是在两种替换策略间摇摆。

### 1.4 对 Loom 的借鉴/排除

可借鉴：

- **SSOT 转折（v3.2.0）**：供应商数据收进自身数据库，live 文件只是投影，
  读取时回填，配合原子写入/备份/并发锁。Loom 已天然具备（Journal 唯一权威、
  Projection 可重建），后续加 Provider profile/usage/session 时不得冒出第二套权威库。
- **OAuth 保留问题（v3.16.1）**：切回官方 Codex 时第三方 key 残留在
  auth.json 导致 401。Loom 的 P2A-W2 从一开始就规定 Codex 委托 OAuth、
  Loom 不碰 auth.json、凭据只在 Broker/Keychain——这就是该问题的正解。
- **协议适配清单**：Responses ↔ Chat Completions、工具调用、SSE 事件、
  usage 缺失、错误规范化——可作 Loom Runtime adapter（如 Pi bridge）验收清单。

不可借鉴：

- 本地代理作为产品核心（Loom 是编排+观测层，Provider 路由归 Runtime/adapter）。
- 请求级热切换（Loom 每个 Run 钉死 exact revision/runtime/generation；
  切换 = 新 Attempt，与「session/context ID 不是 checkpoint 权威」一致）。
- 写外部工具 live 配置（Loom 永不改写 Codex/Pi 的 config 来切供应商）。
- 持有 Provider key 的 profile 库（Loom profile 只存 Broker 引用/委托登录态）。

---

## 2. Kun Agent 对标分析

### 2.1 项目定位与架构

KunAgent/Kun：本地优先的 AI Agent 工作台（Code/Write/Design/Research/
Automation 多个工作区），许可为 PolyForm Noncommercial 1.0.0。

核心架构：

- 一个 `kun serve` 本地 HTTP/SSE 运行时；桌面 GUI、终端 TUI、手机客户端、
  后台任务全部连接它，共享线程、turn、审批、模型连接、用量、后台任务。
- 线程/turn/事件/审批/用量以 append-only JSONL + 原子索引持久化；
  cache-first agent loop（不可变 prompt 前缀、TTL/LRU、inflight、显式压缩）。
- 数据目录锁选举单一实例；`runtime.json` 记录 instance ID/PID/version/
  启动时间/loopback URL/log 路径；build-ID 校验实例发现，GUI/TUI 抢同一实例。
- GUI+TUI+runtime 同版本同 build ID、联合发布节奏：任一目标失败整版不 promote。
- 模型连接 registry：多账号、受保护凭据存储（API key/OAuth token/header
  不进普通配置）、OAuth 刷新；订阅/套餐/API/OpenAI-Anthropic 兼容/自托管。
- TUI 基于 `@earendil-works/pi-tui` inline 模式；`/connect`、`/model`、
  `/sessions`、`/usage`、`/update` 等命令与 GUI 共享同一连接 registry。
- 前身是 DeepSeek-GUI（数据目录 `~/.deepseekgui/kun`，v0.2.32 自动迁移到
  `~/.kun/data`，迁移前备份并保留兼容链接）——同样走了「单工具痛点 →
  多 Provider → 共享运行时+TUI → Graph 编排+治理」的演化线。

### 2.2 Agent Graph（v0.2.32，experimental）

- 按回合选择的编排模式（Direct | Graph），不是第二运行时。
- Lead 把需求编译成经验证的依赖 DAG；策略：auto / fanout_join / pipeline /
  bounded_loop / state_machine / hybrid；边类型：control / data / message。
- 子代理权限 = 父回合 ∩ Graph 策略 ∩ Agent profile ∩ 节点范围；不能递归
  建图、绕过 Lead 验收或扩权；只有 `report_to_parent` 一个上报通道。
- 每个可执行节点必须有源 Lead 的显式 review（pass/revise）；Lead pass 是
  有界数据包的交接；pass 不能覆盖 `validation.valid === false`。
- GraphPatch 用 baseRevision/expectedRevision/expectedSeq 做 CAS；stale 请求
  零副作用；accepted 历史不可变；reducer 拒绝源状态不匹配的事件。
- 状态机：GraphRun draft→validating→ready→running→completing→completed，
  failed/cancelled 为终态；节点 pending/blocked/ready/queued/running/
  submitted/reviewing/accepted + repair/failure/cancel/skip/supersession。
- 调度：依赖校验、失败传播、优先级/重试延迟、并发上限（全局/run/node）、
  封顶指数退避、run/node 墙钟（默认 7 天/24 小时，静默 15 分钟触发监督检查）、
  LoopGate 有界循环、消息/Artifact 字节上限、公平轮转。
- 取消先 fence 为终态，再 abort worker、丢弃迟到结果、settle attempt/node、
  释放 lease、安全处置 worktree、记录清理；重复取消幂等。
- 写节点声明归一化 repo 相对 scope；serialize/lease/Git worktree 策略防冲突；
  校验变更路径与不可变 lease 匹配；未知用户变更需人工处置；
  未验收/冲突/孤儿 worktree 保留。

### 2.3 治理与学习

- Project 身份：Git remote → Git common-dir → 工作区根。
- Profile：不可变版本，origin = builtin/user/ephemeral/learned；生命周期
  candidate → probation → trusted → dormant → archived → deleted。
- 路由：硬性 eligibility（生命周期/任务/风险/能力/工具/Skill/MCP/网络/
  sandbox/scope）+ 召回 + 多维度排序（task-fit/verified-quality/trust/
  freshness/efficiency/confidence/availability/load，权重 32/22/14/8/8/10/3/3）；
  漏选惩罚；dormant 带 rollback 元数据。
- terminal/checkpoint run 生成脱敏有界 Episode（不含 raw reasoning/凭据/
  密钥/完整源码/无界日志）；幂等沉淀要求跨会话的最小已验证 episode 数；
  可复用材料归类为 Agent/Skill/Graph Recipe 候选；evidence 是不可信数据。

### 2.4 Kun 与 Loom 对照表

| 维度 | Kun | Loom |
|---|---|---|
| 运行时 | `kun serve` HTTP/SSE + runtime.json 实例发现/数据目录锁/build-ID 校验 | `loomd` + Unix socket IPC + 产品 socket/lock 原子清理 + probe 身份 |
| 多客户端 | GUI/TUI 对等，关一个不影响另一个 | native App + TUI 共用同一 daemon/service（Cross-client Exit Gate） |
| 状态权威 | append-only JSONL + 原子索引；事件带 sequence/revision/idempotency | SQLite Event Journal 唯一权威 + stream-head CAS + generation fencing（更强） |
| 编排 | Lead 编译 DAG、ready-set scheduler、least-authority 子代理、Lead review 即交接 | Team DAG 执行（S3-W5）：逻辑节点/attempt/generation、单 CAS winner、Evidence 生命周期 |
| 治理/资产 | candidate→probation→trusted→dormant→archived；Skill/Graph Recipe 候选；Episodes 脱敏 | P3A-W1：draft/candidate/active/archived；accepted Run+Evidence promotion；redacted summary |
| 模型/Provider | 自带连接 registry + 受保护凭据库 + OAuth 刷新 | P2A-W2：Codex 委托 OAuth、MiniMax Broker/Keychain；Provider 是外部 Runtime |
| 观察性 | Agent 视角（请求/工具/用量/耗时，入库前脱敏，可关） | AuthorizedFrameObserver + 结构化日志 + 脱敏规则 |
| 变更集成 | 归一化 repo 相对 scope、lease、Git worktree、stale/dirty 检查 | worktree/lease/原子提交治理 |

### 2.5 可借鉴 / 不可照搬

可借鉴：

1. **Agent Graph 是「Agent Scheduling Framework / 并发开发流水线」的现成参考
   实现**：DAG 编译验证、ready-set 调度、least-authority assignment 快照、
   worker 只读上报、Lead review 作为交接、CAS patch、bounded loop、取消先
   fence 再释放 lease/清理 worktree、失败重复归一化后 pause/escalate。
   可与排队的 Decomposition Compiler/单 Integrator/lease fencing/失败分类
   逐条对账。
2. **runtime 实例发现元数据**：runtime.json（ID/PID/version/URL/log path）+
   数据目录锁 + build-ID 校验——补强 Loom 客户端重连时对错 daemon 的防御。
3. **模型连接向导形态**：搜索式目录、Custom provider 置顶、先探测 `/models`
   但不谎报成功（显式「带已填模型保存」）、凭据掩码、确认前不创建——
   即 Provider 连接 sheet 与 cc-switch 式 profile 切换器应长成的样子。
4. **治理生命周期粒度**：probation/trusted/dormant 两档 + 漏选惩罚 +
   dormant 带 rollback 元数据——可作 P3A 后续加固参考。
5. **联合发布节奏**：GUI+TUI+runtime 同版本同 build ID，任一失败不 promote——
   与 Loom「GUI/TUI 任一缺失只能 PARTIAL」的 Cross-client Exit Gate 同原则。

不可照搬：

1. Kun 自己拥有 agent loop 和模型客户端（DeepSeek-compatible client、直接
   管理 Provider）——Loom 的边界是 controller/observer：Pi/Codex/MiniMax 是
   外部 Runtime，Loom 只通过 adapter/bridge 编排，不持有 Provider 路由和 key。
2. 模型请求层自动重试（默认最多 5 次）+ 流中断恢复——Loom Phase 1 明确
   no-retry/hidden retry；即使未来放开，也只能是受控 policy + 新 Attempt/
   generation，不能照抄传输层自动重试。
3. JSONL 作为存储底座——并发写入弱于 Loom 的 SQLite CAS；不要为「参考」降级。
4. 代码本身：Kun 是 PolyForm Noncommercial（非 MIT），只能参考设计，不能抄代码。

---

## 3. 路线落位建议

1. **Kun 成为「Agent Scheduling Framework / 并发开发流水线」（下一大版本）
   的主要对标物**；cc-switch 只贡献「Provider/Model Profile 切换器」UX 概念。
2. **Provider/Model Profile 切换器**排 Phase 2，挂在 Runtime Capability Matrix
   下：用户显式选择、绑定发生于 Run/Attempt 开始前、落一条 Journal fact、
   GUI+TUI 共用真实 IPC；运行中切换 = 新 Attempt。
3. **Phase 3A 后续加固参考 Kun 治理层**：probation/trusted/dormant 渐进信任、
   漏选惩罚、Episode 脱敏沉淀阈值（注意只参考概念，不抄实现）。
4. **community survey 补入 Kun**：当前仓库调研无 Kun 记录；建议按现有
   survey 模板做一次完整对标（含许可、架构、Graph、治理、与 Loom 差异）。
5. 主线程消费时按现有队列治理：在 Phase 3A 完成的安全检查点，作为
   非执行性 roadmap input 落盘到 PRODUCT-PLAN/TECH-PLAN 或独立 amendment，
   不得静默扩项。

---

## 4. 来源

- cc-switch 主仓库：https://github.com/farion1231/cc-switch
- cc-switch Releases：https://github.com/farion1231/cc-switch/releases
- cc-switch CHANGELOG：https://github.com/farion1231/cc-switch/blob/main/CHANGELOG.md
- cc-switch 演进解读：https://gitcode.csdn.net/6a1eecb1662f9a54cb790920.html
- cc-switch 百科：https://baike.baidu.com/item/CC%20Switch/67930143
- Kun 主仓库：https://github.com/KunAgent/Kun
- Kun Runtime（kun serve）：https://github.com/KunAgent/Kun/blob/master/kun/README.md
- Kun Graph Mode：https://github.com/KunAgent/Kun/blob/master/docs/graph-mode.en.md
- Kun TUI：https://github.com/KunAgent/Kun/blob/master/docs/kun-tui.en.md
- Kun v0.2.32 Release：https://github.com/KunAgent/Kun/releases/tag/v0.2.32

---

## 5. 处置说明

本文档仅为侧会话持久化笔记，无任何权威性，不授权任何代码/配置/网络/发布
动作。主线程若采纳，需在 Phase 3A 完成后按队列治理流程转为正式计划输入。
