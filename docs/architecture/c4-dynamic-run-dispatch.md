# C4 Dynamic：Run 认领与终态提交

这张图描述一个已批准 WorkItem 从 Runtime 选择到本地权威终态的执行链。它不描述 Team Draft。

```mermaid
C4Dynamic
  title Dynamic Diagram - Governed Run Dispatch

  ContainerDb(stateStore, "Loom State Store", "SQLite WAL", "Events, claims, grants and projections")
  ContainerDb(workspaceStore, "Managed Workspaces", "Filesystem worktrees", "Per-Run mutable source and outputs")
  ContainerDb(artifactStore, "Evidence Artifact Store", "Content-addressed filesystem", "Immutable evidence bytes")
  System_Ext(agentRuntime, "Agent Runtime", "External supervised Agent process")

  Container_Boundary(daemon, "Loom Daemon") {
    Component(scheduler, "Scheduler", "Application service", "Owns Run lineage and dispatch")
    Component(runtimeRegistry, "Runtime Registry", "Runtime service", "Tracks versions, capabilities, status and capacity")
    Component(workspaceSupervisor, "Workspace and Process Supervisor", "Process service", "Prepares workspace and owns process group")
    Component(agentGrant, "AgentGrant Authorizer", "Security service", "Binds operations to Run claim generation")
    Component(runtimeAdapter, "Runtime Adapter", "Adapter layer", "Translates versioned Bridge messages")
    Component(evidenceRepository, "Evidence Repository", "Persistence component", "Stages and atomically publishes artifacts")
  }

  Rel(scheduler, runtimeRegistry, "1. Select compatible online RuntimeInstance")
  Rel(scheduler, stateStore, "2. Atomically claim Run, increment generation and start prepare lease", "SQLite transaction")
  Rel(scheduler, workspaceSupervisor, "3. Prepare worktree, skills and bounded environment")
  Rel(workspaceSupervisor, workspaceStore, "4. Materialize assigned workspace and source digest", "Filesystem")
  Rel(scheduler, agentGrant, "5. Issue scoped Grant for current generation")
  Rel(agentGrant, stateStore, "6. Persist only Grant hash and authorization facts", "SQLite transaction")
  Rel(workspaceSupervisor, runtimeAdapter, "7. Spawn owned process group")
  Rel(runtimeAdapter, agentRuntime, "8. Dispatch Run, catalog snapshot and AgentGrant", "JSONL/stdio + restricted environment")
  Rel(agentRuntime, runtimeAdapter, "9. Ack, heartbeat and stream ordered output", "JSONL/stdio")
  Rel(runtimeAdapter, agentGrant, "10. Validate generation and allowed operation")
  Rel(runtimeAdapter, evidenceRepository, "11. Stage bounded Evidence")
  Rel(agentRuntime, runtimeAdapter, "12. Submit ready_for_review or process exit")
  Rel(evidenceRepository, artifactStore, "13. Publish immutable artifacts by digest", "Atomic filesystem operations")
  Rel(scheduler, stateStore, "14. Commit Evidence references and terminal Event", "SQLite transaction")
  Rel(scheduler, agentGrant, "15. Revoke Grant and stop accepting the generation")
```

## Recovery rules

- prepare lease 过期时，恢复器递增 generation 后重新认领；旧进程不能恢复授权。
- RuntimeInstance 离线只阻止新认领；已有 Run 根据进程、heartbeat 和 Journal 事实收敛。
- Agent 返回的 terminal result 不是权威状态；第 14 步的本地事务才是权威 terminal。
- Client 断开、通知失败或未来协作 Adapter 回调失败不会回滚已提交 terminal。
- Evidence publish 失败时不提交有效 Evidence 引用，也不把 WorkItem 投影为 Done。
