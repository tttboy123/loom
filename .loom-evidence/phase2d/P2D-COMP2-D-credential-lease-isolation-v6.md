# P2D-COMP2-D Credential Lease Isolation V6

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-D PARTIAL  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The existing Loom Credential Vault lease hot path is now verified end to end
against the real COMP2-D Agent Attempt scopes. No second lease API was added:

```text
Agent Attempt execution context
  -> exact FrozenExecutionBinding
  -> exact Provider Account / reference / revision projection check
  -> CredentialLeaseManager.Acquire(parent = Attempt context)
  -> bounded plaintext callback
  -> close/revoke/parent cancellation zeroizes plaintext
```

The cross-component acceptance creates two Agents in one Team with independent
DeepSeek and MiniMax bindings and independent credential identities. Revoking
the DeepSeek identity terminates only the DeepSeek lease. The MiniMax callback
remains active and its plaintext remains available to that callback until the
Team scope closes. Team close then revokes the MiniMax Attempt context and lease.
Both plaintext buffers are zeroized in place after their respective terminal
boundary.

The credential identity remains the exact tuple of Provider ID, Provider Account
ID, credential reference, and revision frozen in each Agent binding. Resolution
never falls back to Provider ID alone. A credential revoke does not cancel the
peer Agent, its Attempt, or the whole Team. Scope cancellation and account
revocation retain distinct causes.

No secret enters Capability Context, Go context values, Context Capsule,
Journal, Evidence, diagnostics, process arguments, or environment. The Go
context carries cancellation only; plaintext exists solely in the bounded Vault
lease callback.

## Verification

The existing production behavior was strong enough to satisfy the new
cross-component acceptance without a second implementation path. Verification:

```text
go test ./cmd/loomd -run '^TestCOMP2DCredentialRevokeIsolatesAgentAndScopeClosesPeerLease$' -count=1
go test ./cmd/loomd ./internal/credentials/vault -run '^(TestCOMP2D|TestProduct.*Credential|TestCredentialLease)' -count=1
go test -race ./cmd/loomd ./internal/credentials/vault -run '^(TestCOMP2DCredential|TestCredentialLease)' -count=10
go test ./cmd/loomd -count=1
go test ./... -count=1
go vet ./...
git diff --check
```

Tests prove exact per-Agent resolution, account-local revoke, peer survival,
Team-scope lease cancellation, distinct cancellation causes, in-place secret
zeroization, race safety, and repository parity.

## Remaining boundary

COMP2-D remains open. Provider native-session handle leases, tool extension and
channel ownership, component processes, temporary roots, broader Runtime
adapters, and restart/crash cleanup still need exact Scope Effects or equivalent
verified ownership. COMP2-E, installed CV6, mixed-Team ATL9, UI/accounting, and
live Provider acceptance remain mandatory.

No App was built, signed, installed, or launched. No real credential, Provider,
network, user workspace, or external tool was accessed.
