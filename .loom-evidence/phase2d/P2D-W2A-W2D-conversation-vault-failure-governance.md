# P2D-W2A/W2D Conversation Vault Failure Governance

**Date**: 2026-08-12  
**Status**: `CURRENT / SOURCE ACCEPTED`; installed CV6 remains open

## User impact

Ordinary Conversation dispatch now uses the same Credential Vault retry policy
as Agent Attempts. A temporary lease issue remains retryable. Durable failures
such as `vault_decrypt` and `vault_aad_validation` are not presented as retry
loops; the Conversation failure banner instead offers an explicit action to
open Runtime & Providers at the Credential Vault recovery surface.

The action is governed by a closed Vault-stage allowlist. Provider auth, rate
limit, timeout, profile conflict, and retryable lease failures cannot receive a
Vault recovery action. Diagnostics and Incident ID copy remain separate.

## Frozen path

`CredentialLeaseManager` staged error -> Conversation Profile Router -> safe
dispatch failure detail -> IPC `recoverable` -> failed Conversation Attempt ->
Swift chat operation state -> stage-specific recovery action.

The Router computes the stage once and delegates retryability to
`credentials.CredentialFailureRetryable`. The chat API freezes that exact value
on the Attempt. No credential, Prompt, Provider response, or private lower-level
error is added to the wire, transcript, Journal, Evidence, or diagnostics.

## Verification

- Router tests prove `credential_lease_issue` is recoverable and
  `vault_decrypt` is not, including the IPC projection.
- Chat API tests prove `vault_decrypt / credential_unavailable / false` is
  frozen on the failed Attempt with the same Incident ID.
- Swift tests prove only non-recoverable Vault stages expose the recovery
  action.
- Focused normal and race tests pass five consecutive runs.
- Complete Swift passes 200 XCTest cases with one intentional visual-export
  skip plus nine Swift Testing contracts, all with zero failures.
- Full Go and `go vet ./...` pass.

This is source and static Candidate evidence. Installed Loom remains build 39;
no real credential, installed App, or Provider was touched. Installed DeepSeek
conversation recovery and the full CV6 matrix remain open under the sole
`ACTIVE / PARTIAL` Phase 2D Goal.
