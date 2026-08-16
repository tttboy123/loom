# ADR-0018: Provider registry and Run-bound profiles

**Date**: 2026-08-09
**Status**: accepted
**Deciders**: Product Owner, Loom Architecture Controller

## Context

The Phase 2C setup snapshot exposed `codex` and `minimax` as two fixed fields,
and the macOS client rendered the same two rows directly. Adding another model
Provider therefore required coordinated backend, wire-contract, client, and
verification changes. The screen also mixed an Agent Runtime (Codex) with a
model Provider (MiniMax).

CC Switch demonstrates a useful directory and preset experience across Claude
Code, Claude Desktop, Codex, Gemini CLI, Grok Build, OpenCode, OpenClaw, and
Hermes. Its presets collapse to a smaller set of protocol families. Loom needs
the discoverability of that experience without adopting a global current
Provider, overwriting third-party configuration, or allowing credentials to
become execution authority.

## Decision

1. Loom uses one registry of Provider descriptors. A descriptor contains no
   credential and makes no availability claim. The setup snapshot exposes an
   ordered `providers` collection; the legacy Codex and MiniMax fields remain
   temporarily for client compatibility.
2. Agent Runtimes and model Providers are separate UI sections. Codex native
   login remains a Runtime connection. OpenAI is the corresponding model
   Provider for that currently available conversation path.
3. Provider compatibility is protocol-family driven: OpenAI Responses,
   OpenAI-compatible, Anthropic Messages, Gemini Generate Content, Bedrock
   Converse, Vertex Generate Content, Ollama, and native Runtime auth.
4. Fixed API-key Providers use the existing Broker and macOS Keychain. Each
   verification request has a registry-owned HTTPS origin, bounded timeout and
   response, no proxy, no redirect, no generative request, and no secret in a
   snapshot, Journal payload, Evidence, prompt, or log.
5. Managed-cloud identity, local Runtime discovery, and custom endpoints are
   distinct connection kinds. They must not be passed through the fixed API-key
   verifier. Custom endpoints require a later reviewed SSRF and credential
   egress boundary.
6. A connected credential is not an execution route. Conversation and Agent
   execution may use a Provider only after a versioned Runtime Profile binds
   Provider, protocol, model, auth mode, budget, and policy. Each Run/Attempt
   records the exact binding and receives its own applicable Grant.
7. Loom does not adopt a global `current provider`, hidden fallback, or
   request-level hot switch. Multiple Profiles may coexist; routing remains an
   explicit governed decision.

## Consequences

- Provider management scales without adding a new wire field and hard-coded UI
  row for every vendor.
- Users can discover and connect mainstream official Providers and gateways in
  one searchable directory.
- The UI truthfully distinguishes connected credentials from executable
  Profiles and Runtimes.
- Phase 2D must still complete Run-bound Profile materialization and client
  adapters before broad execution compatibility can be claimed.

## Alternatives Considered

### Copy CC Switch configuration files and global switching

Rejected. It makes external config files a second state authority, conflates
Provider choice with process-wide state, and cannot preserve exact per-Run
lineage.

### Add one field and one screen row per Provider

Rejected. It repeats the Phase 2C coupling and makes compatibility changes
unsafe and expensive.

### Accept arbitrary custom verification URLs immediately

Rejected. Without an endpoint policy, DNS/IP fencing, redirect rules, and
explicit credential egress review, this creates an SSRF and secret-disclosure
boundary.
