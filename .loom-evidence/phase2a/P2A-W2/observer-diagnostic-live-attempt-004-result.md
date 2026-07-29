# P2A-W2 Observer Diagnostic Live Attempt 004 Result

**Date**: 2026-07-30
**Lineage**: `p2a-w2-live-20260730-004`
**Candidate commit**: `d0959e69d0de31bdcab4bbfad7a29a801d7f6972`
**Verdict**: `FAIL — NATIVE_JOURNEY_INCOMPATIBLE_AND_SHUTDOWN_STALLED`

## Preflight and materialization

The Controller used the frozen attempt-002 five-Event source database only
after revalidating its regular-file identity, private owner/mode, SHA-256,
SQLite integrity, schema and Event metadata. It did not read, print, copy or
materialize the credential reference or secret.

The fresh attempt root was:

```text
/Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260730-004
```

The root and child directories were private `0700`; the cloned SQLite database
and evidence inputs were regular `0600` files. The exact newly built artifacts
were:

```text
loomd:
298c11a1a2c4ae3dc8492c07d9202345611fb43987e46b230f6fd8d2be5930ea

LoomLocalApp:
8b12670b30aeef25959f4acd024967655b3bf688ccb456d21004ca31d4eddbc7
```

The frozen Pi, canonical Node, Codex native binary, llama-server and GGUF
identities passed before the one permitted daemon start. The separate resident
daemon remained running at its own no-socket lineage and was not signalled,
reconfigured or otherwise mutated.

## Ordinary native journey

The one daemon start created the private product socket and appended exactly
one new `RuntimeInstanceDiscovered` Event. Computer Use opened the exact
attempt-004 native bundle and observed:

- local service connected;
- two Runtime entries, including `Pi 0.82.1 W2 Canary` online;
- Codex `Available`;
- MiniMax `Verified`;
- the Pi Runtime exposed in Team Builder.

The Codex result is narrow live proof for the repaired observer compatibility
boundary: the locked Codex 0.144.1 process passed the production observer and
the native UI showed `Available`. The prior diagnostic and deterministic
stderr-specific tests bind that result to the stdout-or-stderr compatibility
repair while retaining the dual-stream, multiline, unknown-text and identity
rejections. The live evidence does not independently retain the child output,
prove a model request, or broaden Provider behavior. No login or credential
mutation occurred.

The Controller selected MiniMax `Manage` and `Test` exactly once. It did not
focus, read, type, replace, revoke, export or screenshot the secure field. The
visible state remained `Verified`, but the final Journal contained no new
`ProviderCredentialVerified` Event. The subsequent bounded Team Builder read
first returned `Timeout`; one native `Refresh` recovered the setup snapshot.

The Controller then selected `Create Candidate team` exactly once. The product
returned `Incompatible` before presenting the first bounded question. No
TeamDefinition was confirmed or appended. The native app was quit, reopened
from the same exact bundle, and again reconstructed Codex `Available`, MiniMax
`Verified`, the online Pi Runtime and an empty saved-Team list. It was then
quit again.

## Shutdown and postconditions

The attempt daemon closed its listener after the cleanup signal, but did not
exit after bounded `SIGINT` and `SIGTERM` waits. The Controller therefore sent
`SIGKILL` only to the exact attempt-004 daemon PID. The app and attempt daemon
were absent afterward; the separate resident daemon remained running.
Unowned socket and lock artifacts left by the killed attempt were verified to
have no open file owner and removed from the exact product run directory.

Post-run SQLite integrity was `ok`. The final database was a regular private
`0600` file with SHA-256:

```text
5fa9eb4f3f8e836f9ea9421ecd93a7153213f9d01533453922d8cbb94d3f203e
```

Its complete Event-type counts were:

```text
ProviderCredentialConfigured | 1
ProviderCredentialVerified   | 3
RuntimeInstanceDiscovered    | 2
total                        | 6
```

Relative to the inherited five-Event baseline, the only delta was one
`RuntimeInstanceDiscovered` Event on
`runtime_instance:runtime.pi.earendil-works.0.82.1` at sequence `1`.
There was no TeamDefinition, TeamInstance, AgentInstance, WorkItem, Run, Grant,
Evidence, dispatch or execution Event. The isolation root was empty, the
product socket and lock were absent, and no attempt app, daemon, Pi or
llama-server process remained. The database lock file was a private empty
regular file left inside the disposable attempt state directory.

## Result

The observer stderr compatibility repair is live-proven at the narrow native
availability boundary and remains fail-closed. The complete replacement
journey does not satisfy the frozen W2 exit contract because:

1. the single MiniMax `Test` did not append the required new verification fact;
2. Candidate creation stopped at `Incompatible`, so no one-question flow or
   confirmed TeamDefinition occurred;
3. the daemon required forced termination after bounded graceful shutdown.

The sole replacement allowance is consumed. No retry, alternate executable,
additional single-point amendment or component execution B is authorized.
P2A-W2 remains unaccepted, P2A-W3 remains locked and no P2A-W4 exists. Fresh
independent read-only Result-Evidence Review is the next gate.
