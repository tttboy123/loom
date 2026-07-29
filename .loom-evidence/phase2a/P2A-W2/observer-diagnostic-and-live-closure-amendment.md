# P2A-W2 Observer Diagnostic and Live Closure Amendment

**Date**: 2026-07-30
**Status**: FROZEN — Contract Repair 2 fresh independent Re-review PASS
**Parent**: `P2A-W2 Team Builder and Provider Onboarding`
**Reopens**: the accepted Runtime metadata observation adapter and product
daemon failure projection only inside P2A-W2
**Risk**: STRICT — process output, failure disclosure, local Runtime identity,
native Provider/Team setup and one live allowance

## 1. Reason for reopening

The deterministic Isolated Pi Catalog Candidate and its fresh Implementation
Review passed. The single `p2a-w2-live-20260730-003` daemon start then reached
the product IPC `Ready()` boundary, started Runtime observation, and exited
fail-closed with:

```text
daemon failed: observer
```

The destination Journal remained byte-identical to its inherited five-Event
baseline, so the failure preceded every authoritative append. Cleanup removed
the socket and disposable Pi metadata directory. The native app was not
launched.

The current public error deliberately suppresses the wrapped observer error.
That is safe but too coarse to distinguish:

- probe/binding construction;
- Pi version process, stderr or parsing;
- Pi model-list process, stderr or parsing;
- projection refresh;
- discovery/status planning or authoritative append.

Running another daemon without first closing that diagnostic gap would consume
another live allowance without a bounded hypothesis. This amendment closes the
diagnostic and compatibility boundary together. It is not a new WorkItem, does
not create P2A-W4, and does not unlock P2A-W3.

## 2. Required product behavior

### 2.1 Fail-closed safe observer reason codes

The internal error chain must retain both:

1. the exact metadata command stage: `version` or `model-list`;
2. the existing typed cause, including binding change, process failure,
   timeout, output limit, stderr, invalid output or duplicate model.

Retaining a cause must not publish child stdout/stderr or expose a path,
argument, environment value, model path, credential, process detail or
arbitrary wrapped error text.

The daemon CLI may project only one stable allowlisted reason:

```text
observer_probe_factory
observer_metadata_binding
observer_version_process
observer_version_timeout
observer_version_output_limit
observer_version_stderr
observer_version_output
observer_models_process
observer_models_timeout
observer_models_output_limit
observer_models_stderr
observer_models_output
observer_models_duplicate
observer_projection
observer_write
observer_unknown
```

Unknown, joined, conflicting or unclassified errors map to
`observer_unknown`. The exact observer stderr shape is:

```text
daemon failed: <one allowlisted observer reason>\n
```

The CLI continues to return exit code `4` and must never append raw
`error.Error()` text. Existing exact `daemon failed: local_ipc\n` and
`daemon failed: shutdown\n` mappings remain unchanged.

### 2.2 Real locked Pi component closure

After Contract Review `PASS`, one local component lineage may execute exactly
the locked Pi 0.82.1 metadata lifecycle through the Candidate Runner:

```text
p2a-w2-pi-component-20260730-001
```

It must use:

- a fresh private `0700` root with empty HOME, agent, session and temp state;
- the exact installed Pi CLI identity already frozen by W2;
- the exact reviewed Node identity and ordered search paths;
- the exact reviewed llama-server and frozen GGUF identity;
- the Candidate's real atomic `0600` catalog serializer/materializer;
- `PI_OFFLINE=1`, no inherited Pi/auth/session/config environment, no stdin,
  bounded stdout/stderr, timeout and process-group cleanup;
- exactly one version request and one offline model-list request.

This is a component test, not a product canary. It may not start the daemon,
native app, local model server, inference, Codex, Keychain or MiniMax; open a
network socket; access a user Pi home; or write the Journal.

### 2.2.1 Exact skip-closed manifest and enable gate

The only accepted component root and manifest are:

```text
root:
/Users/lune/Library/Application Support/Loom/p2a-w2-pi-component-20260730-001

manifest:
/Users/lune/Library/Application Support/Loom/p2a-w2-pi-component-20260730-001/manifest.json
```

The root and all child directories must be newly created regular directories,
owned by uid `501`, mode `0700`, with no symlink component. The manifest must be
a same-owner regular `0600` file with no symlink. It uses strict JSON with no
unknown or duplicate key and exactly:

