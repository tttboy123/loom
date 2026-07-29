# P2A-W1 Local Product Vertical Live Closure Replacement Contract

**Date**: 2026-07-29  
**Status**: `FROZEN - FRESH INDEPENDENT CONTRACT RE-REVIEW 2 PASS`  
**Parent**: P2A-W1 Local App Shell and Read Experience  
**Replaces**: the remaining live-exit authority of Native App Final Exit
Closure and Native Product Experience Vertical Reopen  
**WorkItem count**: unchanged; this remains the only P2A-W1  
**Live allowance**: `0` until all deterministic and Review gates pass  
**P2A-W2**: `LOCKED`

## 1. Evidence-led reason

The last controlled native-window canary and its fresh independent
Result-Evidence Review closed the only current product failure:

1. the one authoritative `RuntimeInstanceDiscovered` Event is valid and its
   `model_ids` JSON member has type `null`;
2. Projection intentionally rebuilds the historical Runtime record without
   changing Journal bytes;
3. `buildLocalProductSnapshot` clones the nil model list with a nil
   destination;
4. Go JSON therefore emits `"model_ids":null`;
5. strict Swift decoding requires `[String]`, fails closed, and surfaces
   `invalid_response`;
6. the existing Go-server/Swift-client fixture hard-codes a non-empty array and
   misses the authoritative live shape.

The original observer is currently running from `loomd-clean`, the plist and
installed product hashes match the accepted rollback ledger, SQLite integrity
is `ok`, Event count is `1`, the Journal has not been migrated, Git staging is
empty, and the prior live allowance remains consumed at `0`.

This is the single remaining vertical P2A-W1 product closure. It is not a point
Amendment, Reopen 5, P2A-W4, or permission to begin P2A-W2.

## 2. Durable boundary

The fix belongs to the shared local-product Application/API wire boundary:

- Journal remains the only state authority and is byte-unchanged;
- Projection remains a rebuildable cache and preserves the historical record;
- StateWriter and Runtime discovery semantics are unchanged;
- TUI, CLI, daemon IPC, and native App continue to consume the same
  `LocalProductReadService`;
- the local-product snapshot wire contract canonicalizes present empty
  collections to JSON arrays;
- the strict Swift client remains strict and is not widened to accept `null`;
- non-empty collection order and values are preserved exactly;
- stale-view fallback applies the same normalization as a fresh view.

The repair must canonicalize both `model_ids` and
`observed_capabilities`. They are sibling required-array fields and leaving
either nil would preserve the same cross-language failure class.

This contract adds no second read model, schema version, database, queue,
Scheduler, StateWriter, retry, fallback, Provider, Runtime execution, command,
or authority transition.

## 3. Exact owned files

This replacement may modify only:

```text
internal/api/local_product_read.go
internal/api/local_product_read_test.go
cmd/loomd/product_daemon_test.go
internal/localipc/swift_contract_test.go
docs/CURRENT.md
```

It may create only:

```text
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-replacement-contract.md
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-replacement-contract-review.md
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-source-lock.json
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-red.md
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-green.md
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-implementation-review.md
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-candidate-manifest.json
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-transaction.sh
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-transaction-test.sh
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-activation-audit.md
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-live-canary.md
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-result-review.md
```

The accepted Native Product Experience Swift source is an immutable Candidate
input for this closure. This contract does not reopen it.

No Phase 1 evidence, accepted historical P2A-W1 evidence, Go module lock,
Journal, Projection, StateWriter, Swift model decoder, plist, installed file,
credential, Provider, Runtime adapter, Scheduler, Supervisor, Grant, Evidence
authority, or current user data is owned.

An unexpected need for any other source file stops for a reviewed replacement
of this single contract. It does not create another thin contract.

## 4. Exact source authority

Candidate inputs are frozen by
`local-product-live-closure-source-lock.json`.

```text
SHA-256
0bd33d144c5aadcd078bef06a45685b8ac2ed2ab2bf76effe80730ee85e8a128
```

The lock binds:

- base Git commit `3a2c3da5f1682e1acabfebefb6c55e5a57183a20`;
- every accepted prior P2A-W1 Go, Swift, TUI, IPC, build, install, test, module,
  and ADR input by exact path and SHA-256;
