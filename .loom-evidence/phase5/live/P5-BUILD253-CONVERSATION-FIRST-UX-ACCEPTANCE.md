# Phase 5 Build 253 conversation-first UX acceptance

Status: `INSTALLED ACCEPTED / PHASE 5 COMPLETE`

Date: 2026-08-28

## User outcome

Loom opens as a usable pairing workspace with Conversation in the center.
Mission, Team and RoundTable remain governed capabilities around that stable
conversation instead of replacing it with setup or board-first modes. Running,
blocked and failed work exposes progress, Agent output, recovery actions and
privacy-safe Incident details where the user is already working.

Build 253 closes the remaining Phase 5 installed gates: complete keyboard
traversal, installed Conversation/Mission/RoundTable failure recovery and the
rapid App-restart service-adoption race.

## Package identity

- Version: Loom `0.5.3` Build 253.
- Installed path: `/Users/lune/Applications/Loom.app`.
- App executable SHA-256:
  `6ee077ce30177f813188c63bbb1f741bb36ba807b3c4818a0f7465fcf0861473`.
- Bundled daemon SHA-256:
  `87bfc6e7f96a36b37bda4237880bcd08772b83d5526d62c1844d179d9fae947a`.
- Staged and installed hashes match.
- Strict deep signature verification passes for both bundles.
- The installed daemon runs with the installed App PID as its managed parent,
  canonical private state/isolation/UDS arguments and a `0600` Socket.

## Installed setup projection

The concrete signed Swift probe reached the installed daemon over the private
UDS and decoded:

- 24 Provider descriptors;
- seven Runtime instances;
- seven executable Conversation Routes;
- five privacy-safe credential import candidates.

Runtime inventory includes Codex, Claude Code, OpenCode, Pi and Loom Native.
OpenCode + DeepSeek and OpenCode + MiniMax are executable Routes whose Provider
Account remains `deepseek.primary` or `minimax.primary`; they are not duplicate
Provider rows.

## Interaction acceptance

The installed desktop and compact checkpoints verify that:

1. Conversation remains the stable center and the composer regains focus after
   launch, restart, New Task and closing governance context.
2. Mission and RoundTable open beside the same Conversation. Their full setup
   workbenches remain secondary actions.
3. Mission activity shows each Agent's identity, frozen Harness, Provider
   Account, model, status and bounded output. Blocked work accepts guidance for
   a new reviewed Attempt.
4. RoundTable shows every participating Agent result, supports governed seat
   intervention and publishes a conclusion only after explicit acceptance.
5. Long Agent results retain identity and status before detail, preserve whole
   rendered words and expand/collapse through accessible controls.
6. Restart restores the selected Conversation and Mission-linked RoundTable
   without silently resuming Provider work.

The final installed accessibility snapshot is preserved as
`P5-BUILD253-CONVERSATION-FIRST-FINAL.jpeg`.

## Keyboard and accessibility

With macOS Full Keyboard Access temporarily enabled, focus traversed the real
installed path from composer through Add, Route, Model, governance menu, pin,
close, both RoundTable full-result controls, full discussion, New Task,
Conversation and Missions, then returned to the composer. The traversal found
34 unique focus nodes with no duplicate trap. Space expanded and collapsed
Agent results without losing focus. The original system keyboard setting was
restored after acceptance.

The earlier desktop, 900-pixel compact, largest-accessibility-text and SwiftUI
render matrices remain preserved as predecessor evidence.

## Recoverable failure acceptance

Controlled installed fault injection changed only the private UDS mode and
restored it to `0600` immediately after each observation. No Provider dispatch
was admitted during these checks.

- Conversation retained the draft and showed local-service transport stage,
  retry, diagnostics and a copyable Incident ID.
- RoundTable retained its Mission relationship and showed an actionable local
  service identity failure, recovery, diagnostics and Incident
  `ce3f4af9-05a3-4e09-92ae-6a9d2b2011cb`.
- Mission preflight used one correlation identity through request and failure,
  kept the review content, suppressed the old duplicate raw reason and showed
  diagnostics plus Incident
  `ac1806fb-55cb-4ff1-8ce4-3e7401e70fa9`.
- The nested Mission diagnostic preview opened above the Mission review and
  exposed only App/daemon identity, local service health, non-secret setup
  counts and bounded safe events. Credentials, authorization headers,
  environment credentials, prompts, conversations, Provider bodies and raw
  daemon output were excluded.

## Rapid restart recovery

Installed testing found a race when a new App briefly adopted the previous
daemon's reachable Socket, then lost it after the old managed daemon exited.
The App could remain at `Starting local service` because it had no owned child
or termination callback.

Build 253 monitors an adopted Socket while no owned daemon exists. If the
adopted service disappears, the App starts a new bundled daemon through the
same trusted Socket and canonical identity path. A rapid installed restart
moved from App/daemon `16321/16340` to `16512/16592`; the new daemon bound
`--managed-parent-pid 16512`, restored the private Socket and returned the
Conversation to ready without user intervention.

## Source verification

- Complete macOS suite: 432 XCTest cases passed, two conditional skips, zero
  failures.
- Swift Testing: 20 contract cases passed.
- Structured Mission regressions cover local transport correlation, remote
  stage/Incident preservation and actionable recovery rendering.
- The App-host regression covers adoption of an existing local service and
  automatic recovery when that service disappears during rapid restart.
- `git diff --check` passes.

## Completion boundary

Phase 5 is complete for its defined Codex-inspired Conversation-first UX Goal.
This does not reopen Phase 2D or Phase 4, and it does not claim the explicitly
deferred four-real-Provider Team, live account revoke/rate-limit or custom
endpoint matrix.
