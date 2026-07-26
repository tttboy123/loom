# S3-W4 Implementation Review 1

- Date: `2026-07-26`
- Reviewer: `/root/s2_w4_contract_review`
- Mode: fresh independent read-only

## Findings

1. `P1` — Raw Grant could leak through source/workspace path text because only
   file content and stderr were scanned. Required repair: reject token
   substrings in source manifest paths and final change paths, with source and
   child-created filename RED.
2. `P2` — Required fuzz verification was not reproducible on Darwin because
   the fuzz fixture treated platform filename creation rejection as a product
   failure. Required repair: return on fixture setup rejection before invoking
   product code.

## Independent verification

Passed:

- focused Supervisor/Pi tests;
- focused Supervisor/Pi race;
- repository tests;
- vet;
- gofmt diff;
- S3-W4 scope diff check;
- marker/static-boundary manual inspection.

The Reviewer did not rerun the exact 30-run race due time and removed only the
Go-generated fuzz corpus artifact under `internal/supervisor/testdata`.

VERDICT: FAIL
