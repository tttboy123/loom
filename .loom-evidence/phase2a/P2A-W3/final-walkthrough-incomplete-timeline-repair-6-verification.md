# P2A-W3 Incomplete Timeline Repair 6 Verification

Date: 2026-08-03

Changes and Evidence now render records, or a truthful authoritative empty
message, only when all of the following are true:

- the Mission and Timeline both exist;
- the Timeline belongs to the exact Team;
- `gap == nil`;
- `has_more == false`.

Every other state renders an explicit unavailable message with zero rows. The
client does not invent completeness, discard a cursor, auto-page, mutate
Journal state or form a second authority. Team and Plan remain derived from the
bounded Mission snapshot and are unaffected.

Focused PASS:

```text
swift test --package-path apps/macos --filter LocalProductExperienceViewTests/testMissionInspectorTabsExposeDistinctSafeReadOnlyContent
```

The strict fixture covers both `gap != nil` and `has_more=true` for Changes and
Evidence, in addition to complete-history positive rows, nil Timeline and safe
text assertions.

Complete matrix PASS:

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

root005 remains unauthorized until a fresh independent Implementation
Re-review returns PASS against the exact Repair 6 source lock.
