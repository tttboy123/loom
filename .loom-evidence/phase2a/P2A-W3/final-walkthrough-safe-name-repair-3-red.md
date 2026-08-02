# P2A-W3 Final Walkthrough Safe-Name Repair 3 RED

Date: 2026-08-03

root003 proved the actual producer defect: `buildLocalProductMission` assigned
`team.TeamDefinitionID` to the user-facing Mission title. Client-only equality
checks against Mission and Team instance IDs cannot robustly classify a third,
unpublished identifier namespace.

Two causal API regressions were added before production changes:

```text
go test -count=1 ./internal/api -run 'TestLocalProductMissionFacadeUsesRealProjectionAndPreservesStaleView|TestLocalProductReadServicePublishesBoundedSnapshotAndPreservesStaleView'
```

Expected failures:

```text
Title:"team.delivery"
snapshot.Missions[0].Title != "Release Crew"
```

The first fixture has no matching TeamDefinition and requires the
non-identifying `Saved team` fallback. The second has an exact version/scope/
digest-matched TeamDefinition named `Release Crew` and requires that name.

The repair changes only the rebuildable LocalProduct Mission read-model title:
matched saved Teams reuse the existing validated Team display-name resolver;
historical execution-only rows use `Historical mission`. Journal facts and
all internal IDs remain unchanged.

