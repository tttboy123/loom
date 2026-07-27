# Final Live Gate Pi RPC Schema and Fresh Attempt Isolation Verification

- Contract: `PHASE1-FINAL-LIVE-PI-RPC-FRESH-ATTEMPT-1`
- Baseline: `b34d8da635c6037c6f3658d580c1dc75bf541f63`
- Verification date: `2026-07-27`
- Live execution used: `NO`
- Additional canary consumed: `NO`

## Contract gate

Fresh Contract Review 1 returned `FAIL` because the settings publication
language did not explicitly forbid an overwrite-capable temp-file-plus-rename
implementation.

Contract Repair 1 froze:

- direct final-leaf `O_WRONLY|O_CREATE|O_EXCL` creation at `0600`;
- write, `fsync`, close, and exact revalidation before process start;
- rejection without mutation when any object already occupies the leaf; and
- no rename, truncate, unlink, replacement, or reuse.

Fresh independent Contract Repair Review 2B returned `PASS`. Mandatory RED
started only after that verdict.

## Mandatory RED

The runtime focused command exited `1` before production changes. It proved
that the baseline:

- rejected valid Pi `0.82.1` single-text-block arrays;
- accepted the obsolete string content shape;
- did not create the exact private no-retry/no-compaction settings; and
- did not reject every pre-existing settings object before process start.

The app focused command exited `1` at compile time only because the
contract-frozen fresh-attempt and exclusive-SQLite helpers did not exist.

No RED output contained a prompt, credential, raw RPC record, Grant, model
output, or private materialization path.

## Minimal repair

The bounded implementation:

- accepts only one exact Pi `0.82.1` `{type:"text",text:<prompt>}` content
  block and rejects the legacy string, extra or duplicate fields, other block
  types, prompt drift, malformed JSON, invalid UTF-8, and invalid timestamps;
- exclusively creates the exact settings bytes with one final line feed,
  `0600`, `fsync`, close, exact-byte revalidation, and no replacement of any
  pre-existing object;
- explicitly rejects JSON `null` in the shared non-negative-number validator;
- creates a new direct-child `controlled-canary-additional-*` directory at
  exact mode `0700`;
- exclusively pre-creates `canary.sqlite` at exact mode `0600` before
  `sql.Open`; and
- routes only the additional canary to the independent sanitized manifest
  filename.

An initial focused runtime GREEN attempt exposed that Go unmarshalling JSON
`null` into `float64` yields zero without an error. The bounded numeric
validator repair added explicit `null` rejection before the focused matrix was
rerun.

## Verification matrix

All required focused and package checks passed:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821UserMessageCompatibility|BridgeCreatesExactNoRetrySettingsBeforePrompt)$' \
  -count=1
PASS: ok loom-pi-rebuild/internal/runtime/piadapter 0.808s

go test ./internal/app \
  -run '^TestFinalLive(FreshAttemptIsolationAndPrivateSQLite|AdditionalManifestDoesNotAliasPriorEvidence)$' \
  -count=1
PASS: ok loom-pi-rebuild/internal/app 0.594s

go test ./internal/runtime/piadapter ./internal/app -count=1
PASS: piadapter 26.665s; app 7.965s

go test -race ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821UserMessageCompatibility|BridgeCreatesExactNoRetrySettingsBeforePrompt)$' \
  -count=30
PASS: ok loom-pi-rebuild/internal/runtime/piadapter 10.222s

go test -race ./internal/app \
  -run '^TestFinalLive(FreshAttemptIsolationAndPrivateSQLite|AdditionalManifestDoesNotAliasPriorEvidence)$' \
  -count=30
PASS: ok loom-pi-rebuild/internal/app 1.805s

go test ./... -count=1
PASS
```

### Whole-repository race evidence

The first exact `go test -race ./... -count=1` invocation exited `1` after the
test framework's ten-minute timeout. The only failing test was the pre-existing
`TestLocalRuntimeObservationDaemonRejectsConcurrentRunAndActiveClose`; its
stack was blocked in `syscall.forkExec` while starting its metadata fixture.
The three owned files were not on that stack.

The exact failing test was immediately isolated without a product change:

```text
go test -race ./internal/app \
  -run '^TestLocalRuntimeObservationDaemonRejectsConcurrentRunAndActiveClose$' \
  -count=1 -v -timeout=2m
PASS: 0.47s
```

The original, undiluted whole-repository command was then rerun and passed:

```text
go test -race ./... -count=1
PASS: internal/app 27.157s; internal/runtime/piadapter 40.574s; all packages
```

This rerun did not invoke a Runtime, model, Provider, network request, or live
canary and did not consume the one additional-canary authorization.

The remaining required checks passed:

```text
go vet ./...
PASS

go mod verify
PASS: all modules verified

gofmt -d \
  internal/runtime/piadapter/rpc_bridge_adapter.go \
  internal/runtime/piadapter/rpc_bridge_adapter_test.go \
  internal/app/final_live_gate_live_test.go
PASS: empty output

git diff --check
PASS

GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime/piadapter ./internal/app
PASS
```

## Scope and trust audit

Product and harness edits are limited to:

- `internal/runtime/piadapter/rpc_bridge_adapter.go`
- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
- `internal/app/final_live_gate_live_test.go`

No module, dependency, Bridge v1, authority, policy, credential, `.env`,
source-lock, Runtime installation, model installation, Provider, daemon,
scheduler, or queued product capability changed. Existing user-owned worktree
changes remain excluded.

Prior final-live evidence and the old private attempt remain unchanged:

```text
contract.md
9c5e71767f9983bc53089a8c22a03fcf430bd784d5e2fc602b618a9fca9e1dff

contract-review.md
7cbbeb13b4699fee65e0454329306f967bcc16a32e87aca5fe790c43454b22a1

source-lock.json
e542aa319219f486cb45538df53ff49fb03fb180b748b9661bfb2497d58658f5

live-canary.md
2dbd149403f290800503514293a204288bb85c62d7f4c7acb4c409db1a5a1665

replacement-live-canary.md
05434337f83418c9e68e310e1e0281802935711e9cc4eaa5f408641396248273

resolved-live-manifest.json
b1fb029113655aad94e713ca9d5184b0083a847d28f72487e36ef974aa74fe83

source-provenance-amendment.md
a8f9a26573b2ce9db9ccaeb2f315063bbf07d5bd0a0b67754d38ca23d78c7d26

old canary.sqlite
914f4d872246cfb124be17136d6c3077c08dd3bb2697f9b517d8ee14e65e76ae

old bounded source
bfc5aca5df67c1244e4d28031f973404e5a118d5587288609a625906d3d1a54f

old capture
27329300955d85ad20bc17b920e57fbdbff86a0645ed53b16682f2579ba18263

old receipt
4fee93dbd9d62f80f68c5d54c04b4bc49261eea26edf450f19a0398440e44072
```

The old SQLite remains a regular `0600` file owned by uid `501`. The
additional manifest does not yet exist. Loopback port `18427` has no listener,
and no residual Pi or llama process exists.

## Gate result

The implementation is ready for fresh independent Implementation Review.
Neither staging, commit, installed-binding revalidation, nor the additional
controlled live canary is authorized by this verification result alone.

VERDICT: PASS
