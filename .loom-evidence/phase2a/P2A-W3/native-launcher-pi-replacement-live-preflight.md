# P2A-W3 Native Launcher Pi Replacement Live Preflight

**Date**: 2026-08-02  
**Attempt**: `phase2a-w3-live-20260802-pi-005`  
**Manifest SHA-256**:
`6eb40813739faff661b3e93dfd7b5dea395bc11d903a0a118186494b479a4d92`  
**Canonical invocation SHA-256**:
`d91c670f9e0be2695e26c6175a34acb44d2c60e92d92d62bc55fde386a115a65`  
**Status**: `PREFLIGHT PASS / UNCONSUMED`

The invocation Amendment SHA-256 is `f1ab059b...deb6a42`; fresh independent
Amendment Review SHA-256 is `a12484ed...bd268f` and verdict is `PASS` with no
P0/P1/P2.

The new attempt root did not previously exist. It is a user-owned non-symlink
directory mode `0700`; state, manifest and evidence files are regular mode
`0600`; binaries are mode `0700`; the bundle contains no symlink. The default
product socket/lock and start marker are absent, and no `-005` process exists.

Independent preflight reproduced:

```text
initial SQLite  677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2
loomd           e48d898485ecd23aa6e8c8f30c80cc6f7094365e460b800b1713d667164a8d0e
loom            e756a7ed80550995ee57122f5e2d8dd4ac6a0bef20b61dc2f878e8effe3ab5d5
native          f998eabe5320eb24b3f9668222aa08a28c803cf18ef861536d0fca529e2f01b8
Codex wrapper   134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477
Codex native    29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a
llama-server    a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b
GGUF model      cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046
```

SQLite integrity is `ok` and contains the exact six retained facts. The native
bundle is strict-signature valid, arm64, bundle ID
`com.earendilworks.loom.local`, LC_UUID
`C63B9C7D-B0C7-3AB3-A2F4-7C86B8B89DE2`.

The manifest contains exactly 32 ordered argv tokens. Its first token is
`--state`; the 16 flag positions contain only the frozen allowlist, with
`--runtime-dir` exactly twice and every other flag exactly once. Canonical
sorted-key `{executable,args}` JSON including the final LF reproduces
`d91c670f...115a65`. There is no `daemon` token or other positional argument.

No daemon, UI, Pi, model, Provider, Keychain action, preflight or Start occurred
during materialization. The next action may consume exactly one binary
invocation and may not retry.
