# Phase 2C Repair Deterministic Verification

**Phase status**: PARTIAL  
**Source-lock SHA-256**: `73ca98396b3caeea5e936f5543b11d77fbb09a064d32490cfc1592fff4344fa6`  
**Ordered source digest**: `ad6ca86a212fe2ff925ada3781bac7773e3daa1644ec949ad600b4f16ac7a7c2`  
**Repository**: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`  
**Branch**: `codex/loom-platform-slice2`

This record is append-only. Failed and superseded attempts remain visible.

## Attempt 1 - Lock-bound Go Matrix

### Serial normal

Command:

```text
go test -timeout=12m -p 1 ./... -count=1
```

Result: PASS. The complete repository command exited 0. Notable long-running
packages included `cmd/loomd` (63.833s), `internal/localipc` (173.030s), and
`internal/runtime/piadapter` (70.900s).

### Serial race

Command:

```text
go test -race -timeout=12m -p 1 ./... -count=1
```

Result: FAIL. The complete repository command exited 1 after continuing through
all packages. One test failed:

```text
--- FAIL: TestSystemCodexStatusRunnerBoundsOutputAndCancelsProcessGroup (0.82s)
    codex_native_auth_test.go:454: EOF
FAIL loom-pi-rebuild/internal/provider
```

All other reported packages passed, including `cmd/loomd` (246.460s),
`internal/app` (66.719s), `internal/localipc` (186.214s), and
`internal/runtime/piadapter` (90.594s). This failure blocks deterministic
acceptance. It will be diagnosed without rewriting or replacing this attempt.

### Failure diagnosis

The cancellation fixture waits only for `os.Stat(pidFile)` before canceling.
Shell redirection creates the PID file before `printf` writes the PID, so the
test can observe an empty file and kill the process group between those two
operations. The later `fmt.Sscanf` then returns `EOF` at the reported line.
Production process-group cancellation completed with the expected
`context.Canceled`; the observed failure is a test synchronization race.

Focused diagnostic:

```text
go test -race ./internal/provider \
  -run '^TestSystemCodexStatusRunnerBoundsOutputAndCancelsProcessGroup$' \
  -count=50
```

Result: PASS in 29.446s. The intermittent full-matrix failure remains valid and
requires the fixture to wait for a parseable PID, not mere file existence.

## Repair 2 Pre-lock Diagnostic

The fixture now writes the child PID to `child.pid.tmp` and publishes the final
`child.pid` with `/bin/mv`. The production `SystemCodexStatusRunner` is
unchanged.

```text
go test -race ./internal/provider \
  -run '^TestSystemCodexStatusRunnerBoundsOutputAndCancelsProcessGroup$' \
  -count=100
```

Result: PASS in 117.295s. `go test ./internal/provider -count=1` also passed in
8.520s. These are pre-lock diagnostics; Repair 2 still requires Contract
Re-review, a new source lock, and complete normal/race reruns.

## Attempt 2 - Repair 2 Lock-bound Matrix

**Repair 2 source-lock SHA-256**:
`eb0376b6d8f42da8e6ac586b57fe4796fdc887b2c84c14c8dac6988e252feee9`  
**Repair 2 ordered source digest**:
`c8b687fe8fc19a87398439c7c6da326bf4de28eb816e9fe71146cd842600d7e5`

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS |
| Static analysis | `go vet ./...` | PASS |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over every locked Go path | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

The full race command passed `internal/provider` in 3.249s, including the
repaired cancellation fixture. Other affected/high-risk packages passed:
`cmd/loomd` 67.660s, `internal/app` 18.822s, `internal/localipc` 28.706s, and
`internal/runtime/piadapter` 43.260s. No package was substituted by a focused
rerun.

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS: 111 tests, 1 intentional visual-export skip, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS: 111 tests, 1 intentional visual-export skip, 0 failures |

The known Swift 6 warning remains: `QueueCommand<Input>` declares `Sendable`
without constraining `Input: Sendable`. It is pre-existing non-blocking debt and
does not produce a TSAN failure under the current Swift language mode.

### Release bundle

The accepted bundle builder ran from the locked source:

```text
scripts/build-loom-local-app.sh \
  --output /tmp/loom-phase2c-repair2.Aezpwz/Loom.app
```

Result: PASS.

- bundle: `/tmp/loom-phase2c-repair2.Aezpwz/Loom.app`
- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `cb3134e9353ff9c1d237fb8554ddf8db743155ca4868062b280961273624116c`
- executable mode: `0700`
- symlinks: `0`
- `codesign --verify --strict`: PASS

### Post-matrix lock check

The 36 locked source paths recompute exactly to
`c8b687fe8fc19a87398439c7c6da326bf4de28eb816e9fe71146cd842600d7e5`.
The deterministic matrix is green. This does not accept J1-J10, any WorkItem,
ADR-0015, Phase 2C, or Product Owner sign-off.

## Attempt 3 - Repair 3 Lock-bound Matrix

**Repair 3 source-lock SHA-256**:
`43cce64c18d262cbaeb04c0f0da8f91148261b5a752c05a0e61ab6d92d507356`  
**Repair 3 ordered source digest**:
`9a325c4138913a1c092ead8a8704fbdbded4e14c87e2bc9bd35e13f48c5e4302`

Attempt 001 of the cross-client journey found inert TUI Home `i` and `u`
actions after the Repair 2 matrix. Repair 3 is test-first, independently
reviewed, and frozen under the lock above. The complete matrix below is a new
run; no Repair 2 result is reused.

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS in 184.52s |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS in 527.80s |
| Static analysis | `go vet ./...` | PASS in 1.98s |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over every locked Go path | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

Notable normal package times were `cmd/loomd` 65.297s, `internal/localipc`
29.441s, `internal/runtime/piadapter` 50.017s, and `internal/tui` 0.508s.
Notable race package times were `cmd/loomd` 68.154s, `internal/app` 50.177s,
`internal/localipc` 130.941s, `internal/provider` 4.231s,
`internal/runtime/piadapter` 107.956s, and `internal/tui` 1.755s. The complete
provider package, including the repaired cancellation fixture, passed in the
full matrix.

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS in 93.69s: 111 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS in 135.85s: 111 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |

The known Swift 6 `QueueCommand<Input>` Sendable warning remains unchanged and
produced no TSAN failure.

### Release bundle

```text
scripts/build-loom-local-app.sh \
  --output /private/tmp/loom-phase2c-repair3-release-001/Loom.app
```

Result: PASS in 18.87s.

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `cb3134e9353ff9c1d237fb8554ddf8db743155ca4868062b280961273624116c`
- executable mode: `0700`
- symlinks: `0`
- `codesign --verify --strict`: PASS

### Post-matrix lock check

The 36 locked source paths recompute exactly to
`9a325c4138913a1c092ead8a8704fbdbded4e14c87e2bc9bd35e13f48c5e4302`.
The Repair 3 deterministic matrix is green. This does not accept J1-J10, any
WorkItem, ADR-0015, Phase 2C, or Product Owner sign-off.

## Attempt 4 - Repair 4 Lock-bound Matrix

**Repair 4 source-lock SHA-256**:
`6b252744ed8d52df4edd3a0c71fbae7db82fe85655c4aeaaa35d25bf79f2d6fc`  
**Repair 4 ordered source digest**:
`0ebbd1d04b3c8c1c4cd8eecac56d991983f9eb81f8931f82211e6b17da2c4b12`

Attempt 002 of the real cross-client journey exposed that the TUI shared entry
parser ignored physical `tea.KeySpace` events. Repair 4 is test-first,
independently reviewed, and frozen under the lock above. The complete matrix is
a fresh run; no Repair 2 or Repair 3 result is reused.

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS in 187.00s |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS in 712.26s |
| Static analysis | `go vet ./...` | PASS in 0.73s |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over every locked Go path | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

Notable normal package times were `cmd/loomd` 66.252s,
`internal/localipc` 30.062s, `internal/runtime/piadapter` 50.413s, and
`internal/tui` 0.513s. Notable race package times were `cmd/loomd` 351.915s,
`internal/app` 61.694s, `internal/localipc` 99.441s,
`internal/provider` 3.348s, `internal/runtime/piadapter` 65.141s, and
`internal/tui` 1.645s. Every package completed inside the 12-minute package
timeout and no race finding was reported.

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS in 16.00s: 111 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS in 30.09s: 111 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |

The known Swift 6 `QueueCommand<Input>` Sendable warning remains unchanged and
produced no TSAN failure.

### Release bundle

The first invocation correctly rejected a nonexistent output parent with
`invalid output parent`. After creating the fresh temporary parent, the same
locked builder command ran:

```text
scripts/build-loom-local-app.sh \
  --output /private/tmp/loom-phase2c-repair4-release-001/Loom.app
