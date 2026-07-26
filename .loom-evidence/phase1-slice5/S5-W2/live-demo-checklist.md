# Phase 1 Live Demo Human Gate

Status: NON-EXECUTING CHECKLIST

This checklist describes the final human-controlled live gate. It does not
authorize or perform execution and does not claim that a real Runtime task has
passed.

## Preflight

- [ ] Select and inspect the installed Pi Runtime instance.
- [ ] Confirm `loom.bridge.v1`, streaming, local workspace, and no network.
- [ ] Select a private source/evidence root and verify directories are `0700`.
- [ ] Confirm saved Team `saved-team.phase1-engineering`.
- [ ] Confirm Coding WorkPackage version 1 and exact frozen digest.
- [ ] Confirm the bounded task has at most three nodes and three attempts per
  node.

## Approval

- [ ] User explicitly confirms the saved Team and WorkPackage.
- [ ] Customer Rule decisions are read from the Journal.
- [ ] Any required start-Run approval is explicitly decided before dispatch.
- [ ] Live execution authorization is separately recorded by the user.

## Stop and cancel

- [ ] Client delivery can disconnect without cancelling the Run.
- [ ] Run cancellation uses the authoritative cancellation API.
- [ ] Terminal state and Grant revocation are verified from authoritative
  facts.
- [ ] The daemon is neither killed nor restarted as a cancellation shortcut.

## Evidence

- [ ] Authorized output only is shown; tentative deltas remain non-authoritative.
- [ ] Evidence digest, Run lineage, generation, verifier isolation, and terminal
  acceptance are verified.
- [ ] Evidence files are `0600`; no Grant, credential, prompt, or hidden
  reasoning is stored.

## Restart and reconnect

- [ ] Reconnect begins strictly after the last accepted cursor.
- [ ] A fresh Projection rebuilds solely from the Journal.
- [ ] Recovery produces no duplicate Run, Grant, Evidence, Done, or external
  effect.
- [ ] Indeterminate or stale state stops at `human_required`.

## Rollback and final signature

- [ ] Roll back only through accepted state transitions; never edit Journal
  history.
- [ ] Rebuild the Projection and re-check Evidence after rollback/recovery.
- [ ] Record whole-Slice Reviewer PASS.
- [ ] Obtain the user's final review sign-off.

VERDICT: PASS
