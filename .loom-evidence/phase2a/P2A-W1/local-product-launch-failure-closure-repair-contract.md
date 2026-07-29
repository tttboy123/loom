# P2A-W1 Local Product Launch Failure Closure Repair Contract

**Date**: 2026-07-29  
**Status**: `FROZEN - COMPLETE PRE-READY ATTRIBUTION AND RECOVERY SETTLING AMENDMENT - CONTRACT RE-REVIEW 5 PASS`  
**WorkItem**: existing `P2A-W1 Local App Shell and Read Experience`  
**New WorkItem**: none  
**Historical live result**: `FAIL - ROLLED_BACK - HUMAN_REQUIRED`  
**Historical allowance**: `0`  
**Production live authority before Implementation Review**: none

## 1. Authority and purpose

The active Phase 2A goal requires P2A-W1 to install, launch, exit, and restore
the native local product without a terminal. The user's standing instruction
authorizes continuation without repeated confirmation. That authority does not
allow the consumed live transaction to be retried or its failure to be
reinterpreted.

This is the sole vertical repair lineage for the pre-ready launch failure. It
does not create P2A-W4 or split instrumentation, transaction ordering, IPC
preflight, or replacement verification into thin WorkItems.

The bounded diagnosis proves that exact Candidate direct execution, observer
cycles, SQLite, private IPC, launchd, `KeepAlive`, and resident-like private
configuration are stable. The remaining boundary is the production
install/bootout/bootstrap transition plus the exact production socket path.
The retained `daemon failed` text is not sufficient to classify the failing
layer.

## 2. Frozen source inputs

The implementation starts from these exact bytes:

```text
cmd/loomd/run.go
d6c3098ecd9065fc8ca1a8b6e804717be219584d9205f34d8fd56b31daa1d166

cmd/loomd/run_test.go
41dcf9f4fab0818e91406e3d3c8ea1b78253d4e389f3d95b9423b963beda826b

cmd/loomd/product_daemon.go
8300ab1983a322f7f6c50a64430da1ef8b8dd4bf31d62d9e9b728ae6cdb048d5

cmd/loomd/product_daemon_test.go
ea7445861525c4c19958448af7514028b04ad1b5ac958b02c8ce57dab016128d

internal/localipc/server.go
3fb86f96326464e72ceee641519ef7a62d179f7bf8fa7d40e8cb36df22cd6482

internal/localipc/server_test.go
95249849c6168fafe2739c2788f13375514551d76080c06c8b7212ac4626839a

local-product-live-closure-transaction.sh
a23b3d3ca04040c42d8fdb2b891e945e44cf3ff7b2966cca354faf35da6c528a

local-product-live-closure-transaction-test.sh
6f7b8252d861561bc2beed3616fdbd3e8343bb5316497f8f03580796438d2e1d

retained Candidate manifest
7c0ec8cd75129f0184d345ea39684bc1b1850db56907f1b95ea57f61302a1700
```

`internal/localipc/server.go` and `server_test.go` are frozen read-only
references. They may not change unless a deterministic RED first proves the
existing server violates its accepted socket contract. A passing exact-path
preflight is evidence against reopening them.

## 3. Exact owned files

This repair may modify only:

```text
cmd/loomd/run.go
cmd/loomd/run_test.go
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-transaction.sh
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-transaction-test.sh
docs/CURRENT.md
```

It may create only:

```text
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-diagnosis.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-closure-repair-contract.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-closure-repair-contract-review.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-closure-repair-contract-review-2.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-closure-repair-red.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-closure-repair-green.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-closure-repair-implementation-review.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-exact-path-preflight.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-source-lock.json
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-candidate-manifest.json
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-reason.json
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-replacement-activation-audit.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-replacement-live-canary.md
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-replacement-result-review.md
```

No API schema, Journal fact, Projection, StateWriter, local IPC protocol,
installer, App source, Runtime adapter, Provider, credential, accepted ADR,
root policy, or P2A-W2/P2A-W3 file is owned.

## 4. Closed daemon failure classification

The production daemon may expose only these stable stderr messages:

```text
daemon unavailable
daemon failed: observer
daemon failed: local_ipc
daemon failed: shutdown
daemon failed: result
```

Classification requirements:

- `observer` means the observation runner returned a non-context error;
- `local_ipc` means the private server failed before readiness or terminated
  unexpectedly while the observer was active;
- `shutdown` means an otherwise expected stop could not close the private
  server, database, or observer cleanly;
- `result` means the terminal result could not be encoded;
- unknown internal errors fail closed as `daemon failed: shutdown`;
- raw Go error strings, paths, SQL text, request data, environment values,
  credentials, Provider output, model output, and hidden reasoning never enter
  stderr.

