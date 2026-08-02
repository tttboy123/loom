# P2A-W3 Final Native Timeline Pagination Repair 7 Test Proof Diagnosis 1

**Date**: 2026-08-03  
**Status**: `CONFIRMED`

The new cancellation late-error test exposed a production race inside the
original Repair 7 behavior: cancelling the Swift Task fences a subsequent
successful page through `Task.checkCancellation()`, but a client error thrown
directly from the suspended request enters the generic error handler before
that check. Because Task cancellation does not itself advance the Store-local
generation, the stale error can publish `offline/unavailable`.

The original Repair 7 contract already requires cancellation to prevent every
older page **or error** from changing Timeline or connection state, and already
owns `LocalProductStore.swift`. The minimal repair therefore reopens only that
original production file to require `!Task.isCancelled` in every error handler
before presentation mutation. This is not a new Amendment, schema, WorkItem,
authority or cursor mechanism.

The page-schema Store branch is exercised by constructing an invalid-schema
`LocalProductTimelinePage` from otherwise strictly decoded nested values. Wire
tests continue to prove the strict decoder itself rejects the same wire shape;
the Store test is defense-in-depth and does not relax decoding.
