# ADR-0023: Model-driven Loom control tools and governed proposals

**Date**: 2026-08-28
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

Phase 6 made Loom actions discoverable through one typed Slash-command catalog
and a deliberately narrow natural-language router. That router can safely
recognize explicit phrases, but it cannot understand arbitrary requests such as
"align s1, s2 and s3" without continuously adding language-specific patterns.
The conversation model can understand those requests, but model output is not
Loom execution authority and must not call internal IPC routes directly.

The design was informed by two current open-source implementations:

- Codex separates typed tool registration, per-turn model-visible tool specs,
  routing and approval. See the pinned
  [registry](https://github.com/openai/codex/blob/6be2a6ca952ac9f70676ce4dd07fda27175aa9dd/codex-rs/core/src/tools/registry.rs)
  and [router](https://github.com/openai/codex/blob/6be2a6ca952ac9f70676ce4dd07fda27175aa9dd/codex-rs/core/src/tools/router.rs).
- Grok Build separates its typed tool index from permission dispatch and grant
  handling. See the pinned
  [tool index](https://github.com/xai-org/grok-build/blob/9684fa3cdbf2995e30ea8b9b637f1db008f144fc/crates/codegen/xai-grok-shell/src/session/tool_index.rs)
  and [permission manager](https://github.com/xai-org/grok-build/blob/9684fa3cdbf2995e30ea8b9b637f1db008f144fc/crates/codegen/xai-grok-workspace/src/permission/manager/mod.rs).

Loom adopts typed discovery and dispatch, while retaining its stricter rule:
the model may prepare a governed proposal but cannot confirm a state change.

## Decision

1. Loom adds a versioned, built-in Control Tool Registry. Each tool freezes its
   canonical ID, model-facing name, schema, effect class and confirmation
   policy. Duplicate IDs, duplicate names and open JSON shapes fail closed.
2. Tools are supplied per response turn through a Harness-scoped adapter. The
   first adapter is a private owner-loopback MCP server attached to one Codex
   Segment Session. It exposes only the exact enabled tool names.
3. The MCP bearer token is random, turn-scoped and injected only into the child
   environment. It never enters argv, Prompt, transcript, Journal, diagnostics
   or evidence. Calls are bounded per turn and a stopped turn cannot be reused.
4. Read tools may return bounded non-secret metadata. Mutation-capable tools
   return a Proposal only. The first registry contains
   `loom.sessions.search` and `loom.sessions.align.preview`; there is no model-
   visible confirm, cancel or execute tool.
5. A Session alignment Proposal freezes the source Conversation IDs and content
   digests, target content digest, catalog digest, context mode, Segment,
   Attempt, expiry and proposal digest. Source or target drift, expiry and
   replay reject the decision.
6. Confirmation is accepted only through the user-owned App and authenticated
   local IPC route. The request must carry the exact Proposal ID and digest.
   Confirmation is one-time and creates a receipt-bound Context Alignment.
7. The next user message after confirmation creates a new immutable Segment in
   the same visible Conversation. The Segment Session binding includes the
   Context Alignment digest, so an existing native session cannot be reused
   across the alignment boundary.
8. The App publishes a bounded metadata catalog of Conversation ID, safe title
   and update time. It does not publish transcript content. Confirmed Capsule
   construction reads Loom-owned encrypted state: user constraints remain
   authoritative and prior model output remains untrusted.
9. Summary-only alignment transfers goals, accepted decisions, constraints and
   other authoritative user context. Continue-with-context may include bounded
   prior model output with untrusted provenance. Existing Context Capsule
   capacity and omission rules remain authoritative.
10. This first slice is a Codex Conversation vertical slice. OpenCode, Claude
    Code, Pi, Loom Native, Mission, Team and RoundTable tools must reuse the same
    registry, Proposal and confirmation boundary before becoming CURRENT.
11. Only process-local, versioned Loom built-ins are admitted. Arbitrary plugins
    and direct access to Journal, Vault root, policy/grant authority, state
    writers or terminal Run/Attempt authority remain prohibited.

## Alternatives Considered

### Extend the phrase router

- **Pros**: small client-only change.
- **Cons**: language-specific, brittle and incapable of reliable reference
  resolution or multi-step reasoning.
- **Why not**: it mistakes wording for capability and cannot scale to Loom's
  governed object model.

### Let the model invoke mutation routes directly

- **Pros**: fewer confirmation steps.
- **Cons**: model output becomes execution authority and can bypass user review,
  drift detection and replay protection.
- **Why not**: it violates Loom's core governance invariants.

### Expose the complete daemon API as tools

- **Pros**: broad feature coverage immediately.
- **Cons**: excessive authority, unstable schemas and a large prompt-injection
  blast radius.
- **Why not**: capability exposure must be minimal, typed and acceptance-driven.

## Consequences

### Positive

- Natural language is interpreted by the selected model instead of a growing
  list of fixed phrases.
- Tool capability, execution effect and user authority remain independently
  testable.
- Alignment preserves one visible Conversation while freezing a new Segment,
  Context Capsule and Harness Session boundary.

### Negative

- Each Harness needs an adapter that can expose the same typed registry.
- Model behavior still requires eval and installed live acceptance; source
  contract tests cannot prove every phrasing will select the intended tool.
- Stateful requests require an additional confirmation turn by design.

### Risks

- A model may choose the wrong source Conversation. The App displays exact safe
  titles and requires confirmation before context is applied.
- A stale Proposal may refer to changed content. Digest, expiry and one-time
  receipt checks reject it.
- A tool adapter may leak authority. Exact-name enablement, bounded calls,
  turn-scoped tokens and privacy-negative tests constrain the adapter.
