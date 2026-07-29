# P2A-W1 Local Product Launch Failure Closure Repair Contract Review

**Date**: 2026-07-29  
**Reviewer**: fresh independent read-only Reviewer  
**Verdict**: `FAIL`  
**Live action**: none

## Findings

### P1 - repaired Candidate identity was not bound

The first frozen contract allowed changes to daemon source and the transaction,
then described a replacement live canary, but owned no fresh repaired Candidate
manifest. It therefore left an unreviewed choice between the retained failed
Candidate, which cannot contain the repair, and an unbound rebuilt Candidate.

Repair: the same contract now owns exact source-lock and repaired-Candidate
manifest paths. It requires fresh binaries, native executable, nonzero arm64
UUID, canonical signed-bundle manifest, source inputs, toolchains,
transaction, and fixture to be bound and independently reproduced before
activation. The retained failed Candidate is limited to the pre-implementation
exact-path diagnostic.

### P1 - failure reason record had no exact owned path

The first frozen contract required a private closed reason record but did not
name it in the create allowlist.

Repair: the same contract now owns exactly
`local-product-launch-failure-reason.json`, defines its complete closed schema,
atomic creation, mode, owner, symlink refusal, and forbidden fields.

## Passed boundaries

The Reviewer confirmed that the contract otherwise:

- remains one vertical repair inside existing P2A-W1;
- preserves the consumed historical allowance and no-retry result;
- leaves `internal/localipc` read-only unless a causal RED proves otherwise;
- defines non-disclosing reason codes;
- defers any new live allowance until after fresh Implementation Review and a
  separate activation audit;
- keeps P2A-W2 locked.

No product source, transaction, service, App, Journal, Runtime, Provider, or
credential was changed by this failed Review.
