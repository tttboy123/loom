# P2A-W1 Local Product Vertical Live Closure Mandatory RED

**Date**: 2026-07-29  
**Result**: `PASS - RED CONFIRMED`  
**Live action**: none

## Authoritative Application/API RED

Tests first appended valid `RuntimeInstanceDiscovered` Events through the real
Journal and accepted Projection path for:

- the historical `model_ids:null` plus non-empty capabilities shape;
- non-empty model IDs plus `observed_capabilities:null`;
- non-empty ordered values for preservation proof.

The unchanged product failed the two exact nil collection cases:

```text
go test ./internal/api \
  -run TestLocalProductReadServiceCanonicalizesRuntimeCollectionsForWire \
  -count=1

historical_null_model_ids:
ModelIDs: []string(nil)

nil_observed_capabilities:
ObservedCapabilities: []string(nil)

FAIL
```

The non-empty order case passed, proving the RED is the nil-to-empty wire
boundary rather than general Runtime projection failure.

## Real daemon IPC RED

The real product daemon SQLite fixture was changed to the historical
`"model_ids":null` Event shape. The existing daemon, local IPC server, and Go
client then failed the new non-nil collection assertion:

```text
go test ./cmd/loomd \
  -run TestProductDaemonServesRealReadOnlySQLiteOverPrivateUDSAndCleansUp \
  -count=1

ModelIDs: []string(nil)
FAIL
```

## Strict Swift control

The real Go `localipc.Server` fixture now emits the required empty arrays, and
the strict Swift probe accepts them. Additional malformed loopback cases prove
that null model IDs, null capabilities, missing model IDs, wrong type,
duplicate model IDs, and an unknown Runtime field still fail as
`invalid_response`:

```text
go test ./internal/localipc \
  -run 'TestSwiftClient(InteroperatesWithRealGoServer|RejectsMalformedLoopbackResponses)' \
  -count=1

PASS
```

This control is expected to pass before the Go Application/API repair. It
proves the client needs canonical arrays and was not weakened.

No product source, Journal, Projection, StateWriter, module lock, installed
file, resident service, App, Provider, Runtime, credential, or live state
changed. Git staging remained empty.
