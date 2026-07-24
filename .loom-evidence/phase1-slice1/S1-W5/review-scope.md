# S1-W5 Review Scope and Provenance

## Product Candidate

The S1-W5 product Candidate is exactly:

- `cmd/loom/main.go`
- `cmd/loom/query.go`
- `cmd/loom/main_test.go`

All other modified or untracked paths predate S1-W5 or are Controller-owned
evidence/status records. Repository-wide impact checks still apply.

## RED provenance

The first Developer session added mandatory `TestRun*` tests and ran:

`go test ./cmd/loom -run 'TestRun' -count=1`

It failed before implementation with missing `run`, `productionDeps`, and
`runDeps` symbols. That session then wrote the initial implementation but was
interrupted before running GREEN.

## Sequential writer handoff

The Controller observed no files at its last check and interrupted the first
Developer as apparently empty. The files were written immediately around that
boundary. A replacement Developer then inherited them, changed the
package-local source-file paths in one test from `cmd/loom/main.go` and
`cmd/loom/query.go` to `main.go` and `query.go`, formatted the owned files, and
ran the complete GREEN suite.

The sessions did not write concurrently and there is no ambiguous overlapping
diff, but two distinct Developer sessions sequentially touched this Candidate.
This is a Controller orchestration deviation from the GoalSpec
`max_candidate_writers_per_lineage: 1` budget and must be considered explicitly
by the Reviewer. It is not a product repair or hidden as a single-session
lineage.

## Controller evidence

After the handoff, the Controller independently ran the frozen focused
`TestRun` suite, focused race at `-count=50`, CLI full/race, repository
full/race, vet, formatting, diff checks, and both conversation and explicit
Agent route smoke commands. All exited 0.
