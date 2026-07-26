# S4-W1 Fresh Implementation Review 1

Date: 2026-07-26

Reviewer: fresh independent read-only Reviewer

Baseline: `7bb9881`

The Reviewer edited no files. Its bounded package check passed:

```text
go test ./internal/rules ./internal/work ./internal/projection -count=1
```

The implementation verdict is `FAIL` because:

1. `RequestApproval` read only caller-referenced RuleSet streams, so a caller
   could omit a current project/team/work-package/work-item RuleSet and bypass
   a higher-scope reject.
2. activation, request, and decision exact-retry paths compared too little
   persisted command identity and accepted divergent correlation,
   presentation, or request-time payloads.
3. approved decision replay restored the continuation digest but not the exact
   seven-stream resume Candidate heads.
4. exported CustomerAuthorizer requests exposed no bounded accessors and the
   authorized response values had no safe construction path for an
   implementation outside package `rules`.

These are contract defects, not evidence-only gaps. They remain inside the
frozen S4-W1 owned boundary and require bounded Repair 1.

VERDICT: FAIL
