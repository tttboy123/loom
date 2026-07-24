# C4 Level 1：System Context

这张图说明 Loom 在用户本机 Agent 生态中的位置。外部 Agent Runtime 和 Provider 都是黑盒系统，Loom 不假设其内部实现。

```mermaid
C4Context
  title System Context - Loom Platform

  Person(user, "Loom User", "Uses conversation and explicitly invokes Agent teams")

  System(loom, "Loom Platform", "Local-first Agent team orchestration, approvals, evidence, cost and evolution")

  System_Ext(agentRuntimes, "Agent Runtimes", "Codex, Claude Code, Pi, Hermes and other tool-capable agents")
  System_Ext(providers, "Model Providers", "Model APIs or subscription-backed services")
  System_Ext(osSecurity, "OS Security Services", "Keychain, secret store and local process isolation")
  System_Ext(collaboration, "Collaboration Platforms", "Optional boards, issue trackers and team adapters")

  Rel(user, loom, "Chats, explicitly invokes agents, approves actions and reviews results")
  Rel(loom, agentRuntimes, "Creates and supervises bounded agent runs", "Runtime adapters")
  Rel(agentRuntimes, providers, "Calls models when using native authentication", "Provider protocol")
  Rel(loom, providers, "Proxies brokered model requests", "HTTPS")
  Rel(loom, osSecurity, "Resolves secrets and enforces local process boundaries", "OS APIs")
  Rel(collaboration, loom, "Submits or observes work through optional adapters", "Versioned API")

  UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")
```

## Scope

Loom owns the orchestration and evidence lifecycle around Agent Runtime execution. It does not implement the Runtime's internal reasoning loop and does not require a specific Provider.
