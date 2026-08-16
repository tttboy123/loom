# ADR-0019: Per-Agent execution profiles and Provider Accounts

**Date**: 2026-08-09
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

Loom must remain a chat-first pair-programming partner while governing a Team
whose Agents may use different Runtimes, Providers, accounts, and models. A
conversation-level or Team-level Provider switch cannot represent a Team that
simultaneously runs Claude Code with Anthropic, Codex with OpenAI, Loom with
Moonshot/Kimi, and Loom with MiniMax. It also cannot isolate a revoked account
or account-specific limit.

The current domain already gives every `TeamDefinitionRole` a
`RuntimeProfileID`, and `BuildSavedTeamRuntimeBinding` creates one binding per
role. `RuntimeProfile` separates Adapter, Provider, Model, auth mode, limits,
and capabilities, but it does not identify the Provider Account or freeze the
credential reference and revision. That is a P0 lineage and isolation gap.

## Decision

1. Conversation Profiles and Agent Execution Profiles are separate scopes. A
   conversation selection never becomes a Team default or Agent authority.
2. Agent Runtime, Provider, Provider Account, and Model are independent
   dimensions. Loom itself is the Harness Platform and owns the Harness Core.
   During v0.5.x, `AdapterType` is the migration carrier for the Runtime
   Adapter type; `ProviderID` identifies the protocol Provider;
   `ProviderAccountID` identifies one account; `ModelID` identifies the
   selected model.
3. During the v0.5.x migration, the existing Go `RuntimeProfile` is the stored
   Execution Profile contract and `RuntimeProfileID` remains its stable schema
   reference. This avoids a hollow rename while preserving the existing
   `TeamRole -> AgentDefinition -> RuntimeProfile` chain.
4. A non-native Execution Profile binds an opaque credential reference and
   positive credential revision plus a canonical endpoint fingerprint. Secret
   body bytes are never fields of the Profile or frozen binding.
5. `BuildSavedTeamRuntimeBinding` resolves and freezes each role independently.
   Every Agent Attempt carries its own Runtime kind, Runtime Adapter and
   instance, Provider, Provider Account, Model, endpoint fingerprint,
   credential reference and revision, timeout, and required capabilities.
   Dispatch cannot depend on a global Provider client.
6. Provider management owns Provider Accounts, endpoints, and Credentials.
   Agent editing owns the Execution Profile that selects an account and model.
   Team-level values only initialize newly added Agents.
7. Credential or Provider Account failure is Agent-local. Other ready Agents
   retain their own status and dispatch eligibility. The board reports the
   affected account and a specific non-secret reason instead of a Team-wide
   `offline` state.
8. Fallback is absent unless explicitly versioned and approved. Every fallback
   decision is auditable; Loom never silently changes a Claude or Codex Agent's
   Provider.
9. Loom owns the minimum runtime state required for a dependable Agent
   experience: named Provider Accounts, explicit ordered fallback, current
   usability, bounded cooldown after a known failure, and actionable recovery.
10. Loom owns execution-governance accounting needed for Phase 2D: concurrency,
    configured limits and budgets, token/cost usage, error rate, and rate-limit
    outcomes are attributed to the exact Provider Account, Run, Attempt, and
    Agent. UsageHub may later own cross-product Account Pool optimization,
    quota-aware routing, spend analytics, and recommendations by consuming
    redacted facts. Loom does not depend on UsageHub for startup, credential
    resolution, dispatch, fallback, recovery, or the Phase 2D audit chain, and
    UsageHub cannot grant execution authority.

## Alternatives Considered

### One Provider per conversation or Team

Rejected because it prevents heterogeneous Teams and turns one credential
failure into an unnecessary Team-wide outage.

### Resolve credentials globally by Provider ID

Rejected because multiple accounts for one Provider become ambiguous and an
Attempt cannot prove which credential revision authorized its request.

### Silent Provider fallback

Rejected because it changes cost, behavior, data destination, and audit lineage
without explicit authority.

## Consequences

- Loom can combine different Runtime and Provider pairs in one Team while the
  central conversation remains the primary workspace.
- Provider Account identity and credential revision become part of immutable
  execution lineage without disclosing secret bytes.
- Existing RuntimeProfile fixtures and dispatch payloads require a controlled
  migration before Phase 2D can claim multi-Provider Team execution.
- Account usability, fallback decisions, cooldown, and board status must be
  projected per Agent.
- Phase 2D adds the bounded per-Agent and per-Provider-Account accounting needed
  to explain execution, enforce configured limits, and preserve audit lineage.
  Cross-product pooling, optimization, forecasting, and spend-management remain
  a separate UsageHub product boundary.

## 2026-08-13 Harness Platform terminology amendment

DeepSeek Harness Core is accepted as a primary reference implementation for
Loom's native Agent Loop, inbox, capability seams, tool scheduling, cancellation,
and lifecycle behavior. Loom will selectively port those semantics and tests;
it will not embed Cordis or treat DeepSeek Harness session state as a second
authority.

The normative ownership model is:

```text
Loom Harness Platform
  -> Loom Harness Core
    -> Runtime Contract
      -> Runtime Adapter
        -> Loom Native / Pi / Codex / Claude Code / DeepSeek Harness Runtime
          -> Provider Adapter or runtime-owned model transport
```

`RuntimeAdapter` bridges lifecycle, messages, tool calls/results, cancellation,
and consumption proofs for a concrete Runtime. It does not own Team, Run,
Attempt, policy, approval, sandbox, Credential Vault, Provider Account,
Context Capsule, Journal, or recovery authority. Loom Native executes the
Runtime Contract in process and does not need a fictitious Harness Adapter.

Existing `Harness`, `HarnessAdapter`, and `AdapterType` fields remain readable
for v0.5.x compatibility and accepted evidence. New contracts and UI use
`Runtime` and `Runtime Adapter`; a later schema migration must preserve exact
historical bytes and map legacy values deterministically.

## 2026-08-14 composition ownership amendment

ADR-0021 adds a Loom-owned Composition Kernel beneath the Harness Platform
product boundary. Its Profile, Bundle, Capability Context, Effect, and snapshot
contracts compose bounded ports; they do not create a global Runtime, Provider
client, Provider Account, model, credential, policy, or fallback.

Every admitted Agent Attempt retains both its immutable Composition Snapshot
digest and its independent Frozen Execution Binding. Recomposition can affect
new scopes only and cannot rewrite an active or historical per-Agent binding.
Runtime adapters consume narrowed Loom Core ports and remain unable to replace
Journal, Vault, policy, approval, recovery, or terminal authorities.

See [ADR-0021](0021-governed-composition-kernel-and-daemon-strangler.md),
[P2D-COMP1](../../.loom-evidence/phase2d/contracts/P2D-COMP1-composition-kernel.md),
and [P2D-COMP2](../../.loom-evidence/phase2d/contracts/P2D-COMP2-product-daemon-strangler.md).
