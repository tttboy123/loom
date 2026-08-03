# Phase 2A Whole-Phase Controller Verification

**Date**: 2026-08-03  
**Baseline**: `7a27149b31db7ffeefefb86c449ba448c3250fca`  
**Verdict**: `PASS — INDEPENDENT WHOLE-PHASE REVIEW PENDING`

## Fresh matrix

All commands below completed with exit status `0` after the Swift test-fixture
race repair unless a narrower scope is explicitly noted.

```text
GOPROXY=off GOTOOLCHAIN=local go test -count=1 -p=1 ./...
PASS

GOPROXY=off GOTOOLCHAIN=local go test -count=1 -race -p=1 ./...
PASS

GOPROXY=off GOTOOLCHAIN=local go vet ./...
PASS

GOPROXY=off GOTOOLCHAIN=local go mod tidy -diff
PASS — empty diff

GOPROXY=off GOTOOLCHAIN=local go mod verify
PASS — all modules verified

GOPROXY=off GOTOOLCHAIN=local go test -count=1 \
  ./internal/projection ./internal/api ./internal/localipc \
  -run 'Rebuild|GlobalReadView|StrictSwift|Cursor|Timeline'
PASS

swift test --package-path apps/macos
PASS — 75 XCTest, 1 governed visual-only skip, 0 failures;
       4 Swift Testing checks

swift test --sanitize=thread --package-path apps/macos
PASS — same 75 + 4 checks, 0 Thread Sanitizer warnings

swift build -c release --package-path apps/macos
PASS

scripts/test-install-loom-local-product.sh
PASS

scripts/test-build-loom-local-app.sh
PASS — two reproducible signed bundles and native launch smoke

scripts/test-install-loom-local-app.sh
PASS — dry-run/install/update/rollback/failure/signal/symlink checks

scripts/test-p2a-w1-live-evidence.sh
PASS
```

The projection-focused command covers deterministic Journal rebuild,
failure-preserves-old-view behavior, immutable `GlobalReadView`, strict real
Go-server/Swift-client protocol fixtures, cursor and Timeline boundaries. Full
repository Go normal/race commands also cover migrations and
`protocol/bridge/v1`.

## Fresh race RED and repair

The first fresh Swift TSAN run returned exit `1` with two warnings in the
W3-only `SequencedSuspendedTimelineStubClient` continuation array. It was not
waived. The two suspended Timeline test clients are now actor-isolated and six
resolver calls use `await`; focused, normal and full TSAN suites then passed.
See `whole-phase-swift-timeline-fixture-race-repair.md`.

## Format and shared-repository scope

Every Phase 2A Go path is `gofmt` clean. Repository-wide `gofmt -l` reports
only these four committed files from the explicitly excluded Cloud MCP line:

```text
internal/mcp/sdk/apidoc.go
internal/mcp/sdk/creds.go
internal/mcp/sdk/creds_configs.go
internal/mcp/sdk/errors.go
```

The excluded commits are `6473f80c`, `ee1d5cb6`, `fe4b8c92` and `848f068c`.
They are not Phase 2A evidence or Candidate scope. Full repository tests still
passed with them present. Current shared module inputs are:

```text
5661fd814744d16f893969ea7e1987bf2a6552a9112dc73f04cf9a5eeefb7648  go.mod
93eca99624544b76837539ccf81463513f62b7017d9ff006fc8095ef3c5bf873  go.sum
```

## Source and evidence identity

The whole-phase source selection is the ordered union of product/test/script
paths changed by Phase 2A commits between `c30a9bc` and `7a27149b`, excluding
the four Cloud MCP commits, `internal/mcp/**`, generated Swift output,
governance/evidence documents and external live roots.

```text
source paths: 120
ordered digest method: sha256 of sorted "sha256  path" lines
source digest: b2e22696590c40be5a79d4d4c33d6d9bd5a9799dfaad342d9f80be057591b81d
```

The accepted evidence selection is all 462 tracked paths under
`.loom-evidence/phase2a` at `7a27149b` plus the test-race repair record, for 463
paths total. The whole-phase matrix, this verification, the forthcoming lock,
the independent review and `docs/CURRENT.md` are deliberately excluded to
avoid recursive identity.

## Security and non-disclosure

The 120 source paths contain no pasted MiniMax key, OAuth material, bearer
token, raw Grant or hidden reasoning. Three intentional sentinel/public
configuration matches were manually classified:

- `loom-local-offline` local non-secret model-server API key;
- redaction marker names in `rpc_bridge_adapter.go`;
- `sentinel-sensitive-value` in a negative test fixture.

No live process, Provider request, credential mutation, product daemon,
native app, TUI, Pi, model server or canary was started by this whole-phase
verification. The packaging fixture's bounded native launch smoke used a
temporary app with no product socket and terminated it inside the fixture.

## Gate

Controller verification is `PASS`. The next and only permitted action is a
fresh independent read-only Whole-Phase Review of PX-01 through PX-18, the
test-only race repair, current source/evidence identity, historical failure
classification and the three distinct PX-17 product roles. Phase 2B remains
locked pending that review and explicit Product Owner Phase 2A sign-off.
