# P2A-W2 Live Gate Resumption Attempt 002

**Date**: 2026-07-30
**Status**: REVIEWED — fresh independent Contract Review PASS
**Parent result**: `HUMAN_REQUIRED (allowance unconsumed)`
**Product Owner request**: `帮我打开对应的窗口`

## 1. Purpose

The prior attempt reached the user-only SecureField hand-off, consumed no live
allowance, passed Result-Evidence and cleanup review, and was recoverably
removed from the live Application Support paths.

This bounded lineage authorizes one new isolated daemon and one addressable
native Loom window solely to restore that same SecureField hand-off. It does
not accept W2, unlock W3, create W4, or authorize Controller entry of any
credential.

## 2. Exact lineage

```text
attempt_id   = p2a-w2-live-20260730-002
attempt_root = /Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260730-002
state_path   = <attempt_root>/state/loom.db
isolation    = <attempt_root>/isolation
daemon       = <attempt_root>/bin/loomd
bundle       = <attempt_root>/native/Loom.app
socket       = /Users/lune/Library/Application Support/Loom/run/loomd.sock
```

The attempt root, all child directories and the initially absent socket parent
must be freshly created, user-owned, non-symlink and mode `0700`. Ordinary
files and SQLite must be regular, user-owned, non-symlink and mode `0600`.
Executables must be regular, user-owned, non-symlink and mode `0700`. The
private AF_UNIX socket must be user-owned and mode `0600`.

The exact repository identity is:

```text
workspace = /Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild
branch    = codex/loom-platform-slice2
HEAD      = bee8ee63b692f7afcacef36b0cf4ab152290c803
```

Product source and tests must remain byte-identical to the committed W2
checkpoint. Pre-existing unrelated worktree changes remain untouched.

## 3. Reviewed compatibility inputs

The attempt reuses the already reviewed exact targets:

```text
Pi 0.82.1 SHA-256 =
af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca

Node arm64 SHA-256 =
1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8

Codex native arm64 SHA-256 =
29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a
```

The daemon receives the reviewed two-directory ordered Runtime search path,
the reviewed native Codex executable, a credential-clean minimal environment,
and no Provider key, base URL, proxy override, chat value or ambient Loom
state.

The accepted W1 `scripts/build-loom-local-app.sh` boundary builds the exact
addressable `com.earendilworks.loom.local` arm64 bundle under this fresh
attempt. No installed app, LaunchAgent or resident Runtime observer is
replaced or restarted.

## 4. Permitted transition

After Contract Review `PASS`, the Controller may exactly once:

1. create the fresh attempt layout and copy this manifest and its Review;
2. build `loomd` with `-trimpath -buildvcs=false`;
3. build one fresh `Loom.app` with the accepted W1 bundle builder;
4. reproduce source identity, hashes, permissions, signature, architecture,
   no-symlink and zero-Event preflight;
5. start the isolated daemon with the reviewed dual Runtime search paths;
6. require only expected isolated Runtime discovery/status Events;
7. launch the exact fresh bundle through Computer Use;
8. open Team Builder and stop at the visible MiniMax SecureField.

The Controller may position or raise the window but may not read, retrieve,
type, paste or submit a credential.

## 5. Credential boundary

Every credential pasted into chat remains prohibited, including the value the
user most recently asked the Controller to configure. User acceptance of
disclosure risk does not authorize the Controller to bypass the reviewed
Keychain/Broker or Computer Use hand-off boundary.

Only a credential that was never sent to chat may be entered. The user must
personally:

1. focus the native SecureField;
2. type or paste the non-chat credential;
3. activate `Store securely`.

Until that user action occurs:

- the live allowance remains unconsumed;
- no Keychain or credential Event is permitted;
- no Provider request is permitted;
- no Team Candidate confirmation is permitted.

If a chat-pasted credential is knowingly used, if the Controller enters or
submits any credential, or if an unexpected Event/process/socket/identity
appears, the attempt stops and cannot be used as W2 acceptance evidence.

## 6. Stop rule

This lineage authorizes one daemon start, one bundle build and one addressable
bundle launch. Failure before SecureField hand-off stops without an alternate
path. After hand-off, the prior reviewed W2 one-request, TeamDefinition-only,
revocation, secret-negative Result-Evidence Review and cleanup rules apply.
