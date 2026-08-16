# Phase 2C Cross-Client Journey Attempt 002

**Result**: `FAIL_PRE_ACCEPTANCE`  
**Journey ID**: `1a05dbc3-991a-4dab-9aa2-42c8601df464`  
**Source lock SHA-256**: `43cce64c18d262cbaeb04c0f0da8f91148261b5a752c05a0e61ab6d92d507356`

## Failure

Repair 3 made the advertised Home `i` action operational in the real PTY, but
typing a sentence removed every space. Bubble Tea emits a physical space key as
`tea.KeySpace`; the shared TUI entry handler accepted only `tea.KeyRunes`.
Existing tests sent a synthetic multi-rune message containing spaces and did
not reproduce the terminal event sequence.

The observed draft was
`Summarizethetaskentryflowinoneshortsentence.` instead of the typed sentence.
This affects every TUI text-entry mode, including chat, folder, Mission,
builder, and search input. Attempt 002 therefore stopped before sending the
message or continuing J1/J4.

Repair 4 must add a physical `tea.KeySpace` RED, preserve bounded entry limits,
accept spaces through the shared entry handler, pass independent review,
regenerate the source lock, and rerun deterministic verification before a
replacement journey.

## Observed State

- Native App and real PTY TUI both opened chat-first against one clean daemon.
- Home `i` entered the draft, proving Repair 3 itself was active.
- No `chat_message`, Team Draft, Team, Mission, or Run mutation occurred.
- Journal remained at the three expected initialization/runtime facts.
- Duplicate Event IDs: `0`; duplicate idempotency keys: `0`; SQLite integrity:
  `ok`.
- Native App, TUI, daemon, and socket were all closed after the failure.

## Artifact Digests

| Artifact | SHA-256 |
|---|---|
| Native full-display checkpoint | `ab2c79b8405b01594419710d51ff425ffed77263715a6c85509d7887cd692039` |
| Real PTY transcript | `141b96cf20c842c6f1a2182e2bd65e97b9ea7ae5a72373fc0218f3787c970907` |
| Sanitized IPC summary | `5ffd8f1aeec6cbb95a874d46f73a09503cd98bbe85d2c7cad352425b94e031d7` |
| Sanitized daemon log | `b7d41f1298fd00f1e301f9d76a1ef9a8b539d6a6a5d4b321d9e19fef4af24364` |
| Closed SQLite state | `2372dd8084d6bc55090e1a5db738023ac98e3185a3ad954a9ff329ea661e3cd9` |

