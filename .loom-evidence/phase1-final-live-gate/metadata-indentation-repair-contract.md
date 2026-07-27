# Final Live Gate Metadata Indentation Repair

Status: IMPLEMENTATION REVIEW `PASS` — LOCAL ATOMIC REPAIR COMMIT IS THE CURRENT GATE

- Amendment: `PHASE1-FINAL-LIVE-METADATA-INDENTATION-1`
- Risk: `HIGH`
- Baseline: `782942374b1ae783e9b50f64cb056d8650c50c6f`
- Date: `2026-07-27`
- Depends on:
  - Final Live Gate Compatibility Amendment Implementation Review 5 `PASS`;
  - Source Provenance Amendment Contract Review `PASS`;
  - installed exact model/source binding and sanitized pre-live manifest `PASS`;
  - the first controlled live canary's reviewed
    `FAIL — HUMAN_REQUIRED — NO RETRY` evidence; and
  - explicit user authorization on `2026-07-27` for this repair and exactly one
    replacement controlled live canary.

## Exact reason

Installed locked Pi `0.82.1` emits its three-line no-model diagnostic in this
shape:

```text
No models available. Use /login to log into a provider via OAuth or API key. See:
  <absolute-common-parent>/providers.md
  <absolute-common-parent>/models.md
```

The accepted parser modeled the two documentation paths without indentation
and rejects all boundary whitespace. The first authorized canary therefore
failed closed in installed Runtime discovery before llama-server, Pi RPC,
WorkItem/Run/Grant creation, model invocation, Frames, Journal terminal facts,
or Evidence acceptance.

## Owned files

Product and test ownership is limited to:

- `internal/runtime/pi_probe.go`
- `internal/runtime/pi_probe_test.go`

Governance evidence ownership is limited to:

- this contract;
- `metadata-indentation-repair-contract-review.md`;
- `metadata-indentation-repair-implementation-review.md`;
- a sanitized RED/GREEN/check record for this amendment; and
- after the replacement canary, bounded status/evidence synchronization within
  `.loom-evidence/phase1-final-live-gate/**` and the private materialization
  status.

No other Go file, test, dependency, ADR, policy, validator, StateWriter,
Journal, Projection, Bridge, Supervisor, Grant, Evidence, CLI, daemon, model,
Runtime installation, credential, or user-owned dirty file is owned.

## Frozen behavior

The Pi `0.82.1` three-line no-model diagnostic is accepted only when:

1. line 1 is byte-exact and has no leading or trailing whitespace;
2. lines 2 and 3 each begin with exactly two ASCII space bytes (`0x20 0x20`);
3. exactly those two prefix bytes are removed before path validation;
4. the remaining paths have no leading/trailing whitespace or control
   characters;
5. the remaining paths are clean absolute paths with ordered basenames
   `providers.md` then `models.md`;
6. each path's immediate parent basename is exactly `docs`, preserving the
   accepted `validPiMetadataDocPath` guard;
7. both paths have the same non-empty, non-dot parent;
8. there are exactly three physical lines, with at most one final LF; and
9. the paths remain validation-only inputs and are discarded.

The parser must reject with `ErrInvalidPiMetadataOutput` and no model rows for:

- zero, one, or three-or-more ASCII spaces before either path;
- mixed indentation between the two path lines;
- Tab, vertical Tab, form feed, carriage return, non-breaking space, or any
  other control/non-ASCII whitespace used as indentation;
- any trailing whitespace on any line;
- blank/extra lines or more than one final LF;
- relative, unclean, different-parent, non-`docs`-parent, wrong-basename,
  swapped, or extra paths;
- invalid UTF-8, NUL, or embedded control characters; and
- any otherwise malformed metadata.

The existing exact one-line legacy no-model diagnostic remains accepted.
The table-form model parser and every existing size, stderr, UTF-8, row-count,
token, duplicate, and non-disclosure boundary remain unchanged.

## Trust boundary

This repair does not:

