# RT2-1 - Mission-linked RoundTable and frozen Agent seats

Status: `ACCEPTED / INSTALLED BUILD 211`

Date: 2026-08-26

Parent Goal: `Phase 4 - Governed multi-Agent RoundTable deliberation`

## User outcome

A RoundTable is created from one visible Mission and keeps the Conversation,
Mission and Team relationship after App or daemon restart. The user can drag
two to six configured Team Agents into named seats. Before the first round
opens, Loom resolves and freezes each seat's exact non-secret execution route.
The RoundTable no longer relies on a mutable Swift catalog label to explain who
participated.

## Authority model

- The Event Journal remains the only RoundTable state authority.
- A RoundTable Session references one Conversation, Mission and saved Team.
- A participant Seat references one Team role and freezes one validated
  `runtime.FrozenExecutionBinding` digest plus its non-secret projection.
- The frozen projection includes Harness Adapter, Provider, Provider Account,
  credential reference and revision, model, endpoint fingerprint, limits and
  capabilities. Secret material never enters the Session, Seat, Prompt,
  Journal, Evidence, diagnostics or IPC response.
- Opening the first Round freezes membership. Replacing a route or Agent after
  that point creates a new membership revision and a new Round/Attempt; it does
  not mutate an earlier binding.
- Existing schema-v1 RoundTable streams remain readable as legacy ledger-only
  sessions. They cannot start Agent execution until explicitly upgraded through
  a reviewed binding operation.

## Contract shape

`SessionContext`

- `conversation_id`
- `mission_id`
- `team_id`
- `team_version`
- `workspace_id`

`FrozenSeatBinding`

- `agent_definition_id`
- `team_role_kind`
- `runtime_profile_id`
- `execution_binding`
- `execution_binding.binding_digest`
- `membership_revision`

The canonical digest binds the Session identity, Team identity/version, Seat
identity/revision and complete validated Execution Binding. IDs and digests are
bounded and control-character free.

## Compatibility and failure behavior

- Create and add-seat requests fail closed when Mission, Team or binding
  authority is missing, stale or mismatched.
- A duplicate command with the identical canonical payload is idempotent.
- Reusing a Seat ID with a different binding is a conflict.
- One invalid Provider Account blocks only its Seat during preflight; it does
  not mark the entire RoundTable or Team offline.
- Strict IPC decoding rejects unknown fields. Public errors retain an Incident
  ID, stage and actionable non-secret recovery instruction.

## Source gates

1. Create a Mission-linked Session and replay the exact context after restart.
2. Add two Seats with different Provider Accounts and models; replay preserves
   both exact bindings and digests.
3. Reject stale Team version, mismatched role/profile, changed credential
   revision, changed endpoint fingerprint and binding digest substitution.
4. Reject a second binding for the same active membership revision without
   changing the Journal head.
5. Read an existing schema-v1 fixture unchanged and report `legacy_unbound`.
6. Swift strict decoding renders Harness, Provider Account, Model and binding
   status from daemon authority rather than recomputing them from setup state.
7. Drag/Add uses one reviewed request shape and remains keyboard accessible.
8. No credential, Authorization header, Prompt or Provider body appears in
   Journal events, diagnostics or test artifacts.

All eight source gates pass on 2026-08-26. The production daemon resolves
client-selected Mission and Team role identities against the authoritative
projection, freezes the exact runtime binding, rejects client-authored Provider
or credential payloads, and replays the same context and binding after a new
controller/authority instance is constructed over the Journal. The Mission
Room exposes the linked RoundTable entry and Swift renders the frozen route.

Installed Build 208 cloned an existing Mission-linked Session through the real
UDS using only the source Mission/Team link and role selections. The daemon
resolved both current seat bindings, created distinct Agent/Run/Segment/Capsule
identities, and retained the same Mission relationship after daemon restart.
No Provider, credential, model or endpoint fields were accepted from the live
journey client.

## Exit gate

RT2-1 is complete only when domain, projection, strict IPC, Swift model/UI,
restart replay and installed App checks pass. Passing the legacy nine-step
ledger journey alone is insufficient.
