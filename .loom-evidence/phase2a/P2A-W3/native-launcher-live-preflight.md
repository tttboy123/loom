# P2A-W3 Native Launcher and Build Transaction Live Preflight

**Date**: 2026-08-02  
**Implementation source-lock SHA-256**:
`f6bb68656cd70b2c9d75861bc5a77325ea488bd78dcfa49fc0c1cc41dd64d772`  
**Implementation Re-review 2**: `PASS`, SHA-256
`fa524c201f97a1629dc03f25c047f718e4ed3f724b0c82b64dcc20cbc09d99eb`  
**Status**: `TWO FRESH MANIFESTS FROZEN / UNCONSUMED`

## MiniMax lineage

```text
attempt: phase2a-w3-live-20260802-minimax-004
manifest: 889350d951fd9f7b3285a5500df9ddb5d78a7b1cf5701934b5d0d17b778c75d4
initial SQLite: b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22
facts: ProviderCredentialConfigured=1, ProviderCredentialVerified=3,
       RuntimeInstanceDiscovered=1
latest credential revision: 4 verified
daemon: e48d898485ecd23aa6e8c8f30c80cc6f7094365e460b800b1713d667164a8d0e
loom: e756a7ed80550995ee57122f5e2d8dd4ac6a0bef20b61dc2f878e8effe3ab5d5
native executable: f998eabe5320eb24b3f9668222aa08a28c803cf18ef861536d0fca529e2f01b8
local-model flags: exactly zero
```

## Pi lineage

```text
attempt: phase2a-w3-live-20260802-pi-004
manifest: 0b76124d920abdd2eac8fc9c1e1ce05e7c8f66547e074eefbabc0442ede0feaa
initial SQLite: 677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2
facts: exactly six, including saved-Team, Main Agent and Pi Runtime
daemon: e48d898485ecd23aa6e8c8f30c80cc6f7094365e460b800b1713d667164a8d0e
loom: e756a7ed80550995ee57122f5e2d8dd4ac6a0bef20b61dc2f878e8effe3ab5d5
native executable: f998eabe5320eb24b3f9668222aa08a28c803cf18ef861536d0fca529e2f01b8
```

Both roots are new, user-owned, non-symlink directories mode 0700. SQLite,
manifest, source-lock and review copies are regular user-owned mode-0600 files.
SQLite integrity is `ok` and the exact initial fact sets match. Both bundles
are arm64, strict-signature valid, contain no symlink, have bundle ID
`com.earendilworks.loom.local` and LC_UUID
`C63B9C7D-B0C7-3AB3-A2F4-7C86B8B89DE2`.

Pi, Node, official npm Codex wrapper/native binary and locked local
llama-server/model identities match the manifests and reviewed source lock.
The default product socket and lock are absent. No `-004` attempt process is
running. The unrelated pre-existing `demo-resident` observer is excluded and
must not be signalled or modified.

The attempts are strictly sequential. MiniMax permits one daemon start, one
native Test and at most one Provider request; it omits all local-model flags and
therefore cannot initialize Mission execution. Only after complete MiniMax
shutdown may Pi consume one daemon start, one preflight and one explicit Start,
with zero network Provider requests. Neither lineage permits retry or
compaction. A failure consumes that lineage.
