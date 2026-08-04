# v0.4.0 Agent Scheduling Framework — Exit Contract (Gate 1 freeze candidate)

Status: `ACCEPTED` — combined Gate 1 Contract Re-review 2 PASS
(`GATE1-CONTRACT-REVIEW-2.md`, P0=P1=P2=0) after `SF-CONTRACT-REPAIR-1.md`;
no product code authority until RED is established for SF-W1

Date: `2026-08-04`

Baseline: P3A-W1 accepted commit `7d5f0b01` (recorded in `docs/CURRENT.md`).
This Contract reconciles `2026-08-01-interaction-handoff-auth-routing-scheduling-queued-input.md`
(`v0.4.0` release placement), `2026-08-04-governed-self-evolution-pipeline-queued-input.md`
(gap-to-successor closure), `PRODUCT-PLAN.md`, `TECH-PLAN.md`, the accepted
Phase 3A result, and ADR-0014 (single authority architecture). It creates no
second Scheduler, Journal, StateWriter, Projection, queue database, asset
authority, or integration writer; it creates no SF-W4 and no thin WorkItem.

## 1. Lanes

The lane set is exactly the seven lanes below. `QueueJob.lane` (frozen §5)
uses exactly these seven values (`admission|development|repair|test|review|
integration|human`). `admission` is the Scheduler's compile/validate
responsibility; a Job occupies the `admission` lane until it is `admitted`
(the `QueueJobAdmitted` transition), after which it moves to the lane of its
next actor.

| Lane | Owner/actor | Permitted | Forbidden |
|---|---|---|---|
| `admission` | Scheduler (event-driven) | compile+validate Jobs/Candidates from ready DAG; eligibility, conflict arbitration; gap-to-successor compilation | create execution authority, bypass human approval, expand scope |
| `development` | ephemeral Developer worker | modify frozen owned files in its isolated Candidate branch/worktree; fresh context; one claim; exit on completion | write target branch, mark own work accepted, claim multiple tasks |
| `repair` | ephemeral Repair worker (aged/weighted priority) | rework a Candidate from the exact recorded failure context | hidden infinite retry, silent scope change, self-review |
| `test` | deterministic Test Executor process | run frozen commands, persist result Evidence | reinterpret product acceptance, merge, act as Reviewer |
| `review` | independent read-only Reviewer | read Candidate, tests, Evidence, authority trace; verdict | modify product files or target-branch state, review own implementation |
| `integration` | single Integrator | apply one reviewed Candidate to the target branch via CAS/generation fencing | become a second Scheduler, expand scope, hide conflicts |
| `human` | Product Owner / user | explicit approvals, admission decisions, canary activation, adoption | automation that bypasses the lane |

## 2. Failure classification (exactly seven classes)

Every failed Attempt/Job/Candidate records exactly one class, by frozen rules,
not by "blame the Developer":

1. `product_defect` — the product under test fails per its own contract;
2. `test_defect` — the test/harness/evidence is wrong or flaky;
3. `infra_transient` — daemon/runtime/socket/process infrastructure failed
   without product behavior change;
4. `contract_defect` — the frozen contract, schema, or exit condition is
   ambiguous or self-contradictory;
5. `integration_conflict` — owned-path/mutex/resource or branch conflict at
   integration time;
6. `cross_layer` — a failure crossing adapter/sidecar/client layers whose
   classification is not determinable from one layer;
7. `human_required` — requires an authorized human decision (approval,
   admission, activation, adoption, or irreconcilable ambiguity).

Classification is recorded as an authoritative Journal fact bound to the
Attempt generation and the exact evidence digest. Misclassification is a
Contract defect: the Test lane never decides `human_required`; the Reviewer
never rewrites a classification; only the frozen classifier rules may change
it via a reviewed Contract Repair.

### Lane / status transition matrix (frozen)