```

Result: PASS in 2.87s.

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `cb3134e9353ff9c1d237fb8554ddf8db743155ca4868062b280961273624116c`
- executable mode: `0700`
- symlinks: `0`
- `codesign --verify --deep --strict`: PASS

### Post-matrix lock check

The 36 locked source paths recompute exactly to
`0ebbd1d04b3c8c1c4cd8eecac56d991983f9eb81f8931f82211e6b17da2c4b12`.
The source-lock file remains
`6b252744ed8d52df4edd3a0c71fbae7db82fe85655c4aeaaa35d25bf79f2d6fc`,
the staged path count is zero, and the Repair 4 deterministic matrix is green.
This authorizes a fresh attempt 003 fixture; it does not accept J1-J10, any
WorkItem, ADR-0015, Phase 2C, or Product Owner sign-off.

## Attempt 5 - Repair 5 Lock-bound Matrix

**Repair 5 ordered source digest**:
`c07fdfcc2ee0bfda9c7a73963292b89951751618011c5e38d1ba8a3173e90c4a`  
**Final non-self-referential review binding**:
`5631d8ecf64eb609dc8d3037e38012dfe55aab5fdeb8908611dbbb84436a89a1`

Attempt 003 proved the Repair 4 physical-space closure, then exposed that a
5.270493-second first-use `chat_message` response crossed the ordinary
five-second server/TUI deadlines. Repair 5 is test-first and independently
reviewed. Its exact 43-path source basis is frozen by the ordered digest above;
the lock's self hash is intentionally non-normative to avoid review-binding
self-reference. The complete matrix below is a fresh run and no prior result is
reused.

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS in 244.23s |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS in 401.04s |
| Static analysis | `go vet ./...` | PASS in 0.53s |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over every locked Go path | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

Notable normal package times were `cmd/loom` 0.629s, `cmd/loomd` 60.865s,
`internal/localipc` 35.666s, `internal/runtime/piadapter` 89.175s, and
`internal/tui` 0.762s. Notable race package times were `cmd/loom` 2.345s,
`cmd/loomd` 126.413s, `internal/app` 31.591s, `internal/localipc` 46.971s,
`internal/provider` 3.346s, `internal/runtime/piadapter` 72.532s, and
`internal/tui` 1.589s. No race finding was reported.

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS in 4.21s: 111 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS in 19.36s: 111 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |

The Repair 5 long-operation timeout test passed inside both complete suites.
The known Swift 6 `QueueCommand<Input>` Sendable warning remains unchanged and
produced no TSAN failure.

### Release bundle

```text
scripts/build-loom-local-app.sh \
  --output /private/tmp/loom-phase2c-repair5-release-001/Loom.app
```

Result: PASS in 1.56s.

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `6a02ede1a29e4b291bc90934b52e58ccfe24baaf21abb2378fea91cdea6b81b4`
- executable mode: `0700`
- symlinks: `0`
- `codesign --verify --deep --strict`: PASS

### Post-matrix source check

The 43 frozen non-lock source paths recompute exactly to
`c07fdfcc2ee0bfda9c7a73963292b89951751618011c5e38d1ba8a3173e90c4a`.
The final review binding remains
`5631d8ecf64eb609dc8d3037e38012dfe55aab5fdeb8908611dbbb84436a89a1`
and the staged path count is zero. The Repair 5 deterministic matrix is green.
This authorizes preparation of a fresh replacement fixture; it does not accept
J1-J10, any WorkItem, ADR-0015, Phase 2C, or Product Owner sign-off.

## Attempt 6 - Repair 6 Lock-bound Matrix

**Repair 6 ordered source digest**:
`2afe8693ac07ed12d21fcc981485c9e8d1f2b7a61cde519c87238294cbd31971`  
**Repair 6 source-lock SHA-256**:
`b03dac95666bc590907149ba8f5604a4f97682a06207dcfa741c25cafaa3db68`  
**Final lock review**:
`PHASE-2C-REPAIR-6-SOURCE-LOCK-REVIEW-3.md`, `PASS`, P0=P1=P2=0

Attempt 004 proved the Repair 5 deadline, TUI J1/J2, and native folder picker,
then exposed that native `chat_message` encoded `threadID` while the strict
daemon contract requires `thread_id`. Repair 6 is test-first, independently
reviewed, and limited to the native request key mapping plus exact-shape test.
Its exact 44-path non-lock source basis is frozen by the ordered digest above.
No prior matrix result is reused.

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS in 530.75s |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS in 488.38s; no race finding |
| Static analysis | `go vet ./...` | PASS in 0.47s |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over every locked Go path | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

Notable normal package times were `cmd/loom` 1.779s, `cmd/loomd` 203.472s,
`internal/app` 21.927s, `internal/localipc` 155.263s,
`internal/runtime/piadapter` 70.386s, and `internal/tui` 1.728s. Notable race
times were `cmd/loom` 2.540s, `cmd/loomd` 154.226s, `internal/app` 55.204s,
`internal/localipc` 54.377s, `internal/provider` 3.485s,
`internal/runtime/piadapter` 92.365s, and `internal/tui` 1.661s.

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS in 6.22s: 112 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS in 29.34s: 112 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |

The exact `thread_id` plus `content` regression passed in both complete suites.
The existing Swift 6 `QueueCommand<Input>` Sendable warning remains unchanged
and produced no TSAN failure.

### Release bundle

```text
scripts/build-loom-local-app.sh \
  --output /private/tmp/loom-phase2c-repair6-release-001/Loom.app
```

The script first rejected the absent output parent with `invalid output parent`.
After creating that dedicated parent as `0700`, the same build passed in 7.31s.

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `cd79df38f5277873cfd02822e5e0f7c5ff9477725ee9444a6a73ac53a2859a1f`
- executable mode: `0700`
- symlinks: `0`
- `codesign --verify --deep --strict`: PASS

### Post-matrix source check

The 44 frozen non-lock source paths recompute exactly to
`2afe8693ac07ed12d21fcc981485c9e8d1f2b7a61cde519c87238294cbd31971`.
The staged path count is zero and no test or release process remains. The Repair
6 deterministic matrix is green. This authorizes preparation of a fresh clean
attempt 005 fixture; it does not accept J1-J10, any WorkItem, ADR-0015, Phase
2C, or Product Owner sign-off.

## Attempt 7 - Repair 7 Lock-bound Matrix (Failed)

**Repair 7 ordered source digest**:
`a0499852cd942d20e5d238f1ec01fa2e83bfe2ed4ee497b821782b8319095504`  
**Repair 7 source-lock SHA-256**:
`cbe22148444923e14312fa32b7149ba6a541006adfc306702d754249bf843e21`

The serial normal command `go test -timeout=12m -p 1 ./... -count=1` passed in
`414.56s`. The serial race command
`go test -race -timeout=12m -p 1 ./... -count=1` failed in `697.27s` at
`TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage` with
`Team attempt evidence commit main/1: context deadline exceeded`; Side-task
reconciliation then failed closed as unavailable. All other reported packages
passed and no race detector data race was reported.

The failure is causal evidence for Repair 8: `commitTeamTaskOutcomes` shared one
five-second detached context across all outcomes. Repair 8 assigns each attempt
its own bounded 15-second capture/receipt commit context. This source change
invalidates the Repair 7 lock and every passing result above for acceptance.
No Journey, WorkItem, ADR, Phase, staging, commit, push, or merge is authorized.

## Attempt 8 - Repair 8 Lock-bound Matrix

**Repair 8 ordered source digest**:
`e64e0ba41e84b04d0d0255497d94521383aaeebc8327845896d388435e49b745`  
**Repair 8 source-lock SHA-256**:
`641437c30000a71ad5c9d73eeae08aa1f2ed5b3da7c4e7d6058513aef1eb2322`  
**Final contract review**:
`PHASE-2C-REPAIR-8-CONTRACT-REVIEW.md`, SHA-256
`99208476bbf8769cc452876768bf80b2893885c41e4f6c36f174f4cbe188db6b`,
`PASS`, P0=P1=P2=0

Repair 8 replaces the shared terminal-outcome commit deadline exposed by the
Attempt 7 race failure. Each outcome receives its own detached bounded
15-second commit context, and the coordinator attempts every terminal outcome
before joining any errors. Its exact 51-path non-lock source basis is frozen by
the ordered digest above. No prior matrix result is reused.

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS in 416.25s |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS in 527.88s; no race finding |
| Static analysis | `go vet ./...` | PASS, no output |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over every locked Go path | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

Notable normal package times were `cmd/loom` 2.131s, `cmd/loomd` 183.623s,
`internal/app` 11.722s, `internal/localipc` 62.677s,
and `internal/tui` 0.822s. Notable race times were `cmd/loom` 2.120s,
`cmd/loomd` 287.115s, `internal/api` 8.133s, `internal/app` 22.161s,
`internal/localipc` 39.189s, `internal/runtime/piadapter` 63.589s, and
`internal/tui` 1.589s.

The previously failing daemon lineage test passed inside the full race suite.
The dedicated regression also proves that a missing earlier terminal capture
does not prevent later outcomes from receiving Evidence, Team attempt terminal,
Team execution terminal, and receipt commits.

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS: 114 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS: 114 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |

The stale-draft recovery, tool-shaped tentative-prose downgrade, conversation
continuity, and governance-panel regressions passed in both complete suites.
The existing Swift 6 `QueueCommand<Input>` Sendable warning remains unchanged
and produced no TSAN failure.

### Release bundle

```text
scripts/build-loom-local-app.sh \
  --output /private/tmp/loom-repair8-release.pHYPAl/Loom.app
