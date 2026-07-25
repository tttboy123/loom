# S2-W36 Candidate Deliverable

- WorkItem: `S2-W36`
- Title: Prepared Projected Configured Runtime Observer
- Risk: Strict
- Candidate state: `ACCEPTED_PENDING_LOCAL_COMMIT`
- Branch/head: `codex/loom-platform-slice2` at `c8ecc2a`
- Contract SHA-256:
  `391e5c7917bf3745185219cc4b4ed3bcce2c5e48d39f3a043bc31c99697d3e0c`
- Contract review SHA-256:
  `f8c792394e5630d30038d3ce9dea940c972d9fd2f01572215cf5de4e2f3d572c`

## Contract, RED, and Candidate

Fresh contract review returned `PASS` with no findings. Mandatory RED added
only `runtime_observer_test.go`; focused compilation failed solely because the
frozen observer type, constructor, and error symbols were missing.

The minimal Candidate rejects only a nil read model during construction,
shallow-copies the configured factory slice, retains the accepted path-scoped
committer semantics, and performs no construction-time work. A nil receiver
returns five zero outputs plus the frozen error. Every non-nil `RunOnce`
delegates exactly once to S2-W35 and returns its exact outputs/error.

```text
a9421c3b58de00a9a9d44d02f9c511cdf603bf9423a78b18237ea783bd269213  internal/app/runtime_observer.go
ee80f7028fb66a8f68ad4cfc9dcb755591cad6e067ea468451cfde60af1db23a  internal/app/runtime_observer_test.go
```

## Controller verification

The complete strict matrix passes: focused, app, app/runtime/discoveryscan/
state/projection/journal impact, focused-race-50, repository,
repository-race, vet, formatting, and diff.

Focused proof covers nil read-model construction, nil receiver five-zero
behavior, no construction-time dependency calls, caller factory-slice
replacement isolation, exact none/discovery-priority/status delegation,
path-scoped typed-nil committers, nil context propagation, configured
discovery error propagation, explicit retry identity, result accessor
mutation isolation, and static S2-W35-only composition.

The real SQLite proof binds prepared discovery and status observers around a
real accepted projection and committers. Two explicit discovery calls leave
only discovery sequences 1 and 2; two explicit status calls add only status
sequence 3. Event types are exactly discovery/discovery/status, opposite
writers are unused, and a final rebuild proves the changed display name,
online status, discovery sequence 2, and status sequence 3.

## Trust boundary

The product has no exported binding field or accessor and adds no cache,
mutex, retry counter, sequence allocator, timer, goroutine, lifecycle state,
configuration/file/process/network access, direct lower-layer composition,
Journal/SQLite/Event metadata, projection rebuild, scheduler/daemon,
activation, or Slice 3 authority.

## Review gate

Fresh Implementation Review 1 returned `FAIL` on one direct error-matrix proof
gap and found no product defect. Repair 1 is frozen as test-only with the
product hash locked byte-for-byte. Fresh Repair 1 contract review returned
`PASS` with no findings. Mandatory Repair RED failed only on all twelve missing
canonical case markers.

The test-only repair now directly covers canceled/deadline context, invalid
identity drift, missing and typed-nil discovery/status committers, both writer
errors, both result mismatches, and cancellation after a selected discovery
write. Every case proves five zero outputs, exact selected/opposite call counts,
and no fallback or retry. The complete strict matrix passes again and product
remains byte-for-byte unchanged. Fresh Implementation Review 2 returned
`FAIL` on one remaining
test-proof omission and again found no product defect: the configured-discovery
failure used anonymous committers and therefore did not assert zero calls.
Because this is the second same-class failure, fresh read-only problem analysis
was mandatory before Repair 2. That analysis confirmed a false-green
traceability/observability gap and no product defect. Repair 2 is now frozen as
test-only around the exact configured-discovery call tuple `1/0/0`; fresh
Repair 2 contract Review 1 returned `FAIL` because two global call-count
markers already existed outside the target case, so the proposed RED was not
case-local. No implementation began. Amendment 1 now requires unique local
factory/committer variable markers inside `configured_discovery_failure`;
fresh amendment review returned `PASS` with no findings. Mandatory Repair 2
RED failed only on all four missing unique case-local markers.

The repaired configured-discovery case now uses one named factory that returns
a sentinel plus named discovery/status committers. Its one `RunOnce` proves the
exact sentinel, five zero outputs, and runtime call tuple
`factory/discovery/status = 1/0/0`. The complete strict matrix passes again,
the product remains byte-for-byte unchanged, and fresh Repair 2 implementation
review returned `PASS` with no findings after independently rerunning the
complete strict matrix. The Candidate is accepted and may receive its one
exact-scope local atomic commit after a fresh pre-commit matrix and staged-
scope audit.

VERDICT: PASS
