# P2A-W2 Interaction Continuity Implementation Review 1

**Reviewer**: fresh independent read-only Reviewer
**Verdict**: `FAIL`

## Findings

### P1 — Provider/model and preflight review are incomplete

The Builder question surface displayed only the service's responsibility label,
and the confirmation surface displayed purpose/cost without the already-bound
role Runtime, Provider, model, auth mode, permissions, compatibility and budget.
The TUI had the same gap. The underlying strict preview already contains these
fields, so this is a presentation omission and does not require a protocol or
authority expansion.

### P2 — task search/filter is missing

The native left task column and TUI Tasks view had no bounded search/filter.
The repair must retain the selected task when it does not match the query.

### P2 — exact-remove replacement interleaving is unproved

`removeExactFile` checked identity and then removed the path with no test seam
for a replacement inserted after the initial check. The existing replacement
test covered only a replacement installed before close.

## Reviewer verification

The Reviewer independently passed:

```text
go test ./internal/localipc ./internal/tui ./cmd/loomd -run 'Test(PrepareSocket|Server|ProductDaemonReclaims|InteractionContinuity|TaskSelection)' -count=1
swift test --package-path apps/macos --filter InteractionContinuityTests
swift test --package-path apps/macos --filter LocalProductStoreTests
git diff --check
```

No P0 finding was reported. Excluded dirty state was explicitly identified and
must remain outside the Candidate.

## Gate

Implementation Review 1 is `FAIL`. Repair 1 remains inside this same complete
P2A-W2 contract. It creates no Amendment and no W4.
