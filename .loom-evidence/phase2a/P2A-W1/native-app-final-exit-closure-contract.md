# P2A-W1 Native App Final Exit Closure Contract

**Date**: 2026-07-28  
**Status**: `FROZEN — FRESH INDEPENDENT CONTRACT REVIEW PASS`  
**Parent**: P2A-W1 Native App Host and Launchability contracts  
**WorkItem count**: unchanged; this remains P2A-W1  
**Product-source authority**: none  
**Live authority**: none

## 1. Evidence-led boundary

The reviewed P2A-W1 Candidate remains deterministic and launchable. The
consumed replacement canary failed before native app launch because its
temporary transaction omitted an already-frozen runbook step:

```text
create the exact product run directory as user-owned 0700
```

`newProductDaemonRunner` constructs `localipc.Server` before `Run`, and
`localipc.validateSocketPath` rejects a missing socket parent. The Candidate
therefore returned `daemon unavailable`. The transaction then restored the
exact original state, and fresh independent Result-Evidence Review passed.

This is an execution-harness defect, not a product, IPC, daemon, installer,
LC_UUID, Journal, Projection, Provider, Runtime, or native UI defect.
Production source must not be changed to compensate for it.

## 2. Governance reconciliation

Historical results remain append-only and true:

- the latest replacement allowance is `0`;
- the canary result remains `FAIL — ROLLED_BACK — HUMAN_REQUIRED`;
- P2A-W1 remains unaccepted and uncommitted;
- P2A-W2 remains locked.

This contract is the only remaining P2A-W1 exit closure. It does not create
Reopen 5, P2A-W4, a point behavior Amendment, or another product Candidate.
It closes the whole remaining launch transaction and live-evidence boundary in
one governance unit.

Contract Review, RED, GREEN, or Implementation Review grants no live authority.
An earlier blanket authorization, prior activation phrase, or automatic goal
continuation cannot authorize another Candidate bootstrap.

## 3. Exact owned files

This closure may create only:

```text
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-closure-contract.md
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-closure-contract-review.md
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction.sh
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction-test.sh
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-red.md
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-green.md
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-implementation-review.md
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-post-review-audit.md
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-live-canary.md
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-result-review.md
```

It may modify only:

```text
docs/CURRENT.md
```

The transaction and its test are governed evidence harnesses, not installed
product files. No Go, Swift, plist, application resource, builder, installer,
Journal, Projection, StateWriter, daemon, IPC, Runtime, Provider, credential,
authorization, Scheduler, Supervisor, Evidence authority, or accepted ADR is
reopened.

An unexpected need to edit product source fails this contract
`HUMAN_REQUIRED`; it is not silently expanded.

## 4. Exact transaction state machine

The harness has one explicit, fail-closed state machine:

```text
preflight
→ prepared
→ run_root_ready
→ original_quiescent
→ candidate_bootstrapped
→ daemon_ready
→ native_verified
→ lifecycle_verified
→ result_review_pending
→ preserve | rollback
```

Rules:

1. every transition is monotonic and recorded only as a closed enum;
2. rollback is armed before the first mutation;
3. the exact user-owned, non-symlink product run directory is created with
   mode `0700` and owner uid `501` before product/app installation, original
   service bootout, plist replacement, or Candidate bootstrap;
4. the harness validates the directory's canonical path, parent, type, owner,
   and mode both before and after Candidate bootstrap;
5. exactly one explicit initial `launchctl bootstrap` call is permitted;
6. `candidate_bootstrapped=1` is assigned immediately after that command
   returns success and before any PID, socket, readiness, status, or UI wait;
7. a launchd job that exits or is automatically re-executed after successful
   bootstrap remains one consumed bootstrap allowance and a post-bootstrap
   failure;
8. no readiness or UI failure can take a path back to bootstrap;
9. the one later daemon restart is reachable only after the complete native
   screen and relaunch proof and is classified as the frozen lifecycle check,
   never recovery from a failed initial bootstrap;
10. preserve is unreachable until complete live evidence and fresh independent
    Result-Evidence Review both pass.

The harness must not infer consumption from socket readiness. Its terminal
output reports the explicit bootstrap-call count, consumption bit, rollback
count, restart count, and closed result only.

## 5. Exact live paths and authority

Production mode is compiled into the evidence harness as the already-reviewed
fixed path set:

- service label `com.earendilworks.loom.runtime-observer`;
- original product root under the user's Loom Application Support directory;
- default product run directory and `loomd.sock`;
- user Applications `Loom.app`;
- bundle identifier `com.earendilworks.loom.local`.

Production mode accepts no path, uid, service label, socket, plist, executable,
bundle identifier, Provider, Runtime, Team, Journal, or command override from
arguments or ambient environment.

