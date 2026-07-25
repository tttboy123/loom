# S2-W32 Candidate Deliverable

- WorkItem: `S2-W32`
- Title: Discovery-Priority Runtime Observation Write Planning
- Risk: Strict
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Branch/head: `codex/loom-platform-slice2` at `b00f8d8`
- ADR-0007 SHA-256:
  `17ac8f1f22ad87d85eae8f6004dd7e3194fcd944c6244400cf191b4465aa6df6`
- Contract SHA-256:
  `b6c594213350e7a0c1d5bf1dbf5bc7de429d02c8cc04833efbba6f49625ea77b`
- Contract review SHA-256:
  `d7c230849f73abcfcdd1b9f1f1792915ecd8316a0b28dd2443aad2a25ea0c5bb`

## Contract, RED, and Candidate

Fresh contract review returned `PASS` with no findings. Mandatory RED exited
`1` only on the missing frozen error, kind, Candidate, and planner symbols.

The Candidate composes accepted S2-W25 projection validation and S2-W22
discovery/identity/status validation, then compares exact canonical display
name, executable version, capabilities, capacity, models, and source probe.
Current-only or changed inventory selects `discovery`; status-only changes
select `status`; unchanged, empty, or absent-only input selects `none`.
Discovery wins mixed observations while retaining the status-transition count
as audit evidence.

```text
0a7ac25678d71e50f03d81744e8159b1d4d3e4aa663de3a5d3c53a191a3c721b  internal/app/runtime_write_plan.go
57eace7191cfd3c9289bc0b3d0115b03f6acac7b62317e694ecc09003c922f8d  internal/app/runtime_write_plan_test.go
```

## Controller verification

The complete strict matrix passes:

- focused S2-W32;
- app package;
- app/runtime/projection impact;
- focused race at `-count=50`;
- repository and repository-race;
- vet, formatting, and diff;
- accepted ADR-0007 index/link;
- every inventory field, one/multiple status transitions, mixed precedence,
  absence, current-only, invalid/oversized projection, invalid discovery,
  stable identity drift, deterministic cancellation, map-order independence,
  per-fact digest sensitivity, input immutability, and zero Candidate; and
- static no-writer/no-Journal/no-metadata/no-scheduler/no-daemon/
  no-execution-authority boundary.

The product calls no writer or committer and retains no mutable collection.
It adds no Event metadata, retry, unified sequence authority, absence
inference, Journal/SQLite, configuration, scheduler, daemon, process, Runtime
activation, external action, or Slice 3 behavior.

## Review gate

Fresh Implementation Review 1 returned `FAIL` only because the canonical
Candidate digest omitted the exposed `Planned()` fact. Repair 1 is frozen to
add that one existing fact to the payload and its direct sensitivity proof:

```text
e3aaf143d627bd483fab544c930e3ef852f74259dfaaf1f5a15ff28aef3c4daa  implementation-review-1.md
2bab135ee0e8892345e0acae98b5ad69b06e817e7427839cac4f2bb2530547f5  implementation-repair-1-contract.md
6a1adb5210104a0c278109c8a5cdfbe366f4a0646977ed45d59413ff88c85b64  implementation-repair-1-contract-review.md
```

Fresh Repair 1 contract review returned `PASS`. Mandatory Repair RED exited
`1` only because mutating `planned` did not change the digest. The minimal
repair adds `planned` to the versioned payload and copies the exact Candidate
fact. The focused repair and complete strict matrix pass again; no
classification, API, import, or authority behavior changed.

Fresh independent Repair 1 implementation review returned `PASS` with no
findings after independently passing the complete strict matrix. The Candidate
is accepted and may receive its one scoped local atomic commit after a fresh
pre-commit matrix and exact staged-scope audit.

VERDICT: PASS
