# P3A-W1 GUI Evidence Surface Bounded Amendment

Date: `2026-08-04`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

Authorization: Product Owner instruction `2026-08-04` — skip the
Computer-Use-driven real-window automation and verify the WorkItem by
alternative means so development can continue
(`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md`). This Amendment is the
precise definition of the GUI evidence surface of that same already-frozen
alternative method; it expands no authority, schema, credential, product
surface or external action.

WorkItem: the existing and only `P3A-W1`. This Amendment creates no P3A-W2,
no new authority, no new database, no second writer and no thin WorkItem.

## 1. Reason

The frozen `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` §6 states that
"the real window's visual state ... are exercised". In practice the
production native main window renders the app's default workbench view
(navigation rail + mission board); it does not render the Evolution Asset
journey state (candidate/revision/evaluation/binding lists are a TUI and
Swift-model surface, not a main-window view). Real `screencapture` captures
of the window at journey checkpoints are therefore authentic but visually
identical across journeys, so a strict reviewer cannot authenticate them as
per-journey captures of journey-specific visual state.

The independent Operational/Trace final review (FAIL, P1-3) confirmed that
the journal/IPC/TUI/projection evidence and the production Swift client
traffic are strong and consistent; the sole GUI-half objection is the
window screenshot evidence surface. The reviewer's own recommendation was to
freeze a bounded amendment that defines the GUI evidence surface precisely.

## 2. Supersession (bounded)

This Amendment supersedes only the visual-state claim in
`P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` §3(1)/§3(3)/§6 to the extent
it requires the real window's rendered content to reflect journey-specific
Evolution Asset state, replacing it with the precise GUI evidence surface in
§3 below. All other Amendment/Contract requirements remain frozen: one
daemon, one root/socket, unique journey UUID, Event Journal authority, CAS,
projection, materialization, cleanup, evidence modes (0700/0600), scenario
assertions, dual Result axes and atomic commit.

## 3. GUI evidence surface (precise definition)

For each scenario the GUI half is evidenced by:

1. The production native app bundle is launched with
   `--socket <root>/loomd.sock --journey-id <JOURNEY>` against the production
   daemon for every scenario. Its production client launch reads
   (`setup_snapshot` + `snapshot` with `loom-swift-<uuid>` request IDs) are
   recorded in `ipc/request-response-summary.jsonl` and
   `daemon/structured-log.jsonl` under the journey ID (verified: ≥2 such
   records per root).
2. Real window screenshots are captured at journey checkpoints with
   `screencapture` of the actual window and stored 0600 under
   `gui/screenshots/`. Checkpoint identity is the capture's wall-clock order
   within the journey timeline; the captures are of the real production
   window. The window renders the app's default workbench view; the SwiftUI
   main window does not surface the Evolution Asset list view, so captures
   are not required to differ visually per journey. Every driven action
   records its capture reference in `gui/actions.jsonl`
   (`screenshot_relative_path`, frozen §8 field), binding each screenshot to
   its checkpoint.
3. Journey-specific GUI-side behavior is proven by the production Swift
   client code path (`LocalIPCClient`/`LocalProductStore`) over the real
   socket, as recorded in the IPC/daemon logs, and by the cross-observation
   read-back (`--assets JOURNEY`), whose results are recorded in the
   IPC/daemon logs and the projection summary, with explicit read-back
   action rows in `gui/actions.jsonl` where the scenario drives one (for
   example `snapshot_cross_observe`, `explicit_refresh`).
4. No click/type automation of the SwiftUI view layer is claimed, per the
   original Amendment §6; that clause is retained and its "visual state"
   phrase is bounded to the real window's presence/rendering at checkpoints.

## 4. Not claimed

This Amendment does not claim that the main window's rendered pixels reflect
Evolution Asset journey state, and does not authorize any product/UI change,
dependency, migration, staging, push, merge, network or live action.

## 5. Independent Review acceptance

The Amendment passes only if a fresh read-only Reviewer proves: bounded
supersession; the GUI evidence surface is production-only and cannot bypass
authority; scenario assertions/evidence schema unchanged; no P3A-W2/
dependency/migration/staging/push/merge/network action authorized.

VERDICT: `FROZEN — PENDING REVIEW`
