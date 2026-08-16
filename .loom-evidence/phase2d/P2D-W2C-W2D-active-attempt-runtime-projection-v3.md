# P2D-W2C/W2D Active Attempt Runtime Projection V3

Status: `SOURCE VERIFIED / ROUTE SEGMENT AND MULTI-STEP CONSUMPTION OPEN`

Date: 2026-08-14

## User-visible conclusion

The product daemon can now resolve an executing Agent Attempt from a complete,
non-secret identity without accepting a client-supplied execution binding. The
entry exists only while the governed Runtime delegate is active and is revoked
on every return path. This is an internal prerequisite; no Queue, Steer or
Inject IPC is exposed yet.

## Implemented boundary

- The mission executor creates one active-Attempt registry beside its existing
  Attempt Loop authority and Agent Inbox coordinator. Every governed Runtime
  adapter shares that exact registry.
- Registration occurs only after the Adapter request has passed
  `productAttemptLoopBinding`, the first Turn/Step and model request are
  admitted, and `ValidateFrozenExecutionBinding` reproduces the exact binding.
- The lookup key freezes Conversation, Attempt-local Inbox Segment, Agent,
  WorkItem, Run and claim generation. Any missing field or stale generation
  fails closed.
- Registry results contain the validated Attempt Loop binding, budget, frozen
  execution binding, Capsule digest and Incident ID. Mutable capability and
  budget values are defensively cloned.
- A per-registration token and idempotent close prevent an old cleanup path
  from deleting a different registration. The registry is a revocable runtime
  projection only; it does not write Journal facts or grant execution authority.
- The record contains no Prompt, transcript, tool result, Provider response,
  API key, VMK or plaintext Agent input. Credential identity remains only the
  existing opaque reference and revision in `FrozenExecutionBinding`.

## Route Segment limitation

The current Team `ContextCapsule.AuthorityRecord` does not carry a Route Segment
ID. V3 therefore derives a stable Attempt-local Inbox Segment from the existing
Conversation/Run/generation/Agent authority. This is sufficient to prevent
cross-Attempt Inbox mixing, but it is not evidence that Conversation Route
Segments are wired through Team dispatch. Product ingress remains blocked until
the authoritative Segment identity is propagated or an accepted explicit
mapping contract replaces this temporary attempt-local identity.

## Acceptance evidence

- RED failed only on the absent registry, registry-aware adapter constructor,
  query and Attempt-local Segment derivation.
- While a managed delegate is blocked, exact identity lookup returns the
  expected Provider Account, execution digest and Capsule digest.
- Mutating the returned capability slice does not alter a second lookup.
- A one-generation stale query returns `active Agent Attempt not found`.
- Releasing the delegate removes the entry before `Execute` returns; lookup then
  fails closed.
- Mission executor construction proves the deferred Runtime and executor retain
  one registry instance together with one Agent Inbox coordinator.

## Verification

Passed:

```text
go test ./cmd/loomd -run 'TestProductMissionExecutorComposesAgentInboxWithAttemptAuthority|TestProductAttemptLoopRegistersExactActiveAttemptOnlyDuringExecution' -count=20
go test -race ./cmd/loomd -run 'TestProductMissionExecutorComposesAgentInboxWithAttemptAuthority|TestProductAttemptLoopRegistersExactActiveAttemptOnlyDuringExecution' -count=20
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
gofmt -d <changed Go files>
git diff --check -- <active Attempt, composition and status files>
```

## Open gates

- Propagate an authoritative Route Segment identity through Context Capsule and
  Team dispatch, or accept a versioned mapping contract. Do not treat the
  Attempt-local Inbox Segment as a Provider-native or Conversation Segment.
- Extend the Runtime from its current one-Turn/one-Step envelope and consume
  Queue into the next Turn and Steer/Inject into the exact next Step.
- Assemble model-visible input only after atomic consumption, then zeroize its
  plaintext after the Runtime accepts or rejects it.
- Add diagnostics, authenticated IPC and Swift controls only after those
  consumption and Segment gates pass.
- Cross-Runtime conformance, CV6 and mixed-Team ATL9 remain open.

No App was built, signed, launched or installed. No network, Provider, real
credential, user workspace or external Runtime was accessed. Phase 2D remains
the sole `ACTIVE / PARTIAL` Goal.
