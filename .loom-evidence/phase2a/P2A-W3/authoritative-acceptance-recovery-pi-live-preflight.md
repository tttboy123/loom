# P2A-W3 Authoritative Acceptance/Recovery Pi Live Preflight

**Date**: 2026-08-03  
**Attempt**: `phase2a-w3-live-20260803-pi-006`  
**Manifest SHA-256**: `c765a4f48cf2185418af592e22e523d4d8348ed3bbc930399238e4c1c220586f`  
**Canonical invocation SHA-256**: `1d80ca8604a8cf9b233fbfdc2ebd41c3b5492f5cf17af680ffab8f3bb276c715`  
**Final source lock SHA-256**: `3148271012989d1ebdd00d190587f730fb691444bfae9bd4f8f29fe861c981ea`  
**Final Implementation Review SHA-256**: `5544dfc7574b62a2c548469e19f04e28eb3dcad866b39235e95b615b795133a4`  
**Status**: `PREFLIGHT PASS / UNCONSUMED`

The Pi-006 root did not previously exist. It is a user-owned, non-symlink mode
`0700` directory. Its state, manifest and evidence copies are regular mode
`0600`; Go binaries and the native executable are mode `0700`; the native
bundle is strict-signature valid, arm64, bundle ID
`com.earendilworks.loom.local`, LC_UUID
`C63B9C7D-B0C7-3AB3-A2F4-7C86B8B89DE2`. No symlink exists in the attempt.

Exact materialized hashes:

```text
initial SQLite  677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2
loomd           6ee58c70b3cebfa905ed176477faa0a82501835e07336e7d19ebe297d9807878
loom            db0c3bf9c9704e6138ba0ea190c9da7d86b4b76fe47adb1c19378f845ce15408
native          f998eabe5320eb24b3f9668222aa08a28c803cf18ef861536d0fca529e2f01b8
Pi target       af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca
Node            1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8
Codex wrapper   134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477
Codex native    29915529b97697def1a957b0505e770aa6a45744435d62fc263e98d7619e167a
llama-server    a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b
GGUF model      cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046
```

SQLite integrity is `ok` and contains the exact six retained setup facts. The
manifest and retained source-lock/review copies match the repository evidence
byte-for-byte. The manifest has exactly 32 ordered argv tokens, begins with
`--state`, contains only the frozen flag multiset (`--runtime-dir` exactly
twice, every other flag once), and has no positional subcommand. Canonical
sorted-key `{executable,args}` JSON including its final LF reproduces the frozen
invocation digest.

The default product socket and lock, start marker and any Pi-006 process are
absent; isolation is empty. The unrelated `demo-resident` daemon is present on
its own state/isolation boundary and remains untouched.

No daemon, native app, Pi, llama-server, Provider, Keychain action, preflight or
Mission Start occurred during materialization. The next action may consume
exactly one binary invocation, one product preflight and one Start; it may not
retry.
