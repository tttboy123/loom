
 # Phase 2C Exit Contract — Chat-First Client Experience
 
**Status**: frozen with reviewed Repair 2 amendment; Phase not accepted
**Date**: 2026-08-06
**Baseline**: accepted v0.4.1 whole-slice (`7e24ec29`), branch `codex/loom-platform-slice2`
**Parent decision**: ADR-0015
**Audit input**: `.loom-evidence/phase2c/CLIENT-EXPERIENCE-AUDIT.md`
**Plan input**: `.loom-drafts/phase2c-chat-first-experience-plan.md`
 **Repair input**: `PHASE-2C-REPAIR-AMENDMENT.md`
 
 ## 1. Phase Scope
 
 Deliver a chat-first three-pane client experience for the native macOS app and
 the Bubble Tea TUI. The center conversation is the default entry; the Mission
 Board and related governance surfaces become a collapsible right panel. The
 left rail provides Chat, Work, Teams, Attention, Library, Runtimes, recent
 tasks, and an explicit New Mission action. Skills are governed assets inside
 Library rather than a duplicate top-level destination.
 
 No Event Journal, one-writer, Projection, Scheduler, policy, Grant, Evidence,
 or daemon authority model is changed. The narrowly scoped Queue admission
 prerequisite in the Repair Amendment enforces an existing invariant and does
 not add a fourth WorkItem or change Scheduler semantics.

The historical P2C-W1 and P2C-W2 commits remain recorded, but their affected
acceptance journeys are reopened by `PHASE-2C-REPAIR-AMENDMENT.md`. No earlier
green result is evidence for behavior changed by a repair round.
 
 ## 2. WorkItems
 
 Exactly three vertical WorkItems, each with its own contract, RED, verification
 matrix, independent reviews, and atomic local commit:
 
 | ID | Title | File |
 |---|---|---|
 | P2C-W1 | Workspace Shell & Entry | `.loom-evidence/phase2c/contracts/P2C-W1-CONTRACT.md` |
 | P2C-W2 | Chat-First Conversation | `.loom-evidence/phase2c/contracts/P2C-W2-CONTRACT.md` |
 | P2C-W3 | Governance Side Panel | `.loom-evidence/phase2c/contracts/P2C-W3-CONTRACT.md` |
 
 No wrapper-only shell, adapter, token, or button WorkItems exist.
 
 ## 3. Mandatory Journeys
 
 Every journey must run on both the native macOS app and the Bubble Tea TUI
 against the same Go daemon and fixture, using the accepted alternative-
 verification method (production Swift client over real socket, real PTY TUI,
 real native window, `screencapture` checkpoints, restart/reconnect rebuild).
 Computer-Use-driven window automation remains skipped.
 
 | # | Journey | Owner |
 |---|---|---|
 | J1 | Fresh launch → no-folder chat | W1 |
 | J2 | Open Folder → first chat | W1 |
 | J3 | Offline → reconnect → continue | W1 |
 | J4 | Plain chat → explicit “use Agent” → Team Draft | W2 |
 | J5 | Confirm Team Draft → explicit New Mission → governance panel | W2 + W3 |
 | J6 | Mission running → inspect Evidence → approve decision | W3 |
 | J7 | Runtime offline | W3 |
 | J8 | Restart daemon → reconnect → view preserved | W1–W3 |
 | J9 | Keyboard/accessibility traverse every action | W1–W3 |
 | J10 | Light/dark/compact screenshot matrix | W1 |
 
 ## 4. Acceptance Gates
 
 - Each WorkItem: deterministic matrix (Go full/race/vet/tidy/gofmt; Swift
   full/TSAN/Release), cross-client journey evidence, independent
   implementation review PASS, dual-Result review PASS, whole-WorkItem review
   PASS.
 - Phase: all three WorkItems accepted, all ten journeys pass, whole-Phase
   review PASS, Product Owner sign-off.
 - Repair amendment: every P0/P1 row is closed with its listed proof; no inert
   or placeholder-only affordance remains in an accepted journey.
 - Phase 3A source lock and entry review are refreshed only after Phase 2C is
   accepted.
 
 ## 5. Exclusions
 
 - Event Journal, one-writer, Projection, Scheduler, policy, Grant, Evidence,
   or daemon authority changes.
 - New credential capture, OAuth parsing, or Provider negotiation in the client.
 - Public network binding, remote multi-user hosting, or cloud sync.
 - WebSocket/notification state authority.
 - Automatic Team/Mission/Run creation from ordinary chat.
 - Hiding policy, permission, budget, Evidence, or approval gates for visual
   simplicity.
 - Full Web UI, cross-platform Tauri, or browser-based desktop shell.
 - Cross-platform delivery beyond macOS native + Bubble Tea TUI.
 - Push, merge, publish, credential, network, paid, or user-config action.
 
 ## 6. Source-Lock Exclusions
 
 The following pre-existing uncommitted changes are excluded from the P2C
 Candidates unless a WorkItem contract explicitly owns them:
 
 - `internal/execution/` and `internal/production/` changes from prior
   scheduling work.
 - `internal/tui/` non-style changes from prior WorkItems.
 - `apps/macos/Sources/LoomLocalAppCore/` and
   `apps/macos/Sources/LoomLocalAppUI/` existing modifications, except where
   P2C-W1/W2/W3 contracts explicitly claim them.
 - `.loom-evidence/phase1-final-live-gate/` amendments.
 - `go.mod`, `go.sum`, `AGENTS.md`, `PROGRESS.md`, `README.md` drift.
 
 Any overlap must be reconciled in the WorkItem contract and reviewed before
 staging.
 
