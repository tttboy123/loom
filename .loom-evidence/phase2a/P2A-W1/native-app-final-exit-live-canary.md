# P2A-W1 Native App Final Exit Controlled Live Canary

**Date**: 2026-07-29  
**Result**: `FAIL — ROLLED_BACK — HUMAN_REQUIRED`  
**Allowance remaining**: `0`  
**Initial Candidate bootstrap calls**: `1`  
**Native app launch calls**: `1`  
**Native Refresh actions**: `1`  
**Native relaunch calls**: `0`  
**Candidate lifecycle restart calls**: `0`  
**Rollback count**: `1`

## Activation and preflight

The user supplied the exact Final Exit Closure activation phrase. The
post-Review audit was atomically changed from allowance `0` to `1`.

Before mutation, the reviewed harness revalidated:

- harness SHA-256
  `724e35f2508e8fa41c8ab4d5dc97854349ff3fcb01ac1af274e42eef9db90358`;
- exact Candidate `loom`, `loomd`, native executable, arm64 LC_UUID, canonical
  bundle manifest, strict signature, owner, and private modes;
- exact original product/plist hashes and modes;
- original observer loaded;
- exact one-Event SQLite bytes and integrity;
- exact two-report historical crash inventory;
- absent Candidate live paths/native process/Swift cache;
- empty Git staging.

No Provider, model, Runtime execution, Team creation, Journal write,
credential mutation, or authority transition occurred.

## Consumed bootstrap

The exact reviewed transaction:

1. created and validated the owned mode-`0700` product run directory;
2. installed the exact Candidate product and native app;
3. quiesced the original observer;
4. invoked the one allowed Candidate `launchctl bootstrap`;
5. immediately recorded `initial_bootstrap_calls=1` and `consumed=1`;
6. proved Candidate service/socket/status, exact hashes, secret-negative
   process predicate, unchanged Journal, unchanged crash inventory, and empty
   staging.

The harness then emitted:

```text
FINAL_EXIT_READY initial_bootstrap_calls=1 consumed=1 rollback_count=0 restart_count=0
```

There was no readiness-to-bootstrap backedge and no retry.

## Native-window failure

Computer Use targeted only:

```text
com.earendilworks.loom.local
```

The first independently addressable native Home window showed:

```text
Connection status: Offline: invalid_response
Runtimes 0
Teams 0
Runs 0
Attention 0
```

The frozen required Home result was `Runtimes 1`, with the real
`Pi 0.82.1 Resident Demo` Runtime. One bounded native Refresh transitioned
through `Loading` and returned to the same `Offline: invalid_response` result.

No Terminal, Finder, CLI, PTY, archived screenshot, alternate socket, other
application, or Provider surface was used. No credential, internal path, raw
identifier, cursor, hidden reasoning, or Provider material appeared in the
native accessibility evidence.

Because the first required screen failed, no remaining screen, empty-Team
activation, native relaunch, or Candidate lifecycle restart was attempted.
The controller sent `ui_failed` to the same reviewed harness.

## Closed causal diagnosis

Read-only inspection after exact rollback established one deterministic schema
incompatibility:

1. the sole authoritative Journal fact is `RuntimeInstanceDiscovered`;
2. its `payload_json` is valid JSON and its `model_ids` member has JSON type
   `null`;
3. `buildLocalProductSnapshot` copies the nil model list with
   `append([]string(nil), runtime.ModelIDs...)`, which remains a nil Go slice;
4. Go `encoding/json` therefore emits `"model_ids":null`;
5. `LocalProductRuntimeSummary.init(from:)` performs required
   `decode([String].self, forKey: .modelIDs)`, which accepts an array but not
   `null`;
6. `LocalProductStore` maps that decode failure to the closed
   `invalid_response` reason displayed by the native Home view.

The existing real Go-server/Swift-client contract test uses a hard-coded
non-null fixture:

```text
"model_ids":["qwen"]
```

It proves framing/envelope interoperability but does not cover the
authoritative live nil-to-null representation. The Go CLI snapshot preflight
also does not prove Swift strict decoding because Go accepts a JSON `null`
slice.

This is a product schema-normalization/test-coverage defect, not a launch,
LC_UUID, run-directory, socket, service-manager, Journal-integrity, Provider,
Runtime, or Computer Use defect.

## Exact rollback

The harness returned:

```text
FINAL_EXIT_RESULT result=rolled_back initial_bootstrap_calls=1 consumed=1 rollback_count=1 restart_count=0
```

Fresh post-rollback proof:

| Predicate | Result |
|---|---|
| original `loom` hash / mode | exact / `0755` |
| original `loomd` hash / mode | exact / `0755` |
| original wrapper hash / mode | exact / `0700` |
| original plist hash / mode | exact / `0600` |
| original observer | running from exact wrapper |
| target-process five-marker count | `0` |
| SQLite hash / mode | exact / `0600` |
| SQLite integrity / Event count | `ok` / `1` |
| historical crash inventory | exact two reports |
| Candidate app/run/socket/launcher/backups | absent |
| native process | absent |
| Swift cache | absent |
| Git staging | empty |

The harness captured and restored the original provenance values
byte-for-byte.

## Terminal boundary

The sole Final Exit Closure allowance is consumed. Result-Evidence Review may
audit this record and current state but cannot authorize:

- another bootstrap, restart, app launch, or replacement canary;
- a product-source fix or new point closure;
- commit or P2A-W2;
- Provider/Runtime execution, push, merge, release, or publish.

Fresh independent Result-Evidence Review returned `PASS` with no findings. It
accepts only this failed-live evidence and exact rollback; every prohibition
above remains in force.