```json
{
  "schema_version": 1,
  "lineage_id": "p2a-w2-pi-component-20260730-001",
  "component_root": "<exact absolute root above>",
  "isolation_root": "<exact absolute root above>/isolation",
  "local_model_private_root": "/Users/lune/Library/Application Support/Loom/phase1-live",
  "pi_search_entry": {
    "path": "/Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/.bin/pi",
    "resolved_path": "/Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/@earendil-works/pi-coding-agent/dist/cli.js",
    "sha256": "af302f231437eaf6f37691bce4b34234fcb626bcb5eb3910d4fc3f6519bf78ca"
  },
  "node_executable": {
    "path": "/Users/lune/Documents/Codex/devtools/node/bin/node",
    "sha256": "1ee75375e33b94fc34b3b19aede049e11dae90efb63b374dc96d6bdace70c4b8"
  },
  "runtime_search_paths": [
    "/Users/lune/Library/Application Support/Loom/runtimes/pi/0.82.1/node_modules/.bin",
    "/Users/lune/Documents/Codex/devtools/node/bin"
  ],
  "llama_executable": {
    "path": "/Users/lune/Library/Application Support/Loom/phase1-live/runtime/llama-b10107/llama-server",
    "sha256": "a4998768a70ba2be02617ec9d8773accc2952516f4f5a8f38f621ece54cbf04b"
  },
  "model": {
    "path": "/Users/lune/Library/Application Support/Loom/phase1-live/models/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
    "size": 1117320768,
    "sha256": "cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046"
  },
  "timeout": "10s",
  "expected_version": "0.82.1",
  "expected_model_id": "loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m",
  "network_policy": "offline_no_listener"
}
```

Before either installed-Pi execution, the Controller records the Pi search
entry, its exact resolved path/hash and the exact manifest SHA-256. The
component test independently revalidates the permitted upstream leaf symlink
to that exact regular resolved file, all directory components, every named
path/hash/mode/owner/size, ordered search paths, private-root identity, empty
isolation root, local-model private-root identity, expected values and the
exact manifest path.

Both inputs must be present and exact:

```text
LOOM_P2A_W2_LOCKED_PI_COMPONENT=p2a-w2-pi-component-20260730-001
LOOM_P2A_W2_LOCKED_PI_MANIFEST=/Users/lune/Library/Application Support/Loom/p2a-w2-pi-component-20260730-001/manifest.json
```

Absent, extra, malformed, mismatched or unsafe input causes the test to skip
before process construction. The test may not consult any other environment
value for identity/configuration. Before and after execution it asserts no
daemon/app/llama listener or attempt process, an empty isolation root after
Runner cleanup, absent Journal/socket, unchanged locked file identities, and
the structural offline boundary: `PI_OFFLINE=1`, empty inherited credentials,
no loopback model server and no product/network client.

The component result is GREEN only if:

- version stdout parses to the exact accepted Pi version with empty stderr;
- model-list stdout parses through the real production parser to exactly
  `loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m`;
- model-list stderr is empty;
- the private invocation is removed;
- no installed/source identity changed.

At most two component executions are authorized, each with exactly one version
and one model-list command:

1. execution A runs only after Contract Review `PASS`; failure records its safe
   reason as mandatory RED and stops;
2. execution B is permitted only after a code/test repair for execution A and
   fresh deterministic GREEN; if A was GREEN, B is forbidden.

There is no automatic retry or loop. If execution B fails, the component
lineage stops `HUMAN_REQUIRED`. Implementation may not relax stderr, accept
unknown/multiline output, inherit user state, contact the fixed loopback URL,
synthesize a model ID or skip real Pi.

### 2.3 Binding efficiency without weaker identity

The exact 1.117 GB model must not be rehashed redundantly inside a single
metadata observation cycle. Construction must establish the accepted binding
once. Before each command the adapter must still revalidate:

- path and no-symlink chain;
- file identity, owner, mode and size;
- executable identity;
- the model digest at least once for that command.

Version and model-list remain separately fenced. A replacement, mode/owner
change, size change, digest change or directory identity change before either
command must fail before process launch. No cache may survive the configured
factory/Runner lifetime in a way that turns Projection into authority or
accepts a later file identity.

## 3. Mandatory RED and deterministic proof

Before production changes, tests must prove the missing boundary:

1. command failures retain a typed safe `version`/`model-list` stage and typed
   cause while their text excludes the arbitrary source error;
