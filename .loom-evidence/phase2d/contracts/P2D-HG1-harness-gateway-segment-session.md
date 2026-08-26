# P2D-HG1 Contract: Harness Gateway and Segment Session

Status: `ACCEPTED / INSTALLED PASS / BUILD 136`

Parent Goal: `Phase 2D - Loom Harness Gateway and Segment Session`

ADR: `docs/adr/0022-harness-gateway-and-segment-session.md`

## Required artifacts

- versioned `ConfiguredHarness` for Codex, Claude Code, OpenCode, Pi and Loom
  Native;
- immutable built-in `BackendRegistry`;
- exact `SegmentSessionBinding` and deterministic Session identity;
- content-free unified Session/Response event stream;
- per-Session serialization, cross-Conversation concurrency and exact
  Response cancellation;
- Codex App Server Session/Workspace reuse across same-Segment turns;
- compatibility Backends removed from direct production dispatch after all
  five Harness registrations pass parity;
- private UDS and Swift cancellation projection;
- source, race, restart, privacy and installed-live evidence.

## Frozen authority

Every open Session freezes Configured Harness ID/version, backend ID/version,
Conversation, Segment, Workspace digest, execution-binding digest, Provider,
Provider Account, credential revision, model, reasoning effort, immutable
Segment opening Capsule digest and governance-policy digest. Every Response also
freezes its current Attempt Capsule digest. The Gateway compares the Session
fields exactly, validates the Response fields independently and never resolves
them from Provider ID alone.

## Completion gate

This contract was accepted when the installed App completed the Codex
reuse/switch/concurrency/cancel matrix and all five Harnesses became reachable
only through the registered Gateway path. Green unit tests without production
composition or installed behavior remain partial evidence for later changes.

## Current verified slice

- Production composition registers Codex, Claude Code, OpenCode, Pi and Loom
  Native with the built-in Gateway registry.
- Codex owns a persistent App Server Session for each immutable Conversation
  Segment and reuses its exact thread across same-Segment turns.
- The frozen Session authority includes reasoning effort in addition to the
  execution binding, model, Context Capsule and governance policy.
- Exact Response cancellation is projected from Swift through the private UDS
  and product API to Gateway and Codex `turn/interrupt`; a completed interrupt
  leaves the Session reusable.
- Lifecycle tests cover opening admission, idle/max-age drain, exact-once close,
  Gateway shutdown, cross-Conversation concurrency and cancellation. Session
  opening has an independent 30-second production bound; timeout releases the
  exact Harness slot, and a late Backend success is cleaned before retry.
- Unified events freeze event schema, Configured Harness version and Backend
  version while remaining content-free and ordered at the EventSink boundary.
  Operational diagnostics retain and revalidate that complete envelope,
  including event time, sequence, Session/Segment/Workspace/Response identity
  and every privacy-safe frozen binding/Capsule/policy field. The App diagnostic
  export preserves the same v3 authority without Prompt, Provider body,
  credential reference or workspace path. A random non-secret Gateway Instance
  ID prevents sequence and relationship evidence from different daemon
  lifecycles being combined.
- Unhealthy exact Sessions finish native cleanup before terminal failure and
  before the same Session identity may reopen. Session cleanup is independently
  time-bounded in production.
- Registered compatibility Backends have been replaced by governed Segment
  Backends. Codex owns a persistent native process/thread. Claude Code preserves
  one native CLI session identity across same-Segment responses with
  `--session-id` and `--resume`, while using a bounded process per response.
  OpenCode, Pi and Loom Native remain one-shot native protocols inside
  persistent Loom-owned Segment authority.
- Source acceptance proves same-Segment Codex reuse, reviewed
  Codex-to-Loom-Native/DeepSeek Segment transition in one visible Conversation,
  a second target turn reusing the target Gateway Session with an independent
  Attempt Capsule, exact frozen bindings, privacy-negative events and
  cross-Conversation overlap.
- Production composition publishes a native-auth Claude Code Conversation
  Profile through the registered Claude Code Backend without a credential lease.
  The five built-in Harness identities each have source-reachable Conversation
  paths and exact frozen binding/event acceptance.
- Cancellation remains exact when the outer request is cancelled after Codex
  has accepted `turn/start` but before Loom observes the native turn ID
  response. The adapter boundedly recovers that exact ID, interrupts it and
  waits for the interrupted terminal event before Session reuse.
- Startup reconciles persisted ownerless `dispatching` Conversation Attempts
  to durable content-free `cancelled` Attempts before serving requests. The
  encrypted store keeps monotonic revisions through this recovery.
- A real private-UDS source acceptance crosses Local IPC, product Chat API,
  Gateway and the Codex fixture to prove exact cancellation, concurrent peer
  progress and same-Session recovery.
- The G7 source verifier accepts the daemon operational trace, but installed G7
  evidence requires the App's owner-only privacy-safe diagnostic bundle and an
  exact match between its App/daemon build hashes and the installed bundle. The
  installed path validates the complete closed App export schema, retains a
  bounded 512-event window and rejects raw JSONL, missing fields and nested
  unknown metadata. It rejects unknown/content-bearing fields and
  proves Codex reuse, reviewed Codex-to-DeepSeek Segment transition, two target
  responses reusing the exact DeepSeek Session and frozen non-secret authority,
  cross-Conversation overlap, exact cancellation and post-cancel reuse within
  one Gateway Instance. It additionally proves two Codex completions precede
  the target Session opening. Persisted evidence revalidates the complete
  versioned summary rather than trusting a stored pass label. Multiple retained
  instances remain diagnosable but cannot jointly satisfy the matrix.
- The controlled full repository passes with `-p 2`; `cmd/loomd` completes in
  199.682 seconds, `internal/localipc` in 270.873 seconds and the Pi adapter in
  69.219 seconds. The first `-p 4` completion-audit run exposed one transient
  local UDS assertion under build pressure; the exact test passed 20 repeated
  runs and the full package passed serially before the controlled rerun. Key
  cross-layer race tests, vet, the G7 source fixture and 365 XCTest plus 20
  Swift Testing cases pass. Two visual-export tests are conditionally skipped.
  Historical Gateway v1/v2 diagnostics remain read-compatible but only
  instance-bound v3 events can satisfy G7.
- Registry tests prove construction and resolution return copied Configured
  Harness authority, while replay tests prove a completed Response ID cannot
  reach the Backend twice within one Gateway instance.
- The strengthened G7 export and summary harness passes 20 repeated runs;
  negative fixtures cover raw JSONL installation export, incomplete/unknown
  bundle shape, weak persisted summaries and target opening before source
  completion.

## Accepted completion evidence

- Preserve truthful protocol semantics: add native persistent handles for
  OpenCode, Pi or Loom Native only where that runtime actually exposes a stable
  resumable identity, and do not represent Claude Code's per-response process
  as persistent.
- Build 136 completed G7's Codex reuse/switch/concurrency/cancel matrix using
  the privacy-safe App diagnostic export and the canonical owner-controlled
  installation at `$HOME/Applications/Loom.app`.
- The exact App and bundled daemon hashes are frozen in
  `../P2D-HG1-build136-installed-g7.md`; no other bundle path can be paired with
  that evidence.
