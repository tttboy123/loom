# P2A-W3 Timeline Lineage Repair Verification

Date: 2026-08-03

## Closed behavior

`resolveDeliveryLineage` still derives authority from the exact stream plus the
versioned GlobalReadView. If an authoritative Event also carries
`logical_node_id`, `attempt_number`, or both, every present field must match
that derived lineage exactly. The reader now accepts a legitimate subset
instead of requiring absent redundant metadata.

It remains fail-closed for:

- missing authoritative lineage;
- logical-node mismatch;
- attempt mismatch;
- zero attempt;
- malformed attempt.

No Event schema, Journal fact, writer, command identity, CAS, Grant,
generation, Evidence, or Projection authority changed.

## Focused checks

PASS:

```text
go test -count=1 -run 'TestDeliveryLineageFieldsAcceptOnlyIndependentlyCorroboratedPartialMetadata|TestAuthoritativeSafeFieldsRejectAmbiguityAndIgnoreRawKeys|TestReadPageReturnsJournalAuthoritativeTimelineAndReconnectsWithoutDuplicate' ./internal/api
```

The current source was also built as an isolated CLI and read the unchanged
root004 SQLite successfully. The first bounded page contained 22 records,
including a journal-authoritative `verification_rejected` record bound to
logical node `main`, attempt 1. The returned board was `succeeded`, current
attempt 2, with `verification_failed` Attention. No state write occurred.

## Complete matrix

PASS:

```text
go test -count=1 -p=1 ./...
go test -count=1 -race -p=1 ./...
go vet ./...
swift test --package-path apps/macos
swift build -c release --package-path apps/macos
git diff --check
```

Swift executed 63 XCTest tests with one visual-export-only skip and all four
Swift Testing tests. Every executed test passed.

