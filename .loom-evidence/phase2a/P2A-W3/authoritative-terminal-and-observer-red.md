# P2A-W3 Authoritative Terminal and Observer RED

**Date**: 2026-08-02  
**Contract Review**: `PASS`  
**Status**: `CAUSAL RED CONFIRMED`

No production file was changed before these tests.

## Credential terminal RED

```text
go test -count=1 ./internal/credentials \
  -run 'TestCredentialBroker(StoreFailure|StoreFailureTerminal)'
```

Failed exactly because all `not_found|denied|unavailable|unknown` Secret Store
read failures returned immediately, made zero metadata commits, and yielded the
zero result. The bounded-commit recorder was never called and the forced commit
failure fixture was never reached. This causally reproduces the live shape in
which Swift can render a local `Unavailable` error but no next-revision Journal
fact exists.

## Observer attribution RED

```text
go test -count=1 ./cmd/loomd \
  -run 'TestObserverFailureReasonIsClosedTypedAndNonDisclosing|TestRunFreezesCompleteObserverFailureReasonAllowlist'
```

Failed only on the newly frozen closure:

- candidate and construction failures collapsed to `observer_probe_factory`;
- inventory and plan failures collapsed to `observer_unknown`;
- identity metadata collapsed to `observer_write`;
- a direct Journal write conflict collapsed to `observer_unknown`; and
- the public daemon allowlist rejected the five new safe reason codes.

The existing version/models/binding/projection/legacy-write/unknown matrix
continued to match. No live process, Keychain, Provider, Pi, native app, state
mutation, retry, staging, or commit occurred.
