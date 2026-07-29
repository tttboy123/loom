# P2A-W1 Native App Host Live Pre-Bootstrap Repair 2 RED

Date: 2026-07-28  
Status: `RED — LIVE ALLOWANCE UNCONSUMED`

## Trigger

After Live Pre-Bootstrap Repair 1 Re-review `PASS`, a final isolated Candidate
rebuild produced stable Go hashes but a different native executable hash from
the earlier reviewed build. A second fully clean Swift rebuild produced another
different executable hash.

No app or daemon installation, bootout, bootstrap, UI launch, or live mutation
occurred.

## Root cause

Byte comparison proved two independent nondeterministic sources:

1. the Mach-O `LC_UUID` changes on every clean link;
2. local N_OSO symbol-table entries carry current object-file timestamps.

Removing only `LC_UUID` and automatic ad-hoc signing left six timestamp bytes
different. Applying `strip -S` to those unsigned, UUID-free executables removed
the non-runtime N_OSO records and made the two clean builds byte-for-byte
identical.

The existing builder fixture built twice from the same SwiftPM release cache,
so it reused one linked executable and did not prove clean reproducibility.

## Mandatory RED

`scripts/test-build-loom-local-app.sh` now runs `swift package reset` before
each of its two bundle builds and still requires exact executable hashes.
Against the pre-repair builder, the focused test fails because the two clean
executables differ.

The repair must:

- disable linker UUID generation;
- disable linker automatic ad-hoc signing before package assembly;
- remove non-runtime debug/N_OSO symbols with the Apple `strip` tool;
- perform the existing final bundle ad-hoc signature only after stripping;
- preserve strict signature, arm64, owner/mode, no-symlink, bundle-ID, and
  secret-negative checks;
- prove two complete clean builds produce identical signed executables and
  bundle manifests.

## Authority and state

This remains inside the frozen Native App Host builder/test owned files. It
does not alter app behavior, IPC, daemon, Journal, Projection, StateWriter,
Runtime, Provider, or authority. Candidate bootstrap count is `0`; the native
allowance remains `1`, unconsumed; original resident bytes and service remain
unchanged.