- the only four product/test deltas this closure may change:
  `internal/api/local_product_read.go`,
  `internal/api/local_product_read_test.go`,
  `cmd/loomd/product_daemon_test.go`, and
  `internal/localipc/swift_contract_test.go`;
- explicit exclusions for `.build`, private Candidate roots, evidence output,
  installed files, `.codex`, `.loom-drafts`, crash reports, private models,
  and every unrelated dirty-worktree path.

Before every deterministic matrix, Candidate build, Review, and live preflight,
all locked prior inputs must match the source lock. Closure deltas are hashed
only after final GREEN and recorded in the Candidate manifest. The Candidate
manifest must bind the source-lock digest, the four final delta hashes, output
binary/app hashes, UUID, bundle manifest, toolchain versions, and build
commands. It must reject any source path outside the locked inputs plus delta
allowlist.

No build cache, generated file, archived Candidate, installed binary, evidence
file, or untracked unrelated source may be used as a source input.

## 5. Mandatory RED

Before product repair, tests must prove:

1. a real `RuntimeInstanceDiscovered` fixture with
   `"model_ids":null` rebuilds through the accepted Projection path;
2. a second exact authoritative Go-path case with
   `"observed_capabilities":null` or an equivalent nil projected Runtime
   reaches `LocalProductReadService` and currently marshals that field as
   `null`;
3. the resulting local-product snapshots currently marshal
   `"model_ids":null`;
4. the daemon IPC fixture based on the historical Event exposes a nil
   collection;
5. the Swift contract fixture does not cover the live empty-array case.

The focused RED must fail for the missing canonical-array wire behavior, not
for build environment, socket, launchd, network, or fixture corruption.

## 6. Required implementation

`buildLocalProductSnapshot` and its clone path must return independent,
non-nil slices for Runtime `ModelIDs` and `ObservedCapabilities`, including
zero-length input. A small generic helper is allowed only inside
`internal/api/local_product_read.go`.

The implementation must not:

- mutate `projection.RuntimeInstance`;
- sort, deduplicate, add, or remove non-empty values;
- use `omitempty`;
- change schema version `1`;
- normalize by marshal/unmarshal round trips;
- accept unknown fields or `null` in Swift;
- change Event payload bytes or write a new Event;
- hide a Projection failure as successful fresh state.

## 7. Contract-to-test closure

The deterministic evidence must include:

1. Go Application/API tests using the exact historical `model_ids:null` Event
   and an authoritative Go-path `observed_capabilities:null` case,
   asserting returned slices are non-nil, JSON contains the two exact empty
   arrays, repeated caller mutation cannot alias stored state, and stale
   fallback remains canonical;
2. daemon IPC test whose SQLite fixture contains `model_ids:null`, asserting
   the typed response contains non-nil empty lists and restart returns the same
   snapshot;
3. real Go `localipc.Server` to Swift contract probe with
   `"model_ids":[]` and `"observed_capabilities":[]`, while malformed
   `null`, missing, duplicate, unknown, and wrong-type responses remain
   rejected as `invalid_response`;
4. non-empty list order preservation;
5. empty top-level snapshot collections remain JSON arrays;
6. existing strict envelope, peer, framing, timeline, cursor, stale-view, and
   secret-negative tests remain GREEN.

The component evidence is the conjunction of the real historical Event to
daemon IPC proof and the real Go-server to strict Swift empty-array proof. A
hard-coded non-empty-only fixture is insufficient.

## 8. Deterministic verification

After GREEN and after every product repair:

```text
gofmt on owned Go files
go test ./internal/api ./internal/localipc ./cmd/loomd
go test -race ./internal/api ./internal/localipc ./cmd/loomd
go test ./...
go test -race ./...
go vet ./...
go mod tidy -diff
go mod verify
cd apps/macos && swift test
cd apps/macos && swift build -c release
cd apps/macos && swift test --sanitize=thread
scripts/test-install-loom-local-product.sh
.loom-evidence/phase2a/P2A-W1/local-product-live-closure-transaction-test.sh
git diff --check
owned-file and dependency audit
authority/import-direction audit
secret-negative and terminal-control audit
empty Git staging audit
```

No deterministic command may connect to a Provider, execute a Runtime, mutate
the resident Journal, restart launchd, open the App, or use network.

`go mod tidy -diff` is a read-only no-diff gate. Any output or non-zero result
stops as module-lock drift because this closure does not own `go.mod` or
`go.sum`; it must not rewrite, restore, stage, or commit either file.

