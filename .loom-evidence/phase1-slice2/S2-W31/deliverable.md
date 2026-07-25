# S2-W31 Candidate Deliverable

- WorkItem: `S2-W31`
- Title: Projected Runtime Status Reconciliation Coordination
- Risk: Strict
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Branch/head: `codex/loom-platform-slice2` at `47f225b`
- Contract SHA-256:
  `887d8a75237c7e578e966435f2e9701d2a5c3fec6d1b6729af7dcaf055c4749c`
- Contract review SHA-256:
  `d129fd4307d6832c0393fc0bd128a28810c33327d8e43e0da7faced68a710942`

## Contract, RED, and Candidate

Fresh contract review returned `PASS` with no findings. Mandatory RED exited
`1` only on the missing frozen error/coordinator symbols.

The Candidate validates the injected committer/context, builds the exact S2-W25
baseline once from a caller-supplied copied projection Snapshot, checks context,
and delegates exact immutable inputs once to S2-W29. It returns exact
Candidates/errors and adds no Journal query/rebuild, discovery write, metadata,
write-policy, scheduler, daemon, activation, or Slice 3 authority.

```text
bb9b96a6c13f7e6d23a7ce61450143078a7b4a916a77f59c8b50059be0d6a40f  internal/app/runtime_status_projection.go
083a3a8ca6d62d89010a8cf24fab3b9c6b2feec87b0028868f773eba3b6e3169  internal/app/runtime_status_projection_test.go
```

## Controller verification

Focused, app package, app/runtime/state/projection/journal impact,
focused-race-50, repository, repository-race, vet, formatting, diff, import,
mutation, no-policy, and scope checks pass. A real temporary SQLite test appends
prior discovery facts, rebuilds the accepted projection, then runs
S2-W31→S2-W30→S2-W23 for exact status append and explicit exact retry without
appending the current discovery snapshot.

## Implementation Review 1 and Repair 1

Fresh Implementation Review 1 returned `FAIL` on three test-proof gaps while
finding no product-boundary defect:

1. no status-bearing projection proof selected `StatusEventID` and
   `StatusSequence`;
2. only one invalid projection defect was exercised at the S2-W31 boundary; and
3. S2-W29 error/zero-output propagation plus Candidate accessor isolation was
   incomplete.

Repair 1 is frozen and independently reviewed `PASS` as test-only:

```text
f3add131fb15b747799578448bcf7b5c6c0b4388358847f6b866c2ee604bb878  implementation-review-1.md
fb48e87db1bead28c19aee8fe83aa8fe5aa5f5e9f3425f03891c8f4dd7f59765  implementation-repair-1-contract.md
e8e2b07d3ac0eff23e36c3ef8e43a246a6f58d883648b90513afa357326485be  implementation-repair-1-contract-review.md
```

Mandatory Repair RED exited `1` because the existing discovery-only fixture
returned sequence `1` discovery provenance while the new status-bearing
assertion required sequence `2` status provenance. Repair GREEN adds canonical
first-status, consecutive-status, and rediscovery-reset cases; exact sentinel
proof for ten invalid projection classes; invalid discovery, stable-identity
drift, committer, result-mismatch, and deterministic post-baseline context
propagation; plus nested projection, committer-received reconciliation, returned
reconciliation, and returned commit Event accessor isolation.

The complete strict matrix passes again: focused, app package,
app/runtime/state/projection/journal impact, focused race `-count=50`,
repository, repository race, vet, gofmt, and diff. The product remains
byte-for-byte unchanged at its pre-repair digest.

Fresh Repair 1 Implementation Review 1 accepted the provenance and propagation/
mutation closures but returned `FAIL` because the invalid-projection table
separated key and Runtime-core defects without a stable-identity defect. The
same frozen Repair 1 contract already required that case, so the bounded
test-only correction adds one valid-key projection whose `DeviceID` is empty.
That focused identity case and the complete strict matrix pass. The independently
observed one-off Pi-adapter metadata-command failure also passed its targeted
and full reruns and was not classified as an S2-W31 finding.

## Review gate

Fresh independent Repair 1 implementation re-review returned `PASS` with no
findings after independently passing the complete strict matrix. The Candidate
is accepted and may receive its one scoped local atomic commit after a fresh
pre-commit matrix and exact staged-scope audit.

VERDICT: PASS
