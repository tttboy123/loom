# Phase 5 Build 227 Mission continuity acceptance

Status: `INSTALLED ACCEPTED CHECKPOINT / PHASE 5 IN PROGRESS`

Date: 2026-08-27

## User outcome

Loom now starts a genuinely new Mission when the selected saved Team already
has Mission history. The user-entered Mission title remains visible beside the
linked conversation. Accepted Agent results remain readable after the App and
its bundled daemon both restart.

This closes the installed Mission identity and result-continuity defect. It
does not close Phase 5: a new real multi-round RoundTable, broader installed
recovery paths and complete keyboard traversal remain open.

## Installed identity

- App: `/Users/lune/Applications/Loom.app`
- version: `0.5.3`
- build: `227`
- App executable SHA-256:
  `d0cfd746164c1f4e609606b5d7b034877c940c3667ddce7d0ecf61d298b656ab`
- bundled daemon SHA-256:
  `3a51fe37750b83150b9caa5b3affc7b56e9c6113494a240608526d940c5114a4`
- strict deep code-signature verification: passed
- daemon lifecycle: managed child of the App with canonical state, isolation,
  Runtime, socket and managed-parent arguments

## Real Mission

- Mission ID:
  `mission/team-instance-c11a01b82dab3919c50861582fa0a1cc`
- TeamInstance ID: `team-instance-c11a01b82dab3919c50861582fa0a1cc`
- visible title: `Build 225 Mission continuity`
- linked conversation: `thread-1e6f34bc58c186f6c20c5334ab548099`
- Runtime route: Loom Native, `minimax.primary`, `MiniMax-M3`
- outcome: two of two Agent steps succeeded

The App returned to the linked conversation after the explicit start action.
Both Agent outputs were visible while the Mission completed. A full App and
daemon cold restart restored the same Mission title, terminal status, frozen
routes and both accepted outputs.

## Authority and privacy

The daemon reads final output only from the exact terminal Evidence receipt
bound to TeamInstance, logical node, Attempt, WorkItem and Run. It verifies the
receipt digest, output summary digest, closed aggregation schema and bounded
UTF-8 content before projecting a digest-bound final output on the current
board node.

The projection does not append or synthesize a Journal event and does not
change timeline pagination. Prompt text, credentials, Authorization headers,
hidden reasoning and raw Provider responses are not added to Journal facts,
diagnostics or this evidence document.

## Verification

- `go test ./...`: passed
- `go vet ./...`: passed
- `go test ./internal/localipc`: passed, including the strict Swift contract
  probe
- macOS XCTest: 415 tests, 2 conditional skips, 0 failures
- Swift Testing: 20 contract tests, 0 failures
- `git diff --check`: passed
- installed cold-start accessibility inspection: passed

Visual evidence:

- `P5-BUILD225-MISSION-CONTINUITY.png`: live terminal checkpoint that exposed
  the stale presentation-title and restart-output gaps
- `P5-BUILD227-MISSION-RESTART.png`: corrected cold-start title, two succeeded
  Agent results and accepted-Evidence authority message

## Open Phase 5 gates

- installed new multi-round RoundTable with Retry or Replace and conclusion
  acceptance;
- installed cross-surface error and recovery paths;
- complete keyboard traversal on a system with Full Keyboard Access enabled.
