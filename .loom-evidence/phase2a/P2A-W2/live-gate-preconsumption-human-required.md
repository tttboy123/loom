# P2A-W2 Live Gate Pre-consumption Result

**Date**: 2026-07-30
**Status**: HUMAN_REQUIRED — live allowance unconsumed
**Attempt**: `p2a-w2-live-20260729-001`

## Result

The reviewed W2 implementation and both reviewed pre-consumption compatibility
corrections reached the native credential hand-off boundary:

- exact isolated `loomd` SHA-256:
  `1d06d2696272e7560fb86221516daa52d5b8ca0d0cc233c3d5966e3d08aa2180`;
- exact reviewed Pi 0.82.1 target SHA-256:
  `af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca`;
- exact reviewed Node target SHA-256:
  `1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8`;
- exact reviewed Codex native target SHA-256:
  `29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a`;
- exact addressable `Loom.app` executable SHA-256:
  `1f8408a396fbf31c4e15c3561b79ef4df053222b37874ccf187f7e547a07f5e0`;
- bundle identifier: `com.earendilworks.loom.local`;
- bundle architecture: `arm64`;
- bundle signature: strict local verification `PASS`;
- bundle symlink count: `0`;
- socket: exact AF_UNIX path, user-owned, mode `0600`;
- SQLite: user-owned regular file, mode `0600`;
- resident accepted Runtime observer: preserved and not restarted by the
  attempt.

The native Team Builder was independently addressable through Computer Use and
displayed:

```text
Providers
Codex · Unsupported · native_auth
MiniMax Unconfigured · brokered
MiniMax API key
Store securely (disabled)
Available runtimes
No compatible runtime is available.
Create Candidate team
```

The SecureField remained empty and `Store securely` remained disabled across
three consecutive goal turns. The user did not complete the required direct
credential entry and submission inside the native product.

## Authority evidence

Immediately before controlled shutdown:

```text
Journal event count = 1
RuntimeInstanceDiscovered = 1
all other Event types = 0
```

The sole Event belongs to:

```text
stream_id = runtime_instance:runtime.p2a-w2-live.pi.0.82.1
sequence  = 1
```

No credential, Provider test, TeamDefinition, TeamInstance, AgentInstance,
WorkItem, Run, Grant, Evidence, dispatch or generation Event exists. No Team
Candidate was confirmed and no execution was created.

Because the user never activated `Store securely`:

- the live allowance is unconsumed;
- no MiniMax credential was stored;
- no MiniMax network request was made;
- no Keychain mutation or credential Journal fact occurred;
- no secret entered source, process arguments, prompt, log, Journal, Evidence,
  AgentDefinition, screenshot or result artifact;
- the previously chat-pasted secret remained prohibited and unused.

## Controlled shutdown

The Team Builder sheet was dismissed, the addressable Loom window was closed,
and the Loom app quit through Computer Use. The isolated daemon received
`SIGINT` through its owned controlling session and exited normally with:

```json
{
  "completed_cycles": 15,
  "discovery_events": 1,
  "status_events": 0,
  "no_write_cycles": 14,
  "runtime_facts": [
    {
      "runtime_instance_id": "runtime.p2a-w2-live.pi.0.82.1",
      "executable_version": "0.82.1",
      "status": "online",
      "model_ids": null,
      "discovery_sequence": 1,
      "status_sequence": 0
    }
  ]
}
```

Post-shutdown:

- the attempt Loom process is absent;
- the attempt daemon process is absent;
- the product socket is absent;
- the isolated SQLite and attempt root are retained for read-only
  Result-Evidence Review;
- the accepted resident Runtime observer remains outside this attempt.

## Gate

```text
P2A-W2 deterministic implementation = REVIEWED CHECKPOINT
P2A-W2 controlled live result       = HUMAN_REQUIRED (unconsumed)
P2A-W2 acceptance                    = NOT ACCEPTED
P2A-W3                               = LOCKED
P2A-W4                               = DOES NOT EXIST
```

The goal may resume only after the user is available to type a genuinely fresh
MiniMax credential into the native Loom SecureField and personally activate
`Store securely`. Resumption requires a newly reviewed bounded live-attempt
lineage because the addressable app and daemon were cleanly stopped; it may not
reuse a chat value, environment value, hidden retry or this closed process
lineage.
