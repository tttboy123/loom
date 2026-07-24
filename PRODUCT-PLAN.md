# Loom 产品方案

> 草稿 v0.2 · 2026-07-24
> Customer #0: lune
> 状态：产品边界已收敛，可进入 Phase 1 实现

## 1. 产品定义

Loom 是一个本地优先的 Agent 团队编排与协作平台。

它不重新实现 Codex、Claude Code、Pi、Hermes 等底层 Agent 的推理循环，而是在这些运行时之上提供：

- 显式 Agent 入口；
- Main Agent 与 SubAgent 团队编排；
- 团队、任务、成本和验收看板；
- 用户定义的执行规则与批准节点；
- 本地凭证代理；
- Observation、Cost、Governance 三个视图；
- 面向个人体验的共进化 Sidecar。

Loom 的默认入口仍是普通对话。普通问题或普通任务不会自动创建团队。

## 2. 要解决的问题

重度 Agent 用户和小团队通常面临四个问题：

1. **使用入口混乱**：普通对话、单 Agent 和团队执行混在一起，用户不知道一次输入会启动什么。
2. **团队配置成本高**：用户需要反复定义 Agent、模型、工具、职责和协作关系。
3. **执行过程黑盒**：任务拆分、分派、重试、审查、成本与失败原因缺少统一视图。
4. **经验无法复利**：有价值的 Agent、Skill、协作策略和个人偏好不能可靠地沉淀、评测和复用。

## 3. 两种使用模式

### 3.1 普通对话模式

用户直接输入问题或任务时，Loom 保持在普通对话模式：

- 不自动创建 Main Agent；
- 不自动生成团队；
- 不自动打开任务看板；
- 用户可以继续对话，或显式切换到 Agent 模式。

### 3.2 显式 Agent 模式

只有用户明确执行以下任一动作时，Loom 才进入 Agent 团队流程：

- 选择“使用 Agent”；
- 选择一个已存在的 Agent 或 Team；
- 把工作分派给 Agent 或 Team；
- 在普通对话中明确要求使用 Agent 团队执行。

进入 Agent 模式后，Loom 才解析团队：

```text
用户显式使用 Agent
→ 是否已明确定义团队？
  → 是：直接加载团队与任务看板
  → 否：Main Agent 生成 Team Draft
       → Loom 像 grill-me 一样一次询问一个关键选择
       → 实时更新团队、任务、模型、权限和预算草案
       → 用户确认
       → 创建 Agent 实例并开始执行
```

普通任务本身不是组队触发器；用户的显式 Agent 意图才是。

## 4. Agent 团队模型

### 4.1 Main Agent

每个 Agent 团队都有且只有一个 Main Agent。

Main Agent 的固定职责是：

- 理解目标并生成统一工作合同；
- 拆分任务和依赖；
- 选择或建议 SubAgent；
- 分派、重排和汇总工作；
- 根据证据决定重试、请求审查或请求用户决定。

Main Agent 不能退化为万能执行 Agent。它不直接承担具体交付物的修改和自我验收。

Main Agent 的模型、Provider 和提示词可以由用户替换，但这组协调职责不能缺失。

### 4.2 Agent 定义来源

Loom 按以下顺序解析 Agent：

1. **项目 Agent**：项目显式定义，优先级最高；
2. **复用 Agent**：用户个人库中可跨项目复用；
3. **临时 Agent**：当前任务缺少角色时，由 Main Agent 提议创建。

临时 Agent 只在当前团队中生效。任务结束后，Sidecar 可以建议把它保存为项目 Agent 或提升到复用 Agent，用户确认后才持久化。

### 4.3 角色与运行配置解耦

AgentDefinition 描述：

- 职责和边界；
- 输入与交付物；
- 可使用的工具类别；
- 质量要求；
- 协作关系。

RuntimeProfile 描述：

- Agent CLI 或执行适配器；
- Provider 与模型；
- 预算、超时和并发；
- 认证模式；
- fallback 策略。

同一个 AgentDefinition 可以绑定不同 RuntimeProfile。用户可以为项目或单次任务固定运行配置。

### 4.4 Team Draft

模型生成的团队在实例化前必须进入 Team Draft。草案同时包含：

- Main Agent 和 SubAgent；
- 每个角色的职责、模型建议和权限；
- 首轮任务卡、依赖和负责人；
- 预计成本、并发度和超时；
- 验收条件和需要用户批准的节点。

Loom 每次只询问一个会改变团队或执行合同的关键问题。用户最终一次确认后，团队实例和任务看板一起创建。

### 4.5 运行期团队变更

Main Agent 可以自动重排已有 Agent 的任务。

新增、替换或扩权 Agent 时，必须生成 Team Change 草案，说明原因、影响、成本和新增工作。用户确认后才能生效。停止、取消或降权可以立即执行。

## 5. 统一工作合同

Coding、研究、写作和其他知识工作共用同一编排内核。领域差异由 WorkPackage 提供，不创建不同的调度系统。

统一合同包含：

```text
目标
→ 交付物
→ 验收条件
→ 所需证据
→ 工具与数据边界
→ 权限、预算和批准规则
```

WorkPackage 提供领域相关的 Agent 模板、工具、验证器和默认规则。Main Agent 根据用户任务推荐 WorkPackage，用户可以调整。

## 6. 看板与完成规则

看板至少显示：

- 团队与 Agent 状态；
- 工作项、依赖和负责人；
- 当前执行、阻塞和审查；
- 成本、模型和运行时；
- 验收证据与批准请求。

执行 Agent 不能直接把工作项标记为 Done：

```text
Executor 提交 Ready for Review + Evidence
→ Loom 运行确定性验收
→ 根据任务风险和客户规则决定是否创建独立 Verifier
→ 验收通过
→ Loom 更新为 Done
```

