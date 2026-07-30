# P2A-W2 Interaction Continuity Repair 2 RED

**Status**: `RED — ROLE-OPTION CONTINUITY FINDING REPRODUCED`

Fresh independent Repair 1 Implementation Re-review returned `FAIL` with one
P1. The three Implementation Review 1 findings are closed, but the complete
contract still lacks role-option Provider/model presentation and the existing
`builder_edit main_role/subagent_role` confirmation path.

Only frozen owned native and TUI tests changed before Repair 2 implementation.

```text
swift test --package-path apps/macos --filter InteractionContinuityTests
```

Exited `1` because `LocalProductRoleOptionChoice` and the Store's accepted
role-edit fields do not exist.

```text
go test ./internal/tui -run 'TestTeamBuilderPreflightShowsBoundProviderModelAndLimits$' -count=1
```

Exited `1` because the confirmation surface has no `Main role choices`, no
`m change Main role` action, and sends no `builder_edit main_role` command.

Repair 2 remains inside the same complete frozen P2A-W2 owned boundary. It adds
no protocol, application-service field, authority, Amendment or WorkItem and
performs no live action.
