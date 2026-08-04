# P3A-W1 Product Result Review — FINAL v5 roots (FAIL)

Date: `2026-08-04`

Reviewer: independent read-only Reviewer (`p3a_product_result_final_reviewer`).
Scope: the eight final v5 journey roots under
`/private/tmp/loom-p3a-v5-roots/` as listed in
`.loom-evidence/phase3a/P3A-W1/JOURNEY-ROOTS-INVENTORY.md`
(happy-r2 `4a3c3986…`, cancel-reject-retain-r2 `e50e7ca2…`,
stale-view `532849b1…`, concurrent-single-winner `003e8570…`,
crash-before-cas `10938833…`, crash-after-cas `c36fb6e5…`,
projection-failure `31c76a1d…`, slow-client `54fae40e…`), plus the two
newer `-r2` crash replacement roots (`f1ab90a6…`, `0e241370…`) created
during the review window.

Reference contract: `P3A-W1-CONTRACT-REPAIR-1.md` (scenario and product
assertions), `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` §3/§6
(alternative GUI method and its product claims), the journey runbook and
the source-lock manifest. All audits were performed read-only and
independently of the candidate's own verify script.

## Verdict

```text
P0 = 0
P1 = 2
P2 = 3
Product Result: FAIL
Overall: FAIL
```

The eight scenarios' data-level product assertions (Journal cardinality,
event types, CAS winner, crash seams, projection preservation/rebuild,
timeout semantics, artifact resolution, SQLite integrity) are all
consistent with the frozen product contract and with each root's own
`result.md`. However, two product claims required by the Alternative
Verification Amendment are not supported by the evidence: (1) the
"real window's visual state" claim rests on byte-identical template
screenshots that cannot be authenticated as per-journey captures; and
(2) in the two crash roots of the documented final set, the production
native app never connected, so the cross-client restart claim is not
met by the app client. Documentation-precision issues also remain in
two `result.md` files.

## Verified PASS (data-level product assertions, all eight roots)

1. **S1 happy** — create/evaluate/activate/bind all `ok=true` through the
   production Swift client over the real daemon socket; the real PTY TUI
   drove team build and Mission execution (builder_start/builder_answer/
   builder_confirm, preflight, explicit start, "Execution accepted ·
   Running"); Journal shows the full lifecycle with
   `RuntimeSkillMaterializationPublished` ×2 and `...Cleaned` ×2,
   `EvidenceSubmitted` ×3, `RunTerminalCommitted` ×3,
   `WorkItemReadyForReview` ×2 and `WorkItemDone` ×1; projection
   1/1/1/1/1/2 with `matches_journal=true`; artifacts 8/8
   `match=true`; SQLite integrity ok, 498 events, 492 journey-correlated,
   zero duplicate Event IDs/idempotency keys, zero stream gaps.
2. **S2 cancel-reject-retain** — TUI keystrokes show create A/B, `x`
   pending reject, `esc` cancel with **zero** mutation (no
   `evolution_asset_command` for the canceled action and no extra Journal
   fact), `x`+`y` reject A committed, `h`+`y` retain B committed; Journal
   has exactly one `EvolutionAssetCandidateRejected` and one
   `EvolutionAssetCandidateRetained`; probe reads 2/2/2.
3. **S3 stale-view-digest-generation** — base candidate created via TUI;
   probe `activate` with `stale_view` rejected
   (`error_code=stale_view`, `response_ok=false`) and with `wrong_digest`
   rejected (`error_code=digest_mismatch`); Journal contains only the
   create chain (no evaluation/activation), evaluations=0, undecided.
4. **S4 concurrent-single-winner** — one candidate; two concurrent
   rejects: exactly one `EvolutionAssetCandidateRejected` committed
   (`response_ok=true`), the loser received `error_code=conflict`; no
   other effects.
5. **S5 crash-before-cas (-r2 replacement)** — `activate` triggered the
   before-CAS seam (daemon exit code 92 constant
   `productJourneyCrashBeforeCASExitCode`); zero
   `EvolutionAssetCandidateActivated`; restart read-only recovery shows
   the create+evaluation state intact; isolation recovery clean.
6. **S6 crash-after-cas (-r2 replacement)** — `activate` committed exactly
   one `EvolutionAssetCandidateActivated`, then the daemon exited at the
   after-CAS seam (exit code 93 constant
   `productJourneyCrashAfterCASExitCode`); client received an error and
   did not synthesize success; restart reads the single committed effect.
7. **S7 projection-failure** — `activate` committed once; daemon log
   records `phase=post_commit_refresh, outcome=fail,
   error_code=state_unavailable`; pre-restart read-back (recorded in
   `result.md`) shows the old immutable view (`bindings:0,
   materializations:0`, no activation); post-restart projection
   `matches_journal=true` with the activation visible.
8. **S8 slow-client** — harness `fault_kind=slow_response,
   fault_action=activate, delay_millis=7000`; `activate` committed once;
   the client-side record (`gui/actions.jsonl`, timeline) shows
   `timeout shown; no hidden retry` and never synthesized success; the
   daemon-side eventual response row (`response_ok=true` after the
   client-deadline cancellation) does not contradict the client-side
   truth; explicit refresh and clean restart observe exactly one effect
   with no duplicate Event/Evidence.

Across all roots: projection `matches_journal=true`; SQLite
`integrity_check=ok`, zero duplicate Event IDs, zero duplicate
idempotency keys, zero stream gaps, zero FK violations; artifact digests
all `match=true`; real PTY transcripts containing `Loom ·`; IPC logs
contain both production Swift client (probe; plus app `loom-swift-<uuid>`
rows in the non-crash roots and in the `-r2` crash roots) and TUI
traffic.

## Blocking findings

### F1 (P1) — Screenshot evidence is not authentic per-journey capture

`gui/screenshots/01-main-window.png` is byte-identical (SHA-256
`c1c9b6a9…`, 162,322 bytes, 2200×1440 default blank shell) across nine
roots including happy-r2, cancel-reject-retain-r2, stale-view,
concurrent, projection-failure, slow-client and **crash-before-cas-r2**,
and is byte-identical to screenshots in the superseded v3/v4 generations
(e.g. v4 cancel-reject-retain and projection-failure roots). The original
v5 crash roots share a second identical image (`2270a113…`); the
crash-after-cas-r2 root's single image (`b0c58e0a…`, 163,148 bytes) is
also a blank-shell 2200×1440 capture with no capture timestamp.
`02-after-actions.png` is byte-identical to `01-main-window.png` in the
happy and concurrent roots. `gui/app-console.log` is empty in every root
and no `gui/actions.jsonl` record references a screenshot
(`screenshot_relative_path` empty everywhere).

Consequence: the Alternative Verification Amendment §6 product claim that
"the real window's visual state … [is] exercised" is not supported, and
each `result.md`'s GUI real-window checkpoint claim cannot be
authenticated as a per-journey capture of journey-specific window state.
This applies to all eight roots of the final set.

### F2 (P1) — Production app never connected in the two documented crash roots

In the inventory's final crash roots (`crash-before-cas` `10938833…` and
`crash-after-cas-before-response` `c36fb6e5…`), `ipc/…jsonl`,
`daemon/structured-log.jsonl` and `timeline.jsonl` contain zero
app-originated requests (`loom-swift-<uuid>` launch reads are present at
the top of the happy, cancel, stale-view and concurrent roots but absent
throughout both crash roots; every GUI row is `loom-swift-contract-probe`).
The production app therefore never connected in the crash scenarios, and
the product claim that restart/reconnect is observed by "both clients"
is satisfied only by the probe and TUI. The `-r2` replacement roots
(`f1ab90a6…`, `0e241370…`, created during this review window) do launch
and connect the app and verify PASS, but they are not the documented
final set and the inventory has not been updated.

