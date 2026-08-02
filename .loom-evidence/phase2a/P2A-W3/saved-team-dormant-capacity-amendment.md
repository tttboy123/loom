# P2A-W3 Saved-Team Dormant Capacity Amendment

**Date**: 2026-08-02  
**Parent contract**: `P2A-W3 Controlled Execution Experience`  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Branch**: `codex/loom-platform-slice2`  
**Risk**: Strict accepted-authority reopen  
**Status**: `FROZEN / CONTRACT REVIEW PENDING`  
**Implementation**: `LOCKED`  
**Live action**: `LOCKED`

## 1. Authorization and reason for reopening

The Product Owner explicitly authorized:

> 授权冻结并评审 P2A-W3 Saved-Team Dormant Capacity Amendment；仅重开
> saved_team_binding authority 及测试。

The post-review live-precondition audit proved a vertical contradiction:

- accepted Builder confirmation persists a TeamDefinition but no TeamInstance;
- the accepted Saved-Team instance plan creates exactly one Main AgentInstance;
- every SubAgent in that plan is dormant and creates no AgentInstance;
- installed Pi truthfully advertises Runtime capacity `1`; but
- `BuildSavedTeamRuntimeBinding` counts Main plus dormant SubAgent selections as
  materialization-time usage `2`, so the confirmed-Team product path cannot be
  materialized.

This is not permission to bypass Runtime capacity. It is a bounded correction
to what consumes capacity during direct saved-Team materialization.

## 2. Decision

For the accepted direct saved-Team materialization path:

1. exactly one Main role is active-on-materialization and consumes one unit of
   advertised Runtime capacity;
2. zero, one, or two SubAgent roles remain dormant metadata and consume zero
   materialization-time capacity because no SubAgent AgentInstance, Run, lease,
   claim, process, or dispatch is created;
3. every Main and dormant SubAgent selection must still resolve and pass the
   exact TeamDefinition, AgentDefinition, RuntimeProfile, RuntimeInstance,
   Runtime status, adapter, model and capability checks;
4. the immutable Candidate must still retain every selected role binding and
   its discovery/binding digest; and
5. validation must rebuild the Candidate from current sources and reject
   tampering or source drift exactly as before.

The implementation may change only the capacity-demand calculation inside the
saved-Team binding authority. It must not change public function signatures,
Candidate fields, digest fields, normalization, errors or selection rules.

## 3. Authority boundary

Only these accepted product/test files are reopened:

- `internal/teams/saved_team_binding.go`
- `internal/teams/saved_team_binding_test.go`

Governance evidence may change only under:

- `.loom-evidence/phase2a/P2A-W3/**`
- `docs/CURRENT.md`

All other parent W3 product files remain byte-locked until this Contract Review
passes. After PASS, the existing parent W3 ownership may resume only for Repair
3; this Amendment itself grants no new product-file ownership.

## 4. Preserved invariants

The Amendment must preserve all of the following:

- Runtime discovery remains authoritative for the positive capacity value;
- `runtime.NewRuntimeInstance` and `runtime.ValidateBinding` remain unchanged;
- the Main still requires one unit of materialization-time capacity;
- no capacity value is raised, synthesized, decremented, reserved or persisted;
- no dormant SubAgent is activated or represented as an AgentInstance;
- no execution-time, dispatch-time or active-Run capacity rule changes;
- later SubAgent activation, if separately authorized in the future, must obtain
  a fresh current binding and pass the existing dispatch/CAS/capacity and
  generation-fencing authorities at activation time;
- one-writer Journal, StateWriter, Projection, TeamExecution coordinator, Grant,
  Supervisor, Evidence, Runtime adapter and bridge boundaries remain unchanged;
- failure remains zero-Candidate and fail-closed; and
- P2A-W4 does not exist.

The existing `ErrSavedTeamRuntimeCapacityExceeded` API remains intact. Under the
accepted exactly-one-Main saved-Team shape and valid positive Runtime instance,
the materialization demand is one, so a valid source cannot under-advertise that
demand. Invalid or zero capacity still fails earlier through the accepted
Runtime instance validation.

## 5. Rejected alternatives

- Do not raise Pi capacity from `1` to `2`.
- Do not invent a second RuntimeInstance or duplicate discovery observation.
- Do not seed TeamInstance/AgentInstance facts directly in a fixture or SQLite.
- Do not append raw Journal Events or bypass `CommitSavedTeamInstanceRecordSet`.
- Do not count dormant definitions as active processes or capacity reservations.
- Do not weaken model, status, adapter, capability, identity, digest or current-
  source validation.
- Do not add an activation flag, capacity field, second authority or new schema.

## 6. Mandatory RED

Before production behavior changes, tests in the reopened test file must prove:

1. Main plus one dormant SubAgent on the same capacity-1 Runtime succeeds and
   preserves both exact bindings;
2. Main plus two dormant SubAgents on the same capacity-1 Runtime succeeds and
   preserves all three exact bindings;
3. the unchanged implementation fails those tests only with
   `ErrSavedTeamRuntimeCapacityExceeded`;
4. Main-only binding on capacity 1 remains valid;
5. offline, incompatible, disabled, model-missing, adapter-mismatched,
   capability-missing, malformed and identity/source mismatch cases still fail;
6. current-source validation accepts the capacity-1 dormant binding, rejects
   source/tamper drift, and returns zero output on failure;
7. input/discovery capacity is not mutated or reserved; and
8. the import/trust boundary remains standard library plus accepted
   `internal/agents` and `internal/runtime` contracts.

The obsolete expectation that capacity `2` must reject a three-role saved Team
must be replaced, not silently deleted: its causal history is preserved in the
parent S2-W11 contract and the P2A-W3 live-precondition audit.

## 7. Deterministic gates after implementation

At minimum:

- focused RED/GREEN for `BuildSavedTeamRuntimeBinding` and
  `ValidateSavedTeamRuntimeBinding`;
- full `go test ./internal/teams -count=1`;
- focused repeated race at least 50 times;
- whole-repository `go test ./... -count=1`, `go test -race ./... -count=1`,
  `go vet ./...` and format/diff checks;
- exact scope/import/secret checks;
- rerun the complete parent P2A-W3 deterministic matrix after Repair 3; and
- obtain a fresh independent P2A-W3 Implementation Review PASS for the new
  source lock before freezing any live manifest.

No live action is authorized by Contract Review PASS alone.

## 8. Exit and stop conditions

Contract Review must return `FAIL` if the decision can weaken active execution
capacity, permits a dormant role to run without fresh authority, or requires
any product/test path outside the two reopened files.

Implementation stops `HUMAN_REQUIRED` if the two-file change cannot preserve all
binding/source/digest validation, if another accepted authority/schema must
change, or if the parent W3 vertical path still requires a bypass.

Rollback is the two-file Amendment diff plus the later single atomic W3 commit;
no Journal history, accepted Team, credential, Runtime state or unrelated dirty
path may be deleted or rewritten.

## 9. Reviewer questions

The independent Reviewer must answer:

1. Is the active-on-materialization versus dormant distinction exact and
   consistent with accepted S2-W12/S2-W13 behavior?
2. Does any wording weaken dispatch/run-time capacity or future activation
   checks?
3. Are two product/test files sufficient, with no hidden schema or caller
   changes?
4. Do the RED and verification requirements preserve every original binding
   trust check?
5. Is the Amendment safe to implement with `live action = locked`?

**VERDICT**: `FROZEN / CONTRACT REVIEW PENDING`
