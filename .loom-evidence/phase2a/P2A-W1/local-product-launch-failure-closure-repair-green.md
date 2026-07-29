# P2A-W1 Local Product Launch Failure Closure GREEN

**Date**: 2026-07-29  
**Status**: `GREEN - COMPLETE DETERMINISTIC MATRIX PASS`  
**Production bootstrap count**: `0`  
**Native App launch count**: `0`  
**Live allowance**: `0`

## Causal implementation

The repair stays inside the one reviewed P2A-W1 vertical boundary:

- `productDaemonRunner` classifies server-before-ready and unexpected server
  termination as `local_ipc`;
- non-context observer failures are `observer`;
- close/database/expected-stop cleanup failures are `shutdown`;
- the CLI maps only the closed codes and maps unknown internal errors to
  `shutdown`;
- result encoding is `result`;
- raw errors remain process-internal and never enter daemon stderr.

The switch transaction now:

- boots out the original observer and proves absence before changing installed
  executable or plist bytes;
- creates the run root only after absence;
- runs a bounded private one-cycle Candidate on the exact production socket
  path before installation;
- binds a fresh source lock, Candidate manifest, artifacts, App UUID, signed
  bundle manifest, and private log paths;
- sets Candidate `KeepAlive=false`, preventing launchd from turning a
  pre-ready failure into a hidden retry;
- performs exactly one initial bootstrap;
- atomically writes one uid `501`, mode `0600`, no-replace, symlink-refusing
  closed reason JSON on a pre-ready bootstrap/readiness failure;
- rolls back immediately and retains only that bounded reason record.

No IPC schema, Journal fact, Projection, StateWriter, installer, Swift source,
App source, Provider/Runtime adapter, credential, accepted ADR, or P2A-W2 file
changed.

## RED to GREEN trace

The preserved RED proved:

- server-before-ready and observer-after-ready errors were unclassified;
- observer, local IPC, unknown, and close failures all emitted
  `daemon failed`;
- result encoding emitted the same generic text;
- the transaction had no governed reason record, exact-path boundary, or
  original-absence-before-install ordering.

Focused GREEN:

```text
go test ./cmd/loomd -run \
  'TestRunWritesClosedDaemonFailureReasonCodes|TestRunClassifiesResultEncodingWithoutDisclosingWriterError|TestProductDaemonClassifiesLifecycleFailureBoundaries' \
  -count=1
PASS

go test ./cmd/loomd ./internal/localipc
PASS

sh local-product-live-closure-transaction-test.sh
local product closure transaction fixture PASS
```

The transaction fixture additionally proves:

- order is `original_absent → exact_path_preflight → candidate_install →
  bootstrap`;
- readiness failure records reason before rollback;
- reason JSON has exactly the nine frozen keys and closed values;
- pre-existing and symlinked reason targets are rejected without modification;
- the success path has one initial bootstrap and one explicit later restart;
- no reason record exists on success;
- Candidate plist logs are private and `KeepAlive=false`.

## Complete deterministic matrix

All valid sequential commands passed:

```text
go test -count=1 ./...
PASS

go test -race -count=1 ./...
PASS

go vet ./...
PASS

go mod tidy -diff
PASS - no output

go mod verify
all modules verified

cd apps/macos && swift test
27 tests, 0 failures, 1 governed visual-export skip

cd apps/macos && swift build -c release
PASS

cd apps/macos && swift test --sanitize=thread
27 tests, 0 failures, 1 governed visual-export skip

scripts/test-build-loom-local-app.sh
native app build fixture PASS

scripts/test-install-loom-local-app.sh
native app installer fixture PASS

scripts/test-install-loom-local-product.sh
installer fixture PASS

sh -n transaction and installer scripts
PASS

transaction fixture
PASS

gofmt, credential-value, environment-read, mutation-authority,
reason-absence, git diff, and empty-staging audits
PASS
```

## Verification resource-interference incident

One invalid matrix invocation ran ordinary full Go, race full Go, and Swift
debug tests concurrently. The two Go matrices both failed only in
`internal/app` real-process fixtures at the same Pi metadata `version`
timeout. During that resource saturation, the pre-existing resident observer
also exceeded its frozen ten-second process timeout; its original
`KeepAlive=true` policy restarted it until launchd reached runs `18`, PID
`97912`, last exit code `4`.

This was not hidden or reclassified as a product test result. No launchctl
command, installed byte, plist, provenance value, Journal byte/Event, report,
App, run root, Provider/Runtime authority, credential, or staging changed.
After the concurrent processes ended:

- ordinary full Go passed sequentially;
- race full Go passed sequentially;
- Swift debug/release/Thread Sanitizer passed sequentially;
- the resident stayed PID `97912`, runs `18`, state `running` across a
  15-second observation and a later 5-second audit;
- installed and Journal hashes remained frozen and SQLite stayed integrity
  `ok` with one Event.

The fresh Implementation Reviewer must decide whether this disclosed
environmental continuity drift is compatible with the contract. It cannot be
silently converted into live authority.

## Fresh repaired source and Candidate identity

Source lock:

```text
path=local-product-launch-failure-repair-source-lock.json
sha256=333097f377427eda8ef5c6c0d2a3fc4da47a3075a7f67a93c8637441ad679570
mode=0600
owner_uid=501
locked_inputs=53
owned_delta=4
```

Candidate manifest:

```text
path=local-product-launch-failure-repair-candidate-manifest.json
sha256=ca47404f4761a26a6f9bfcc8ce18091832ea59314bd85a736ef2e8e3d2a00a58
mode=0600
owner_uid=501
```

The retained fresh private Candidate root is uid `501`, mode `0700`, contains
only the Candidate, and has no symlink, Journal copy, credential, Provider
environment, raw model output, or hidden reasoning. Build caches were removed.

| Artifact | Exact identity |
|---|---|
| `loom` | `b12e5261632efc10585163ea323a22951f736b5565b5a717b0cf77a2765d0f29` |
| `loomd` | `af29fbb9cfa46ac0b97321a3240c9e7fd4048f0f64ce55cfd1b139a1a691d6e6` |
| App executable | `3aedc0ba90be3cdfb3df0865016abebbc9cc5b659bfa6a445ae4a54af272cfba` |
| arm64 `LC_UUID` | `93E3FC61-0A19-3379-BFDB-8CA7726F59F6` |
| canonical signed bundle manifest | `bdbf57511fbbcd56c583af3e2bb84f03d45f2b012b4a1eb6a5f087203e2d4a58` |

Transaction identity:

```text
transaction_sha256=089aa7d92d9dfdea73b02c56449e0f07532af11229fc0661af2025e819351d1d
fixture_sha256=11e3e9920f0dc9baf7824528bd48b200a55bb293f837456b015e149b73d02336
```

The transaction no longer contains the retained failed Candidate path. It
binds the fresh repair source lock and Candidate manifest. The absent
replacement activation audit leaves the production entry fail-closed:

```text
exit_code=20
LOCAL_PRODUCT_CLOSURE_RESULT result=preflight_failure initial_bootstrap_calls=0 consumed=0 rollback_count=0 restart_count=0
```

The first evidence wrapper for this zero-bootstrap check used zsh's read-only
`status` variable after the transaction returned; no result was retained from
that wrapper. A corrected wrapper reproduced the exact output above. Both
executions stopped before mutation with zero bootstrap and zero allowance.

## Gate

Fresh independent Implementation Review is required. This GREEN grants no
activation record, production install, bootstrap, restart, native-window
action, live allowance, commit, acceptance, or P2A-W2 work.
