# P2A-W1 Native App Host RED

**Date**: 2026-07-28
**Status**: RED captured
**Contract**: `native-app-host-contract-revision.md`

## Command

```text
cd apps/macos && swift test
```

## Result

Exit status: `1`

```text
error: 'macos': Source files for target LoomLocalAppCore should be located
under 'Sources/LoomLocalAppCore', or a custom sources path can be set with the
'path' property in Package.swift
```

The tests were written before product implementation and require:

- strict direct-IPC framing and response validation;
- the complete closed Go v1 error-code set;
- strict snapshot and timeline schema decoding;
- prior-view preservation after refresh failure;
- zero-Team activation without a timeline request;
- bounded terminal/control/bidi-safe text.

No historical Cockpit source, bridge, cache, or workspace implementation was
copied. No daemon, app, Journal, Provider, Runtime, installer, or live process
was changed or activated.

## Packaging transaction RED

Commands:

```text
sh scripts/test-build-loom-local-app.sh
sh scripts/test-install-loom-local-app.sh
```

Both exited `1` before the build/install implementation existed:

```text
RED: native app builder is missing
RED: native app build/install transaction is missing
```

The tests already fixed bundle identity, arm64, signature, permissions,
reproducible executable/plist bytes, static exclusions, dry-run immutability,
atomic replacement, injected-failure restoration, rollback, and symlink
rejection.
