# P2A-W3 Authoritative Acceptance Time and Terminal Closure Contract

**Date**: 2026-08-03  
**Status**: `FROZEN / PENDING INDEPENDENT CONTRACT REVIEW`  
**Risk**: `STRICT / AUTHORITY REOPEN`  
**Baseline commit**: `848f068cbc0307f14473f6a71961db949a8734ca`  
**Parent**: unique `P2A-W3 Controlled Execution Experience`  
**Replacement Result Review SHA-256**:
`65ed939fa5c76e74ca449479958884ef944cad5e5eed38c7f7dc2bb6e734cc01`  
**New WorkItem**: none; `P2A-W4` does not exist

## 1. Basis and stop-state

MiniMax-004 passed. Pi-005 live-proved the complete source and independent
Verifier WorkItem/Run/generation/Grant/capacity/Evidence lineages, but stopped
after Verifier `EvidenceSubmitted`: no `WorkItemVerificationCommitted`,
`TeamNodeAcceptanceCommitted` or `TeamExecutionTerminal` appeared. The fresh
Result Reviewer accepted that evidence and kept the product `HUMAN_REQUIRED`.

Read-only diagnosis identifies a causal boundary that must be proven RED:
Mission compilation freezes `TeamExecutionRequest.AuthoritativeTime`; the
Coordinator later builds `AcceptanceDecision` with that old instant; Work
Authority obtains a fresh operation time after source plus Verifier execution
and requires exact equality before appending acceptance and terminal Events.
Fixed-clock tests can make this invalid live invariant appear correct.

No consumed live lineage may be reused. This contract reopens the accepted
Work Authority only for authoritative acceptance decision time and the adjacent
Coordinator terminal path; it is not a retry-only amendment.

## 2. Binding invariants

- Event Journal remains the only state authority. Projection remains a
  rebuildable cache.
- Work Authority, not Coordinator, Agent, Runtime, UI or caller, owns the final
  acceptance decision time and acceptance Event transaction.
- Source and independent Verifier lineage, generation fencing, Grant,
  Evidence, capacity, CAS, recovery policy and executor-cannot-mark-done
  invariants remain unchanged.
- No Event type/schema, Journal primitive, Projection schema, StateWriter,
  Rules policy, Supervisor, Pi RPC/adapter, Provider, credential, native/IPC
  protocol, scheduler or queue changes.
- No hidden retry, fallback, terminal synthesis, second authority or direct
  SQLite mutation.
- Raw Grant, credentials, model output, paths, hidden reasoning and private
  errors remain outside public IPC, logs and Journal payloads.
- The unrelated `demo-resident` observer remains untouched.

## 3. Boundary A — authority-owned acceptance decision

`TeamNodeAcceptanceInput` must no longer carry a caller-created final
`AcceptanceDecision`. It carries the already frozen acceptance contract,
deterministic result, optional independent Verifier Candidate/receipt, recovery
policy and exact source lineage.

For a new acceptance transaction, Work Authority must:

1. validate source and Verifier lineages, receipts, semantic digests, Team node,
   current generation and stream heads before authority mutation;
2. obtain exactly one UTC operation instant from its injected authority clock;
3. call the existing pure `verification.DecideAcceptance` itself with that
   instant;
4. use the resulting decision and the same instant for
   `WorkItemVerificationCommitted`, `WorkItemDone|Rejected`,
   `TeamNodeAcceptanceCommitted` and, when terminal, `TeamExecutionTerminal`;
5. append the complete batch through the existing
   `AppendBatchIfStreamHeads` CAS only.

The repair must not merely delete the time check while retaining caller time
authority, accept a stale Start timestamp, round/truncate clocks, widen a time
tolerance, or make system time part of idempotency ambiguity.

## 4. Boundary B — exact idempotency and Coordinator routing

An already committed exact acceptance must replay idempotently without
constructing a different current-time decision. Existing acceptance may be
recognized only after exact Team/source/Verifier/receipt/contract/result/policy
lineage validation. A semantic mismatch, different receipt, generation drift,
conflicting existing outcome or stream-head race remains a typed conflict.

