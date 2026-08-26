# Phase 2D installed live evidence: DeepSeek, mixed Team, isolation

Status: `PARTIAL / LIVE EVIDENCE CAPTURED`

Date: 2026-08-21

## Verified

- Installed bundle: `/Users/lune/Applications/Loom.app`, v0.5.3 build 65.
- Bundled `loomd` starts with the managed parent and canonical state/socket
  arguments; the installed UDS is reachable after cold launch.
- DeepSeek brokered conversation: `cmd/loom-net-probe` received a real
  bounded reply through the installed app and reported network egress.
- Mixed Provider Team: installed-live reruns created four independent Agent
  bindings, with DeepSeek and MiniMax accounts both preflight-ready; Mission
  start returned `running` and the accounting board contained separate account
  rows. The latest fresh run passed after the timeline stream repair.
- Failure isolation: after MiniMax credential revoke, two DeepSeek nodes stayed
  `ready`; only two MiniMax nodes became `blocked` with the credential-specific
  recovery reason. No global offline state was emitted.
- Cleanup was corrected to use explicit credential Replace for revoked records,
  followed by Verify. Final installed snapshot: Vault `unlocked`, DeepSeek
  `verified` revision 2, MiniMax `verified` revision 8.

## Addendum: installed Web Mission and timeline recovery

The installed Web Mission was rerun after enabling the enrollment-only
transport path. The first rerun exposed a host-level DuckDuckGo Lite block;
the bounded GitHub Repository API fallback was then added to the built-in
search transport. After reinstall and cold launch, a fresh Enrollment-bound
Agent produced a governed WebSearch execution fact and the Team board reached
`succeeded`.

Timeline recovery was also completed. Invalid historical delivery lineage now
produces an explicit `delivery_record_invalid` stream gap with a board and
attention item. The installed sweep reported 48 readable timelines, 18
recoverable gaps, and 0 `state_unavailable` responses.

The installed Web/MCP isolation gate passed: revoking a bound `web_search`
Enrollment blocked only that Agent while its unbound peer remained ready. The
live Web Mission also recorded bounded WebSearch/WebFetch facts through the
installed daemon and its final board status was `succeeded`.

This file contains no API key, authorization header, prompt, conversation body,
Provider response, or raw credential material.
