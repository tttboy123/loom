# P2B-W1 Independent Whole-Candidate Review

Date: 2026-08-03

Reviewer: independent read-only whole-Candidate Reviewer

Verdict: `PASS`

- P0: 0
- P1: 0
- P2: 0

Exact staging and one atomic local commit are authorized, provided the index is
restricted to `exact-candidate-boundary.md` and preserves every explicit
exclusion.

## Reproduced boundary and evidence

The Reviewer reproduced repository identity at branch
`codex/loom-platform-slice2`, baseline HEAD
`7a27149b31db7ffeefefb86c449ba448c3250fca`, the frozen one-W1 contract,
explicit-confirmation-only v1, one Journal/authority/projection path, exact
Candidate ownership and the absence of P2B-W2 or wrapper-only splits.

It reproduced:

- original source lock SHA-256
  `411593c724dfc78424dbaf99c4936b2367b92dd65b402b6841220d5e738ef97c`;
- supplemental presentation lock SHA-256
  `8e19132d75d4ae10da25f07d11f1e211a4b12ee40eee764961adf610ea75e970`;
- both current supplemental Swift hashes;
- retained canary SQLite and manifest hashes;
- immutable SQLite integrity `ok`, 296 Events, zero duplicate Event IDs and
  idempotency keys, and exactly one ContextPacket, continuation, parent effect
  and typed decision;
- screenshot and accessibility transcript hashes.

## Deterministic checks

The independent Reviewer ran and passed the following bounded deterministic
checks. Controller exact-index recheck clarifies the diff command as the
product/source check shown here:

```text
git diff --check -- '*.go' '*.swift' '*.json'
go test -count=1 ./cmd/loomd -run '^TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC$'
go test -count=1 ./cmd/loomd -run 'SideTask|ProductMissionExecutionVerticalLoopback|Handoff|StrictSwift.*Side-task|Restart'
go test -count=1 ./internal/work ./internal/projection ./internal/app ./internal/api ./internal/localipc ./internal/tui -run 'SideTask|Handoff|ContextPacket|ParentContinuation|Projection|Strict|TUI|Proposal|Decision|Restart'
go test -count=1 -race ./internal/work ./internal/projection ./internal/app ./cmd/loomd -run 'SideTask|ProductMissionExecutionVerticalLoopback|ParentContinuation|ParentHandoff|Projection|Handoff'
go vet ./internal/work ./internal/projection ./internal/app ./internal/api ./internal/localipc ./internal/tui ./cmd/loomd
```

A broader concurrent package run encountered one older Pi metadata timeout
test. Its isolated reproduction passed, and the P2B-focused `cmd/loomd` run
passed; the Reviewer therefore did not classify it as a P2B finding.

Full `git diff --check` additionally reports the already reviewed CommonMark
hard-line-break bytes in governance Markdown. Those bytes are intentional and
locked; the exact-index Go, Swift and JSON check is clean. This clarification
does not change product bytes, the Reviewer verdict or any P2B authority claim.

No authority canary, daemon, Provider, network or other live path was run. The
Reviewer did not edit, stage or commit.
