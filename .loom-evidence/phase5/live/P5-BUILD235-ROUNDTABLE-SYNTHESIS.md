# Phase 5 Build 235 RoundTable synthesis acceptance

Status: `INSTALLED CHECKPOINT / PHASE 5 IN PROGRESS`

## Boundary

This checkpoint verifies the Conversation-side, Mission-linked RoundTable flow
without recording Provider output, prompts, credentials or conversation text in
the evidence tree.

## Installed identity

- App version: `0.5.3`
- App build: `235`
- App bundle: `/Users/lune/Applications/Loom.app`
- Source branch: `codex/phase4-roundtable-governed-deliberation`
- App executable digest: `3d680ff95ea44c6db962756f37cbc99ded58b06f69b8ed77f8d86f1e51f4f03a`
- Bundled daemon digest: `dd1f3cd5f3b810d19b9e306f91b90ffd22459de4b0c4cca980c88699fb77ad41`

## Live matrix

- Opened Loom directly; the bundled daemon started as its managed child.
- Restored the exact Mission-linked RoundTable beside the Conversation.
- Ran two independently frozen Loom Native/MiniMax Agent seats.
- Verified the authoritative Mission context item reached each seat Capsule.
- Verified prior Agent outputs reached follow-up Attempts only as bounded,
  provenance-bound untrusted context.
- Observed one seat-local timeout while the successful peer remained visible.
- Used the visible Incident state and bounded retry composer; the retried seat
  succeeded without replacing the original discussion prompt.
- Completed a substantive Lead synthesis and independent peer critique.
- Explicitly accepted the Lead synthesis.
- Quit Loom and cold-started the installed App; the concluded Session and both
  latest Agent results restored.

## Automated gates

- Complete Swift package: 416 XCTest cases passed, two conditional skips.
- Swift Testing contracts: 20 passed.
- Complete Go repository passed.
- `go vet ./...` passed.
- Focused RoundTable, App, native Runtime and daemon race suites passed.
- Strict installed bundle signature passed; packaged and installed App/daemon
  executable digests match exactly.

## Privacy

This record contains only non-secret identities and outcome metadata. It omits
API keys, Authorization headers, prompts, Agent output, Provider response bodies
and hidden reasoning.

Build 235 is an installed Phase 5 checkpoint, not Phase 5 completion.
