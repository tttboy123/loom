# P3A-W1 Operational and Trace Behavior Review (SUPERSEDED — historical)

Status: `SUPERSEDED — HISTORICAL`. This record certified the v3/v4 root
generation (binaries daemon `de6c3087…`/GUI `74b1f32f…`, happy 147 events)
before the Whole-Candidate repair sequence rebuilt the final binaries and
re-ran all eight journeys on the final v5 `-r2` roots. The final-generation
certification chain is recorded in
`P3A-W1-DUAL-RESULT-REVIEWS.md` (Addendum 2026-08-04),
`P3A-W1-PRODUCT-RESULT-REVIEW-FINAL-V6.md`,
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT-REVIEW-4.md`,
`P3A-W1-OPERATIONAL-TRACE-REVIEW-FINAL-V5.md` and
`P3A-W1-WHOLE-CANDIDATE-REVIEW.md`. Do not read this file as the
final-generation certification.

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (`p3a_optrace_reviewer3`,
Codex CLI, read-only sandbox). Verdict delivered by the Reviewer and
persisted by the orchestrator as the gate artifact.

## Verdict

```text
P0 = 0
P1 = 0
P2 = 1 (non-blocking, documentation precision only)
Operational and Trace Behavior: PASS
Overall: PASS
```

Scope verified (read-only) against `P3A-W1-CONTRACT-REPAIR-1.md` §8, the
Alternative Verification Amendment, and the runbook, for all eight final
roots (v3 happy + crash-before-cas; v4 cancel-reject-retain,
stale-view-digest-generation, concurrent-single-winner,
crash-after-cas-before-response, projection-failure-rebuild-reconnect,
slow-client-redelivery-clean-restart):

1. **Schema conformance** — every JSONL and JSON file's keyset is exactly the
   §8/runbook set (`gui/actions.jsonl`, `tui/keystrokes.jsonl`,
   `timeline.jsonl`, `ipc/request-response-summary.jsonl`,
   `daemon/structured-log.jsonl`, processes pre/post with **no extra `root`
   field**, projection/artifacts/journal summaries, manifest with all 31 §8
   fields, `production_components` = kind/path_digest/sha256/build_id,
   `evidence_files` = relative_path/sha256/size/mode sorted by path).
2. **Journey correlation** — zero drift: every daemon, IPC, and timeline
   record carries the root's journey ID; daemon sequences are strictly
   monotonic and continue across restarts (per-process
   `monotonic_offset_micros` resets only at restart boundaries, which the
   contract permits).
3. **Dual-client coverage** — every root's IPC log contains both
   `client_kind=gui` (production Swift probe) and `client_kind=tui` traffic.
4. **GUI/TUI evidence** — every transcript is real PTY output containing
   `Loom ·`; every root has ≥1 genuine PNG screenshot (0600, real window
   resolutions).
5. **Postflight** — processes/sockets/locks/leases/temps all empty;
   cleanup-proof confirms socket absent, isolation residue 0, journey
   processes 0; `loomd.sock`/`loomd.sock.lock` absent.
6. **Permissions & hygiene** — roots 0700, all evidence files 0600; no
   `sk-*`/api-key/bearer/private-key material in any evidence.
7. **Manifest integrity** — `manifest_digest` self-consistent; all evidence
   files match on-disk digests/sizes/modes; aggregate digest fields
   cross-reference the evidence files; `product_result` /
   `operational_trace_result` = PASS; `network_allowed=false`,
   `provider_credentials_present=false`.
8. **Binary identity** — daemon `de6c3087…`, TUI `29e3775c…`, GUI
   `74b1f32f…` match both `production_components` and the on-disk binaries in
   every root.
9. **Independent SQLite audit** — integrity ok, event counts match, zero
   duplicate Event IDs/idempotency keys, zero stream gaps, zero FK
   violations, stream-heads match the DB, `journey_event_count` matches.
10. **Variant error paths recorded truthfully** — S3
    `stale_view`/`digest_mismatch` (zero writes), S4 one `conflict` loser, S5
    `before_cas crash`, S6 `after_cas_before_response crash` with exactly one
    committed activation and no redelivered mutation, S7 `post_commit_refresh
    fail state_unavailable` with old view preserved pre-restart and
    `matches_journal=true` post-restart, S8 delayed response with client
    timeout recorded and no hidden retry.

## Non-blocking P2 (documentation precision only)

In the happy and cancel-reject roots, `result.md` says the Journal is "all
correlated with the journey ID," while a small number of platform foundation
events carry their own causation IDs (identity-index initialization,
`RuntimeInstanceDiscovered`, and in happy the saved-team/agent setup). The
authoritative `journal/sqlite-summary.json` records this exactly (happy
141/147, cancel-reject 8/11 journey-correlated), and the daemon/IPC/timeline
logs carry the journey ID on every record — so this is wording imprecision,
not trace drift.

VERDICT: `PASS`
