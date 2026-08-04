# v0.4.0 Agent Scheduling Framework — Gate 1 Combined Contract Re-review (Contract Repair 1) — PASS

Date: `2026-08-04`

Reviewer: fresh independent read-only Contract Reviewer
(`p3a_gate1_contract_rereviewer`), separate from the Gate 1 freeze and
Contract Repair 1 authorship. Review artifact only; no contract, goal,
product, or evidence file was modified by this review. All audits were
performed read-only and independently, against the actual bytes of the
amended files (the repair document's prose was not accepted as evidence).

Scope: the combined Gate 1 freeze candidate after `SF-CONTRACT-REPAIR-1.md`
closes the seven P1 and ten P2 findings of `GATE1-CONTRACT-REVIEW-1.md`
(FAIL, P0=0 P1=7 P2=10) —

- `docs/adr/0014-agent-scheduling-framework.md` (ADR-0014);
- `.loom-evidence/agent-scheduling/SF-EXIT-CONTRACT.md` (repaired Exit Contract);
- `.loom-evidence/agent-scheduling/SF-WORKITEMS.md` (repaired WorkItem freeze);
- `.loom-evidence/agent-scheduling/GATE0-AUDIT.md` (Gate 0 evidence);
- `.loom-evidence/agent-scheduling/GATE1-CONTRACT-REVIEW-1.md` (prior FAIL);
- `.loom-evidence/agent-scheduling/SF-CONTRACT-REPAIR-1.md` (closure claims).

Authority checked against: the governing goal
(`pasted-text-1.txt`, Agent Scheduling Framework / concurrent development
pipeline), the two queued inputs the freeze reconciles
(`2026-08-01-interaction-handoff-auth-routing-scheduling-queued-input.md`,
`2026-08-04-governed-self-evolution-pipeline-queued-input.md`),
`PRODUCT-PLAN.md` / `TECH-PLAN.md` (architecture reference), the accepted
P3A-W1 baseline `7d5f0b01`, and the accepted alternative-verification +
GUI-evidence-surface amendments under `.loom-evidence/phase3a/`.

## 1. Repository identity verification (Gate 0 re-check)

| Check | Command / evidence | Result |
|---|---|---|
| physical cwd | `pwd` | `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild` |
| Git top-level | `git rev-parse --show-toplevel` | identical to cwd |
| branch | `git branch --show-current` | `codex/loom-platform-slice2` |
| HEAD | `git rev-parse HEAD` | `7d5f0b01d5675820f01def08b3888f56e99e841a` |
| docs/CURRENT.md record | tail section | `P3A-W1 = ACCEPTED / LOCAL ATOMIC COMMIT COMPLETE` at `7d5f0b01`; `V0.4.0 = GATE 0 PASS / GATE 1 FREEZED PENDING CONTRACT REVIEW` |
| P3A commit scope | `git show --name-only 7d5f0b01` | 102 files; `internal/projection/team_execution_test.go`, root docs, `.codex/**`, `.loom-drafts/**` absent from the commit |
| staging state | `git diff --cached --name-only` | empty (nothing staged) |
| product-code lock | `git diff HEAD` over product paths (excluding docs/CURRENT.md, root docs, `.loom-evidence/**`, `.codex/**`, `.loom-drafts/**`, `apps/macos/.build/**`, `internal/projection/team_execution_test.go`) | empty — product code unchanged since the accepted P3A baseline |
| dirty boundary | `git status --short` | matches the Gate 0 audit exclusion classes (root docs, `internal/projection/team_execution_test.go`, phase1-final-live-gate, earlier-slice evidence, `.codex/**`, `.loom-drafts/**`, `apps/macos/.build/**`, current-slice `docs/CURRENT.md`, and the v0.4.0 governance set) |

Result: identity and dirty-boundary checks PASS.

## 2. Gate 0 prerequisite spot-verification

- Event Journal authority + `AppendBatchIfStreamHeads`:
  `internal/journal/store.go` — verified present.
- Lease/generation fencing: `internal/work/run_authority.go`
  (`ErrRunLeaseActive`, `ErrRunLeaseExpired`, `ErrStaleClaimGeneration`,
  `ClaimGeneration`) — verified present.
- Runtime capability matrix: `internal/runtime/catalog.go`
  (`RuntimeProfile`, `ValidateBinding`) — verified present.
- Reusable process runner: `internal/runtime/piadapter/` — verified present.
- Frozen canary fixture: `~/Library/Application Support/Loom/phase1-live`
  and the pinned Pi runtime — verified present.
- Alternative verification substrate: committed
  `P3A-W1-ALTERNATIVE-VERIFICATION-AMENDMENT.md` and
  `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md` — verified present and
  consistent with Exit Contract §13.

No Gate 0 prerequisite is missing. Result: PASS.

## 3. P1 closure verification (F1–F7), each against the actual amended text

| Finding | Required closure | Verified in amended files | Result |
|---|---|---|---|
| F1 lane/status state machine | Legal lane/status transition matrix; `QueueJob.lane` aligned with the seven §1 lanes including `admission` | `SF-EXIT-CONTRACT.md` §1 freezes the matrix (source status, event, guard, target status; terminal statuses `integrated`/`rejected`/`cancelled`/`human_required`; `human_required` resumes only through an explicit Product Owner decision event). §5 `QueueJob.lane` is exactly `admission|development|repair|test|review|integration|human`, and the header states `admission` is the Scheduler's compile/validate lane until `QueueJobAdmitted`. The prior §1/§5 contradiction is removed. | CLOSED |
| F2 Event payload schemas | Per-event payload fields, stream identity, idempotency formula | §5 freezes: every named Event payload = the corresponding record's frozen fields plus `correlation_id`/`evidence_digests`; stream identity = `job_id` lineage; idempotency key per transition identity (`attempt_id`, `candidate_id`, `integration_id`, `gap_id`, `canary_id`, `release_id`). | CLOSED |
| F3 Gap Proposal schema | Field-exact `GapProposal` record | §5 freezes the `GapProposal` record with source type/IDs/digests, affected capability, observed/expected behavior, user impact, confidence, uncertainty, reproducibility, privacy classification, proposed scope, owned-path/resource claims, risk class, rollback idea, duplicate/supersession, disposition; `GapProposalCreated` payload = the record fields; duplicate digest-bound observations converge on one `gap_id`; stale/unauthorized evidence cannot create a successor. | CLOSED |
| F4 crash seam | Representable before/after-CAS seam | Frozen `Attempt` record includes `crash_seam` (`before_cas|after_cas|null`) and `crash_effect_cardinality` (`zero_effects|single_effect|null`); §1 Attempt lifecycle records `crashed` with the frozen `crash_seam` per the P3A S5/S6 pattern. | CLOSED |
| F5 dual-Scheduler single winner | Explicit RED + acceptance item | §10 RED item 21 and §16 minimum acceptance item 13 both state: two Schedulers competing to dispatch the same ready Job ⇒ exactly one CAS winner, loser rejected with zero side effects. | CLOSED |
| F6 TUI ownership SF-W1/SF-W2 | TUI files for mandatory journeys | `SF-WORKITEMS.md` SF-W1 allowlist includes `internal/tui/queue.go` + `queue_test.go`; SF-W2 allowlist includes `internal/tui/workers.go` + `workers_test.go`. | CLOSED |
| F7 restart/reconnect/recovery in every journey | Explicit restart checkpoint per WorkItem | Each WorkItem journey in `SF-WORKITEMS.md` (SF-W1/SF-W2/SF-W3) and the canary in `SF-EXIT-CONTRACT.md` §12 include an explicit daemon-restart mid-journey checkpoint with both clients observing the rebuilt state through their own production reads and no duplicate facts; §13 makes a restart/reconnect/recovery checkpoint mandatory for every WorkItem. | CLOSED |

All seven P1 findings: CLOSED.

## 4. P2 closure verification (P2-1…P2-10), each against the actual amended text

| Finding | Required closure | Verified in amended files | Result |
|---|---|---|---|
| P2-1 | Shared-path carve-out for `internal/localipc/protocol.go`/`protocol_test.go` (additive only) | `SF-WORKITEMS.md` header: reopened "only additively (new action IDs/params; existing action semantics and wire format unchanged)". | CLOSED |
| P2-2 | Queue projection wiring specified | Header: queue projection is "a separate rebuildable read model in `internal/projection/queue.go`"; the accepted P3A core `internal/projection/projection.go` is not reopened. | CLOSED |
| P2-3 | Decomposition Compiler ownership | `SF-EXIT-CONTRACT.md` §8: implemented by SF-W1 (`internal/queue/admission.go`, `internal/queue/conflict_arbiter.go`, `internal/queue/gap_proposal.go`); no separate compiler package. | CLOSED |
| P2-4 | Scheduler ownership | Header: event-driven Scheduler has no separate package — SF-W1 admission (`admission.go`, `conflict_arbiter.go`, `gap_proposal.go`) + SF-W2 dispatch (`internal/schedule/router.go`). | CLOSED |
| P2-5 | Cross-WorkItem `protocol.go` sequencing | Header: each WorkItem extends the protocol against the prior accepted commit; parallel Candidates never both include `protocol.go` — the Conflict Arbiter serializes it as a shared owned path. | CLOSED |
| P2-6 | Budget/network/privacy deferred | `SF-EXIT-CONTRACT.md` §4: budget, network and privacy admission compatibility deferred to the `v0.3.x` planning boundary; v0.4.0 Admission covers owned paths, mutex keys, authority and resource claims only. | CLOSED |
| P2-7 | Docs-only Gate 1 governance commit | §14: "A docs-only Gate 1 governance commit (ADR-0014 + Exit Contract + WorkItems + Gate 0 audit + SF-CONTRACT-REPAIR-1 + `docs/CURRENT.md`/`docs/adr/README.md` records) is frozen as one atomic local commit after the combined Contract Review PASS and before any RED." | CLOSED |
| P2-8 | Timeline/Attention content guardrail | §9: "Timeline/Attention/streaming content never contains hidden reasoning, credentials, raw Grant tokens, or per-token output (frozen content guardrail; only authorized, generation-bound frames are published)." | CLOSED |
| P2-9 | Capacity "frozen limits" defined | §4: pool limits are operator-configurable through the frozen configuration surface; the no-oversell invariant (active workers ≤ configured pool capacity) is the hard bound enforced by the Admission/Conflict Arbiter. | CLOSED |
| P2-10 | Plan reconciliation precise | §8: `PRODUCT-PLAN.md`/`TECH-PLAN.md` are the architecture reference (TECH-PLAN contains the Scheduler/Task-Graph and queue-state architecture; PRODUCT-PLAN/TECH-PLAN both state no sandbox scheduler/checkpoint is state authority); the `v0.4.0` release label and self-evolution boundary come from the frozen queued inputs, not the plans. | CLOSED |

All ten P2 findings: CLOSED.

## 5. Mandatory RED coverage (goal list → Exit Contract §10)

Every goal RED item maps to a frozen §10 item: admission/eligibility (#1);
owned-path conflict (#2); same mutex (#3); lease expiry/stale generation
(#4); crashed worker reclaim (#5); late result (#6); misclassification
surfaced (#7); repair starvation (#8); test failure → Repair, never
auto-Integrate (#9); Reviewer cannot write product (#10); dual Integrator
CAS (#11); Journal rebuild (#12); restart/replay no duplicates (#13); canary
double-run blocked (#14); slow client no hidden retry (#15); projection
failure preserves old view (#16); cross-client journey missing ⇒ WorkItem
incomplete (#17); gap-to-successor unauthorized/stale evidence (#18);
duplicate gap convergence (#19); protected-authority fail closed (#20);
dual Scheduler CAS single winner (#21 — the previously missing item).

Result: complete, including the previously missing dual-Scheduler RED.

## 6. Minimum acceptance coverage (goal list → Exit Contract §16)

Goal minimums all map: parallel independent WorkItems (§16.1); Test Executor
before Development completes (§16.2); classified failure → Repair (§16.3);
crash reclaim from Journal (§16.4); stale generation zero side effects
(§16.5); same mutex/path never parallel (§16.6); Reviewer/Integrator
discipline (§16.7); Journal rebuild (§16.8); canary no duplicates/oversell/
projection-failure/crash (§16.9); authorized generation-bound streaming
(§16.10); GUI+TUI journeys + dual Result P0=P1=P2=0 (§16.11); gap-to-successor
minimums (§16.12); dual Scheduler CAS winner (§16.13).

Result: complete.

## 7. Architecture, boundaries, and governance checks

- **Single authority**: ADR-0014 Decision explicitly forbids a second
  Scheduler/Journal/StateWriter/Projection/queue database/asset authority/
  integration writer; Timeline/Attention/streaming are projections, never an
  authority. Verified consistent with both queued inputs.
- **Exactly three vertical WorkItems**: SF-W1 Queue/Admission, SF-W2 Worker
  Pools/Recovery/Routing, SF-W3 Integration/Canary/Timeline; no SF-W4, no
  thin WorkItem; gap-to-successor folded into the same three boundaries
  (Exit Contract §15).
- **Observability and bounded recovery ownership frozen explicitly**:
  bounded recovery → SF-W2; streaming node output + Timeline/Attention →
  SF-W3 (§9). No silent omission and no single-point Amendment route.
- **Cross-client Exit Gate**: §13 requires real GUI + TUI vertical journeys
  per WorkItem using the accepted alternative-verification method (production
  native window with `--socket --journey-id`, real PTY TUI, production Swift
  client over the real socket; Computer-Use-driven window automation skipped
  per PO instruction 2026-08-04), with the GUI evidence surface per the
  frozen `P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md`; either client missing ⇒
  WorkItem PARTIAL.
- **Exclusions and stop conditions**: ADR Non-decisions and Exit Contract §17
  match the goal (no push/merge, no network/paid/user configuration, no
  second authority, no hidden infinite retry, no per-token Journal writes,
  no auto-activation, etc.).

## 8. Dual-axis verdict

```text
P0 = 0
P1 = 0
P2 = 0
Product/Authority: PASS
Operational and Trace Governance: PASS
VERDICT: PASS
```

## 9. Close conditions

1. All seven P1 (F1–F7) and all ten P2 (P2-1…P2-10) findings of
   `GATE1-CONTRACT-REVIEW-1.md` are closed by verified text in the amended
   `SF-EXIT-CONTRACT.md` and `SF-WORKITEMS.md`.
2. The goal's mandatory RED set and minimum-acceptance set are fully covered,
   including the previously missing dual-Scheduler CAS single-winner items
   (§10 #21, §16 #13).
3. Repository identity, dirty boundary, and product-code lock verified.
4. The next step is the frozen docs-only Gate 1 governance commit (§14,
   P2-7) after this PASS, then SF-W1 RED before any product write.

## 10. Conclusion

The repaired combined Gate 1 freeze reaches the exactness the goal requires:
frozen lane/status state machine, per-event payload and idempotency
semantics, field-exact GapProposal schema, crash-seam representation,
dual-Scheduler acceptance, per-WorkItem TUI ownership, and mandatory
restart/reconnect journeys. The architecture remains single-authority, the
product code is locked, and no authority, schema, credential, or external
action is expanded. The combined v0.4.0 Gate 1 Contract Re-review returns
`PASS` with no findings.

VERDICT: `PASS`
