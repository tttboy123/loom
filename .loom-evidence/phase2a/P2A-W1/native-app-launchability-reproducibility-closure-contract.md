# P2A-W1 Native App Launchability and Reproducibility Closure Contract

**Date**: `2026-07-28`
**Status**: `FROZEN — FRESH INDEPENDENT CONTRACT RE-REVIEW 2 PASS`
**Parent**: frozen P2A-W1 Native App Host Contract Revision
**WorkItem count**: unchanged; this remains P2A-W1
**Live authority**: none

## 1. Vertical repair boundary

The consumed native-window canary proved that the exact reviewed Candidate
daemon and IPC read path worked, but both native app invocations aborted in
dyld before SwiftUI entrypoint execution:

```text
DYLD: missing LC_UUID load command
```

The root cause is the combined reviewed behavior:

1. the builder links with `-no_uuid`;
2. the build fixture requires the application UUID to be absent.

This contract repairs the complete native packaging acceptance boundary:
Mach-O launchability, reproducible linker identity, final signature identity,
private-bundle process survival, and the exact rollback result ledger.

It does not create Reopen 5, P2A-W4, another daemon/app architecture, or a thin
writer/adapter WorkItem. No SwiftUI feature, IPC schema, daemon, Journal,
Projection, Provider, Runtime, authorization, or execution behavior is
reopened.

## 2. Exact owned files

Product/test modifications are limited to:

```text
scripts/build-loom-local-app.sh
scripts/test-build-loom-local-app.sh
```

Governance/evidence modifications are limited to:

```text
docs/CURRENT.md
.loom-evidence/phase2a/P2A-W1/native-app-launchability-*.md
```

The existing canary and Result Review files remain append-only evidence. They
may be referenced but not rewritten to claim product success.

Any need to modify Swift, Go, plist, installer, daemon, IPC, credential,
authority, or another WorkItem stops `HUMAN_REQUIRED`; it does not silently
expand this contract.

## 3. Linker and signature decision

The builder must:

1. replace both `-Xlinker -no_uuid` arguments with
   `-Xlinker -reproducible`;
2. retain `-Xlinker -no_adhoc_codesign` so intermediate Swift linking does not
   introduce an uncontrolled signature;
3. retain `strip -S` to remove non-runtime debug and N_OSO path/timestamp
   surfaces;
4. perform exactly one final bundle ad-hoc signature with
   `--timestamp=none`;
5. reject a missing, zero, random-per-build, or multi-architecture-mismatched
   `LC_UUID`;
6. reject `-no_uuid` and `-random_uuid` in the production builder.

The final arm64 UUID must be the linker's content-derived identity. The build
must not patch Mach-O bytes manually, invent an independent UUID store, or
normalize away a runtime-required load command.

## 4. Reproducibility definition

Two complete builds, each preceded by `swift package reset`, must produce:

- one non-empty arm64 `LC_UUID` in each executable;
- the same UUID value across both builds;
- byte-identical signed `LoomLocalApp` executables;
- byte-identical `Info.plist` and CodeResources;
- the same complete bundle file set, modes, and canonical manifest digest;
- strict-valid ad-hoc signatures;
- no symlink, LC_UUID omission, N_OSO path/timestamp surface, secret surface,
  or unreviewed dependency.

Canonical manifest bytes remain:

```text
raw-relative-path sort
relative_path NUL stat-%Sp-mode NUL lowercase-sha256 LF
```

followed by lowercase SHA-256.

Reproducibility is not weakened to a source-only or normalized comparison.
The two final signed bundles remain byte-identical.

## 5. Private launchability smoke

The build fixture must execute the exact first private built bundle's
`Contents/MacOS/LoomLocalApp` without installing it and prove:

1. all static UUID, architecture, and signature gates pass before spawn;
2. a before-snapshot of `~/Library/Logs/DiagnosticReports` records only
   filenames and hashes for existing `LoomLocalApp-*.ips` reports;
