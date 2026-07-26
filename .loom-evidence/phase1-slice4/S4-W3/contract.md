# S4-W3 Contract — Evidence Verification and WorkItem Acceptance Integration

Status: FROZEN — independent Contract Repair Review 2 PASS.

- WorkItem: `S4-W3`
- Risk: `HIGH`
- Baseline: `6d3cbf2`
- Date: `2026-07-26`
- Depends on: accepted S4-W1 and S4-W2
- Required amendment: `../EXIT-CONTRACT-AMENDMENT-3.md`
- Capability: one user-observable, Journal-authoritative verification and
  terminal-once WorkItem acceptance boundary

S4-W3 is the third and final Slice 4 product WorkItem. Acceptance contract,
deterministic checks, verifier routing/execution, Evidence binding, rejection
recovery, Work/Team CAS, Projection, and controlled whole-Slice proof are
internal parts of this one Candidate. No verifier-router, Evidence adapter,
acceptance writer, recovery wrapper, projection, canary, S4-W4, or other thin
WorkItem may follow.

## Exact ownership

New product/tests:

- `internal/verification/acceptance.go`
- `internal/verification/acceptance_test.go`
- `internal/work/verification_authority.go`
- `internal/work/verification_authority_test.go`

Reopened product/tests, only as allowed by Amendment 3:

- `internal/rules/recovery_policy.go`
- `internal/rules/recovery_policy_test.go`
- `internal/work/run_authority.go`
- `internal/work/run_authority_test.go`
- `internal/work/team_execution_authority.go`
- `internal/work/team_execution_authority_test.go`
- `internal/app/team_execution.go`
- `internal/app/team_execution_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/run_authority.go`
- `internal/projection/run_authority_test.go`
- `internal/projection/team_execution.go`
- `internal/projection/team_execution_test.go`
- `internal/projection/global_read_view.go`
- `internal/projection/global_read_view_test.go`

Governance:

- `.loom-evidence/phase1-slice4/EXIT-CONTRACT-AMENDMENT-3.md`
- `.loom-evidence/phase1-slice4/EXIT-CONTRACT-AMENDMENT-3-REVIEW-*.md`
- `.loom-evidence/phase1-slice4/S4-W3/**`
- Controller-owned S4-W3 hunk in `docs/CURRENT.md`

No other file is owned. `AGENTS.md`, `PROGRESS.md`, `.codex/**`,
`.loom-drafts/**`, and the post-S3 scratch queue remain untouched/unstaged.

## Immutable acceptance contract

`verification.AcceptanceContract` is a non-forgeable immutable value created
only through a validating constructor. It freezes:

- schema/version;
- exact sorted unique acceptance criteria, each bounded to 1–256 UTF-8 bytes,
  at most 16 criteria and 2048 total bytes;
- risk: exactly `low`, `medium`, or `high`;
- independent-verifier requirement derived from risk, never caller-selected:
  `low=false`, `medium=true`, `high=true`; and
- canonical SHA-256 digest over a schema tag and length-prefixed fields.

Accessors return copies. Zero values, invalid UTF-8, control characters,
duplicates after trimming, unsorted ambiguity, invalid risk, unknown versions,
digest mutation, and limit overflow fail closed. Rejection routing is not
duplicated here; it remains owned by the already-frozen per-node S4-W2
RecoveryPolicy.

`AcceptanceContract` never contains credentials, Grant material, raw model
output, hidden reasoning, or a client/UI decision.

## Frozen Team binding

Before the first Team dispatch, every node freezes in
`TeamExecutionPlanned`:

- AcceptanceContract version and digest;
- risk and independent-verifier requirement;
- verifier AgentInstance and RuntimeInstance bindings when required;
- verifier workflow path key;
- verifier maximum one execution for each source attempt.

Low risk must have no verifier binding. Medium/high must bind one verifier
AgentInstance distinct from the source executor AgentInstance and a discovered
RuntimeInstance. The verifier may share a Runtime only when normal capacity
authority permits it. Any changed acceptance digest/risk/verifier binding/path
on re-entry conflicts before later WorkItem/Run/Grant/Evidence writes.

