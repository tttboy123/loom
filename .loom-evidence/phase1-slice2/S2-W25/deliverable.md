# S2-W25 Candidate Deliverable

- WorkItem: `S2-W25`
- Title: Runtime Status Baseline Projection Adapter
- Risk: Strict
- Candidate state: `ACCEPTED`
- Branch/head: `codex/loom-platform-slice2` at `9838779`
- Repaired contract SHA-256:
  `52e04294aaf86313b09bb769be134095921fdeb6a3818e412b4565e2650985e3`
- Active Implementation Repair 1 amended contract SHA-256:
  `d47939a110667b640cb3f264a7edb74557ee9e0988e99928549cd358cd297a49`
- S2-W22 amendment SHA-256:
  `737585e1e8084e21e3287b145211b145e2bff4eac692b221bb5c88e5162cbcc7`

## Contract lineage

Contract Review 1 returned `FAIL` because the original status validation could
accept a forged first-link pair where discovery and previous Event
ID/sequence equality disagreed. Contract Repair 1 requires:

```text
StatusPreviousEventID == DiscoveryEventID
if and only if
StatusPreviousSequence == DiscoverySequence
```

Fresh independent Contract Repair 1 Review 2 returned `PASS` with no blocking
findings. No product or test file changed before that PASS.

## Candidate

The Candidate adds a pure bounded adapter from copied S2-W24 Runtime projection
records to renamed S2-W22 status baselines. It:

- accepts zero through 32 records and rejects oversize without truncation;
- validates map identity, canonical Runtime core, model order, complete
  discovery provenance, and all-or-none status provenance;
- rejects forged discovery/previous pair mismatches in both directions;
- selects the latest status Event after one or consecutive status changes;
- selects the latest discovery Event before status or after rediscovery;
- returns deterministic ID order and copied Runtime/capability data; and
- returns no partial baseline on invalid input.

The contained S2-W22 amendment renames discovery-specific baseline provenance
to generic previous status-bearing provenance and increments the baseline
digest version from `1` to `2`. Transition and Candidate shapes remain
unchanged.

## Mandatory RED

Before product changes, the focused command exited `1` only because
`BuildRuntimeStatusBaselines` and
`ErrInvalidRuntimeStatusBaselineProjection` did not exist. No syntax,
dependency, environment, or unrelated failure occurred.

## Candidate file digests

```text
89e10f9eacb790207be1b56bffc1fce954414988e970a1d43f15f61f42539123  internal/runtime/status_reconciliation.go
effa3c4e24915db6387fc6ba6768b4e97de61af249875c5b2f46e4d9e0f10bf6  internal/runtime/status_reconciliation_test.go
4bda177bc4280405f7ac1c6814b2b56eab5e003ab892a88eca5bf4851cff80e8  internal/state/runtime_status_writer_test.go
e11f1a59599e2aacde5c59c9a1e2a7739fbd34b43b7b7cd5b25347afcca43351  internal/projection/runtime_status_test.go
b39b8183c3279d4c1bf5f92846d44b346a83d7f65f7b6aa702afce5085a68b86  internal/projection/runtime_status_baseline.go
51a58e604bff218889aa8823ef087f4a9d477bbaaf9787481e0459b0e486c317  internal/projection/runtime_status_baseline_test.go
```

## Implementation Repair 1

Implementation Review 1 found that per-record validation still admitted Event
ID reuse across different Runtime records. After the fresh Repair 1 contract
Reviewer returned `PASS`, mandatory Repair RED added all nine combinations among
discovery, current status, and previous status roles. Every case failed because
the reviewed Candidate returned a non-error baseline.

The minimal repair adds one in-memory Event ID to Runtime owner table. It
registers discovery identity for every record and current/previous status
identity only for complete status records. A repeated identity is accepted only
for the same Runtime owner, preserving the legitimate first-status
discovery/previous alias. Any other cross-record reuse fails with no partial
baseline.

The full Controller matrix below was rerun after Repair 1 and passed without a
transient failure.

## Controller verification

| Check | Result |
|---|---|
| focused S2-W25 adapter | `PASS` |
| renamed S2-W22 reconciliation | `PASS` |
| S2-W23/S2-W24 writer/projection regression | `PASS` |
| runtime/state/projection packages | `PASS` |
| runtime/state/projection/journal impact | `PASS` |
| focused race `-count=30` | `PASS` |
| `go test ./... -count=1` | `PASS`, all packages green |
| `go test -race ./... -count=1` | `PASS`, all packages green |
| `go vet ./...` | `PASS` |
| frozen-file `gofmt -d` | `PASS`, no output |
| `git diff --check` | `PASS`, no output |
| old discovery-specific field search | `PASS`, no product/test matches |
| import/non-disclosure/scope checks | `PASS` |
| branch/head | `PASS`, `codex/loom-platform-slice2` at `9838779` |

Focused evidence covers:

- nil/empty, discovery-only, first status, consecutive status, and rediscovery
  selection;
- deterministic map-order normalization, fresh-call equality, canonical Runtime
  conversion, and input/output slice isolation;
- map-key mismatch, oversize, invalid Runtime/capability/model/discovery fields,
  non-UTC times, all ten partial status-field variants, invalid digests/source/
  Event IDs/sequences, exact-next/overflow/discovery-order failures, and both
  forged pair-mismatch directions;
- generic previous status-bearing provenance through S2-W22 transitions and
  version-2 baseline digest sensitivity; and
- all nine same-role/cross-role Event ID reuse combinations across Runtime
  records, plus the legitimate same-record first-status alias; and
- static pure-adapter import/no-write/no-replay/no-discovery/
  no-reconciliation/no-execution/non-disclosure boundaries.

## Trust and residual state

Accepted S2-W24 Journal replay remains authority for the Event chain that
produced a Snapshot. The adapter revalidates one copied record but does not
reconstruct omitted history or create a second state authority. S2-W22 remains
pure reconciliation authority; S2-W23 remains Runtime status Event construction
and append authority.

No Journal query/replay, Event append, StateWriter, discovery/reconciliation
invocation, direct SQL/schema/config/file write, Pi process, installed Pi, user
Pi state, credential, network, scheduler, daemon, Agent session, prompt, model
call, Runtime selection/reservation/activation, push, merge, rebase, reset,
release, or external mutation was used.

## Review gate

Fresh independent Implementation Review 1 returned `FAIL` because the adapter
did not reject cross-record Event ID reuse that accepted Journal replay cannot
produce. The bounded Implementation Repair 1 amended contract at
`d47939a...7a49` passed fresh independent contract review with no blocking
findings. Mandatory Repair RED failed on all nine required cases; the minimal
owner-set repair and complete strict matrix are now GREEN. Fresh independent
Repair 1 implementation review independently passed the full matrix, confirmed
Review 1 closure, and returned `PASS` with no findings:
`.loom-evidence/phase1-slice2/S2-W25/implementation-review-2.md`.

VERDICT: PASS
