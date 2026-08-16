# P2D-COMP2-B Declarative Route Registry V1

**Date**: 2026-08-14  
**Goal**: Phase 2D only  
**Status**: SOURCE VERIFIED / COMP2-C-D OPEN  
**Contract**: `contracts/P2D-COMP2-product-daemon-strangler.md`  
**Decision**: `../../docs/adr/0021-governed-composition-kernel-and-daemon-strangler.md`

## Accepted boundary

The product composition snapshot now contains 47 exact method-level
RouteDescriptors instead of `legacy.product.dispatch`. Every route freezes its
schema, built-in Bundle owner, required and handler capability, unavailable
error, Incident policy, and privacy class. Desktop, headless, and test Profiles
require and digest the same sorted route set.

Product startup now constructs `productRouteServices` and calls
`newProductRouteHandler` directly. `localProductHandlerWithComposition` remains
only as a positional compatibility wrapper for existing tests. Availability is
resolved by the typed `productRouteRegistry`; the existing method dispatch
switch remains the behavior oracle for COMP2-C extraction.

## Preserved behavior

- all 47 existing local IPC methods remain registered exactly once;
- setup, Vault, Conversation, mission, assets, Queue, workers, integration,
  permission, execution, production, rule, and standing-order unavailable
  responses preserve their existing public codes;
- Journey IDs, Conversation failure stage, credential failure stage, and
  degraded production-write behavior remain unchanged;
- unknown methods remain outside the registry and reach the existing fail-closed
  decoder path;
- descriptors and snapshots contain no request body, Prompt, transcript,
  Provider response, credential, Authorization header, VMK, or service object;
- protected Core capability ownership and all existing Vault, Journal, policy,
  terminal authority, IPC attestation, and Attempt binding boundaries remain
  unchanged.

## Verification

Passed on 2026-08-14:

```text
go test ./cmd/loomd -run '^TestCOMP2[AB]' -count=1
go test -race ./cmd/loomd -run '^TestCOMP2[AB]' -count=20
go test ./cmd/loomd
go test ./...
go vet ./cmd/loomd
go vet ./...
git diff --check
```

The COMP2-B tests prove the exact 47-method manifest, per-Profile snapshot,
absence of the aggregate legacy route, descriptor/runtime unavailable-code
parity, unknown-method exclusion, and AST-level prohibition on product startup
calling the positional compatibility handler.

## Open boundary

This does not complete P2D-COMP2 or Phase 2D. COMP2-C must move service
construction into bounded built-in Bundles and COMP2-D must attach Product,
Conversation, Team, Agent, Attempt, and Turn scope ownership to existing
lifecycle boundaries. The monolithic dispatch switch remains temporarily;
COMP2-E cannot remove it until parity, race, restart, privacy, shutdown,
installed CV6, and mixed-Team ATL9 gates pass.

No App was built, signed, installed, or launched. No credential, Provider,
network, user workspace, or external tool was accessed.
