# Final Live Gate Pi RPC Schema and Fresh Attempt Isolation Amendment

Status: CONTRACT REPAIR REVIEW 2 `PASS` — MANDATORY RED IS THE CURRENT GATE

- Amendment: `PHASE1-FINAL-LIVE-PI-RPC-FRESH-ATTEMPT-1`
- Risk: `HIGH`
- Baseline: `b34d8da635c6037c6f3658d580c1dc75bf541f63`
- Date: `2026-07-27`
- Depends on:
  - Final Live Gate Compatibility Amendment Implementation Review 5 `PASS`;
  - Final Live Gate Source Provenance Amendment Contract Review `PASS`;
  - Final Live Gate Metadata Indentation Repair Contract and Implementation
    Reviews `PASS`;
  - local atomic metadata repair commit
    `b34d8da635c6037c6f3658d580c1dc75bf541f63`;
  - the original and replacement controlled canaries' reviewed `FAIL`
    evidence, with both prior authorizations consumed; and
  - explicit user authorization on `2026-07-27` for this amendment and exactly
    one additional controlled live canary after fresh Reviewer `PASS`.

## Exact reason

The one authorized replacement canary passed installed Runtime discovery,
loopback model-server startup, and Team dispatch, then failed closed at the
first Pi user-message event:

```text
response=true agent=true turns=1 message=false done=false settled=false
```

Installed locked Pi `0.82.1` constructs the user message as:

```json
{
  "role": "user",
  "content": [{"type": "text", "text": "<exact prompt>"}],
  "timestamp": 0
}
```

The accepted Loom parser currently requires `content` to be a JSON string.
This is a source-level diagnosis supported by the installed locked source; it
does not claim a captured raw live transcript.

The recovery audit also proved four attempt-boundary gaps:

1. Pi Agent retry defaults to enabled with three retries unless private
   settings disable it.
2. Pi compaction defaults to enabled and context-overflow recovery may perform
   one compact-and-retry.
3. the current live harness reuses one `controlled-canary` directory and
   `canary.sqlite`; and
4. the current live harness rewrites the prior resolved manifest path.

The installed CLI's frozen `--no-approve` argument forces project trust off, so
project `.pi/settings.json` cannot override the Run-private settings. Provider
request retry defaults to zero, but this amendment binds it explicitly rather
than relying on that default.

## Contract Review 1 and bounded Repair 1

Fresh independent Contract Review 1 returned `FAIL` with one Important finding:
the original phrase "atomically materialize" did not explicitly prohibit the
accepted local temp-file-plus-`os.Rename` pattern from replacing an existing
`settings.json`.

Contract Repair 1 freezes final-leaf no-replace publication:

- open the final `settings.json` path itself with
  `O_WRONLY|O_CREATE|O_EXCL` and mode `0600`;
- fail if any object already occupies that final path;
- write the complete exact bytes, `fsync`, close, and revalidate the final leaf
  before `command.Start`;
- start no consumer while the leaf is absent, empty, partial, open, or
  unvalidated; and
- never truncate, unlink, rename over, replace, or otherwise reuse an existing
  final settings path.

Direct exclusive final-leaf creation is the frozen implementation. A temporary
file followed by overwrite-capable `os.Rename` is explicitly forbidden.

## Owned files

Product/test ownership is limited to:

- `internal/runtime/piadapter/rpc_bridge_adapter.go`
- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
- `internal/app/final_live_gate_live_test.go`

Governance evidence ownership is limited to:

- this contract;
- `pi-rpc-schema-fresh-attempt-contract-review.md`;
- `pi-rpc-schema-fresh-attempt-verification.md`;
- `pi-rpc-schema-fresh-attempt-implementation-review.md`;
- after the implementation commit and immediately before the one additional
  live invocation,
  `resolved-live-manifest-additional-canary.json`;
- after that invocation, `additional-live-canary.md`; and
- the private materialization status for bounded result synchronization only.

All existing files under `.loom-evidence/phase1-final-live-gate/**`, including
`resolved-live-manifest.json`, `live-canary.md`,
`replacement-live-canary.md`, `contract.md`, `contract-review.md`, and
`source-lock.json`, are read-only inputs to this amendment. The existing
private `controlled-canary` root, SQLite database, and Evidence artifacts are
also read-only inputs. This amendment must not rename, overwrite, append to, or
delete any of them.

No other Go file, test, dependency, module, ADR, policy, validator,
StateWriter, Journal implementation, Projection implementation, Bridge v1,
Supervisor, Grant, Evidence authority, CLI, daemon, model, Runtime
installation, source lock, credential, environment file, or user-owned dirty
file is owned.

## Frozen Pi `0.82.1` user-message schema

`piRPCUserMessage` may accept exactly:

