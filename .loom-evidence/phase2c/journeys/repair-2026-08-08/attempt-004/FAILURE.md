# Phase 2C Repair Journey Attempt 004 Failure

**Date**: 2026-08-08  
**Journey ID**: `574ace69-ecd7-40ff-abe4-46aa2bf09257`  
**Fixture root**: `/private/tmp/loom-phase2c-chat-20260808-004`  
**Source-lock SHA-256**: `b93896959d5790a013e4d975d7fd0c38b9135e82f8127ac6b65a7afbaafa5612`  
**Outcome**: `FAIL - NATIVE_CHAT_MESSAGE_WIRE_KEY_MISMATCH`

## Failure

Repair 5 passed its complete lock-bound matrix, and attempt 004 proved the TUI
J1 and J2 chat paths through a real PTY. Physical spaces were preserved, the
first-use response completed inside the extended deadline, folder display was
sanitized, and the private workspace sentinel was not read or copied.

The native J2 path then selected `Documents` through the macOS directory picker
and displayed only that safe folder name. Every native send attempt reached the
daemon but returned `invalid_request`. The controlled IPC summary records five
failed GUI `chat_message` requests, while both TUI `chat_message` requests in
the same fixture passed.

The source-level cause is exact: `LocalProductChatThreadRequest` maps its Swift
property `threadID` to the required JSON key `thread_id`, but
`LocalProductChatMessageRequest` has no matching `CodingKeys`. Swift therefore
encodes `threadID`. The daemon's strict decoder requires `thread_id`, rejects
the unknown/missing field pair, and returns `invalid_request`. The native store
restores the draft after the error, so the user sees a send action that appears
to do nothing.

This is a native wire-contract defect, not a model, Team, Mission, Run, or
authority transition. Repair 6 must add the missing exact key mapping and a
regression test that proves the encoded message request contains exactly
`thread_id` and `content`, with no `threadID` key.

## Completed Journey Evidence

- J1 TUI: ordinary no-folder chat passed in `3.979671` seconds.
- J2 TUI: selected `workspace-j2`, displayed only the safe folder name, and
  completed a tentative chat response without reading the sentinel.
- J2 Native: the system directory picker selected `Documents`; both title and
  composer context showed only `Documents`.
- J2 Native chat: failed at the strict IPC boundary with `invalid_request`.
- J3-J10: not attempted after the first source defect was proven.

## Authority and Shutdown Audit

- Final Journal events: exactly three:
  `WorkRunIdentityIndexInitialized`, `AgentGrantIdentityIndexInitialized`, and
  `RuntimeInstanceDiscovered`.
- Team events: `0`.
- Mission and execution Run facts: `0`.
- Duplicate event IDs: `0`.
- SQLite `PRAGMA integrity_check`: `ok`.
- The private J2 sentinel appeared only in its source file.
- TUI, Native, and daemon all stopped; related attempt binaries after shutdown:
  `0`.
- Journey socket and socket lock after shutdown: absent.

## Artifact Bindings

| Artifact | SHA-256 |
|---|---|
| `tui/transcript.txt` | `3f68804f3f0a3f252eab6b44ad4722e6d99d439e4e1cbaa03c678898b54fd5df` |
| `ipc/request-response-summary.jsonl` | `9a71d0d5778beac3014261bf6791a1b3719a28d0126d46dec741dbc5ca3e4335` |
| `daemon/structured-log.jsonl` | `d727f1ba73b77d22604880d1f77b75eb36ab323434b194f69ce371260633bdc9` |
| `state/loom.db` | `a7179393ed9d8b42d599f3a5132f62e4f19ddddc50534afa26e23fbb23c9e01d` |
| `state/chat-threads.json` | `3a2ba82871623d309ceac678ce9b2a7a7f72f9563e4ae3ddbe332c897d4eff89` |
| `source/source-lock.json` | `b93896959d5790a013e4d975d7fd0c38b9135e82f8127ac6b65a7afbaafa5612` |
| `bin/loom` | `9dd4ea8930160094f649c3bdea470a6aa059c47f7a042d484b0e8f518780d0be` |
| `bin/loomd` | `a96d0df6390fc784d90d6a234ce31b742730c45f13b412f510567eda0dbaade8` |
| Native executable | `6a02ede1a29e4b291bc90934b52e58ccfe24baaf21abb2378fea91cdea6b81b4` |
| Native safe-folder screenshot | `62b7135f269c2992394d931bf1014bb0c4c811d38773990d5f5a291673106fa5` |
| Native failed-send screenshot | `cd5c6b30d4d8b7b35eb23e1e52a89de46d7caa209eeaa473ac5fd6fdc12ed56b` |

No screenshot or partial journey result from this consumed attempt is promoted
as acceptance evidence. Attempts 001-004 remain immutable historical records.