Accepted historical Team streams without S4-W3 bindings replay as
`legacy_acceptance_unbound`. They remain queryable but cannot acquire S4-W3
acceptance authority or be rewritten.

The existing per-node `teamSemanticBindingPayload` adds exactly:

```text
acceptance_contract_version, acceptance_contract_digest, risk,
independent_verifier_required, verifier_agent_instance_id,
verifier_runtime_instance_id, verifier_workflow_path
```

All fields participate in the existing sorted canonical Team semantic-binding
digest. `TeamExecutionPlanned` retains its accepted payload/envelope and binds
the updated canonical digest. Its deterministic Event ID remains:

```text
TeamExecutionPlanned, team_instance_id, plan_digest,
canonical_complete_semantic_binding_digest
```

Exact replay permits either historical schema without any S4-W3 fields
(`legacy_acceptance_unbound`) or the complete new field set. Partial/defaulted
S4-W3 bindings are invalid.

## Executor boundary

A source executor may produce only:

- a generation-fenced terminal Run;
- immutable authorized-output Evidence;
- exact S4-W2 classification; and
- `WorkItemReadyForReview`.

For `valid_nonempty` or allowed `valid_empty`, the Team node becomes
`ready_for_review` with dependency satisfaction false. It is not `succeeded`
and cannot unblock dependents. Transient/invalid output continues through the
accepted S4-W2 recovery path.

No executor API accepts or returns `done`, `accepted`, a verification result, a
caller-supplied acceptance boolean, or an authoritative rejection route.

## Deterministic verification

`verification.VerifyDeterministic` is pure and receives immutable values:

- exact AcceptanceContract;
- exact Team/node/attempt and WorkItem/Run/generation binding;
- exact source Evidence ID/digest, OutputSummary digest, OutputContract
  version/digest, classification and classification digest;
- exact acceptance criteria from the contract; and
- terminal status.

It validates complete binding, a successful terminal Run, a valid output
classification, non-zero exact Evidence identities/digests, and all contract
invariants. It returns an immutable deterministic result with canonical digest:

- `accepted` for a valid low-risk input;
- `needs_independent_verifier` for a valid medium/high-risk input; or
- `rejected` with one bounded enumerated reason code.

It performs no I/O and never sees raw artifact bytes, raw Frames, credentials,
Grant material, prompts, or hidden reasoning.

## Independent Verifier lineage

For `needs_independent_verifier`, the coordinator derives stable identities
from Team/plan/node/source-attempt/source-Evidence/AcceptanceContract:

- verifier WorkItem;
- verifier Run;
- verifier Evidence.

The identities are lower-case SHA-256/UUID-form values over these exact
length-prefixed inputs:

```text
verifier-work: team_instance_id, plan_digest, logical_node_id,
source_attempt_number, source_work_item_id, source_run_id,
source_evidence_digest, acceptance_contract_digest

verifier-run: verifier_work_item_id, verifier_agent_instance_id,
verifier_runtime_instance_id, verifier_workflow_path

verifier-evidence: verifier_work_item_id, verifier_run_id,
source_evidence_digest, acceptance_contract_digest
```

The existing Grant authority continues to generate its own collision-checked
Grant ID/token and bind it to the deterministic verifier WorkItem/Run Claim;
S4-W3 does not invent a second Grant identity rule.

The verifier lineage is distinct from the source WorkItem/Run/Grant/Evidence.
Its AgentInstance must be distinct from the source executor. The coordinator
uses the accepted `CreateAndAssign → Claim → AgentGrant.Issue → Supervisor
Execute → AttemptCapture finalize → CommitTerminal` sequence. Normal Runtime
status/capacity, Claim generation, Grant operation/expiry, Frame
binding/sequence, and terminal-once rules apply unchanged.

The verifier receives only a bounded verification request containing exact
source digests, criteria, risk, and allowed reason-code vocabulary. It does not
receive the source Grant, credentials, hidden reasoning, or unrestricted
artifact paths.

The verifier Candidate is reconstructed restart-safely from the accepted
generation-fenced verifier Run terminal plus its exact Store-returned Evidence
receipt:

- terminal `succeeded` with empty reason means Candidate `accepted` and reason
  `criteria_satisfied`;