1. an object with exactly `role`, `content`, and `timestamp`;
2. `role` byte-exactly equal to `user`;
3. `content` as an array containing exactly one object;
4. that object containing exactly `type` and `text`;
5. `type` byte-exactly equal to `text`;
6. `text` byte-exactly equal to the expected bounded prompt; and
7. a finite, non-negative numeric timestamp.

The parser must reject:

- the former string `content` representation;
- null, scalar, object, empty-array, or multi-block content;
- image, tool, thinking, custom, or any non-text block;
- missing, extra, or duplicate keys at either object level;
- empty, changed, normalized, prefixed, suffixed, or otherwise non-exact prompt
  text;
- text signatures or any other optional block field;
- negative, non-finite, string, null, missing, or extra timestamp values;
- invalid UTF-8, malformed JSON, or duplicate-key JSON; and
- any mixed string/array compatibility mode.

The same exact representation must be revalidated at `message_start`,
`message_end`, and `agent_end`. Assistant parsing remains strict and unchanged:
tool, thinking, retry, compaction, extension, unknown, error, oversized, and
identity-mismatched events continue to fail closed.

## Frozen no-retry and no-compaction binding

Before Pi starts, the adapter must exclusively materialize an exact
current-Run-private `settings.json` at its final path under the already
validated `0700` `PI_CODING_AGENT_DIR`. It must open that final leaf with
`O_WRONLY|O_CREATE|O_EXCL` and mode `0600`. Any pre-existing final-path object
fails closed without truncation, deletion, rename, replacement, or content
change. The newly created leaf must contain exact canonical bytes for:

```json
{
  "compaction": {"enabled": false},
  "retry": {
    "enabled": false,
    "maxRetries": 0,
    "baseDelayMs": 0,
    "provider": {
      "maxRetries": 0,
      "maxRetryDelayMs": 0
    }
  }
}
```

The materialized file may use compact JSON plus one final LF. No other setting
is allowed. The adapter must write the complete bytes, `fsync`, close, and
revalidate the final leaf as exact regular/non-symlink/`0600` content before
`command.Start`. No process may observe the file while it is absent, empty,
partial, open, or unvalidated. The adapter retains the frozen `--no-approve`,
`--no-session`, `--no-tools`, `--no-extensions`, `--no-skills`,
`--no-prompt-templates`, `--no-themes`, and `--no-context-files` arguments.

The test process must prove the exact settings file exists before reading the
first RPC request. Missing, malformed, non-private, symlinked, overwritten, or
non-exact settings fail before prompt acceptance. A separate test must prove a
pre-existing regular file, symlink, directory, or other object at the final
settings path is not overwritten and prevents process start. No
`set_auto_retry` RPC command, retry event, hidden retry, fallback, or recovery
continuation is added.

## Frozen fresh-attempt boundary

The additional canary must:

1. create a new unpredictable attempt directory directly below the already
   validated private root using exclusive temporary-directory creation;
2. require that directory to be a non-symlink directory with exact mode
   `0700`;
3. never select, reopen, migrate, or append to the old `controlled-canary`
   directory or any prior attempt directory;
4. place metadata isolation, Runtime home/temp, bounded source, SQLite, and
   Evidence below only the new directory;
5. pre-create the SQLite leaf with `O_CREATE|O_EXCL`, exact mode `0600`, close
   it, revalidate regular/non-symlink/`0600`, and only then call `sql.Open`;
6. fail closed on any directory or SQLite collision instead of reusing state;
7. retain the new directory and its failure/success evidence for recovery; and
8. leave every prior private attempt byte-for-byte untouched.

Logical DAG attempt number remains `1` inside the new, independent Journal.
The external controlled-canary authorization count is distinct: original `1`,
replacement `1`, this additional authorization `1`, remaining after invocation
`0`.

## Independent sanitized manifest

Before metadata discovery, model startup, or Pi execution, the additional
canary must create:

```text
.loom-evidence/phase1-final-live-gate/resolved-live-manifest-additional-canary.json
```

It must use the accepted inspector, digest, containment, ownership,
regular-file, non-symlink, and `0600` rules. It must not overwrite the prior
`resolved-live-manifest.json`. The new manifest remains sanitized: no private
root, executable/model path, prompt, credential, mirror URL, raw Grant, hidden
reasoning, or model output.

## Mandatory RED

Before production/harness behavior changes, tests must encode:

1. the exact Pi `0.82.1` single-text-block user message and complete rejection
   matrix;
2. exact settings bytes and `0600` materialization before the fixture reads
   the prompt;
3. two fresh attempt-root creations are distinct and `0700`;
4. SQLite is created once as regular non-symlink `0600`, and collision reuse
   fails;
5. the additional manifest path differs from and does not mutate the prior
   manifest path.

Focused RED commands:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821UserMessageCompatibility|BridgeCreatesExactNoRetrySettingsBeforePrompt)$' \
  -count=1

