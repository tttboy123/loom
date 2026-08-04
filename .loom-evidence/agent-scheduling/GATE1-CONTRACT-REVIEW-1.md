# v0.4.0 Agent Scheduling Framework — Gate 1 Combined Contract Review 1

Date: `2026-08-04`

Reviewer: fresh independent Contract Reviewer (read-only; separate from the
Gate 1 freeze authorship). Review artifact only; no product or contract file
was modified by this review.

Scope: the combined Gate 1 freeze candidate —

- `docs/adr/0014-agent-scheduling-framework.md` (ADR-0014);
- `.loom-evidence/agent-scheduling/SF-EXIT-CONTRACT.md` (Exit Contract);
- `.loom-evidence/agent-scheduling/SF-WORKITEMS.md` (SF-W1/W2/W3 freeze);
- `.loom-evidence/agent-scheduling/GATE0-AUDIT.md` (Gate 0 evidence).

Authority checked against: the governing goal
(`pasted-text-1.txt`, Agent Scheduling Framework / concurrent development
pipeline), the two queued inputs the freeze claims to reconcile
(`2026-08-01-interaction-handoff-auth-routing-scheduling-queued-input.md`,
`2026-08-04-governed-self-evolution-pipeline-queued-input.md`), the accepted
P3A-W1 baseline `7d5f0b01`, and the accepted alternative-verification +
GUI-evidence-surface amendments under `.loom-evidence/phase3a/`.

VERDICT: `FAIL` — P0=0, P1=7, P2=10. Product/Authority `FAIL`;
Operational and Trace Governance `FAIL`. Product code remains locked until a
fresh independent Contract Re-review returns `PASS`.

---

## 1. Repository identity verification (Gate 0 re-check)

| Check | Command / evidence | Result |
|---|---|---|
| physical cwd | `pwd` | `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild` |
| Git top-level | `git rev-parse --show-toplevel` | identical to cwd |
| branch | `git branch --show-current` | `codex/loom-platform-slice2` |
| HEAD | `git rev-parse HEAD` | `7d5f0b01d5675820f01def08b3888f56e99e841a` |
| docs/CURRENT.md record | tail section | `P3A-W1 = ACCEPTED / LOCAL ATOMIC COMMIT COMPLETE` at `7d5f0b01`; `V0.4.0 = GATE 0 PASS / GATE 1 FREEZED PENDING CONTRACT REVIEW` |
| P3A commit scope | `git show --stat 7d5f0b01` | 102 files changed; `internal/projection/team_execution_test.go`, root docs, `.codex/**`, `.loom-drafts/**` absent from the commit |
| staging state | `git diff --cached --name-only` | empty (nothing staged) |
| dirty boundary | `git status --short` | matches the GATE0 audit exclusion classes (root docs, `internal/projection/team_execution_test.go`, phase1-final-live-gate, earlier evidence, `.codex/**`, `.loom-drafts/**`, `apps/macos/.build/**`, plus current-slice `docs/CURRENT.md`/`PROGRESS.md` and the v0.4.0 governance set) |

Result: identity and dirty-boundary checks PASS.

## 2. Gate 0 prerequisite spot-verification

- Event Journal authority + `AppendBatchIfStreamHeads`: `internal/journal/store.go:256` — verified present.
- Lease/generation fencing: `internal/work/run_authority.go` — `ErrRunLeaseActive`, `ErrRunLeaseExpired`, `ErrStaleClaimGeneration`, `ClaimGeneration` — verified present.
- Runtime capability matrix: `internal/runtime/catalog.go` — `RuntimeProfile`, `ValidateBinding`, `ErrMissingCapability` — verified present.
- Reusable process runner: `internal/runtime/piadapter/` — verified present (P3A-owned).
- Frozen canary fixture: `~/Library/Application Support/Loom/phase1-live` and the pinned Pi binary — verified present.
- Alternative verification substrate: committed `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` and `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` — verified present and consistent with Exit Contract §13.

