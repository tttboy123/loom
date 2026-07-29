# P2A-W2 Deterministic Checkpoint Final Verification

**Date**: 2026-07-29
**Status**: PASS
**Checkpoint kind**: reviewed deterministic implementation; live remains
`HUMAN_REQUIRED`

## Authority state

- Frozen W2 contract SHA-256:
  `f2e7a4d27866da4ca5f08623db6d1082587f8fd4e986f3c39f1d839dd0609a55`.
- Frozen product/test Merkle SHA-256:
  `bd11f85b1b46cfd4927131484f77e8dff36afd9b5793d18ef061aec6b72a4dac`.
- Fresh independent Implementation Review: `PASS`, no P0/P1/P2.
- Fresh independent Live Gate and Checkpoint Amendment Contract Review:
  `PASS`, no P0/P1/P2.
- Live canary: not started.
- Live allowance: unconsumed.
- MiniMax credential: no fresh product-entered credential available; the prior
  chat-pasted value remains prohibited and unused.

## Final reproduced matrix

The exact pre-commit Candidate passed:

```text
GOCACHE=/private/tmp/loom-p2a-w2-gocache go test ./... -count=1
GOCACHE=/private/tmp/loom-p2a-w2-race-gocache go test -race ./... -count=1
GOCACHE=/private/tmp/loom-p2a-w2-vet-gocache go vet ./...
GOCACHE=/private/tmp/loom-p2a-w2-tidy-gocache go mod tidy -diff
go mod verify
CGO_ENABLED=0 GOCACHE=/private/tmp/loom-p2a-w2-cgo0-gocache \
  go test ./internal/credentials -count=1
GOCACHE=/private/tmp/loom-p2a-w2-keychain-gocache \
  go test ./internal/credentials \
  -run TestKeychainStoreConfigurationIsFixedAndBounded -count=1
swift test --package-path apps/macos
swift build -c release --package-path apps/macos
swift test --sanitize=thread --package-path apps/macos
git diff --check
```

The complete Go and repository-race matrices passed every package. The Swift
debug and thread-sanitizer matrices each passed 27 XCTest cases with one
expected visual-audit-only skip, plus all three Swift Testing cases. The Swift
release build passed.

The first `go mod tidy -diff` invocation used the sandbox-denied default user Go
cache and failed only with `operation not permitted`. The required rerun used a
fresh isolated `/private/tmp` cache and passed with empty output. This was an
environment permission denial, not a product or dependency change.

## Exactness and negative checks

- every file in `live-source-lock.json` matched its SHA-256;
- the reproducible Merkle recomputed exactly;
- `gofmt -l` over every locked Go file returned empty;
- `go mod tidy -diff` returned empty and `go mod verify` reported all modules
  verified;
- staging was empty before the governed staging step;
- secret-pattern scans over W2 evidence, credential/Provider code, setup
  application/daemon code, and native sources returned no match;
- no network, Provider request, real Codex process, Keychain mutation, daemon,
  native app, Journal canary, Runtime activation, launchctl, install, push,
  merge, or publish action occurred.

The Candidate is ready for the one exact local deterministic checkpoint commit
authorized by the reviewed amendment. That commit does not accept W2, claim
live delivery, or unlock P2A-W3.
