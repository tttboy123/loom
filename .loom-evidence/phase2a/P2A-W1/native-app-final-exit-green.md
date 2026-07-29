# P2A-W1 Native App Final Exit Closure GREEN

**Date**: 2026-07-29  
**Result**: `GREEN — COMPLETE DETERMINISTIC MATRIX PASS`  
**Live authority**: none  
**Candidate bootstrap count**: `0`  
**Native app launch count**: `0`

## Implemented boundary

The only implementation file is the governed evidence harness:

```text
.loom-evidence/phase2a/P2A-W1/native-app-final-exit-transaction.sh
```

Production mode contains the exact reviewed Candidate/original paths, hashes,
LC_UUID, bundle manifest, service identity, Journal/view, crash inventory,
run-root, rollback, native handoff, restart, and preserve gates. It:

- rejects all arguments and ambient path/identity overrides;
- remains hard locked while the post-Review audit file is absent;
- creates and validates the exact owned mode-`0700` run root before installer,
  plist, bootout, or bootstrap mutation;
- sets `initial_calls=1` and `consumed=1` immediately after successful
  `launchctl bootstrap`, before readiness or socket waits;
- exposes no path from readiness failure back to bootstrap;
- permits one later restart only after the `ui_verified` transition;
- preserves only after `result_review_pass`;
- rolls back on every failure, EOF, or signal after mutation;
- restores exact original bytes, modes, provenance values, service, Journal,
  crash inventory, paths, and staging;
- emits only closed result/count lines and suppresses raw command errors.

Production execution was not activated.

## Behavioral fixture

Fixture mode is available only below a canonical private
`/private/tmp/loom-final-exit-test.*` root with an exact sentinel. It uses
internal closed fake service-manager functions and a real private AF_UNIX
socket. It executes no fixture-provided code and cannot address live product
paths.

The passing matrix proves:

- pre-bootstrap failure: initial calls `0`, consumed `0`, rollback `1`;
- readiness failure after successful bootstrap: initial calls `1`, consumed
  `1`, rollback `1`, restart `0`, no retry;
- success fixture: one initial bootstrap, one classified lifecycle restart,
  no rollback;
- absent sentinel, non-private mode, symlinked root/control/run directory,
  non-canonical path, unknown scenario/argument, foreign owner when supported,
  and every closed ambient production override fail closed;
- output is bounded and credential-value negative;
- fixture roots and sockets are removed after every case.

The initial RED test used one too-many parent traversal when resolving
`repo_root`. The missing harness still made that RED semantically true, but the
path defect became visible once the harness existed. GREEN corrected it to the
actual repository root before accepting any passing result.

The first fixture GREEN used executable fake commands supplied inside the
fixture. Security review rejected that injection surface even though the test
owned those scripts. The final implementation moved fake bootstrap/bootout
inside the harness and reran the complete fixture successfully.

Pre-Review self-audit also moved final private-root validation before disabling
the rollback trap. A damaged/replaced transaction root can therefore never
leave the Candidate preserved through a failed final cleanup check. Syntax,
fixture, and production-lock checks passed again after that ordering repair.

The same audit found that the second historical crash digest had been
transcribed with one extra trailing character. Production would have rejected
it before mutation, but the typo would have made the gate unusable. The value
was corrected to the exact current 64-character SHA-256, and the fixture now
locks every frozen Candidate/original/Journal/view/crash digest and LC_UUID.
Syntax, constant-shape, fixture, and diff checks passed again.

## Exact verification

### Harness and packaging

```text
sh -n native-app-final-exit-transaction.sh
PASS

sh -n native-app-final-exit-transaction-test.sh
PASS

native-app-final-exit-transaction-test.sh
final exit transaction fixture PASS

production mode without post-Review audit
exit 20
FINAL_EXIT_RESULT result=preflight_failure initial_bootstrap_calls=0 consumed=0 rollback_count=0 restart_count=0

scripts/test-build-loom-local-app.sh
native app build fixture PASS

scripts/test-install-loom-local-app.sh
native app installer fixture PASS

scripts/test-install-loom-local-product.sh
installer fixture PASS
```

The fixture also passed from an `env -i` command context with only the reviewed
system `PATH`.

### Swift

```text
swift test --package-path apps/macos
17 tests, 0 failures

swift build -c release --package-path apps/macos
PASS
```

The Swift package cache was removed with `swift package reset` after
verification.

### Go and repository

```text
go test -count=1 ./...
PASS

go test -race -count=1 ./...
PASS

go vet ./...
PASS

go mod verify
all modules verified

gofmt -l <all Go files>
no output

git diff --check
PASS

git diff --cached --check
PASS
```

No full-suite retry was needed. `shellcheck` is not installed; both governed
shell files pass the platform `/bin/sh` parser and the behavioral fixture.

## Candidate identity

The retained private reviewed Candidate remains exact:

| Surface | Result |
|---|---|
| `loom` | `3ad7deb7...19f` |
| `loomd` | `ff5a00dc...357` |
| native executable | `f473684c...06b` |
| arm64 LC_UUID | `CE91F84E-4333-35DB-B493-88FADCBC6EC1` |
| canonical app manifest | `e29c1b6c...cbd` |
| bundle signature | strict valid |
| Candidate root/binaries/app modes | `0700`, uid `501` |