- terminal `failed` with reason exactly `criteria_not_satisfied` or
  `insufficient_evidence` means Candidate `rejected`;
- every other status/reason combination is invalid.

`verification.VerifierCandidateFromTerminal` is pure, binds the verifier
WorkItem/Run/generation/Agent/Runtime plus Evidence ID/digest/summary digest,
and returns an immutable canonical digest. It remains a Candidate, never
authority. Malformed, missing, stale-generation, unauthorized, mismatched, or
divergent terminal/Evidence combinations fail closed and are never published
as a verification fact. Raw verifier Frames stay only in the immutable Evidence
artifact; restart needs no raw artifact read or second metadata store.

Restart rules:

- finalized verifier receipt plus missing Journal metadata is committed without
  re-execution;
- expired never-started verifier Claim may use the accepted single rebound
  behavior;
- an indeterminate running verifier returns typed `human_required`, never
  silently re-executes;
- exact re-entry is idempotent; divergent re-entry conflicts.

## Acceptance decision

`verification.DecideAcceptance` is pure and consumes:

- the exact AcceptanceContract;
- deterministic result;
- optional exact verifier Candidate and verifier Evidence receipt;
- source and verifier lineage bindings; and
- authoritative decision time.

It returns an immutable canonical `AcceptanceDecision`:

- `accepted`; or
- `rejected`.

An accepted low-risk decision has no verifier fields. Medium/high accepted
decisions require the exact distinct verifier Run/generation/Agent/Grant
authorization/Evidence lineage. A valid verifier rejection yields only
`rejected`; it does not choose retry, fallback, degraded, blocked, or
human-required.

`rules.RecoveryPolicy` is extended only with the explicit trigger
`verification_rejected` plus the exact AcceptanceDecision digest. On that
trigger it owns the resulting `retry`, `degraded`, `blocked`, or
`human_required` action and any exact `retry_at`, using the already-frozen
attempt/credit/fallback/approval inputs. Workflow fallback is forbidden for a
verification rejection. Existing output-classification behavior is unchanged.
The scheduler only executes the pure RecoveryDecision; it does not invent
acceptance or retry.

## Typed errors

The Candidate adds exactly these public typed errors:

`internal/verification`:

- `ErrInvalidAcceptanceContract`
- `ErrAcceptanceContractDigestMismatch`
- `ErrInvalidDeterministicVerification`
- `ErrInvalidVerifierCandidate`
- `ErrInvalidAcceptanceDecision`

`internal/work`:

- `ErrInvalidWorkItemAcceptance`
- `ErrWorkItemAcceptanceConflict`
- `ErrWorkItemAlreadyAccepted`
- `ErrIndependentVerificationRequired`
- `ErrVerifierLineageMismatch`
- `ErrLegacyAcceptanceUnbound`

Validation/zero-value failures map to the applicable `ErrInvalid*`. A missing
required verifier maps to `ErrIndependentVerificationRequired`. Exact lineage
or Grant/Evidence mismatch maps to `ErrVerifierLineageMismatch`. Stale heads,
divergent idempotency, reordered facts, or concurrent loss map to
`ErrWorkItemAcceptanceConflict`. A second non-identical terminal acceptance
maps to `ErrWorkItemAlreadyAccepted`. Context cancellation/deadline errors
remain discoverable with `errors.Is` and are never normalized to authority
success.

## Exact Event schemas and identities

All new Events use schema version 1, canonical exact JSON, UTC `emitted_at`,
non-empty canonical correlation ID, idempotency key equal to Event ID, and the
existing `newEvent` envelope.

`WorkItemVerificationCommitted` is written to
`work-item/<source_work_item_id>` and contains exactly:

```text
team_instance_id, plan_digest, logical_node_id, attempt_number,
work_item_id, run_id, claim_id, claim_generation,
source_evidence_id, source_evidence_digest, output_summary_digest,
output_contract_version, output_contract_digest,
output_classification, output_classification_digest,
acceptance_contract_version, acceptance_contract_digest, risk,
deterministic_result_digest, verifier_required,
verifier_work_item_id, verifier_run_id, verifier_claim_id,
verifier_claim_generation, verifier_runtime_instance_id,
verifier_agent_instance_id, verifier_grant_id,
verifier_evidence_id, verifier_evidence_digest,
verifier_output_summary_digest, verifier_candidate_kind,
verifier_reason_code, verifier_candidate_digest,
acceptance_decision_kind, acceptance_decision_digest, decided_at
```

