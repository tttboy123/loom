# P2D-HG1 Build 136 Installed G7 Acceptance

Status: `ACCEPTED / INSTALLED PASS`

Date: 2026-08-25

## Installed identity

- Product: Loom `0.5.3` Build 136
- Canonical installation: `$HOME/Applications/Loom.app`
- App SHA-256: `8f32efe304ee604315b401389004ea3a8abf464818d01413dd0fcddb7f276657`
- Bundled daemon SHA-256: `9dbffa286b430b2833ae04497a0943886dfd8c45762a549b4fc04391d673e786`
- Deep and strict code-signature verification: PASS

## Installed matrix

The App-exported diagnostic bundle passes `scripts/phase2d-hg1-trace.jq` with
one Gateway Instance and one complete privacy-safe v3 lifecycle:

1. Two Codex responses complete in the immutable source Segment before the
   target Session opens.
2. A reviewed Route Transition creates a Loom Native/DeepSeek Segment in the
   same visible Conversation.
3. Two target responses reuse that Segment Session with independent Attempt
   Capsule digests and unchanged target execution authority.
4. A peer Conversation response overlaps the cancelled response.
5. Cancellation freezes one exact Incident, Response and Attempt.
6. The cancelled Session remains reusable and completes a later response.

The selected cancellation Incident is
`loom-chat-e9331db5-c4ee-4897-8076-8d5227546799`. The owner-only generated gate
document is stored under
`.build-artifacts/live-build136/evidence-g7-pass/` and the user-exported bundle
is `/Users/lune/Documents/loom-diagnostics-build136-g7-pass.json`.

## Verification

- Reproducible production App build: PASS
- `swift test --package-path apps/macos`: 368 XCTest cases, two conditional
  skips, zero failures; 20 Swift Testing cases, zero failures
- `go test ./internal/contextcapsule ./internal/api ./internal/runtime/harnessadapter ./internal/harnessgateway -count=1`: PASS
- `go test ./cmd/loomd -count=1 -timeout=15m`: PASS
- `go vet ./...`: PASS
- `scripts/test-phase2d-live-acceptance.sh`: PASS
- `git diff --check`: PASS

## Privacy boundary

The accepted evidence contains only non-secret build identity, safe opaque
Gateway/Session/Segment/Response/Attempt identifiers, versioned event sequence,
execution authority metadata and digests. It excludes credentials, credential
references, authorization headers, environment credentials, Prompt and
transcript content, Provider bodies, raw console output and Workspace paths.
