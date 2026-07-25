# S2-W29 Implementation Repair 1 Contract

- WorkItem: `S2-W29`
- Status: `REPAIR_CONTRACT_FROZEN`
- Active contract SHA-256:
  `9a56217e50d0e1a93e37b33bd043c1dbd8c6fa11b762b66000264c71fab97dbe`
- Pre-repair product SHA-256:
  `127ccf3609da4e1bed76cb832f6ec67b1625ca9444aa458dd5f464a2b4ad6e11`
- Pre-repair test SHA-256:
  `9742ee4191cea2d785f3a7ad2f626e7937ccfbb5bc8b2736371ec112baba533c`
- Trigger: S2-W29 Implementation Review 1 `FAIL`

## Repair scope

Close exactly the three Review 1 findings without changing the frozen public
API, Event/status semantics, imports, delegation count, or authority boundary.

### 1. Post-reconciliation context gate

Move the existing post-reconciliation `ctx.Err()` check before the
zero-transition return. A valid no-change reconciliation may return success
only after that check passes.

Add a deterministic test context whose `Err()` becomes
`context.Canceled` only at the first coordinator-observed boundary after a
one-observation no-change reconciliation. Before the product repair, the test
must fail because the coordinator returns the reconciliation as success. After
repair it must return two zero Candidates plus canonical `context.Canceled`,
with zero committer calls.

### 2. Isolated public-result validation

Introduce one private read-only result interface containing exactly the seven
S2-W23 public accessors already used by the validator:

```text
Committed
SourceReconciliationDigest
BaselineDigest
SourceDiscoveryDigest
Events
EventCount
CommitDigest
```

The concrete accepted `state.RuntimeStatusCommitCandidate` must satisfy this
private interface and the public coordinator signature remains unchanged.

Add a direct table over one known-valid result view. Each case mutates exactly
one fact while all earlier and later facts remain valid:

- committed flag;
- reconciliation digest;
- baseline digest;
- discovery digest;
- Event count;
- Event accessor length; and
- digest shape.

The unmodified view must pass. Every single-field mutation must fail. Existing
coordinator-level zero/unrelated-result tests may remain as integration proof
but cannot substitute for this isolated matrix.

### 3. Real static boundary assertions

Replace the no-op AST inspection with assertions that:

- reject every `go` statement;
- retain the exact import allowlist;
- reject direct Journal append/projection-baseline/discovery execution;
- reject time/ID/Event-metadata allocation;
- reject scheduler/ticker/sleep/daemon/activation/process/network/filesystem
  entry points; and
- reject frozen Event envelope/payload construction markers.

The assertions must inspect the complete product AST/source and fail on at
least one explicit forbidden node/token path; a callback that only returns a
boolean is not evidence.

## Mandatory Repair RED

Before product repair:

1. add the delayed-cancellation no-change regression and run only that test to
   observe the current incorrect success;
2. add the isolated result-view table and real static assertions; and
3. run the focused S2-W29 command to observe failure against the unrepaired
   product.

The RED must reflect these Review 1 gaps, not syntax, dependency, or environment
failure.

## Preserved behavior

All other S2-W29 behavior, public symbols, error identity, exact once/zero-call
delegation, zero outputs on error, real SQLite retry, imports, strict matrix,
and explicit exclusions remain unchanged.

## Verification and review

After the minimal repair, rerun the complete active-contract strict matrix.
Fresh independent Repair 1 implementation review `PASS` is required before
acceptance or local commit. This is Repair 1 of at most three.

VERDICT: REPAIR_CONTRACT_FROZEN