```

The fresh output parent was user-owned mode `0700`. The production build passed
in 70.76s.

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `fd8603d6bb7ecd99aa32ba972d54b022fa0b7143f3091f51ee509833ab2534e4`
- bundle directories and executable mode: `0700`
- non-executable file mode: `0600`
- symlinks: `0`
- `codesign --verify --deep --strict --verbose=4`: PASS

### Post-matrix source check

The 51 frozen non-lock source paths recompute exactly to
`e64e0ba41e84b04d0d0255497d94521383aaeebc8327845896d388435e49b745`.
The source-lock file remains
`641437c30000a71ad5c9d73eeae08aa1f2ed5b3da7c4e7d6058513aef1eb2322`,
the staged path count is zero, and no Repair 8 test or release process remains.
The unrelated user-owned `demo-resident` daemon was observed and left running.
The Repair 8 deterministic matrix is green. This authorizes preparation of a
fresh clean attempt 006 fixture; it does not accept J1-J10, any WorkItem,
ADR-0015, Phase 2C, or Product Owner sign-off.

## Attempt 9 - Repair 9 Lock-bound Matrix

**Repair 9 ordered source digest**:
`15696d1f7f59d3e64a80839e4021972a8661bacae5b6e115057ee581b18d5867`  
**Repair 9 source-lock SHA-256**:
`fe5ea9bf23399c0a8e7c9361629b0a72d84321ba55206b373b1cadfc90758952`  
**Final contract re-review**:
`PHASE-2C-REPAIR-9-FINAL-REREVIEW-3.md`, SHA-256
`99ce0ce1fda571fbd974ce1db6c26bdc71bd7618d78e0e1795a9b99992bb6c84`,
`PASS`, P0=P1=P2=0

Repair 9 closes the exact TUI decision operation drift, restart-expired Team
Draft recovery, consumed/stale Mission preflight continuity, and Native Store
prepared-action/result validation. The exact 51-path candidate, including 50
non-lock paths, is frozen by the ordered digest above. No prior matrix result
is reused.

### Preserved initial normal failure

The first serial normal command
`go test -timeout=12m -p 1 ./... -count=1` failed after `487.89s` in
`TestProductDaemonExecutionCompositionMaterializesConfirmedTeamForPreflight`.
Its first `StartBuilder` call returned `local product unavailable`; every other
reported package passed. Repair 9 did not change daemon composition or that
test, so the failure was investigated before any replacement matrix result was
accepted.

On the same frozen source:

- the exact test passed 20 consecutive focused runs;
- the complete `cmd/loomd` package passed in `148.166s`;
- a replacement full serial normal run passed in `395.23s`;
- the full serial race run also passed, including the same daemon test.

No process leak or source drift followed the first failure. This is retained as
a non-reproducing startup availability observation, not erased, promoted, or
used to claim a product repair.

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal, first run | `go test -timeout=12m -p 1 ./... -count=1` | FAIL in 487.89s; one daemon startup availability failure |
| Exact causal repetition | focused daemon test, `-count=20 -v` | PASS 20/20 in 0.748s package time |
| Complete daemon package | `go test -timeout=12m ./cmd/loomd -count=1` | PASS in 148.166s |
| Full serial normal, replacement | `go test -timeout=12m -p 1 ./... -count=1` | PASS in 395.23s |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS in 308.77s; no race finding |
| Static analysis | `go vet ./...` | PASS, no output |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over all 26 locked Go paths via `xargs` | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

An initial zsh formatting diagnostic incorrectly used the special variable
name `path`, clobbered `PATH` inside its loop, and emitted a false shell-level
PASS based on empty stdout. It was rejected and replaced by the explicit
26-path `xargs gofmt -l` gate above.

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS: 120 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS: 120 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |

The exact decision submit, non-authoritative/identity-drift rejection,
stale/missing Team Draft recovery, successful-start consumption, stale
preflight expiration, no-repeat Start, conversation continuity, and governance
panel regressions passed in both complete suites. The existing Swift 6
`QueueCommand<Input>` Sendable warning remains unchanged and produced no TSAN
failure.

### Release bundle

```text
scripts/build-loom-local-app.sh \
  --output /private/tmp/loom-repair9-release.5kASyR/Loom.app
```

The fresh output parent was user-owned mode `0700`. The production build
passed.

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `d18d16a0a538892c194a5e620489c97f52643e0930c04e8fc9815d17b4b31822`
- bundle directories and executable mode: `0700`
- non-executable file mode: `0600`
- symlinks: `0`
- `codesign --verify --deep --strict --verbose=4`: PASS

### Post-matrix source check

The 50 frozen non-lock source paths recompute exactly to
`15696d1f7f59d3e64a80839e4021972a8661bacae5b6e115057ee581b18d5867`.
The source-lock file remains
`fe5ea9bf23399c0a8e7c9361629b0a72d84321ba55206b373b1cadfc90758952`,
the staged path count is zero, and no Repair 9 test or release process remains.
The unrelated user-owned `demo-resident` daemon was observed and left running.
The Repair 9 replacement deterministic matrix is green. This authorizes
preparation of a fresh clean Attempt 007 fixture; it does not accept J1-J10,
any WorkItem, ADR-0015, Phase 2C, or Product Owner sign-off.

## Attempt 10 - Repair 10 Lock-bound Matrix

**Repair 10 ordered source digest**:
`7396c1c42e0663f2ccd60a6ec51b53577884ab2cd9a1583349ff4afcea7f11ee`  
**Repair 10 source-lock SHA-256**:
`69278d6bb5b08fe47cd370e21de5ebee74e73d3e17625b1eccad993cd967508f`  
**Final independent review**:
`PHASE-2C-REPAIR-10-CONTRACT-REVIEW.md`, SHA-256
`2491129d677f6482379faadd91d5df724b8f90d43a7c1a4444f268eae17db70f`,
`PASS`, P0=P1=P2=0

Repair 10 adds the strict Phase 2C-only private manifest source for one
authoritative Runtime online-to-offline transition. It reuses the accepted
Runtime discovery, projected baseline, reconciliation, prepared committer,
Event Journal, and projection path before IPC readiness. Exact post-commit
daemon restart is a verified zero-append replay; manifest, Event, payload,
projection, path, permission, source, time, or identity drift fails closed.
Ordinary Pi observation owns the offline-to-online recovery fact. No prior
matrix result is reused.

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS; all packages green |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS; no race finding |
| Static analysis | `go vet ./...` | PASS, no output |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over all 26 locked Go paths | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

The frozen runs include the private-manifest rejection matrix, exact Journal
fact and payload checks, offline projection before IPC, exact zero-append daemon
restart, unmatched-offline rejection with byte-identical Journal facts, and
ordinary Pi offline-to-online recovery.

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS: 120 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS: 120 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |

The existing Swift 6 `QueueCommand<Input>` Sendable warning remains unchanged
and produced no Thread Sanitizer failure.

### Release bundle

```text
scripts/build-loom-local-app.sh \
  --output /private/tmp/loom-repair10-release.LwXV8f/Loom.app
