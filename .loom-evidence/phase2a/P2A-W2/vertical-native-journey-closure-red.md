# P2A-W2 Vertical Native Journey Closure Mandatory RED

**Date**: 2026-07-30
**Contract**: `Vertical Native Journey Closure Repair`
**Baseline**: `326f797`
**Result**: `RED — EXPECTED PRODUCT GAPS REPRODUCED`

## Scope

Only deterministic local fixtures were used. No installed Pi, Codex process,
Keychain item, Provider network, native app, resident daemon, LaunchAgent or
live canary was invoked.

## RED results

The pre-production Candidate failed at the four frozen boundaries:

```text
TestCredentialBrokerCommitsObservedVerificationAfterCallerCancellation
  Verify() error = context canceled

TestKeychainReadExplicitlyDisablesAuthenticationUI
  Keychain query does not explicitly fail closed on auth UI

TestServerUsesExtendedDeadlineOnlyForCredentialVerify
  credential_verify deadline remaining = 4.999998667s, want (9s, 10s]

TestProductSetupRefreshesRuntimeCatalogAfterServiceConstruction
  SetupSnapshot retained only runtime-legacy, no role options
```

The setup fixture constructs the service after five Events and then appends the
sixth Event, matching the failed attempt-004 Event classes:

```text
ProviderCredentialConfigured: 1
ProviderCredentialVerified:   3
RuntimeInstanceDiscovered:    2
```

The first Runtime has `model_ids:null`; the sixth Event introduces a distinct
online model-capable Runtime. Before repair, Projection contained both Runtime
facts while the setup service retained its construction-time catalog.

## Preserved passing boundary

`TestCredentialBrokerStoreFailureBeforeObservationCommitsNoFact` passed before
production changes. This proves the repair did not need to weaken the existing
pre-observation Secret Store failure behavior.

No product source was modified before these expected failures were captured.

VERDICT: PASS
