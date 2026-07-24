# C4 Deployment：本地运行拓扑

Phase 1 是单机部署。进程隔离仍然重要：Loom daemon、Agent 子进程和 Sidecar 具有不同信任等级。

```mermaid
C4Deployment
  title Deployment Diagram - Loom Local-First Runtime

  Deployment_Node(workstation, "User Workstation", "macOS / Linux / Windows", "Primary local trust boundary") {
    Deployment_Node(userSession, "User Session", "Terminal or desktop session", "Interactive client process") {
      Container(client, "Loom CLI / TUI", "Go binary", "Conversation, Agent entry, board and approvals")
    }

    Deployment_Node(controlProcess, "Loom Control Process", "Long-running local process", "Trusted orchestration runtime") {
      Container(daemon, "Loom Daemon", "Go modular monolith", "Orchestration, rules, supervision and Broker")
    }

    Deployment_Node(dataDirectory, "Loom Data Directory", "User-private filesystem", "Phase 1 uses 0700 directories and 0600 files") {
      ContainerDb(stateStore, "Loom State Store", "SQLite WAL", "Events, projections and evidence metadata")
      ContainerDb(artifactStore, "Evidence Artifact Store", "Content-addressed files", "Immutable logs, reports and receipts")
    }

    Deployment_Node(agentProcessGroup, "Agent Process Groups", "Child processes", "Bounded external Agent runtimes") {
      Container(agentRuntime, "Agent Runtime", "Codex / Claude Code / Pi / Hermes", "Executes assigned WorkItems")
    }

    Deployment_Node(sidecarProcess, "Evolution Process", "On-demand or idle-time process", "Personal improvement isolation") {
      Container(sidecar, "Evolution Sidecar", "Local process", "Evaluates and proposes candidates")
    }

    Deployment_Node(osSecurityNode, "OS Security Boundary", "Keychain / secret store", "Protects Provider credentials") {
      ContainerDb(secretStore, "Secret Store", "OS managed", "Provider credentials and encryption keys")
    }
  }

  Deployment_Node(externalNetwork, "External Network", "HTTPS", "Untrusted network boundary") {
    Container_Ext(providerApi, "Provider API", "Remote service", "Model inference")
    Container_Ext(routerBackend, "Optional Router Backend", "Remote or local service", "Provider fallback and quota routing")
  }

  Rel(client, daemon, "Submits commands and reads projections", "Local API")
  Rel(daemon, stateStore, "Persists events and projections", "SQLite")
  Rel(daemon, artifactStore, "Atomically publishes evidence", "Filesystem")
  Rel(daemon, agentRuntime, "Supervises process group and Bridge", "stdio/JSONL")
  Rel(agentRuntime, daemon, "Streams events and brokered requests", "stdio + local HTTP/UDS")
  Rel(sidecar, daemon, "Reads redacted terminal data and submits candidates", "Restricted local API")
  Rel(daemon, secretStore, "Resolves credentials", "OS API")
  Rel(daemon, providerApi, "Sends brokered requests", "HTTPS")
  Rel(daemon, routerBackend, "Delegates optional Provider routing", "Versioned adapter")
  Rel(agentRuntime, providerApi, "Calls models only in native-auth mode", "Provider protocol")
```

## Process ownership

- Daemon 为每个 Run 创建独立 process group，并负责 timeout、cancel 和 cleanup。
- SQLite、Artifact Store 与 Secret Store 目录不挂载到 Agent process group。
- Phase 1 的 Loom State 和 Artifact Store 是用户私有文件，不宣称应用层加密；磁盘加密由 OS 提供。
- Sidecar 默认不常驻；即使崩溃也不能改变正在运行的任务。
- Provider 网络请求必须携带实际认证模式，便于 UI 和 Evidence 如实展示隔离等级。
