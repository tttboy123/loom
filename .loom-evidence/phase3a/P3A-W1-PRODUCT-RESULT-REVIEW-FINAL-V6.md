# P3A-W1 Product Result Review — FINAL v6 (PASS)

Date: `2026-08-04`

Reviewer: independent read-only Reviewer (`p3a_product_result_reviewer_v6`).

Scope: the eight final journey roots listed in
`.loom-evidence/phase3a/P3A-W1/JOURNEY-ROOTS-INVENTORY.md` (final v5
generation with `-r2` crash replacements and corrected happy/cancel roots),
under `/private/tmp/loom-p3a-v5-roots/`. Reference contract:
`P3A-W1-CONTRACT-REPAIR-1.md` scenario/product assertions, the frozen
`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md`, and the frozen
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` GUI evidence surface.

All audits were performed read-only and independently of the candidate's
verify script (own jq/sqlite3 `-readonly`/sha256 checks over every root's
manifest, result.md, gui/actions.jsonl, ipc/request-response-summary.jsonl,
daemon/structured-log.jsonl, timeline.jsonl, journal/event-summary.json,
journal/sqlite-summary.json, projection/summary.json,
artifacts/digest-verification.json, processes/postflight.json,
cleanup-proof.txt and screenshots; the candidate's verify script was also
re-run and returned PASS for all eight roots).

## Verdict

```text
P0 = 0
P1 = 0
P2 = 0
Product Result: PASS
```

## Verified PASS (per-scenario product assertions)

1. **S1 happy-create-evaluate-activate-bind-execute-clean** — full lifecycle
   through the production Swift client over the real daemon socket
   (create/evaluate/activate/bind all `response_ok=true` in IPC) plus the
   real PTY TUI team build/Mission execution; Journal contains
   definition/revision/candidate/evaluation/activation/binding events,
   `RuntimeSkillMaterializationPublished` x2 with the terminal/cleanup
   chain; projection 1/1/1/1/1/2 with `matches_journal=true`; artifacts 8/8
   `match=true`; SQLite `integrity_check=ok`, zero duplicate Event IDs /
   idempotency keys, zero stream gaps, zero FK violations; 498 events,
   492 journey-correlated as stated in result.md (6 platform foundation
   events carry their own causation IDs, matching journal/sqlite-summary).
2. **S2 cancel-reject-retain** — two candidates created via TUI; the
   canceled reject produced zero `evolution_asset_command` and zero Journal
   facts; Journal has exactly one `EvolutionAssetCandidateRejected` and one
   `EvolutionAssetCandidateRetained`; probe cross-observes 2/2/2;
   result.md states the precise 8/11 journey correlation (3 platform
   foundation events).
3. **S3 stale-view-digest-generation** — IPC records both rejections:
   `evolution_asset_command` `response_ok=false` `error_code=stale_view`
   and `error_code=digest_mismatch`; Journal contains only the create chain
   (no evaluation/activation); projection evaluations=0 and
   `matches_journal=true`; gui/actions.jsonl records "stale_view rejected
   zero-write" / "digest_mismatch rejected zero-write" / "snapshot
   unchanged".
4. **S4 concurrent-single-winner** — exactly one
   `EvolutionAssetCandidateRejected` committed (IPC
   `evolution_asset_command` `response_ok=true`); the loser's concurrent
   reject received `response_ok=false` `error_code=conflict`; no other
   effects; gui/actions.jsonl distinguishes winner/loser.
5. **S5 crash-before-cas (-r2)** — daemon structured log row:
   `operation=activate, phase=before_cas, outcome=crash,
   error_code=unavailable, authority_event_ids=[]`; Journal has zero
   `EvolutionAssetCandidateActivated` (create+evaluation only); source
   constant `productJourneyCrashBeforeCASExitCode = 92` binds the
   result.md/gui exit-92 claim; restart read-only snapshot intact.
6. **S6 crash-after-cas (-r2)** — daemon structured log row:
   `operation=activate, phase=after_cas_before_response, outcome=crash,
   error_code=unavailable, authority_event_ids=["p3a-candidate_activated-…"]`;
   exactly one `EvolutionAssetCandidateActivated` committed; client
   received an error and did not synthesize success (gui/actions.jsonl
   "daemon exit 93 after commit; client error, no success synthesized");
   source constant `productJourneyCrashAfterCASExitCode = 93`; restart
   reads the single committed effect; no duplicate Event.
7. **S7 projection-failure-rebuild-reconnect** — activation committed once;
   daemon log `post_commit_refresh` failure; IPC records
   `error_code=state_unavailable`; result.md documents the pre-restart old
   immutable view (bindings 0 / materializations 0) and the post-restart
   rebuild with `matches_journal=true`; both clients observe the committed
   state after reconnect.
8. **S8 slow-client-redelivery-clean-restart** — activation committed once;
   gui/actions.jsonl and timeline show client timeout ("timeout shown; no
   hidden retry"); daemon-side eventual `response_ok=true` row is the
   server's delayed response and is not contradicted by the client-side
   truth; explicit refresh and clean restart observe exactly one effect
   with no duplicate Event/Evidence.

## GUI evidence surface (P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT §3)

- Every root has exactly two production-app-originated launch reads
  (`setup_snapshot` + `snapshot` with `loom-swift-<uuid>` request IDs) in
  `ipc/request-response-summary.jsonl`, journey-correlated, mirrored in
  `daemon/structured-log.jsonl` — satisfying the ≥2 launch-read records
  required per root (closes the prior F1 finding for both crash roots).
- Every root has at least one real 2200x1440 PNG screenshot stored 0600
  under `gui/screenshots/`; every `gui/actions.jsonl` record references a
  screenshot via `screenshot_relative_path` (closes the prior F5 finding).
- Journey-specific GUI-side behavior is proven by the production Swift
  client (`LocalIPCClient`/`LocalProductStore` via
  `LoomLocalAppContractProbe`) over the real socket, with truthful
  variant/error paths in IPC, daemon logs and gui/actions.jsonl, and by
  the `--assets <JOURNEY>` cross-observation read-back matching the
  projection summaries.
- The window's pixels are not claimed to reflect journey-specific state;
  per the frozen amendment the main window renders the default workbench
  view and captures are checkpoint-bound by wall-clock order, so
  per-journey visual difference is not required.

## Documentation precision

- happy result.md: precise 492/498 journey correlation (no stale 141/147);
  the unsupported in-root "clean restart" claim was removed (restart
  idempotency is evidenced by S5/S6/S7/S8 roots) — prior F3/F4 closed.
- cancel result.md: precise 8/11 journey correlation.

## No blocking findings

No P0/P1/P2 findings. The screenshots' identical blank-shell pixels are
explicitly bounded by the frozen GUI Evidence Surface Amendment (separately
under independent review); from the Product Result side the evidence
satisfies that amendment's defined surface.

VERDICT: `PASS` (P0=0, P1=0, P2=0)
