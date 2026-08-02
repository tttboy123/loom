# P2A-W3 Complete Live Compatibility Causal RED

**Date**: 2026-08-02  
**Contract hash**: `8e7d77ef06c9867799d5dd759db6c826d77a76a7a2960b17ad55ba35946d5f75`  
**Baseline**: `848f068cbc0307f14473f6a71961db949a8734ca`

The following focused tests were added before production changes and fail for
the intended frozen causes:

```text
go test ./internal/app -run '^TestProjectionMissionExecutionBindingSourceUsesConfirmedExactPiTeam$' -count=1
--- FAIL: ... mission execution conflict

go test ./internal/api -run '^TestLocalProductReadServicePublishesBoundedSnapshotAndPreservesStaleView$' -count=1
--- FAIL: ... DisplayName:"team.delivery" ... want "Release Crew"

go test ./cmd/loomd -run '^(TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC|TestProductCredentialConcurrentLostResponseRecoveryObservesProviderOnce)$' -count=1
--- FAIL: TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC
    metadata timeout stopped product IPC: product daemon failed: observer
--- FAIL: TestProductCredentialConcurrentLostResponseRecoveryObservesProviderOnce
    different stale operation error = <nil>
```

The first TeamDefinition fixture attempt used an empty project generation and
was rejected by Projection before the product assertion. The fixture alone was
corrected to a valid non-empty saved-definition generation, after which the
test reached and reproduced the actual internal-ID display defect above.

The existing deterministic TUI `KeyRunes` test already preserves the exact
objective `Ship the reviewed release`, including spaces, through preflight.
The observed terminal collapse is therefore classified as a PTY/computer-input
artifact; no speculative TUI production change is authorized.

Final fail-closed self-review added one more causal RED before source lock:

```text
TestContainableProductObserverTimeoutRejectsMixedFailureChains
mixed observer chain was containable: safe test Pi metadata failure
    pi metadata binding changed
```

The repair requires every leaf in the containable error tree to match the typed
metadata timeout. A timeout joined with binding, process, cleanup or unknown
failure is therefore fatal rather than silently contained.

Implementation Review 1 then required the same boundary through the real Pi
metadata process path. Replacing the synthetic observer with a deterministic
`pi --version` / `pi --list-models` process fixture produced the intended
additional RED:

```text
TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC
metadata timeout stopped product IPC: product daemon failed: observer
reason=observer_models_timeout
```

The real chain contains the typed timeout plus the structural
`ErrLocalRuntimeObservationDaemonCycle` and `ErrRuntimeDiscoveryFailed`
wrappers. The repair may allow only those structural wrappers around the exact
typed version/list-models timeout; a mixed binding, process, cleanup or unknown
leaf must still remain fatal. The repaired component test also requires real
UDS `ping`, snapshot, setup/Codex status, exactly one `--list-models` invocation,
no hidden retry and clean controlled shutdown.

Implementation Review 2 identified the remaining visibility gap. The repaired
real component assertion first produced this causal RED:

```text
TestProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC
snapshot health = api.LocalProductHealth{
    Daemon:"serving_request", Journal:"available", Projection:"current",
}
```

The first repair used the existing snapshot `Partial`, `Reason` and
`Health.Daemon` fields. It records no Journal fact: the process-local health
source may publish only `observer_version_timeout` or
`observer_models_timeout`, retains Journal `available` and Projection
`current`, and fails closed for any unknown health value.

Implementation Review 3 then identified a cross-client schema incompatibility:
the repaired Go snapshot emitted `health.daemon = "partial"`, while the locked
strict Swift schema v2 decoder accepts only `serving_request`. The daemon is in
fact still serving product requests. The final repair therefore leaves daemon,
Journal and Projection health at `serving_request|available|current` and
publishes the bounded Runtime degradation only through `Partial = true` plus
the exact timeout reason. A real Go UDS -> strict Swift client fixture is the
required regression proof; the Swift decoder remains unchanged and fail-closed.
