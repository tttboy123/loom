# P2D-W2C Pi Context Retrieval Transport v1

**Date**: 2026-08-13  
**Status**: `CURRENT / SOURCE VERIFIED / POST-BUILD-64`  
**Goal**: unified Phase 2D only  
**WorkItem**: existing P2D-W2C with P2D-W2D diagnostics

## Result

The exact locked Pi 0.82.1 Harness can now invoke the existing Attempt-bound
`context_retrieval` broker through one private, one-use extension transport. A
Pi Attempt receives the tool only when its immutable execution binding freezes
the capability and the Supervisor supplies the matching scoped retriever.
Capability and broker mismatches fail before process start.

The adapter creates an owner-only extension root and source file for one
Attempt, plus an owner-only Unix-domain socket. The service accepts only the
exact child process PID, one random per-Attempt capability, and one strict
`loom_read_context` request. It revalidates item ID, content digest, artifact
scope, trust class, source class, and returned content digest before returning a
bounded result. Credential references, secret scope, hidden reasoning, Provider
bodies, duplicate fields, unknown fields, a second tool call, and identity or
classification drift fail closed.

Pi's two-turn RPC transcript is parsed as a closed lifecycle. The first turn may
contain exactly one ContextRead tool call; the second may contain only final
assistant text. Retrieved content is delivered only to Pi's native tool-result
message and is never projected into Bridge frames, Journal, Evidence, or
operational diagnostics. Both model rounds are attributed to the same Attempt
with overflow-checked combined token and cost accounting.

## Runtime conformance

`context_retrieval` is not inferred from a `0.82.1` version string. The Runtime
probe now has an explicit `ContextRetrievalConformance` port. The production Pi
metadata runner offers that conformance only when its executable bytes match the
locked SHA-256 identity and all executable, isolation-root, and search-path
bindings still validate. The probe invokes it only for exact Pi 0.82.1 and
publishes the capability only after success.

Ordinary probes, historical Runtime facts, test fixtures, typed-nil providers,
other versions, and failed conformance therefore receive no ContextRead
capability. This preserves existing authority and prevents version-only
capability upgrades. Product profiles require the observed capability, and each
new Attempt freezes it in the execution-binding digest.

## Private transport

- Extension root mode is `0700`; extension source and socket mode are `0600`.
- Roots, source, and socket are regular/expected filesystem objects owned by the
  current user; insecure directories, symlinks, malformed IDs, and path drift
  are rejected.
- Darwin uses `LOCAL_PEERPID`; Linux uses `SO_PEERCRED`. Unsupported platforms
  fail closed.
- Long Darwin socket paths use one exact random owner-only short root under
  `/tmp`, with exact cleanup rather than broad deletion.
- The one-use capability, request, response, and mutable line buffers are
  bounded and best-effort zeroized. Closing the Attempt cancels an accepted
  connection and removes the exact extension/socket artifacts.
- Extension source and diagnostics contain no API key, Authorization header,
  Prompt text, retrieved body, Provider response, or credential material.

## Verification

- Context extension tests cover exact successful retrieval, cleanup, close
  cancellation, insecure roots, malformed IDs, protocol drift, item drift,
  content-free source, and the Darwin socket-length boundary.
- Strict RPC tests cover one tool then final text, second-tool rejection,
  result/digest/classification drift, typed-nil retrievers, missing Authority,
  capability mismatch, and legacy diagnostic compatibility.
- Probe tests cover explicit and runner-provided conformance, typed nil,
  version mismatch, verification failure, no-conformance Pi 0.82.1, and
  immutable capability observations.
- Daemon regressions prove copied Pi 0.82.1 authority remains a no-write
  observation and ordinary offline recovery does not rewrite capabilities.
- The opt-in locked component gate starts the real local Pi 0.82.1 executable
  against a loopback fake OpenAI-compatible endpoint. It performs two Provider
  rounds, one native extension/UDS broker read, exact result delivery, combined
  accounting, content-free Bridge output, and cleanup. It passed 20 consecutive
  race-enabled runs.
- `go test ./...`, affected package race tests, `go vet ./...`,
  `git diff --check`, and `go mod verify` pass.

## Open boundary

This closes the bounded Pi 0.82.1 Harness-callable ContextRead transport source
slice. It does not activate a general multi-tool loop, persistent tool-result
queue, crash resume, Codex or Claude Code Attempt-scoped MCP transport,
parallel Aggregation Attempts, encrypted export, or installed CV6 acceptance.

Build 64 predates this source. No bundle was built, launched, installed, or
connected to a real Provider. Installed Loom remains v0.5.2 build 39 and was not
modified. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