```

The fresh output parent is user-owned mode `0700`; the production build passed.

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `d18d16a0a538892c194a5e620489c97f52643e0930c04e8fc9815d17b4b31822`
- bundle directories and executable mode: `0700`
- non-executable file mode: `0600`
- symlinks: `0`
- `codesign --verify --deep --strict --verbose=4`: PASS

### Post-matrix source check

The 50 frozen non-lock source paths recompute exactly to
`7396c1c42e0663f2ccd60a6ec51b53577884ab2cd9a1583349ff4afcea7f11ee`.
The source-lock file remains
`69278d6bb5b08fe47cd370e21de5ebee74e73d3e17625b1eccad993cd967508f`,
the staged path count is zero, `git diff --check` is clean, and no Repair 10
test or Release process remains. The Repair 10 replacement deterministic matrix
is green. This authorizes preparation of a new clean replacement journey; it
does not accept J1-J10, any WorkItem, ADR-0015, Phase 2C, or Product Owner
sign-off.

## Repair 11 - Causal RED/GREEN

### RED

| Boundary | Command | Exact failure |
|---|---|---|
| TUI Attention | `go test ./internal/tui -run 'TestAttentionRefreshReloadsEveryRenderedSource' -count=1` | FAIL: Attention `r` returned the single permission-attention command rather than a two-source batch. |
| Native Recent | `swift test --filter LocalProductStoreTests/testWorkspaceAttentionUsesActionContextAndKindFallback` | FAIL: both authoritative attention items rendered `Needs your attention` instead of `Restore Runtime` and `Inspect Failure`. |

The failures were causal and preceded the Repair 11 product changes.

### GREEN

| Gate | Command | Result |
|---|---|---|
| Focused TUI | `go test ./internal/tui -run 'TestAttentionRefreshAtomicallyReloadsEveryRenderedSource' -count=1` | PASS |
| Full TUI package | `go test ./internal/tui -count=1` | PASS |
| Focused TUI race | `go test -race ./internal/tui -run 'TestAttentionRefreshAtomicallyReloadsEveryRenderedSource|TestModelNavigatesAllReadScreensAndNeverCreatesMutationCommand' -count=1` | PASS |
| Focused Native | `swift test --filter LocalProductStoreTests/testWorkspaceAttentionUsesActionContextAndKindFallback` | PASS, including ANSI-control sanitization and kind fallback |
| Native Store suite | `swift test --filter LocalProductStoreTests` | PASS: 37 tests, 0 failures |
| Focused Native TSAN | `swift test --sanitize=thread --filter LocalProductStoreTests/testWorkspaceAttentionUsesActionContextAndKindFallback` | PASS: 1 test, 0 failures |

The existing Swift 6 `QueueCommand<Input>` Sendable warning remains unchanged.
These pre-lock checks authorize independent source review only. They are not the
final lock-bound deterministic matrix or Journey evidence.

### Source-review remediation

The first independent source review returned `P0=0`, `P1=1`, `P2=1`. The
two-command batch could expose one source before the other settled and retain
stale permission actions after a permission read failure. Its test covered only
one successful completion order.

The remediation now returns one generation-bound aggregate after both reads
settle. Loading remains true until that result arrives; older generations are
ignored; a snapshot failure preserves the last product snapshot; and either
source failure removes permission actions before presenting the existing
truthful error state. New tests cover `r`, Tab entry, Shift-Tab entry, no
permission capability, permission and snapshot failure, and older completions
arriving both before and after the current result.

| Gate | Command | Result |
|---|---|---|
| Full TUI package | `go test ./internal/tui -count=1` | PASS |
| Repair 11 focused race | `go test -race ./internal/tui -run 'TestAttentionRefresh|TestAttentionEntry|TestModelNavigatesAllReadScreensAndNeverCreatesMutationCommand' -count=1` | PASS |
| Diff hygiene | `git diff --check` | PASS |

These remediation checks authorize independent source re-review only. The
Repair 10 source lock remains invalidated.

### Source re-review 2 remediation

Re-review 2 returned `P0=0`, `P1=1`, `P2=0`: cached permission actions were
hidden by loading presentation but remained reachable by `g`, `a`, or `x`
before the aggregate refresh completed. `beginAttentionRefresh` now removes
those cached permission actions before starting the next generation. A focused
regression proves all three keys produce no command during that interval.

| Gate | Command | Result |
|---|---|---|
| Focused TUI | `go test ./internal/tui -run 'TestAttentionRefresh|TestAttentionEntry' -count=1` | PASS |
| Focused TUI race | `go test -race ./internal/tui -run 'TestAttentionRefresh|TestAttentionEntry' -count=1` | PASS |

The exact current bytes require another independent source re-review before a
replacement source lock may be generated.

### Final source re-review

Independent source Re-review 3 returned `PASS`, `P0=0`, `P1=0`, `P2=0`. The
adjacent `go test ./cmd/loom ./internal/tui -count=1` and matching race command
both pass after the final permission-window fix. `LocalProductStoreTests`
remains green at 37 tests with no failures. Final status-byte binding and a
replacement source lock are the current gates; no Journey evidence is yet
authorized.

The first final status-byte review returned `P0=0`, `P1=0`, `P2=1` because the
Candidate Boundary Purpose still described completed Repair 11 review gates as
remaining. The Purpose now records those gates as passed and leaves only the
replacement lock, deterministic matrix, and clean journey as pending. An exact
status-byte re-review is required before lock generation.

## Repair 11 - First lock-bound matrix failure

The first Repair 11 lock bound 50 non-lock paths to ordered digest
`b85242621bbd18719399db5108de3c1701f9edbbc67d49da416bda2b65f5df39`;
its lock SHA-256 was
`c028fa5ebbf454d428ac7566adefada7271c5adfcd36e9a50b1b6933bb64ec9b`.
Independent lock review passed with no P0/P1/P2 findings.

`go test -timeout=12m -p 1 ./... -count=1` then failed only in
`TestStrictSwiftClientReadsSetupAndStartsCandidateFromRealGoServer`. The real
Go-to-Swift contract probe explicitly compiles `LocalProductStore.swift` but not
`SafeText.swift`, so the Repair 11 Store reference to `SafeText` did not compile.
All other packages completed green. The failed lock remains preserved evidence
and no race, Swift, Release, or Journey result is reused from it.

The bounded remediation stays in the declared `LocalProductStore.swift` path:
attention titles use an equivalent private, length-bounded sanitizer that drops
CSI/OSC escapes, control characters, and bidi controls. No IPC test, compile
manifest, product dependency, Event, authority, or filesystem boundary changes.
A causal focused contract-probe GREEN, Swift tests, independent source re-review,
new lock, and complete replacement matrix are required.

### Matrix-remediation GREEN

| Gate | Command | Result |
|---|---|---|
| Real Go-to-Swift contract | `go test ./internal/localipc -run TestStrictSwiftClientReadsSetupAndStartsCandidateFromRealGoServer -count=1` | PASS in 22.619s |
| Native Attention safety | `swift test --filter LocalProductStoreTests/testWorkspaceAttentionUsesActionContextAndKindFallback` | PASS; CSI, OSC, bidi, 48-character bound, action context, and kind fallback |
| Diff hygiene | `git diff --check` | PASS |

These checks authorize independent exact-byte remediation review only. The
failed Repair 11 lock remains invalid and no later matrix result is reused.

Independent matrix-remediation review returned `PASS`, `P0=0`, `P1=0`,
`P2=0`. Adjacent verification also passed:

| Gate | Command | Result |
|---|---|---|
| Full local IPC | `go test ./internal/localipc -count=1` | PASS in 67.882s |
| Native Store suite | `swift test --filter LocalProductStoreTests` | PASS: 37 tests, 0 failures |
| Native Attention TSAN | `swift test --sanitize=thread --filter LocalProductStoreTests/testWorkspaceAttentionUsesActionContextAndKindFallback` | PASS: 1 test, 0 failures |

A new source lock is now authorized. The complete replacement matrix remains
unrun and no Journey is authorized.

## Repair 11 - Replacement lock-bound matrix

**Ordered source digest**:
`076ad9ddfe5a9b816dc74f9fbba6024bad074c99db2e0799fd5f19d3950e9f39`  
**Source-lock SHA-256**:
`f9bc2e977ec72488b8fa382ed0900a80f90000508f722714b5cb120380450948`

Independent replacement-lock review returned `PASS`, `P0=0`, `P1=0`,
`P2=0` and recomputed every binding.

### Go

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS; all packages green, including standalone Swift Store probe |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS; no race finding |
| Static analysis | `go vet ./...` | PASS, no output |
| Module integrity | `go mod tidy -diff` | PASS, no output |
| Formatting | `gofmt -l` over all locked Go paths | PASS, no output |
| Diff hygiene | `git diff --check` | PASS, no output |

### Swift

| Gate | Command | Result |
|---|---|---|
| Standard | `swift test` | PASS: 121 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |
| Thread Sanitizer | `swift test --sanitize=thread` | PASS: 121 XCTest cases, 1 intentional visual-export skip, 4 Swift Testing cases, 0 failures |

The existing Swift 6 `QueueCommand<Input>` Sendable warning remains unchanged
and produced no Thread Sanitizer failure.

### Release bundle

Fresh output:
`/private/tmp/loom-repair11-release.cXY4zH/Loom.app`

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable: `Contents/MacOS/LoomLocalApp`
- executable SHA-256:
  `d5f747d151ca34928bf57cd93c758272373067292c2139ea295f10f687f45d84`
- bundle directories and executable mode: `0700`
- non-executable file mode: `0600`
- symlinks: `0`
- `codesign --verify --deep --strict --verbose=4`: PASS

### Post-matrix source check

The 50 frozen non-lock paths still recompute exactly to
`076ad9ddfe5a9b816dc74f9fbba6024bad074c99db2e0799fd5f19d3950e9f39`.
The source-lock file remains
`f9bc2e977ec72488b8fa382ed0900a80f90000508f722714b5cb120380450948`
and staged path count remains zero. The complete replacement deterministic
matrix is green. This authorizes a fresh clean Attempt 010 only; it does not
accept J1-J10, ADR-0015, Phase 2C, or Product Owner sign-off.

## Repair 12 - Native Governance Escape

Live signed-Release J9 in Attempt 011 proved unique AX labels and keyboard
activation, then failed because Escape left the visible governance inspector
open. The bounded native repair routes `onExitCommand` through a tested
governance dismissal transition. Independent source and source-lock reviews
both returned `PASS`, `P0=0`, `P1=0`, `P2=0`.

**Ordered source digest**:
`c7a4112c3a0fe594541f94d911dfb21b3edbc854fd8cccbd9cec528bbda48b93`  
**Source-lock SHA-256**:
`0914a4db0049b8e6cbc128d08ed769518efba178ef13ce1a95ae8ae67dea7809`

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS; no race finding |
| Static analysis | `go vet ./...` | PASS; no output |
| Module integrity | `go mod tidy -diff` | PASS; no output |
| Locked Go formatting | `gofmt -d` | PASS; no output |
| Diff hygiene | `git diff --check` | PASS; no output |
| Swift standard | `swift test` | PASS: 122 XCTest, 1 intentional visual skip, 4 Swift Testing, 0 failures |
| Swift Thread Sanitizer | `swift test --sanitize=thread` | PASS: same counts, no TSAN finding |

The existing `QueueCommand<Input>` Swift Sendable warning is unchanged.

Fresh signed Release:
`/private/tmp/loom-repair12-release.JmztUM/Loom.app`

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `858f3519d3a8ad5cc7146aa1c561d65c4decbf368db35b9398c8883adc441abd`
- bundle directories and executable: `0700`
- non-executable files: `0600`
- symlinks: `0`
- `codesign --verify --deep --strict --verbose=4`: PASS

Post-matrix source and lock hashes remain exact and staged path count is zero.
This authorizes only a fresh clean J1-J10 replacement journey.

## Repair 13 - Native Action Labels

Attempt 012 proved Repair 12 Escape behavior but found three enabled native
product buttons without readable AX help. Repair 13 adds stable help labels to
those existing actions. Causal RED failed all three exact assertions; focused
GREEN passed the new regression plus the four shell state tests. Independent
source re-review, final status re-review, and source-lock review all returned
`PASS`, `P0=0`, `P1=0`, `P2=0`.

**Ordered source digest**:
`b009d12a1f6c6cce08108a7b2844b90c885daf7aa0bb5a3321ac5ff4a7f5a167`  
**Source-lock SHA-256**:
`e3bc7c34ff7b6605e3f849b8f15d0006b768b77c29519a821e7e46f1e9493fd6`

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS; no race finding |
| Static analysis | `go vet ./...` | PASS; no output |
| Module integrity | `go mod tidy -diff` | PASS; no output |
| Locked Go formatting | `gofmt -d` | PASS; no output |
| Diff hygiene | `git diff --check` | PASS; no output |
| Swift standard | `swift test` | PASS: 123 XCTest, 1 intentional visual skip, 4 Swift Testing, 0 failures |
| Swift Thread Sanitizer | `swift test --sanitize=thread` | PASS: same counts, no TSAN finding |

The existing `QueueCommand<Input>` Swift Sendable warning is unchanged.

Fresh signed Release:
`/private/tmp/loom-repair13-release.jC58Va/Loom.app`

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `4812bef1c28a2a547dca7626ea6ce1a4ed3c188f312cf53f9a118094247eca85`
- bundle directories and executable: `0700`
- non-executable files: `0600`
- symlinks: `0`
- `codesign --verify --deep --strict --verbose=4`: PASS

Direct AX enumeration of that signed Release in the offline empty state exposed
all three repaired actions together. Every enabled product `AXPress` button had
non-empty help, with `unlabeled_enabled_product_press=0`; standard close,
minimize, and full-screen controls were separately identified by native AX
subrole. The temporary app exited and global keyboard mode was restored to its
original absent state.

Post-matrix source digest, lock SHA, and staged count remain exact. This
authorizes only a fresh clean J1-J10 replacement journey; it does not accept a
Journey, WorkItem, ADR-0015, Phase 2C, or Product Owner sign-off.

## Repair 14 - Long-operation Response Grace

Attempt 013 exposed an equal 10-second client/server deadline race immediately
after daemon restart. Repair 14 keeps handler work at 10 seconds, gives server
response I/O 12 seconds, and gives Go/Swift product clients 15 seconds for only
`credential_verify`, `mission_execution`, and `chat_message`; ordinary methods
remain at five seconds. Explicit Go extended deadlines at or below 12 seconds
are rejected, and earlier context cancellation still wins. The Swift method
policy and lower-level exchange validator share the tested 15-second maximum.

Causal RED/GREEN, focused cancellation normal/race, adjacent Go normal/race,
real Go-to-Swift probes, Swift IPC normal/TSAN, and independent source review
passed. The first status review failed with two P2 wording findings; Re-review 2
closed those and found three remaining exact-gate wording P2s. Re-review 3 and
source-lock review passed with `P0=0`, `P1=0`, `P2=0`.

**Ordered source digest**:
`994d9c9e6595f114094f75607146b4e0ec049d60bf0b95b64b0b4abdf8da155e`  
**Source-lock SHA-256**:
`0f0ac771ca405d85f61f0a0d712e9d9c6b0341c1a86289cc3f90f254d9791f85`

| Gate | Command | Result |
|---|---|---|
| Full serial normal | `go test -timeout=12m -p 1 ./... -count=1` | PASS |
| Full serial race | `go test -race -timeout=12m -p 1 ./... -count=1` | PASS; no race finding |
| Static analysis | `go vet ./...` | PASS; no output |
| Module integrity | `go mod tidy -diff` | PASS; no output |
| Locked Go formatting | `gofmt -d` | PASS; no output |
| Diff hygiene | `git diff --check` | PASS; no output |
| Swift standard | `swift test` | PASS: 123 XCTest, 1 intentional visual skip, 4 Swift Testing, 0 failures |
| Swift Thread Sanitizer | `swift test --sanitize thread` | PASS: same counts, no TSAN finding |

The existing `QueueCommand<Input>` Swift Sendable warning is unchanged.

Fresh signed Release:
`/private/tmp/loom-repair14-release.wNXmGr/Loom.app`

- bundle identifier: `com.earendilworks.loom.local`
- architecture: `arm64`
- executable SHA-256:
  `46ce7677c38555bb302f5949d974d533aa60fd902ad0af47c778648e9c437ed2`
- bundle directories and executable: `0700`
- non-executable files: `0600`
- symlinks: `0`
- `codesign --verify --deep --strict --verbose=4`: PASS

Post-matrix source digest, lock SHA, and staged count remain exact. This
authorizes only a fresh clean J1-J10 replacement journey; it does not accept a
Journey, WorkItem, ADR-0015, Phase 2C, or Product Owner sign-off.

## Repair 15 Causal RED - Native Recent Action Help

The exact focused command
`swift test --filter LoomGraphiteViewTests/testWorkspaceShellNamesEveryLiveJourneyAction`
built successfully and failed with one expected assertion at
`LoomGraphiteViewTests.swift:88`. The new assertion required the existing
Recent-row helper to expose dynamic help; the pre-GREEN product source had only
the corresponding accessibility label. Result: `RED PASS`.

## Repair 15 First GREEN and Source Review 1

The exact causal test passed with 1 XCTest and 0 failures, and the complete
`LoomGraphiteViewTests` class passed with 7 XCTest and 0 failures. The first
GREEN directly interpolated `task.title` into help.

Source Review 1 returned `FAIL`, `P0=0`, `P1=0`, `P2=2`. Candidate status text
still named RED as pending, and raw saved-team names were not proven safe for
direct AX-help interpolation. The first GREEN is superseded; no source lock or
journey was authorized.

The pre-existing `QueueCommand<Input>` Swift Sendable warning was unchanged.

## Repair 15 Bounded-label Remediation

- Complete `LoomGraphiteViewTests`: PASS, 8 XCTest, 0 failures.
- Existing `SafeTextTests`: PASS, 4 XCTest, 0 failures.
- Hostile title proof covers CSI escape removal, the 48-character title bound,
  and exact safe output.
- Empty control/newline/tab input returns the fixed `Open recent task` fallback.
- The one helper output feeds both `.accessibilityLabel` and `.help`.

Source Re-review 2 remains required; this evidence does not authorize a source
lock or journey.

## Repair 15 Source Re-review 3

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`. The bounded shared-label remediation,
tests, contract, Candidate Boundary, current status, and deterministic evidence
are consistent. Final status-byte review and a replacement lock remain
required.

