# P2D-W2A/W2C Parallel Route Aggregation v1

Status: `SOURCE VERIFIED / POST-BUILD-64`

Date: 2026-08-13

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Accepted boundary

- One stable Agent may own two or more explicit `route_sibling` execution
  nodes and exactly one `aggregation` node in one versioned route group.
- Every sibling and aggregation node creates a separate Attempt and freezes one
  Execution Binding. A single Attempt never carries multiple Providers.
- Duplicate stable Agent identity is accepted only inside a valid group with
  identical role and external dependencies. Missing or extra sources, direct
  sibling consumers, group drift, and Agent drift fail closed.
- The product limit remains nine unique Agents. Explicit physical route and
  aggregation nodes are separately bounded to 27.
- Ordinary ExecutionPlan canonical bytes are unchanged. Explicit node kind and
  group enter the plan digest and survive Journal replay, planning-wave
  reconstruction, Projection, Board wire, and Swift decoding.

## Aggregation authority

- The coordinator releases Synthesis only after every exact sibling dependency
  is succeeded. A failed sibling leaves an independent healthy sibling intact
  and does not trigger an implicit Provider fallback.
- The Evidence Store reads the exact finalized receipt, verifies the artifact
  digest, strictly decodes the Attempt artifact, and extracts only authorized
  `MessageEvent` payloads. Ack, result, private runtime frames, and scratchpad
  are excluded.
- The Aggregation Capsule records Attempt, Evidence, and output-summary lineage
  as authoritative metadata. Model-produced output is always
  `untrusted_model_output` with source provenance. It cannot become system or
  authoritative context.
- Agent, route group, plan node, Evidence digest, output-summary digest, or
  source-node substitution fails closed. Base Capsule omissions are rejected at
  this boundary; aggregation-output budget omissions remain explicit in the
  Capsule manifest.
- Provider Account accounting is retained for each sibling and Synthesis Run;
  no provider-global bucket is introduced.

## User-facing governance

The Team inspector labels projected physical rows as `Parallel provider route`
and `Synthesis`. It continues to show each row's Harness, Provider Account,
Model, credential revision, limits, status, diagnostics, and accounting. The
opaque route-group ID is not rendered in user-facing copy.

## Privacy checks

Journal and Board state contain no sibling model output, Prompt, Provider body,
credential, Authorization Header, ciphertext, nonce, or private scratchpad.
Aggregation content exists only in the bounded dispatch Capsule and exact
Evidence read path. Mutable evidence buffers are zeroized after use.

## Verification

- `go test ./... -count=1` passed with exit code 0.
- `go test -race ./internal/teams ./internal/app ./internal/work ./internal/projection ./internal/api ./internal/evidence ./internal/contextcapsule -count=1` passed.
- `swift test --package-path apps/macos` passed 202 XCTest cases with one
  intentional visual-export skip, plus nine Swift Testing contracts.
- `git diff --check` passed.

## Remaining boundary

This slice supplies generic source execution authority and governance. It does
not yet provide product Team-builder RouteSet authoring, ordinary Conversation
parallel-route UX, real Harness aggregation, installed restart proof, CV6, or
live Provider/mixed-Team acceptance. No bundle was built, launched, or
installed, and no real credential or Provider was accessed. Installed Loom
remains v0.5.2 build 39. Phase 2D remains the sole `ACTIVE / PARTIAL` Goal.
