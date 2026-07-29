# P2A-W1 Native Product Experience Mandatory RED

**Date**: 2026-07-29
**Result**: `PASS - RED CONFIRMED`
**Live action**: none

## Baseline

Before the new tests, `cd apps/macos && swift test` passed all 17 existing
tests.

## Frozen RED

Tests were added first for:

- the exact five task-first destinations;
- source-ordered Home and Work grouping;
- distinct connection/preserved-view states;
- functional copy with no visible em dash;
- Team selection remaining inside Teams;
- the real SwiftUI `ContentView` imported by tests;
- the non-connecting state, appearance, and accessibility-size render matrix;
- the shared 44-point recovery-action target.

The unchanged product then failed:

```text
error: no such module 'LoomLocalAppUI'
```

The command exited `1`. This is the exact missing frozen UI boundary, not an
unrelated environment, dependency, daemon, socket, Provider, Runtime, Journal,
or live failure. No product source had changed when RED was captured.

## Implementation Review Repair 1 RED

Implementation Review 1 returned `FAIL` on preview-path termination,
state-insensitive render evidence, sanitized internal-ID fallback, and Team
timeline failure presentation.

Regression tests were added before the product repair. The focused command
exited `1` on the exact missing symbols:

```text
LocalProductStore has no member timelineState
LocalProductExperience has no member visibleName
```

The strengthened render matrix then correctly rejected the original weak
evidence: its state images collapsed to two appearance-only digests and failed
the state-distinguishing assertion. This was a genuine evidence RED. No live
action occurred.

## Implementation Review Repair 2 RED

Implementation Re-review 2 confirmed three prior findings closed but found the
preview parent-symlink closure ineffective. A regression made the exact gap
executable by placing the expected preview beneath a symlinked parent. Before
the repair, the focused test exited `1`:

```text
XCTAssertThrowsError failed: did not throw an error
```

The validator had resolved the requested and expected instances of the same
parent and compared those equal values. It did not prove that the frozen
expected parent itself was a canonical, non-symlink directory. No preview was
written and no live action occurred.

## Visual Audit Repair RED

The first reviewed offscreen preview was rejected because its reserved sidebar
column was completely blank. A pixel-bounded regression was added before the
view repair. It sampled the real sidebar region across all eight presentation
states, light and dark appearances, and the accessibility-width render.

The unchanged view failed every sample with:

```text
sidebarContrastPixelCount = 0
XCTAssertGreaterThan failed: 0 is not greater than 250
```

This converts the visual finding into a deterministic failure instead of
accepting a content-only screenshot. No app, native window, socket, service, or
live action ran.
