# P2A-W3 Complete Live Codex Replacement Result

**Date**: 2026-08-02  
**Attempt**: `phase2a-w3-live-20260802-codex-002`  
**Manifest SHA-256**:
`7f00efcf01f20c70f1aec5a9cda6e14e7581afd791a2d361c3b44596a7df5e87`  
**Allowance**: consumed once; no retry  
**Verdict**: `PASS`

## Preflight

- parent source lock `03d213...3ce`, closure source lock
  `904b1c...0ccd` and Implementation Review 4 `PASS` matched;
- private attempt root was user-owned `0700`, copied authoritative SQLite and
  manifest were regular user-owned `0600`;
- attempt `loomd`, `loom`, signed arm64 native executable, Codex and deterministic
  timeout-fixture identities matched the frozen manifest;
- the copied initial SQLite matched the frozen
  `677624b6...e1a2` source byte-for-byte and had integrity `ok`; and
- the default product socket and lock were absent before the only daemon start.

## Controlled live result

The daemon used a deterministic Pi metadata fixture: `--version` returned
`0.82.1`, while the only `--list-models` invocation exceeded the exact `2s`
process timeout. Its invocation counter reached exactly `1`. More than three
seconds after that timeout, the same daemon and private UDS remained alive.

Computer Use opened the exact attempt-native `Loom.app`. The ordinary
`Runtime & Providers` product sheet displayed:

```text
Codex   Available
MiniMax Unconfigured
```

The main workspace simultaneously displayed:

```text
Partial authoritative view · partial_view
```

The ordinary New Mission sheet selected the saved human Team name
`P2AW3ControlledTeam`. With objective
`Verify Codex native auth while Pi metadata is partial`, one read-only product
preflight returned:

```text
1 node · native_auth · qwen2.5-coder-1.5b-instruct-q4-k-m
Capacity available: 1
Preflight is ready. Starting still requires your click.
```

No Start, Codex execution, MiniMax request, Runtime execution, credential
mutation, direct IPC command or SQLite mutation occurred.

## Shutdown and authoritative postflight

The native app quit normally. One exact interrupt stopped the single controlled
daemon with exit `0` and:

```json
{"completed_cycles":0,"discovery_events":0,"status_events":0,"no_write_cycles":0,"runtime_facts":[]}
```

The final SQLite remained byte-identical to its frozen initial state:

```text
677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2
```

Integrity is `ok`; its complete six facts remain one each of
`AgentGrantIdentityIndexInitialized`, `WorkRunIdentityIndexInitialized`,
`RuntimeInstanceDiscovered`, `TeamDefinitionSaved`, `TeamInstanceCreated` and
`AgentInstanceCreated`. There is no Mission, WorkItem, Run, Grant, Evidence or
execution fact. The product socket/lock are absent, isolation is empty, the
attempt processes are absent and the retained non-state surface contains no
credential/token pattern.

**VERDICT**: `PASS — CODEX AVAILABLE / PI TIMEOUT CONTAINED / PREFLIGHT READY`
