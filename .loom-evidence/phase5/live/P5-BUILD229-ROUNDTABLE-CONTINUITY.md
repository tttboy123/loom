# Phase 5 Build 229 RoundTable continuity checkpoint

Status: `INSTALLED ACCEPTED CHECKPOINT / PHASE 5 IN PROGRESS`

Date: 2026-08-27

## User outcome

RoundTable Retry now continues the original discussion instead of replacing it
with the user's retry guidance. The failure reason and recovery controls are
consistent between the conversation inspector and full workbench. A concluded
Session restores both Agent outputs after an App and daemon cold restart.

This is not Phase 5 completion. The real run proved that the next synthesizing
Attempt still needs relevant prior Agent results as bounded, provenance-bearing
untrusted context before Loom can claim substantive multi-Agent deliberation.

## Installed identity

- App: `/Users/lune/Applications/Loom.app`
- version: `0.5.3`
- build: `229`
- App executable SHA-256:
  `c704f5150d7deca1befdf5707762b660d5d6fe4c9b0f7b9a87106c432d35258e`
- bundled daemon SHA-256:
  `829dc776df68841fdbc0a8b08caf53ada030c1bba52e0bbd4da65f0b754fcb3d`
- strict deep code-signature verification: passed
- daemon lifecycle: managed child of the App with canonical state, isolation,
  Runtime, socket and managed-parent arguments

## Real workflow

- Mission: `mission/team-instance-c11a01b82dab3919c50861582fa0a1cc`
- linked conversation: `thread-1e6f34bc58c186f6c20c5334ab548099`
- RoundTable Session: `rt-20260827-0827`
- visible Mission title: `Build 225 Mission continuity`
- seats: two Loom Native Agents using `minimax.primary` / `MiniMax-M3`

Build 227 had already recorded one failed participant and a Lead retry whose
Capsule had lost the original discussion prompt. Build 228 retried the remaining
participant with the bounded guidance `Answer the original RoundTable question
directly in one concise sentence.` The new Attempt succeeded and explicitly
responded to the original request to compare Agent results, proving the original
prompt was present. It no longer reported that the prompt itself was missing.

The response also correctly reported that no prior Agent result artifacts were
present. This is retained as evidence of the next Context Capsule gap rather
than treated as a successful synthesis. The user-facing workflow allowed the
Session to conclude only after both seats were resolved. Build 229 then cold
started the installed App and restored the concluded Session, both frozen routes
and both visible outputs beside the same conversation.

## Authority and privacy

Retry locates one exact prior Context Capsule authority by Conversation,
execution Team, Agent, role and Capsule digest. It reads the original
`roundtable-prompt` only when its kind, trust, scope, priority, source and
Session/Round provenance all match. Duplicate, missing or substituted authority
fails closed.

The new Capsule keeps the prompt as an authoritative conversation goal and adds
retry guidance separately as a role-restricted confirmed constraint. Prior model
output is not promoted. Prompt and guidance are absent from Journal payloads
and diagnostics. This document includes only the explicit non-sensitive retry
guidance used for installed acceptance; it does not include the protected
discussion prompt or any Provider response body.

## Verification

- `go test ./...`: passed
- `go vet ./...`: passed
- focused RoundTable application, authority and daemon route suites: passed
- macOS XCTest: 415 tests, 2 conditional skips, 0 failures
- Swift Testing: 20 contract tests, 0 failures
- strict deep code-signature verification: passed
- installed cold-start accessibility inspection: passed; the inspection result
  is recorded as non-content metadata only, without retaining Provider output
  screenshots in Evidence

## Open Phase 5 gates

- disclose relevant prior Agent results to a new synthesis Attempt only as
  bounded, provenance-bearing untrusted context;
- complete a fresh substantive multi-round RoundTable and accept its conclusion;
- finish broader installed error-path and complete keyboard traversal acceptance.
