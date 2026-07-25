# S2-W28 Implementation Repair 1 Contract

- WorkItem: `S2-W28`
- Risk: Strict
- Repair count: `1`
- Status: `REPAIR_CONTRACT_FROZEN`
- Trigger: `implementation-review-1.md`
- Active contract SHA-256:
  `424554aca51c6d00ff1624e3d5cba386843b4c6aaa2edcf58bcce3e068b1ade8`
- Pre-repair product SHA-256:
  `9daf0916e4fa1f8c5780e1a66c31c79d718d18a13bcea61fec7ef435cb9d6907`
- Pre-repair test SHA-256:
  `14adfaf86f401ae1759954fd7283df82b0b586247826647a8311435efd5c73aa`

## Owned files

- `internal/app/runtime_discovery_committer.go`
- `internal/app/runtime_discovery_committer_test.go`
- `.loom-evidence/phase1-slice2/S2-W28/deliverable.md`

## Frozen repair

At the start of `CommitRuntimeDiscovery`, reject any of:

```text
c == nil
nil/typed-nil c.appender
nil/typed-nil c.provider
ctx == nil
```

with a zero Candidate plus
`ErrInvalidPreparedRuntimeDiscoveryCommitter`, before context, provider, or
appender use.

## Mandatory Repair RED

Before product repair, add a direct test using:

```go
var adapter PreparedRuntimeDiscoveryCommitter
```

The test must fail by panic on the reviewed Candidate. After the minimal repair
it must return the frozen error, zero Candidate, and no call. Constructor and
nil receiver coverage remain unchanged.

No other behavior, API, import, error, delegation, or authority boundary
changes. Rerun the complete strict matrix.

VERDICT: REPAIR_CONTRACT_FROZEN
