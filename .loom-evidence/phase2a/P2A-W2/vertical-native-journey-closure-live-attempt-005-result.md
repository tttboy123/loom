# P2A-W2 Vertical Native Journey Closure Live Attempt 005 Result

**Date**: 2026-07-30
**Lineage**: `p2a-w2-live-20260730-005`
**Candidate commit**: `715f1f42212ad5a61dabb24e74e1d3e7d53cacdb`
**Verdict**: `FAIL — PROVIDER_FACT_MISSING_AND_SHUTDOWN_STALLED`

## Preflight and materialization

The Controller revalidated the frozen attempt-002 source database as a regular
private `0600` file owned by uid `501`, SHA-256
`b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22`,
SQLite integrity `ok`, and exactly five Events:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 3
RuntimeInstanceDiscovered    | 1
```

It created the fresh private `0700` root:

```text
/Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260730-005
```

The cloned database remained byte-identical to the frozen source. The exact
post-Implementation-Review Candidate artifacts were:

```text
loomd:
31d6c73ec9a91ffb0e906413631a40e82444f44b40f711ab536f092143e87b76

LoomLocalApp:
75043fd285fd4c065f61a654851ab60f2f0b158e7e2be3a56e05e583e21013f2
```

The arm64 app signature, private modes, no-symlink attempt tree, empty
isolation root, absent product socket and exact locked Pi, Node, native Codex,
llama-server and GGUF identities passed before the one daemon start. The
separate resident daemon was frozen at PID `66336` and executable SHA-256
`e5ab283c75473619094442eb349324b2bf09b8f59250ca1b7fd8b85fffb0014a`.

## One controlled native journey

The one permitted daemon start created the private `0600` product socket and
appended exactly one model-capable `RuntimeInstanceDiscovered` Event.
Computer Use opened the exact attempt-005 bundle and observed:

- local service connected;
- two Runtime records, including the model-capable online
  `Pi 0.82.1 W2 Canary`;
- Codex `Available`;
- MiniMax `Verified`;
- Main and SubAgent role options available in Team Builder.

The Controller opened MiniMax management and activated `Test` exactly once. It
did not read, focus, retrieve, type, replace, revoke, export, log or screenshot
the secure value. After the bounded request and terminal-commit budgets, the
native state remained `Verified`, but the Journal still contained the inherited
three `ProviderCredentialVerified` Events. No new verification fact was
appended. The Test was not retried.

The Team Builder local read then reported `Timeout`; one explicit local
`Refresh` recovered it. The Controller completed the bounded question sequence
with:

```text
name    = Local Coding Team
purpose = Prepare a reviewed local code change
```

It selected the only compatible Main and SubAgent roles and activated
`Confirm saved team` exactly once. The Journal appended exactly one
`TeamDefinitionSaved` Event. The app was quit and reopened from the same exact
bundle. The restarted app reconstructed Codex `Available`, MiniMax `Verified`,
the model-capable Pi Runtime and `Local Coding Team · Active` from the
Journal/Projection. It was then quit again.

## Shutdown and cleanup

The Controller sent one normal interrupt to the exact attempt daemon. It did
not exit after 35 seconds. One exact-PID `SIGTERM` followed and the daemon
still did not exit after another bounded ten seconds. The Controller therefore
sent `SIGKILL` only to attempt PID `80523`.

The final cleanup proves:

- the app and attempt daemon are absent;
- no attempt Pi or llama-server process remains;
- the isolation root is empty;
- the unowned private product socket was removed;
- the attempt SQLite lock is an unowned empty regular private file;
- the immediate post-cleanup observation still found the separate resident
  daemon at preflight PID `66336` with the same executable hash and arguments.

Every Controller cleanup signal named only attempt PID `80523`; no Controller
command signalled or reconfigured the resident daemon, its LaunchAgent or
installed Runtime.

The first independent Result-Evidence Review later found that PID `66336` had
exited and the same launchd-managed resident lineage was running under a
different PID. A subsequent read-only Controller snapshot found launchd
reporting repeated restarts and last exit code `4`, while the executable hash
and configured arguments remained the same. The cause and exact transition are
not established by this canary evidence. Resident PID continuity beyond the
immediate post-cleanup check is therefore explicitly not claimed.

## Authoritative final state

SQLite integrity is `ok`. The final database is a regular private `0600` file
with SHA-256:

```text
e7104f121e409d057e86023b26b388f38b7aea28ee8eeb5b7101b3b4fabe688a
```

Its complete Event counts are:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 3
RuntimeInstanceDiscovered    | 2
TeamDefinitionSaved          | 1
total                        | 7
```

Relative to the five-Event source baseline, the exact delta is one
model-capable Runtime discovery and one saved TeamDefinition. There is no
TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence, dispatch or
execution Event.

## Result

The Candidate closes the stale setup failure in the real native journey:
Team Builder consumed the post-observation model-capable Runtime, saved exactly
one TeamDefinition, and recovered it after app restart. The complete W2 exit
contract nevertheless fails because:

1. the one permitted MiniMax `Test` appended no terminal verification fact;
2. the attempt daemon did not complete graceful joined shutdown and required
   exact-PID forced cleanup.

The only attempt-005 allowance is consumed. No retry, alternate executable,
second canary or new P2A-W4 is authorized. P2A-W2 remains unaccepted and
P2A-W3 remains locked pending fresh independent read-only Result-Evidence
Review.