go test ./internal/app \
  -run '^TestFinalLive(FreshAttemptIsolationAndPrivateSQLite|AdditionalManifestDoesNotAliasPriorEvidence)$' \
  -count=1
```

At least one runtime test must fail because baseline rejects valid array
content, and the settings-before-prompt test must fail because baseline creates
no settings file. The app test may initially fail to compile only on the
contract-frozen missing helpers.

RED output must not include private absolute paths, prompts, credentials, raw
RPC records, Grants, or model output.

## Minimal GREEN

Implementation is limited to:

- replace string user-content acceptance with exact single-text-block array
  acceptance;
- exclusively create and revalidate the exact private
  no-retry/no-compaction settings at the final path without replacing any
  existing object, alongside the existing private model config;
- add fresh attempt-root and exclusive private SQLite helpers in the opt-in
  final-live harness;
- route the additional canary exclusively through those helpers; and
- route resolved-manifest creation to the new frozen evidence filename.

No compatibility union, generic message-block framework, general settings
writer, retry coordinator, scheduler, daemon, schema migration, new dependency,
or unrelated refactor is allowed.

## Verification matrix

Before Implementation Review:

```text
go test ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821UserMessageCompatibility|BridgeCreatesExactNoRetrySettingsBeforePrompt)$' \
  -count=1

go test ./internal/app \
  -run '^TestFinalLive(FreshAttemptIsolationAndPrivateSQLite|AdditionalManifestDoesNotAliasPriorEvidence)$' \
  -count=1

go test ./internal/runtime/piadapter ./internal/app -count=1

go test -race ./internal/runtime/piadapter \
  -run '^TestPiRPC(Pi0821UserMessageCompatibility|BridgeCreatesExactNoRetrySettingsBeforePrompt)$' \
  -count=30

go test -race ./internal/app \
  -run '^TestFinalLive(FreshAttemptIsolationAndPrivateSQLite|AdditionalManifestDoesNotAliasPriorEvidence)$' \
  -count=30

go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go mod verify

gofmt -d \
  internal/runtime/piadapter/rpc_bridge_adapter.go \
  internal/runtime/piadapter/rpc_bridge_adapter_test.go \
  internal/app/final_live_gate_live_test.go

git diff --check

GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime/piadapter ./internal/app
```

Scope/trust audit must additionally prove:

- only the three owned Go files and new amendment evidence changed;
- prior final-live evidence hashes and old private SQLite hash are unchanged;
- no dependency/module, Bridge v1, authority, policy, credential, `.env`,
  source lock, Runtime/model installation, or excluded user-owned file changed;
- no network, Runtime, Provider, model process, or live canary was used during
  RED/GREEN/review; and
- no listener or residual Pi/llama process exists.

## Review, commit, and live gate

1. Fresh independent Contract Review `PASS` is required before test edits.
2. Mandatory RED precedes all production/harness behavior changes.
3. The full matrix and fresh independent Implementation Review must return
   `PASS`.
4. Only then may one local atomic commit include the three owned Go files and
   this amendment's contract/review/verification/implementation-review files.
   Existing failed-canary/provenance evidence and excluded user dirt remain
   unstaged.
5. After that commit, the Controller must freshly revalidate exact installed
   Pi, llama.cpp, model, source lock, private ownership/modes, GGUF header,
   digests, sanitized manifest preconditions, empty loopback port, and absence
   of residual processes.
6. Only after every pre-live check passes may exactly one additional invocation
   run:

```text
go test ./internal/app \
  -run '^TestFinalLiveGatePiRPCOfflineModel$' \
  -count=1 -v
```

The invocation may start only the accepted loopback llama.cpp server and one Pi
RPC process using the exact locked offline model. Success or failure consumes
the authorization. Any failure stops immediately with no retry, hidden rerun,
second additional canary, alternate model/source/Provider, parser expansion, or
fallback.

Fresh independent result-evidence review is required after the invocation.
Even a successful result may advance only to
`READY_FOR_FINAL_USER_SIGNOFF`; it cannot supply the user's final sign-off.

## Explicit exclusions

This amendment does not:

- modify Bridge v1, Journal/StateWriter/Projection authority, policy, Grant,
  Evidence, credential, or verification authority;
- change the installed Pi, llama.cpp, model, source provenance, mirror
  acceptance, or Runtime binding;
- add tools, images, thinking, hidden reasoning, session resume, checkpoint,
  retry, fallback, Provider switching, remote/network traffic, daemon,
  scheduler, autonomy, Web/TUI, Phase 2, or queued product capabilities;
- overwrite, summarize away, delete, or reinterpret either prior failed
  attempt; or
- authorize push, merge, rebase, reset, force, release, publication, or final
  user sign-off.

VERDICT: PASS
