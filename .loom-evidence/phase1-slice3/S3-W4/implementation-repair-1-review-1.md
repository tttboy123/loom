# S3-W4 Implementation Repair 1 Review 1

- Date: `2026-07-26`
- Reviewer: `/root/s2_w4_contract_review`
- Mode: fresh independent read-only

## Finding

`P1` — cancellation/process-group readiness was not stable under repeated race.

The Reviewer reproduced:

```text
grandchild PID readiness timed out
grandchild PID = "": strconv.Atoi: parsing "": invalid syntax
```

The reader treated any successful `ReadFile(grandchild.pid)` as readiness,
while helper `os.WriteFile` can expose the created/truncated file before PID
bytes are written. Required repair: publish readiness atomically or wait until
a valid positive PID is parsed, then rerun repeated race proof.

## Passed independently

- raw Grant source/change path focused tests;
- 5s fuzz;
- focused package ordinary/race;
- repository tests;
- vet;
- format/diff;
- Amendment 3 impacted subtest `-race -count=100`;
- exact Amendment 3 diff and unchanged 100ms timeout.

No fuzz artifacts remained.

VERDICT: FAIL
