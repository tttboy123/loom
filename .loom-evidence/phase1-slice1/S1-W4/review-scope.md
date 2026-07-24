# S1-W4 Review Scope and Provenance

## Product Candidate

Only these product files belong to the S1-W4 Developer Candidate:

- `internal/projection/projection.go`
- `internal/projection/projection_test.go`

The Developer's initial and both repair completion reports identify only those
two files. No S1-W4 Developer modified documentation, progress, Evidence
records, prior packages, dependencies, CLI, or migrations.

## Controller-owned evidence and status files

These files are outside the product Candidate and are expected Controller
workflow records:

- `.loom-evidence/phase1-slice1/S1-W4/contract.md`
- `.loom-evidence/phase1-slice1/S1-W4/repair-1-contract.md`
- `.loom-evidence/phase1-slice1/S1-W4/repair-2-contract.md`
- `.loom-evidence/phase1-slice1/S1-W4/review-scope.md`
- `PROGRESS.md`

## Pre-existing excluded change

`docs/CURRENT.md` was updated by the Controller immediately after
S1-W3-L2 received its fresh Reviewer PASS and before S1-W4 was frozen. It
records accepted S1-W3 behavior and identifies S1-W4 as the next checkpoint.
It is not an S1-W4 Developer change and must not be attributed to the S1-W4
Candidate.

Because this checkout began from an implementation-not-started commit and the
Slice work is intentionally uncommitted, repository-wide `git diff` cannot
separate sequential WorkItem provenance. Review must use this frozen ownership
manifest and inspect the two Candidate product files, while still running
repository-wide impact checks.
