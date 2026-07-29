# P2A-W1 Native App Final Exit Result-Evidence Review

**Date**: 2026-07-29  
**Verdict**: `PASS`  
**Findings**: none  
**Review mode**: fresh, independent, read-only  
**Live authority**: none

## Evidence sufficiency

The failure record is coherent and sufficient.

The Reviewer independently confirmed the closed causal chain:

1. the authoritative Journal payload is valid JSON;
2. its `model_ids` member has JSON type `null`, while
   `observed_capabilities` is an array;
3. runtime discovery accepts JSON `null` into a nil Go slice and its clone
   helper preserves nil;
4. the local product read model preserves that nil model list, so Go emits
   `"model_ids":null`;
5. the Swift runtime summary requires `[String]` and rejects `null`;
6. the Swift store maps the decode failure to the closed
   `invalid_response` reason observed in the native Home view;
7. the existing real Go-server/Swift-client contract fixture hard-codes
   `"model_ids":["qwen"]` and therefore does not cover the authoritative
   live nil/null shape.

This proves a product schema-normalization and cross-language test-coverage
defect. It does not support a launch, LC_UUID, run-directory, socket,
service-manager, Journal-integrity, Provider, Runtime, or Computer Use cause.

## Invocation accounting

The Reviewer confirmed:

```text
initial_bootstrap_calls=1
consumed=1
rollback_count=1
restart_count=0
retry=0
```

The successful initial Candidate bootstrap consumed the one allowance before
readiness. `ui_failed` made native relaunch and the Candidate lifecycle-restart
branch unreachable. No backedge or second bootstrap exists.

## Independent rollback proof

Every read-only predicate matched:

- exact original `loom`, `loomd`, wrapper, plist, and SQLite hashes/modes;
- observer running from exact `loomd-clean`;
- target-process five-marker count `0`, without emitting a process row or
  environment value;
- SQLite integrity `ok`, one Event, payload/model JSON types `object/null`;
- exact two historical crash hashes;
- Candidate app, run root, socket, launcher/backups, native process, and Swift
  cache absent;
- Git staging empty;
- working and cached diff checks pass;
- current transaction hash matches the reviewed canary hash.

The harness's terminal `rolled_back` result also proves its captured
provenance values were restored and compared byte-for-byte before it emitted
success.

## Authority

This `PASS` accepts only the failed-live evidence and exact rollback. It does
not accept P2A-W1 product delivery and authorizes no:

- retry, replacement canary, bootstrap, restart, or app launch;
- source change, new point closure, or silent contract expansion;
- commit or P2A-W2 work;
- Provider/Runtime execution, push, merge, release, or publish.

Terminal status remains:

```text
P2A-W1: FAIL — ROLLED_BACK — HUMAN_REQUIRED
allowance: 0
P2A-W2: LOCKED
```

