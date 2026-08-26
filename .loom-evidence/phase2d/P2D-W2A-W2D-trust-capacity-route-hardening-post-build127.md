# Phase 2D trust, capacity and Route Transition hardening

Status: `SOURCE VERIFIED / INSTALL PENDING`

Date: 2026-08-24

Build 127 remains the installed predecessor. This evidence covers the current
post-Build-127 source and does not claim a new installed bundle.

## Closed review findings

- A stale route conflict no longer causes the App to silently freeze a new
  binding and retry with `summary_only`. The draft and actionable Incident
  remain visible, and the next dispatch returns to explicit Route Transition
  review.
- Missing immutable source Segment authority is represented as unavailable for
  trust domain, retention mode and data region. It cannot fall back to mutable
  Profile policy or bypass acknowledgement.
- Model and reasoning-effort changes use the same reviewed Transition path as
  Harness, Provider Account, credential, policy and route changes. The reviewed
  target binding and reasoning value are frozen and revalidated at confirm.
- Capacity token counting runs only after policy and scope admission. Filtered
  or access-denied content never crosses the TokenCounter boundary.
- Pi now freezes explicit `unavailable` capacity authority instead of using the
  legacy capacity-free Capsule path.
- Authenticated private-UDS acceptance now includes same-Profile model and
  reasoning transitions in addition to all three context disclosure modes,
  missing/stale authority rejection and privacy-negative checks.
- Trust confirmation is no longer a Swift-only gate. The App emits a canonical
  `route_transition_review_digest` bound to the thread, immutable source
  Segment/binding, exact target binding, disclosure mode and changed trust
  dimensions. The daemon recomputes it inside the per-Conversation authority
  lock, freezes it on the new Segment and Attempt, and rejects omission,
  substitution, drift or replay before any authoritative mutation.
- Capacity admission now validates and canonically orders the complete input
  set before invoking a TokenCounter. Added boundaries cover declared overflow,
  zero/one-token budgets, oversized required items, unavailable model capacity,
  P0/P1 preservation, deterministic omissions and content-free receipts.

## Second parallel revalidation

- A confirmed App transition now consumes its generation immediately. Reusing
  the same review before send cannot replace the selected context mode or
  canonical acknowledgement.
- Route review schema v2 binds target reasoning effort. Swift and Go share one
  canonical digest vector, and loomd recomputes the exact target reasoning,
  binding and context mode under the Conversation lock. Reasoning added after
  review fails before mutation or responder dispatch.
- Stored Build 127 v1 Segment review digests remain readable for migration
  continuity. New wire requests must equal the recomputed v2 acknowledgement.
- Capacity authority rechecks TokenCounter ID/version after counting and rejects
  combined reserved-output/tool-overhead overflow, combined admitted/omitted
  contribution overflow, and extension totals that omit fixed policy/access
  omissions.
- Segment and Attempt now compare the complete canonical Capacity Projection,
  including contribution counts and token totals. Recomputing a plain binding
  digest cannot make an independently valid but drifted Attempt projection
  authoritative.
- The authenticated private-UDS matrix independently covers credential
  revision, trust domain, retention and data-region changes, plus trust-review
  reasoning tamper, with zero thread mutation and zero responder dispatch on
  rejection.

## Third parallel closure

- Route review schema v3 also binds the exact target Conversation Profile ID.
  A Profile alias cannot substitute a different route after confirmation even
  when the remaining binding fields match. Swift and Go share the canonical
  digest vector
  `fcc87e6a321b2f7b303831173b63c928a394f5abe0519df5efcb94ae01474398`.
- Missing or incomplete v2 policy authority always crosses a review boundary,
  including a legacy blank-to-blank transition. Changing model or reasoning
  after review consumes the confirmation and requires a new one. A Segment
  without frozen binding authority fails closed and exposes Start new
  conversation rather than a dead-end reload action.
- Conversation capacity counting occurs only inside policy/scope admission.
  Every dispatch freezes an independent Attempt Capsule, disclosure receipt and
  capacity usage projection; the Segment opening Capsule remains immutable.
- Production Team role, dependency and aggregation Capsule construction now
  shares one frozen capacity authority and TokenCounter. Counter drift,
  overflow, invalid extension and unsafe source substitution fail closed.
- Combined acceptance proves a reviewed Codex-to-Loom-Native/DeepSeek
  transition in one visible Conversation, then a second target turn with the
  same target Segment and Gateway Session but a different Attempt Capsule and
  binding digest.

