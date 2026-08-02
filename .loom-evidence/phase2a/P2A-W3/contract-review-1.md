# P2A-W3 Contract Review 1

**Date**: 2026-08-01  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Reviewed contract SHA-256**:
`3c51239566af97e87d48567603a18c8d0f11f8082a400ced8132e458b111dddd`  
**Reviewer**: fresh independent read-only Reviewer  
**Verdict**: `FAIL`

## P0

None.

## P1

1. The contract incorrectly claimed authoritative WorkPackage/objective lineage
   and restart reconstruction although accepted `TeamExecutionPlanned` facts
   contain Team/plan/node/semantic facts, not WorkPackage ID/digest or objective
   digest. The owned boundary excluded the authority/Event/projection files
   that would be needed to add those facts.
2. The mandatory real Swift IPC verification could not cover the new
   `mission_execution` method because
   `apps/macos/Sources/LoomLocalAppContractProbe/main.swift` was not owned.
3. PX-02 required new product-daemon/Journal health presentation, but
   `internal/api/local_product_read.go`, its tests, and the strict Swift product
   model files were not owned.

## P2

- Prepared approval/claim-fence pause, daemon-context cancellation, and typed
  deterministic recovery are conceptually implementable without a new generic
  paused state.
- The three distinct live claims avoid overclaim: Codex native-auth and MiniMax
  brokered verification are product-path canaries; only Pi is an execution
  adapter canary.

## Required repair

- Keep WorkPackage as a typed pre-start proposal and make clear that durable
  execution authority begins at the accepted TeamExecution plan/dispatch facts.
- Reconstruct only exact plan/node/semantic/runtime recipes already frozen in
  Journal/catalog state; never infer a WorkPackage lineage after start.
- Add the strict Swift contract probe and PX-02 read/wire/model files and tests
  to exact ownership and acceptance coverage.
- Obtain a fresh independent Contract Repair 1 re-review before RED.
