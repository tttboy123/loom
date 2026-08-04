# v0.4.0 Gate 1 Contract Repair 1 (frozen)

Date: `2026-08-04`

Status: `ACCEPTED` — GATE 1 CONTRACT RE-REVIEW 2 PASS
(`GATE1-CONTRACT-REVIEW-2.md`, P0=P1=P2=0)

Closes the seven P1 findings and the implementability P2 items of
`GATE1-CONTRACT-REVIEW-1.md` (FAIL, P0=0 P1=7 P2=10) by amending
`SF-EXIT-CONTRACT.md` and `SF-WORKITEMS.md`. ADR-0014 is unchanged in
substance (its single-authority architecture and non-decisions passed).

## P1 closures

1. **F1 lane/status state machine** — `SF-EXIT-CONTRACT.md` §1 now freezes a
   legal lane/status transition matrix (source status, event, guard, target
   status; terminal statuses; `human_required` resume rule) and states that
   `QueueJob.lane` uses exactly the seven §1 lane values
   (`admission|development|repair|test|review|integration|human`; `admission`
   is the Scheduler's compile/validate lane until the `QueueJobAdmitted`
   transition moves the Job to its next actor's lane) — removing the §1/§5
   contradiction.
2. **F2 Event payload schemas** — `SF-EXIT-CONTRACT.md` §5 now freezes the
   per-event payload rule: each named Event's payload is the corresponding
   record's frozen fields plus `correlation_id`/`evidence_digests`, with
   frozen stream identity (`job_id` lineage) and idempotency key per event
   (`attempt_id`, `candidate_id`, `integration_id`, `gap_id`, `canary_id`,
   `release_id`).
3. **F3 Gap Proposal schema** — §5 now freezes the field-exact `GapProposal`
   record (source type/IDs/digests, affected capability, observed/expected
   behavior, user impact, confidence, uncertainty, reproducibility, privacy
   classification, proposed scope, owned-path/resource claims, risk class,
   rollback idea, duplicate/supersession, disposition) with one `gap_id` per
   digest-bound source and stale/unauthorized evidence barred from creating a
   successor.
4. **F4 crash seam** — the frozen `Attempt` record now includes
   `crash_seam` (`before_cas|after_cas|null`) and
   `crash_effect_cardinality` (`zero_effects|single_effect|null`), and the
   §1 transition matrix records the seam truthfully per the P3A S5/S6 pattern.
5. **F5 dual-Scheduler single winner** — §10 RED now includes item 21 (two
   Schedulers competing to dispatch the same ready Job ⇒ exactly one CAS
   winner, loser rejected with zero side effects) and §16 minimum acceptance
   now includes item 13 (same assertion).
6. **F6 TUI ownership for SF-W1/SF-W2** — `SF-WORKITEMS.md` allowlists now
   include the exact TUI files needed for queue and worker visibility
   (`internal/tui/queue.go` + test in SF-W1; `internal/tui/workers.go` + test
   in SF-W2), so each WorkItem's mandatory journey is observable in the real
   PTY TUI.
7. **F7 restart/reconnect/recovery in every journey** — each WorkItem journey
   (and the canary) now includes an explicit daemon-restart mid-journey
   checkpoint with both clients observing the rebuilt state through their own
   production reads and no duplicate facts; `SF-EXIT-CONTRACT.md` §13 makes
   this mandatory for every WorkItem.

## P2 closures (implementability set)

- **P2-1** shared-path carve-out for `internal/localipc/protocol.go` /
  `protocol_test.go` (additive only; existing semantics/wire unchanged) —
  `SF-WORKITEMS.md` header.
- **P2-2** queue projection is a separate rebuildable read model in
  `internal/projection/queue.go`; the P3A core `internal/projection/
  projection.go` is not reopened — `SF-WORKITEMS.md` header.
- **P2-3** Decomposition Compiler implementation owned by SF-W1
  (`admission.go`, `conflict_arbiter.go`, `gap_proposal.go`) —
  `SF-EXIT-CONTRACT.md` §8.
- **P2-4** Scheduler implemented by SF-W1 admission + SF-W2 `router.go`, no
  separate scheduler package/authority — `SF-WORKITEMS.md` header.
- **P2-5** cross-WorkItem `protocol.go` sequencing: additive against the
  prior accepted commit; the Conflict Arbiter serializes it as a shared owned
  path so parallel Candidates never both include it — `SF-WORKITEMS.md`
  header.
- **P2-6** budget/network/privacy admission deferred to the `v0.3.x`
  planning boundary; v0.4.0 admission covers owned paths/mutexes/authority/
  resources only — `SF-EXIT-CONTRACT.md` §4.
- **P2-7** docs-only Gate 1 governance commit frozen (ADR-0014 + Exit
  Contract + WorkItems + Gate 0 audit + this Repair + CURRENT/README records)
  as one atomic local commit after Contract Review PASS and before any RED —
  `SF-EXIT-CONTRACT.md` §14.
- **P2-8** Timeline/Attention/streaming content guardrail (never hidden
  reasoning, credentials, raw Grants, or per-token output) — `SF-EXIT-CONTRACT.md`
  §9.
- **P2-9** pool limits operator-configurable with the no-oversell invariant
  as the hard bound — `SF-EXIT-CONTRACT.md` §4.
- **P2-10** `PRODUCT-PLAN.md`/`TECH-PLAN.md` are the architecture reference;
  the `v0.4.0` release label and self-evolution boundary come from the frozen
  queued inputs — `SF-EXIT-CONTRACT.md` §8.

## Re-review scope

A fresh independent read-only Contract Re-review must verify each closure
against the amended files and return `P0=P1=P2=0` with
Product/Authority PASS and Operational/Trace Governance PASS before any RED
or product write.

VERDICT: `ACCEPTED` — GATE 1 CONTRACT RE-REVIEW 2 PASS
