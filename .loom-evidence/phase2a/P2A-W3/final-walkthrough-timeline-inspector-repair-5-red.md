# P2A-W3 Timeline and Inspector Repair 5 RED

Date: 2026-08-03

Repair 4's independent Review returned `FAIL` for two bounded reasons:

1. a present but empty `logical_node_id` could be interpreted as omission;
2. the durable regression stopped at a helper instead of traversing the real
   safe-field, lineage-resolution and authoritative-mapper path.

The root004 native walkthrough also proved a separate read-only presentation
defect: selecting Plan, Changes or Evidence changed the selected tab while the
Inspector continued to render Team Pulse.

## Authoritative mapper RED

The full-path fixture was added before the production seam was narrowed. It
failed to compile because the mapper still required the concrete
`projection.GlobalReadView`, preventing the bounded lineage fixture from
reaching `safeEventFields → resolveDeliveryLineage → mapAuthoritativeRecords`:

```text
go test -count=1 -run 'TestAuthoritativeMappingAcceptsAttemptOnlyRejectionAndRejectsMalformedPresence' ./internal/api

cannot use selected (variable of struct type apiTestTimelineLineageView) as projection.GlobalReadView value in argument to mapAuthoritativeRecords
FAIL loom-pi-rebuild/internal/api [build failed]
```

## Native Inspector RED

The distinct-tab fixture was added before the read-only section builder. It
failed to compile because no tab-specific presentation boundary existed:

```text
swift test --package-path apps/macos --filter LocalProductExperienceViewTests/testMissionInspectorTabsExposeDistinctSafeReadOnlyContent

cannot find 'missionInspectorSection' in scope
error: fatalError
```

No live root was created and no state-changing product action was performed
during RED.