Existing exit-code meanings remain unchanged. CLI and App code must not parse
these messages as state authority.

The only governed failure-reason artifact path is:

```text
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-reason.json
```

It may be created only after a Candidate bootstrap has been counted and before
rollback completes. It is a canonical JSON object containing exactly:

```text
schema_version
classification
failure_phase
launchd_state
launchd_runs
launchd_exit_code
initial_bootstrap_calls
consumed
rollback_count
restart_count
```

`classification` is one of the closed daemon messages above. State is one of
`running`, `not_running`, `exited`, or `unknown`. Count fields are bounded
nonnegative integers. Exit code is a bounded integer or `null`. Unknown keys,
free text, paths, timestamps, process IDs, environment names/values, and raw
stderr are prohibited. The file must be uid `501`, mode `0600`, written
atomically, and rejected if it already exists or is a symlink.

`failure_phase` is exactly one of:

```text
bootstrap
readiness
run_root
socket
process_identity
product_identity
app_identity
status_snapshot
journal
crash_inventory
staging
unclassified
```

The phase is transaction attribution, not daemon error text and not state
authority.

## 5. Mandatory RED

Before implementation:

1. focused `cmd/loomd` tests must fail because observer, local IPC, shutdown,
   and result failures collapse to the same text;
2. product-daemon tests must fail because server-before-ready and
   observer-after-ready failures lack stable typed classification;
3. the transaction fixture must fail because it does not require:
   - original service absence before Candidate installation;
   - exact-path preflight before Candidate bootstrap;
   - a `0600` closed reason record on pre-ready failure;
   - exact one-bootstrap/no-retry accounting after any failure.

RED evidence records only test names, expected closed codes, exit status, and
bounded command output.

## 6. Exact-path preflight

After Contract Review PASS and before product implementation, one bounded
preflight may test the exact production socket path without restarting or
replacing the resident observer:

1. verify exact original observer PID/program, installed hashes, plist,
   provenance presence, SQLite hash/integrity/Event count, absent App/process,
   absent run root/socket, crash inventory, and empty staging;
2. create the exact run root as uid `501`, mode `0700`, with no symlink;
3. run the exact retained Candidate directly with:
   - a private SQLite copy;
   - a symlink-free private copy of the resident isolation root;
   - the frozen runtime/probe/device/display configuration;
   - `max-cycles=1`;
   - the exact production socket path;
   - an empty process environment and no Provider credential;
4. read one typed status response through the exact Candidate CLI;
5. require exit `0`, empty stderr, one Candidate process, and no native App;
6. remove the socket, lock, exact run root, private copies, and diagnostic
   process;
7. revalidate unchanged resident observer PID, Journal, installed bytes,
   reports, and staging.

Any deviation stops `HUMAN_REQUIRED`; it does not consume or create a
production live allowance.

### 6.1 Invalid evidence harness and bounded replacement

The first exact-path execution did not produce a governed PASS or product
FAIL. It reached the exact socket-path exercise and cleaned up, but the
evidence harness required a nonexistent nested
`local_product_snapshot` JSON object. The accepted CLI intentionally flattens
the embedded `LocalProductSnapshot` into the top-level status envelope.
Cleanup then removed the bounded stdout before the failed predicate could be
classified. This is an evidence-harness defect, not evidence that the product
passed or failed.

The post-cleanup audit proves the original observer remains PID `85936`,
state `running`, runs `1`; the exact run root, Candidate process, and native
App are absent; the resident Journal retains its exact hash, integrity `ok`,
and one Event; staging is empty.

After a fresh independent Contract Re-review PASS, exactly one replacement
preflight may run under the unchanged section 6 constraints. It must preserve
the bounded private outputs until it has recorded exit codes, stderr byte
counts, process count, and typed-status validity. Typed validity requires a
top-level object with `command=status`, `source_mode=daemon_api`,
`schema_version`, `view_version`, and canonical top-level `runtimes`, `teams`,
`runs`, `evidence`, and `attention` arrays. It must not require the nonexistent
nested object. The replacement is not a production bootstrap, live canary,
retry of the consumed transaction, or live allowance.

## 7. Switch transaction repair

The transaction must:

- arm rollback before mutation;
- boot out the original observer and prove the old PID and label are absent
  before changing installed executable or plist bytes;
- install the exact reviewed Candidate only after that absence proof;
- create and validate the exact private run root after the old service is
  absent;
- bootstrap the Candidate exactly once;
- wait for both the exact label and exact private socket;
- record only the closed reason code plus launchd state, run count, and exit
  code if readiness fails;
- never record a process environment or raw daemon error;
- roll back immediately on any pre-ready failure;
- never retry, alternate the socket, or perform a second Candidate bootstrap;
- restore the exact original installation, plist, metadata, Journal, reports,
  and observer;