No Gate 0 prerequisite is missing. Result: PASS.

## 3. Requirements coverage (PASS items)

| Goal / queued-input requirement | Frozen location | Result |
|---|---|---|
| Single Event Journal authority, no second Scheduler/Journal/StateWriter/Projection/queue DB/asset authority/integration writer | ADR Decision; Exit Contract header; §7 | PASS |
| Timeline/Attention/streaming are projections, never authority | ADR Decision; Exit Contract §9; SF-W3 | PASS |
| No model client, Provider route, key, or agent loop owned | ADR Non-decisions; ADR Context (Kun reference, no code) | PASS |
| Non-decisions match goal exclusions (Phase 3B/v0.3, Web/Marketplace, Provider fallback, session-checkpoint authority, auto-approval, hidden reasoning/per-token Journal, auto-activation, unreviewed scripts, network/paid/user config, push/merge, user Skills, second authority) | ADR Non-decisions | PASS |
| Seven required lanes (Admission, Development, Repair, Test, Review, Integration, Human) | Exit Contract §1; queued input v0.4.0 lane list | PASS |
| Exactly seven failure classes with truthful routing; misclassification = contract defect | Exit Contract §2 | PASS |
| At-least-once dispatch + idempotent CAS + lease/generation fencing; stale/late rejected with zero side effects | Exit Contract §3; SF-W2 | PASS |
| Fairness: Repair aging/weighted priority, no starvation; event-driven Scheduler; no Agent busy-poll; no oversell | Exit Contract §4; ADR invariants 3/6 | PASS |
| QueueJob/Attempt/Candidate record schemas | Exit Contract §5 (records) | PASS (record level) |
| Replay/restart: projections rebuildable; old view preserved until complete; `matches_journal=true` | Exit Contract §6 (P3A S5–S8 patterns) | PASS |
| Authority boundaries: protected writers/validators fail closed; Scheduler never invents acceptance; Integrator sole target-branch writer; Reviewer read-only; Test deterministic; Developer cannot review own work | Exit Contract §7; lanes | PASS |
| Decomposition Compiler rules: DAG cycles, owned-path, mutex, authority, resource, vertical capability, exit conditions, integration strategy | Exit Contract §8 (8 rules) | PASS |
| Bounded-recovery ownership frozen (SF-W2); streaming/Timeline/Attention ownership frozen (SF-W3) | Exit Contract §9; ADR Consequences; SF-WORKITEMS | PASS |
| Controlled self-host canary: one offline Run, locked fixture, exact bindings, no duplicates/oversell, projection-failure + crash recovery, two parallel workers, Test before Development completes | Exit Contract §12; SF-W3 | PASS |
| Cross-client Exit Gate per WorkItem with the accepted alternative-verification method; either client missing ⇒ PARTIAL; evidence modes 0700/0600; failures not overwritten | Exit Contract §13; SF-WORKITEMS journeys | PASS (as designed; see F6/F7 for gaps) |
| Exact staging + one atomic local commit per WorkItem; no push/merge; whole-slice review | Exit Contract §14 | PASS |
| Self-evolution folded into the same three boundaries; no SF-W4; no thin WorkItem | Exit Contract §15; queued input "Delivery shape" | PASS |
| Stop conditions | Exit Contract §17 | PASS |
| Exactly three vertical WorkItems with owned-file allowlists, deliverables, RED, journeys, verification | SF-WORKITEMS | PASS (see F2/F6/F7/P2 findings) |

## 4. Mandatory RED mapping (goal list -> Exit Contract §10)

