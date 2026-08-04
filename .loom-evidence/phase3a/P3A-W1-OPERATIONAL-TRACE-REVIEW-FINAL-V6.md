# P3A-W1 Operational and Trace Behavior Review — FINAL v6 (PASS)

Date: `2026-08-04`

Reviewer: independent read-only verification pass on the FINAL v5 `-r2`
generation (orchestrator + independent reviewers). Verdict delivered after
the Whole-Candidate repair sequence and re-freeze of all eight final roots.

## Verdict

```text
P0 = 0
P1 = 0
P2 = 0
Operational and Trace Behavior: PASS
```

## Scope and method

Audited read-only, independently of the candidate's own verify script, the
eight FINAL v5 roots under `/private/tmp/loom-p3a-v5-roots`:

| Scenario | Root | Journey ID |
|---|---|---|
| happy-create-evaluate-activate-bind-execute-clean | happy-create-evaluate-activate-bind-execute-clean-r2 | 4a3c3986-3bcf-4d65-973a-b0bb47d7c204 |
| cancel-reject-retain | cancel-reject-retain-r2 | e50e7ca2-308b-4f7f-9ad7-13b28ebeb9ad |
| stale-view-digest-generation | stale-view-digest-generation | 532849b1-ba8c-4733-a34c-5b80949e5c80 |
| concurrent-single-winner | concurrent-single-winner | 003e8570-5422-4c87-b25a-ea2d1b0b9037 |
| crash-before-cas | crash-before-cas-r2 | f1ab90a6-1508-46dd-9835-cc9ea7b64d09 |
| crash-after-cas-before-response | crash-after-cas-before-response-r2 | 0e241370-b085-45a6-ab85-9f95159d26c6 |
| projection-failure-rebuild-reconnect | projection-failure-rebuild-reconnect | 31c76a1d-2256-4cb9-82c0-12628e0a8161 |
| slow-client-redelivery-clean-restart | slow-client-redelivery-clean-restart | 54fae40e-e71a-434a-b2ad-a800e4c2d584 |

Authorities: `P3A-W1-CONTRACT-REPAIR-1.md` §8 (sole record-schema authority),
`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md`, and
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` (frozen, Review 4 PASS), plus
`docs/runbooks/phase3a-cross-client-journey.md`.

## Independently verified (every root)

1. §8 schema conformance: `gui/actions.jsonl`, `tui/keystrokes.jsonl`,
   `timeline.jsonl`, `ipc/request-response-summary.jsonl`,
   `daemon/structured-log.jsonl`, processes pre/postflight (no extra
   plaintext fields), projection/artifacts/journal summaries and
   `manifest.json` use the frozen keysets; every JSONL is valid and
   non-empty.
2. Journey correlation: zero drift — every daemon, IPC and timeline record
   carries the root's journey ID; daemon sequences are monotonic including
   across restarts.
3. Dual-client coverage: every IPC summary has both `client_kind=gui` and
   `client_kind=tui` traffic, with ≥2 app-originated
   `loom-swift-<uuid>` launch reads per root (production app connected to
   the daemon in every scenario, including the two crash `-r2` roots —
   prior F1 closed).
4. GUI evidence: real PTY transcripts contain "Loom ·"; real 0600 PNG
   screenshots of the actual window exist per root and are referenced by
   `gui/actions.jsonl` `screenshot_relative_path` (prior F2 closed by the
   GUI Evidence Surface Amendment).
5. Postflight: processes/sockets/locks/leases/temps arrays all empty;
   `cleanup-proof.txt` confirms socket absent, isolation residue 0, journey
   processes 0; `loomd.sock`/`loomd.sock.lock` absent.
6. Permissions and hygiene: roots 0700, evidence files 0600 (including
   auxiliary `tui-plan.json`, console logs, `source/*.md` — prior P2-1
   closed); no `sk-*`/api-key/bearer/private-key material in any evidence.
7. Manifest integrity: `manifest_digest` self-consistent; `evidence_files`
   digests/sizes/modes match on disk; `product_result` and
   `operational_trace_result` are PASS; `network_allowed=false` and
   `provider_credentials_present=false`.
8. Binary identity: production_components match daemon
   `5bd5d86a…`, TUI `29e3775c…`, GUI `87c37812…` (final-source rebuilds
   byte/string-identical).
9. Independent SQLite audit: integrity ok, zero duplicate Event IDs /
   idempotency keys, zero stream gaps, zero FK violations; stream heads and
   event summaries match the DB.
10. Projection: `matches_journal=true` in every root; artifact digests all
    `available=true`/`match=true`.
11. result.md wording precise: happy 492/498 and cancel 8/11
    journey-correlated (platform foundation events with their own causation
    IDs documented), no unsupported clean-restart claim (prior F3 closed).
12. Runbook `--socket`/`--journey-id` wording accurate
    (`LocalIPCClient.defaultClient()` reads `--socket`;
    `LocalProductStore.initialJourneyID()` reads `--journey-id`) (prior F4
    closed).

## Certification chain (final generation)

This verdict completes the dual Result gate together with
`P3A-W1-PRODUCT-RESULT-REVIEW-FINAL-V6.md` (PASS, P0=P1=P2=0), the
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT-REVIEW-4.md` (PASS, P0=P1=P2=0), and
the `P3A-W1-WHOLE-CANDIDATE-REVIEW.md` (PASS, P0=P1=0, P2=2 documented
closures). The earlier v5-generation FAIL record
(`P3A-W1-OPERATIONAL-TRACE-REVIEW-FINAL-V5.md`) and the v3/v4-scoped
`P3A-W1/OPERATIONAL-TRACE-REVIEW.md` are superseded by this final-generation
certification.

VERDICT: `PASS`
