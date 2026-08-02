# P2A-W3 Final Walkthrough root006 Preflight

**Date**: 2026-08-03  
**Attempt**: `phase2a-w3-walkthrough-20260803-006`  
**Verdict**: `PASS — one read-only walkthrough may start`

The manifest SHA-256 is
`93debdf1cb4219c85f3424ddeabffaea44c35b8461c3a8fad69678723272c03d`.
The exact Repair 7 source lock copy SHA-256 is
`6d258b302c800b7d484ec1e20771e77a3543ff18d55ab84d43102b06047b83cb`
and the independent Implementation Re-review copy SHA-256 is
`92d89911143a8c5f63ed6d4ca72cec9c14ae4fe386472340fd766cda060a2f0f`.

## Fresh isolated materialization

- root:
  `/Users/lune/Library/Application Support/Loom/phase2a-w3-walkthrough-20260803-006`
- root was absent before materialization, is owner uid 501, mode `0700`, and is
  neither a symlink nor contains a symlink;
- accepted Pi-006 state was copied to a new regular mode-`0600` SQLite;
- SQLite SHA-256:
  `3af352562fbc85cb47c9509096655f2cda4a05ddbb999702bc764ab90f77ed56`;
- immutable integrity is `ok` and Event count is exactly `103`;
- isolation and evidence directories are private and empty except for locked
  preflight inputs;
- the default product socket and socket lock are absent;
- no root006 process or open SQLite handle exists;
- unrelated `demo-resident` remains running on its own state boundary.

## Current artifacts

```text
dc8172acef0c867760c970521b680702f79fe0dfb7c6a81319ef3e918b91c205  bin/loom
8be1487b2b868d1e4ce17a5606d1c72b79183dd8f98d443eaf29033c2492e078  bin/loomd
96f6d93f944921ee3e31e6f47c2ce710573e7914493f2e371056d08ad0e9e01a  native/Loom.app/Contents/MacOS/LoomLocalApp
```

The native bundle is strict-signature valid, arm64, bundle ID
`com.earendilworks.loom.local`, and LC_UUID
`DB497D0C-9EF3-3568-82A5-947953A07F0F`.

Exactly one daemon start, one native launch and one TUI launch are now allowed.
Only read-only navigation and one unchanged observation cycle are permitted.
No objective, preflight, Start, Provider Test, approval, recovery, cancel,
terminal, credential or Provider request is authorized.
