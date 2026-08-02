# P2A-W3 Final Walkthrough Navigation Repair 2 RED

Date: 2026-08-03

Implementation Review 1 rejected source lock
`f1cde4c072fa47bd0c526d79851c91a94fd64a6fc40eccdc50770b3cd8bb36ba`
because partial, stale, and offline-preserved snapshots could falsely render an
authoritative empty Attention state and authoritative History/Compare label.

The causal RED added
`testMissionWorkspaceSnapshotPresentationFailsClosed` before the repair.

Command:

```text
swift test --package-path apps/macos --filter LocalProductExperienceViewTests/testMissionWorkspaceSnapshotPresentationFailsClosed
```

It failed because `missionSnapshotPresentation` and its closed current,
partial, preserved, and unavailable states did not exist.

The repair makes currentness explicit. Only an online snapshot can produce
the definite `Nothing needs you` empty state. Partial and preserved views say
that items may be missing; offline/fatal data is labeled preserved rather than
current; a missing snapshot is unavailable. Teams in a preserved view no
longer claim that execution is available.

