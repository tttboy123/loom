# SF-W3 cross-client journey

This runbook is the cross-client Exit Gate journey for the single `SF-W3`
(Single-writer Integration + controlled self-host canary +
Timeline/Attention). It follows the accepted alternative-verification method
(Product Owner instruction 2026-08-04): production native window launched
with `--socket --journey-id`, real PTY TUI, mutations/reads through the
production Swift client over the real daemon socket; Computer-Use-driven
window automation is skipped.

## Scenario assertions (frozen)

1. One reviewed Candidate is integrated by the single Integrator
   (`IntegrationStarted` + `CandidateIntegrated`); a competing stale
   integration on the same target branch loses via CAS with zero effects.
2. The controlled self-host canary runs once offline with exact
   runtime/model/skill bindings (`CanaryStarted` + `CanaryCompleted`); a
   duplicate start is blocked by idempotent CAS.
3. Streaming node output is published only when authorized and
   generation-bound; unauthorized/stale/malformed frames are rejected with
   zero effects; Timeline/Attention are projections, never an authority.
4. A later Run binds the exact release revision (`ReleaseAdoptedByLaterRun`);
   rollback restores the prior eligible version (`ReleaseRolledBack`)
   without rewriting Run/Evidence history.
5. Restart/reconnect checkpoint: the daemon restarts mid-journey and both
   clients observe the rebuilt release/canary/timeline/attention state with
   no duplicate Event/Evidence/effect.

## Driving commands (production Swift client over the real socket)

```sh
PROBE=apps/macos/.build/arm64-apple-macosx/release/LoomLocalAppContractProbe
$PROBE --socket ROOT/loomd.sock --integration-snapshot JOURNEY
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY integrate     INPUT.json
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY start_canary  INPUT.json
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY publish_frame INPUT.json
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY adopt         INPUT.json
$PROBE --socket ROOT/loomd.sock --integration-command JOURNEY rollback      INPUT.json
```

The real PTY TUI navigates to the Integration screen (Tab through the screen
cycle, `r` refresh, `q` quit) and both clients observe the same projection.
The GUI evidence surface follows the frozen
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md`.
