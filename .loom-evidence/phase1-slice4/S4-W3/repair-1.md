# S4-W3 Repair 1

Date: 2026-07-26
Baseline: `6d3cbf2`
Repairs: Implementation Review 1

The bounded repair changes only the frozen S4-W3 Team semantic-binding schema:

- the Journal writer now serializes the acceptance risk field as exact `risk`;
- Team Projection now replays exact `risk`;
- the high-risk verifier integration test asserts that
  `TeamExecutionPlanned` contains `"risk":"high"` and rejects the former
  `acceptance_risk` spelling.

The repair does not change the internal `AcceptanceRisk` field name, expand
ownership, add an Event, change authority, activate an external surface, or
create another WorkItem.

Post-repair verification:

```text
go test ./internal/verification ./internal/rules ./internal/work ./internal/app ./internal/projection -count=1
PASS

go test -race ./internal/verification ./internal/rules ./internal/work ./internal/app ./internal/projection -count=10
PASS

go test ./internal/evidence ./internal/authorization ./internal/supervisor ./internal/runtime/piadapter ./internal/teams -count=1
PASS

go test ./... -count=1
PASS

go test -race ./... -count=1
PASS

go vet ./...
PASS

gofmt -d <all S4-W3 owned Go files>
PASS (empty output)

git diff --check
PASS

GOOS=windows GOARCH=amd64 go build ./internal/verification ./internal/rules ./internal/work ./internal/app ./internal/projection
PASS
```

VERDICT: PASS
