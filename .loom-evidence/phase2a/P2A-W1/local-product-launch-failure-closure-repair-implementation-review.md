# P2A-W1 Local Product Launch Failure Closure Implementation Review

**Status**: `FAIL`

**Reviewer**: fresh independent read-only Implementation Reviewer

**Activation authority**: none

## Findings

### P1 — exact-path Candidate is not joined on the signal path

`run_exact_path_preflight` starts the Candidate `loomd` in the background and
stores `exact_pid`, but termination and `wait` exist only on its normal path.
If HUP, INT, or TERM reaches the transaction shell during this window, the
exit trap enters `rollback`; rollback does not terminate or join `exact_pid`.
A signal sent only to the shell is not guaranteed to reach its child. The
fixture has no test for this window.

This violates bounded cleanup and blocks activation.

### P1 — source lock does not close all production inputs

Contract section 8.1 requires every product/build/test input used by `loom`,
`loomd`, and the native App. Independent enumeration with:

```text
go list -deps -json ./cmd/loom ./cmd/loomd
```

found 89 in-repository production Go inputs. Only 13 are listed in
`locked_inputs`; 75 unlisted inputs are byte-equal to the frozen base, while
`internal/api/local_product_read.go` is both unlisted and absent from that
base. The fresh `loomd` therefore has an unbound production input upstream of
the Candidate manifest.

This violates the frozen source-identity closure and blocks activation.

### P1 — resident continuity drift is outside the frozen preservation gate

The accepted exact-path evidence froze resident PID `85936`, runs `1`. The
GREEN record discloses PID `97912`, runs `18`, last exit `4`, after invalid
parallel verification saturated real Pi metadata fixtures. Installed bytes,
plist, Journal, reports, App/run absence, and staging remained exact, but
process identity and launchd counters did not.

The frozen contract grants no authority to silently rebaseline this drift.
Activation remains blocked pending an explicit post-repair continuity
reconciliation and reviewed activation audit.

## Independently reproduced passing evidence

- Contract, RED, GREEN, exact-path evidence, transaction, fixture, source-lock,
  and Candidate-manifest record hashes match their supplied values.
- All 53 listed source-lock hashes and four owned deltas match.
- Fresh Candidate hashes, uid/modes, arm64 architecture, nonzero UUID, strict
  signature, bundle manifest, privacy scan, and toolchain match.
- Focused daemon lifecycle and non-disclosure tests pass.
- `go test ./internal/localipc -count=1` passes.
- `go test -count=1 ./...` passes sequentially.
- The identity hash direction is acyclic.
- Activation and failure-reason records are absent.
- No live state was read or changed by the Reviewer.

## Verdict

`FAIL`

Implementation Review PASS was not reached. No activation, installation,
bootstrap, restart, native App launch, Provider action, Runtime action, or live
canary is authorized. Repairs remain inside the one frozen P2A-W1 contract;
even a later Implementation Review PASS requires a separate activation audit.

## Repair RED 1

The transaction fixture was extended before implementation with both static
and signal-window requirements. It fails causally:

```text
RED: transaction has no bounded exact-path Candidate cleanup helper
RED: rollback does not terminate and join the exact-path Candidate
```

No production transaction, service, installed product, Journal, Candidate, or
live state changed during this RED.

## Repair RED 2

After Contract Re-review 4 PASS, the transaction fixture causally proves the
reviewed continuity binding is absent:

```text
RED: transaction does not strictly bind the audited resident PID
RED: transaction does not strictly bind the audited resident run count
RED: transaction does not recheck the audited PID immediately before bootout
RED: transaction does not recheck the audited run count immediately before bootout
RED: audited resident identity is not checked before rollback arm and bootout
```

No activation record exists, and no service, installed product, App, Journal,
Candidate, Provider, Runtime, credential, staging, or live state changed.

## Repair GREEN

The two implementation defects and the reviewed continuity binding are closed:

- `stop_exact_path_candidate` performs bounded TERM, bounded wait, KILL
  fallback, and direct-child `wait`; both timeout and rollback paths use it.
- The exact signal-window fixture launches a real child, signals only the
  harness shell, and proves exit `98`, one rollback, the closed cleanup order,
  absence of the run root, and no surviving child. Repeated runs pass.
- The repaired source lock binds `201` exact inputs. Its deterministic
  `go list -deps -test -json ./...` closure contains `171` in-repository Go
  production/test/cgo/assembly/syso/embed inputs. All 16 native Swift
  source/test/resource/package inputs are present.
- The fixture independently re-enumerates the Go closure, requires it to be a
  subset of the lock, verifies both recorded counts, and reproduces every
  locked SHA-256.
- The Candidate manifest binds the repaired source-lock record.
- The transaction strictly parses exactly one bounded decimal `Resident PID`
  and `Resident Runs` field, matches them during preflight, then matches them
  again immediately before rollback arm and bootout.
- Missing activation evidence remains zero-bootstrap and zero-mutation:

```text
LOCAL_PRODUCT_CLOSURE_RESULT result=preflight_failure initial_bootstrap_calls=0 consumed=0 rollback_count=0 restart_count=0
```

Current exact identity:

```text
source lock
8c75b532951de6ce8372fb76584d8246d625e310b7300e0af9c022b8cd903abb

Candidate manifest record
ec03f11e6b46512994ce2d9c389b019102feadb11398a986ff346b6f3d234167

transaction
42ceeb37364a2bfd065d7347f211c1c8d892e55d75b58025d84bb2c516770ca5

transaction fixture
71dc4a53e8b0097307e04ff0c3d7f933bd1f4bdbcd15cc7f488530e3657c7aca

contract
047f4bc691bc7304f0ba2e8082081ba1a91a176bae4aa4519ce4eff01b618fe3
```

