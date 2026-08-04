# P3A-W1 Operational and Trace Behavior Review — FINAL v5 roots (FAIL)

Date: `2026-08-04`

Reviewer: independent read-only Reviewer (`p3a_optrace_final_reviewer`).
Scope: the eight final v5 journey roots under
`/private/tmp/loom-p3a-v5-roots/` (happy-r2 `4a3c3986…`,
cancel-reject-retain-r2 `e50e7ca2…`, stale-view `532849b1…`,
concurrent-single-winner `003e8570…`, crash-before-cas `10938833…`,
crash-after-cas-before-response `c36fb6e5…`,
projection-failure-rebuild-reconnect `31c76a1d…`,
slow-client-redelivery-clean-restart `54fae40e…`).

Reference contract: `P3A-W1-CONTRACT-REPAIR-1.md` §8,
`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` §3/§5/§6, the journey
runbook, and the source-lock manifest. All audits below were performed
read-only and independently of `verify-phase3a-cross-client-journey.sh`
(the candidate's own verifier), including an independent SQLite audit and
independent binary/source binding checks.

## Verdict

```text
P0 = 0
P1 = 3
P2 = 1
Operational and Trace Behavior: FAIL
Overall: FAIL
```

The eight roots' journal/IPC/TUI/projection evidence is strong and
consistent, but the GUI real-window half required by the frozen Alternative
Verification Amendment is not satisfied: in two journeys the production app
never connected to the daemon, and the screenshot evidence is byte-identical
across journeys and identical to the superseded v4 generation, so it cannot
be authenticated as per-journey checkpoint captures of journey-specific
state. The final happy root's `result.md` also carries stale event counts
that contradict its own journal. The gate therefore does not close.

## Verified PASS (all eight roots)

1. **§8 schema conformance** — `manifest.json` field set and order exactly
   matches the frozen 31-field schema; `manifest_digest` is
   self-consistent under the `jq -cS 'del(.manifest_digest)'` canonical
   form; `evidence_files` digests/sizes/modes match on disk for every
   listed file; `production_components` items are exactly
   `kind/path_digest/sha256/build_id`; every JSONL line has the exact frozen
   keyset.
2. **Binary identity and source binding** — on-disk `bin/loomd` =
   `5bd5d86a…`, `bin/loom` = `29e3775c…`, `app/Loom.app/.../LoomLocalApp` =
   `87c37812…` in every root, matching `manifest.json`
   `production_components` and `journey-context.json`. Independent rebuilds
   from the current workspace source reproduce the daemon and TUI digests
   byte-for-byte; the GUI rebuild is string-identical with the frozen binary
   (same symbols at same addresses; only the random LC_UUID differs).
3. **Journey correlation** — zero drift: every `ipc/`,
   `daemon/structured-log.jsonl`, and `timeline.jsonl` record carries the
   root journey ID; daemon `sequence` is globally monotonic including
   across restarts (0 reset marks); per-process `monotonic_offset_micros`
   resets only at restart boundaries (the single non-increase in the
   slow-client root is exactly the restart boundary, offsets
   `8328900 → 586537`).
4. **Dual-client coverage** — every root's IPC log contains both
   `client_kind=gui` (production Swift probe) and `client_kind=tui`
   traffic; the TUI is production client traffic through the real PTY.
5. **TUI evidence** — every `tui/transcript.txt` is real PTY content
   containing `Loom ·`; `tui/keystrokes.jsonl` schema-conformant.
6. **Postflight and cleanup** — `processes/postflight.json`
   processes/sockets/locks/leases/temps arrays all empty; preflight and
   postflight items carry no extra `root` field; `loomd.sock` and
   `loomd.sock.lock` absent; `processes/cleanup-proof.txt` present.
7. **Permissions and hygiene** — roots 0700, evidence files 0600; no
   `sk-*`/api-key/bearer/private-key material in result, actions,
   keystrokes, timeline, IPC, or daemon logs; `network_allowed=false`,
   `provider_credentials_present=false`.
8. **Independent SQLite audit** — `PRAGMA integrity_check` = ok;
   foreign-key violations 0; duplicate Event IDs 0; duplicate idempotency
   keys 0; stream gaps 0; `journal/stream-heads.json` heads match the DB;
   `journal/event-summary.json` count matches the DB; `journey_event_count`
   ≤ event_count and consistent with the harness correlation.
9. **Projection** — `projection/summary.json` `matches_journal=true` in
   every root.
10. **Artifacts** — `artifacts/digest-verification.json` entries all
    `available=true`/`match=true`; on-disk artifacts under
    `state/evidence/artifacts/sha256/` hash-match their expected digests.
11. **Scenario variant truths** — S3: zero decision/evaluation events;
    S4: exactly one `EvolutionAssetCandidateRejected`; S5: zero activation
    event after before-CAS crash (exit 92 recorded); S6: exactly one
    committed activation after after-CAS crash (exit 93 recorded); S7:
    `phase=post_commit_refresh, outcome=fail, error_code=state_unavailable`
    recorded, old view preserved pre-restart, `matches_journal=true`
    post-restart; S8: client-side timeout recorded with no hidden retry and
    exactly one activation.

## Blocking findings

### F1 (P1) — Production app never connected in the two crash journeys

In `crash-before-cas` and `crash-after-cas-before-response`, the
`ipc/request-response-summary.jsonl`, `daemon/structured-log.jsonl`, and
`timeline.jsonl` contain **zero** app-originated requests. Every record is
either `loom-swift-contract-probe` (the probe) or `loom-client-*` (TUI).
The production app's launch reads (`setup_snapshot` + `snapshot` from
`loom-swift-<uuid>` request IDs) appear at rows 1-2 in the happy,
cancel-reject-retain, stale-view, and concurrent roots, but are absent
throughout the two crash roots. The daemon records every request
(`wrap()` in `cmd/loomd/product_daemon.go` defaults an empty request
journey ID to the manifest journey ID), so a connected app would appear.

