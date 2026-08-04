# SF-W1 Schema Amendment 2 (bounded, frozen)

Date: `2026-08-04`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the only and existing `SF-W1`. No SF-W4, no thin WorkItem, no new
authority, no second database, no external action.

## 1. Reason

`SF-EXIT-CONTRACT.md` §5 freezes the event name `SuccessorProposalCreated`
but freezes no corresponding record, while the SF-W1 Gap Proposal deliverable
requires a compiled successor proposal that creates no WorkItem side effect.
The §5 payload rule ("each named Event's payload is the corresponding
record's frozen fields plus `correlation_id`/`evidence_digests`") needs that
record to be field-exact.

## 2. Supersession (bounded)

This Amendment adds one field-exact `SuccessorProposal` record to §5. No
existing record, field, event name, payload rule, authority boundary, RED,
journey, or acceptance item changes.

## 3. `SuccessorProposal` record (new, frozen)

```json
{
  "successor_proposal_id": "uuid-v4",
  "gap_id": "uuid-v4",
  "job_submission": {
    "source": "gap_proposal",
    "dag_node_id": "string",
    "dependencies": ["dag_node_id"],
    "owned_paths": ["relative-path"],
    "mutex_keys": ["string"],
    "resource_claims": {"runtime": "string", "slots": 1, "model": "string"},
    "max_attempts": 3,
    "capability_kind": "string",
    "exit_conditions": ["string"],
    "verification_strategy": "string",
    "integration_strategy": "string",
    "protected_authority_paths": ["string"]
  },
  "source_evidence_digests": ["sha256"],
  "disposition": "propose_successor",
  "created_at": "rfc3339",
  "correlation_id": "journey-or-source-id"
}
```

`SuccessorProposalCreated` payload = the record fields above plus
`evidence_digests` (the gap's source digest binding); stream identity =
`gap_id`; idempotency key = `successor_proposal_id`. Compilation is
read-only — it creates no QueueJob, WorkItem, Run, or side effect; only a
later `AdmissionDecisionRecorded` may create the successor WorkItem under
SF-W1 admission rules.

## 4. Not claimed

No new authority, no P3A record schema change, no second writer/database,
no push/merge/network/migration, no change to any RED/journey/acceptance
item. Stale or unauthorized evidence still cannot create a successor
(SF-W1 RED #6).

## 5. Independent review acceptance

PASS requires a fresh read-only Reviewer to prove: bounded supersession;
field-exact new record; read-only compilation with no WorkItem side effect;
no RED/journey/acceptance/authority change.

VERDICT: `FROZEN — PENDING REVIEW`