Legal transitions are exactly the table below; every transition is an
authoritative Event with `correlation_id` and binds `evidence_digests`
where applicable. Terminal statuses are `integrated`, `rejected`,
`cancelled` and (for the human lane) `human_required`; a terminal status
never resumes. `human_required` resumes only through an explicit
Product Owner decision event.

| From status | Event | Guard | To status |
|---|---|---|---|
| `queued` | `QueueJobAdmitted` | eligibility + DAG-ready + no conflict (Decomposition Compiler) | `admitted` |
| `admitted` | `QueueJobDispatched` | capacity available; lease CAS | `dispatched` |
| `dispatched` | `AttemptClaimed` | worker claim CAS; lease issued | `in_progress` |
| `in_progress` | `AttemptResultRecorded` / `AttemptCrashed` / `StaleResultRejected` | generation + lease validity; crash seam recorded | `waiting_review` / `queued` (retry) / `dispatched` (reclaim) / `human_required` (exhaustion) |
| `waiting_review` | `ReviewVerdictRecorded` | independent read-only Reviewer | `ready_integrate` / `queued` (repair) / `rejected` |
| `ready_integrate` | `IntegrationStarted` | single Integrator CAS | `in_progress` (integration attempt) |
| `ready_integrate` / `in_progress` | `CandidateIntegrated` / `IntegrationRejected` | Integrator CAS + generation | `integrated` / `queued` (repair) / `rejected` |
| any active | `QueueJobCancelled` | authorized actor (human or admission policy) | `cancelled` |
| any active (undecidable) | `AdmissionDecisionRecorded` | human lane | `human_required` |

`Attempt` lifecycle: `claimed → running → succeeded|failed|crashed|
stale_rejected`; `crashed` records the frozen `crash_seam` field
(`before_cas`|`after_cas`) and the terminal effect cardinality
(`zero_effects`|`single_effect`).

## 3. Dispatch, idempotency, lease, and generation fencing

- Dispatch is at-least-once; the effect is once via idempotent CAS
  (`AppendBatchIfStreamHeads` or the equivalent Event CAS) keyed by
  `job_id` + `attempt_id`.
- Every worker/attempt holds a lease with an expiry; prepare-lease expiry
  produces exactly one new generation; `claim_generation` is monotonic.
- Late results, stale generations, expired leases, and duplicate
  `attempt_id`s are rejected with zero side effects and a recorded
  `stale_generation_rejected` fact.
- Crash reclamation is bounded: a crashed worker's lease expires; a
  Reconciler (in SF-W2) reclaims the Job with a new generation; the crash
  point (before/after CAS) is recorded truthfully (P3A S5/S6 seams as the
  pattern).
- Retry/backoff is bounded by frozen policy (max attempts, exponential
  backoff with cap, aging weight). There is no hidden infinite retry; retry
  exhaustion routes to `repair` or `human_required` per classification.

## 4. Fairness

- New Development does not starve Repair: Repair is prioritized with
  aging/weighted fairness (weight grows with wait time and class severity).
- The Scheduler is event-driven; it dispatches only DAG-ready, eligible,
  non-conflicting work. No Agent busy-polls.
- Capacity is never oversold: active workers per pool ≤ frozen limits; an
  admission request beyond capacity waits in the Queue Projection (no hidden
  spawn).
- Pool limits are operator-configurable through the frozen configuration
  surface; the no-oversell invariant (active workers ≤ configured pool
  capacity) is the hard bound enforced by the Admission/Conflict Arbiter.
- Budget, network and privacy admission compatibility are deferred to the
  `v0.3.x` planning boundary (the v0.4.0 queued-input Admission list covers
  owned paths, mutex keys, authority and resource claims only); the freeze
  does not claim budget/network/privacy admission for v0.4.0.

## 5. Schema (frozen)

