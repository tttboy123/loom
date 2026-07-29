# P2A-W1 Replacement Result-Evidence Review 1

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer
**Verdict**: `FAIL`

## Finding

The product, fail-closed metadata result, invocation accounting, and rollback
state were coherent, but the evidence labeled
`5c7e59572c7a79ba1a30bdf9f5f187cea276cbd1ca69a0a7383e150b23417193`
as the canonical stream-head digest.

That value was produced from pipe-delimited query text. The accepted
`GlobalReadView` implementation instead hashes each sorted head as:

```text
stream_id NUL decimal_sequence NUL event_id newline
```

For the retained one-head SQLite state, the implementation-defined digest is:

```text
6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05
```

The evidence therefore did not reproduce the exact canonical digest it
claimed. No product, installed-state, SQLite-byte, Event-count, rollback,
secret, or invocation defect was found.

## Required repair

Correct the evidence label and value from the current immutable SQLite bytes,
preserve the original noncanonical measurement as a disclosed methodology
error, and receive a fresh result-evidence re-review. No live action or third
bootstrap is permitted.
