# Build 188 Blocked Mission Intervention Evidence

Status: `CURRENT / INSTALLED / POST-ACCEPTANCE FOLLOW-UP`

This usability correction remains inside the accepted Phase 2D Goal. It exposes
existing new-Attempt authority from the Mission Room and does not introduce a
new product Goal or permit an Agent to retry itself.

## User outcome

- Blocked, Failed and Cancelled Mission rooms show a fixed `Continue this
  Mission` composer beneath the Mission history.
- The user enters what the Team should change or try next; empty and oversized
  drafts cannot proceed.
- `Review & continue` opens a prefilled `Continue Mission` review with the exact
  Team, current Mission title and normalized guidance as the new objective.
- The user must confirm Mission context, review the exact preflight bindings and
  explicitly choose `Start new Attempt` before any Provider work begins.
- The prior Attempt, output, failure, accounting and Incident remain unchanged.
- Closing review preserves the Mission-scoped draft. A successful continuation
  clears it.
- If the exact Team is no longer executable, continuation is disabled and the
  UI offers Runtime & Providers recovery.

## Authority boundary

The composer does not send input to a terminal Agent and does not mutate the
failed Attempt. It prepares the existing `new_attempt=true` execution path. A
new correlation ID, preflight digest, frozen bindings and explicit user start
remain required. Runtime Queue/Steer/Inject continues to apply only to active,
nonterminal Agent bindings.

## Verification

- `LocalProductExperienceViewTests` cover eligible terminal states, active-state
  rejection, whitespace normalization, empty input, the 4,096-byte boundary,
  exact Team/title/objective preparation and continuation sheet naming.
- `swift test --package-path apps/macos`: 379 XCTest cases, two conditional
  skips and zero failures; 20 Swift Testing contract cases and zero failures.
- `scripts/test-build-loom-local-app.sh`: PASS for two reproducible production
  builds.
- Installed bundle: Loom `0.5.3` Build 188 at
  `/Users/lune/Applications/Loom.app`.
- `codesign --verify --deep --strict`: PASS.
- Installed App and bundled daemon launch as one managed product with canonical
  state, isolation, runtime, socket and managed-parent arguments.
- Installed accessibility acceptance on `MiniMax live Agent conversation`
  verified the disabled empty state, enabled bounded draft, prefilled review,
  context-confirmation gate, close-without-dispatch path and retained draft.
  The temporary acceptance draft was then cleared.
