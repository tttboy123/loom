# P2A-W3 Final Walkthrough Safe-Name Repair 2 RED

Date: 2026-08-03

The bounded Repair 2 first changed the Board fixture to the exact incident
shape: `Mission.Title == Mission.MissionID`, with a confirmed Team display name
available. The existing presentation helper resolved it to `Release Team` and
the test forbids both Mission and Node internal IDs.

The new causal setup Runtime test then ran before production changes:

```text
go test -count=1 ./internal/tui -run 'TestMissionWorkbenchStartsOnBoardAndUsesExactNavigation|TestTeamBuilderRuntimeDisplayNameFailsClosed'
```

Expected failure:

```text
Runtime · runtime-internal-setup · online · 0.82.1
```

This proves the remaining P1 was isolated to the setup-backed Team Builder
Runtime row rather than the snapshot-backed Runtime resolver.

