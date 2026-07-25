# S3-W3 Mandatory RED

- Baseline: `5517a06`
- Date: `2026-07-26`
- Product files changed before RED: none
- Test files added:
  - `internal/authorization/authority_test.go`
  - `internal/projection/grant_authority_test.go`

All six frozen markers occur exactly once.

Command:

```text
go test ./internal/authorization ./internal/projection -count=1
```

Result: exit `1`, as required.

The new `internal/authorization` package had no non-test Go file and compilation
failed only on missing frozen S3-W3 symbols, beginning with:

```text
undefined: Authority
undefined: NewAuthority
undefined: IssueInput
undefined: Token
undefined: Operation
undefined: AuthorizeInput
```

The projection package also failed because it imports the still-missing
authorization product boundary. No existing package behavior failed, no
product file changed, and no unrelated error was used as RED evidence.

VERDICT: PASS
