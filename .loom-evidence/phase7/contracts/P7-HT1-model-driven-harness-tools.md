# P7-HT1 - Model-driven Loom Harness tools

Status: `PARTIAL / INSTALLED BUILD 259 / LIVE MODEL GATE PENDING`

Date: 2026-08-28

## Goal

Let the Conversation model understand ordinary language and use narrow Loom
Harness tools, while keeping every state change under explicit user governance.
The first vertical slice supports searching Conversation metadata and preparing
a proposal to align selected Conversations into the current visible
Conversation.

## User journey

1. The user says, for example, "Align s1 and s2 with this conversation."
2. The model uses `loom_sessions_search` when it needs to resolve a title or ID.
3. The model calls `loom_sessions_align_preview` with exact Conversation IDs and
   either `summary_only` or `continue_with_context`.
4. Loom returns a Proposal. Nothing has changed yet.
5. The Conversation displays one review card with source titles, context scope,
   status, Confirm and Cancel.
6. Confirm creates a one-time alignment receipt. The user's next message starts
   a new immutable Segment whose binding freezes that receipt digest.
7. Context Capsule construction imports approved source context under Loom's
   authority and provenance rules. Old model output cannot become a system or
   authoritative instruction.

## Source contract

- `internal/controltool` owns the versioned typed registry, strict schemas,
  effect classes, Proposal shape, digest and bind-once Gateway seam.
- `internal/runtime/harnessadapter/control_mcp.go` owns the private MCP server,
  exact tool exposure, turn token, call bound and proposal capture.
- `CodexSegmentSession` starts one control server per Segment Session and puts
  the secret token only in the child environment, never argv.
- `productCodexSegmentBackend` freezes one control turn around each model
  response and returns captured Proposals through the Harness Gateway.
- `LocalProductChatAPI` resolves source state, validates drift/replay/expiry,
  creates the receipt and applies it only to the next Segment.
- `chat_control_decision` is a metadata-only local IPC route. It is not exposed
  to the model.
- macOS strict models decode the closed Proposal and receipt shapes. The store
  publishes a bounded metadata-only Session catalog and the Conversation shows
  the governed confirmation card.

## Security and privacy

1. No tool has direct Journal, Vault, credential, policy, grant, StateWriter or
   terminal Run/Attempt authority.
2. The model receives no transcript catalog. Search returns ID, title and update
   time only.
3. Proposal confirmation is user-only, digest-bound, expiring and one-time.
4. Source and target changes reject the Proposal instead of silently importing
   different content.
5. Context alignment creates a different Segment Session ID even when Harness,
   Provider and model are unchanged.
6. Prompt, transcript, Provider body, hidden reasoning, API key and MCP token do
   not enter operational diagnostics or evidence.

## Acceptance matrix

- Registry identities, strict schemas and digest are deterministic.
- MCP lists only the two admitted tools and rejects bad token, unknown fields,
  calls outside an active turn and more than eight calls per turn.
- Codex argv contains no token and enables only exact Loom tool names.
- Runtime Begin/End preserves Conversation, Segment, Attempt, Incident, catalog
  and Proposal across the Harness Gateway response.
- No alignment exists before confirmation.
- Confirmed alignment creates a new Segment on the next user message.
- Summary-only imports authoritative user content but excludes prior model
  replies; replay and source drift fail closed.
- Swift models reject unknown fields; decision requests contain no transcript or
  credential fields.
- Confirmation card renders at 560px and 360px accessibility size without
  overflow and exposes separate Confirm/Cancel controls.
- Existing Provider, Runtime, Vault, Mission, Team and RoundTable contracts
  remain green in the complete daemon and macOS suites.

## Installed acceptance

- Loom `0.5.5` Build 259 is installed at
  `/Users/lune/Applications/Loom.app`; candidate/install App and daemon hashes
  are equal and the strict deep signature passes.
- The managed daemon starts from the installed bundle and serves the real
  private UDS. Read-only `setup_snapshot` returns 24 Providers, seven online
  Runtimes, seven Conversation Profiles and five metadata-only import
  candidates.
- Persisted Loom Native Runtime facts migrate from one ambiguous label to
  distinct DeepSeek, Kimi and MiniMax labels without changing Runtime identity
  or frozen Team/Agent bindings.
- Installed Runtime and Provider UI is nonblank and exposes Codex, Claude Code,
  OpenCode, Pi and the three distinct Loom Native rows.
- Detailed evidence and screenshots are in
  `../verification/P7-HT1-installed-build259.md`.

## Current limits

- No paid Provider request or real model-selection eval was run in this slice.
- OpenCode, Claude Code, Pi and Loom Native control adapters are `TARGET`.
- Mission, Team, RoundTable, diagnostics and other Loom capability tools are
  `TARGET`; Phase 6 commands remain the current fallback for those surfaces.
