# S5-W2 Verification Matrix

Candidate baseline: `93bdb1f`

## Focused product packages

```sh
go test ./internal/work ./internal/app -count=1
```

```text
ok  	loom-pi-rebuild/internal/work	0.687s
ok  	loom-pi-rebuild/internal/app	5.954s
```

## Adjacent accepted authorities

```sh
go test ./internal/api ./internal/authorization ./internal/evidence \
  ./internal/journal ./internal/projection ./internal/rules \
  ./internal/runtime ./internal/supervisor ./internal/teams \
  ./internal/verification -count=1
```

All ten packages passed. Observed package times ranged from `0.745s` to
`5.373s`.

## Repeated race gate

```sh
go test -race -count=10 ./internal/work ./internal/app ./internal/api
```

```text
ok  	loom-pi-rebuild/internal/work	41.350s
ok  	loom-pi-rebuild/internal/app	172.574s
ok  	loom-pi-rebuild/internal/api	11.307s
```

## Whole repository

```sh
go test ./...
go test -race ./...
```

Both commands passed every package. The new `internal/app` suite completed in
`9.808s` without race and `23.273s` with race.

## Static, portability, and module gates

```sh
go vet ./...
gofmt -d internal/work/work_package.go \
  internal/work/work_package_test.go \
  internal/app/phase1_engineering_demo_test.go
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true ./internal/work
go mod verify
```

All commands exited zero. `gofmt -d`, `git diff --check`, and `go vet` emitted
no findings. Windows compilation passed, and `go mod verify` reported
`all modules verified`.

## Scope and safety audit

- Candidate product/test files exactly match the frozen ownership.
- No existing authority, migration, dependency, CLI, daemon, Runtime adapter,
  or accepted test file changed.
- `AGENTS.md`, `PROGRESS.md`, `.codex/**`, `.loom-drafts/**`, and the queued
  Slice 3 product-input file remain excluded and unstaged.
- No network/process execution, installed Runtime, Provider/model traffic,
  credential, user path, external effect, or autonomous activation occurs.
- The manifest is exact, non-executing, under 8 KiB, and contains no absolute
  user path or secret.

VERDICT: PASS

## Post-Repair 1 rerun

The complete matrix was rerun after Implementation Repair 1.

```text
focused WorkPackage/demo: PASS
internal/work: 0.851s
internal/app: 2.416s

focused full packages:
internal/work: 1.623s
internal/app: 8.695s

adjacent accepted authorities: PASS
whole repository: PASS (internal/app 10.119s)

repeated race:
internal/work: 63.902s
internal/app: 204.858s
internal/api: 19.141s

whole-repository race: PASS (internal/app 18.620s)
go vet: PASS
gofmt -d: clean
git diff --check: clean
Windows internal/work compile: PASS (0.003s)
go mod verify: all modules verified
```

This post-repair result supersedes the earlier pre-review timings while
retaining the same `PASS` verdict.

## Post-Repair 2 rerun

The entire matrix was run again after the Grant lifecycle assertions.

```text
focused full packages:
internal/work: 2.535s
internal/app: 14.052s

whole repository: PASS (internal/app 5.851s)
adjacent accepted authorities: PASS

repeated race:
internal/work: 72.149s
internal/app: 213.838s
internal/api: 25.500s

whole-repository race: PASS (internal/app 20.701s)
go vet: PASS
gofmt -d: clean
git diff --check: clean
Windows internal/work compile: PASS (0.003s)
go mod verify: all modules verified
```

This is the final pre-Review 3 verification record.