The schemas below are field-exact and additive. Every named Event payload is
the corresponding record's frozen fields plus `correlation_id` and
`evidence_digests` (where the transition binds evidence), with stream
identity = `job_id` (Queue/Attempt/Candidate/Gap lineage) and idempotency
key = the transition's unique event identity (e.g. `attempt_id` for
`AttemptClaimed`/`AttemptResultRecorded`/`AttemptCrashed`,
`candidate_id` for `CandidateReadyForReview`/`ReviewVerdictRecorded`/
`CandidateReadyIntegrate`, `integration_id` for
`IntegrationStarted`/`CandidateIntegrated`/`IntegrationRejected`,
`gap_id` for `GapProposalCreated`/`SuccessorProposalCreated`,
`canary_id` for `CanaryStarted`/`CanaryCompleted`,
`release_id` for `ReleaseCandidatePublished`/`ReleaseAdoptedByLaterRun`/
`ReleaseRolledBack`). No new authority, database or second writer is created.

### `QueueJob` (Event/Projection record)

```json
{
  "job_id": "uuid-v4",
  "source": "gap_proposal|mission|user_queued|repair|test_result|review_verdict",
  "dag_node_id": "string",
  "dependencies": ["dag_node_id"],
  "status": "queued|admitted|dispatched|in_progress|waiting_review|ready_integrate|integrated|rejected|cancelled|human_required",
  "lane": "admission|development|repair|test|review|integration|human",
  "owned_paths": ["relative-path"],
  "mutex_keys": ["string"],
  "resource_claims": {"runtime": "string", "slots": 1, "model": "string"},
  "attempt_count": 0,
  "max_attempts": 3,
  "created_at": "rfc3339",
  "correlation_id": "journey-or-source-id"
}
```

### `Attempt` (Event/Projection record)

```json
{
  "attempt_id": "uuid-v4",
  "job_id": "uuid-v4",
  "generation": 1,
  "lease_expires_at": "rfc3339",
  "claim_cas": "string",
  "status": "claimed|running|succeeded|failed|crashed|stale_rejected",
  "crash_seam": "before_cas|after_cas|null",
  "crash_effect_cardinality": "zero_effects|single_effect|null",
  "failure_class": "product_defect|test_defect|infra_transient|contract_defect|integration_conflict|cross_layer|human_required|null",
  "evidence_digests": ["sha256"],
  "candidate_branch": "branch-name",
  "candidate_worktree": "path",
  "started_at": "rfc3339",
  "finished_at": "rfc3339|null"
}
```

### `Candidate`

```json
{
  "candidate_id": "uuid-v4",
  "job_id": "uuid-v4",
  "attempt_id": "uuid-v4",
  "branch": "codex/...",
  "worktree": "independent path",
  "base_commit": "sha256",
  "owned_files": ["relative-path"],
  "status": "in_progress|ready_for_review|review_pending|reviewed|ready_integrate|integrated|rejected",
  "review_verdict": "PASS|FAIL|null",
  "verification": {"focused": "PASS", "race": "PASS", "matrix": "PASS"},
  "source_lock_digest": "sha256"
}
```

### `GapProposal` (Event/Projection record, frozen)

```json
{
  "gap_id": "uuid-v4",
  "source": {
    "source_type": "run_failure|incomplete_run|user_report|review_observation",
    "source_ids": ["uuid-v4"],
    "source_digests": ["sha256"]
  },
  "affected_capability": "string",
  "observed_behavior": "string",
  "expected_behavior": "string",
  "user_impact": "string",
  "confidence": "high|medium|low",
  "uncertainty": "string|null",
  "reproducibility": "string",
  "privacy_classification": "none|pii|credential|other_sensitive",
  "proposed_scope": "string",
  "owned_path_claims": ["relative-path"],
  "resource_claims": {"runtime": "string", "slots": 1, "model": "string"},
  "risk_class": "low|medium|high",
  "rollback_idea": "string|null",
  "duplicate_of": "uuid-v4|null",
  "supersedes": ["uuid-v4"],
  "disposition": "observe|reject|merge_duplicate|human_required|propose_successor",
  "created_at": "rfc3339",
  "correlation_id": "journey-or-source-id"
}
```

