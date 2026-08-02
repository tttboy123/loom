# P2A-W3 Authoritative Live Construction Failure Diagnosis

**Date**: 2026-08-02  
**Status**: `READ-ONLY CAUSAL DIAGNOSIS / NO RETRY / NO STATE MUTATION`

## Conclusion

Both fresh lineages failed at the same pre-socket setup-construction boundary.
The concrete incompatibility is the Codex executable identity supplied by the
live manifests:

```text
/Users/lune/Documents/Codex/devtools/npm/bin/codex
```

That path is a symbolic link:

```text
../lib/node_modules/@openai/codex/bin/codex.js
```

The target is an executable regular file at:

```text
/Users/lune/Documents/Codex/devtools/npm/lib/node_modules/@openai/codex/bin/codex.js
```

and currently has SHA-256
`134063e133f0b4244fa3b251acf973d4fe4b4aeeacbdc135211bf480f59f1477`.

## Causal path

1. `run()` accepted the non-empty `--codex-executable` value and passed it into
   `productionDaemonBuilder`.
2. `productionDaemonBuilder` constructed the Runtime observer, then called the
   product runner with `CodexExecutable` and the live local-model execution
   config.
3. `newProductDaemonRunnerWithPreparedDecisions` rebuilds Projection and calls
   `buildProductSetupService` before execution composition and before the
   product IPC server is created.
4. Because the Codex path is non-empty, `buildProductSetupService` always calls
   `provider.NewCodexNativeAuthObserver`.
5. `NewCodexNativeAuthObserver` uses `os.Lstat` and requires
   `Mode().IsRegular()` plus an executable bit. `Lstat` observes the supplied
   npm path as a symbolic link, so construction fails closed with
   `ErrInvalidCodexNativeAuthConfig`.
6. `buildProductSetupService` safely replaces the internal leaf with
   `setup native auth unavailable`.
7. `run()` currently replaces every builder error with the generic public line
   `daemon unavailable` and exit code `3`.

The absence of `state/execution`, product socket, Provider/Pi/model calls and
all Journal deltas is consistent with this exact order. The retained live
output alone did not preserve the internal error, but the frozen manifest path,
current file identity, and unconditional constructor path make the
incompatibility reproducible from read-only evidence.

## Product gaps

This exposes two coupled gaps that must be repaired together inside the unique
P2A-W3 boundary:

1. Loom accepts a normal user-level npm executable path at CLI parsing but the
   strict Codex observer rejects that same symlink during construction. The
   product must resolve and bind a canonical regular executable identity once,
   fail closed on drift, and continue rejecting non-regular final targets.
2. Safe build-stage failures are not attributable. The daemon must publish one
   stable allowlisted construction reason without raw paths or errors, while
   keeping unknown or ambiguous failures closed.

A regression gate must exercise the real `run -> productionDaemonBuilder ->
setup -> execution -> Go IPC server` construction path with a symlinked
user-level Codex launcher and the local-model execution config. Existing copied
state coverage omitted the execution config and therefore could not catch this
live compatibility boundary.

Neither consumed lineage may be retried. A future live attempt requires a new
reviewed manifest after deterministic repair and independent Implementation
Review PASS.