低风险任务可以由 Main Agent 基于确定性证据验收；高风险或客户指定任务必须由独立 Verifier 检查。

## 7. 三个视图

三个视图来自同一执行记录，但回答不同问题：

| View | 回答 |
|---|---|
| Observation | 团队正在做什么，为什么这样拆分，发生了哪些变更和失败 |
| Cost | 每个 Agent、模型、工作项和重试花费多少 |
| Governance | 哪些客户规则被匹配，哪些动作继续、暂停或等待批准 |

## 8. 客户定义的执行规则

Loom 不替客户决定所有执行边界。客户可以按项目、团队、工作包或单次任务定义规则。

规则至少可以表达：

- 匹配条件；
- 适用对象；
- 执行效果；
- 批准人或批准方式；
- 超时与拒绝后的处理。

`require_approval` 是一等执行效果：匹配后，关联工作项进入等待批准状态，直到客户批准、拒绝或取消。客户也可以定义仅记录、告警、拒绝或自定义动作。

协议校验、无效凭证、消息重放和进程归属等属于运行时正确性，不作为可绕过的客户业务规则。

## 9. Credential Broker

Loom 的 AgentKey 不是 Provider 通用凭证。

在 Broker 模式下：

- 真实 Provider Key 只由本地 Credential Broker 持有；
- Agent 获得 Loom 本地地址和任务级临时凭证；
- Broker 校验任务、Provider、模型、额度和有效期；
- Broker 使用真实凭证代发请求并记录成本；
- Agent 不能从临时凭证推导真实 Key。

支持 Provider 原生短期凭证时，Loom 可以使用 Provider 官方机制。无法接入 Broker 的第三方 CLI 使用原生认证，并在 UI 中明确显示其凭证隔离等级。

## 10. 共进化 Sidecar

Sidecar 是独立于执行主路径的本地个人服务：

- 在任务结束或系统空闲时读取允许的执行轨迹；
- 沉淀可撤销的个人偏好和项目事实；
- 生成 Skill、AgentDefinition 和协作策略候选；
- 离线评测候选版本；
- 通过个人改进摘要请求用户激活。

Sidecar 不直接修改运行中的团队、权限或任务。个人记忆和轨迹默认本地、按用户与项目隔离并加密。同步和团队共享默认关闭。

## 11. 产品边界

### Loom 负责

- 显式 Agent 模式入口；
- Main Agent 和团队生命周期；
- Agent 定义解析和实例化；
- 任务图、分派、取消和恢复；
- 客户规则、批准和验收；
- Credential Broker 与成本计量；
- 看板和三个视图；
- 共进化候选管理。

### Loom 不负责

- 重新实现底层 Agent 推理循环；
- 训练基础模型；
- 强迫客户采用固定 Agent、模型或团队结构；
- 在用户未显式选择 Agent 时自动组队；
- 未经用户激活就修改长期 Agent、Skill 或协作策略；
- 在 v1.0 前提供强依赖云端的运行模式。

## 12. 可验证的成功指标

所有成本和质量结论都是待验证假设，不预设“90% 质量”或固定节省比例。

| 阶段 | 验收指标 |
|---|---|
| Phase 1 | 普通对话不触发组队；显式 Agent 入口完成两种团队解析路径；一个真实任务经过执行、验收和 Done |
| Month 1 | Customer #0 连续使用 10 个真实任务；团队草案确认率、返工率、成本和完成时间可查询 |
| Month 3 | 10 个外部用户各完成至少 5 个任务；能区分项目 Agent、复用 Agent 和临时 Agent |
| Month 6 | 至少两种 Agent Runtime、两种 Provider 认证模式和一个可替换路由后端通过兼容性验证 |

## 13. 路线图

### Phase 1：真实纵向切片

- 普通对话与显式 Agent 入口分流；
- Main Agent 固定职责；
- 已定义团队直接加载；
- 未定义团队生成 Team Draft 并逐项确认；
- AgentDefinition / RuntimeProfile 解耦；
- Main Agent + 最多两个 SubAgent；
- 任务看板和确定性验收；
- 客户规则触发 `require_approval`；
- 一个真实 Agent Runtime；
- SQLite 持久化和 CLI 视图。

### Phase 2：凭证与运行时

- Credential Broker；
- Broker、Provider 原生短期凭证和 CLI 原生认证三种模式；
- 第二种 Agent Runtime；
- 成本与 fallback 数据；
- TUI。

### Phase 3：复用与共进化

- 项目 Agent 和个人复用 Agent 库；
- Sidecar 记忆、Skill 和 Agent 候选；
- 离线评测、版本、激活和回滚；
- 多 WorkPackage。

### Phase 4：互通

- 可替换 Provider 路由后端；
- Multica 等协作平台适配；
- 可选 Web UI；
- 明确的导入、导出和共享合同。

## 14. 关键决策

1. 普通任务不会自动进入团队定义；显式 Agent 意图才触发。
2. Main Agent 职责固定，实现和模型可替换。
3. Main Agent 不退化为执行者，可以创建受限 SubAgent。
4. 团队来源包括项目定义、个人复用和任务临时生成。
5. 模型生成的团队必须先经过 Team Draft 和用户确认。
6. AgentDefinition 与 RuntimeProfile 解耦。
7. Coding 与知识工作共用编排内核，差异封装为 WorkPackage。
8. 执行 Agent 不能自我宣布 Done。
9. 客户定义执行规则，`require_approval` 会真实暂停工作。
10. AgentKey 在 Broker 模式下是 Loom 本地临时凭证，不冒充 Provider 凭证。
11. Sidecar 后台学习、生成候选，用户显式激活。
