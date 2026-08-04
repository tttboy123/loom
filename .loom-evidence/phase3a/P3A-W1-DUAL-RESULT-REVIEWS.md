# P3A-W1 Dual Result Independent Review Chain (final)

Date: `2026-08-04`

Scope: the eight final cross-client journey roots (all frozen and verified by
`scripts/run-phase3a-cross-client-journey.sh verify --root`), Product Result
and Operational/Trace Behavior, read-only independent Reviewers.

## Review A — Operational and Trace Behavior (PASS)

```text
P0 = 0  P1 = 0  P2 = 1 (non-blocking, documentation precision)
Operational and Trace Behavior: PASS
```

Verified across all eight roots: §8 evidence schema conformance (including
no extra `root` field in processes pre/postflight); journey correlation on
every daemon/IPC/timeline record with zero drift and monotonic daemon
sequences across restarts; dual-client coverage (gui probe + tui) in every
IPC summary; real PTY transcripts containing "Loom ·"; real native-window
PNG screenshots (0600); postflight arrays all empty with cleanup-proof and
no socket/lock residue; 0700/0600 permissions; secret hygiene; manifest
digest self-consistency with matching evidence digests/sizes/modes;
`network_allowed=false` and `provider_credentials_present=false`; frozen
binary digests (daemon de6c3087…, TUI 29e3775c…, GUI 74b1f32f…) matching
`production_components`; independent SQLite audit (integrity ok, zero
duplicate Event IDs/idempotency keys, zero stream gaps, zero FK violations);
truthful variant error paths for S3/S4/S5/S6/S7/S8.

Original P2: result.md "all correlated with the journey ID" wording in the
happy and cancel-reject roots vs. the authoritative `journey_event_count`
(141/147, 8/11). **CLOSED**: the two result.md files now state the precise
counts and the platform-foundation-event exception; both roots were
re-frozen and re-verified PASS.

## Review B — Product Result (PASS)

```text
P0 = 0  P1 = 0  P2 = 2 (non-blocking)
Product Result: PASS
```

Verified all eight scenario product assertions against result.md, daemon/IPC
logs, journal event summaries, projection and artifact verification:

- S1 happy: full lifecycle create→evaluate→activate→bind→preflight→start→
  materialize (2 Pi runs)→Evidence→accept→terminal→clean; 147 events;
  projection 1/1/1/1/1/2 matches_journal=true; terminal chain present.
- S2 cancel-reject-retain: exactly 2 creates, 1 reject, 1 retain; the
  canceled action produced no command and no fact.
- S3 stale-view-digest-generation: stale_view and digest_mismatch
  activation attempts rejected with zero new Journal facts; read-back
  undecided, evaluations=0.
- S4 concurrent-single-winner: exactly one `EvolutionAssetCandidateRejected`;
  loser got a `conflict` error.
- S5 crash-before-cas: before-CAS crash (exit 92), zero activation event;
  restart read-only recovery intact.
- S6 crash-after-cas-before-response: exactly one activation committed,
  after-CAS crash (exit 93), client error with no synthesized success;
  restart reads the single effect.
- S7 projection-failure-rebuild-reconnect: one activation; post_commit_refresh
  fail; old view preserved pre-restart; post-restart matches_journal=true.
- S8 slow-client-redelivery-clean-restart: one activation; client timeout
  (5s deadline vs 7s delay), no hidden retry; refresh and clean restart show
  one effect with no duplicate Event/Evidence.

Original P2a: S1 result.md "all correlated" wording (same as Review A).
**CLOSED** as above.

Original P2b: S8 daemon-side IPC row for the delayed activate records
`response_ok=true` after the 7s delay, which could be misread as client
success. **CLOSED**: the S8 result.md now explicitly explains that the
daemon-side row is the server's eventual response while the client-side
truth (timeout, no synthesized success, no retry) is in gui/actions.jsonl
and the timeline; the root was re-frozen and re-verified PASS.

## Final verdict

```text
Product Result: PASS
Operational and Trace Behavior: PASS
P0 = 0  P1 = 0  P2 = 0 (documented closures)
```

## Addendum (2026-08-04) — final-generation certification (v5 `-r2` roots)

The Review A/B chain above originally certified the v3/v4 generation. After
the Whole-Candidate repair sequence (isolation-recovery journey-root
derivation fix; final binaries rebuilt from final source; all eight journeys
re-run and re-frozen on fresh v5 roots with the `-r2` crash replacements;
source-lock regenerated `0d5d4296…`; auxiliary files 0600; result.md counts
corrected) the FINAL generation is certified as follows:

- **Final roots**: the eight roots listed in
  `P3A-W1/JOURNEY-ROOTS-INVENTORY.md` (final-v5 section) with frozen
  binaries daemon `5bd5d86a…`, TUI `29e3775c…`, GUI `87c37812…`, all
  re-verified PASS by `scripts/run-phase3a-cross-client-journey.sh verify
  --root`.
- **GUI evidence surface**: the screenshot visual-state claim is now
  precisely defined by the frozen
  `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` (independent Reviews 3 and 4,
  final Review 4 `P0=0 P1=0 P2=0` PASS); per-journey GUI behavior is proven
  by the production Swift client IPC records (≥2 `loom-swift-<uuid>` launch
  reads per root) and the read-back rows in `gui/actions.jsonl`.