`GapProposalCreated` payload = the record fields above; duplicate
observations of the same digest-bound gap converge on one `gap_id`; stale or
unauthorized evidence cannot create a successor.

### Event names (additive; existing Journal schema untouched)

`QueueJobCreated`, `QueueJobAdmitted`, `QueueJobDispatched`, `AttemptClaimed`,
`AttemptResultRecorded`, `AttemptCrashed`, `LeaseReclaimed`, `GenerationAdvanced`,
`StaleResultRejected`, `CandidateReadyForReview`, `ReviewVerdictRecorded`,
`CandidateReadyIntegrate`, `IntegrationStarted`, `CandidateIntegrated`,
`IntegrationRejected`, `QueueJobCancelled`, `GapProposalCreated`,
`SuccessorProposalCreated`, `AdmissionDecisionRecorded`, `CanaryStarted`,
`CanaryCompleted`, `ReleaseCandidatePublished`, `ReleaseAdoptedByLaterRun`,
`ReleaseRolledBack`. Every event carries `correlation_id` (journey/source id)
and binds `evidence_digests`.

## 6. Replay / restart semantics

- Queue state, leases, claims, Candidates, decisions, gap proposals, and
  adoption state are projections over the Event Journal and immutable
  artifacts; restart/replay reconstructs them (no hidden state authority).
- Replayed dispatch re-sends at-least-once with idempotent CAS; duplicate
  events/effects are impossible (P3A S8 slow-client + S5/S6 crash seams prove
  the pattern).
- A daemon restart preserves old views until the new projection is complete
  (P3A S7 pattern); `matches_journal=true` is required post-restart.

## 7. Authority boundaries

- The Journal (via its accepted AppendBatchIfStreamHeads API), the frozen
  validators, credential boundaries, and authoritative state writers are
  protected: no online Agent, Sidecar, model, or self-host cycle adds them to
  its owned scope or weakens their tests.
- The Scheduler is authoritative only for dispatch decisions; it never
  invents acceptance, retry semantics, or policy.
- The Integrator is the only target-branch writer; the Reviewer is read-only;
  the Test Executor is deterministic; the Developer cannot act as Reviewer.
- Side effects require policy + contract + grant + evidence lifecycle; agent
  and model output is always Proposal/Candidate, never execution authority.

## 8. Decomposition Compiler rules (frozen)

Implementation ownership (frozen): the Compiler is implemented by SF-W1
(`internal/queue/admission.go`, `internal/queue/conflict_arbiter.go`,
`internal/queue/gap_proposal.go`); there is no separate compiler package.
`PRODUCT-PLAN.md`/`TECH-PLAN.md` are the architecture reference; the `v0.4.0`
release label and the self-evolution gap-to-successor boundary come from the
frozen queued inputs (no release-label claim is imported from the plans).

The Compiler validates every Job/Candidate DAG before Admission:

1. **DAG cycles** — rejected with a typed error and a Journal fact.
2. **owned-path conflicts** — two ready jobs owning the same path are not
   dispatched in parallel.
3. **mutex conflicts** — same mutex key ⇒ serialized.
4. **authority conflicts** — a Candidate may not claim protected authority
   paths (root policy, validators, Journal writers, credential boundaries,
   Scheduler/Integrator authority) unless a separately reviewed
   human-governed contract permits it; otherwise fail closed.
5. **resource conflicts** — runtime/model/slot claims that exceed pool
   capacity or collide with active claims are not admitted (no oversell).
6. **vertical capability** — a Job must produce vertical product value;
   wrapper-only, adapter-only, coordinator-only, or visual-only
   decompositions are rejected (no thin WorkItem).
7. **exit conditions** — every Job has frozen exit conditions and
   verification strategy; absence is a compile error.
8. **integration strategy** — every Candidate names its target branch,
   base commit, and single Integrator; competing/stale integrations are
   rejected by CAS/generation fencing.

## 9. Observability and bounded-recovery ownership (frozen)

