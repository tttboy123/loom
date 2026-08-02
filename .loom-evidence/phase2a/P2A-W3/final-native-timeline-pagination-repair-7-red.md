# P2A-W3 Final Native Timeline Pagination Repair 7 RED

**Date**: 2026-08-03  
**Gate**: RED reproduced against the pre-Repair implementation

## Store behavior RED

```text
swift test --package-path apps/macos --filter LocalProductStoreTests
```

Result: expected `FAIL`; 22 tests executed with 24 assertion failures, all in
the new Repair 7 regressions. Existing focused tests remained green.

The current Store demonstrably:

- issued one request instead of the required two and published `has_more=true`;
- published explicit gaps and nested Board Team drift;
- did not observe later duplicate delivery or repeated cursor;
- published late success/error from an obsolete Team selection;
- published a late success after task cancellation;
- stopped after one page instead of reaching the eight-page fail-closed bound.

## Cursor grammar RED

```text
swift test --package-path apps/macos --filter LocalIPCClientTests/testTimelineCursor
```

Result: expected compile `FAIL`; the frozen
`LocalIPCClient.validTimelineCursor` symbol does not exist. The errors occur only
at the new canonical long-cursor tests. The tests distinguish a valid cursor
above 256 bytes from normal identifiers and require rejection of invalid
modulus, non-zero trailing bits, padding, whitespace, non-ASCII and values above
32 KiB.

No product implementation was changed before these failures were captured.
