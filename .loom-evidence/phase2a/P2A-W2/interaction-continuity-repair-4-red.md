# P2A-W2 Interaction Continuity Repair 4 RED

**Status**: `RED — SAME-AGENT ALTERNATE PROFILE DRIFT REPRODUCED`

Fresh independent Repair 3 Re-review returned `FAIL` with one P1 and one P2.
Role-option selected-state matching used only kind plus AgentDefinition. The
catalog permits the same AgentDefinition to appear in multiple options bound to
different Runtime profiles/instances.

Native and TUI RED fixtures use one AgentDefinition with two exact role-option
bindings. Both clients incorrectly mark both options selected and copy the
current option's model/Provider/auth onto the alternate profile.

Repair 4 must match the complete available binding tuple:

```text
kind
+ agent_definition_id
+ runtime_profile_id
+ runtime_instance_id
```

Exact role-option ID remains the edit command value. No protocol, catalog,
authority, Amendment, WorkItem or live action is added.