Every verifier field is the empty/zero value for low risk and all are required
for medium/high. Its Event ID is the deterministic SHA-256/UUID-form identity
over:

```text
WorkItemVerificationCommitted, team_instance_id, plan_digest,
logical_node_id, attempt_number, work_item_id, run_id, claim_generation,
source_evidence_id, source_evidence_digest, acceptance_contract_digest,
deterministic_result_digest, verifier_candidate_digest,
acceptance_decision_digest
```

For `accepted`, `WorkItemDone` follows on the same WorkItem stream and contains
exactly:

```text
work_item_id, run_id, claim_generation, status="done",
verification_event_id, acceptance_decision_digest,
source_evidence_id, source_evidence_digest,
verifier_evidence_id, verifier_evidence_digest
```

Its causation is the verification Event and its deterministic ID binds:

```text
WorkItemDone, work_item_id, verification_event_id,
acceptance_decision_digest, source_evidence_digest, verifier_evidence_digest
```

For `rejected`, `WorkItemRejected` instead follows on the same stream and
contains exactly:

```text
work_item_id, run_id, claim_generation, status="rejected",
verification_event_id, acceptance_decision_digest,
recovery_trigger="verification_rejected",
recovery_policy_version, recovery_policy_digest,
attempt_number, max_attempts, credits_before
```

Its deterministic ID binds every listed field plus the verification Event ID.
It intentionally contains no action or `retry_at`: the frozen RecoveryPolicy,
not acceptance, owns those values.

`TeamNodeAcceptanceCommitted` is written to
`team-execution/<team_instance_id>` with causation equal to the WorkItem outcome
Event. It contains exactly:

```text
team_instance_id, plan_digest, logical_node_id, attempt_number,
work_item_id, work_outcome_event_id,
acceptance_contract_version, acceptance_contract_digest,
acceptance_decision_kind, acceptance_decision_digest, decided_at,
node_status, dependency_satisfied,
recovery_trigger, recovery_policy_version, recovery_policy_digest,
credits_before
```

Accepted stores `node_status="succeeded"`,
`dependency_satisfied=true`, and empty recovery fields. Rejected stores
`node_status="awaiting_recovery"`, `dependency_satisfied=false`,
`recovery_trigger="verification_rejected"`, and exact frozen RecoveryPolicy
version/digest plus credits. Its deterministic ID binds every listed identity,
outcome, decision, and recovery field.

Any resulting `TeamExecutionTerminal` retains the existing exact payload and
identity rules and is caused by `TeamNodeAcceptanceCommitted`.

## Journal acceptance authority

`work.Authority.CommitTeamNodeAcceptance` is the only new acceptance writer. It
accepts the exact immutable `AcceptanceDecision` plus Store-returned source and
optional verifier receipts. It uses `ReadStreamSet` over only the touched:

- Team execution;
- source WorkItem/Run/Evidence;
- verifier WorkItem/Run/Grant/Evidence when required; and
- Runtime status/capacity streams needed to validate the verifier terminal.

It reconstructs the authoritative state and validates:

- source WorkItem is exactly `ready_for_review`;
- exact Team/plan/node/attempt and frozen AcceptanceContract binding;
- exact source S4-W2 classification and Evidence receipt;
- exact deterministic-result digest;
- risk route and required absence/presence of verifier lineage;
- verifier WorkItem/Run/generation/Agent/Runtime/Grant/Evidence binding,
  exact terminal status/reason, Store-returned receipt/summary, recorded Grant
  authorization, Candidate digest, and distinctness;
- decision time, rejection trigger, frozen RecoveryPolicy, attempt limit, and
  current credits;
- current stream heads and terminal-once state.

One `AppendBatchIfStreamHeads` transaction records:

1. `WorkItemVerificationCommitted` with contract/result/decision and source plus
   optional verifier digest lineage;
2. exactly one `WorkItemDone` for accepted, or `WorkItemRejected` for a bounded
   verification rejection;
