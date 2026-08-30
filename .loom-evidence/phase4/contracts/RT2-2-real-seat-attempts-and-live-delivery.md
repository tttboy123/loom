# RT2-2 - Real seat Attempts and visible discussion delivery

Status: `ACCEPTED / INSTALLED BUILD 211`

Date: 2026-08-26

Parent Goal: `Phase 4 - Governed multi-Agent RoundTable deliberation`

## User outcome

Starting a RoundTable round dispatches the configured Agents. Each seat shows
when it starts, what route is frozen, when new visible output arrives and
whether it succeeded or failed. The user no longer clicks a canned
propose/relay/ack sequence to simulate an Agent discussion.

## Authority and privacy

- Every seat execution is one immutable Agent Attempt bound to one Session,
  Round, Seat membership revision, `FrozenSeatBinding` digest and Role Context
  Capsule digest.
- The Journal records lifecycle metadata, content digests, encrypted payload
  references, safe failure codes and Incident IDs. It does not record Prompt,
  model output text, Provider response bodies, credentials or Authorization.
- Visible output uses the existing tentative delivery pattern while running.
  Accepted terminal text is read through an encrypted Loom-owned payload store.
- A Provider or Harness failure terminates only that seat Attempt. Other seats
  continue and the Round remains inspectable.
- Model output is a Candidate. It cannot conclude the RoundTable, mutate the
  Mission or become an AlignmentSummary without Moderator synthesis and user
  confirmation in RT2-4.

## Attempt projection

Each projected Attempt includes:

- `attempt_id`, `round_id`, `seat_id`, `attempt_number`
- `execution_team_id`, `work_item_id`, `run_id`, `claim_generation`
- `runtime_instance_id`, `agent_instance_id`, `execution_binding_digest`
- `membership_revision`, `seat_binding_digest`
- `context_capsule_digest`, `payload_reference`
- `status`: `running`, `succeeded`, `failed`, `cancelled`
- `output_digest` only after terminal payload commit
- safe `incident_id`, `failure_code`, `failure_stage`, `retryable`
- `started_at`, `completed_at`

## Source verification

1. Two seats in one round start distinct Attempts with distinct binding and
   Capsule digests where their routes or roles differ.
2. Attempt start rejects a legacy-unbound seat, wrong round, stale membership,
   binding substitution and duplicate active Attempt.
3. Terminal success requires an already committed encrypted payload reference
   and matching output digest; Journal payload contains no output text.
4. One failed Attempt preserves a peer running/succeeded Attempt and publishes
   an actionable seat-local failure.
5. Replay after daemon restart preserves exact Attempt lifecycle metadata.
6. Swift strictly decodes and renders per-seat state and incremental output.
7. Production dispatch invokes the existing governed Runtime Adapter and Role
   Context Capsule path rather than a new Provider client.
8. Installed App live gate shows output appearing before the final result.

Gates 1-7 are implemented and source verified. The production path compiles a
RoundTable round into an isolated synthetic Team execution, then invokes the
existing `TeamCoordinator -> Supervisor -> Attempt Loop -> Runtime Adapter`
chain. A controlled mixed outcome proves that one successful seat persists its
visible output through the encrypted Attempt Payload store while one failed
seat records only safe Incident metadata. The Swift workbench polls the
ephemeral authorized delivery projection once per second and can recover a
successful terminal body from encrypted storage after restart.

Gate 8 is accepted. Installed sessions retained incremental authorized output,
one successful seat's encrypted terminal payload and an independently failed
peer. Build 208 additionally proved that a Loom Native seat consumed a live
Steer into a second governed model Step while its peer continued independently.
The Journal retained only lifecycle metadata and digests.

## Compatibility

The legacy manual ledger remains read compatible and clearly labelled as a
non-executing legacy session. New Mission-linked sessions use Agent Attempts;
the UI does not present canned text as model output.