## External state

After the complete matrix:

- original `loom`: `60c90ada...698`, mode `0755`, uid `501`;
- original `loomd`: `e5ab283c...14a`, mode `0755`, uid `501`;
- wrapper: `ff556602...3e4`, mode `0700`, uid `501`;
- plist: `2a6dc3e1...1f4`, mode `0600`, uid `501`;
- SQLite: `91ae07e0...8a4`, mode `0600`, uid `501`;
- observer: running from exact `loomd-clean`;
- original daemon/wrapper provenance attribute names: present;
- target-process five-marker count: `0`;
- SQLite integrity/Event count: `ok` / `1`;
- canonical view digest: `6f43ee12...e05`;
- crash inventory: exact two historical reports;
- Candidate app/run/socket/launcher/native process and Swift cache: absent;
- Git staging: empty.

A first naive evidence security scan searched for the literal negative-test
token `sk-` and flagged the fixture's own deny pattern. It was corrected to
value-shaped credential expressions; the final closure files contain no
credential value or Provider endpoint value.

## Gate

Fresh independent Implementation Review is required next. GREEN grants no
install, bootstrap, restart, Computer Use action, native canary, commit, or
P2A-W2 work. Current live allowance remains `0`.

## Implementation Review repair 1

The first independent Implementation Review returned `FAIL` with two bounded
harness findings:

1. fixture counter writes followed a pre-existing symlink and could mutate a
   file outside the canonical fixture root;
2. production preflight required `Loom.command` to be absent but did not reject
   a pre-existing `Loom.command.previous`, while rollback removes that path.

The repair remains inside the two frozen harness files:

- fixture counters are now held in process memory and materialized with a
  private same-directory temporary file plus atomic replacement; any existing
  non-regular, symlinked, foreign-owned, or non-`0600` counter is rejected;
- the fixture has explicit bootstrap, rollback, and restart counter-symlink
  cases and proves the outside-file SHA-256 remains unchanged;
- production preflight now requires both `Loom.command` and
  `Loom.command.previous` to be absent, including a symlink spelling.

Post-repair syntax, the normal fixture, all three counter-symlink cases, an
`env -i` fixture invocation, diff checks, and the production live lock pass.
Production remains exactly:

```text
exit 20
FINAL_EXIT_RESULT result=preflight_failure initial_bootstrap_calls=0 consumed=0 rollback_count=0 restart_count=0
```

No live service, installed path, Journal, app, Candidate, or staging state was
mutated. Fresh independent re-review is required; allowance remains `0`.

## Implementation Review repair 2

The first Repair 1 re-review confirmed both prior fixes, but remained `FAIL`
on two additional harness boundaries:

1. inherited interpreter and Git control variables could affect direct
   `python3` and `git` commands; a private `PYTHONPATH/sitecustomize.py`
   reproduction executed before the prior fixture completed;
2. production exact-absence checks for `Loom.command`,
   `loom.previous`, and `loomd.previous` used only `! -e`, so a dangling
   symlink could be mistaken for an absent path.

Repair 2 establishes a clean process environment before fixture or production
dispatch. The first process uses only shell parameter checks, then re-executes
the same fixed harness through `/usr/bin/env -i` with the exact system `PATH`
and `LC_ALL=C`. The re-executed process accepts only the internal clean marker
plus shell-generated `PWD`, `SHLVL`, and `_`; it removes the marker before any
fixture or production command. A forged marker accompanied by any injected
environment name fails closed.

The fixture now proves all of these inherited controls are inert:

```text
BASH_ENV
ENV
PYTHONPATH
GIT_INDEX_FILE
LOOM_ROLLBACK_TEST_SIGNAL_STEP
```

The private `sitecustomize.py` and shell-init markers remain absent, the ready
fixture still completes with the exact closed result, and a forged clean
marker plus `PYTHONPATH` is rejected. Production preflight and rollback now
require both `! -e` and `! -L` for the launcher and both binary backup paths,
as already done for the launcher backup.

Both syntax checks, normal fixture, clean outer `env -i` fixture, exact
production exit-`20` lock, and both diff checks pass after Repair 2. No live
mutation occurred. Fresh independent re-review remains mandatory and live
allowance remains `0`.

## Fresh post-repair complete matrix

After Repair 2, the complete required matrix was rerun from the final harness
bytes:

```text
native app build fixture
PASS

native app installer fixture
PASS

local product installer fixture
PASS

Swift tests
17 tests, 0 failures

Swift release build
PASS

go test -count=1 ./...
PASS

go test -race -count=1 ./...
PASS

go vet ./...
PASS

go mod verify
all modules verified

gofmt -l
no output

git diff --check
PASS

git diff --cached --check
PASS

secret value-shaped scan
PASS
```

The Swift package cache was reset afterward. The exact original identities,
running observer, marker count `0`, Journal bytes/integrity/Event count,
Candidate manifest/signature, two-report crash inventory, absent live
Candidate paths, and empty staging were reproduced again.

Fresh independent Implementation re-review then returned `PASS` with no
findings. This does not create live authority; the post-Review audit allowance
is `0`.
