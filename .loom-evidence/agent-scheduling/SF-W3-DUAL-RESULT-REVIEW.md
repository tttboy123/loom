# SF-W3 Dual Result Review (PASS)

Date: `2026-08-05`

Scope: `/private/tmp/sf3-journey-final` (journey
`addcee11-8224-40db-b665-fa6a40a82e22`).

## Verdict

```text
P0 = 0
P1 = 0
P2 = 0
Product Result: PASS
Operational and Trace Behavior: PASS
```

Product (verified): journal contains exactly IntegrationStarted ×1,
CandidateIntegrated ×1, CanaryStarted ×1, CanaryCompleted ×1,
NodeOutputFramePublished ×2, ReleaseAdoptedByLaterRun ×1,
ReleaseRolledBack ×1; single Integrator CAS winner with competing stale
integration rejected; canary one-shot with duplicate blocked;
unauthorized frame denied / authorized frame published; later Run adopted;
rollback restored prior version; projection-failure preserve demonstrated
(old view kept while fault active, rebuilt after clearing); projection
matches journal; result.md truthful.

Operational/Trace (verified): §8-style evidence schema; zero journey drift;
dual-client coverage with app-originated loom-swift rows; real PTY
transcript with "Loom ·" and the Integration screen; real 0600 screenshot;
postflight arrays empty with cleanup-proof and no socket/lock residue;
0700/0600 permissions; secret hygiene clean; restart/reconnect checkpoint
rebuilt the identical state; `scripts/verify-sf3-cross-client-journey.sh`
PASS.

VERDICT: `PASS`
