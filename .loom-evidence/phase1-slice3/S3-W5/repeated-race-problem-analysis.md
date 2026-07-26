# S3-W5 Repeated-Race Problem Analysis

Date: 2026-07-26

Analyst: independent read-only Problem Analyst

## Trigger

The frozen combined command:

```text
go test -race ./internal/supervisor ./internal/runtime/piadapter \
  ./internal/teams ./internal/work ./internal/authorization \
  ./internal/projection ./internal/app -count=10
```

passed every package except `internal/runtime/piadapter`. The failures were:

- five-second grandchild PID readiness expired without a readable PID file;
- three-second valid local metadata probe timed out; and
- three-second Supervisor integration Runtime profile timed out.

## Classification

Verification-harness and host-scheduling budget exhaustion, not a demonstrated
Pi product defect. S3-W5 acceptance remains open until the unchanged combined
command passes.

## Evidence

- There was no Go data-race report, Bridge mismatch, observer failure, cleanup
  leak, or stable product failure.
- PID publication uses write/close plus atomic rename. `bytes="" parse=<nil>`
  means no PID file became readable before the readiness deadline, not partial
  publication.
- The Supervisor path failed closed with `ErrRuntimeTimeout` and context
  deadline exceeded.
- S3-W5 synchronously validates and authorizes every inbound Frame, including
  the Journal authorization transition, inside the inherited integration
  timeout.
- The affected families passed isolated race repetition. The complete Pi
  Adapter package also passed isolated `-race -count=10`.
- A read-only reproduction observed a Supervisor success case crossing the
  three-second fixture timeout at approximately 3.56 seconds without a race or
  protocol failure.
- The eight-logical-CPU host was oversubscribed during inspection, with another
  repeated race test process active.

## Bounded recommendation

Freeze a test-only amendment reopening only the two Pi Adapter test files.
Increase only the three non-timeout fixture budgets to ten seconds, preserve
all dedicated timeout/cancellation/product assertions, and rerun the exact
combined command unchanged.

VERDICT: TEST-HARNESS STABILITY REPAIR REQUIRED