## Repair 15 Status Re-review 2

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`. The two Status Review 1 gate-text
findings are closed. Replacement source lock generation and independent lock
review remain required before the full matrix.

## Repair 15 Source Lock

Independent lock review: `PASS`, `P0=0`, `P1=0`, `P2=0`.

- Ordered 50-path digest:
  `6bcca52489a38f9bc5b80955172a0d229011b484ccd17ff4f59cdfa46fe41e8e`
- Lock SHA-256:
  `e2fb37fc858888f306781b3402bbde052bc474b3a08aff11e0a18cea72c9e89d`
- Superseded Repair 14 lock:
  `0f0ac771ca405d85f61f0a0d712e9d9c6b0341c1a86289cc3f90f254d9791f85`

The complete fresh lock-bound matrix is now authorized. No Journey or Phase
acceptance is authorized by this lock.

## Repair 15 Lock-bound Matrix Failure

The first full serial normal command
`go test -timeout=12m -p 1 ./... -count=1` failed only in
`TestProductDaemonProjectsControlledRuntimeOfflineBeforeIPCAndReplaysOnRestart`
at `product_daemon_test.go:8201`: the snapshot call returned
`local product unavailable`. Every other reported package passed.

The exact frozen test was then run 20 consecutive times and reproduced the
same failure once at the same line; 19 runs passed. This is a reproducible test
readiness race, not an erased or promoted transient. Repair 16 invalidates the
Repair 15 lock. No race matrix, Swift matrix, Release, or Journey is authorized
until the reviewed readiness repair passes a replacement lock and full matrix.

## Repair 16 Exact GREEN

The first GREEN passed `runner.server` at all call sites; two black-box
production-builder fixtures are statically typed as `daemonRunner`, so the
package failed compilation with two expected missing-field errors. No product
source or interface was expanded.

The bounded correction passes each exact runner to the test helper. The helper
fail-closed asserts `*productDaemonRunner`, awaits only its server's `Ready()`
channel with a two-second test timeout, then retains the existing socket type
check. Exact test result with `-count=100`: PASS in `5.520s`, zero failures.

Complete `cmd/loomd` normal/race and independent source review remain required.

## Repair 16 Daemon Package

- Complete `cmd/loomd` normal: PASS in `262.284s`.
- Complete `cmd/loomd` race: PASS in `227.471s`; no race finding.
- Exact count-100 readiness test remains PASS.

Independent source review, replacement lock, and the complete fresh matrix
remain required.

## Repair 16 Source Review 1

Result: `FAIL`, `P0=0`, `P1=0`, `P2=1`. Source mechanics and 16 generation
bindings were correct; Candidate Preflight still named completed GREEN as the
current gate. Product and test bytes remain unchanged. Source Re-review 2 is
required before a replacement lock.

## Repair 16 Source Re-review 2

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`. The stale gate text is closed and the
test-only generation readiness barrier is accepted for final status review and
replacement lock preparation.

