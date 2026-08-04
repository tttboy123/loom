# P3A-W1 Cross-client Journey Roots Inventory (final — v5)

Date: `2026-08-04`

Status: `COMPLETE — ALL 8 SCENARIOS FROZEN AND VERIFIED PASS (FINAL BINARIES)`

Verification method: `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT` (frozen,
independent Review PASS). Mutations and reads are driven through the
production Swift client (`LocalIPCClient`/`LocalProductStore` via
`LoomLocalAppContractProbe`) over the real daemon socket, the production
native window is launched with `--socket --journey-id` and captured with
`screencapture`, and the production TUI is driven through a real PTY. The
GUI evidence surface (app launch reads, checkpoint-bound screenshots,
production Swift client records and cross-observation read-back; no claim
that the main-window pixels differ per journey) is precisely defined by the
frozen `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` (SHA-256
`4461f1d72f9b3c77f8aeac0b4b2a720a0a772299c583682c6a3690d4dc240e09`,
Independent Review 3 PASS and Review 4 final PASS with P0=P1=P2=0).

## Final roots (verified PASS, Product Result PASS / Operational Trace PASS)

All roots use the FINAL binaries rebuilt from the accepted source state:
daemon `5bd5d86ab918abd0f966c44105e3793b3fc5f32664a415209ea1987b7ed706b3`,
TUI `29e3775c541d5264e4db5507719d555739524f31d4934ab8d9ef313bcb1813b3`,
GUI `87c378122909eaf06aea78415e8fe9f941cf371b78d577cee38bf8c52fa14478`
(built from the final Swift source including the public
`sha256Text`/`canonicalEvolutionAssetDigests` surface), baseline commit
`6d380233b5b89309a1a7ce3919aa611654e0f4ee`.

| Scenario | Root | Journey ID | Verified |
|---|---|---|---|
| happy-create-evaluate-activate-bind-execute-clean | /private/tmp/loom-p3a-v5-roots/happy-create-evaluate-activate-bind-execute-clean-r2 | 4a3c3986-3bcf-4d65-973a-b0bb47d7c204 | PASS |
| cancel-reject-retain | /private/tmp/loom-p3a-v5-roots/cancel-reject-retain-r2 | e50e7ca2-308b-4f7f-9ad7-13b28ebeb9ad | PASS |
| stale-view-digest-generation | /private/tmp/loom-p3a-v5-roots/stale-view-digest-generation | 532849b1-ba8c-4733-a34c-5b80949e5c80 | PASS |
| concurrent-single-winner | /private/tmp/loom-p3a-v5-roots/concurrent-single-winner | 003e8570-5422-4c87-b25a-ea2d1b0b9037 | PASS |
| crash-before-cas | /private/tmp/loom-p3a-v5-roots/crash-before-cas-r2 | f1ab90a6-1508-46dd-9835-cc9ea7b64d09 | PASS |
| crash-after-cas-before-response | /private/tmp/loom-p3a-v5-roots/crash-after-cas-before-response-r2 | 0e241370-b085-45a6-ab85-9f95159d26c6 | PASS |
| projection-failure-rebuild-reconnect | /private/tmp/loom-p3a-v5-roots/projection-failure-rebuild-reconnect | 31c76a1d-2256-4cb9-82c0-12628e0a8161 | PASS |
| slow-client-redelivery-clean-restart | /private/tmp/loom-p3a-v5-roots/slow-client-redelivery-clean-restart | 54fae40e-e71a-434a-b2ad-a800e4c2d584 | PASS |

Each root contains the frozen evidence bundle (result.md, gui/actions.jsonl
and screenshots, tui/transcript.txt and keystrokes.jsonl, timeline.jsonl,
ipc/request-response-summary.jsonl, daemon/structured-log.jsonl, journal
summaries, projection/summary.json, artifacts/digest-verification.json,
processes pre/postflight and cleanup-proof.txt, manifest.json) with 0700/0600
identities and was re-verified by
`scripts/run-phase3a-cross-client-journey.sh verify --root`.

Scenario assertions verified per root:

- S1 happy: full lifecycle create→evaluate→activate→bind→preflight→start→
  materialize (Pi ×2)→Evidence→accept→terminal→clean; 498 events; projection
  1/1/1/1/1/2 matches journal (journey_event_count 492/498).
- S2 cancel-reject-retain: TUI cancel produced zero commands/facts; one
  reject and one retain committed; Swift probe cross-observes 2/2/2.
- S3 stale-view-digest-generation: stale_view and wrong_digest activation
  attempts rejected (`error:stale_view:true`, `error:digest_mismatch:false`)
  with zero new Journal facts.
- S4 concurrent-single-winner: two concurrent rejects, exactly one CAS
  winner (one `EvolutionAssetCandidateRejected`); loser got a
  conflict/stale error.
- S5 crash-before-cas: activate crashed at the before-CAS seam (exit 92),
  zero activation event; read-only restart recovery intact.
- S6 crash-after-cas-before-response: activate committed once then crashed
  at the after-CAS seam (exit 93); client received `error:invalid_response`
  without synthesizing success; restart reads the single committed effect.
- S7 projection-failure-rebuild-reconnect: activate committed once; the
  controlled post-commit refresh failed (`post_commit_refresh fail
  state_unavailable`); old view preserved pre-restart; restart rebuilt the
  committed state from Journal for both clients.
- S8 slow-client-redelivery-clean-restart: activate committed once; the 7s
  controlled delay exceeded the client 5s deadline (`error:timeout`), no
  hidden retry; explicit refresh/reconnect and clean restart observe one
  effect with no duplicate Event/Evidence.

## Superseded / historical roots (immutable, not part of final acceptance)

The earlier v1–v4 root generations under `/private/tmp/loom-p3a-v2-roots`,
`/private/tmp/loom-p3a-v3-roots` and `/private/tmp/loom-p3a-v4-roots`, and
the superseded first v5 runs (happy `488d1364…` and cancel-reject-retain
`5f0f6672…`), are retained as immutable historical material only. They bound
earlier binary generations (daemon `de6c3087…`/GUI `74b1f32f…`) or an
intermediate evidence state; only the v5 roots above bind the final source.

The review record `P3A-W1/OPERATIONAL-TRACE-REVIEW.md` is likewise marked
`SUPERSEDED — HISTORICAL` (it certified the v3/v4 generation). The
final-generation Operational/Trace certification chain is recorded in
`P3A-W1-DUAL-RESULT-REVIEWS.md` (Addendum and Closure addendum 2026-08-04),
`P3A-W1-OPERATIONAL-TRACE-REVIEW-FINAL-V6.md` (final PASS, superseding the
V5 FAIL record), `P3A-W1-PRODUCT-RESULT-REVIEW-FINAL-V6.md`,
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT-REVIEW-4.md` and
`P3A-W1-WHOLE-CANDIDATE-REVIEW.md`.

## Execution contract reference

Daemon start, harness manifest, runtime fixture paths and the alternative
verification evidence contract are frozen in
`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md`,
`docs/runbooks/phase3a-cross-client-journey.md`, `runtime-fixture.json` and
`P3A-W1-CONTRACT-REPAIR-1.md` §8.
