# P2A-W3 Final Walkthrough Safe-Name Repair RED

Date: 2026-08-03

Two causal RED checks were added before production changes.

Go command:

```text
go test -count=1 ./internal/tui -run 'TestMissionWorkbenchStartsOnBoardAndUsesExactNavigation|TestModelPresentsRecentWorkWithoutRawInternalIdentifiers'
```

Expected failure:

```text
Mission Detail · mission/team-1
Compare view missing "Compare"
```

The first failure proved that the primary Board exposed `mission_id`. The
second proved that History/Compare was not reachable as a real TUI screen and
its existing renderer had no accepted safe-name proof.

Swift command:

```text
swift test --package-path apps/macos --filter LocalProductExperienceViewTests/testMissionRailRendersTeamsAttentionAndHistoryComparePages
```

Expected compile failure:

```text
cannot find 'missionDisplayTitle' in scope
```

The missing helper is the exact presentation boundary needed to distinguish a
real user-facing title from a title equal to `mission_id` or
`team_instance_id`, then resolve the confirmed Team display name without
changing authoritative records.