## Repair 16 Status Re-review 2

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`. The Candidate Purpose now names Repair
16 and all gate bytes are consistent. Replacement source lock generation and
independent review remain required.

## Repair 16 Source Lock

Independent review: `PASS`, `P0=0`, `P1=0`, `P2=0`.

- Ordered 50-path digest:
  `1bd015cca2a4fd9e63353188b483f6b7dc92000362080b5cc81934a634e49de3`
- Lock SHA-256:
  `53f26fcf94ea727bbe75cee29f225eadf6ac6fb4a2164e4077633cb6e67ed60a`
- Superseded Repair 15 lock:
  `e2fb37fc858888f306781b3402bbde052bc474b3a08aff11e0a18cea72c9e89d`

The complete fresh lock-bound matrix is authorized; no Journey or acceptance
is authorized.

## Repair 16 Lock-bound Matrix And Release

The complete matrix was rerun against the independently reviewed Repair 16
lock. Every required command passed:

- `go test -timeout=12m -p 1 ./... -count=1`: PASS; `cmd/loomd` completed in
  `260.780s` and the complete command exited zero.
- `go test -race -timeout=12m -p 1 ./... -count=1`: PASS; `cmd/loomd`
  completed in `223.224s`, the complete command exited zero, and no race was
  reported.
- `go vet ./...`: PASS.
- `go mod tidy -diff`: PASS with no module-file drift.
- locked Go `gofmt -d`: PASS with no output.
- `git diff --check`: PASS.
- `swift test`: PASS, 124 XCTest with one intentional visual-audit skip and
  zero failures; all four Swift Testing tests passed.
- `swift test --sanitize thread`: PASS, 124 XCTest with the same intentional
  skip and zero failures; all four Swift Testing tests passed and Thread
  Sanitizer reported no race.

Fresh signed Release:
`/private/tmp/loom-repair16-release.w4cXQt/Loom.app`

- bundle identifier: `com.earendilworks.loom.local`
- executable: arm64 `LoomLocalApp`
- executable SHA-256:
  `dd254c85802228c79ce1707656ecbaa078737651f2b3575b415384a63cd86595`
- ordered bundle-file hash digest:
  `bc02c226f3d143518f98e0540ac6eafa1d307d08db8179cfb4258604747fa3e8`
- `codesign --verify --deep --strict --verbose=4`: PASS
- directories/executable: mode `0700`; non-executable files: mode `0600`
- symlinks: zero

Before the Release build and again after all bundle checks, the ordered
50-path digest was
`1bd015cca2a4fd9e63353188b483f6b7dc92000362080b5cc81934a634e49de3`,
the lock SHA-256 was
`53f26fcf94ea727bbe75cee29f225eadf6ac6fb4a2164e4077633cb6e67ed60a`,
and the staging inventory was zero. The lock-bound matrix and signed Release
authorize only a fresh clean Attempt 016; they do not accept a Journey, Phase,
ADR, or product result.

## Repair 17 Causal RED And Bounded GREEN

Attempts 016-018 are preserved as failed evidence. Attempt 018 passed J1-J6
and controlled Runtime-offline J7, then ordinary Runtime recovery failed in
`build_execution`. A read-only diagnostic build exposed the wrapped cause as
`mission execution conflict`: the projected TeamExecution aggregate was still
`running` while its only node was `ready_for_review`.

Before the production change,
`TestAuthoritativeMissionRestartKeepsReadyForReviewQuiescent` failed with that
exact conflict. The bounded GREEN classifies only a non-empty aggregate
`running` execution with at least one `ready_for_review` node and no node outside
`succeeded` or `ready_for_review` as quiescent. It performs no reconstruction
or runner call. Focused restart tests and the complete `internal/app` package
passed. A table-driven predicate test additionally covers the mixed
`succeeded`/`ready_for_review` topology and rejects pending, recovery-aggregate,
and empty topologies.

Focused daemon restart coverage also passed in `2.197s`:

- `TestProductMissionExecutionCompositionReconcilesExactJournalLineageBeforeIPC`
- `TestProductDaemonProjectsControlledRuntimeOfflineBeforeIPCAndReplaysOnRestart`
- `TestControlledRuntimeOfflineRecoversThroughOrdinaryPiObservation`

Supporting live proof used candidate binaries outside the repository:

- `loom` SHA-256:
  `a6618f316744dac681e4834dedf77d5a3cf8e43bb0a6c715836be215e9610d63`
- `loomd` SHA-256:
  `306af27c69276460cb06c9e2fcb48d19f3021ef5b0b11455bc15d0defcd0a023`
- ordinary recovery appended exactly one second
  `RuntimeInstanceStatusChanged` transition to `online`
- the Mission remained `ready_for_review`, Run count remained two, and Journal
  verification was `139/139/139` with integrity valid
- a subsequent normal restart preserved status and timeline byte-identically,
  appended no Event, and retained `139/139/139`

This mixed-source run is causal support only. Independent Repair 17 review, a
replacement source lock, complete matrix, fresh signed Release, and a clean
J1-J10 journey remain mandatory.

## Repair 17 Combined Review 1

Result: `FAIL`, `P0=0`, `P1=0`, `P2=2`.

The independent reviewer found no source defect and accepted the narrow
quiescent predicate, its projection semantics, causal restart test, predicate
table, existing active-resume/unknown-state coverage, complete `internal/app`,
and focused daemon checks. Two stale status statements blocked lock generation:
the Repair Amendment header still named the Repair 16 lock gate, and Candidate
Purpose still listed already-passed Repair 16 lock/matrix/Release work as
pending. Product and test bytes remain unchanged. Exact-byte re-review is
required.

## Repair 17 Exact-byte Re-review 2

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`.

Both stale status statements are closed. The reviewer searched the complete
current contract/Candidate/status/evidence bytes, recomputed the production and
test hashes, and found no source, test, authority, or gate inconsistency.
Final status-byte review is required before replacement lock generation.

## Repair 17 Final Status Review 3

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`.

The reviewer verified all current gate statements, both Repair 17 review
records, failed Attempts 016-018, unchanged production/test hashes, zero staged
paths, and the absence of premature matrix, Release, Journey, Phase, or ADR
authorization. Replacement source-lock generation and independent lock review
are authorized.

## Repair 17 Source Lock

Independent review: `PASS`, `P0=0`, `P1=0`, `P2=0`.

- Ordered 50-path digest:
  `e82334d38e561010eb32654062b9198b71cbb1b0d77a3ff15acdf0e183a03273`
- Lock SHA-256:
  `3968a970cc3d8743aa3c518b28c10f713086cef5699fb3d5a33c52229421a5ea`
- Superseded Repair 16 lock:
  `53f26fcf94ea727bbe75cee29f225eadf6ac6fb4a2164e4077633cb6e67ed60a`

The reviewer independently recomputed the JSON, all paths, digest, normative
hashes, Status Review 3 binding, 18 failure records, identity, exclusions, and
zero staging. The complete lock-bound matrix is authorized. Signed Release,
Journey, Phase, ADR, WorkItem, implementation Result, and Product Owner
acceptance remain unauthorized.

## Repair 17 Lock-bound Matrix Attempt 1 Failure

The first complete Go normal command failed and stops the matrix:

- command: `go test -timeout=12m -p 1 ./... -count=1`
- exit: `1`
- wall/user/system: `1751.30s` / `279.38s` / `80.32s`
- `cmd/loomd` timed out at `721.048s`; the active test was
  `TestProductMissionExecutionVerticalLoopbackClosesAuthorizedLineage`, blocked
  in `buildProductDaemonSwiftContractProbe` while waiting for an external Swift
  compile that had run for `4m48s`
- `internal/provider` failed
  `TestSystemCodexLoginControllerStartsExactSingletonAndJoinsOnClose` at
  `0.92s` with `EOF`
- the subsequent `internal/localipc` Swift-probe package completed in
  `606.651s`; all other reported packages passed

No race, vet, format, Swift, TSAN, or Release step is authorized from this
failed attempt. Exact reproductions and causal classification are required.

## Repair 18 Causal Classification

- Exact Swift journey test: PASS in `99.926s`, wall `105.26s`. The product path
  is live; the full-package failure is cumulative duplicate cold construction.
- Exact provider test at `-count=100`: FAIL in `178.057s`, wall `185.98s`, with
  two identical `EOF` failures while parsing the just-created PID fixture.
- No Swift compiler, Go test, daemon test-binary, or local IPC process remained
  after the failed full command and exact reproductions.

Repair 18 freezes process-local Swift probe construction/cleanup and complete
PID-fixture readiness only. Independent contract review is required before any
test edit.

## Repair 18 Contract Review 1

Result: `FAIL`, `P0=0`, `P1=0`, `P2=1`.

The causal record, proposed synchronization, cleanup, assertions, source
inventory, and production exclusions were accepted. Candidate Purpose still
said the boundary ended at Repair 17. It now says Repair 18; exact-byte contract
re-review is required before implementation.

## Repair 18 Contract Re-review 2

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`.

