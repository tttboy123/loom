# P2A-W1 Native App Host Review Repair 1 RED

**Date**: 2026-07-28
**Status**: RED captured
**Parent**: `native-app-host-implementation-review-1.md`

The focused Swift tests exited `1`:

```text
testRequestIDGrammarIsStrictASCII:
XCTAssertFalse failed for "réquest-1"

testSanitizeNormalizesSingleLineWhitespace:
"alpha\nbeta\tgammadelta" != "alpha beta gammadelta"
```

The build fixture exited `1` because the shadow builder did not emit the
required closed `forbidden native app source surface` rejection before later
build inputs.

The installer fixture exited `1` with:

```text
injected app installation signal unexpectedly succeeded
```

The fourth RED is a bounded same-contract safety strengthening: an interrupt
after the bundle swap must restore both current and previous app transactions
exactly rather than silently succeeding or deleting rollback state.

A final same-contract UI-state RED failed to compile because
`LocalProductConnectionState` had no `fatal` member. This fixes the contract's
explicit distinction between recoverable/offline and nonrecoverable daemon
errors before Re-review.

No installed app, resident daemon, Journal, Provider, Runtime, staging, or live
authority changed during RED.