- **Product Result (final)**: independent Product Result Review
  (`P3A-W1-PRODUCT-RESULT-REVIEW-FINAL-V6.md`) returned
  `P0=0 P1=0 P2=0` PASS on all eight final roots.
- **Operational and Trace Behavior (final)**: independent
  Operational/Trace Review on the final roots
  (`P3A-W1-OPERATIONAL-TRACE-REVIEW-FINAL-V5.md` and the final
  generation certification chain) PASS with the F1 (crash-root app
  connection, closed by `-r2`), F2 (GUI evidence surface, closed by the
  Amendment), F3 (result.md counts, closed) and F4 (runbook wording,
  closed) findings resolved.
- **Whole-Candidate (final)**: `P3A-W1-WHOLE-CANDIDATE-REVIEW.md`
  `P0=0 P1=0 P2=2 (documentation closures, this addendum + supersession
  markers)` PASS; exact staging inventory ready with
  `internal/projection/team_execution_test.go` excluded.

This Addendum supersedes the earlier-generation digests and event counts in
the Review A/B text above for the purpose of final acceptance.

## Closure addendum (2026-08-04) — Whole-Candidate Review P2-1/P2-2

Dated completion of the two documentation closures required by
`P3A-W1-WHOLE-CANDIDATE-REVIEW.md` (P2-1 and P2-2). No product code,
authority, schema, scenario assertion, journey root or evidence file was
changed; only this review record, the inventory supersession marker and
`docs/CURRENT.md` were updated.

1. **Final-generation binary digests** (re-asserted): daemon
   `5bd5d86ab918abd0f966c44105e3793b3fc5f32664a415209ea1987b7ed706b3`, TUI
   `29e3775c541d5264e4db5507719d555739524f31d4934ab8d9ef313bcb1813b3`, GUI
   `87c378122909eaf06aea78415e8fe9f941cf371b78d577cee38bf8c52fa14478` —
   matching every final root's `production_components` and fresh builds from
   the accepted source state.
2. **The eight final `-r2` roots** (under `/private/tmp/loom-p3a-v5-roots/`):
   `happy-create-evaluate-activate-bind-execute-clean-r2`
   (journey `4a3c3986-3bcf-4d65-973a-b0bb47d7c204`),
   `cancel-reject-retain-r2` (`e50e7ca2-308b-4f7f-9ad7-13b28ebeb9ad`),
   `stale-view-digest-generation` (`532849b1-ba8c-4733-a34c-5b80949e5c80`),
   `concurrent-single-winner` (`003e8570-5422-4c87-b25a-ea2d1b0b9037`),
   `crash-before-cas-r2` (`f1ab90a6-1508-46dd-9835-cc9ea7b64d09`),
   `crash-after-cas-before-response-r2` (`0e241370-b085-45a6-ab85-9f95159d26c6`),
   `projection-failure-rebuild-reconnect` (`31c76a1d-2256-4cb9-82c0-12628e0a8161`),
   `slow-client-redelivery-clean-restart` (`54fae40e-e71a-434a-b2ad-a800e4c2d584`).
   Happy root: **498 Journal events, 492 journey-correlated** (the remaining
   6 are platform foundation events with their own causation IDs), matching
   `result.md`, `journal/sqlite-summary.json` and the independent SQLite audit.
3. **Supersession record**: the Review A/Review B text above certified the
   superseded v4 generation (daemon `de6c3087…`/GUI `74b1f32f…`, happy 147
   events); it is retained for the audit trail and is superseded for final
   acceptance by the first Addendum and this closure addendum.
4. **Final-generation certification chain**: Product Result Review
   `P3A-W1-PRODUCT-RESULT-REVIEW-FINAL-V6.md` PASS (P0=P1=P2=0, final roots);
   Operational and Trace Behavior Review
   `P3A-W1-OPERATIONAL-TRACE-REVIEW-FINAL-V6.md` PASS (P0=P1=P2=0, final v5
   `-r2` roots; supersedes the V5 FAIL record); GUI Evidence Surface
   Amendment Review 4 PASS (P0=P1=P2=0, on-disk bytes SHA-256
   `4461f1d72f9b3c77f8aeac0b4b2a720a0a772299c583682c6a3690d4dc240e09` equal
   to the reviewed digest); Whole-Candidate Review PASS (P0=0 P1=0 P2=2)
   with these closures.
5. **Verify re-run**: `scripts/verify-phase3a-cross-client-journey.sh` was
   re-executed read-only over all eight final roots on `2026-08-04` after the
   review chain — **PASS for all eight (exit 0 each)**, scenario IDs and
   journey IDs identical to the inventory.
6. **P2-2 supersession marker**: `P3A-W1/OPERATIONAL-TRACE-REVIEW.md` is
   titled and marked `SUPERSEDED — HISTORICAL` (v3/v4 generation) and is
   referenced as superseded in `P3A-W1/JOURNEY-ROOTS-INVENTORY.md`; the
   final-generation Operational/Trace certification is the chain in item 4
   (V6 Product Result Review, Amendment Review 4 and the Whole-Candidate
   Review's independent final-root Operational/Trace audit).

The Whole-Candidate Review closures are now complete; the candidate is ready
for exact staging and one atomic local commit.
