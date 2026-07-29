# P2A-W2 Vertical Native Journey Closure Implementation Verification

**Date**: 2026-07-30
**Contract**: `Vertical Native Journey Closure Repair`
**Baseline**: `326f797`
**Result**: `GREEN — FRESH INDEPENDENT IMPLEMENTATION REVIEW REQUIRED`

## Implemented vertical behavior

- `SetupSnapshot` and `StartBuilder` rebuild Projection and derive a canonical
  catalog from the same immutable `GlobalReadView`.
- The exact six-Event attempt-004 fixture selects the later model-capable
  Runtime; the earlier Runtime's `model_ids:null` is normalized only at the
  read/wire boundary.
- A Builder session stores its exact catalog, domain catalog, view version and
  binding digest. A changed current catalog or view rejects the next command
  with no session mutation or silent Runtime/model rebinding.
- After one real MiniMax verification result, the Broker performs exactly one
  metadata append with a fresh cancellation-independent context bounded to one
  second. Provider verification is not retried and Secret Store failure before
  observation still appends zero facts.
- macOS Keychain queries explicitly fail closed instead of opening an
  authentication UI.
- Go and Swift retain five-second defaults and grant exactly
  `credential_verify` a ten-second request budget.
- Product shutdown is sequential and joined:
  local IPC, Runtime observer, setup/native-auth owner, then database. Every
  owner closes exactly once; internal failures retain the stage while the CLI
  continues exposing only its closed shutdown result.

## Focused verification

```text
go test ./internal/app -run TestLocalProductSetup -count=1
go test ./internal/credentials -run 'TestCredentialBroker|TestKeychain' -count=1
go test ./internal/localipc -run TestServer -count=1
go test ./cmd/loomd -run \
  'TestProductSetupRefreshesRuntimeCatalogAfterServiceConstruction|\
TestProductDaemonRealSetupServiceConfirmsCandidateOverPrivateUDS|\
TestProductDaemonClose' -count=1
swift test --package-path apps/macos \
  --filter 'LocalIPCClientTests|LocalProductStoreTests'
```

All focused commands passed. The Swift focused run executed 15 tests with zero
failures.

Focused race repetition also passed:

```text
go test -race ./cmd/loomd -run \
  'TestProductDaemon(CloseIsOrderedJoinedAndExactlyOnce|\
CancellationJoinsHandlerAndObserver)|\
TestProductSetupRefreshesRuntimeCatalogAfterServiceConstruction' \
  -count=20
```

## Complete deterministic matrix

All commands passed:

```text
go test -p 1 ./... -count=1
go test -race -p 1 ./... -count=1
go vet ./...
go mod tidy -diff
go mod verify
swift test --package-path apps/macos
swift build --package-path apps/macos -c release
swift test --package-path apps/macos --sanitize=thread
git diff --check
```

Swift debug and thread-sanitizer runs each executed 33 XCTest cases with zero
failures and one intentionally skipped visual-audit export. The Swift Testing
strict setup-wire suite also passed four tests. The release build completed.

## Security and scope

- The owned Candidate diff contains no API key, bearer token, authorization
  header or `PRIVATE_SECRET` match.
- No secret is added to source, arguments, environment, prompt, Journal,
  Evidence, AgentDefinition, logs or screenshots.
- No Journal schema, StateWriter authority, Projection authority, Runtime
  parser/probe, Team execution, Grant, Evidence or Scheduler boundary changed.
- No P2A-W4 was created.
- Existing unrelated dirty files and the separate resident daemon were not
  modified, signalled or reconfigured.
- No live action is authorized until fresh independent Implementation Review
  returns `PASS`.

VERDICT: PASS
