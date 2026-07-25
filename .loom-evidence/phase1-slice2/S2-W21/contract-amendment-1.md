# S2-W21 Contract Amendment 1

- Amendment: `1`
- Trigger: pre-RED source-contract reconciliation
- Previous contract SHA-256:
  `1a0063b957669edc0242f91341595fa08de880ce6c8359385410a5281a0a1df1`
- Product changes made before amendment: none

## Conflict

The authoritative accepted `runtime.NewRuntimeInstance` contract does not
require `ExecutableVersion` to be nonempty. Accepted S2-W2 discovery and S2-W20
Event writing therefore also permit an empty executable version. The S2-W21
illustrative JSON incorrectly labeled that field nonempty.

## Amendment

`ExecutableVersion` remains an exact projected string and may be empty. All
other RuntimeInstance validation remains delegated to
`runtime.NewRuntimeInstance`, with canonical-form equality required before
projection.

No ownership, read-model field, Event schema, rediscovery rule, status
semantics, dependency, execution surface, or external action changes.

VERDICT: AMENDMENT_FROZEN
