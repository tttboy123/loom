# P2A-W2 Observer Diagnostic Implementation Verification

**Date**: 2026-07-30
**Status**: `PASS — awaiting fresh independent Implementation Review`
**Baseline HEAD**: `bd74c2e8c71d6ec699e46352459f2f4298a46584`

## Scope

This verification covers the same-W2
`Observer Diagnostic and Live Closure Amendment`, including:

- typed, safe Pi metadata command-stage errors;
- allowlisted daemon observer reason projection;
- projection-refresh classification;
- one catalog identity validation per command;
- strict skip-closed locked Pi component gate;
- Contract Repair 3 canonical Node identity;
- locked component execution A GREEN.

It creates no P2A-W4 and does not unlock P2A-W3.

## RED and repairs retained

Mandatory RED failed only on the absent frozen production symbols:

```text
PiMetadataFailureCommand
ErrRuntimeObservationProjectionRefresh
observerFailureReason
piLocalModelCatalog.validateBinding
```

The first complete repository run then exposed an accepted package-boundary
failure:

```text
TestRunProjectionSynchronizedRuntimeObservationOnceStaticBoundary
forbidden product import "fmt"
```

The same lineage removed `fmt` and used the already accepted `errors.Join`
boundary to retain both the projection sentinel and typed cause. Focused and
repeated race checks passed before the complete matrices were restarted from
the beginning.

The first enabled locked-component preflight stopped before process
construction with `component_node_identity`. Contract Repair 3 froze and
received fresh independent Review `PASS` for the canonical regular Node path.
The zero-command skip did not consume execution A.

## Focused verification

Focused runtime, piadapter, app and loomd tests passed for:

- safe version/model-list command-stage retention;
- every closed observer reason and conflicting/unknown fallback;
- exact CLI exit/error projection;
- projection refresh failure retention;
- bound local catalog materialization;
- exactly one catalog validation per command;
- real parser catalog flow;
- strict manifest duplicate/unknown/trailing/malformed rejection;
- normal no-enable component skip.

The same focused union passed `-race -count=10`. No enabled component
environment was present, so those deterministic commands executed no installed
Pi.

## Complete Go matrix

```text
go test -p 1 ./... -count=1
PASS

go test -race -p 1 ./... -count=1
PASS

go vet ./...
PASS

go mod tidy -diff
PASS — empty diff

go mod verify
PASS — all modules verified

gofmt owned files
PASS — empty file list

git diff --check
PASS
```

All Go packages completed with exit code `0`. The locked component test skipped
in normal repository matrices because its exact two enable inputs were absent.

## Swift matrix

```text
swift test --package-path apps/macos
PASS — 31 XCTest, 1 expected visual-export skip, 4 Swift Testing

swift build -c release --package-path apps/macos
PASS

swift test --sanitize=thread --package-path apps/macos
PASS — 31 XCTest, 1 expected visual-export skip, 4 Swift Testing
```

## Locked component

Execution A passed through the real
Factory→Runner→DiscoverRuntime→production-parser path:

```text
TestLockedPiComponent
PASS (12.43s)
```

It observed exact Pi version `0.82.1` and exact model ID
`loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m`. Postconditions revalidated
empty isolation, absent component Journal/socket/PID artifacts, absent port
18427 listener, absent component/llama/attempt process, and unchanged
Pi/Node/llama/GGUF identities. Execution B is forbidden.

## Security and authority checks

- candidate diff generic secret scan: `PASS`;
- owned P2A-W2 tree `sk-cp--` scan: `PASS`;
- raw Pi stdout/stderr is inspected only inside the parent parser and never
  projected by daemon/test failure output;
- tests inspect `err.Error()` only to assert that a private cause is absent;
- no credential value, token, Provider secret or raw process diagnostic was
  added;
- no Journal schema, StateWriter, Projection authority, Swift decoder/UI,
  credential store, resident observer configuration, W3 or W4 file changed;
- unrelated pre-existing Phase 1 evidence, `AGENTS.md`, `PROGRESS.md`, drafts,
  skills and Swift build output remain outside Candidate staging.

No replacement live canary 004 ran. Fresh independent Implementation Review is
the current gate.

VERDICT: PASS
