# Phase 5 Build 243 RoundTable result hierarchy

Status: `INSTALLED PARTIAL / VISUAL CLICK GATE OPEN / PHASE 5 IN PROGRESS`

## Boundary

This checkpoint applies Codex-inspired progressive disclosure to the
Conversation-side RoundTable inspector. Agent identity, frozen route and status
must remain scannable before long result detail, while the complete result stays
available through an explicit action.

## Installed identity

- App version: `0.5.3`
- App build: `243`
- App bundle: `/Users/lune/Applications/Loom.app`
- Source branch: `codex/phase4-roundtable-governed-deliberation`
- App executable digest: `0e2cdd7b2ae7e260d777756366ff725aaedc4db25858cf97563842113bd6180f`
- Bundled daemon digest: `87bfc6e7f96a36b37bda4237880bcd08772b83d5526d62c1844d179d9fae947a`
- Strict bundle signature: passed
- Packaged and installed executable digests: exact match

The daemon digest is identical to Build 242; this checkpoint changes only the
macOS result presentation and its tests.

## Change

- Each long Agent result defaults to its latest 900-character preview.
- `Show full result` reveals the complete selectable result; `Show less` returns
  to the compact comparison state.
- Expansion is scoped to the exact Attempt ID, so one Agent's detail does not
  expand every peer.
- Both actions include the Agent display name in their VoiceOver label and use a
  stable Attempt-scoped accessibility identifier.
- Markdown remains inline-only and active links remain removed from Agent output.

## Verification

- Complete macOS package: 418 XCTest cases passed, two conditional skips.
- Swift Testing contracts: 20 passed.
- Focused RED/GREEN coverage verifies compact tail preview, complete expansion,
  markdown rendering and link removal.
- Phase 5 shell rendering passed at 1,280 x 760 and 900 x 760.
- Strict installed bundle verification and package/install hash equality passed.
- The installed managed daemon completed Agent Runtime startup successfully in
  5,551 ms for this launch; this is an observational sample, not a new startup
  distribution claim.
- `git diff --check` passed.

## Open installed visual gate

The App and daemon launched, but macOS entered the lock screen before the final
installed screenshot and click loop. The first capture was black and a second
capture showed the lock screen, proving the missing visual evidence is a locked
interactive session rather than a claimed App rendering result. No password was
requested, read or bypassed.

After the user unlocks the session, acceptance still needs to confirm:

1. both Agent headers and statuses are visible before expanding a long result;
2. `Show full result` reveals the complete result without shifting unrelated
   controls incoherently;
3. `Show less` returns to the compact state;
4. keyboard focus and VoiceOver labels reach both actions.

Build 243 is the current installed Phase 5 checkpoint, not Phase 5 completion.
