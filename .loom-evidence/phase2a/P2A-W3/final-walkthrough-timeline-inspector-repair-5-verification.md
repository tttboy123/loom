# P2A-W3 Timeline and Inspector Repair 5 Verification

Date: 2026-08-03

## Closed behavior

The Timeline reader continues to derive lineage only from the exact
authoritative stream and the versioned GlobalReadView. Payload lineage remains
corroborating metadata, never authority. Presence and value are now evaluated
separately:

- an omitted `logical_node_id` or `attempt_number` is permitted;
- every present field must be valid and exactly match derived lineage;
- present-empty node, node mismatch, malformed/zero/mismatched attempt and
  unbound lineage are rejected with `ErrInvalidDeliveryRecord`.

The permanent regression passes a real attempt-only `WorkItemRejected` Event
through `mapAuthoritativeRecords`, which includes duplicate-key-safe field
decoding and lineage resolution. It also exercises all malformed-presence
cases through that same path.

The native Inspector now renders four distinct read-only projections:

- Team: role, state and Attempt;
- Plan: safe step name, state and Attempt;
- Changes: journal-authoritative review, verification and acceptance facts;
- Evidence: journal-authoritative ordinal Evidence references.

Inspector output uses safe human names, never raw IDs or digests. Missing or
mismatched Timeline data renders unavailable instead of inventing an empty
authoritative result. The change adds no command, mutation, schema, writer,
CAS, Grant, generation, Evidence or Projection authority.

## Focused checks

PASS:

```text
go test -count=1 -run 'TestAuthoritativeMappingAcceptsAttemptOnlyRejectionAndRejectsMalformedPresence|TestDeliveryLineageFieldsAcceptOnlyIndependentlyCorroboratedPartialMetadata|TestAuthoritativeSafeFieldsRejectAmbiguityAndIgnoreRawKeys|TestReadPageReturnsJournalAuthoritativeTimelineAndReconnectsWithoutDuplicate' ./internal/api
swift test --package-path apps/macos --filter LocalProductExperienceViewTests/testMissionInspectorTabsExposeDistinctSafeReadOnlyContent
```

The current CLI also read the unchanged root004 SQLite successfully and
returned exactly one journal-authoritative `verification_rejected` record for
logical node `main`, attempt 1. Its board remained `succeeded` with current
attempt 2 and `source_mode=offline_recovery`. No state write occurred.

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

Swift executed 64 XCTest tests with one visual-export-only skip and all four
Swift Testing tests. Every executed test passed.

root005 remains unauthorized until fresh independent Implementation Re-review
returns PASS against the exact source lock.