- leave the reason record as `0600`, uid `501`, inside the governed evidence
  directory.

The fixture must prove ordering, one bootstrap, zero retry, rollback on each
failure phase, exact activation binding, and no App launch before readiness.

## 8. Verification

Required deterministic verification:

```text
focused cmd/loomd tests
focused internal/localipc tests
go test ./cmd/loomd ./internal/localipc
go test ./...
go test -race ./...
go vet ./...
go mod tidy -diff
go mod verify
gofmt check
shell syntax checks
transaction fixture matrix
Swift debug tests
Swift release build
Swift Thread Sanitizer tests
native App installer fixture
local product installer fixture
credential-negative scan
authority-boundary scan
git diff --check
git staging empty
```

All immutable original resident artifacts and failed-Candidate evidence must
remain exact. Historical PID/run observations remain immutable evidence but
are not asserted to be unchanged after the disclosed KeepAlive restarts; the
continuity rules in section 8.2 apply.

## 8.1 Fresh repaired Candidate identity

The retained failed Candidate is authorized only for the exact-path preflight.
It is never eligible for replacement live activation.

After deterministic GREEN, a fresh private repaired Candidate must be built
from the reviewed source bytes. Its source lock must be written to the exact
path:

```text
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-source-lock.json
```

The source lock must bind:

- base commit;
- every product/build/test input used by `loom`, `loomd`, and the native App;
- exact pre-repair source hashes from section 2;
- exact post-repair hashes for every owned delta;
- Go module locks, Swift package inputs, installer inputs, and bundle-builder
  inputs;
- Go, Swift, linker, signing, and host architecture versions.

The source lock does not contain transaction or transaction-fixture hashes.
This keeps source identity upstream of the transaction instead of creating a
mutual hash dependency.

The Candidate manifest must be written to the exact path:

```text
.loom-evidence/phase2a/P2A-W1/local-product-launch-failure-repair-candidate-manifest.json
```

It must bind:

- source-lock SHA-256;
- fresh `loom` and `loomd` SHA-256;
- native App executable SHA-256;
- one nonzero arm64 `LC_UUID`;
- canonical signed-bundle manifest SHA-256;
- builder/toolchain identities;
- uid/mode/no-symlink/privacy checks for the private Candidate;
- absence of credentials, Provider environment, private source paths,
  resident Journal copies, raw model output, and hidden reasoning.

Both JSON documents must be canonical, uid `501`, mode `0600`, and contain no
absolute private Candidate path. The private Candidate parent/root and bundle
directories must be uid `501`, mode `0700`; executables are `0700`; ordinary
bundle files are `0600`.

The repaired transaction must freeze the exact source-lock record hash,
Candidate-manifest record hash, artifact hashes, UUID, and bundle-manifest
hash. It cannot contain its own hash, and neither upstream identity document
contains the transaction or fixture hash.

The post-Review activation audit is the single downstream binding record. It
must bind the source-lock record hash, Candidate-manifest record hash, exact
transaction hash, exact transaction-fixture hash, artifact hashes, UUID, and
bundle-manifest hash. The Implementation Reviewer and activation audit must
reproduce this acyclic chain. A different build or retained failed Candidate
fails closed before bootstrap.

## 8.2 Resident continuity rebaseline

The accepted preflight observation at PID `85936`, runs `1`, and the disclosed
GREEN observation at PID `97912`, runs `18`, remain immutable historical
evidence. The post-repair sequential verification matrix caused the unchanged
KeepAlive observer to reach PID `95529`, runs `35`, last exit `4`, followed by
PID `95853`, runs `36`. No launchctl lifecycle command was used by that
verification. Installed executable, wrapper, plist, Journal, and historical
report bytes remained exact.

PID and launchd run count are process observations, not state authority. They
cannot be silently called exact continuity after a KeepAlive restart. For this
repair, continuity means all of the following:

- exact installed `loom`, `loomd`, wrapper, and plist hashes and modes;
- exact label and wrapper program, with one running resident process and no
  Candidate target marker;
- exact Journal hash, integrity `ok`, and one Event;
- exact historical crash-report inventory and hashes;
- absent native App, run root, socket, replacement files, activation record,
  failure-reason record, and staged changes;
- no Provider credential or Runtime execution;
- a stable pre-activation PID and run count across three samples spanning at
  least sixteen seconds, taken after resource-heavy verification has ended.

The post-Review activation audit must disclose every historical observation
above, freeze the current stable PID and run count in exactly one
`Resident PID` field and one `Resident Runs` field, and prove the immutable
continuity predicates. It may not erase, reinterpret, or reset launchd
counters.