2. exact reason mapping covers every allowlisted class and maps joined,
   conflicting, unknown and private-detail errors to `observer_unknown`;
3. the CLI emits only the stable reason and exit `4`;
4. stdout/stderr/path/environment/private error text never crosses the daemon
   boundary;
5. binding replacement between commands and during validation fails before
   launch;
6. the real locked Pi component test is skip-closed without its exact
   Controller-supplied manifest and leaves no residue.

Required GREEN:

```text
focused runtime/piadapter/app/loomd tests
focused race repetitions
go test -p 1 ./... -count=1
go test -race -p 1 ./... -count=1
go vet ./...
go mod tidy -diff
go mod verify
gofmt and git diff --check
Swift debug, release and thread-sanitizer matrices
owned-file and secret-negative scans
locked Pi component lineage GREEN
fresh independent Implementation Review
```

Normal repository tests may not execute installed Pi or any live component.
The locked component test must require the exact manifest/enable input and skip
without side effects otherwise.

## 4. Exact ownership

Only these files may change:

```text
internal/runtime/pi_probe.go
internal/runtime/pi_probe_test.go
internal/runtime/piadapter/process_runner.go
internal/runtime/piadapter/process_runner_test.go
internal/runtime/piadapter/local_model_catalog.go
internal/runtime/piadapter/local_model_catalog_test.go
internal/runtime/piadapter/local_probe.go
internal/runtime/piadapter/local_probe_test.go
internal/runtime/piadapter/locked_pi_component_test.go
internal/app/runtime_daemon.go
internal/app/runtime_daemon_test.go
internal/app/runtime_observation_loop.go
internal/app/runtime_observation_loop_test.go
cmd/loomd/product_daemon.go
cmd/loomd/product_daemon_test.go
cmd/loomd/run.go
cmd/loomd/run_test.go
docs/CURRENT.md
.loom-evidence/phase2a/P2A-W2/
```

No Journal schema, StateWriter, Projection authority, Runtime/Team domain
contract, Swift decoder/UI, credential store, resident observer configuration,
Phase 1 evidence or unrelated dirty file is owned.

## 5. One replacement W2 live closure

Only locked component GREEN, complete deterministic GREEN and fresh
Implementation Review `PASS` authorize one replacement lineage:

```text
p2a-w2-live-20260730-004
```

It must use:

- a fresh private `0700` root and a fresh regular `0600` database;
- the same exact frozen attempt-002 five-Event source baseline and all
  source/destination integrity, hash, schema, stream and no-symlink checks from
  the reviewed attempt-003 amendment;
- newly built exact Candidate daemon and addressable native app;
- exact reviewed Pi, Node, Codex, llama-server and GGUF identities;
- an isolated private socket;
- no LaunchAgent or resident-observer mutation.

The source credential reference may be used only through the existing
Credential Broker and OS Secret Store under the Product Owner's recorded
source-provenance waiver. The Controller may not read, export, enumerate, copy,
print, type, log, screenshot or otherwise materialize the secret.

One daemon start and one ordinary native journey must prove:

1. Runtime discovery writes exactly one new model-capable fact or an exact
   valid higher-sequence rediscovery fact and Team Builder exposes compatible
   Main/SubAgent role options;
2. Codex is `available/native_auth` from exactly one valid stdout-or-stderr
   status line without login when already connected;
3. MiniMax is recovered from the Broker and one explicit native `Test` appends
   exactly one new closed verification fact;
4. the ordinary user completes the one-question Candidate and explicitly
   confirms exactly one TeamDefinition;
5. no TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence, dispatch or
   execution is created;
6. native app restart reconstructs the same Provider, Runtime and saved-Team
   result from Journal/Projection;
7. post-exit socket/process cleanup, permissions, Event delta, exact binaries
   and secret-negative surfaces pass.

If startup fails, the stable reason code is recorded and the lineage stops. No
silent retry, alternate executable, argument change, inherited state,
additional canary or single-point amendment is permitted.

## 6. Exit semantics

Fresh independent Result-Evidence Review must verify both the locked component
result and the replacement native journey. P2A-W2 is accepted only if:

- Contract Review is `PASS`;
- locked Pi component result is GREEN;
- deterministic Implementation Review is `PASS`;
- replacement live Result-Evidence Review is `PASS`;
- the exact native journey satisfies every item above.

Anything less leaves P2A-W2 unaccepted, P2A-W3 locked and no P2A-W4.
