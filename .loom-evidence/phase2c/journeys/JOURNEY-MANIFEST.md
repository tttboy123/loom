
 # Phase 2C Cross-Client Journey Manifest
 
 **Status**: frozen — part of Phase 2C Exit Contract  
 **Date**: 2026-08-06  
 **Parent**: `.loom-evidence/phase2c/contracts/PHASE-2C-EXIT-CONTRACT.md`
 
 ## Method
 
 Every journey runs on both the native macOS app and the Bubble Tea TUI against
 the same Go daemon and fixture. Verification uses the accepted alternative-
 verification method: production Swift client over the real daemon socket, real
 PTY TUI, real native window launched with `--socket --journey-id`,
 `screencapture` checkpoints, and restart/reconnect rebuild evidence.
 
 Computer-Use-driven window automation remains skipped unless explicitly
 reauthorized.
 
 ## Journeys
 
 | # | Journey | Owner | Key assertions |
 |---|---|---|---|
 | J1 | Fresh launch → no-folder chat | W1 | No Team/Mission facts created; composer visible; offline state truthful. |
 | J2 | Open Folder → first chat | W1 | Native folder picker used; path not typed; scoped preflight before file access. |
 | J3 | Offline → reconnect → continue | W1 | Distinct states; recovery action visible; view rebuilds from Journal on reconnect. |
 | J4 | Plain chat → explicit “use Agent” → Team Draft | W2 | Main Agent proposes a structured Draft; no fact created until user confirms. |
 | J5 | Confirm Team Draft → Mission starts → governance panel | W2 + W3 | Right panel shows Board; center shows milestones; Journal contains the facts. |
 | J6 | Mission running → inspect Evidence → approve decision | W3 | Right panel switches to Evidence/Decisions; approval creates a Journal fact. |
 | J7 | Runtime offline | W3 | Panel shows Runtime health; status distinct from Mission state; recovery actionable. |
 | J8 | Restart daemon → reconnect → view preserved | W1–W3 | Journal rebuilds the same views; no duplicate facts; no client-side authority. |
 | J9 | Keyboard/accessibility traverse every action | W1–W3 | VoiceOver/Computer Use identifies every button uniquely; focus visible; Escape works. |
 | J10 | Light/dark/compact screenshot matrix | W1 | Contrast passes; no ad-hoc system colors; tokens consistent across views. |
 
 ## Evidence Schema
 
 Each journey record includes:
 
 - `journey_id` (UUID)
 - `surface` (`macos` | `tui`)
 - `workitem` (`W1` | `W2` | `W3` | cross)
 - `ipc_log` (daemon requests/responses, redacted)
 - `native_screenshot` or `tui_transcript`
 - `journal_facts` (authoritative Event IDs)
 - `projection_version` (rebuild checkpoint)
 - `restart_rebuild_facts` (for J8)
 - `review_verdict` (P0/P1/P2)
 
 No prompt content, raw output, raw filesystem paths, credentials, Grants,
 hidden reasoning, or per-token deltas are recorded.
 
 ## Postflight
 
 After the last journey, verify:
 
 1. All 10 journeys pass on both surfaces.
 2. No duplicate, missing, or reordered Events compared to the fixture.
 3. No stale-generation view accepted after reconnect.
 4. Screenshot matrix shows consistent tokens across light/dark/compact.
 5. Accessibility dump shows unique actions for every interactive element.
 