- call `strings.TrimSpace` or equivalent on the complete path and then accept a
  broader whitespace class;
- accept arbitrary indentation, a variable prefix, or unindented Pi `0.82.1`
  documentation paths;
- remove or weaken the exact immediate-parent basename `docs` guard;
- expose, persist, log, or return either absolute documentation path;
- change the installed Pi, llama.cpp, model, source lock, mirror trust,
  credentials, environment policy, Runtime runner, Bridge, or live harness;
- add retry, fallback, another model/source, remote Provider traffic, daemon
  activation, or autonomous execution; or
- authorize more than one replacement controlled canary.

The Event Journal, StateWriter, Projection, policy, Grant, Evidence, and final
user sign-off authorities remain unchanged.

## Mandatory RED

Before product code changes, `internal/runtime/pi_probe_test.go` must encode the
complete exact-prefix matrix. At minimum it must prove:

- the installed Pi shape with exactly two spaces on both path lines is accepted
  with and without one final LF; and
- 0/1/3 spaces, mixed indentation, Tab/non-ASCII indentation, trailing
  whitespace, non-`docs` parents, wrong paths, extra lines, and control
  characters remain rejected.

The focused command must fail against baseline production code because the
exact two-space installed Pi shape returns `ErrInvalidPiMetadataOutput`.

```text
go test ./internal/runtime \
  -run '^TestPi0821NoModelsDiagnosticCompatibility$' \
  -count=1
```

RED evidence must record the exact failing assertion without any private
absolute installed path.

## Minimal GREEN

Production code may remove exactly the two required ASCII prefix bytes from
lines 2 and 3 only after proving their exact presence. All existing strict path
validation then applies to the de-indented values. No general normalization or
new parser mode is allowed.

## Verification matrix

Before Implementation Review:

```text
go test ./internal/runtime \
  -run '^TestPi0821NoModelsDiagnosticCompatibility$' \
  -count=1

go test ./internal/runtime -count=1

go test -race ./internal/runtime \
  -run '^TestPi0821NoModelsDiagnosticCompatibility$' \
  -count=50

go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go mod verify
gofmt -d internal/runtime/pi_probe.go internal/runtime/pi_probe_test.go
git diff --check
```

Scope audit must prove:

- product changes are limited to the two owned runtime files;
- governance changes are limited to the owned final-live evidence;
- no dependency or module file changed;
- excluded user-owned dirt remains unstaged; and
- no process, network, credential, model, or live canary was used during
  RED/GREEN/review.

## Review and commit gate

1. Fresh Contract Review `PASS` is required before the test edit.
2. Mandatory RED must precede production code changes.
3. The complete verification matrix and fresh independent Implementation
   Review must return `PASS`.
4. Only then may one narrow local atomic repair commit be created from the two
   owned product files and this amendment's reviewed evidence. Existing
   uncommitted provenance/failed-canary evidence and excluded user dirt remain
   unstaged.
5. The replacement canary must execute the exact committed repair.

## Replacement controlled live canary

After the repair commit, the Controller must freshly verify:

- exact installed Pi, llama.cpp, and model bindings against the frozen source
  lock and sanitized manifest;
- private-root/file ownership, modes, regular-file/non-symlink status, and
  containment;
- model byte size, GGUF v3 header, and full SHA-256;
- no listener on the frozen loopback port and no residual final-live process;
  and
- the exact opt-in environment and existing live test command.

Exactly one replacement invocation is authorized:

```text
go test ./internal/app \
  -run '^TestFinalLiveGatePiRPCOfflineModel$' \
  -count=1 -v
```

The existing test may start only the controlled loopback llama.cpp server and
one Pi RPC execution with the frozen offline model. Any failure stops
immediately. There is no second replacement, retry, fallback, alternate source,
alternate model, parser expansion, or hidden rerun.

Successful replacement evidence still requires fresh independent evidence and
scope review. It cannot supply the user's final Phase 1 sign-off.

VERDICT: PASS
