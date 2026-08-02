# P2A-W3 Final Native Timeline Pagination Repair 7 Contract Repair 2

**Date**: 2026-08-03  
**Status**: FROZEN — repaired Contract Review pending  
**Parent Repair 1**: `final-native-timeline-pagination-repair-7-contract-repair-1.md`  
**Review**: `final-native-timeline-pagination-repair-7-contract-repair-1-review.md`

This repair resolves the sole remaining canonical-cursor ambiguity. Every other
Repair 7 and Repair 1 requirement remains frozen.

For a non-empty continuation cursor, the Swift validator must:

1. enforce the 1...32,768 byte ASCII unpadded base64url alphabet already frozen;
2. temporarily decode RawURL base64 and immediately re-encode it as canonical
   unpadded RawURL base64;
3. require the re-encoded UTF-8 bytes to equal the original cursor byte-for-byte.

This transport-only canonicality check is equivalent to Go v1
`RawURLEncoding.DecodeString` plus `RawURLEncoding.EncodeToString(data) ==
encoded`. It does not parse the decoded cursor JSON, infer stream heads,
normalize or replace the transmitted string, persist it, log it, or treat it as
authority. The exact original string is sent after validation.

Mandatory RED additionally covers:

- length modulo four equal to one;
- a RawURL string with non-zero unused trailing bits whose decode/re-encode
  differs;
- a valid authoritative cursor above 256 bytes whose exact original bytes reach
  the Go server unchanged.

No owned file or authority boundary changes from Repair 1.