The transaction must strictly parse those two closed decimal fields and
require the audited PID and run count to match immediately before any bootout.
A mismatch fails with zero bootstrap and zero mutation. Rollback restores the
same logical resident service, exact bytes, metadata, Journal, and reports;
because a bootout/bootstrap necessarily creates a new OS process, rollback
does not claim to restore the historical PID or launchd run count.

This is a bounded amendment to the existing P2A-W1 exit gate, not a new
WorkItem, a service reset, or live authority. It requires fresh independent
Contract Re-review before transaction RED or implementation.

## 8.3 Complete pre-READY attribution and recovery settling

The consumed replacement result remains:

```text
FAIL - ROLLBACK_INCOMPLETE
HUMAN_REQUIRED
NO RETRY
allowance=0
```

The missing exact predicate must not be inferred. This amendment changes only
the non-live transaction/fixture/evidence closure inside the existing P2A-W1
lineage. It does not create or restore an allowance.

After the sole Candidate bootstrap is counted, every path through
`LOCAL_PRODUCT_CLOSURE_READY` must be closed:

- bootstrap invocation;
- label/socket readiness;
- run-root ownership;
- socket ownership/mode;
- Candidate process identity and credential-marker absence;
- installed product bytes/modes;
- installed App bytes/bundle/signature;
- typed status snapshot;
- Journal;
- crash inventory;
- staging.

Each failure must select exactly one `failure_phase`, attempt the governed
reason record, and emit exactly one closed line:

```text
LOCAL_PRODUCT_CLOSURE_FAILURE phase=<closed-enum> reason_recorded=<0|1>
```

Reason writing is best-effort evidence preservation. A reason-writer failure
sets `reason_recorded=0` but cannot hide or replace the primary phase. An exit
trap must fail closed as `unclassified` if any consumed pre-READY exit reaches
rollback without a prior phase emission. No raw error, path, PID, environment,
credential, Provider output, model output, or hidden reasoning may enter the
line or JSON.

The fixture must causally RED then prove:

- every phase routes to one closed failure line and one rollback;
- reason-record success produces the exact ten-key JSON schema;
- existing/symlink/invalid reason targets produce `reason_recorded=0` while
  preserving the primary phase and rollback;
- no phase produces a retry or second Candidate bootstrap;
- the `unclassified` trap catches an injected uncovered consumed exit;
- successful READY emits no failure line;
- all historical fixture outputs outside this new failure line remain exact.

`wait_original_running` must cover a sixty-second bounded envelope, exceeding
the observed launchd throttle plus observer startup delay. Its deterministic
test uses an injected fast clock/state sequence rather than sleeping sixty
seconds. Rollback may report `rolled_back` only after the exact immutable
continuity predicates and a running original service pass. Otherwise it
remains `rollback_incomplete`.

Implementation Review may evaluate this non-live repair, but no activation
audit or canary is permitted by this amendment. Any future live action
requires a new explicit, reviewed authority record after this repair passes.

## 9. Review and replacement live gate

A fresh independent Implementation Reviewer must reproduce:

- source hashes and owned-file confinement;
- causal RED and minimal GREEN;
- exact-path preflight;
- closed reason-code non-disclosure;
- switch ordering and one-bootstrap/no-retry fixture;
- full deterministic matrix;
- fresh repaired source lock, Candidate manifest, binaries, UUID, signed
  bundle manifest, transaction, and fixture;
- immutable resident continuity plus the section 8.2 rebaseline evidence and
  empty staging.

Implementation Review PASS alone grants no production launch.

Only after that PASS may a post-Review activation audit decide whether the
user's standing continuation authorization and the active Phase 2A objective
permit one new replacement allowance. If activated, it is exactly one
bootstrap of the exact fresh repaired Candidate bound above and one
native-window canary. Any failure consumes it, writes the exact closed reason
record if pre-ready, rolls back, and stops `HUMAN_REQUIRED`. It cannot
authorize the retained failed Candidate, an alternate build, a second retry,
P2A-W2, Provider/Runtime execution, credential mutation,
commit/push/merge/release, or acceptance.

## 10. Exit

This repair passes only when:

- Contract Review PASS;
- RED is causal;
- exact-path preflight PASS;
- implementation and complete matrix PASS;
- fresh independent Implementation Review PASS;
- a separately activated single replacement live canary proves native Home,
  Work, Teams, Inbox, System, Refresh, quit/relaunch recovery, one later
  classified daemon lifecycle restart, unchanged Journal, no Provider marker,
  no credential exposure, and exact installed state;
- fresh Result-Evidence Review PASS.

Until every gate passes, P2A-W1 remains unaccepted and uncommitted, and P2A-W2
remains locked.
