# Multica 架构对照与 Loom 吸收边界

> 对照快照：`multica-ai/multica@8d18d3a9ec3f4bc850f4379a280825335fedae5a`
> 核对日期：2026-07-24

本记录用于解释 Loom 从 Multica 吸收了哪些已验证模式，以及为什么不复制其整体产品架构。Multica
是多用户协作控制面与本地执行 daemon；Loom Phase 1 是本地优先、conversation-first 的治理与编排
内核。两者可以复用机制，但不能共享状态权威或产品中心。

## 对照

| 维度 | Multica 当前模式 | Loom 选择 |
|---|---|---|
| 用户入口 | Workspace、Issue、Project 和 Board | 普通对话默认；显式 Agent 后才创建 Team/WorkItem |
| Agent 创建 | 空白、模板、AI Builder；对话旁实时 Draft | 吸收实时 Draft；一次只问一个关键选择；确认后实例化 |
| Agent 配置 | Agent 行直接绑定 Runtime、模型和参数 | AgentDefinition、RuntimeProfile、RuntimeInstance 三层分离 |
| Runtime | 一个 daemon 自动发现多种本地 Agent CLI | 吸收自动发现、版本、能力、状态和容量快照 |
| 团队 | Squad 路由给 Leader；成员不会自动 fan-out | Team 必须产生真实 SubAgent WorkItem、Run、Grant 和 Evidence |
| 调度 | PostgreSQL queue、claim、prepare lease、stale recovery | 吸收 generation-fenced claim 与 prepare lease，落在本地 Journal |
| 本地授权 | task-scoped `mat_` API token | 吸收为 AgentGrant；与 Provider CredentialGrant 分离 |
| 工作目录 | 每任务隔离 workspace、repo cache/worktree、GC | 吸收 managed workspace、source digest、保守 GC |
| 状态 | PostgreSQL 当前状态表 + realtime notification | Event Journal 事实源 + 可重建 Projection |
| 终态 | daemon 通过有界 HTTP 重试回传 server | 本地事务提交 terminal；网络通知不成为状态权威 |
| Evidence | task messages、result、session、branch、attachments | 不可变 artifact + digest + acceptance/Verifier Evidence |
| 共进化 | Skill、模板和运行资产位于主平台 | 独立 Sidecar 只生成候选，不能在线修改执行面 |

## 已吸收

1. 单 daemon 管理多个项目和多种 Agent CLI。
2. Runtime 自动发现、版本探测、能力目录、在线状态与容量。
3. AI Builder 的对话与实时结构化草案，但保留 Loom 的一次一问。
4. Draft 只能引用可用模型、Skill、成员和权限目录中的真实 ID。
5. Run claim、prepare lease、认领恢复和旧执行者隔离。
6. 绑定 Agent、Run 和认领代次的任务级本地授权。
7. 每任务受管 workspace、Git repo cache/worktree 和保守 GC。
8. Timeline、Attention 和 Runtime 可用性作为同一 Journal 的轻量投影。

## 刻意不复制

1. 不让 Issue Board 成为使用 Agent 的前置入口或第二状态权威。
2. 不采用“Squad 只路由给 Leader”的假团队语义。
3. Phase 1 不引入 PostgreSQL、Redis、S3 或多节点 API 控制面。
4. 不把 AgentDefinition 直接绑定某台设备或长期 `runtime_id`。
5. 不把任务级 Loom Token 描述为 Provider 凭证。
6. 不用网络 terminal callback 取代本地 Event Journal 权威。
7. 不因 workspace GC 删除已引用失败 Evidence、Source digest 或 terminal Receipt。

## Loom 形成的组合优势

```text
Multica 的低摩擦 Agent Builder、Runtime discovery、claim/lease 和 workspace 工程
+
Loom 的显式 Agent 边界、真实多 Agent 任务图、客户规则、不可变 Evidence 和本地状态权威
=
无需先管理 Issue，也能安全创建、运行、观察和改进 Agent Team
```

## 来源

- [Multica repository](https://github.com/multica-ai/multica)
- [CLI and Agent Daemon Guide](https://github.com/multica-ai/multica/blob/8d18d3a9ec3f4bc850f4379a280825335fedae5a/CLI_AND_DAEMON.md)
- [Agent Builder handler](https://github.com/multica-ai/multica/blob/8d18d3a9ec3f4bc850f4379a280825335fedae5a/server/internal/handler/agent_builder.go)
- [Agent Creation Studio](https://github.com/multica-ai/multica/blob/8d18d3a9ec3f4bc850f4379a280825335fedae5a/packages/views/agents/components/agent-creation-studio.tsx)
- [Task claim queries](https://github.com/multica-ai/multica/blob/8d18d3a9ec3f4bc850f4379a280825335fedae5a/server/pkg/db/queries/agent.sql)
- [Squad runtime contract](https://github.com/multica-ai/multica/blob/8d18d3a9ec3f4bc850f4379a280825335fedae5a/server/internal/service/builtin_skills/multica-squads/SKILL.md)
