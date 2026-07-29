# P2A-W1 Native Product Experience Implementation Review 1

**Date**: 2026-07-29
**Review mode**: fresh, independent, read-only
**Verdict**: `FAIL`
**Live authority**: none

## Findings

1. `P1`: preview export compared the caller path with `XCTAssertEqual`, then
   continued to the write. A mismatch could therefore write outside the owned
   evidence path. The exact path and symlink boundary must terminate before
   rendering or writing.
2. `P1`: the render matrix asserted only a non-nil `NSImage`. A blank, clipped,
   or state-insensitive image could pass. It needs bounded encoded-content and
   state-distinguishing proof.
3. `P1`: visible-name fallback sanitized the candidate but compared the
   unsanitized candidate with the internal ID. A control-decorated ID could
   sanitize back to the raw identifier and become visible.
4. `P2`: a completed Team timeline failure left selection present and timeline
   nil, so the view continued to display an indeterminate Loading state.

The task-first IA, UI target move, dependency injection, source-ordered Work
activity, absent Compare, grouped Team/Inbox/System surfaces, semantic palette,
Command-R, target size, authority boundaries, schema exclusion, W2 lock, and
empty staging otherwise match the contract.

`VERDICT: FAIL`

## Implementation Re-review 2

**Verdict**: `FAIL`

The first Repair closed render strength, sanitized-ID fallback, and timeline
failure state. Preview leaf-symlink rejection also passed. One `P1` remained:
the parent check resolved the same parent URL on both sides, so it could not
detect an expected parent that was itself a symlink outside the repository.

Repair requirement: require the expected owned parent to equal its own
canonical resolved path and add a symlink-parent regression before export.

No live action occurred.

## Implementation Re-review 3

**Review mode**: fresh, independent, read-only
**Verdict**: `PASS`

No findings.

The Reviewer confirmed exact standardized-path equality, the frozen expected
parent equaling its own canonical resolution, leaf-symlink rejection, and the
new parent-symlink regression. It also reconfirmed 16 distinct
state/appearance render digests with bounded content checks, test-only
automatic-refresh suppression, sanitized-name fallback, and explicit
idle/loading/loaded/unavailable/fatal Team timeline states.

Fresh `swift test` passed 27 tests with only the locked preview-export test
skipped. Diff checks passed, staging was empty, and the preview path remained
absent and not a symlink. The separate schema defect, failed-live result,
allowance `0`, and P2A-W2 lock remain preserved. No preview, app, service, or
live action ran.

`VERDICT: PASS`

## Implementation Re-review 4

**Review mode**: fresh, independent, read-only
**Verdict**: `PASS`

No findings.

After Visual Audit 1 rejected the blank sidebar, the Reviewer confirmed that
the production `ContentView` now uses a native `HSplitView`, exactly five
semantic 44-point sidebar buttons, and a `NavigationStack` detail surface.
Refresh remains toolbar/recovery-only; Team selection and navigation remain
read-only presentation interactions.

The sidebar-contrast regression covers all eight states, light and dark
appearances, and accessibility width. Full Swift tests passed 27 tests with one
locked export skipped; release build, Thread Sanitizer, diff checks, prohibited
copy/palette audits, and empty staging passed. The original preview digest
remains recorded as `FAIL`.

The Reviewer permits exactly one replacement offscreen preview using only the
reviewed in-memory stub and exact owned PNG path. This creates no live or
product allowance.

`VERDICT: PASS`