3. dyld keeps the process alive for a bounded one-second observation window;
4. the process is the exact private bundle executable;
5. the process is then terminated by the fixture and reaches quiescence;
6. the after-snapshot contains no new `LoomLocalApp-*.ips` report;
7. the fixture emits no process row, environment value, user path, IPC body,
   Provider marker value, or UI content;
8. failure or signal cleanup terminates only the captured exact child PID and
   removes only the fixture-owned private root.

If a new crash report appears despite the pre-spawn gates, the fixture fails,
leaves that user-owned report untouched, records only its name and hash, and
stops `HUMAN_REQUIRED`. It must not delete or rewrite DiagnosticReports.

This smoke may render a transient local window but performs no Computer Use
action, app installation, Candidate daemon bootstrap/restart, Provider/Runtime
request, Journal write, Team mutation, or live canary. It is deterministic
launch verification, not ordinary-user acceptance.

## 6. Mandatory RED

Before changing the builder, the repaired build fixture must fail because:

- the current executable has no `LC_UUID`.

RED must reject the missing UUID before spawning the executable. It must
snapshot the existing DiagnosticReports inventory before and after, prove no
new report appeared, and record the command, nonzero result, closed failure
reason, unchanged product source, no installed app/run/socket/launcher,
running original observer, unchanged Journal hash/Event count, and empty
staging.

## 7. Deterministic GREEN

After the two owned product/test files are repaired:

```text
scripts/test-build-loom-local-app.sh
cd apps/macos && swift test
cd apps/macos && swift build -c release
scripts/test-install-loom-local-app.sh
scripts/test-install-loom-local-product.sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go mod verify
git diff --check
git diff --cached --check
```

GREEN must record:

- both non-empty identical UUIDs;
- both exact signed executable and canonical bundle manifest hashes;
- private process survival and exact quiescence;
- strict signature, architecture, bundle ID, ownership/mode, symlink,
  security-scan, and rollback fixtures;
- running original observer and zero target-process Provider markers;
- unchanged SQLite hash, integrity, Event count, and canonical view;
- absent installed Candidate app/run/socket/launcher/native process;
- empty staging.

## 8. Review and live boundary

Fresh independent Contract Review must pass before RED or implementation.
Fresh independent Implementation Review must pass after the complete GREEN
matrix.

Neither Review authorizes another native-window canary. The prior native live
allowance remains `0`; its `FAIL — ROLLED_BACK — HUMAN_REQUIRED` result remains
historically true.

Any future installed native-window canary requires new explicit post-Review
governance that reconciles the consumed no-retry boundary. No activation is
embedded or implied by this repair contract.

## 9. Exit

This closure is complete only when:

1. Contract Review is `PASS`;
2. mandatory RED is preserved;
3. deterministic GREEN and the complete matrix are `PASS`;
4. fresh independent Implementation Review is `PASS`;
5. original resident state remains exact and staging remains empty.

At that point P2A-W1 is still not product-accepted because no replacement live
authority exists. P2A-W2 remains locked until a later explicitly governed
native-window live `PASS`, fresh Result-Evidence Review `PASS`, and the atomic
P2A-W1 commit.

## 10. Contract Review Repair 1

Contract Review 1 returned `FAIL` because the proposed RED would execute the
known no-UUID binary and could generate an unowned macOS crash report outside
the private fixture root.

This repair:

1. makes missing-UUID RED a pre-spawn structural failure;
2. requires unchanged DiagnosticReports inventory across RED;
3. permits the one-second process smoke only after UUID, architecture, and
   signature gates pass in GREEN;
4. requires before/after name-and-hash inventory around that smoke;
5. forbids deleting or rewriting an unexpected user crash report and stops
   `HUMAN_REQUIRED` if one appears.

No owned file, product behavior, linker decision, live authority, WorkItem
count, or allowance accounting changed. Fresh independent Contract Re-review
is required before RED or implementation.
