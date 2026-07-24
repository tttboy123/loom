# Loom Platform

Loom 是一个本地优先的 Agent 团队编排与协作平台。

当前分支处于 **implementation-ready / implementation-not-started**：

- 产品入口和边界已定义；
- Phase 1 领域模型、Runtime 发现、任务级授权、租约、Schema、协议、状态机和验收已定义；
- Codex 引导期开发团队、验证流程和只读代码分析角色已定义；
- 尚无产品代码、可运行 CLI 或真实 Demo。

## 文档入口

1. [产品方案](./PRODUCT-PLAN.md)
2. [技术方案](./TECH-PLAN.md)
3. [架构与流程](./docs/ARCHITECTURE.md)
4. [C4 技术架构](./docs/architecture/README.md)
5. [当前状态](./docs/CURRENT.md)
6. [Codex 开发流程](./docs/DEVELOPMENT.md)
7. [Architecture Decision Records](./docs/adr/README.md)

## 关键入口规则

普通对话不会自动创建 Agent 团队。只有用户显式选择 Use Agent、Agent、Team 或明确分派时，才进入团队解析：

```text
已有完整团队
→ 直接加载

没有完整团队
→ Main Agent 生成 Team Draft
→ 一次确认一个关键选择
→ 用户接受
→ 绑定真实 RuntimeInstance
→ 创建 Agent 实例和任务看板
```

## Phase 1 开工边界

Phase 1 从 [TECH-PLAN.md 的实施切片](./TECH-PLAN.md#14-phase-1-实施切片) 开始，并以其中的
[22 项验收](./TECH-PLAN.md#15-phase-1-验收) 为完成标准。

Phase 1 可以使用 Agent Runtime 原生认证，但必须显示 `native_auth`，不能宣称已经具备
Credential Broker 隔离。Phase 1 的 `AgentGrant` 只授权当前 Run 调用 Loom 本地能力；真实
Broker 代发属于 Phase 2。
