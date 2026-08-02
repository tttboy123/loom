# P2A-W3 Replacement Live Failure Diagnosis

**Date**: 2026-08-02  
**Status**: `READ-ONLY DIAGNOSIS / NON-AUTHORITATIVE`  
**Scope**: the failed MiniMax and Pi replacement attempts only

This note does not freeze a contract, authorize another live attempt, create
P2A-W4, or change any execution authority. It records what the retained live
evidence and current source can and cannot prove before a governed P2A-W3 repair
boundary is reviewed.

## MiniMax: confirmed authority/presentation gap

The retained attempt proves that one explicit Test action changed the native UI
from inherited `Verified` through `Testing` to `Unavailable`, while the Journal
retained exactly the three inherited `ProviderCredentialVerified` facts and no
new revision or terminal fact.

The current source explains that exact shape:

1. `LocalProductStore.verifyMiniMax` maps every non-conflict setup error to the
   local UI state `Unavailable`.
2. `CredentialBroker.Verify` reads the secret store before it invokes the
   Provider verifier.
3. A Keychain/helper read failure returns a closed store error immediately.
   `TestCredentialBrokerStoreFailureBeforeObservationCommitsNoFact` explicitly
   requires zero verifier calls and zero metadata commits for that path.
4. By contrast, a Provider timeout/unavailable response is a valid typed
   `VerificationUnavailable` result and is committed as the next
   `ProviderCredentialVerified` fact with rejected/unavailable metadata.

Therefore the replacement result did not complete the normal Provider
unavailable path. It failed before the Provider observation/commit boundary,
with the process-owned Keychain/helper read path the reachable closed boundary.
The retained client error deliberately does not distinguish not-found, denied,
helper failure, or store unavailable, so the exact store leaf cannot be recovered
after the fact.

This is a product defect, not merely missing evidence: a user-visible terminal
verification result can currently exist without a corresponding authoritative
attempt outcome. The repair must make one explicit Test operation produce one
idempotent terminal metadata fact even when the credential store fails before a
Provider call. It must not claim that the Provider rejected the credential and
must not expose the credential, store path, helper output, or raw error.

## Pi: confirmed diagnostic loss, live leaf not recoverable

The replacement attempt copied the accepted six-fact state from the first Pi
attempt, started the daemon once, and received only:

```text
daemon failed: observer_unknown
```

The SQLite remained byte-identical and contains the prior canonical
`RuntimeInstanceDiscovered` fact for
`runtime_instance:pi-0.82.1-p2a-w3-pi`, source probe
`p2a-w3-pi-probe`, Pi `0.82.1`, capacity `1`, and the exact local model. No
product socket or execution authority was reached.

Static tracing shows that `observer_unknown` is a lossy fallback, not a causal
classification. `observerFailureReason` recognizes only projection refresh,
selected application write wrappers, binding drift, uniquely tagged metadata
command failures, and probe-factory failure. Reachable observer errors such as
an invalid installed candidate, probe construction failure, runtime inventory
normalization failure, write-plan failure, daemon identity/time metadata
failure, and unwrapped Journal append failure are collapsed to
`observer_unknown`. The public daemon message then discards the original safe
error tree.

The retained result contains neither a safe leaf code nor a component trace, so
it cannot prove that the copied Runtime state, installed Pi output, metadata
binding, identity source, or write path was the failing leaf. Assigning one of
those causes now would be speculation. The state being byte-identical proves
only that the failure occurred before a successful append.

The repair must close the complete observer taxonomy in one boundary: every
expected fail-closed leaf must map to one stable safe reason code, exactly one
code must survive the full error tree, ambiguous/multiple leaves must remain
closed, and raw paths/output must remain absent. Deterministic component tests
must cover a pre-existing Runtime fact plus the real locked Pi 0.82.1 component
shape before any additional live attempt is considered.

## Required single P2A-W3 repair boundary

A safe continuation is one reviewed P2A-W3 vertical repair, not two amendments:

- authoritative credential verification terminalization for pre-Provider store
  failures, with operation-ID idempotency and no false Provider claim;
- exhaustive safe observer reason-code propagation and deterministic copied-state
  Pi component coverage;
- strict Swift refresh behavior that derives the final credential state from the
  returned authoritative revision/snapshot rather than a local-only terminal;
- focused, full, race, vet, security, strict Swift, and platform checks;
- independent Implementation Review before any newly frozen live allowance;
- at most one fresh isolated MiniMax verification attempt and one fresh isolated
  Pi controlled-execution attempt, followed by one independent Result Review.

Until that boundary is frozen and reviewed, the authoritative state remains:

`HUMAN_REQUIRED / NO WALKTHROUGH / NO COMMIT / P2A-W4 DOES NOT EXIST`.
