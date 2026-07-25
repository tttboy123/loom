# S2-W18 Contract Amendment 2

- Lineage: `S2-W18`
- Trigger: pre-review strict-matrix architecture failure
- Product repair count: `0/3`
- Head: `b73cf8b`

## Evidence

The S2-W18 focused tests passed, but package/repository/repository-race checks
failed on the accepted S2-W1 import boundary:

```text
TestRuntimeImportBoundaryStaysPureDomain
pi_process_other.go imports forbidden boundary package "os/exec"
```

`internal/runtime/catalog_test.go` freezes `internal/runtime` as a pure domain/
port package and rejects concrete `os/exec`. `TECH-PLAN.md §13.1` likewise says
domain modules do not import concrete Agent CLIs.

## Amendment

Move the concrete Pi process adapter into the new child package
`internal/runtime/piadapter`, which imports the accepted parent Runtime ports.
The parent `internal/runtime` package and all accepted files remain unchanged.

Owned product/test paths become:

- `internal/runtime/piadapter/process_runner.go`
- `internal/runtime/piadapter/process_runner_test.go`
- `internal/runtime/piadapter/process_unix.go`
- `internal/runtime/piadapter/process_other.go`

No behavior, request, isolation, trust, cleanup, or activation boundary is
expanded. The focused test command and package matrix are updated for the child
package. Moving/writing these paths requires a fresh independent amended-
contract PASS.

VERDICT: PASS
