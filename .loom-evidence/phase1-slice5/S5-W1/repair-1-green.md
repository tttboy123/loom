# S5-W1 Implementation Repair 1 GREEN

Date: 2026-07-26
Baseline: `006db8c`
Gate: fresh Implementation Repair 1 Review 2

## Reviewer finding repair

Implementation Review 1's only finding was Candidate provenance. The repair
adds `scope-provenance.md`, which binds review and eventual staging to the
exact frozen S5-W1 ownership set. Pre-existing user-owned shared-worktree
changes remain untouched, excluded, and unstaged.

## Final high-risk lineage canary

Before Repair Review 2, the external app-to-API canary was raised from
low-risk acceptance to a real high-risk path with a distinct verifier Runtime,
executor, Run, generation, Grant, WorkItem, and Evidence lineage. This exposed
two S5-W1 resolver defects:

1. verifier WorkItems do not duplicate Team/logical-attempt fields; their
   authoritative Team relation is the accepted source WorkItem's exact
   verifier references. `relatedWorkItemLineage` now validates that parent
   WorkItem against the projected source attempt and exact verifier
   WorkItem/Run/Agent identities;
2. the related cursor scope included verifier Run, Evidence, and Grant streams
   but omitted `work-item/<verifier_work_item_id>`. The scope now validates and
   includes that stream.

The canary now requires authoritative source Run/Evidence and verifier
WorkItem/Run/Evidence records in the final page, all resolved to `main`,
attempt 1. It also proves verifier tentative output is absent from the source
subscription and all tentative text is absent from Journal.

## Verification

```text
go test ./internal/app \
  -run '^TestTeamExecutionStreamReceivesPostCaptureAuthorizedOutput$' \
  -count=10
PASS

go test ./internal/api ./internal/app ./internal/journal \
  ./internal/projection ./cmd/loom
PASS

go test -race -count=10 ./internal/journal ./internal/projection \
  ./internal/api ./internal/app ./cmd/loom
PASS

go test ./...
PASS

go test -race ./...
PASS

go vet ./...
PASS

GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/journal ./internal/projection ./internal/api ./cmd/loom
PASS

gofmt -d <all frozen S5-W1 Go files>
PASS (empty output)

git diff --check
PASS

go mod verify
PASS
```

No dependency, migration, writer, daemon, network, installed Runtime,
Provider/model traffic, credential, external action, or S5-W2/S5-W3 scope was
added.

VERDICT: PASS
