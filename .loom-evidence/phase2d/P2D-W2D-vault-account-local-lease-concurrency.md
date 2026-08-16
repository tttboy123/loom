# P2D-W2D Vault Account-local Lease Concurrency

Status: `CURRENT / SOURCE VERIFIED / BUILD 47 PACKAGED / NOT INSTALLED`

Date: 2026-08-12

Parent Goal: `Phase 2D - Multi-Provider, Multi-Model Agent Team Orchestration`

## Finding

The Credential Vault and `CredentialLeaseManager` already supported exact
Provider Account identities and concurrent leases, but
`productCredentialVaultRuntime.UseCredential` retained its global runtime mutex
through the complete Provider callback. A slow call for one Provider Account
therefore serialized otherwise independent account calls and delayed Vault
governance operations behind remote network latency.

That behavior contradicted Phase 2D account-local dispatch and failure-isolation
semantics even though it did not weaken credential binding.

## Repair

`UseCredential` now validates the Vault state and snapshots the current lease
access while holding the runtime mutex, then releases that mutex before issuing
and using the exact short-lived credential lease. The lease manager remains the
owner of callback cancellation, expiry, revoke, rotation barriers, and
plaintext zeroization.

The repair does not cache a credential, convert it to a long-lived string,
weaken reference/revision/account validation, or introduce a Keychain,
plaintext, argv, environment, Journal, Evidence, or diagnostics path.

## RED/GREEN evidence

The RED test held a DeepSeek account callback open and required an Anthropic
account callback to enter before release. The previous runtime mutex serialized
the second callback and failed after 250 ms. With the scoped lock boundary, the
callbacks overlap. Focused normal and race tests also retain the existing Vault
lock and rotation behavior: active leases are cancelled and zeroized, and
pre-rotation leases cannot be reused.

A post-freeze test-only reinforcement exercises the complete runtime boundary:
`UseCredential` enters an active lease callback, `Lock now` closes the manager,
the callback receives cancellation, lock completes without deadlock, and Vault
status becomes `locked`. Its focused normal and race runs pass. It does not
change build-47 product bytes or artifact hashes.

## Remaining boundary

This is CV3 source evidence. It does not prove installed multi-account Provider
traffic, account-level rate/cost accounting, or the CV6 no-helper live matrix.
Those gates remain open under the single Phase 2D Goal.

The source is frozen in the unlaunched, uninstalled v0.5.2 build 47 Candidate.
Complete source and package checks pass; the exact artifact hashes and boundary
are in
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-deliverables/phase2d-v0.5.2-build47-candidate-2026-08-12/BUILD-MANIFEST.md`.