3. `TeamNodeAcceptanceCommitted` with the same decision digest and resulting
   node status; and
4. `TeamExecutionTerminal` only if all nodes are terminal after acceptance.

Accepted means WorkItem status exactly `done`, node status `succeeded`, and
dependency satisfaction true. `done` is terminal-once. Rejected means source
WorkItem `rejected` and Team node `awaiting_recovery`; it does not itself
schedule anything.

The exact acceptance CAS head set is:

```text
team-execution/<team>,
work-item/<source>, run/<source>, evidence/<source>,
and, when verifier_required:
work-item/<verifier>, run/<verifier>, agent-grant/<verifier_run>,
evidence/<verifier>, runtime_instance:<verifier_runtime>,
runtime_capacity:<verifier_runtime>
```

Every stream read to validate the command has an exact
`StreamHeadExpectation`, including read-only source/verifier Run, Grant,
Evidence, Runtime status, and capacity streams. Only the source WorkItem and
Team streams receive new Events. Missing, extra, stale, or changed heads fail
before append. The transaction never uses `ReadAll`.

The exact same command returns the existing result without new Events.
Divergent, stale, concurrent, forged, or reordered commands return a typed
conflict with zero partial Events. Exact idempotency requires identical
correlation ID, complete Event payloads, Event IDs, source/verifier receipts,
contract/result/Candidate/decision digests, and resulting Work/Team states.
Two concurrent acceptors produce exactly one verification fact and one
terminal consequence.

## Rejection recovery handoff

After a rejected acceptance CAS, coordinator restart/re-entry reads the Team
node `awaiting_recovery` plus its persisted:

```text
recovery_trigger="verification_rejected",
acceptance_decision_digest,
recovery_policy_version/digest,
attempt_number/max_attempts/credits_before
```

It reconstructs `rules.RecoveryInput` with the exact source attempt,
classification/Evidence lineage, prior classifications, remaining credits,
frozen RecoveryPolicy, AcceptanceDecision digest, and authoritative current
time. `rules.DecideRecovery` returns the normal sealed `RecoveryDecision`.

For a verification rejection:

- `retry` is permitted only when attempts and credits remain and sets exact
  `retry_at = decision_time + RecoveryPolicy.retry_delay`;
- workflow fallback is always rejected;
- exhaustion yields the RecoveryPolicy's frozen `degraded`, `blocked`, or
  `human_required`;
- the RecoveryDecision binds
  `recovery_trigger="verification_rejected"` and the exact
  AcceptanceDecision digest.

`ScheduleTeamNodeRecovery` validates those new bindings in addition to all
existing S4-W2 fields, writes the existing `TeamNodeRecoveryRecorded` fact with
the added trigger/AcceptanceDecision digest, and, for retry, schedules the
normal distinct next WorkItem/Run lineage. Its existing CAS, due-time,
generation, credit, and exact-idempotency rules remain authoritative.

A crash after `WorkItemRejected`/`TeamNodeAcceptanceCommitted` but before the
recovery fact replays the persisted handoff and writes exactly one recovery
fact. A crash after the recovery fact returns exact idempotent state. A changed
policy, decision digest, time, credits, action, or `retry_at` conflicts with no
duplicate attempt.

## Projection and read view

The existing Projection and `GlobalReadView` expose copied:

- WorkItem verification status and terminal acceptance status;
- AcceptanceContract version/digest and risk;
- deterministic result digest;
- verifier WorkItem/Run/Agent/Evidence identities when present;
- AcceptanceDecision kind/digest/time/reason code;
- recovery trigger and AcceptanceDecision-to-RecoveryDecision handoff;
- Team node `ready_for_review`, `succeeded`, rejection/recovery, and terminal
  aggregation.

Replay validates exact causation, stream sequence, digest agreement, distinct
source/verifier lineage, accepted-only Done, terminal-once, and Team/Work
agreement. Historical S3/S4-W2 facts remain readable without invented
acceptance. Candidate-first rebuild failure preserves the previous immutable
view. Accessors deep-copy every requested record/slice.

Projection, view version, timeline, notifications, and client presence never
authorize verification or completion.

## Mandatory RED