The stale Purpose statement is closed. The independent reviewer accepted the
causal RED, exact two-file test scope, process-owned build cleanup, strict PID
readiness, unchanged RED hashes, and zero staging. Bounded test-only
implementation is authorized.

## Repair 18 Exact GREEN

- provider PID-readiness exact test at `-count=100`: PASS in `67.207s`, wall
  `69.02s`, zero `EOF`
- both real Swift contract-probe client tests plus
  `TestProductDaemonSwiftContractProbeBuildIsShared` in one test process: PASS
  in `46.770s`, wall `51.58s`
- complete `cmd/loomd` normal: PASS in `50.382s`, wall `52.52s`
- complete `cmd/loomd` race: PASS in `47.593s`, wall `52.80s`, no race finding
- complete `internal/provider` normal: PASS in `2.430s`
- complete `internal/provider` race: PASS in `3.143s`, no race finding
- `cmd/loomd` compile-only, provider single-run, gofmt, and diff checks pass

The Swift probe is built once per test process in a private temporary root,
both callers receive the same executable path, and `TestMain` removes the root
after `m.Run()`. The provider fixture proceeds only after its PID bytes parse as
one positive decimal PID. Independent source review is required before a
replacement lock.

## Repair 18 Source Review 1

Result: `FAIL`, `P0=0`, `P1=0`, `P2=2`.

No source-mechanics defect was found. The reviewer accepted the single package
`TestMain`, `sync.Once` visibility, private-root cleanup, retained build output,
same-executable proof, strict PID parsing, exact GREEN, no leaked root, and zero
staging. Candidate Purpose still left Repair 17 lock review pending, and the
Repair 18 GREEN status was not the physical `docs/CURRENT.md` tail. Both status
issues are corrected; exact-byte source re-review is required.

