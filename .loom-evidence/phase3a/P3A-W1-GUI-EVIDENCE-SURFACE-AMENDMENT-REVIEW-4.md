# P3A-W1 GUI Evidence Surface Amendment — Independent Review 4 (PASS)

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (`p3a_gui_amendment_review_4`).
Scope: `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` reviewed bytes SHA-256
`4461f1d72f9b3c77f8aeac0b4b2a720a0a772299c583682c6a3690d4dc240e09` (the
post-Review-3 revision), against the frozen
`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` (bytes
`e7710c140345f3da5a18a362547fa2f83da9388db3d8dfc4adc70c697c3b4965`, equal to the
frozen reviewed digest), the frozen `P3A-W1-CONTRACT-REPAIR-1.md` §8 evidence
schema, and the eight final v5 journey roots under
`/private/tmp/loom-p3a-v5-roots/` (with the `-r2` crash replacements).
All audits were performed read-only and independently of the candidate's own
scripts.

## Verdict

```text
P0 = 0
P1 = 0
P2 = 0
Product/Authority: PASS
Operational/Trace Governance: PASS
Overall Amendment: PASS
```

## Delta vs Review 3

Review 3 reviewed bytes `12265126…` and returned PASS with one non-blocking P2:
tighten §3(3) so the cross-observation read-back is recorded "in the IPC/daemon
logs and the projection summary" or with per-root action rows where applicable.
The reviewed revision `4461f1d7…` implements exactly that tightening: §3(3) now
reads "whose results are recorded in the IPC/daemon logs and the projection
summary, with explicit read-back action rows in `gui/actions.jsonl` where the
scenario drives one (for example `snapshot_cross_observe`,
`explicit_refresh`)". No other semantic change was found; the supersession
clause, evidence-surface definition, schema references and non-claims are
unchanged from the Review-3-reviewed text.

## §5 acceptance checks (each independently verified)

1. **Bounded supersession.** The Amendment supersedes only the visual-state
   claim in `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` §3(1)/§3(3)/§6 to
   the extent it requires the real window's rendered content to reflect
   journey-specific Evolution Asset state, replacing it with the precise GUI
   evidence surface in its own §3. §3(4) explicitly retains the original
   no-click/type clause and bounds its "visual state" phrase to the real
   window's presence/rendering at checkpoints. Every other requirement is
   explicitly preserved (§2: one daemon, one root/socket, unique journey UUID,
   Event Journal authority, CAS, projection, materialization, cleanup,
   0700/0600 evidence modes, scenario assertions, dual Result axes, atomic
   commit). The Alternative Verification Amendment bytes on disk equal the
   frozen reviewed digest `e7710c14…`, so the supersession target is the
   reviewed revision.

2. **GUI evidence surface is production-only and cannot bypass authority.**
   - §3(1): the production native app bundle is launched with
     `--socket <root>/loomd.sock --journey-id <JOURNEY>` against the
     production daemon per scenario. `LocalIPCClient.defaultClient()` parses
     `--socket`; `LocalProductStore.initialJourneyID()` parses `--journey-id`
     with the same UUID v4 validation used by the wire contract. In all eight
     final roots `ipc/request-response-summary.jsonl` contains exactly two
     app-originated launch reads (`setup_snapshot` + `snapshot` with
     `loom-swift-<uuid>` request IDs, non-probe), and
     `daemon/structured-log.jsonl` contains the corresponding four
     request/response rows, all journey-correlated (single distinct journey ID
     per root) — satisfying the Amendment's "≥2 such records per root" and
     closing the prior F1 (crash roots not connected).
   - §3(3): actions are driven through the production Swift client
     (`LocalIPCClient`/`LocalProductStore` via `LoomLocalAppContractProbe`).
     `LoomLocalAppContractProbe/main.swift` constructs `LocalIPCClient` over
     the socket and calls `evolutionAssetSnapshot`/`evolutionAssetCommand`; no
     SQLite/Projection/service direct call, mock, Preview or ViewModel
     injection exists in the probe or the driven path. Probe snapshot rows are
     present in every root (1–5 per root), and where the scenario drives an
     explicit read-back action row it exists in `gui/actions.jsonl`
     (`snapshot_cross_observe` in cancel-reject-retain, `explicit_refresh` in
     slow-client); other roots carry their read-back in the IPC/daemon logs
     and projection summary, consistent with the "where the scenario drives
     one" qualifier.
   - §3(4): no click/type automation of the SwiftUI view layer is claimed.

3. **Scenario assertions and evidence schema unchanged.** The Amendment
   preserves the eight scenario IDs, per-scenario assertions, manifest/result/
   timeline/IPC/projection/artifact/process evidence bundle and the frozen §8
   fields. `gui/actions.jsonl` in all eight roots uses exactly the frozen §8
   field set (`sequence, monotonic_offset_micros, action, control_id,
   input_digest, expected_visible_state, observed_visible_state,
   screenshot_relative_path`); every action row carries a non-empty
   `screenshot_relative_path`; screenshots are real PNG captures
   (signature-verified, 2200×1440) stored 0600 under `gui/screenshots/`. The
   Amendment adds no schema field and changes no authority or wire protocol.

4. **No P3A-W2 / dependency / migration / staging / push / merge / network /
   live action authorized.** §4 explicitly denies all of these, and the
   Amendment's operative content is limited to the precise definition of the
   GUI evidence surface. No P3A-W2 is created; no second writer, queue or
   authority is introduced.

## Conclusion

The reviewed revision `4461f1d7…` is bounded, production-only,
schema-preserving and authorizes no new action. It addresses the Review-3 P2
without expanding scope, and every §3 evidence-surface claim was independently
verified against the eight final roots. It closes the prior GUI-half objection
(F2: screenshots cannot be authenticated as journey-specific visual state) by
precisely defining what the GUI evidence surface is and is not, consistent with
the Product Owner's instruction to verify by alternative means.

VERDICT: `PASS`
