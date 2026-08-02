# P2A-W3 Native Launcher and Build Transaction Repair RED

**Date**: 2026-08-02  
**Boundary**: existing Native Launcher and Build Transaction Closure Contract  
**Authority/schema expansion**: none

Final pre-lock inspection found three fail-closed attribution gaps within the
already owned `cmd/loomd` files:

1. a nested `build_decision` plus `build_setup_native_auth` tree returned only
   the outer reason instead of closing ambiguous;
2. a real invalid local IPC server construction returned an untyped error and
   therefore published `build_unknown` instead of `build_ipc`; and
3. the product runner relabeled an already typed setup-native-auth failure as
   `build_decision`.

Regression tests were added first. The exact focused RED commands failed as
required:

```text
go test -count=1 ./cmd/loomd -run \
  'TestDaemonBuildFailureNestedAmbiguityFailsClosed|TestProductDaemonClassifiesConstructionIPCFailure'

FAIL:
  build reason = "build_unknown", want build_ipc
  reason = "build_decision", want build_unknown

go test -count=1 ./cmd/loomd \
  -run TestProductDaemonPreservesSetupConstructionFailureReason

FAIL:
  build reason = "build_decision", want build_setup_native_auth
```

The repair records a typed reason and continues traversing wrapped causes so
different nested reasons become `build_unknown`, preserves the typed setup
failure returned by `buildProductSetupService`, and wraps actual local IPC
server construction failure as `build_ipc`. No raw cause or path is published.

The three exact regression tests then passed. No process, socket, Provider,
credential, model or live canary was started by this RED/repair.
