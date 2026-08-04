# SF-W2 Dual Result Review (PASS)

Date: `2026-08-05`

Scope: `/private/tmp/sf2-journey-final` (journey
`d0d5f598-617b-4bb9-b0bb-81162c83dbbc`).

## Verdict

```text
P0 = 0
P1 = 0
P2 = 0
Product Result: PASS
Operational and Trace Behavior: PASS
```

Product (verified): journal contains exactly AttemptClaimed ×3,
AttemptCrashed ×1 (after_cas, single_effect), LeaseReclaimed ×1,
GenerationAdvanced ×1, StaleResultRejected ×1, AttemptResultRecorded ×1;
two independent workers claimed in parallel; crash reclaimed with exactly
one new generation; stale result rejected; test_defect failure recorded;
Reviewer write denied; projection matches journal; result.md truthful.

Operational/Trace (verified): §8-style evidence schema; zero journey drift;
dual-client coverage with app-originated loom-swift rows; real PTY
transcript with "Loom ·" and the Workers screen; real 0600 screenshot;
postflight arrays empty with cleanup-proof and no socket/lock residue;
0700/0600 permissions; secret hygiene clean; restart/reconnect checkpoint
rebuilt the identical state; `scripts/verify-sf2-cross-client-journey.sh`
PASS.

VERDICT: `PASS`
