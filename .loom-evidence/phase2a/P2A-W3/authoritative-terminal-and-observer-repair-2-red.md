# P2A-W3 Authoritative Terminal and Observer Repair 2 RED

**Date**: 2026-08-02  
**Cause**: Implementation Re-review 2 P1 findings  
**Status**: `CAUSAL RED CONFIRMED`

Before changing production classification, this same-command incompatible
known-leaf case was added:

```text
Pi version process failure + Pi version stderr failure
```

The focused observer matrix failed exactly because the implementation returned
`observer_version_process` rather than `observer_unknown`:

```text
go test -count=1 ./cmd/loomd \
  -run TestObserverFailureReasonIsClosedTypedAndNonDisclosing
```

The second finding was structurally present before test execution: an unset
environment variable selected a synthetic one-event fallback. Repair 2 removes
that branch and makes the retained six-fact database a mandatory self-contained
fixture for every focused/full invocation.

No live process, Provider, Secret Store, Pi Runtime, native app, state mutation,
staging, or commit occurred during RED.