| Goal RED requirement | Exit Contract §10 | Result |
|---|---|---|
| admission/eligibility not met ⇒ not dispatched | #1 | PASS |
| owned-path conflict ⇒ not parallel | #2 | PASS |
| same mutex ⇒ not parallel | #3 | PASS |
| lease expiry / stale generation ⇒ rejected, zero side effects | #4 | PASS |
| crashed worker reclaimed, exactly one new generation | #5 | PASS |
| late result after generation advance rejected | #6 | PASS |
| failure misclassified ⇒ surfaced | #7 | PASS |
| Repair starvation ⇒ fairness test | #8 | PASS |
| test failure ⇒ Repair, never auto-Integrate | #9 | PASS |
| Reviewer cannot write product | #10 | PASS |
| two Integrators ⇒ one CAS winner, no duplicates | #11 | PASS |
| Journal rebuild ⇒ queue projection identical | #12 | PASS |
| restart/replay ⇒ no duplicate Event/Evidence/effect | #13 | PASS |
| canary double-run blocked by idempotent CAS | #14 | PASS |
| slow client ⇒ no hidden retry, one effect on reconnect | #15 | PASS |
| projection failure ⇒ old view preserved, rebuild on restart | #16 | PASS |
| cross-client journey missing ⇒ WorkItem incomplete | #17 | PASS |
| self-evolution: unauthorized/stale evidence cannot create successor | #18 | PASS |
| duplicate gap observations converge on one `gap_id` | #19 | PASS |
| protected-authority path modification ⇒ fail closed | #20 | PASS |
| (goal minimum acceptance) two Schedulers competing dispatch ⇒ one CAS winner | absent from §10 and §16 | **MISSING — F5** |

## 5. Minimum acceptance mapping (goal list -> Exit Contract §16)

Goal items 1–11 (parallel independent WorkItems, Test before Development
completes, Repair routing, crash reclaim, stale rejection, mutex/path
serialization, Reviewer/Integrator discipline, Journal rebuild, canary
no-duplicates/oversell/projection-failure/crash recovery, authorized
generation-bound streaming, GUI+TUI + dual Result P0=P1=P2=0) all map to §16
items 1–11. Gap-to-successor minimums map to §16.12. The goal item "two
Schedulers competing dispatch ⇒ one CAS winner" has **no §16 counterpart** —
**MISSING — F5**. Budget/network/privacy admission compatibility from the
self-evolution queued input has no frozen reconciliation — **P2-6**.

## 6. Findings

### P0

None. The freeze does not create a second authority, database, writer, or
thin WorkItem; no authority/safety violation found.

### P1 (blocking — must be closed by a reviewed Contract Repair 1)

**F1 — Lane state machine is not frozen; §1/§5 lane enum is internally
contradictory.**
The goal requires "lane 状态机与失败分类" frozen. Exit Contract §1 defines
seven lanes with Permitted/Forbidden but no legal transition table; §5 freezes
`QueueJob.status` and `QueueJob.lane` enums only. An implementer cannot derive
the legal transitions (e.g., `queued→admitted→dispatched→in_progress→
waiting_review→ready_integrate→integrated`; which statuses are terminal;
whether `cancelled`/`human_required` can resume; which actor may move between
lanes). Additionally §5 `QueueJob.lane` is
`development|repair|test|review|integration|human` while §1 defines an
`admission` lane — the two tables contradict. Closure: freeze an explicit
lane/status transition matrix (source status, event, guard, target status) and
align the lane enum with §1.

**F2 — New Event payload schemas are not frozen.**
The goal requires "Queue/Event/Job/Candidate/Attempt schema" frozen. §5 freezes
three record schemas and an event-name list, but no per-event payload
(field-by-field), stream/idempotency formula, or record→event mapping (which
event carries which record/payload; `AttemptClaimed` fields; `LeaseReclaimed`
old/new generation; `GenerationAdvanced`; `AdmissionDecisionRecorded`, etc.).
The P3A-W1 contract froze "every Event and IPC action field"; this freeze does
not reach the same exactness. Closure: freeze each named Event's exact payload
fields, types, stream identity, and idempotency formula.

