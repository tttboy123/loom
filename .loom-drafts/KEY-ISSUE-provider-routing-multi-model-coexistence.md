# KEY ISSUE — Provider/模型路由与多 Provider 并存

日期：2026-08-04

状态：DRAFT — 排队输入（非权威，不进入当前候选）

优先级：重点问题，需要专门解决（P0 级产品能力缺口，规划为专门垂直，不在当前
调度框架目标边界内）。

## 1. 问题陈述（实测）

CC Switch 类本地路由的当前机制是"全局单 current provider"（数据库 is_current=1
+ 本地代理按 provider 转发），与请求中的模型名无关。实测证据：

- 请求携带 model=kimi-k3，但代理按 is_current=1 的 DeepSeek 转发；
- DeepSeek 上游返回 400：The supported API model names are deepseek-v4-pro or
  deepseek-v4-flash, but you passed kimi-k3；
- 结果：同一会话无法按任务/工作负载选择模型；"同时使用多个 provider"只能靠
  CLI 直连绕过（DeepSeek 直连 api.deepseek.com，Kimi 走代理）。

## 2. 问题本质

"用哪个 Provider/模型 + 用谁的 Key + 在哪个任务/会话里"三件事被合并成了一个全局
开关。需要拆开并绑定到"一次执行（Run）"粒度：

- Provider/模型选择 = 分派决策，受能力矩阵、预算、策略约束；
- Key 管理 = 每种 Provider 独立、可撤销、按 Run 授权；
- 多个 Provider 并存 = 多个 Profile/Runtime 实例共存，而不是互斥切换。

## 3. Loom 架构映射

### 已存在的地基（当前代码）

- RuntimeProfile（provider + model + auth_mode + 预算/超时）与
  RuntimeInstance（发现、在线状态、能力、容量）分离；
- Runtime 能力矩阵校验（internal/runtime/catalog.go，ErrMissingCapability，
  profile 所需能力 ⊆ 实例观测能力）；
- Credential Broker 三种认证模式：brokered（Agent→Broker→Provider）、
  provider_ephemeral（Provider 官方短期凭证）、native_auth（CLI 直连，
  Loom 不持 key）；真实 Key 仅在 Keychain；
- 每次 Run 的 CredentialGrant（token hash + Run + Provider + model + 额度 +
  过期，Run 结束撤销）；
- 明确禁止 Provider 自动 fallback / 隐藏无限重试。

### 缺失（需要重点解决的增量）

- 可替换 Provider 路由后端：按 Run 解析 Provider/模型/认证，而不是全局
  current provider（PRODUCT-PLAN Month 6 目标：至少两种 Agent Runtime、
  两种 Provider 认证模式和一个可替换路由后端；v0.3+ 可替换 Provider 路由后端）；
- 多 Provider 并存与按任务/Team 绑定模型的用户流程（"Kimi 团队"与
  "DeepSeek 团队"并存）；
- 预算/隐私策略参与路由决策；Provider 状态与成本数据作为路由输入。

## 4. 边界与落点

- 当前正在执行的调度框架目标（v0.4）明确排除 Provider 路由、fallback/
  checkpoint、模型客户端与 key；本问题不并入当前候选，避免越界；
- 建议落点：调度框架完成后，作为专门"路由与能力注册表"垂直（v0.3 方向），
  复用 capability matrix + Credential Broker + per-Run grant 三个地基；
- 可选的最小先行验证：基于现有 daemon 的 native_auth + RuntimeProfile
  做一个窄原型（两个 Profile/Team 并存、各走各的认证模式），验证领域模型
  能承载该需求，不实现通用路由后端。

## 5. 成功标准（草案）

- 同一 Loom 实例内两个 Provider/模型并存，各自有独立凭据与预算；
- 每次 Run 的路由由分派决策决定（能力矩阵 + 策略），无需全局开关；
- 切换/新增 Provider 不破坏既有会话与证据链；Key 永不进入 Journal/Evidence；
- 无隐藏 fallback 与无限重试；失败分类与 human_required lane 保持不变。