This violates Amendment §3(1): the production native app must be launched
with the journey against the daemon socket for every scenario. In the two
crash scenarios the real-window half was not exercised at all.

### F2 (P1) — Screenshots are not authenticated per-journey checkpoint captures

`gui/screenshots/01-main-window.png` is byte-identical
(SHA-256 `c1c9b6a9…`, 162,322 bytes) across six of the eight final roots
(happy-r2, cancel-reject-retain-r2, concurrent-single-winner,
projection-failure-rebuild-reconnect, slow-client-redelivery-clean-restart,
stale-view-digest-generation) and across the two superseded v5 happy roots,
and is byte-identical to the v4-generation cancel-reject-retain root's
screenshot. `crash-before-cas` and `crash-after-cas-before-response` share a
second identical image (`2270a113…`). Where a second checkpoint exists
(`02-after-actions.png` in happy and concurrent), it is byte-identical to
`01-main-window.png`. All images decode to the app's default blank shell
(≈83% rgb(240,240,240), ≈15% gray sidebar; 2200×1440; macOS
"Screenshot"-style EXIF/XMP with no capture timestamp), and
`gui/app-console.log` is empty in every root.

The evidence therefore cannot establish that a live per-journey
`screencapture` of the production window at action checkpoints occurred, or
that the window's journey-specific authoritative state was ever captured.
This falls short of Amendment §3(3) ("Real window screenshots are captured
at checkpoints with screencapture of the actual window") and §6's claim
that "the real window's visual state … [is] exercised". Note that no
`gui/actions.jsonl` action references a screenshot
(`screenshot_relative_path` is empty in every root), so the screenshots
carry no checkpoint binding.

### F3 (P1) — Final happy root result.md contains stale event counts

`happy-create-evaluate-activate-bind-execute-clean-r2/result.md` states
"141/147 Journal events are journey-correlated (the remaining 6 are
platform foundation events)". Its own `journal/sqlite-summary.json` records
`event_count=498`, `journey_event_count=492` (verified against the DB:
498 events). "141/147" is the superseded v4-era count. The same stale text
appears in the superseded first v5 happy root (which has only 12 events).
The final root's narrative is inconsistent with its own frozen data; the
earlier review chain's P2 closure ("result.md files now state the precise
counts") is not satisfied by the final root.

## Non-blocking finding

### F4 (P2) — Runbook documents an app `--journey-id` argument the app does not parse

`docs/runbooks/phase3a-cross-client-journey.md` says the native executable
is started with `--journey-id JOURNEY` "and falls back to a fresh local
UUID when the argument is absent". `LoomLocalApp.swift` and
`LocalIPCClient.defaultClient()` parse only `--socket`; no `--journey-id`
handling exists. The runbook is not aligned with the shipped app source.

## Evidence locations

- `ipc/request-response-summary.jsonl` in crash-before-cas /
  crash-after-cas roots: no `loom-swift-<uuid>` rows (F1).
- `gui/screenshots/*.png` digests across all roots and the v4
  cancel-reject-retain root (F2); independent PNG decode of
  `01-main-window.png` / `02-after-actions.png` (F2).
- `happy-create-evaluate-activate-bind-execute-clean-r2/result.md` vs
  `journal/sqlite-summary.json` and the SQLite DB (F3).
- `apps/macos/Sources/LoomLocalApp/LoomLocalApp.swift`,
  `LocalIPCClient.defaultClient()` (F4).
- Independent audit: `/tmp/p3a-final-optrace-audit.py` (read-only;
  all eight roots PASS every schema/integrity/correlation check, which is
  why F1-F3 are assertions about the app/screenshot halves that the script
  cannot infer from the JSONL alone).

## Recommended next step

Do not stage or commit P3A-W1 as accepted. Re-freeze the affected journeys
with the production app genuinely launched and connected per scenario and
authentic per-journey screenshot captures, or prepare a bounded, frozen,
independently reviewed amendment that explicitly removes the real-window
visual-state claim and defines the GUI evidence surface precisely (probe +
TUI over the daemon), then re-run the eight journeys and the whole gate.

VERDICT: `FAIL` (P0=0, P1=3, P2=1)
