# P2A-W1 Replacement Result-Evidence Review 2

**Date**: 2026-07-28
**Reviewer**: independent read-only Reviewer
**Verdict**: `PASS`
**Blocking findings**: none

## Findings

1. Evidence Repair 1 correctly identifies
   `5c7e59572c7a79ba1a30bdf9f5f187cea276cbd1ca69a0a7383e150b23417193`
   as the prior noncanonical pipe-delimited row hash.
2. Both gate files now use the implementation-defined canonical digest:
   `6f43ee12d6593a9726e01e730cd8dd9dd509343cae86fb5af5cf266d321f9e05`.
3. The Reviewer independently reproduced that digest from the current
   immutable one-Event SQLite head using the exact sorted NUL-delimited
   `GlobalReadView` format.
4. Installed rollback state remains exact: original binary/wrapper/plist/
   SQLite hashes, absent launcher/product run directory/default and historical
   sockets, clean disk plist, running restored observer, and empty staged diff.
5. Evidence Repair 1 performed no TUI/Computer Use, service, product,
   credential, Provider, Runtime, SQLite, staging, or live action.
6. No third live allowance exists. The result remains
   `FAIL — ROLLED_BACK — HUMAN_REQUIRED`.

This `PASS` validates the fail-closed result evidence and exact rollback. It
does not constitute P2A-W1 live product delivery or acceptance.
