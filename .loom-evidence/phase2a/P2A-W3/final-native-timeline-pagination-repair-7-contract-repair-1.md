# P2A-W3 Final Native Timeline Pagination Repair 7 Contract Repair 1

**Date**: 2026-08-03  
**Status**: FROZEN — repaired Contract Review pending  
**Parent Repair 7**: `final-native-timeline-pagination-repair-7-contract.md`  
**Review**: `final-native-timeline-pagination-repair-7-contract-review-1.md`

This repair changes only the three ambiguities found by Contract Review 1. All
other Repair 7 requirements remain frozen.

## 1. Opaque authoritative cursor compatibility

`timeline_page.cursor` is not a Loom business identifier. The Swift client must
validate it with a dedicated closed grammar that matches the accepted Go v1
wire boundary:

- empty is permitted only for the first request;
- a continuation is 1...32,768 UTF-8 bytes;
- ASCII canonical unpadded base64url only: `A-Z`, `a-z`, `0-9`, `-`, `_`;
- `=` padding, whitespace, non-ASCII and every other character reject locally.

The client does not decode, reinterpret, truncate, normalize or persist the
cursor. It sends the exact accepted bytes returned by the prior page. Ordinary
Team/source/definition/request identifiers retain the existing 256-byte
`validIdentifier` boundary.

Mandatory Swift RED must prove a canonical cursor above 256 bytes is accepted,
while empty first request remains accepted and padded, over-32-KiB, whitespace,
non-ASCII and non-base64url continuations reject before transport.

## 2. Nested page identity

Before any page contributes records, the Store must require:

- page schema version 1, exact requested Team and non-empty view version;
- Board schema version 1, Board Team equal to the requested Team, and Board view
  equal to the page view;
- every Attention item has schema version 1, exact requested Team and a
  non-empty unique `attention_id` within that page;
- every Timeline record has schema version 1, exact requested Team and a
  non-empty unique `delivery_id` across the complete load.

The first valid page establishes view, Board and Attention. Later pages must
also pass their own nested checks and equal the first page values exactly.
First-page nested mismatch and later-page drift are separate mandatory REDs.

## 3. Authoritative multi-page compatibility proof

The vertical test must use this exact path:

```text
SQLite Journal fixture
  -> LocalProductReadService / TeamExecutionStream
  -> localProductHandler
  -> localipc.Server
  -> LocalIPCClient
  -> LocalProductStore bounded aggregation
```

The fixture must make the first 64-record page incomplete, obtain its cursor
from the authoritative service, prove that cursor is greater than 256 bytes,
and prove the second request contains that cursor byte-for-byte. A static Go
fixture handler or invented short cursor cannot satisfy this proof. The Store
must finish with `gap=nil`, `has_more=false`, ordered unique records and no
second authority mutation.

The separate Swift Store fixture RED/GREEN continues to prove Source Evidence,
independent Verifier Evidence and canonical terminal aggregation. The fresh
103-Event replacement walkthrough remains the real product proof of those
records through the accepted live database copy.

## Exact additional owned files

Repair 7 additionally reopens only:

- `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
- `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`
- `cmd/loomd/product_daemon_test.go`

These paths were already owned by the parent P2A-W3. No production Go handler,
API, authority, Journal, Projection or protocol file is reopened. All excluded
user files and non-W3 evidence remain untouched.
