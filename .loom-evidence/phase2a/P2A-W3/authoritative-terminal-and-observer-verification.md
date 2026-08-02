# P2A-W3 Authoritative Terminal and Observer Verification

**Date**: 2026-08-02  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Contract Review**: `PASS`  
**Status**: `DETERMINISTIC PASS / IMPLEMENTATION REVIEW PENDING`  
**Live allowance**: none exercised

## Focused causal closure

The post-RED implementation passed the focused credential, daemon, public
reason-code, copied-state Pi, timeout-containment, real Swift-to-Go UDS, and
Swift presentation tests. The focused Go packages also passed under the race
detector, and the cancellation-sensitive daemon test passed five consecutive
runs.

The real UDS fixture traverses:

```text
Swift Client
  -> Go local IPC server
  -> LocalProductSetupService
  -> CredentialBroker
  -> Event Journal
  -> rebuilt authoritative snapshot
  -> strict Swift decoder
```

It starts from an inherited verified credential state, injects a Secret Store
read denial, proves zero Provider calls, and observes exactly one next-revision
`ProviderCredentialVerified` fact with `status=rejected` and
`reason=unavailable`. The operation idempotency key remains exact and no secret
or raw helper error crosses the wire.

After Implementation Review 1 failed, Repair 1 removed every private-cause
exception from the observer error-tree classifier. A known leaf mixed with any
unknown leaf now always resolves to `observer_unknown`; direct unique typed
sentinels retain their exact allowlisted reasons. The focused four-case RED
failed before this behavior changed and passed afterward.

Repair 2 makes the copied-state Pi 0.82.1 proof self-contained. The exact
reviewed database bytes are stored as the compressed evidence fixture:

```text
.loom-evidence/phase2a/P2A-W3/retained-pi-six-fact-state.sqlite.gz.b64
```

Every ordinary focused/full test decodes that fixture and requires the frozen
source SHA-256
`677624b624b68ddd908939766752177a802bd90b657075133ef845e67e5461a2`,
copies those bytes into the hermetic private fixture, and verifies exactly one
each of the six retained fact types, including `TeamDefinitionSaved`,
`TeamInstanceCreated`, `AgentInstanceCreated`, and the exact Pi 0.82.1 Runtime
and qualified local-model identity. One unchanged observation keeps the product
socket serving. The complete public observer reason matrix is then injected at
the production classification boundary. Every success/failure check preserves
the exact event list, saved-Team payloads and final database SHA-256.

Repair 2 also requires both the Pi metadata command and the derived failure
category to be unique. A same-command `process + stderr` tree now fails closed
as `observer_unknown`, while repeated copies of one identical typed category
still produce the exact public reason.

## Complete deterministic matrix

All of the following completed with exit status `0`:

```text
go test -count=1 -p 1 ./...
go test -count=1 -race -p 1 ./...
go vet ./...
go mod tidy -diff
git diff --check
gofmt -d <all owned Go files>
/usr/bin/swift test --package-path apps/macos --quiet
/usr/bin/swift test --package-path apps/macos --sanitize thread --quiet
/usr/bin/swift build --package-path apps/macos -c release --arch arm64 --quiet
```

Both Swift test modes executed 60 XCTest cases with one expected skip and zero
failures, plus four Swift Testing cases with zero failures. The arm64 Release
build completed without diagnostics.

After Repair 2, the complete serialized Go and Go race matrices, both Swift
test modes, and the arm64 Release build were run again and returned exit status
`0`.

An earlier default-parallel whole-repository attempt ran two independent Swift
compiler trees from Go fixtures at the same time. That resource contention
caused external process/setup deadlines in `cmd/loomd` and pre-existing
`internal/app` daemon tests. The affected packages, their standalone race
forms, and the complete package matrix all pass when the repository packages
are serialized with `-p 1`. This is recorded as test-infrastructure contention,
not hidden as a product PASS and not repaired by widening product timeouts or
modifying locked application tests.

## Security and authority checks

- literal MiniMax secret-key-prefix matches across owned source, product docs,
  and P2A-W3 evidence: `0` files;
- literal long `Authorization: Bearer ...` matches across the same boundary:
  `0` files;
- `LocalProductStore.swift` remains byte-identical to the accepted compatibility
  source lock; only its test needed the explicit unavailable reason fixture;
- Event schema, StateWriter, Projection, Supervisor, Bridge v1, Provider
  verifier, Keychain implementation, scheduler, retry, and compaction behavior
  remain outside this repair.

Locked authority hashes remain exact:

```text
496fa5d51f790c3b4289c84cfd31ec9c89c9aa064f72a660085f6992f67160dc  internal/localipc/protocol.go
5a453e1b219d409054a3153423f9a4084c85f02eb413be34b0648d331739bf37  internal/state/local_product_setup_writer.go
7ea7dfd1b270b0166e64e3a8753818464411692ba671d5afae00962311c37673  internal/projection/local_product_setup.go
968005d032ff6c0d8fedbaf28e7c3789ab084ea5fbcd660a060a0043a337d637  internal/supervisor/managed_execution.go
1c40aa90d1be2012ff6f0ea637f152722c2153dc2ace8b877dc8bacff9938365  protocol/bridge/v1/frame.go
```

No Provider, Secret Store, Pi Runtime, native window, daemon activation, live
canary, Journal mutation outside hermetic fixtures, staging, or commit occurred
during this deterministic verification.
