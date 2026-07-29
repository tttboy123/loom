# P2A-W1 Native Product Experience Deterministic GREEN

**Date**: 2026-07-29
**Result**: `PASS`
**Live action**: none

## Delivered boundary

The native app now presents one task-first read workspace:

```text
Home
Work
Teams
Inbox
System
```

- Home orders human attention, source-ordered Work activity, observable teams,
  and system readiness.
- Work owns Runs and accepted Evidence.
- Teams owns selection and the selected Team timeline.
- Inbox promotes the required human action.
- System owns connection health and Runtime inventory.
- Compare and unsupported creation or execution actions are absent.

The old eight-destination operator-console hierarchy and equal metric-card Home
were removed.

## Architecture

The real SwiftUI view moved into the importable `LoomLocalAppUI` target. The
executable and tests use the same `ContentView`. `LocalProductExperience` is a
pure presentation projection over the accepted snapshot and closed connection
state. It adds no IPC, persistence, command, authority, or dependency.

## Verification

```text
swift test
PASS - 27 tests, 1 visual-export test skipped by default

swift build -c release
PASS

swift test --sanitize=thread
PASS - 27 tests, 1 visual-export test skipped by default

git diff --check
PASS
```

The real `ContentView` rendered through in-memory stubs across:

- loading;
- connected empty;
- connected populated;
- partial;
- stale with preserved view;
- offline without a view;
- offline with preserved view;
- fatal;
- light and dark appearance;
- accessibility-size type.

Static audits passed for:

- exactly one system accent color and semantic macOS colors;
- SF Symbols only;
- Command-R, accessible refresh labels and hints, native controls, and shared
  44-point explicit recovery targets;
- no visible em dash;
- no AI-purple/pink palette, gradient, glass wall, Compare, recency claim,
  fake creation action, raw wire identifier, Provider, or credential copy;
- no new package or remote dependency;
- empty Git staging.

One validation orchestration attempt initially started release and Thread
Sanitizer against the same SwiftPM build directory. SwiftPM safely serialized
them. Both commands were then rerun sequentially and passed; no product defect
or source repair resulted.

## Implementation Review Repair 1

The four Review findings are repaired:

1. preview export uses a terminating guard for the exact owned URL, resolves
   the parent boundary, rejects a symlink target, and has mismatch/symlink
   regression coverage;
2. the real-view matrix disables only automatic appearance refresh while still
   injecting the same stub-only Store, verifies every expected presentation
   state, 1,100 by 720 pixels, bounded PNG content, color variation, and 16
   distinct light/dark state digests, then repeats at 780 by 720 with an
   accessibility Dynamic Type size;
3. visible-name fallback compares sanitized candidate and sanitized internal
   ID, so a control-decorated ID returns the generic label;
4. Team timeline now has explicit idle, loading, loaded, unavailable, and fatal
   states; a completed failure cannot remain an indeterminate loading view.

Full Swift tests, release build, Thread Sanitizer, diff, import/dependency,
visible-copy, focus/animation, exact preview-absence, and staging checks pass
after the repair.

## Implementation Review Repair 2

Preview export now first proves exact standardized-path equality, then requires
the frozen expected parent to equal its own canonical resolved path, and
finally rejects a symbolic-link leaf. The focused parent-symlink regression
changed from the recorded RED to:

```text
swift test --filter \
  LocalProductExperienceViewTests/testPreviewPathValidationRejectsMismatchAndSymlink
PASS - 1 test, 0 failures
```

This closes the remaining Re-review 2 finding without changing the preview
allowance or any production execution path.

The complete post-repair matrix also passes:

```text
swift test
PASS - 27 tests, 1 locked visual-export test skipped

swift build -c release
PASS

swift test --sanitize=thread
PASS - 27 tests, 1 locked visual-export test skipped

git diff --check
PASS
```

The preview target remains absent and is not a symlink. Static audits remain
clean for prohibited visual copy/palette patterns, truncation, hidden focus,
implicit animation, remote dependencies, staged changes, and scope expansion.

## Visual Audit Repair

The first preview revealed that macOS offscreen rendering reserved the
`NavigationSplitView` sidebar width but did not paint its native List content.
The same production `ContentView` now uses a native `HSplitView` workspace with
an explicit semantic sidebar, five 44-point plain buttons, visible product
identity, selected-state treatment using the single system accent, local
connection status, and a `NavigationStack` detail surface retaining the
toolbar.

The new sidebar-contrast regression changed from zero contrast samples in every
state to `PASS` across the complete state, light/dark, and accessibility-width
matrix. This is a product view repair, not a test-only snapshot substitute.

Post-repair verification passes:

```text
swift test
PASS - 27 tests, 1 locked visual-export test skipped

swift build -c release
PASS

swift test --sanitize=thread
PASS - 27 tests, 1 locked visual-export test skipped

git diff --check
PASS
```

The prohibited-copy/palette, truncation, focus, animation, dependency, scope,
and staging audits remain clean. The first failed preview has not yet been
replaced; fresh independent Implementation Re-review is the gate.

## Preserved terminal boundary

No app executable, socket, daemon, service manager, Runtime, Provider, model,
Journal, installer, or native-window canary was run. The diagnosed
`model_ids:null` defect is unchanged.

```text
P2A-W1: FAIL - ROLLED_BACK - HUMAN_REQUIRED
live allowance: 0
P2A-W2: LOCKED
```

Fresh independent Implementation Review is the current gate for this UI-only
reopen.

## Accepted Replacement Preview

Implementation Re-review 4 returned `PASS` with no findings. The single
reviewed replacement offscreen export then passed visual inspection at 1100 by
720 pixels with SHA-256
`ef75ea2c5a1241b61c7d593bd86ca29cfd58587ac7cc64303d43f04efb0fa122`.
The exact five destinations, selected state, product identity, connection
status, attention-first hierarchy, source-ordered Work, team grouping, and
system readiness are visibly present.
