# P2B-W1 Native Drawer Presentation Repair

Date: 2026-08-03

Scope: same P2B-W1; no new WorkItem, schema, authority, writer, IPC method,
Provider action, dispatch or authority canary.

## Result defect

The first unlocked Computer Use inspection opened the current source-linked
native `MissionWorkbench` against a bounded read-only fixture derived from the
retained `canary-001` SQLite and immutable summary Artifact. It showed the
Side-task title, mode, status, summary, Evidence, Artifact, usage and decision
menu, but did not always show `purpose`, empty `uncertainties`, empty
`scope_delta` or an explicit next-action sentence.

Pre-repair screenshot:

```text
/Users/lune/Library/Application Support/Loom/phase2b-visual/side-task-drawer-001/native-drawer-pre-repair.png
```

The screenshot is mode `0600`. It is failure evidence, not a PASS artifact.

## Mandatory RED and repair

The first focused RED failed to compile because
`sideTaskDrawerPresentation` did not exist. The minimal repair added a pure
presentation mapping and made these contract fields explicit:

- `Purpose`;
- `Uncertainty`, including `None recorded`;
- `Scope`, including `No parent scope expansion`;
- `Next action`.

Independent Review then found one P1: `status=decided` had been incorrectly
treated as a completed parent effect. A second focused RED produced three
failures and froze the distinct semantics:

- `decided + effect_status=pending` waits for the authorized parent effect;
- `effect_status=completed` reports the parent effect complete;
- `decided + effect_status=none` reports the decision recorded with no parent
  effect required.

The repaired focused test passed.

## Post-repair verification

- `go test -p 1 ./...`: PASS;
- `go test -race -p 1 ./...`: PASS;
- `go vet ./...`: PASS;
- `go mod tidy -diff`: PASS, empty diff;
- Swift full suite: 80 XCTest, 1 visual-export-only skip, 0 failures, plus
  4 Swift Testing tests PASS;
- Swift TSAN: identical PASS result;
- Swift Release build: PASS.

No Go byte changed during the second presentation repair, so the Go matrix
remains bound to the current Go Candidate bytes.

## Boundary

Only these product/test files changed for this Result repair:

```text
apps/macos/Sources/LoomLocalAppUI/MissionWorkbench.swift
apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift
```

The consumed authority canary was not rerun or replaced. The original
`source-lock.json` remains immutable evidence of its exact bytes.