- **Bounded recovery** (retry/backoff policy, crash reclamation, stale
  generation rejection, no hidden infinite retry) is owned by **SF-W2**
  (Ephemeral Worker Pools + Lease/Reconciler + routing).
- **Streaming node output** capture (authorized, generation-bound,
  malformed/stale-frame rejection) and **Timeline/Attention** (projection +
  client push; never an authority) are owned by **SF-W3** (Single-writer
  Integration + controlled self-host canary).
- No single-point Amendment may silently move either ownership; changes
require a reviewed bounded Amendment.
- Timeline/Attention/streaming content never contains hidden reasoning,
  credentials, raw Grant tokens, or per-token output (frozen content
  guardrail; only authorized, generation-bound frames are published).

## 10. Mandatory RED coverage (each must exist as a failing test first)

1. admission/eligibility not met ⇒ not dispatched;
2. owned-path conflict ⇒ not parallel;
3. same mutex ⇒ not parallel;
4. lease expired / stale generation result ⇒ rejected, zero side effects;
5. crashed worker ⇒ reclaimed via lease with exactly one new generation;
6. late result after generation advance ⇒ rejected;
7. failure misclassified ⇒ classifier error surfaced;
8. Repair starvation ⇒ aging/weighted fairness unit test;
9. test failure routed to Repair, never auto-Integrated;
10. Reviewer attempts product write ⇒ denied;
11. two Integrators compete ⇒ one CAS winner, no duplicate effects;
12. Journal rebuild ⇒ queue/projection state identical after replay;
13. restart/replay ⇒ no duplicate Event/Evidence/effect;
14. canary double-run ⇒ blocked by idempotent CAS;
15. slow client ⇒ no hidden retry, one effect on reconnect;
16. projection failure ⇒ old view preserved, rebuild on restart;
17. cross-client journey missing ⇒ WorkItem cannot be marked complete;
18. gap-to-successor: unauthorized/stale evidence cannot create a successor;
19. duplicate gap observations converge on one `gap_id`;
20. protected-authority path modification ⇒ fail closed.
21. two Schedulers compete to dispatch the same ready Job ⇒ exactly one CAS
    winner, the loser's dispatch rejected with zero side effects.

## 11. Verification matrix

Per Candidate: focused tests + focused race; per risk gate and integration
checkpoint: full `go test ./...`, `go test -race ./...`, `go vet ./...`,
`go mod tidy` (no diff), `gofmt` (clean), Swift full + TSAN + Release,
replay/CAS/restart/security checks, Decomposition Compiler matrix, and the
cross-client journeys. Results are Evidence artifacts bound to the attempt.

## 12. Controlled self-host canary (per SF-W3, at the slice level)

- One canary Run, offline (`--offline`, `--no-approve`), locked runtime
  fixture, exact model/skill bindings.
- Proves: two independent Developer workers in parallel on two Candidates;
  Test Executor running before development completes; a classified failure
  entering Repair; crash reclamation; stale-generation rejection; single CAS
  winner; single Integrator; Journal rebuild; no duplicate Run/Evidence/effect;
  no capacity oversell; projection failure preserving the old view; streaming
  node output authorized/generation-bound; GUI+TUI journey with the frozen
  alternative evidence surface (screenshots, PTY transcript, IPC, daemon log,
  Journal/SQLite summary, artifact digests, cleanup proof).
- the canary includes an explicit restart/reconnect checkpoint: the daemon
  restarts mid-canary and both clients observe the rebuilt state with no
  duplicate Run/Evidence/effect.

## 13. Cross-client Exit Gate (per WorkItem)