## Repair 18 Source Re-review 2

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`.

Both status P2s are closed. The reviewer recomputed both implementation hashes,
accepted the process-local build and PID-readiness mechanics, found one current
EOF status record, no leaked Swift root, and zero staging. Final status review
is required before replacement lock preparation.

## Repair 18 Status Review 3

Result: `FAIL`, `P0=0`, `P1=0`, `P2=1`.

All source, evidence, inventory, provenance, cleanup, staging, and Phase/ADR
checks passed. Candidate Purpose still called completed Source Re-review 2 the
current gate. Purpose now records that PASS and names Status Re-review 4 as the
gate. Implementation bytes remain unchanged.

## Repair 18 Status Re-review 4

Result: `PASS`, `P0=0`, `P1=0`, `P2=0`.

The final stale Purpose gate is closed. Review counts, implementation hashes,
failed-matrix provenance, source inventory, cleanup, zero staging, and
authorization boundaries agree. Replacement source-lock generation and
independent review are authorized.

## Repair 18 Source-lock Reviews

Source-lock Review 1 returned `FAIL`, `P0=0`, `P1=0`, `P2=1`, but its sole
finding quoted a stale Amendment header that was not present in the reviewed
disk bytes. No source or lock byte changed. Exact Source-lock Re-review 2 first
re-read line 3 and its SHA, then returned `PASS`, `P0=0`, `P1=0`, `P2=0`.

- ordered 50-path digest:
  `2a79e59416049f26257c08bf5bd8e37ba263fea3e7e338f8a5f2d6eb8d49080e`
- lock SHA-256:
  `b02d23b7091b16f783ab8e503b65afa85a14a8875fcade1e55b8af4ab110d715`
- superseded Repair 17 lock:
  `3968a970cc3d8743aa3c518b28c10f713086cef5699fb3d5a33c52229421a5ea`

The complete fresh lock-bound matrix is authorized. No Release, Journey,
Result, WorkItem, Phase, ADR, or Product Owner acceptance is authorized.

## Repair 18 Complete Lock-bound Matrix And Release

The complete matrix was rerun against the independently reviewed Repair 18
lock. Every required command passed:

- `go test -timeout=12m -p 1 ./... -count=1`: PASS, wall `341.41s`;
  `cmd/loomd` completed in `52.273s`, `internal/localipc` in `138.434s`, and
  `internal/provider` in `2.995s`
- `go test -race -timeout=12m -p 1 ./... -count=1`: PASS, wall `689.82s`;
  `cmd/loomd` completed in `117.793s`, `internal/localipc` in `124.902s`, and
  `internal/provider` in `6.851s`; no race finding
- `go vet ./...`: PASS
- `go mod tidy -diff`: PASS with no module drift
- locked Go `gofmt -d`: PASS with no output after correcting one shell NUL
  encoding mistake that had not read any source file
- `git diff --check`: PASS
- `swift test`: PASS, wall `81.32s`; 124 XCTest, one intentional visual-audit
  skip, zero failures; all four Swift Testing tests passed
- `swift test --sanitize thread`: PASS, wall `133.46s`; 124 XCTest, the same
  intentional skip, zero failures; all four Swift Testing tests passed and
  Thread Sanitizer reported no race

Fresh signed Release:
`/private/tmp/loom-repair18-release.qhNqbB/Loom.app`

- build-script wall time: `26.68s`
- bundle identifier: `com.earendilworks.loom.local`
- executable: thin arm64 `LoomLocalApp`
- executable SHA-256:
  `dd254c85802228c79ce1707656ecbaa078737651f2b3575b415384a63cd86595`
- ordered three-file bundle digest:
  `59fb1dc4b3369848eb76f83cfd0abc0027b80a2e6c979748efb0e99074a8350e`
- ad-hoc `codesign --verify --deep --strict --verbose=4`: PASS
- directories and executable mode `0700`; non-executable files mode `0600`
- symlinks: zero

Before the Release build and after all checks, the ordered 50-path digest was
`2a79e59416049f26257c08bf5bd8e37ba263fea3e7e338f8a5f2d6eb8d49080e`,
the lock SHA was
`b02d23b7091b16f783ab8e503b65afa85a14a8875fcade1e55b8af4ab110d715`,
staging was zero, and no test/build process or Repair 18 Swift root remained.
The Release authorizes only a fresh clean Journey attempt; it does not accept
the Journey, Phase, ADR, Result, WorkItem, or product.

## Repair 19 Live J9 Failure, RED, and GREEN

Attempt 022 ran against the reviewed Repair 18 lock
`b02d23b7091b16f783ab8e503b65afa85a14a8875fcade1e55b8af4ab110d715`.
J1-J8 authority and restart boundaries and the J10 visual matrix passed, but
the signed-Release J9 action audit found four visually distinct Recent rows with
only two action labels: two `Open recent task Recent work` and two
`Open recent task Attempt 022 Review Team`. The attempt is preserved at
`journeys/repair-2026-08-08/attempt-022/` and excluded from acceptance.

The focused Repair 19 test first failed to compile because
`LoomRecentTaskActionLabel.make` still accepted only `for:` while the test
required `subtitle:` and `position:`. The bounded implementation then added a
sanitized 32-character subtitle and clamped one-based visible position while
retaining the sanitized 48-character title. The exact helper result feeds both
the accessibility label and help.

Focused GREEN:

- `swift test --package-path apps/macos --filter LoomGraphiteViewTests/testRecentTaskActionLabelSanitizesBoundsAndFallsBack`:
  PASS, one test, zero failures
- hostile title/subtitle content is sanitized and bounded
- a non-positive position falls back to one
- duplicate title/status inputs at positions three and four yield distinct
  labels without raw IDs

Independent Repair 19 contract/source/status review is required before a new
source lock. No complete matrix, signed Release, replacement live J9/J10,
Phase, ADR, Result, WorkItem, or Product Owner acceptance is authorized yet.

### Repair 19 Evidence Correction 1

Review 1 returned `P2=1` because the GREEN summary above says the duplicate
title/status inputs used positions three and four. The exact focused test uses
positions one and two. The assertion and implementation still prove that two
otherwise identical inputs with distinct positive visible positions produce
distinct labels. Product and test bytes are unchanged. This correction
supersedes only that position-number phrase; it does not convert Review 1 to a
PASS. Exact-byte re-review is required before replacement source-lock
generation or J1-J8 carry-forward authorization.

## Repair 19 Replacement Source Lock

Exact-byte Contract/Source/Status Re-review 2 returned `PASS` with
`P0=P1=P2=0` and authorized replacement lock generation. The generated lock
binds the same 50 source paths and supersedes Repair 18 lock
`b02d23b7091b16f783ab8e503b65afa85a14a8875fcade1e55b8af4ab110d715`.

- ordered 50-path digest:
  `f9ab656522b79c7ec2cc4394820dcb545d4d0509d676fff3303a2a7c76b64816`
- Repair 19 lock SHA-256:
  `89089a7485386a9b9e4feb3eb9005c7615a624e946567cb8971cf3a0a93355bb`
- Re-review 2 SHA-256:
  `34d1daeccf3b4dc8b9334cc0668972cdcad7545206d546dd797498d1ae6b40a1`
- failed Journey records preserved: 22
- staged paths: zero

Independent source-lock review is the current gate. No complete matrix, signed
Release, replacement J9/J10, Phase, ADR, Result, WorkItem, or Product Owner
acceptance is authorized before that PASS.

## Repair 19 Source-lock Review 1

Independent Source-lock Review 1 returned `PASS` with `P0=P1=P2=0`. It
recomputed the 50-path digest, lock SHA, normative hashes, review binding,
inventory cardinality/uniqueness/existence, all 22 failed records, repository
identity, and zero staging without a finding. The complete lock-bound matrix is
authorized. No signed Release, replacement J9/J10, Phase, ADR, Result,
WorkItem, or Product Owner acceptance is authorized yet.

## Repair 19 First Complete Matrix Failure

The independently reviewed Repair 19 lock remained byte-exact at ordered digest
`f9ab656522b79c7ec2cc4394820dcb545d4d0509d676fff3303a2a7c76b64816`
and lock SHA
`89089a7485386a9b9e4feb3eb9005c7615a624e946567cb8971cf3a0a93355bb`
when the complete Go normal matrix ran.

- command: `go test -timeout=12m -p 1 ./... -count=1`
- result: `FAIL`, wall `753.73s`
- sole failing package: `internal/app`
- sole failing test:
  `TestPhase1EngineeringDemoApprovalRestartReconnectAndRecovery`
- exact failure: competing coordinator 1 returned
  `Team execution incomplete`
- `cmd/loomd`: PASS in `376.505s`
- `internal/localipc`: PASS in `183.129s`
- all other packages: PASS or no test files
- raw log SHA-256:
  `7037604deb7924f87f807faab33a8796cbc39d7633d6736ba1eea4007f60afd0`

The exact failing test subsequently passed 100/100 in `167.398s`; this does not
erase the full-matrix failure. Repetition log SHA-256:
`a8369fe069b990ef121ee9627bd3f6ba1e4ddf5b3fbb2a8f5360fd7e2b2c23e8`.
No source byte changed during either run, the frozen digest and lock SHA
recomputed exactly, and staged paths remained zero. Repair 20 contract review
is required before any test implementation or replacement lock.

## Repair 20 RED And Bounded GREEN

Independent Repair 20 Contract Review 1 returned `PASS` with `P0=P1=P2=0` and
authorized the frozen test-only implementation.

Deterministic RED:

- command: `go test ./internal/app -run '^TestExpectedDemoRecoveryLoser$' -count=1`
- result: expected build failure
- exact error: `undefined: expectedDemoRecoveryLoser`

Bounded GREEN adds one closed test helper in
`internal/app/phase1_engineering_demo_test.go`. Existing conflict errors remain
expected losers. `ErrTeamExecutionIncomplete` is expected only with zero
executed node IDs; an incomplete dispatcher and unrelated errors remain
rejected. The existing exactly-one `main` winner and downstream exact Event,
call-count, evidence, and idempotent-restart assertions are unchanged.

- focused classification plus recovery scenario: PASS in `3.020s`
- recovery scenario count 100: PASS in `192.325s`, wall `198.75s`; log SHA-256
  `574567b0d752724ca97db2b35a9e894bfadd2b03154d4d0bd66555666b6033d0`
- complete `internal/app` normal: PASS in `6.451s`, wall `8.47s`; log SHA-256
  `441ae300add8b9584b9ff031af48c8bf54a91d104cc071a206434d629cef1403`
- complete `internal/app` race: PASS in `31.385s`, wall `33.63s`; no race
  finding; log SHA-256
  `f2051283742a6234b92c0326b3bbbce9ba1af62e7720f2652c7118941070d001`
- Repair 20 test SHA-256:
  `2a5805bc59248420a67700533a3e73492ea503d3be60148f119f0ddb6f58c487`
- unchanged production `internal/app/team_execution.go` SHA-256:
  `5f079ad96269849524a2eef145d97ea2ad6ddeab5664849076c7bbc7b86e3cde`
- `gofmt`, `git diff --check`, and zero staging: PASS

Independent Repair 20 source/status review is required before replacement lock
generation. No complete matrix, Release, J9/J10, Phase, ADR, Result, WorkItem,
or Product Owner acceptance is authorized.

## Repair 20 Replacement Source Lock

Independent Source/Status Review 1 returned `PASS` with `P0=P1=P2=0` and
authorized replacement lock generation. The lock now binds 51 source paths,
including the newly admitted Repair 20 test, and supersedes Repair 19 lock
`89089a7485386a9b9e4feb3eb9005c7615a624e946567cb8971cf3a0a93355bb`.

- ordered 51-path digest:
  `6fa2f269fefdaed41e60da0c64faf8f173924c427f23c1c5b0937693fab6cfa7`
- Repair 20 lock SHA-256:
  `7ad412e6b0122f3f37dfbbdaa3b5127f22d8b850963a1f1949fd14bf38ec2f1b`
- Source/Status Review 1 SHA-256:
  `36a27558df4079cc371ad09f491a3589a7ef82deda0362c5362ed3bc890772ea`
- candidate paths including the self-excluded lock: 52
- failed Journey records preserved: 22
- staged paths: zero

Independent Repair 20 source-lock review is the current gate. No complete
matrix, signed Release, J9/J10, Phase, ADR, Result, WorkItem, or Product Owner
acceptance is authorized before that PASS.

## Repair 20 Source-lock Review 1

Independent review returned `PASS` with `P0=P1=P2=0`. It recomputed the lock
SHA, ordered 51-path digest, 52-path Candidate parity, normative hashes, review
binding, path order/uniqueness/existence, all 22 failed records, repository
identity, `PARTIAL` status, and zero staging. The complete lock-bound matrix is
authorized. No signed Release, Journey, Phase, ADR, Result, WorkItem, or Product
Owner acceptance is authorized yet.

## Repair 20 Complete Lock-bound Matrix And Signed Release

The complete matrix ran against independently reviewed Repair 20 lock
`7ad412e6b0122f3f37dfbbdaa3b5127f22d8b850963a1f1949fd14bf38ec2f1b`
and ordered 51-path digest
`6fa2f269fefdaed41e60da0c64faf8f173924c427f23c1c5b0937693fab6cfa7`.
Every required command passed:

- `go test -timeout=12m -p 1 ./... -count=1`: PASS, wall `222.31s`;
  `cmd/loomd` `78.833s`, `internal/app` `8.310s`, `internal/localipc`
  `46.020s`; log SHA-256
  `2259bd2f5f031a0c14aeb0fcb5cff11755dce7a37896f8916ffe92c4a7b89ac0`
- `go test -race -timeout=12m -p 1 ./... -count=1`: PASS, wall
  `240.13s`; `cmd/loomd` `51.036s`, `internal/app` `21.661s`,
  `internal/localipc` `42.843s`; no race finding; log SHA-256
  `7f0309aa250accdba3913d03c5934506b1ef1a8b12fcb7d02685385c8877924e`
- `go vet ./...`: PASS, wall `0.31s`
- `go mod tidy -diff`: PASS with no module drift, wall `0.03s`
- locked Go `gofmt -d`: PASS with empty output after correcting one zsh
  newline-splitting harness invocation that wrote no source file
- `git diff --check`: PASS with empty output
- `swift test --package-path apps/macos`: PASS, wall `4.27s`; 124 XCTest,
  one intentional visual-audit skip, zero failures; all four Swift Testing
  tests passed; log SHA-256
  `16b877a52264ed69fce125511a18e87952b8c681b289d357d9e7ce337e2694f6`
- `swift test --package-path apps/macos --sanitize thread`: PASS, wall
  `22.64s`; the same 124 XCTest/one skip and four Swift Testing tests passed;
  Thread Sanitizer reported no race; log SHA-256
  `29f4a3623bc0f5f43db62c353cbb19b443ba94fd09e855782781fa96f3a6c17e`

The TSAN build repeated the existing Swift 6 future-compatibility warning for
the unconstrained `QueueCommand<Input: Encodable>` Sendable generic. It is not a
Swift 5 build error or race finding and is outside Repair 20's locked test-only
scope.

Fresh signed Release:
`/private/tmp/loom-repair20-release.VRnl2r/Loom.app`

- build wall: `11.22s`
- bundle identifier: `com.earendilworks.loom.local`
- executable architecture: thin `arm64`
- executable SHA-256:
  `a9b02d9e60afa96f8c79e18a73e8e8ca15bde61b1564ae0e1d5086dc7203bc4f`
- ordered three-file bundle digest:
  `cbef8c096504bb4c6264833c8c0afc0ed5414e94952a8076cf6a5dbeb78a6844`
- ad-hoc `codesign --verify --deep --strict --verbose=4`: PASS
- directories and executable mode `0700`; non-executable files mode `0600`
- symlinks: zero

The source digest and lock SHA recomputed exactly before and after the Release
build, and staged paths remained zero. This Release authorizes only replacement
live J9/J10 against carried Attempt 022 J1-J8 evidence. It does not accept the
Journey, Phase, ADR, Result, WorkItem, or product.
