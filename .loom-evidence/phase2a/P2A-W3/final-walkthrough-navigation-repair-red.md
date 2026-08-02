# P2A-W3 Final Walkthrough Navigation Repair RED

Date: 2026-08-03

The causal RED added
`testWorkspaceRailRoutesAreRealAndPreserveMissionContinuity` before product
implementation.

Command:

```text
swift test --package-path apps/macos --filter MissionOrchestrationTests/testWorkspaceRailRoutesAreRealAndPreserveMissionContinuity
```

Expected causal failure occurred at compile time:

- `MissionWorkspaceState` had no `showTeams`, `showAttention`, or
  `showLibrary` method;
- `MissionWorkbenchRoute` had no `teams`, `attention`, or `library` case.

Direct source inspection also confirmed the visible `Teams`, `Needs You`, and
`Library` controls invoked `{}` and therefore could not reach the frozen
walkthrough surfaces.

