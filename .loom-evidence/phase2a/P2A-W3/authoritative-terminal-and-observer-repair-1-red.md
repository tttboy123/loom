# P2A-W3 Authoritative Terminal and Observer Repair 1 RED

**Date**: 2026-08-02  
**Cause**: Implementation Review 1 P1 findings  
**Status**: `CAUSAL RED CONFIRMED`

Before changing production classification, the observer matrix was tightened
to require `observer_unknown` for all four previously excepted shapes:

```text
probe factory + unknown leaf
projection + unknown leaf
write + unknown leaf
identity metadata + unknown leaf
```

The focused command failed all four cases exactly because the implementation
returned the corresponding concrete reason:

```text
go test -count=1 ./cmd/loomd \
  -run TestObserverFailureReasonIsClosedTypedAndNonDisclosing
```

The retained-state evidence gap was independently causal from source
inspection: the former test created a fresh database and appended one synthetic
Runtime fact. It did not read or copy the retained six-fact live database, and
therefore could not prove its saved-Team facts or exact database bytes stayed
unchanged.

No live process, Provider, Secret Store, Pi Runtime, native app, state mutation,
staging, or commit occurred during RED.
