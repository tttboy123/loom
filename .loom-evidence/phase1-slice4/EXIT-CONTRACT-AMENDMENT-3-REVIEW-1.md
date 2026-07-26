# Slice 4 Exit Contract Amendment 3 / S4-W3 Contract Review 1

Reviewer: fresh independent read-only Contract Reviewer

Date: 2026-07-26
Baseline: `6d3cbf2`

## Findings

1. The contract reconstructed a restart-safe verifier Candidate from Run
   terminal plus Store-returned receipt but still required Work authority to
   validate unavailable raw authorized Frame history. Amendment 3 intentionally
   does not reopen Evidence storage, so those requirements contradicted.
2. The contract named acceptance Events but did not freeze the typed errors,
   exact payload fields, deterministic Event-ID inputs, complete CAS head set,
   exact idempotency rules, or the persisted
   `verification_rejected → RecoveryPolicy → ScheduleTeamNodeRecovery`
   handoff required by the Slice 4 Exit Contract.

## Required bounded repair

- Validate the verifier through exact generation-fenced Run terminal
  status/reason and Store-returned receipt/summary only; do not require a raw
  Evidence read API.
- Freeze exact typed errors, payloads, Event identity inputs, CAS expectations,
  replay/idempotency behavior, and rejection-recovery handoff including
  recovery trigger, AcceptanceDecision digest, credits, `retry_at`, and
  restart behavior.

The Reviewer otherwise found the one-writer authority split, executor
`ready_for_review` limit, distinct verifier lineage, no second Projection/Team
writer, and no S4-W4/Phase 2/external activation direction sound.

VERDICT: FAIL