Each WorkItem requires a real GUI + TUI vertical journey using the accepted
alternative-verification method (PO instruction 2026-08-04): production native
window launched with `--socket --journey-id`, real PTY TUI, production Swift
client over the real socket, unique journey ID threaded through client/IPC/
daemon/journal/projection/evidence/cleanup. The GUI evidence surface follows
the frozen GUI Evidence Surface Amendment (app launch reads, checkpoint-bound
screenshots, production Swift client records; no claim that window pixels
differ per journey). Either client missing ⇒ WorkItem is `PARTIAL`, never
complete. Evidence modes 0700/0600, manifest/result, timeline, daemon logs,
pre/post process/socket/lock/lease cleanup. Failures are not overwritten;
replacements need Re-review with a new journey root.
Every WorkItem journey must include at least one explicit
restart/reconnect/recovery checkpoint (daemon restart mid-journey with both
clients observing the rebuilt state through their own production reads), per
the Alternative Verification Amendment's retained reconnect/restart
requirement.

## 14. Exact staging and atomic commit

- Each WorkItem stages exactly its frozen owned files (a `source-lock.json`
  with `intentionally_excluded`, digest-verified against disk), excluding
  root docs, configuration, drafts, generated builds, and other slices'
  evidence.
- One atomic local commit per WorkItem; no push/merge; the final slice gets a
  whole-slice review. A Slice accepted in the authoritative repository is not
  recreated under the same WorkItem IDs elsewhere.
- A docs-only Gate 1 governance commit (ADR-0014 + Exit Contract +
  WorkItems + Gate 0 audit + SF-CONTRACT-REPAIR-1 + `docs/CURRENT.md`/
  `docs/adr/README.md` records) is frozen as one atomic local commit after
  the combined Contract Review PASS and before any RED.

## 15. Self-evolution reconciliation (folded, not a fourth boundary)

- SF-W1 (Queue State/Projection + Admission) also owns Gap Proposal,
  deduplication, successor compilation, eligibility, and conflict
  arbitration.
- SF-W2 (Worker Pools + routing) also owns isolated successor lineage, bounded
  repair, deterministic Test, read-only Review, and stale-result rejection.
- SF-W3 (Integration + canary + Timeline/Attention) also owns versioned
  release handoff, later-Run exact binding, rollback proof, and
  Attention/HumanRequired presentation.
- Protected authority changes remain human-governed; integration and adoption
  are separate authoritative transitions; running Teams/Attempts keep their
  original bindings; rollback never rewrites Run/Evidence history.

## 16. Minimum acceptance (all must be true at the slice end)

1. Two independent WorkItems truly developed in parallel by two independent
   workers on separate Candidates.
2. Test Executor runs independently before development completes.
3. Test failure accurately enters Repair by one of the seven classes.
4. Worker crash reclaimed by lease; state reconstructed from Journal.
5. Stale generation rejected with zero side effects.
6. Same mutex / same owned path never parallel.
7. Reviewer cannot write product/state; Integrator is the only target-branch
   writer.
8. Queue state fully rebuildable from the Journal (restart/replay identical).
9. Controlled canary has no duplicate Run/Evidence/effect; no capacity
   oversell; projection failure preserves the old view; crash recovery
   correct.
10. Streaming node output published only when authorized and generation-
    bound; unauthorized/stale/malformed frames not published.
11. GUI+TUI real journeys per WorkItem + dual Result (Product Result,
    Operational and Trace Behavior) PASS, P0=P1=P2=0.
12. Gap-to-successor minimums (from the queued input): one authorized
    failed/incomplete Run ⇒ exactly one digest-bound Gap Proposal and no
    WorkItem side effect; duplicates converge; admission creates one isolated
    successor lineage; a later Run demonstrably binds the new revision/digest;
    rollback restores the prior eligible version without rewriting history.
13. Two Schedulers competing to dispatch the same ready Job: exactly one CAS
    winner; the loser's dispatch is rejected with zero side effects.

## 17. Stop conditions

Identity mismatch, P3A baseline missing, unreviewed authority/schema/credential
expansion, Gate 1 unable to close within the three vertical boundaries, a thin
WorkItem, any independent Review FAIL, or un-isolatable dirtiness ⇒ stop
`HUMAN_REQUIRED`; no silent scope expansion.

VERDICT: `ACCEPTED` — GATE 1 CONTRACT RE-REVIEW 2 PASS