**F3 — Gap Proposal record schema is missing.**
The Exit Contract claims to reconcile the governed self-evolution queued
input, whose Gap Proposal must bind source type/IDs/digests, affected
capability, observed/expected behavior, user impact, confidence, uncertainty,
reproducibility, privacy classification, proposed scope, owned-path/resource
claims, risk class, rollback idea, and duplicate/supersession. §5 names
`GapProposalCreated`/`SuccessorProposalCreated` events but freezes no Gap
Proposal record; SF-W1 deliverable 3 covers only `gap_id`, digest binding,
dispositions, and dedup. Implementers would have to import fields from a
non-authoritative queued input. Closure: add a frozen `GapProposal` record
schema to §5 with the required field set.

**F4 — The before/after-CAS crash seam cannot be represented.**
§3 requires the crash point (before/after CAS) to be "recorded truthfully
(P3A S5/S6 seams as the pattern)", but the frozen `Attempt` record has no
crash-phase/`crash_point` field and no event payload defines where the seam is
recorded (`AttemptCrashed` is named only). Closure: add a frozen crash-seam
field (or equivalent event payload) to the Attempt/event schema.

**F5 — Dual-Scheduler single-winner acceptance is missing.**
The goal's minimum acceptance requires "双 scheduler 竞争 dispatch 只有一个 CAS
winner". §10 RED and §16 minimum acceptance omit it; §3 implies it via
idempotent CAS but the acceptance/RED contract does not state it, so the slice
end-state cannot be verified against the goal. Closure: add the explicit RED
item and §16 acceptance item.

**F6 — SF-W1 and SF-W2 own no TUI surface for their mandatory journeys.**
SF-W1 and SF-W2 journeys require both clients to observe queue admission,
conflict serialization, parallel workers, crash reclamation, Repair entry, and
Review-denial ("visible in both clients"), but neither allowlist owns any
`internal/tui/` file; SF-W3 owns only `internal/tui/attention.go`. The current
P3A-owned TUI (`internal/tui/model.go`, 4046 lines) has screens (Board, Assets,
Timeline, Attention, …) but no queue/worker surface, and the allowlist header
forbids modifying accepted P3A files. Closure: add the exact TUI files needed
for queue/worker visibility to SF-W1/SF-W2 allowlists (e.g.,
`internal/tui/queue.go` new view), or freeze explicitly how existing TUI
surfaces expose the new state without code change.

**F7 — Per-WorkItem journeys do not include reconnect/restart/recovery.**
The goal's Cross-client Exit Gate requires each WorkItem's GUI+TUI journey to
cover 重连/restart/恢复, and the Alternative Verification Amendment retains TUI
"reconnect and restart" as unchanged. The frozen SF-W1/W2/W3 journeys contain
no restart/reconnect/recovery checkpoint. Closure: add at least one explicit
restart/reconnect/recovery checkpoint to each WorkItem's journey definition
(e.g., daemon restart mid-journey with both clients observing the rebuilt
state), or a frozen slice-level restart journey mapped per WorkItem.

### P2 (non-blocking precision — should be closed with Contract Repair 1)

