# P2A-W1 Deterministic GREEN Evidence

**Date**: 2026-07-28
**Baseline**: `3a2c3da5f1682e1acabfebefb6c55e5a57183a20`
**Candidate status**: deterministic GREEN and Implementation Review 3 `PASS`;
resident live delivery `HUMAN_REQUIRED` after two fail-closed rolled-back gates

## Behavior proved

- `GlobalReadView` publishes bounded, stable-ID, deep-copied pages for saved
  Teams, Team executions, Runtimes, Runs, and Evidence.
- A terminal, headed, non-empty execution-only Team has an explicit
  `historical_execution_only`, non-executable, read-only timeline anchor.
  Saved Teams retain their normal confirmed/executable anchor. Invalid,
  nonterminal, empty, missing-head, and mismatched anchors fail closed.
- `LocalProductReadService` returns typed bounded snapshots, actionable
  Attention derived from the current immutable view, and existing
  authoritative Team timeline DTOs. A failed Projection refresh rebuilds the
  exact requested cursor page from the last immutable `GlobalReadView`, marks
  only safe `projection_refresh_failed` metadata, and cannot return another
  client's cached page.
- Private IPC v1 uses length-prefixed bounded frames, strict duplicate/unknown
  JSON rejection, closed error codes, same-effective-UID peer checks, a private
  owned socket directory, 16-connection/handler bounds, cancelable client I/O,
  handler deadlines, atomic Accept registration, and exact device/inode
  cleanup. A request is dispatched only after exactly one frame followed by
  EOF; a half-open peer fails closed. Trusted in-process handlers must observe
  their bounded context and run in the tracked connection lifecycle, so
  `Serve`/`Close` cannot leave detached handler goroutines.
- A stale socket can be reclaimed only after the exclusive lock, exact
  owner/mode checks, and a closed `ECONNREFUSED`/`ENOENT` classification.
  Replacing either lock or live socket preserves the replacement and returns
  an explicit cleanup error.
- `loom` with no arguments and `loom app` route to the TUI. Ordinary
  `status`/`timeline` use the daemon API; `--state` is explicit,
  mutually-exclusive offline recovery.
- The Bubble Tea model covers Home, Runtimes, Teams, Runs/History, Evidence,
  Compare, Attention, and Team Timeline. Two returned Runs can be selected for
  exact summary/Evidence comparison. Partial, stale, offline, fatal protocol,
  gap, board, and Attention states are explicit; `q` cancels in-flight IPC.
  Navigation is pure/read-only, undersized viewports are bounded, and ANSI,
  bidi, control, invalid-UTF-8, newline, and unbounded-cell input is sanitized.
- `loomd --socket` composes the existing observer with the read-only product
  service. The server becomes ready before observer execution; sibling
  cancellation, read-only database close, observer close, socket cleanup, and
  observer-only compatibility are explicit.
- A non-empty migrated SQLite fixture containing Team, Runtime, Run, and
  Evidence facts is read through the real private UDS by the typed client and
  TUI. The real compiled `loom` binary then reads status and timeline from that
  exact daemon, state file, view version, Team, Runtime, Run, and Evidence.
  Team timeline cursor reconnect, a second daemon lifecycle, and the same
  canonical snapshot are proved. Exact Journal Event count and a canonical
  stream-head digest remain unchanged. The separate synthetic transport test
  preserves the stream-gap exit contract.
- The user-level non-networked installer validates private owned inputs and
  destinations, rejects symlink targets, installs exact bytes at mode `0700`,
  creates a credential-free Finder launcher, performs a no-write dry run,
  launches the installed real Bubble Tea binary under a controlled PTY,
  transactionally restores all controlled files after an injected
  intermediate failure, and exactly rolls back both binaries plus launcher.

## Verification commands

The following completed successfully from the repository root:

