# P2A-W3 Native Launcher Build Methodology Amendment

**Date**: 2026-08-02  
**Status**: `FROZEN / PENDING INDEPENDENT REVIEW`  
**Parent contract SHA-256**:
`4493858a9be2411ba6cac1594df7bfdc0e4e7be062f5ec528b7c3746c7d30b18`  
**Authority/schema expansion**: none  
**New WorkItem**: none; `P2A-W4` does not exist

## Causal reason

Mandatory RED proved that `NewLocalRuntimeObservationDaemon` binds a non-nil
`LocalModelCatalog` before setup/execution/IPC and requires the production-frozen
model SHA-256. A small hermetic model fixture therefore cannot traverse the
real execution-enabled production builder. Reopening Pi adapter production or
test authority would be broader and less representative than using the exact
already materialized component.

## Amended verification method

The parent contract's phrase "deterministic local-model catalog fixture" is
replaced only for the execution-enabled product construction gate by a
controlled, source-locked local component dependency:

```text
private root:
  /Users/lune/Library/Application Support/Loom/phase1-live
model:
  models/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf
model SHA-256:
  cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046
model size:
  1117320768
server:
  runtime/llama-b10107/llama-server
server SHA-256:
  a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b
```

The ordinary repository matrix remains hermetic: without
`LOOM_P2A_W3_LOCKED_MODEL_ROOT`, the copied-state test exercises the no-execution
path and does not claim the component gate. The Controller must separately run
the exact execution-enabled test with:

```text
LOOM_P2A_W3_LOCKED_MODEL_ROOT="/Users/lune/Library/Application Support/Loom/phase1-live"
go test -count=1 ./cmd/loomd -run TestProductDaemonCopiedRuntimeStateObservesPi0821WithoutRewrite
```

When the variable is present the test must fail, not skip, for missing paths,
wrong ownership/mode/type, symlinks, size/hash drift, builder failure, missing
execution composition, socket failure, Journal mutation, process/retry residue,
or cleanup failure. It must use the exact retained six-fact SQLite and official
npm-launcher fixture, reach the real Go IPC server, and remain zero-write.

The gate only reads and hashes the model/server during binding. It must not
start llama-server, Pi, native UI, Keychain, Provider, Mission or any live
canary. Ordinary full/race/platform gates and the independent Implementation
Review remain mandatory in addition to this component gate.

## Scope and stop rules

- No Pi adapter file, digest constant, runtime authority or test hook changes.
- Owned files remain exactly those in the parent contract.
- No model copy, permission change, xattr change, network access or process
  activation.
- Missing/drifted component means `HUMAN_REQUIRED`; it must not fall back to a
  fake digest or silently skip the mandatory Controller command.
- This amendment authorizes no live manifest, retry, walkthrough, stage or
  commit.

Until independent review PASS:

```text
P2A-W3 = HUMAN_REQUIRED / METHODOLOGY AMENDMENT REVIEW PENDING
NO LIVE / NO COMMIT / NO P2A-W4
```
