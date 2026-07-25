# S3-W3 Candidate Deliverable — AgentGrant Authority

- Baseline: `5517a06`
- Contract SHA256:
  `73ee0a5892a4f4a0fcbb3a595d0c7948e82d3212612b34fde95f1e7232207434`
- Amendment 1 SHA256:
  `48e7d166e4e32e4265bc33d7e79bd5d030f4642abe653f4964ed93a888a9ac0c`
- Candidate state: `ready_for_review`
- Date: `2026-07-26`

## Delivered boundary

The Candidate implements the complete frozen local AgentGrant authority:

- exact 48-byte random one-time token issuance with UUID-v4 Grant identity;
- SHA-256 hash-only Event persistence and constant-time token comparison;
- exact Run, generation, operation, ResourceRef, WorkItem, AgentInstance, and
  Harness binding;
- append-only, linearized authorization facts with per-Run RequestID
  uniqueness across Grant generations;
- explicit revocation, automatic expiry replacement, and generation rotation;
- Run-head plus Grant-head CAS for issuance and same-stream one-winner
  concurrency;
- immutable snapshots and strict rebuildable projection validation.

The implementation keeps the accepted serial orders for Run-versus-Grant
races: a Run-first mutation makes a stale Issue conflict; a Grant-first Issue
may be followed by a legal Run mutation. Same Grant-stream contenders still
produce exactly one winner.

No Provider CredentialGrant, Broker, secret store, adapter, supervisor, Team
DAG, process, network, model, Runtime activation, or daemon capability is
added.

## TDD evidence

Mandatory RED was recorded before product implementation. All six frozen
markers occur exactly once, and the focused command failed only on missing
S3-W3 symbols and behavior.

During GREEN, a targeted regression test proved that nested `%#v` formatting
of `IssuedGrant` could expose the token through an ordinary struct field. The
Candidate now stores the token behind a private redacting box; `%v`, `%+v`,
`%#v`, JSON, Events, SQLite rows, projections, and returned errors do not
expose the raw token.

## Verification

All commands passed against the complete Candidate:

```text
go test ./internal/authorization ./internal/projection -count=1
go test -race ./internal/authorization ./internal/projection -count=30
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./internal/authorization -run '^$' \
  -fuzz '^FuzzGrantTokenAndReplayNeverPanic$' -fuzztime=5s
gofmt -d <all owned Go files>
git diff --check
```

The fuzz run completed 327,803 executions with 146 total interesting inputs.
Package coverage is 85.3% for `internal/authorization` and 82.8% for
`internal/projection`. Export, import, marker, scope, and token-leak scans also
passed. Journal trigger failure, closed-source replay, fabricated historical
references, generation replacement, and both accepted Run/Grant race orders
have explicit tests.

## Security review

The bounded STRIDE review found and closed the nested-format disclosure above:

- spoofing is constrained by secret, Grant ID, generation, and exact binding;
- tampering is constrained by strict Event schemas, historical references, and
  dual-stream CAS;
- repudiation is constrained by persisted authorization facts and RequestID;
- disclosure is constrained by hash-only persistence, redaction, and generic
  errors;
- denial-of-service inputs are bounded by exact token, ID, operation, and time
  validation;
- elevation is constrained by the frozen operation allowlist and absence of
  Provider, daemon, network, and client-identity authority.

No dependency was added.

VERDICT: PASS