## Non-blocking findings

### F3 (P2) — cancel-reject-retain result.md correlation wording

`cancel-reject-retain-r2/result.md` states "all correlated with the
journey ID", but 3 of its 11 Journal events carry platform-foundation
correlation IDs (two sentinel IDs and the runtime-discovery ID), i.e.
8/11 are journey-correlated. The happy root was corrected at
2026-08-04T18:02Z to state 492/498 precisely; the cancel root retains the
stale wording of the previously closed P2a finding.

### F4 (P2) — happy result.md "clean restart" claim not evidenced in-root

`happy-create-evaluate-activate-bind-execute-clean-r2/result.md` claims
"Clean restart adds no duplicate fact", but the root contains no daemon
restart: the daemon structured log has zero monotonic-offset reset
boundaries and there is no restart action/snapshot row in
`gui/actions.jsonl`, `timeline.jsonl` or `ipc/…jsonl`. Restart
idempotency is demonstrated by the S5/S6/S7/S8 roots and by the
duplicate-idempotency-key checks, but not by this root's evidence, so the
narrative overstates what the root demonstrates.

### F5 (P2) — screenshots are not checkpoint-bound

Every `gui/actions.jsonl` record has an empty `screenshot_relative_path`,
so even where a screenshot exists it is not bound to a driven action, and
the freeze contract's checkpoint-capture intent is not evidenced.

## Recommended next step

Do not stage or commit P3A-W1 as accepted. (a) Update the final root set
to the `-r2` crash replacements and the corrected happy root, and refresh
`JOURNEY-ROOTS-INVENTORY.md`; (b) re-run the journeys (or otherwise
re-capture evidence) with authentic per-journey `screencapture` of the
production window at action checkpoints, referenced from
`gui/actions.jsonl`; (c) regenerate `cancel-reject-retain-r2/result.md`
with precise 8/11 correlation counts; (d) either evidence the happy-root
clean-restart check or remove the unsupported claim; (e) align the runbook
with the app's actual `--socket` argument. Then re-run the dual Result
gate.

VERDICT: `FAIL` (P0=0, P1=2, P2=3)
