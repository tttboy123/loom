# P2D-W2A/W2C/W2D Authoritative Route Segment Binding V4

Status: `SOURCE VERIFIED / MULTI-STEP CONSUMPTION AND INSTALLED LIVE OPEN`

Date: 2026-08-14

## User-visible conclusion

Every governed Team Agent Attempt now has a versioned Route Segment identity
that is frozen beside its Context Capsule and Execution Binding. The Segment is
projected to the Team board and strict Swift client, so later Queue, Steer and
Inject controls can target an exact Agent route without accepting an internal
binding from the client. Runtime consumption and installed-live behavior remain
open.

## Implemented boundary

- `contextcapsule.RouteSegmentBinding` is a content-free v1 contract over
  Segment, Conversation, Team, Agent, Role, Attempt number, Capsule digest and
  Frozen Execution Binding digest. Its SHA-256 digest uses a dedicated canonical
  domain.
- Team dispatch deterministically creates a distinct Segment for every logical
  Attempt. The binding is part of `TeamReadySetDispatched`, the context selection
  digest and `TeamAttemptRecord`.
- Team replay validates the complete Segment contract and recomputes the exact
  deterministic binding. Conversation, Agent, Role, Attempt, Capsule or
  Execution substitution fails closed.
- The application coordinator rebuilds the same Segment when an Aggregation
  Capsule changes, preserves it across claim-generation recovery, and compares
  the authoritative dispatched binding before executing a task.
- Supervisor requires the Segment whenever a Context Capsule is present and
  checks it against the exact Capsule authority, Agent identity and frozen
  Execution Binding before calling a Runtime adapter.
- The product Attempt Loop and active-Attempt registry now consume the supplied
  authoritative Segment. The previous deterministic Attempt-local substitute
  has been removed.
- Projection and Team board expose only `route_segment_available`, Segment ID
  and Segment digest. No Capsule content, Prompt, Provider response or secret is
  added to the Journal or board.
- Swift `LocalProductNode` accepts absent fields from older daemons, but when a
  Segment is available it requires an execution binding, Context Capsule,
  bounded identifier and canonical digest. Unknown fields remain rejected.

## Compatibility

Context Capsule schema v1 is unchanged, so persisted encrypted Capsules and
their receipt digests are not invalidated. Legacy projection events without a
Context Capsule may omit Route Segment fields. New authoritative Team dispatch
with a Capsule always requires the binding.

## Acceptance evidence

- Contract RED failed on the missing Route Segment types; authority RED failed
  on the missing Team Attempt binding; Supervisor RED failed on the missing
  transport field.
- Attempt 1 and Attempt 2 with the same logical route receive distinct Segment
  identities and digests.
- Team Journal replay preserves the exact binding and rejects route lineage
  drift.
- Parallel Provider siblings, Aggregation and explicit fallback retain
  independent Segment/Attempt lineage under race execution.
- Active registry lookup uses the authoritative Segment and still revokes it on
  delegate completion.
- The first repository run exposed two strict Swift `invalid_response`
  failures. Updating the closed Swift DTO fixed both real Go-to-Swift loopback
  contracts without weakening unknown-key rejection.

## Verification

Passed:

```text
go test -race ./internal/contextcapsule ./internal/work ./internal/supervisor ./internal/projection ./internal/api ./cmd/loomd -run '<Route Segment focused matrix>' -count=10
go test -race ./internal/app -run 'TestParallelRouteSiblingsDispatchIndependentAttemptsThenAggregation|TestFourProviderTeamRevokedCredentialIsolatesOneAgent|TestPhase2DTeamExecutionUsesOnlyExactApprovedFallbackBinding' -count=3
go test ./cmd/loomd -run 'TestProductDaemonServesAuthoritativeNilCollectionsToStrictSwiftClient|TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage' -count=1
go test ./... -count=1
swift test --package-path apps/macos
go vet ./...
gofmt -d <changed Go files>
git diff --check -- <Route Segment Go, Swift and evidence files>
```

Swift result: 203 XCTest cases passed, one visual-export test skipped by its
existing environment gate, plus 10 Swift Testing cases passed.

## Open gates

- Extend the product Attempt Runtime beyond one Turn and one Step.
- Atomically consume Queue into the next Turn and Steer/Inject into the exact
  next Step, then decrypt and zeroize only that consumed input set.
- Add authenticated daemon IPC, operational stages and Swift governance
  controls after the model-input consumption path exists.
- Complete cross-Runtime conformance, installed CV6 and mixed-Team ATL9.

No App was signed, launched or installed. No network, Provider, real credential,
user workspace or external Runtime was accessed. Phase 2D remains the sole
`ACTIVE / PARTIAL` Goal.
