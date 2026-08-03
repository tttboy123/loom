# Phase 2A Whole-Phase Exit Matrix

**Date**: 2026-08-03  
**Repository**: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`  
**Branch**: `codex/loom-platform-slice2`  
**Accepted implementation HEAD**: `7a27149b31db7ffeefefb86c449ba448c3250fca`  
**Status**: Product Owner sign-off accepted; Phase 2A closed

This matrix reconciles the frozen historical rows in `EXIT-CONTRACT.md`; it
does not rewrite them. Failed W1/W2/W3 live attempts remain failed evidence.
Only later accepted current-source and consumed isolated evidence may close a
product capability.

## Exit reconciliation

| ID | State | Current proof and bounded interpretation |
|---|---|---|
| PX-01 | DONE | W1 install/build fixtures prove user-level packaging; W3 root006 opens the signed native app and real private daemon socket without terminal configuration. Historical W1 `model_ids:null` live failure remains preserved and is not called a pass. |
| PX-02 | DONE | `LocalProductReadService`, `GlobalReadView`, strict IPC and current native/TUI views expose bounded daemon, Journal/projection, Runtime, Provider, Team, Run, Attention and recovery truth. root006 proves the current product renders authoritative succeeded state and one no-write daemon cycle. |
| PX-03 | DONE | Current Board, Teams, Runs/History, Compare, Evidence, Attention and Team Timeline are projection-backed and bounded. root006 durable native screenshots and raw/clean TUI transcripts prove these surfaces, including a second bounded Timeline page. |
| PX-04 | DONE | W2 accepted user-visible Mission/Team decision path and W3 strict timeline paging bind a selected human-named Team; tests reject nested identity drift, gaps, duplicates and cursor replay. root006 uses `P2AW3ControlledTeam`, not a typed ID. |
| PX-05 | DONE | Go stream/IPC and Swift store tests cover cursor reconnect, bounded pages, gap/duplicate/cancellation/generation fencing and preserved milestones. root006 reads the next page without write or error. Tentative output remains non-authoritative and is not journaled per token. |
| PX-06 | DONE | W2 accepted once-per-question builder, edit/save/confirm and Candidate-only boundaries. W3 tests and root006 show Team Builder while execution uses only an explicitly confirmed saved Team; no draft implicitly creates a Run. |
| PX-07 | DONE | Saved-Team binding and Dormant Capacity repair validate exact Agent/Runtime/model/permission/compatibility bindings, materialize only the active Main against capacity, keep SubAgents dormant and fail closed on conflicts. |
| PX-08 | DONE | Consumed Codex replacement `phase2a-w3-live-20260802-codex-002` passes the contract's native-auth selection role: native product displays `Codex Available`, returns a ready preflight with `native_auth`, performs no OAuth extraction and leaves SQLite byte-identical. |
| PX-09 | DONE | W2 broker/Keychain tests cover configure/replace/revoke and non-disclosure. Consumed MiniMax-004 performs exactly one product-sheet Test and appends exactly one fail-closed authoritative revision-5 terminal (`rejected/unavailable`) containing only an opaque credential reference. |
| PX-10 | DONE | W2 Pi catalog evidence and current probe/adapter tests preserve isolated Pi 0.82.1 metadata, model/capability disclosure and strict bridge compatibility. Pi-006 binds the exact local model/Runtime and isolated attempt root. |
| PX-11 | DONE | Pi-006 uses the ordinary product path for one preflight and one Start of the confirmed human-named Team. It closes the DAG with no typed internal identifiers or Provider environment variables entered by the user. |
| PX-12 | DONE | Pi-006 and root006 prove node/Attempt/Run/generation/Runtime/Grant/Evidence lineage, four authorized Evidence rows, rejection, bounded recovery and one canonical succeeded Team terminal. The UI marks terminal state from authority rather than tentative output. |
| PX-13 | DONE | Current application/authority tests cover cancel, approval fence, bounded recovery, stale generation/CAS rejection and capacity. Pi-006 proves one product Start, a Journal-recorded bounded recovery, four distinct lineages, one terminal, active capacity max 1 and no hidden Controller retry or duplicate side effect. |
| PX-14 | DONE | root006 Runs/History and Compare expose four terminal Runs, selected previous/current Runs, Runtime identity, Evidence counts and recovery history. Missing usage/cost/Skill values are truthfully omitted rather than invented. |
| PX-15 | DONE | Attention remains a projection over actionable accepted facts. root006 renders one Attention item; tests preserve approval/blocked/human-required/retry/verification categories without UI-local authority. |
| PX-16 | DONE | User-level product and native-app installer/build fixtures prove dry-run, install, update, rollback, signal/failure atomicity, private modes, reproducible bundle identity and launch smoke. root006 proves clean daemon/app/TUI shutdown, socket/lock removal and empty isolation without granting autonomy. |
| PX-17 | DONE | The three separately frozen and consumed product-path roles pass: Codex-002 native-auth availability plus zero-write ready preflight; MiniMax-004 one bounded brokered Test plus authoritative rejected/unavailable terminal; Pi-006 one controlled local execution plus accepted recovery/terminal. These are distinct claims—Codex/MiniMax are not misreported as execution adapters. |
| PX-18 | DONE | root006 completes the no-terminal native and TUI journey with durable screenshots/transcripts and byte-identical 103-Event state. Fresh controller verification and independent Whole-Phase Review pass with P0/P1/P2 all zero. On 2026-08-03 the Product Owner explicitly approved the Phase 2A whole-phase sign-off and authorized entry into Phase 2B contract governance. |

## Exact evidence anchors

- W1 deterministic checkpoint:
  `P2A-W1/p2a-w1-atomic-commit-verification.md`
- preserved W1 live failure:
  `P2A-W1/native-app-final-exit-result-review.md`
- accepted W2 live decision closure:
  `P2A-W2/mission-decision-live-attempt-004-result-review.md`
- Codex role:
  `P2A-W3/complete-live-codex-replacement-result.md`
- MiniMax role:
  `P2A-W3/native-launcher-minimax-live-result.md`
- MiniMax/Pi result review:
  `P2A-W3/native-launcher-replacement-live-result-review.md`
- accepted Pi execution/recovery:
  `P2A-W3/authoritative-acceptance-recovery-pi-live-result-review.md`
- no-terminal journey:
  `P2A-W3/final-walkthrough-root006-result-review.md`
- W3 final source/evidence lock:
  `P2A-W3/final-source-evidence-lock.json`

## Claim boundary

PX-01 through PX-18 are independently confirmed `DONE` and the Product Owner
sign-off is preserved in `PHASE2A-PRODUCT-OWNER-SIGNOFF.md`. Phase 2A is
closed. Contract governance for the single P2B-W1 may begin; this matrix does
not authorize P2B product writes, a live canary, production activation,
P2A-W4, P2B-W2, push or merge.
