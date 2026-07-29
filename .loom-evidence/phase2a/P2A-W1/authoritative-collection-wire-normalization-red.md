# P2A-W1 Authoritative Collection Wire Normalization RED

**Date**: 2026-07-29  
**Result**: `PASS - BEHAVIOR RED CONFIRMED`  
**Live action**: none

## Genuine focused RED

The test-first snapshot regression called the shared clone/wire boundary with
nil top-level collections. With production source unchanged, the test failed
on the exact missing invariant:

```text
go test ./internal/api \
  -run TestLocalProductSnapshotCloneCanonicalizesAllRequiredCollectionsForWire \
  -count=1

teams:null
runs:null
evidence:null
attention:null
FAIL
```

The same payload already emitted Runtime `model_ids:[]` and
`observed_capabilities:[]`, proving the RED is the incomplete shared top-level
normalization rather than the previously repaired historical Runtime path.

An initial sandboxed run stopped before compilation because the user Go build
cache was inaccessible. It was discarded and rerun with `GOCACHE` under
`/private/tmp`; only the exact assertion failure above counts as RED.

## Real cross-language control

The new component fixture uses:

```text
historical model_ids:null Journal Event
-> Projection
-> LocalProductReadService
-> production localProductHandler
-> real localipc.Server over a private UDS
-> compiled LoomLocalAppContractProbe
-> strict Swift LocalIPCClient
```

The sandbox blocks local Unix socket readiness, as independently reproduced by
an unchanged existing Server lifecycle test. The same bounded fixture was then
run outside the socket sandbox, with no network, resident socket, Provider,
Runtime, launchd, installation, or App access, and passed:

```text
TestProductDaemonServesAuthoritativeNilCollectionsToStrictSwiftClient
PASS
```

This honest pre-repair control proves the previously repaired nested Runtime
path reaches the strict Swift client as an empty array. It is not represented
as a failing product RED.

No production source or Swift source had changed when these results were
captured.