## Harness Gateway checkpoint

The P2D-HG1 core contract is now implemented and race-verified: immutable
Configured Harness registry, deterministic Segment Session identity, exact
Response authority validation, same-Session serialization, cross-Conversation
concurrency, response-scoped cancellation and content-free monotonic events.
Production composition registers Codex, Claude Code, OpenCode, Pi and Loom
Native. Claude Code additionally resumes one frozen native CLI session identity
across same-Segment turns; installed behavior is not claimed here.

## Verification

- `go test -p 4 ./... -count=1`: PASS.
- `go test -race ./internal/contextcapsule ./internal/api ./cmd/loomd -count=1`:
  PASS.
- `go test -race ./internal/harnessgateway -count=1`: PASS.
- `go vet ./...`: PASS.
- `swift test --package-path apps/macos`: 344 tests, 2 conditional skips,
  0 failures.
- `git diff --check`: PASS.

The complete Go repository and `go vet ./...` also pass after the cross-layer
review contract. A shared Go/Swift canonical digest vector prevents wire-format
drift. The full run additionally corrected the HG1 Codex App Server boundary so
its process directory and native thread CWD use the frozen workspace while the
private prompt file remains under the private runtime root.

No credential was read or changed and no external Provider request was made.
Installed-App packaging and UI acceptance remain required before this source
hardening can supersede Build 127 as installed evidence.

## Revalidation results

- `go test ./internal/contextcapsule ./internal/api -count=1`: PASS.
- Focused private-UDS Route Transition and combined authority tests: PASS.
- Focused `-race` across `internal/contextcapsule`, `internal/api` and
  `cmd/loomd`: PASS.
- `go vet ./internal/contextcapsule ./internal/api ./cmd/loomd`: PASS.
- Full `swift test --package-path apps/macos`: 352 tests, two conditional gates
  skipped, zero failures.
- Repository-wide `go test -p 4 ./... -count=1`: every package passed except
  `internal/localipc`, whose default package-level ten-minute budget expired
  while the strict Swift setup probe was compiling. The exact probe passed
  alone in 165.277 seconds; the complete package passed with `-timeout 15m` in
  435.475 seconds.
- Scoped `git diff --check`: PASS.

These are source results. Build 127 remains the installed predecessor; no new
bundle, credential access or external Provider call is claimed.

## Latest closure verification

- Affected Context Capsule, Chat API, Team execution, Harness Gateway, Harness
  adapter and daemon packages: PASS.
- Combined Route Transition, second target-turn Session reuse and production
  Team capacity acceptance: PASS.
- Focused race tests, `go vet ./...` and `git diff --check`: PASS.
- Full `swift test --package-path apps/macos`: 355 tests, two conditional skips,
  zero failures.
- Repository-wide Go run: every product package passed. One Pi cancellation
  child-start marker timed out only under full parallel load; the exact test and
  the complete `internal/runtime/piadapter` package passed on immediate rerun.
  `cmd/loomd` passed in 306.851 seconds and `internal/localipc` in 599.481
  seconds.

No credential, network, package, installation or installed-App action was used.
This closes the requested source work only; Build 127 remains the installed
predecessor.

## Parallel all-transition and capacity-admission revalidation (2026-08-25)

Three additional source gaps were found and closed:

- dispatch-safe rebuilding preserves the exact Capacity Projection and matching
  TokenCounter instead of dropping to the legacy builder;
- controlled Mission, Conversation and Team role/dependency/aggregation
  admission reject capacity-free production Capsules;
- every existing-thread new Segment with binding authority requires a canonical
  v3 Route Transition review, including same-trust model, reasoning, Provider,
  account and credential revision changes.

Additional acceptance proves unknown Harness dispatch invokes no executor,
same-Provider multi-account routing leases the exact account/reference/revision,
and ADR-0022 Codex-to-Loom-Native Segment transition carries complete capacity
authority through the Gateway lifecycle.

Verification:

- `go test -p 4 ./... -count=1 -timeout=15m`: PASS; `cmd/loomd` 273.638s,
  `internal/localipc` 460.562s, Pi adapter 111.539s.
- Focused `-race` across Context Capsule, Chat API, Team and daemon Route/
  Gateway/Conversation acceptance: PASS.
- `go vet ./...`: PASS.
- `swift test --package-path apps/macos`: 359 XCTest cases and 20 Swift Testing
  cases passed; two conditional visual-export tests skipped.

No credential, network, package, installation or installed-App action was used.
Build 127 remains the installed predecessor.