**P2-1 — Allowlist header contradicts P3A-owned protocol files.**
SF-WORKITEMS header forbids modifying accepted P2A/P2B/P3A files while SF-W1/
W2/W3 allowlists include P3A-owned `internal/localipc/protocol.go` and
`protocol_test.go` ("additive actions only"). Closure: explicit carve-out
("accepted P3A `internal/localipc/protocol.go`/`protocol_test.go` are reopened
only additively: new action IDs/params; existing action semantics and wire
format unchanged").

**P2-2 — Queue projection wiring mechanism unspecified.**
`internal/projection/projection.go` is P3A-owned and absent from every
allowlist, yet it is where the core `Snapshot` applies events. The freeze must
state that the queue projection is a separate rebuildable read model in the
new `internal/projection/queue.go` files (not folded into the core Snapshot),
or the allowlist must be amended. As written, implementers cannot tell whether
opening `projection.go` is permitted.

**P2-3 — Decomposition Compiler implementation ownership unnamed.**
§8 freezes the Compiler rules and SF-W1 verification mentions the Compiler
matrix, but no owned file is named for the Compiler implementation. Closure:
explicitly assign it to SF-W1 (`admission.go`/`gap_proposal.go`/
`conflict_arbiter.go`).

**P2-4 — Scheduler implementation ownership unnamed.**
The resident event-driven Scheduler spans SF-W1 admission (compile/validate)
and SF-W2 router (dispatch); no owned file is named. Closure: freeze that the
Scheduler is implemented by SF-W1 admission + SF-W2 `router.go`, with no
separate scheduler package or authority.

**P2-5 — Cross-WorkItem `protocol.go` sequencing not stated.**
The same file appears in all three allowlists. Closure: state that each
WorkItem's protocol extension is additive against the prior accepted commit and
that parallel Candidates never both include `protocol.go` (conflict arbiter
serializes it as a shared owned path).

**P2-6 — Budget/network/privacy admission compatibility not reconciled.**
The self-evolution queued input's Admission list includes budget, network, and
privacy compatibility; §8 and SF-W1 deliverable 2 freeze only
owned-path/mutex/authority/resource. The freeze should explicitly defer
budget/network/privacy to the v0.3.0 planning boundary (the v0.4.0 queued-input
section lists only owned paths/mutexes/authorities/resources) so the
"reconciles" claim is precise.

**P2-7 — Gate 1 governance commit not frozen.**
§14 defines only per-WorkItem commits; GATE0-AUDIT says `docs/CURRENT.md`
"carries into the v0.4.0 Gate 1 commit". Closure: freeze the docs-only Gate 1
governance commit (ADR-0014 + Exit Contract + WorkItems + Gate 0 audit +
CURRENT/PROGRESS records) as one atomic local commit after Contract Review
PASS and before RED.

**P2-8 — Timeline/Attention content guardrails not explicit.**
The goal forbids hidden reasoning, credentials, and per-token output in
Timeline/Attention/streaming. §9/§16.10 freeze authorization and
generation-bound publication but not the content exclusions. Closure: add the
explicit "never contains hidden reasoning, credentials, raw Grants, or
per-token output" constraint.

**P2-9 — Capacity "frozen limits" undefined.**
§4 says "active workers per pool ≤ frozen limits" without a mechanism or
defaults. Closure: freeze the pool-limit configuration surface (or state that
limits are operator-configurable with the no-oversell invariant as the hard
bound).

**P2-10 — Plan reconciliation claim is soft.**
The Exit Contract claims to reconcile `PRODUCT-PLAN.md`/`TECH-PLAN.md`; the
plans contain the Scheduler/queue architecture (TECH-PLAN Scheduler/queue
sections) but no `v0.4.0` release label. Closure: state that the queued inputs
place the release label and TECH-PLAN is the architecture reference, keeping
the claim precise.

## 7. Dual-axis verdict

**Product/Authority: `FAIL`** — P0=0, P1=4 (F2, F3, F4, F6 — plus F1's schema
contradiction counted below), P2=6. The single-authority architecture and
non-decision boundaries are sound and complete, but the frozen schema/state
machine exactness the goal requires is not reached.

**Operational and Trace Governance: `FAIL`** — P0=0, P1=5 (F1, F5, F6, F7,
plus F4), P2=4. Lane transitions, crash-seam recording, dual-Scheduler
acceptance, per-WorkItem TUI ownership, and restart/reconnect journeys are not
yet verifiable from the frozen text.

## 8. Close conditions

1. Freeze a **Contract Repair 1** addressing all seven P1 findings (F1–F7) and
   the P2 precision items that affect implementability (at minimum P2-1,
   P2-2, P2-3, P2-4, P2-5, P2-6, P2-7, P2-8).
2. Obtain a **fresh independent Contract Re-review `PASS`** (P0=P1=P2=0) on
   the repaired combined freeze.
3. Product code remains locked until that PASS; the first RED for SF-W1 is
   captured only afterward.

VERDICT: `FAIL` — P0=0, P1=7, P2=10 — pending Contract Repair 1 + fresh
independent Re-review.
