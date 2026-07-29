# P2A-W2 Credential Transaction and Shutdown Closure Implementation Verification

Date: 2026-07-30

Status: REPAIR 1 ACCEPTED — fresh independent Re-review PASS

## Implemented boundary

- Production Keychain Put, Read and Delete now use one short-lived copy of the
  exact daemon executable in internal helper mode.
- The parent sends the operation only through inherited anonymous request and
  response pipes. The helper receives an empty environment; stdin, stdout and
  stderr do not carry the protocol.
- The helper authenticates the live parent PID and start identity, exact
  executable device/inode/owner/mode/SHA-256 identity, normal daemon arguments,
  the exact private `0600` product socket's kernel peer PID and the anonymous
  pipe descriptor types before it parses an operation.
- The strict bounded binary protocol rejects unknown operations/statuses,
  truncation, trailing bytes, overflow and an attested-socket mismatch.
- Each parent-side Put, Read and Delete has the earlier of the caller deadline
  and an exact two-second total helper budget. Cancellation kills and waits for
  the exact child and closes every pipe.
- The direct Security.framework implementation and its fixed service,
  accessibility, non-synchronizable and no-authentication-UI attributes remain
  unchanged and are reachable only inside the authenticated helper in the
  production product-daemon path.
- Production `credential_verify` now rebuilds the authoritative Provider
  projection before Broker access. Exact current revision delegates once;
  exact same-reference `expected+1` terminal verified/rejected returns the
  committed result with no Keychain, Provider or append; every other stale
  shape fails closed as a metadata conflict.
- The existing five-second default IPC, ten-second credential-verify IPC,
  five-second Provider and one-second cancellation-independent terminal commit
  budgets are unchanged. No Journal, StateWriter, external IPC schema, Swift
  decoder or native UI contract changed.

## RED

The mandatory pre-implementation RED is recorded in
`credential-transaction-and-shutdown-closure-red.md`. It failed only on the
missing helper protocol/store and product verify-recovery boundaries.

## Focused GREEN

```text
go test ./internal/credentials ./internal/provider ./internal/localipc ./internal/app ./cmd/loomd -count=1
```

PASS.

```text
go test -race ./internal/credentials ./internal/localipc ./cmd/loomd \
  -run 'Test(KeychainHelper|ProcessKeychainStore|ProductCredentialVerify|LoomdCredentialHelper|ServerUsesExtendedDeadline|ServerClose)' \
  -count=5
```

PASS.

The focused fixtures prove strict Put/Read/Delete protocol success, malformed
and trailing request rejection, regular-file descriptor rejection, attested
socket mismatch rejection before store access, kernel socket peer identity,
executable metadata/digest binding, output-producing helper failure,
malformed-response failure, cancellation kill-and-join, the exact operation
deadline on all three operations, direct and pipe-only activation rejection,
exact-current delegation, terminal lost-response recovery and all other stale
shape rejection before the Broker. A ten-run focused race repetition also
proves that two concurrent same-revision verify calls are serialized around
the projection precheck and delegate, so only one reaches the Provider and the
second returns the committed terminal result.

No focused test reads, writes, deletes or enumerates the real Keychain item,
contacts a Provider, starts the resident daemon, launches the app or uses the
network.

## Complete GREEN

All commands below ran against the final Candidate:

```text
go test -p 1 ./... -count=1
go test -race -p 1 ./... -count=1
go vet ./...
go mod tidy -diff
go mod verify
git diff --check
```

PASS. `go mod tidy -diff` was empty and `go mod verify` reported all modules
verified.

```text
swift test --package-path apps/macos
swift build -c release --package-path apps/macos
swift test --sanitize=thread --package-path apps/macos
```

PASS. Debug and thread-sanitizer runs each executed 33 XCTest cases with zero
failures and one intentional preview-export skip, plus four Swift Testing
contract cases with zero failures. The release build passed.

The exact owned Go and evidence files pass `gofmt`; the secret-negative scan
found no MiniMax key, base URL, Authorization or Bearer material. The existing
unrelated dirty and untracked files were not edited or included.

## Non-gate diagnostic

An additional `CGO_ENABLED=0 go test -p 1 ./... -count=1` diagnostic compiled
the new generic boundary but failed existing product-daemon setup tests because
the pre-existing non-Darwin/non-cgo Keychain implementation deliberately
returns unavailable. Phase 2A's reviewed OS Secret Store target and required
matrix are Darwin with cgo; this diagnostic is not presented as a gate PASS and
did not trigger a scope expansion.

## Remaining gates

1. Atomic Candidate commit including only the owned implementation, tests and
   evidence.
2. Only after that gate, the already authorized single replacement controlled
   canary lineage `p2a-w2-live-20260730-006`.

P2A-W3 remains locked and no P2A-W4 exists.

## Implementation Review 1 and Repair 1

Fresh independent Implementation Review 1 returned `FAIL — REPAIR REQUIRED`
because a `NewProductKeychainStore` construction error still returned `nil,
nil`, allowing the daemon to continue without setup. The Reviewer also noted
that the frozen amendment's header retained its pre-Re-review status.

Repair 1 first reproduced the production construction branch as RED, then:

- returns a closed `setup credential boundary unavailable` error instead of a
  nil setup service;
- adds a targeted test proving the production process-Keychain construction
  failure cannot degrade into a partially running setup surface; and
- corrects only the amendment status header to record the already completed
  Contract Repair 1 Re-review PASS.

Ten-run focused and race repetitions pass. The complete focused, serial
repository, repository-race, vet, module and diff matrix above was rerun against
Repair 1 and passed. No live or external action ran. Fresh independent Repair 1
Re-review returned `PASS` with no P0, P1 or P2 finding.