Sequential Go test/race/vet/module verification, Swift debug/release/Thread
Sanitizer, native build/install fixtures, local-product installer fixture,
shell syntax, transaction fixture, diff, staging, and zero-activation checks
pass. The resource-heavy matrix advanced the unchanged KeepAlive observer to
PID `95853`, runs `36`; three post-matrix samples spanning sixteen seconds were
stable. Installed bytes, plist, Journal hash/integrity/one Event, historical
reports, absent App/run root, and empty staging remained exact.

This GREEN grants no live allowance. Fresh independent Implementation
Re-review is required, followed by a separate activation audit.

## Implementation Re-review

**Verdict**: `PASS`

**Findings**:

- P0: none
- P1: none
- P2: none

The fresh independent read-only Reviewer reproduced:

- bounded TERM, KILL fallback, and direct-child join on the exact-path cleanup
  path, including `set -e` safety and the real shell-only signal fixture;
- `171` automatically enumerated Go inputs contained by `201` locked inputs,
  zero missing inputs, zero hash mismatches, all 16 Swift inputs, and the
  acyclic source-lock to Candidate to transaction identity chain;
- preservation of all historical PID/run observations under reviewed section
  8.2;
- unique bounded activation count parsing and both resident matches before
  rollback arm and bootout;
- focused daemon lifecycle/non-disclosure tests and the transaction fixture.

The retained sequential full-matrix evidence was reviewed but not rerun, so
the Reviewer did not recreate the known resident Pi resource contention. No
live service, Candidate, App, Journal, Provider, Runtime, activation record,
credential, or staging was inspected or changed.

This PASS grants no activation. A separate post-Review activation audit remains
mandatory.

## Repair RED 3

After Contract Re-review 5 PASS, the non-live transaction fixture causally
fails because complete pre-READY attribution and recovery settling do not
exist:

```text
RED: transaction cannot preserve a closed primary pre-READY phase
RED: transaction has no closed pre-READY failure line
RED: original-service recovery envelope is shorter than sixty seconds
RED: transaction does not route bootstrap through closed attribution
RED: transaction does not route readiness through closed attribution
RED: transaction does not route run_root through closed attribution
RED: transaction does not route socket through closed attribution
RED: transaction does not route process_identity through closed attribution
RED: transaction does not route product_identity through closed attribution
RED: transaction does not route app_identity through closed attribution
RED: transaction does not route status_snapshot through closed attribution
RED: transaction does not route journal through closed attribution
RED: transaction does not route crash_inventory through closed attribution
RED: transaction does not route staging through closed attribution
RED: transaction does not route unclassified through closed attribution
```

The consumed live result, allowance `0`, original resident, installed bytes,
App/run absence, Journal, Candidate, Provider/Runtime boundary, credentials,
and staging were not touched.

## Repair GREEN 3

The non-live transaction closure now passes:

- every named consumed pre-READY predicate routes through
  `fail_pre_ready <phase>`;
- a shared recorder attempts the exact ten-key reason JSON, then emits exactly
  one `LOCAL_PRODUCT_CLOSURE_FAILURE` line with `reason_recorded=0|1`;
- a reason target that already exists or is a symlink preserves the primary
  phase, emits `reason_recorded=0`, rolls back once, and does not replace the
  target;
- the EXIT trap captures a real injected uncovered consumed exit as
  `unclassified`;
- all twelve closed phases produce one bootstrap, one failure line, one
  rollback, zero restart, no run root, and no retry;
- the success fixture emits no failure line;
- `wait_original_running` uses the same generic bounded loop with 240
  quarter-second probes, while the fast fixture reaches recovery on probe 81
  without sleeping;
- missing/consumed activation evidence still fails before any live read or
  mutation with exact zero-bootstrap output.

Current exact non-live implementation:

```text
transaction
99b448a8284fc58aadb0e57a5d66969cc84f1e6cd2360c475298f86d2150212b

transaction fixture
8066a20d0b31fc67a5ddc1fae029f2baf3787baf8b8167b7c4da2302508cfe4d

contract
4713e56acd1a463e0081b57bade31118a8c1b68926dc5b5d7640aedfdefc2f5b
```

Shell syntax, the complete transaction fixture, repeated phase/signal tests,
source-lock reproduction, diff, staging, and zero-activation checks pass. No
Go or Swift product source changed, so the previously reviewed sequential
full matrix remains the applicable product evidence and was not rerun against
the unstable resident observer.

The allowance remains `0`. This GREEN grants no activation, retry, App launch,
Provider/Runtime action, P2A-W1 acceptance/commit, or P2A-W2 unlock. Fresh
independent Implementation Re-review is required.

## Implementation Re-review 6

**Verdict**: `PASS`

**Findings**:

- P0: none
- P1: none
- P2: none

The fresh independent read-only Reviewer reproduced the exact transaction,
fixture, contract, and evidence hashes; the ten-key schema and twelve-value
closed enum; all consumed pre-READY routes; primary-phase preservation when
reason writing fails; nonrecursive `unclassified` EXIT handling; no-replace
reason targets; no failure line on success; shared 240-probe production and
probe-81 fast recovery; zero retry/second bootstrap; and zero-live consumed
activation preflight. Shell syntax and the complete transaction fixture pass.

No production transaction, activation artifact, live service, Candidate, App,
Journal, credential, or staging surface was inspected or mutated.

This PASS grants no activation. Allowance remains `0`, and any future live
action requires a new explicit reviewed authority record after the consumed
failure.
