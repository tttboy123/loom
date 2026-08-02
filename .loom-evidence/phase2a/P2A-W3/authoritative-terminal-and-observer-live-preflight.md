# P2A-W3 Authoritative Terminal and Observer Live Preflight

**Date**: 2026-08-02  
**Implementation source-lock SHA-256**:
`9ddf62b829760fa4c0b10f2488f40b008dc423ecee1cbea906e77292419430a8`  
**Implementation Re-review 3**: `PASS`  
**Status**: `TWO FRESH MANIFESTS FROZEN / UNCONSUMED`

## MiniMax lineage

```text
attempt: phase2a-w3-live-20260802-minimax-003
manifest: f9d220345fa578dee316d5e30cc39178540a28941796e4cb342a018790b031d5
initial SQLite: b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22
facts: ProviderCredentialConfigured=1, ProviderCredentialVerified=3
latest credential revision: 4 verified
daemon: 49f3db211527ff951c5d75c615ed34086ceaf80a9b1a1ddf482878e3999ba8af
loom: 54081d2438e29db83692decd5f485b3331db38e8cad027fceeb01815ce1c19be
native executable: f998eabe5320eb24b3f9668222aa08a28c803cf18ef861536d0fca529e2f01b8
```

## Pi lineage

```text
attempt: phase2a-w3-live-20260802-pi-003
manifest: 4c8240e787c85ceb53bf5aeee957874d3040748f25ed2fbf30e8bb62d07e44ea
initial SQLite: 677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2
facts: exactly six, including saved-Team, one Main Agent and Pi Runtime
daemon: 49f3db211527ff951c5d75c615ed34086ceaf80a9b1a1ddf482878e3999ba8af
loom: 54081d2438e29db83692decd5f485b3331db38e8cad027fceeb01815ce1c19be
native executable: f998eabe5320eb24b3f9668222aa08a28c803cf18ef861536d0fca529e2f01b8
```

Both attempt roots are fresh, user-owned, non-symlink and mode 0700. SQLite and
manifest copies are regular user-owned mode-0600 files. Both native bundles are
strict-signature valid, arm64, bundle ID `com.earendilworks.loom.local`, with
LC_UUID `C63B9C7D-B0C7-3AB3-A2F4-7C86B8B89DE2`. Pi 0.82.1, Node, llama-server
and local-model hashes match the manifests. SQLite integrity is `ok`.

The default product socket and lock are absent. No attempt process is running.
The unrelated pre-existing `demo-resident` observer is visible and excluded;
it must not be signalled or modified.

The attempts are sequential. MiniMax permits one daemon start, one native Test
and at most one Provider request. Pi remains unstarted until MiniMax shutdown.
Pi permits one daemon start, one preflight and one Start, with zero network
Provider requests. Neither lineage permits retry or compaction.