The Coordinator must stop constructing the final acceptance decision. After
Work Authority returns the authoritative Team record, it may:

- return success only from the authoritative accepted node/Team status; or
- schedule the existing verification-rejected recovery using the returned
  node's authoritative acceptance decision digest and recovery state.

It may not infer acceptance from Runtime success, Evidence presence or a local
Verifier result alone.

## 5. Mandatory RED and vertical proof

Before behavior change, a new causal test must use a UTC clock whose successive
calls return distinct increasing instants. It must cross the actual product
execution composition with deterministic source and independent Verifier
executors and reproduce the current stop:

```text
source Run + Evidence
-> verifier Run + Evidence
-> ErrInvalidWorkItemAcceptance
-> zero TeamNodeAcceptanceCommitted
-> zero TeamExecutionTerminal
```

After repair, the same advancing-clock path must prove:

- exactly two distinct terminal Runs, Grants and Evidence receipts;
- exactly one WorkItem verification/outcome, node acceptance and Team terminal;
- authoritative decision time is later than the initial plan/start time;
- acceptance batch Events share the one authority-owned operation instant;
- final Team status is `succeeded`; Snapshot and Timeline agree;
- reconnect/read-only refresh adds zero Events and redispatches nothing;
- replay of the exact acceptance is idempotent even though the clock advances;
- mismatched lineage and injected clock/CAS failure append zero partial Events.

The existing fixed-clock matrices remain and must still pass.

## 6. Exact owned files

Only these production/test files may change after Contract Review PASS:

```text
internal/work/verification_authority.go
internal/work/verification_authority_test.go
internal/app/team_execution.go
internal/app/team_execution_test.go
cmd/loomd/product_daemon_test.go
```

Governance/evidence may change only under:

```text
.loom-evidence/phase2a/P2A-W3/
docs/CURRENT.md
```

All other source and accepted authority files remain locked. If causal RED
requires another production file or Event/schema change, stop `HUMAN_REQUIRED`
instead of expanding scope.

## 7. Verification and review gates

In order:

1. immutable contract hash and fresh independent Contract Review PASS;
2. causal advancing-clock RED before behavior change;
3. focused Work Authority, Team Coordinator and product vertical tests;
4. repeated and race tests including advancing clocks, idempotent replay and
   concurrent acceptance CAS;
5. complete serialized repository normal/race, vet, module, format, diff,
   locked-authority, protocol, secret and non-disclosure gates;
6. existing Swift normal, ThreadSanitizer and arm64 Release gates unchanged;
7. immutable source lock and fresh independent Implementation Review PASS.

Review must fail for caller-owned decision time, tolerance/rounding, weakened
lineage/generation/CAS, non-idempotent time-dependent replay, duplicate terminal,
partial batch, recovery drift, hidden retry, authority/file-scope expansion,
raw error disclosure or locked-file drift.

## 8. Final live allowance

Contract freeze and Contract Review authorize no live action.

After Implementation Review PASS only, at most one wholly new Pi lineage may
be frozen. It must use a fresh 0700 root, 0600 pre-created retained six-fact
SQLite, exact source/artifact/external hashes, canonical 32-token argv
attestation, one daemon invocation, one preflight, one Start, zero Provider
requests, no retry/compaction, complete postflight and fresh independent Result
Review. It must prove source + Verifier Evidence and exactly one canonical Team
terminal through the ordinary native product surface.

If it fails, stop. No further replacement, walkthrough or commit is authorized.

## 9. Exit condition

Only Contract Review, causal RED, implementation, full deterministic matrix,
Implementation Review, one final live result, Result Review, parent no-terminal
walkthrough, final lock and atomic W3 commit may accept P2A-W3.

Until Contract Review PASS:

```text
P2A-W2 = ACCEPTED
P2A-W3 = HUMAN_REQUIRED / ACCEPTANCE-TIME TERMINAL CONTRACT REVIEW PENDING / NO LIVE / NO WALKTHROUGH / NO COMMIT
P2A-W4 = DOES NOT EXIST
```
