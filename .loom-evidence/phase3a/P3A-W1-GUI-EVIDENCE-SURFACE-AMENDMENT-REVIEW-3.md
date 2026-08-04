# P3A-W1 GUI Evidence Surface Amendment — Independent Review 3 (PASS)

Date: `2026-08-04`

Reviewer: fresh independent read-only Reviewer (`p3a_gui_amendment_review_3`).
Scope: `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md`, reviewed bytes SHA-256
`1226512686417299dd4c2d5e199f076678909e879981115d6fd5ac102a06ed80`, against the
frozen `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` (reviewed bytes
`e7710c140345f3da5a18a362547fa2f83da9388db3d8dfc4adc70c697c3b4965`), the frozen
`P3A-W1-CONTRACT-REPAIR-1.md` §8 evidence schema and the eight final v5 journey
roots. All audits were performed read-only and independently of the candidate's
own scripts.

## Verdict

```text
P0 = 0
P1 = 0
P2 = 1 (non-blocking, documentation precision)
Product/Authority: PASS
Operational/Trace Governance: PASS
Overall Amendment: PASS
```

## §5 acceptance checks (each independently verified)

1. **Bounded supersession.** The Amendment supersedes only the visual-state
   claim in `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` §3(1)/§3(3)/§6 to
   the extent it requires the real window's rendered content to reflect
   journey-specific Evolution Asset state, replacing it with the precise GUI
   evidence surface in its own §3. Every other requirement is explicitly
   preserved (one daemon, one root/socket, unique journey UUID, Event Journal
   authority, CAS, projection, materialization, cleanup, 0700/0600 evidence
   modes, scenario assertions, dual Result axes, atomic commit). The
   Alternative Verification Amendment bytes on disk equal the frozen reviewed
   digest `e7710c14…` (Review 4 of that chain), so the supersession target is
   the reviewed revision.

2. **GUI evidence surface is production-only and cannot bypass authority.**
   - §3(1): the production app is launched against the production daemon per
     scenario. The production `LocalIPCClient.defaultClient()` parses
     `--socket` and `LocalProductStore.initialJourneyID()` parses
     `--journey-id`. In all eight final roots the IPC summary contains exactly
     two app-originated launch reads (`setup_snapshot` + `snapshot` with
     `loom-swift-<uuid>` request IDs) and the daemon structured log contains
     the corresponding four request/response rows, journey-correlated —
     satisfying the Amendment's "≥2 such records per root" and closing the
     prior F1 (crash roots not connected).
   - §3(3): actions are driven through the production Swift client
     (`LocalIPCClient`/`LocalProductStore` via `LoomLocalAppContractProbe`).
     `LoomLocalAppContractProbe/main.swift` constructs `LocalIPCClient` over
     the socket and calls `evolutionAssetSnapshot`/`evolutionAssetCommand`;
     no SQLite/Projection/service direct call, mock, Preview or ViewModel
     injection exists in the probe or the driven path.
   - §3(4): no click/type automation of the SwiftUI view layer is claimed.

3. **Scenario assertions and evidence schema unchanged.** The Amendment
   preserves the eight scenario IDs, per-scenario assertions, manifest/result/
   timeline/IPC/projection/artifact/process evidence bundle and the frozen §8
   fields. `gui/actions.jsonl` in all eight roots uses exactly the frozen §8
   fields (`sequence, monotonic_offset_micros, action, control_id,
   input_digest, expected_visible_state, observed_visible_state,
   screenshot_relative_path`); every action row carries
   `screenshot_relative_path`; screenshots are real PNG captures stored 0600
   under `gui/screenshots/`. The Amendment adds no schema field and changes no
   authority or wire protocol.

4. **No P3A-W2 / dependency / migration / staging / push / merge / network /
   live action authorized.** §4 explicitly denies all of these, and the
   Amendment's operative content is limited to the definition of the GUI
   evidence surface. No P3A-W2 is created; no second writer, queue or
   authority is introduced.

## Non-blocking P2 (documentation precision)

§3(3) says the cross-observation read-back (`--assets JOURNEY`) is "recorded
in `gui/actions.jsonl` and the projection summary". In the happy root the
read-back is evidenced by `loom-swift-contract-probe`
`evolution_asset_snapshot` rows in `ipc/request-response-summary.jsonl` plus
`projection/summary.json` (and result.md) rather than by an explicit
`gui/actions.jsonl` row; other roots (e.g. cancel `snapshot_cross_observe`,
slow-client `explicit_refresh`) carry explicit read-back action rows.
Recommend tightening the wording to "recorded in the IPC/daemon logs and the
projection summary" (or noting per-root action rows where applicable). This
does not affect the amendment's definitional correctness or the underlying
evidence, which substantively satisfies the surface as defined.

## Conclusion

The Amendment is bounded, production-only, schema-preserving and authorizes no
new action. It closes the prior GUI-half objection (F2: screenshots cannot be
authenticated as journey-specific visual state) by precisely defining what the
GUI evidence surface is and is not, consistent with the Product Owner's
instruction to verify by alternative means.

VERDICT: `PASS`
