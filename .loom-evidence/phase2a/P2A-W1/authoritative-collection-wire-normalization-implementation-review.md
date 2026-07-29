# P2A-W1 Authoritative Collection Wire Normalization Implementation Review

**Date**: 2026-07-29  
**Mode**: fresh, independent, read-only  
**Verdict**: `PASS`  
**Live authority**: none

## Findings

- P0: none;
- P1: none;
- P2: none.

## Independent evidence

The Reviewer inspected the frozen contract, Contract Review, RED/GREEN
records, owned Go/docs files, strict Swift client/model/probe sources, and
source-lock hashes.

It confirmed:

- nil snapshot collections become fresh JSON arrays for top-level `runtimes`,
  `teams`, `runs`, `evidence`, and `attention` plus nested Runtime `model_ids`
  and `observed_capabilities`;
- non-empty values and order are preserved by copying;
- timeline `records`, `board.nodes`, and `attention` remain canonical arrays;
- the real historical `model_ids:null` route crosses Projection,
  `LocalProductReadService`, production `localProductHandler`, real
  `localipc.Server`, and compiled Swift `LocalIPCClient`;
- Swift source hashes exactly match the GREEN/source-lock values and the strict
  decoder is unchanged;
- RED is honest: the newly exposed top-level nil bug failed before repair,
  while the previously repaired nested Runtime route was recorded as a
  passing control rather than fabricated as RED;
- no W4, authority, Journal, Projection, StateWriter, module, staging, or live
  surface change exists inside the reviewed reopen.

The Reviewer independently ran:

```text
go test ./internal/api -run 'TestLocalProduct.*Collection' -count=1
PASS

go test ./cmd/loomd \
  -run TestProductDaemonServesAuthoritativeNilCollectionsToStrictSwiftClient \
  -count=1
PASS outside the managed UDS sandbox
```

The managed sandbox failure was limited to private UDS readiness; the exact
focused fixture passed under approved local escalation without network,
resident service/socket, launchctl, App, Provider, or Runtime access.

`VERDICT: PASS`

This Review closes only the non-live P2A-W1 wire-normalization reopen. It grants
no live canary, retry, P2A-W1 acceptance, or P2A-W2 authority.