Before production behavior changes, tests must compile or fail behaviorally for:

1. immutable AcceptanceContract validation/digest/mutation isolation;
2. valid source output remaining `ready_for_review`, never Team success;
3. low-risk deterministic acceptance and terminal-once Done;
4. medium/high distinct verifier WorkItem/Run/Grant/Evidence lineage;
5. verifier Candidate exact decoding and stale/unauthorized rejection;
6. verified rejection routed to explicit retry/exhaustion outcomes;
7. concurrent acceptance one-winner and zero partial mutation;
8. restart recovery without duplicate verifier Run/Evidence/Done;
9. Projection/view exact replay, deep copy, and old-view failure preservation;
10. controlled whole-Slice approval/restart/recovery/verification/Done canary.

Static marker tests may supplement but cannot replace behavioral RED.

## Acceptance tests

The Candidate must prove:

1. low-risk exact deterministic acceptance reaches WorkItem `done` and Team
   node `succeeded` once, with no verifier lineage;
2. medium/high cannot complete without a distinct AgentInstance plus distinct
   verifier WorkItem/Run/Grant/Evidence;
3. executor, verifier, caller, stale Projection, and malformed Candidate cannot
   write accepted/rejected/Done;
4. source and verifier receipts, classification, contract, result, decision,
   generation, Grant authorization, and Team binding are mutation-tested;
5. rejection retry is explicit, due-time fenced, bounded, uses a distinct next
   attempt lineage, and exhaustion produces the frozen terminal result;
6. concurrent acceptance has one CAS winner or exact idempotent replay, never
   duplicate facts or partial streams;
7. crash/reopen at verifier dispatch, receipt finalized, verification metadata
   committed, and Work/Team acceptance committed does not duplicate execution,
   Evidence, or Done;
8. Team dependencies do not unlock at `ready_for_review`; they unlock only
   after accepted Done;
9. Projection malformed/missing/duplicate/reordered verification facts fail
   candidate rebuild and preserve the old view;
10. Journal/Projection contain only bounded digests/IDs/status/reason codes, not
    raw output, raw Grant, credentials, prompts, hidden reasoning, or personal
    identifiers;
11. controlled local SQLite/Supervisor canary proves approval restart,
    valid-empty handling, transient-empty retry, retry exhaustion, stale
    generation rejection, independent verifier isolation, terminal-once Done,
    and Projection failure preservation;
12. no dependency, daemon/API/CLI, real Runtime/Provider, checkpoint, S4-W4,
    Slice 5, Phase 2, or external action is added.

## Required checks

Focused:

```text
go test ./internal/verification ./internal/rules ./internal/work ./internal/app ./internal/projection -count=1
go test -race ./internal/verification ./internal/rules ./internal/work ./internal/app ./internal/projection -count=10
```

Impact:

```text
go test ./internal/evidence ./internal/authorization ./internal/supervisor ./internal/runtime/piadapter ./internal/teams -count=1
```

Repository:

```text
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
```

Static/platform:

```text
gofmt -d <all S4-W3 owned Go files>
git diff --check
GOOS=windows GOARCH=amd64 go build ./internal/verification ./internal/rules ./internal/work ./internal/app ./internal/projection
```

Audits must show:

- only exact owned files changed/staged;
- no `go.mod`/`go.sum` or dependency change;
- one Work acceptance writer and no second Journal/Projection/Team authority;
- no raw secret/Grant/output/reasoning/personal data in Events or Projection;
- no hidden retry, Provider/model fallback, autonomous loop, network, daemon,
  API/CLI/Web/TUI, or external side effect;
- complete RED, GREEN, integration, restart, concurrency, mutation, projection,
  and Reviewer evidence.

After every check passes, a fresh independent implementation Reviewer must
return `PASS`. The Developer may report only `ready_for_review`; the Controller
then accepts and creates one local atomic S4-W3 commit.

## Trust and activation boundary

All execution proof uses deterministic in-process Supervisor adapters, fake
clocks, private temporary Evidence roots, and local SQLite. It does not install
or contact a Runtime/Provider, authenticate a user, activate a daemon or
autonomous execution, expose an API/CLI/Web/TUI, send notifications, use
credentials, or perform a paid/network/external action.