The only non-production mode is deterministic fixture mode. It requires both:

1. an explicit `--fixture-root` beneath a fresh private temporary directory;
2. a fixture-owned sentinel created by the test.

Fixture mode cannot address the live service label, live plist, live Journal,
live app destination, or any path outside its canonical fixture root.

## 6. Secret and evidence boundary

The harness and test may emit only closed predicates, digests, modes, counts,
safe reason codes, and fixture-relative names.

They must never emit or persist:

- a process row or complete command line;
- a launchd environment row or value;
- a Provider or credential value;
- raw Grant, prompt, model output, hidden reasoning, Event payload, or SQLite
  row;
- an unbounded log, Accessibility tree, screenshot, or IPC body;
- a private transaction path in repository evidence.

Presence-only checks use the existing frozen marker-name set internally and
return only a count. The harness never executes unrestricted `launchctl print`
or `ps eww` output to its caller; any required command is reduced inside the
same pipeline before output.

## 7. Mandatory RED

After Contract Review `PASS`, the transaction fixture test is added first and
must fail because the reviewed transaction harness does not yet exist.

The RED test specifies behavior, not shell line order:

1. starting from an absent fixture run directory, the fake Candidate builder
   rejects bootstrap unless an owned mode-`0700` parent exists;
2. the old omitted-directory behavior therefore reaches
   `daemon_unavailable`;
3. successful fake bootstrap followed by injected readiness failure must
   report allowance consumed, rollback exactly once, and no second bootstrap;
4. a failure before fake bootstrap must report allowance unconsumed and restore
   the exact fixture pre-state;
5. fixture escape, symlink parent, wrong owner where supported, wrong mode,
   unknown decision, and ambient override attempts must fail closed.

RED changes no live state and leaves the original observer, Journal, installed
paths, crash inventory, Swift cache, and staging exact.

## 8. Deterministic GREEN

The minimum harness implementation makes the RED behavior pass. The fixture
must use real filesystem permissions and sockets plus bounded fake
service-manager commands; it must not use the live LaunchAgent, installed
product, Journal, app, or Computer Use.

Required verification:

```text
sh -n .loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction.sh
sh -n .loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction-test.sh
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction-test.sh
scripts/test-build-loom-local-app.sh
scripts/test-install-loom-local-app.sh
scripts/test-install-loom-local-product.sh
cd apps/macos && swift test
cd apps/macos && swift build -c release
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
git diff --check
git diff --cached --check
```

GREEN must additionally reproduce:

- the frozen Candidate hashes, arm64 LC_UUID, signature, and bundle manifest;
- exact original hashes/modes/provenance names and running observer;
- target-process marker count `0` without emitting a process row;
- unchanged SQLite hash, integrity, one Event, and canonical view digest;
- exact two-report crash inventory;
- absent Candidate app/run/socket/launcher/native process and Swift cache;
- empty staging.

Fresh independent Implementation Review must inspect the harness, run its
fixture, and reproduce the external predicates without mutation.

## 9. Post-Review live gate

Only after Contract Review, RED, GREEN, full verification, Implementation
Review, and a new post-Review activation audit all pass may the user grant one
final Candidate bootstrap by sending exactly:

```text
P2A-W1 Final Native Launch Transaction Closure and one controlled native-window canary
```

No earlier message substitutes.

The canary must execute the exact reviewed harness and close in one run:

1. exact Candidate and original preflight;
2. run-root preparation before bootstrap;
3. one initial Candidate bootstrap and typed daemon snapshot;
4. Computer Use against only `com.earendilworks.loom.local`;
5. all eight frozen screens plus empty-Team activation;
6. native quit/relaunch;
7. one required Candidate daemon lifecycle restart;
8. Journal/view/crash/secret-negative recovery proof;
9. rollback on any failure;
10. preserve only after full live `PASS` and fresh independent Result-Evidence
    Review `PASS`.

There is no second run, alternate socket, Terminal/CLI/PTY evidence
substitution, Provider/Runtime execution, Journal write, hidden retry, or
further P2A-W1 closure split.

## 10. Exit

Only complete live `PASS`, exact preserved Candidate state, fresh independent
Result-Evidence Review `PASS`, and one atomic P2A-W1 commit unlock P2A-W2.

Any failure terminates:

```text
FAIL — ROLLED_BACK — HUMAN_REQUIRED
allowance: 0
P2A-W2: LOCKED
```

Until the exact post-Review activation exists, the terminal status is:

```text
P2A-W1:
HUMAN_REQUIRED — FINAL EXIT CLOSURE LIVE LOCKED

P2A-W2:
LOCKED
```
