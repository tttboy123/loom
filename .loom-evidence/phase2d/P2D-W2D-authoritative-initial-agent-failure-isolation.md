# P2D-W2D Authoritative Initial Agent Failure Isolation

Date: 2026-08-12

Status: SOURCE PASS / STATIC CANDIDATE; INSTALLED LIVE MATRIX OPEN

## Acceptance boundary

This increment closes the source gap where one Agent's preflight failure
rejected the entire Team before an authoritative plan existed. It does not
claim the installed mixed-Provider live matrix is complete.

## Implemented authority

- `TeamExecutionPlanned` freezes one privacy-safe initial Route summary for
  every Agent.
- Each direct failure becomes one append-only `TeamNodeInitiallyBlocked` fact.
- Only declared dependents receive canonical `dependency_blocked` facts.
- Independent healthy siblings remain schedulable.
- Initially blocked nodes receive no Attempt, grant, capacity reservation, or
  Adapter call.
- An all-blocked Team receives observable terminal authority with zero
  Attempts.
- Attempt 1 must match the initial Route. Retry or approved fallback remains a
  new immutable Attempt binding.

## Failure and privacy contract

The direct codes cover credential unavailability, credential revision
conflict, Runtime unavailability, incompatible binding, capacity
unavailability, and unavailable approved fallback. Codes and stages use a
strict lowercase identifier grammar. Reasons are bounded, trimmed, valid UTF-8
without control characters. Dependency failures retain source provenance and
retryability.

The Team Board receives only Harness, Provider, Provider Account, Model,
reasoning, timeout, budget, capabilities, credential revision, safe diagnostic
metadata, and Incident ID. It never receives credential reference, endpoint
fingerprint, binding digest, secret bytes, Prompt, Capsule/transcript content,
or Provider response.

## User experience

Team Pulse can show the exact Route and actionable failure before Attempt 1.
The affected Agent remains visible with diagnostics and retry review while
healthy peers continue independently. No-attempt state is labeled `Not
started`; a native credentialless Route is labeled `Native auth`. The Team is
not collapsed into generic `offline` or `Unavailable` presentation.

Swift Store preflight readiness follows the same authority: a mixed
`blocked + ready` preflight permits Start, while an all-blocked preflight
remains fail closed. A RED test first proved the old all-nodes-ready gate
prevented the Start command; the minimal Store fix and both positive/negative
regressions then passed.

## Verification

Passed on the dirty shared source lineage at
`651f156afda37a8e703cbc0396f9f38b7912600b`:

```text
go test ./internal/teams ./internal/work ./internal/projection ./internal/api ./internal/app -count=1
go test -race ./internal/teams ./internal/work ./internal/projection ./internal/api ./internal/app -count=1
go vet ./...
git diff --check
go test -p 1 ./... -count=1 -timeout=15m
swift test --package-path apps/macos
```

All Go packages passed. Swift passed 194 XCTest cases with one intentional
visual preview/export skip and eight Swift Testing contracts, with zero
failures.

The source regression includes one directly blocked Agent, one dependent Main,
and one healthy independent Agent. It proves the direct Agent and dependent
remain blocked with zero calls while the healthy Agent receives one Attempt
and succeeds. A separate all-blocked regression proves terminal authority with
zero Attempts and no Runtime requirement.

## Open live gates

- Install and explicitly launch the build 53 Candidate under user approval.
- Execute a four-Agent Team across Anthropic, OpenAI, DeepSeek/Moonshot, and
  MiniMax Provider Accounts.
- Revoke, rate-limit, and timeout one account and prove only its Agent and
  declared dependents block.
- Attribute concurrency, limits, budget, tokens, cost, and errors to the exact
  Provider Account and Attempt.
- Exercise approved fallback without silent Provider, account, model, or
  Harness substitution.
- Complete Credential Vault CV6 restart, rotation, migration, and mixed-Team
  installed acceptance.

Normal dispatch uses Loom Credential Vault and short-lived account/revision
leases. Keychain remains only an explicit optional one-time migration source;
it is not in the Conversation or Agent hot path. No credential or Provider
request was used for this source increment. Installed Loom remains v0.5.2
build 39.

Build 52 is retained only as rejected historical package evidence. Static
product review found its Swift Store still required every Agent to be ready.
It was never installed or launched. Build 53 contains the corrected App entry
gate.
