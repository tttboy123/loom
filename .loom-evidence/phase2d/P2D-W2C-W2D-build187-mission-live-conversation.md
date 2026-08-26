# Build 187 Mission Agent Conversation Evidence

Status: `CURRENT / INSTALLED / POST-ACCEPTANCE FOLLOW-UP`

This follow-up remains inside the accepted Phase 2D Goal. It improves Mission
execution visibility without reopening Phase 2D or changing execution
authority.

## User outcome

- A visible nonterminal Mission is refreshed once per second.
- Mission Room renders `Agent conversation` before the Team reaches a terminal
  state.
- Each Agent block shows Harness, Provider Account, model and current status.
- Consecutive `node_output_delta` records are grouped by logical node and
  Attempt, and only `payload.textDelta` is displayed.
- Output is capped at 24,000 characters per Agent and collapsed to the latest
  2,400 characters by default.
- A terminal refresh retains output already observed by the App.
- A transient cursor conflict during live pagination keeps the last complete
  activity snapshot visible and allows the next bounded poll to restart from
  the authoritative head.

## Authority and privacy boundary

Visible Agent output is tentative until accepted terminal Evidence exists. It
does not become Journal authority and cannot authorize execution. API keys,
Authorization headers, prompts, Provider response envelopes and hidden
reasoning are not read or rendered by this feature. An App/daemon restart cannot
reconstruct tentative plaintext that was never committed as Evidence; the UI
states that historical activity is unavailable instead of fabricating it.

## Installed live observation

Build 186 supplied the live Provider observation used to validate this Build
187 behavior. A new Team named `MiniMax Live Activity Build186` used two Loom
Native Agents, each bound to `minimax.primary`, credential revision 23 and
`MiniMax-M3`; no Luna, DeepSeek or fallback route was selected.

The Mission `MiniMax live Agent conversation` immediately showed `Live`, 0/2
progress and both Agent statuses. While the coordinator remained Running, the
subagent's complete three-step plan and concrete improvement appeared in the
Agent conversation section, and progress advanced to 1/2. The coordinator later
ended with `Provider Http` after retries. The final Mission was therefore
Blocked, while the successful subagent, its exact binding, usage/accounting and
Incident diagnostics remained visible. This proves live process visibility and
failure isolation; it does not claim that the final Provider request succeeded.

Build 187 adds the cursor-conflict preservation regression on top of that live
path and is the installed canonical bundle.

## Verification

- `swift test --package-path apps/macos`: 377 XCTest cases, two conditional
  skips, zero failures; 20 Swift Testing contract cases, zero failures.
- Focused Store regressions cover transient timeline failure, cursor-conflict
  preservation, terminal tentative-output retention and unique Mission start
  correlation.
- View tests cover Agent grouping, metadata, truncation, expand/collapse and the
  explicit historical-output boundary.
- `scripts/test-build-loom-local-app.sh`: PASS for two reproducible production
  builds.
- Installed bundle: Loom `0.5.3` Build 187 at
  `/Users/lune/Applications/Loom.app`.
- `codesign --verify --deep --strict`: PASS.
- Installed App starts the bundled managed daemon with canonical state,
  isolation, runtime, socket and managed-parent arguments.
- Installed accessibility inspection exposes Mission Room, Agent conversation,
  per-Agent bindings, progress, Evidence and Incident diagnostics.
