# P2A-W2 Live Gate Native Bundle Materialization Correction

**Date**: 2026-07-29
**Status**: REVIEWED — fresh independent Contract Review PASS
**Parent**: reviewed P2A-W2 Live Gate and Deterministic Checkpoint Commit
Amendment

## 1. Pre-consumption presentation result

The corrected daemon startup passed and remains the sole isolated W2 product
daemon. It created the exact private socket and appended only the expected
isolated `RuntimeInstanceDiscovered` Event.

The frozen manifest then launched the copied SwiftPM release executable at:

```text
<attempt_root>/bin/LoomLocalApp
```

The executable created a native process, but a bare SwiftPM executable has no
`CFBundleIdentifier`. LaunchServices and the approved Computer Use boundary
therefore cannot independently address its window. Read-only LaunchServices
inspection returned:

```text
CFBundleIdentifier    = NULL
LSDisplayName         = LoomLocalApp
CFBundleExecutablePath = <attempt_root>/bin/LoomLocalApp
```

The process was terminated without UI interaction. No SecureField was focused,
no credential was entered or stored, no Keychain or Provider action occurred,
and no authoritative Event beyond the expected Runtime observation exists.
The sole live allowance remains unconsumed.

## 2. Existing accepted product boundary

P2A-W1 already accepted the reproducible native host:

```text
scripts/build-loom-local-app.sh
bundle_id = com.earendilworks.loom.local
bundle    = Loom.app
```

That builder:

- compiles the current reviewed Swift sources for arm64 release;
- uses the reviewed `apps/macos/Resources/Info.plist`;
- creates a new no-symlink `Loom.app`;
- assigns `0700` directories and executable, `0600` ordinary files;
- strips debug symbols, applies a local ad-hoc signature, and verifies it;
- accepts only a fresh absolute output ending in `/Loom.app`;
- performs no install, daemon mutation, Provider request or authority write.

The W2 live manifest incorrectly selected the intermediate SwiftPM executable
instead of the accepted independently addressable native product host.

## 3. Exact correction

Without restarting or replacing the running W2 daemon, the same unconsumed
attempt may:

1. create the fresh user-owned non-symlink directory
   `<attempt_root>/native` with mode `0700`;
2. run exactly:

   ```text
   scripts/build-loom-local-app.sh \
     --output <attempt_root>/native/Loom.app
   ```

3. record and verify the complete bundle manifest, executable SHA-256, bundle
   identifier, arm64 architecture, owner, modes, absence of symlinks and strict
   code-signature validity;
4. launch exactly `<attempt_root>/native/Loom.app` through the approved
   Computer Use boundary;
5. use that one addressable window for the already frozen W2 journey.

The existing bare executable remains retained as build evidence but may not be
launched again. The accepted bundle builder must use the current reviewed
source tree; any source drift, pre-existing output, build failure, manifest
mismatch, invalid signature or unexpected Event stops before bundle launch.

## 4. Authority and consumption preservation

- This is a W2 presentation/materialization correction, not W3 or W4.
- It changes no product source, daemon, socket, database, StateWriter, Journal,
  Projection, Provider, Keychain, Team or execution authority.
- It does not restart the corrected daemon and does not permit another daemon
  startup.
- The non-addressable raw process performed no UI action and did not consume
  the canary. The sole allowance remains consumed only when the user enters a
  fresh secret and activates `Store securely`.
- The user must still type and submit the secret personally. Computer Use may
  position the app but may not read, type, paste, retrieve or submit the
  credential.
- All one-request, TeamDefinition-only, revocation, Result-Evidence Review,
  cleanup, no-hidden-retry and secret-negative evidence rules remain
  unchanged.
- If the addressable bundle cannot launch or expose the frozen journey, the W2
  live result stops without another app build or launch.

Fresh independent Contract Review `PASS` is required before bundle build or
launch.
