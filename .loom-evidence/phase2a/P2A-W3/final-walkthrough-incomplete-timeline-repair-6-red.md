# P2A-W3 Incomplete Timeline Repair 6 RED

Date: 2026-08-03

Repair 5 Re-review proved that a loaded Timeline page is not necessarily a
complete history. The native Inspector must not interpret either an explicit
stream gap or `has_more=true` as an authoritative empty or complete result.

The negative fixtures were added first using strict decoded Timeline pages.
Both a gap page and an incomplete first page retained the existing
verification and Evidence rows instead of failing closed:

```text
swift test --package-path apps/macos --filter LocalProductExperienceViewTests/testMissionInspectorTabsExposeDistinctSafeReadOnlyContent

XCTAssertEqual failed: ["Verification recorded · Main · Attempt 1"] is not equal to []
XCTAssertTrue failed: changes must not present incomplete history as empty
XCTAssertEqual failed: ["Evidence 1 · Main · Attempt 1"] is not equal to []
XCTAssertTrue failed: evidence must not present incomplete history as empty
Executed 1 test, with 8 failures
```

No live root or product mutation occurred during RED.
