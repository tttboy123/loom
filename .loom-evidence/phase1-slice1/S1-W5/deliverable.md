# S1-W5 Deliverable

## 1. What changed

- Added `loom route` for structured Mode Router input and deterministic JSON
  mode output.
- Added `loom status` for read-only SQLite state opening, projection rebuild,
  and sorted mode, WorkItem, and Evidence JSON output.
- Added frozen exit codes `0` success, `2` invalid input, and `3` state
  unavailable.
- Added success, invalid-input, unavailable-state, replay-failure,
  cancellation, read-only, deterministic-order, and no-undeclared-call tests.
- Added URI-safe handling for real local state filenames containing reserved
  URI characters.

## 2. Result and exact evidence

- Initial mandatory RED:
  `go test ./cmd/loom -run 'TestRun' -count=1`
  failed to compile because `run`, `productionDeps`, and `runDeps` did not
  exist.
- The first Developer wrote the initial Candidate but was interrupted before
  GREEN. A replacement Developer sequentially corrected a package-local test
  path and ran GREEN. This two-session writer handoff is a disclosed Controller
  process deviation; there was no concurrent or ambiguous overlapping write.
- Initial Controller verification passed focused, focused race at `-count=50`,
  CLI full/race, repository full/race, vet, formatting, diff, and route smokes.
- The first fresh Reviewer returned `VERDICT: FAIL` after a live database named
  with `?` proved that raw path interpolation changed SQLite URI parsing.
- Repair 1 RED reproduced exit `3` for existing migrated
  `journal?query.db` and `journal#fragment.db`.
- Repair 1 encoded the absolute local path with `net/url.URL` while preserving
  SQLite `mode=ro` and `query_only(1)`.
- Controller verification after Repair 1 exited 0:
  - special paths at `-count=100`: `ok ... 0.684s`;
  - special paths race at `-count=50`: `ok ... 2.841s`;
  - focused CLI: `ok ... 0.330s`;
  - focused CLI race at `-count=50`: `ok ... 4.725s`;
  - CLI full/race: `0.286s` / `1.374s`;
  - repository full/race: all packages passed;
  - `go vet ./...`, `gofmt` cleanliness, and `git diff --check` passed;
  - binary route smokes returned exact conversation and agent JSON.
- A new fresh Reviewer independently passed package/repository/race/vet/diff
  checks and a stronger decoy-versus-special-path live smoke, then returned
  `VERDICT: PASS`.

## 3. Affected files and behavior

- `cmd/loom/main.go`
- `cmd/loom/query.go`
- `cmd/loom/main_test.go`
- Route never opens state and delegates exclusively to the accepted Mode
  Router.
- Status opens only an existing read-only state database, queries exclusively
  through the accepted projection, closes the handle, and never fabricates
  empty success when state is unavailable.
- No command creates Team, Agent, Run, daemon, network, or background behavior.

## 4. Remaining risk and unverified boundary

- This is a direct local read-only state adapter because no daemon/API exists
  yet; no live reconnect or daemon lifecycle was verified.
- SQLite and binary runtime checks ran on the local Darwin host only.
- Verification used local Go 1.26.4; the Go 1.22 floor was not run with a
  separately installed toolchain.
- The sequential Developer-session handoff deviated from the one-writer budget,
  though the Reviewer found no product correctness blocker from it.
- All Slice 1 work remains uncommitted in a dirty/untracked detached checkout.

## 5. Next executable step

Run the final Slice 1 repository digest checks, confirm all five deliverables
end in `VERDICT: PASS`, update final local status, and stop at the Slice 1
terminal boundary. Commit, push, merge, or Slice 2 require new human direction.

VERDICT: PASS
