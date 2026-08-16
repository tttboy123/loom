# P2D-W2A Conversation Route Transition Confirmation

**Date**: 2026-08-12  
**Status**: `CURRENT / PACKAGED / NOT INSTALLED`  
**Goal**: unified Phase 2D  
**WorkItems**: existing P2D-W2A with W2D disclosure governance

## User-visible outcome

Changing Provider, Provider Account, Model, Harness, or credential revision in
an active ordinary Conversation no longer changes the route immediately. Loom
presents one native review sheet that shows the current and proposed execution
binding, states that a new immutable Segment will be created, and requires the
user to choose one context disclosure mode:

- Continue with context;
- Summary only;
- Start clean.

The review displays friendly Harness, exact non-secret Provider Account, Model,
and credential revision labels. It does not display a credential, Prompt,
Provider response, native session handle, ciphertext, nonce, or hidden
reasoning. The previous detached Context mode menu was removed so disclosure is
decided at the route transition where it applies.

## State and authority boundary

`requestConversationProfileSelection` returns a pending transition without
changing the selected Profile when the visible Conversation has continuity.
The transition freezes source and target Profile records, the current thread
anchor, and a Store generation. Confirmation succeeds only if all remain
current and the authoritative Profile directory still contains both exact
records.

Selection, anchor, directory, or revision drift fails closed. The sheet stays
open and tells the user to close it and select the route again. A blank
Conversation can select its first Profile directly because no prior Segment or
context disclosure exists. The existing async thread-load generation fence
continues to prevent an old loaded thread from overwriting the user's route.

Confirmation sets the selected disclosure mode and existing Segment route
state together. Provider-native session state and credentials are not reused
across the resulting Segment. This client presentation does not itself dispatch
a Provider request, approve fallback, or write Journal authority.

## Verification

- focused Store tests cover explicit confirmation, stale-selection rejection,
  blank-Conversation direct selection, unloaded persisted anchors, and Segment
  continuity;
- UI source contracts prove the Picker uses the review request path, all three
  context modes and disclosure receipt copy are present, direct selection and
  the detached Context mode menu are absent;
- the native sheet lays out at 560 x 520 in light/large and
  dark/accessibility-2 environments;
- complete `swift test --package-path apps/macos` passes 191 XCTest cases with
  one intentional visual-preview skip and eight Swift Testing contracts, with
  zero failures;
- `git diff --check` passes.

## Open live gates

This source is packaged as the unlaunched, uninstalled v0.5.2 build 50
Candidate at
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build50-candidate-2026-08-12/BUILD-MANIFEST.md`.
Release, arm64, deep-signature, owner-only/no-symlink, route-contract, and ZIP
byte-equivalence gates pass. It is not installed or interaction-reviewed in the
installed App.
Installed Codex-to-DeepSeek same-visible-Conversation transition, real
DeepSeek reply, disclosure receipt inspection, restart continuity, credential
Vault CV6, mixed-Team isolation, accounting, and approved fallback remain open.
Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
