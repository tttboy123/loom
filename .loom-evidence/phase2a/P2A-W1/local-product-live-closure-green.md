# P2A-W1 Local Product Vertical Live Closure GREEN

**Date**: 2026-07-29  
**Result**: `GREEN - COMPLETE DETERMINISTIC MATRIX PASS`  
**Live authority**: none  
**Candidate bootstrap count**: `0`  
**Native app launch count**: `0`  
**Live allowance**: `0`

## Causal implementation

The shared read-only Application/API boundary now canonicalizes the two Runtime
collection fields with a fresh allocation:

- a historical nil `model_ids` slice becomes non-nil `[]`;
- a historical nil `observed_capabilities` slice becomes non-nil `[]`;
- non-empty values and source order are preserved;
- returned snapshots and stale fallback snapshots do not alias projection
  storage.

No Journal, Projection, StateWriter, IPC envelope, Swift decoder, Runtime,
Provider, credential, daemon lifecycle, or installed-state behavior changed.
Strict Swift decoding still rejects null, missing, duplicate, wrong-type, and
unknown Runtime fields.

## RED to GREEN trace

The preserved RED proves:

1. real Event to Projection to `LocalProductReadService` returned nil
   `model_ids`;
2. the sibling nil `observed_capabilities` case also returned nil;
3. a real SQLite to daemon IPC snapshot exposed nil `model_ids`;
4. strict Swift accepted canonical empty arrays and rejected null/malformed
   fields.

The minimal implementation is only
`cloneCanonicalLocalProductCollection`, used by snapshot construction and
snapshot cloning.

## Deterministic verification

All commands below completed without Provider access, Runtime execution,
resident Journal mutation, launchd restart, or App launch.

```text
go test ./internal/api ./internal/localipc ./cmd/loomd
PASS

go test -race ./internal/api ./internal/localipc ./cmd/loomd
PASS

go test ./...
PASS

go test -race ./...
PASS

go vet ./...
PASS

go mod tidy -diff
PASS - no output

go mod verify
all modules verified

cd apps/macos && swift test
27 tests, 0 failures, 1 governed visual-export skip

cd apps/macos && swift build -c release
PASS

cd apps/macos && swift test --sanitize=thread
27 tests, 0 failures, 1 governed visual-export skip

scripts/test-install-loom-local-product.sh
installer fixture PASS

local-product-live-closure-transaction-test.sh
local product closure transaction fixture PASS

git diff --check
PASS
```

Supplemental native App build and installer fixtures were also rerun
sequentially and passed. An earlier non-contract supplemental attempt started
both SwiftPM fixtures concurrently against the same `.build` directory; one
fixture correctly failed while SwiftPM serialized the shared cache. No product
or live state changed. Serial execution then passed both fixtures. The
generated Swift cache was removed with `swift package reset` after Candidate
materialization.

## Source and boundary audit

- source-lock SHA-256:
  `0bd33d144c5aadcd078bef06a45685b8ac2ed2ab2bf76effe80730ee85e8a128`;
- all 53 locked accepted inputs reproduce their exact hashes;
- the only final product/test deltas are the four contract-owned files;
- `go.mod` and `go.sum` retain the exact source-lock hashes;
- owned Go files are `gofmt` clean;
- mutation-authority/import, credential-value, terminal-control, diff, and
  empty-staging audits pass;
- resident service, installed product, App, Journal, socket, and LaunchAgent
  were not touched.

## Fresh Candidate

The new Candidate was built only after GREEN in a user-owned private `0700`
ephemeral root. Its manifest contains no absolute private path, credential,
Provider environment, or Journal copy.

| Artifact | Exact identity |
|---|---|
| `loom` | `b12e5261632efc10585163ea323a22951f736b5565b5a717b0cf77a2765d0f29` |
| `loomd` | `fdca8152e924d32e2a0163a74e0e357afbcd135fce58be25f1c531d3ebb1596b` |
| native executable | `3aedc0ba90be3cdfb3df0865016abebbc9cc5b659bfa6a445ae4a54af272cfba` |
| arm64 `LC_UUID` | `93E3FC61-0A19-3379-BFDB-8CA7726F59F6` |
| canonical App manifest | `bdbf57511fbbcd56c583af3e2bb84f03d45f2b012b4a1eb6a5f087203e2d4a58` |
| Candidate record | `7c0ec8cd75129f0184d345ea39684bc1b1850db56907f1b95ea57f61302a1700` |
| transaction | `a23b3d3ca04040c42d8fdb2b891e945e44cf3ff7b2966cca354faf35da6c528a` |
| transaction fixture | `6f7b8252d861561bc2beed3616fdbd3e8343bb5316497f8f03580796438d2e1d` |

The manifest also freezes the exact Go, Swift/Swift driver, swiftlang, Clang,
Xcode, target, host architecture, and macOS versions. The App is
strict-signature valid, arm64, has a non-zero `LC_UUID`, contains no symlink,
and uses user-owned `0700` directories/executables with `0600` ordinary files.

## Transaction proof

The new transaction is initially locked by the absent post-Review activation
audit and returns:

```text
LOCAL_PRODUCT_CLOSURE_RESULT result=preflight_failure initial_bootstrap_calls=0 consumed=0 rollback_count=0 restart_count=0
```

Its deterministic fixture proves:

- pre-bootstrap failure: `0` bootstrap, allowance unconsumed, rollback `1`;
- post-bootstrap readiness failure: exactly `1` bootstrap, consumed, rollback
  `1`, no retry;
- success path: exactly one initial bootstrap plus one classified later
  lifecycle restart;
- symlink, ownership, mode, non-canonical path, ambient override, unknown
  argument/scenario, and counter-escape cases fail closed;
- output is bounded and credential-value negative.

Production preflight additionally binds the exact source lock, four delta
hashes, Candidate record, Candidate artifacts, App signature/UUID/manifest,
accepted original installed bytes, LaunchAgent identity, one-Event Journal,
view version, crash inventory, absent staging, and private path/mode/owner
state before any mutation.

## Gate

Fresh independent Implementation Review is now required. This GREEN grants no
install, bootstrap, restart, native-window action, activation audit, live
allowance, commit, acceptance, or P2A-W2 work.

## Implementation Review repair 1

Implementation Review 1 returned `FAIL` on one P1 evidence-transaction defect:
the transaction named an activation record outside the contract's exact create
allowlist. It otherwise independently reproduced the source, Candidate,
causal implementation, complete matrix, privacy, and staging gates.

The repair changes only the already owned transaction and fixture:

- the transaction now reads exactly
  `local-product-live-closure-activation-audit.md`;
- the fixture asserts that exact binding and rejects the obsolete unowned
  `post-review` spelling.

After repair, shell syntax, fixture failure/success/escape cases, the
production zero-bootstrap live lock, full repository tests, full race, vet,
module no-drift/verify, Swift debug/release/Thread Sanitizer, installer,
source-lock, Candidate manifest, format, diff, and empty-staging gates all
pass. The Swift cache was removed again after verification. Fresh independent
Implementation Re-review is required; live allowance remains `0`.