## 9. Candidate materialization

Only after deterministic GREEN, build a fresh private Candidate containing the
current reviewed P2A-W1 product sources and accepted UI:

- Candidate root and all directories: user-owned `0700`;
- Candidate files: user-owned and no broader than required executable modes;
- fresh `loom`, `loomd`, and `Loom.app`;
- strict-valid application signature;
- non-zero arm64 `LC_UUID`;
- canonical bundle manifest;
- exact SHA-256 values recorded in the immutable Candidate manifest;
- no credential, Provider environment, private path, or live Journal copy in
  the Candidate manifest.

The controlled transaction must bind the exact manifest and fail before
bootstrap on any byte, UUID, signature, owner, mode, path, symlink, plist,
Journal, crash-inventory, process, or service pre-state mismatch.

## 10. Fresh independent Implementation Review

Review must independently confirm:

- the causal fix is at the shared API boundary;
- exact nil-to-empty normalization and non-empty order preservation;
- strict Swift behavior remains fail-closed;
- Journal/Projection/StateWriter and installed state are untouched;
- RED is genuine and all deterministic matrices pass;
- Candidate manifest and transaction have no unreviewed degree of freedom;
- rollback is armed before mutation;
- one bootstrap, no retry, one later lifecycle restart only after all UI
  screens pass;
- staging is empty.

Contract Review, GREEN, Candidate build, and Implementation Review grant no live
authority by themselves.

## 11. Single replacement controlled live closure

After Implementation Review `PASS`, the already requested Phase 2A objective
permits preparation of a post-Review activation audit, but Candidate bootstrap
remains unavailable until that audit proves the exact current pre-state and
records a single allowance `1`.

The transaction may then perform exactly one initial Candidate bootstrap and:

1. install the exact Candidate atomically with rollback armed;
2. preserve the exact one-Event Journal bytes and stream heads;
3. prove daemon readiness and CLI status through the private IPC API;
4. open only the Loom native App;
5. prove Home visibly shows one `Pi 0.82.1 Resident Demo` Runtime and the exact
   five accepted destinations without `invalid_response`;
6. prove Work, Teams, Inbox, and System render their authoritative empty or
   populated state without requiring an ID, path, cursor, launchctl, or
   Provider environment;
7. perform one bounded native Refresh;
8. quit and relaunch the App, proving the same view version and state recover
   from Journal/Projection;
9. perform the one classified daemon lifecycle restart, then prove the same
   view again;
10. prove no duplicate Event, Run, Evidence, side effect, crash report, secret
    disclosure, raw internal identifier, or terminal control sequence;
11. leave the successful exact Candidate installed only pending fresh
    Result-Evidence Review.

No Team creation, Provider configuration, Runtime execution, approval,
credential change, network access, hidden retry, alternate socket, second
bootstrap, or P2A-W2 behavior is permitted.

Any post-bootstrap failure consumes the allowance, performs exact rollback, and
stops `FAIL - ROLLED_BACK - HUMAN_REQUIRED`. A pre-bootstrap mismatch preserves
allowance `1` only when no mutation or bootstrap occurred and the mismatch is
fully recorded.

## 12. Exit and commit gate

Fresh independent Result-Evidence Review must verify the live result,
invocation accounting, screenshots/accessibility evidence, Journal integrity,
unchanged heads, secret-negative proof, process cleanup or installed Candidate
state, and staging.

Only a live `PASS` plus Result-Evidence Review `PASS` permits:

1. acceptance of P2A-W1;
2. one atomic local P2A-W1 commit containing only owned Candidate/evidence
   files already belonging to the W1 lineage;
3. a `docs/CURRENT.md` transition that unlocks contract freezing for P2A-W2.

This contract does not authorize push, merge, release, publish, P2A-W2
implementation, Provider live use, or Runtime execution.

## 13. Stop conditions

Stop `HUMAN_REQUIRED` when:

- an unowned authority or product file is required;
- the current historical Event cannot be supported without Journal mutation;
- strict Swift must be weakened to accept ambiguous wire data;
- safe atomic install/rollback or OS ownership/mode proof cannot be established;
- secret-negative checks fail and cleanup would change credentials;
- the same blocking condition survives three governed attempts.
