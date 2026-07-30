# P2A-W2 Mission Workbench Repair 3 GREEN

Date: 2026-07-30

Status: PASS — CONTROLLER VERIFICATION

Source lock:
`2badc23c740b0c24fad494a02f2bf8956a60e4dfb2f35f17701fab6695213b55`

## Closure

Repair 3 remains inside the same frozen P2A-W2 contract.

- The ordinary production `newProductDaemonRunner` can load a controlled
  Mission fixture only from one exact private manifest named by
  `LOOM_CONTROLLED_MISSION_FIXTURE_MANIFEST`.
- The manifest, state file and containing directories are absolute,
  canonical, user-owned and private; the state path, attempt root, fresh
  Artifact root, purpose and authoritative time are bound.
- Fixture construction uses the accepted Journal, Projection, Rules, Work,
  Run, Evidence and verification paths. It creates five real TeamExecution
  Missions and four exact prepared commands. There is no UI-only transition or
  second authority.
- With no manifest, production retains the empty fail-closed registry.
- Consumed commands disappear and every remaining exact command receives the
  new immutable GlobalReadView version.
- Schema 2 Swift decoding now requires `prepared_decisions` to be a real JSON
  array and rejects `null`; Go supplies an empty array when no command exists.
- A Review-lane Mission without a prepared acceptance command opens a
  read-only Review Gate. `Accept Result` and `Request Changes` remain disabled,
  and dismissing the sheet makes no IPC or authority call.

## Exact verification

PASS:

```text
go test ./...
go test -race ./...
go vet ./...
swift test --package-path apps/macos
swift test --package-path apps/macos --sanitize=thread
swift test -c release --package-path apps/macos
swift build --package-path apps/macos -c release
git diff --check
```

Focused production and authority proofs also passed:

```text
go test ./cmd/loomd \
  -run 'TestProductDaemonProductionRunner(LoadsControlledMissionFixture|WiresFailClosedDecisionRegistry)' \
  -count=1 -v
go test ./internal/app \
  -run TestControlledMissionDecisionFixtureBuildsRealJournalBackedRegistry \
  -count=1 -v
swift test --package-path apps/macos \
  --filter 'LocalProductStoreTests|MissionOrchestrationTests|LocalProductDecisionModelsTests'
```

The exact race command ran alone and passed. Swift debug, Thread Sanitizer and
Release each executed 50 XCTest cases with one explicitly gated visual-export
skip plus four Swift Testing cases, with zero failures. The visual exports were
then run explicitly and passed.

The 31-file source lock reproduces with zero mismatches. Forbidden-authority,
secret-pattern and diff-format scans are empty.

The manifest carries a lexical 40-hex source-commit claim but the runner does
not self-attest that field. The post-commit preflight must independently bind
the exact commit and binary hashes and cannot use this field alone as proof.

## No-live declaration

No external or installed daemon, Provider request, Keychain mutation,
installed application or live canary was used. The product socket and all
authority inputs exercised here were test-owned and isolated.
