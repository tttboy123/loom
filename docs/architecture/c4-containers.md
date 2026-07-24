# C4 Level 2：Containers

Container 表示独立运行或持久化的单元，不等同于 Go package。

```mermaid
C4Container
  title Container Diagram - Loom Platform

  Person(user, "Loom User", "Chats, invokes Agent teams and approves actions")
  System_Ext(agentRuntimes, "Agent Runtimes", "External coding or knowledge-work Agent processes")
  System_Ext(providers, "Model Providers", "Model inference and subscription services")
  System_Ext(osSecurity, "OS Security Services", "Keychain, secret store and process APIs")

  System_Boundary(loomBoundary, "Loom Platform") {
    Container(client, "CLI / TUI Client", "Go", "Conversation, explicit Agent entry, board and approvals")
    Container(daemon, "Loom Daemon", "Go modular monolith", "Owns orchestration, policy, process supervision and state transitions")
    ContainerDb(stateStore, "Loom State Store", "SQLite WAL", "Append-only events, projections, evidence metadata and approvals")
    ContainerDb(artifactStore, "Evidence Artifact Store", "Content-addressed filesystem", "Immutable logs, reports, receipts and exports")
    Container(sidecar, "Evolution Sidecar", "Local process", "Builds evaluated Memory, Skill, Agent and strategy candidates")
  }

  Rel(user, client, "Uses")
  Rel(client, daemon, "Submits commands and decisions", "Local versioned API")
  Rel(daemon, client, "Streams projections and approval requests", "Local event stream")
  Rel(daemon, stateStore, "Appends events and updates projections", "SQLite transactions")
  Rel(daemon, artifactStore, "Atomically publishes and reads evidence artifacts", "Filesystem")
  Rel(daemon, agentRuntimes, "Starts, dispatches and cancels bounded runs", "JSONL/stdio")
  Rel(agentRuntimes, daemon, "Streams events, evidence and brokered requests", "JSONL + local HTTP/UDS")
  Rel(daemon, providers, "Proxies brokered model requests", "HTTPS")
  Rel(agentRuntimes, providers, "Calls models in native-auth mode", "Provider protocol")
  Rel(daemon, osSecurity, "Resolves Provider credentials and manages process groups", "OS APIs")
  Rel(sidecar, daemon, "Reads redacted terminal traces and submits candidates", "Restricted local API")

  UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")
```

## Container boundary

- Daemon 是唯一可以提交权威状态转移的进程。
- Client 只提交命令和读取投影。
- Sidecar 不能直接写 SQLite。
- Client、Agent Runtime 和 Sidecar 不能直接写 Artifact Store。
- Agent Runtime 不直接访问 Loom State Store。
