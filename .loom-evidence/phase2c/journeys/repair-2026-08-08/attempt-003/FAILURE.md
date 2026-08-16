# Phase 2C Repair Journey Attempt 003 Failure

**Date**: 2026-08-08  
**Journey ID**: `4881ef2d-215e-43b7-85b9-309bddc48b33`  
**Fixture root**: `/private/tmp/loom-phase2c-chat-20260808-003`  
**Source-lock SHA-256**: `6b252744ed8d52df4edd3a0c71fbae7db82fe85655c4aeaaa35d25bf79f2d6fc`  
**Outcome**: `FAIL - CHAT_MESSAGE_DEADLINE_TOO_SHORT`

## Failure

The Repair 4 physical-space closure passed in the real PTY. After Home `i`, the
exact input rendered with every physical space preserved:

```text
Draft · Summarize the task entry flow in one short sentence.
```

On Enter, TUI showed `Starting local service...` and then
`State unavailable · timeout`. Refresh returned an empty chat thread, so J1 was
not accepted and J2-J10 were not attempted.

The controlled daemon log proves the cause. `chat_message` request
`loom-client-2` was received at monotonic offset `89819010` microseconds and its
successful response was produced at `95089503`, a duration of `5270493`
microseconds. The Go local IPC server and TUI client both use a five-second
deadline for this method. The response therefore arrived after the connection
and client deadlines: the daemon recorded `response_ok=true`, while the TUI
received `local IPC timeout` and did not publish the returned thread.

This is a first-use local-model cold-start defect, not a Team, Mission, Journal,
or authority transition. Repair 5 must give `chat_message` the existing bounded
extended request window consistently across server, Go/TUI client, and native
client, with tests proving ordinary methods retain the five-second bound.

## Preflight Notes

Before the daemon opened a socket, its private-directory checks rejected the
new fixture first at `build_observer` and then at `build_ipc`. The fixture was
corrected to `0700` directories and `0600` harness files. At those points the
Journal contained zero events and no socket existed, so these fail-closed
preflight checks did not consume or replace the journey. Their logs remain at
the fixture root.

## Authority and Shutdown Audit

- Final Journal events: exactly three:
  `WorkRunIdentityIndexInitialized`, `AgentGrantIdentityIndexInitialized`, and
  `RuntimeInstanceDiscovered`.
- Team events: `0`.
- Mission and execution Run facts: `0`.
- Duplicate event IDs: `0`.
- SQLite `PRAGMA integrity_check`: `ok`.
- TUI exited normally, Native received `TERM`, daemon received `SIGINT` and
  exited `0`.
- Related processes after shutdown: `0`.
- Journey socket after shutdown: absent.

## Artifact Bindings

| Artifact | SHA-256 |
|---|---|
| `tui/transcript.txt` | `cc7f0bc50bfe20cc6b764b74946b838d6094c365dbccfb5bdd7d4a6b49cf6b92` |
| `ipc/request-response-summary.jsonl` | `eeddd3fdaf15cbe1f447ee83256c28886b2de86c018dbc501942f111f6263999` |
| `daemon/structured-log.jsonl` | `c637d7aee8c511fc450375af136e9e6dd5421a03d7140224c29d2c5752c32e3f` |
| `state/loom.db` | `a9e8723b8aba546d95468823ca6bf6896b23dc6b70cd94c20b4f995099eaa5f1` |
| `source/source-lock.json` | `6b252744ed8d52df4edd3a0c71fbae7db82fe85655c4aeaaa35d25bf79f2d6fc` |
| `bin/loom` | `df5e230664d1c65a46523e25b719d647b8d2f05a1d4690dab2d87de88de31305` |
| `bin/loomd` | `3b4b0a142f4b6cadcd417abc83d1266333e9e82efc8550c8a4492727e712a999` |
| Native executable | `cb3134e9353ff9c1d237fb8554ddf8db743155ca4868062b280961273624116c` |

No screenshot from this consumed failed attempt is promoted as acceptance
evidence. Attempts 001, 002, and 003 remain immutable historical records.