```text
gofmt on every owned Go file
go test ./internal/projection ./internal/api ./internal/localipc ./internal/tui ./cmd/loom ./cmd/loomd
go test -race ./internal/api ./internal/localipc ./internal/tui ./cmd/loomd
go test ./...
go test -race ./...
go test -count=1 ./...
go test -race -count=1 ./...
go test -count=10 ./internal/localipc ./cmd/loomd
go test -race -count=3 ./internal/localipc ./cmd/loomd
go vet ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn go mod tidy
go mod verify
GOOS=windows GOARCH=amd64 go test -c -o /tmp/loom-p2a-w1-tui.test.exe ./internal/tui
GOOS=windows GOARCH=amd64 go test -c -o /tmp/loom-p2a-w1-cli.test.exe ./cmd/loom
go test -cover ./internal/localipc ./internal/tui
scripts/test-install-loom-local-product.sh
git diff --check
```

The frozen matrix writes `GOOS=windows ... go test`, but a macOS host cannot
execute the resulting PE binary. Per the Contract Reviewer advisory, the two
`go test -c` commands are the compile-only portability proof. Both passed.
The two wholly new product packages report statement coverage of `80.9%` for
`internal/localipc` and `86.8%` for `internal/tui`; the daemon package reports
`73.7%` across new product and accepted legacy observer code.

## Dependency and license

- Direct dependency:
  `github.com/charmbracelet/bubbletea v1.3.4`
- Locked module checksum:
  `h1:kCg7B+jSCFPLYRA52SDZjr51kG/fMUEoPoZrkaDHyoI=`
- Locked `go.mod` checksum:
  `h1:dtcUCyCGEX3g9tosuYiut3MXgY/Jsv9nKVdibKKRRXo=`
- Locally materialized upstream license: MIT
- License file SHA-256:
  `9ca49aaf6748976a8f0a9d7448f34a5f5edf1f363385f1a789e09e328c3d3f84`
- `go mod verify`: `all modules verified`
- Loom's `go 1.22` directive is unchanged.

One optional `go list -m all` attempt against the default
`proxy.golang.org` timed out in the controlled network. Repeating with the
already reviewed `goproxy.cn` transport and signed
`sum.golang.google.cn` checksum endpoint completed. No checksum bypass,
`replace`, vendoring, or alternate module bytes were used.

## Boundary audits

- `internal/tui` and `cmd/loom/tui.go` contain no SQL, SQLite driver, Journal,
  Projection, shell execution, service-manager, or network HTTP import.
- `internal/api` does not import TUI or local IPC.
- `internal/localipc` does not import CLI, daemon, StateWriter, Journal, or
  Projection.
- The W1 secret-negative scan found only the installer test's literal list of
  forbidden credential marker names; it found no credential value, Provider
  URL, token, or API key.
- The pre-existing user-owned Phase 1, `AGENTS.md`, and `PROGRESS.md` changes
  remain excluded.
- `com.earendilworks.loom.runtime-observer` remains listed as running and was
  not unloaded, reloaded, rewritten, or signalled during W1.

## Not yet authorized or claimed

- Live Gate 1 failed at immediate launchd bootstrap and rolled back exactly.
  Reviewed Amendment 2 allowed one replacement after bounded namespace
  quiescence. That replacement bootstrapped the candidate and private socket
  but found residual Provider keys in loaded launchd metadata, so it rolled
  back before opening the TUI.
- No installed binary, LaunchAgent plist, credential, Provider, Runtime
  execution, Team, WorkPackage, Run, Journal fact, or user state was changed by
  deterministic verification.
- Implementation Reviews 1 and 2 returned `FAIL`; their complete findings are
  preserved in `implementation-review-1.md` and
  `implementation-review-2.md`. Repair 1 and Repair 2 close those findings in
  deterministic evidence.
- Implementation Review 3 returned `PASS` with no findings after independent
  focused, race, repository, vet, Windows, installer, module, and diff checks.
- Both live allowances are consumed. W1 is `HUMAN_REQUIRED` and may not be
  accepted, installed, committed, retried, or advanced to W2 under the frozen
  contract.
