# P2A-W3 Native Launcher and Build Transaction Causal RED

**Date**: 2026-08-02  
**Status**: `CAUSAL RED / LOCKED-BOUNDARY CONFLICT / HUMAN_REQUIRED`

## Initial RED

Before production changes, focused tests failed only on the missing frozen
symbols:

```text
undefined: ResolveCodexNativeExecutable
undefined: newDaemonBuildFailure
```

After the minimal official npm launcher resolver and safe build-reason plumbing
were added, their focused tests passed:

```text
ok loom-pi-rebuild/internal/provider
ok loom-pi-rebuild/cmd/loomd
```

The resolver is data-only, selects the package-local native executable rather
than `codex.js`, and arbitrary symlink rejection remains closed. Build reason
tests prove exact `build_setup_native_auth` and ambiguous-stage
`build_unknown`, with no private path in stderr.

## Mandatory vertical RED and new scope conflict

The contract-required retained six-fact, npm-launcher, local-model-enabled
production builder test fails before setup/execution/IPC:

```text
=== RUN   TestProductDaemonCopiedRuntimeStateObservesPi0821WithoutRewrite
build reason=build_observer leaf=invalid local runtime observation daemon
--- FAIL
```

This is causal. `NewLocalRuntimeObservationDaemon` constructs the Pi probe
factory; a non-nil `LocalModelCatalog` immediately calls
`bindPiLocalModelCatalog` with the production-frozen model digest
`cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`.
Therefore a small deterministic model fixture can never reach the product
builder's execution branch. The only current passing value is the external
1,117,320,768-byte locked model.

The reviewed contract requires a deterministic live-shaped fixture but locks
all Pi adapter files and says to stop `HUMAN_REQUIRED` if RED proves a locked
file is required. Continuing now would require one of:

1. a reviewed reopening of the Pi adapter for a test-only injected expected
   digest/binding seam that production cannot access; or
2. an explicit methodology amendment making the real 1.1GB installed model a
   mandatory non-hermetic component dependency.

Option 1 is recommended because it keeps the ordinary deterministic matrix
hermetic while production remains pinned to the exact frozen digest. Neither
option is silently authorized by the current owned-file list.

No live action, Provider, Keychain, Pi process, model process, staging or commit
occurred. The two earlier live lineages remain consumed and untouched.
