# P2A-W2 Isolated Pi Catalog and Live Closure Amendment

**Date**: 2026-07-30
**Status**: FROZEN — Repair 1 fresh independent Contract Re-review PASS
**Parent**: `P2A-W2 Team Builder and Provider Onboarding Contract`
**Risk**: STRICT — process isolation, local model binding, Provider credential
recovery, native product activation

## 1. Reason for reopening

The reviewed owner-waived diagnostic established two independent compatibility
defects:

1. Codex 0.144.1 writes its successful `login status` line to stderr. The
   reviewed Provider Connection amendment has now repaired this defect.
2. Pi 0.82.1 lists only models belonging to a configured Provider. Loom's
   metadata runner intentionally creates a fresh `PI_CODING_AGENT_DIR` for
   every call and inherits no user credential or Pi state, so its current
   `--list-models` result is necessarily empty.

The empty Pi catalog prevents construction of every Team Builder role option.
It is not safe to invent a model ID in the product setup service or to inherit
the user's Pi configuration. Phase 1 already accepted one exact, secret-free
`loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m` declaration and exact local
GGUF/llama-server binding. This amendment reuses that accepted adapter
capability as the only source for an isolated metadata catalog.

This remains one vertical repair and live closure inside P2A-W2. It creates no
P2A-W4 and does not unlock or modify P2A-W3 before W2 acceptance.

## 2. Product behavior

When and only when the service manager supplies all three reviewed local-model
paths:

```text
private root
llama-server executable
GGUF model
```

the daemon must:

1. bind the private directory chain, regular executable and exact frozen model
   digest through the accepted read-only local-model inspector;
2. retain the bound file identities and revalidate them before every metadata
   process call;
3. materialize a fresh private `0600` `models.json` inside that call's
   disposable Pi agent directory;
4. use the same serializer, Provider/model IDs, context/output limits,
   compatibility flags and documented non-secret `loom-local-offline`
   placeholder as the accepted Pi RPC adapter;
5. run Pi with the existing clean environment, `PI_OFFLINE=1`, bounded output,
   timeout and process-group cleanup;
6. parse the real Pi 0.82.1 `--list-models` output and publish the resulting
   model ID through the existing Runtime discovery Event and Projection.

The metadata-only loopback URL is fixed by Loom and is never contacted during
discovery. Catalog materialization does not start llama-server, open a socket,
perform inference, resolve a Provider secret, or make a network request.

If the local-model fields are absent, existing observer-only daemon behavior is
unchanged and may truthfully discover an empty model collection. If any field
is present but the complete set is not exact, or any bound identity changes,
construction/observation fails closed before publishing a model-capable Runtime
fact.

The paths are service-manager inputs. They are never requested from or shown to
an ordinary native-app user.

## 3. Authority and trust boundaries

- Runtime discovery remains the only authority for Runtime model capability.
  Team Builder may not synthesize or supplement `model_ids`.
- The Event Journal, StateWriter and Projection are unchanged.
- The shared local catalog is adapter configuration, not a second model,
  credential, state or execution authority.
- The catalog contains no real secret. The placeholder
  `loom-local-offline` is fixed public configuration and cannot authorize an
  external Provider.
- No user Pi home, `auth.json`, `models.json`, environment credential or
  session state is read or copied.
- No model/llama path, digest, internal Provider ID or placeholder crosses the
  native product IPC/UI boundary.
- A missing, replaced, symlinked, wrong-owner, wrong-mode, wrong-digest or
  out-of-root model/server binding fails closed.
- The separate accepted resident observer and its state remain untouched.

## 4. Exact implementation ownership

Only these existing P2A-W2/accepted Runtime-adapter files may be reopened:

```text
internal/runtime/piadapter/local_model_catalog.go
internal/runtime/piadapter/local_model_catalog_test.go
internal/runtime/piadapter/local_model_server.go
internal/runtime/piadapter/rpc_bridge_adapter.go
internal/runtime/piadapter/rpc_bridge_adapter_test.go
internal/runtime/piadapter/process_runner.go
internal/runtime/piadapter/process_runner_test.go
internal/runtime/piadapter/local_probe.go
internal/runtime/piadapter/local_probe_test.go
internal/app/runtime_daemon.go
internal/app/runtime_daemon_test.go
cmd/loomd/run.go
cmd/loomd/run_test.go
cmd/loomd/product_daemon_test.go
docs/CURRENT.md
.loom-evidence/phase2a/P2A-W2/
```

The new catalog files are allowed only to extract and share the already
accepted local Pi catalog serializer/binding. No Journal schema, migration,
Team/Agent authority, scheduler, execution policy, credential store, Swift
decoder, Phase 1 evidence, resident daemon configuration or unrelated dirty
file is owned.

## 5. Mandatory RED and deterministic verification

Before production changes, tests must fail for the missing behavior:

1. a real Pi-shaped fixture under a fresh metadata agent directory sees the
   exact shared `models.json` and returns the accepted local model;
2. product-daemon construction with exact local-model inputs flows through
   factory → runner → real parser → Runtime Event → Projection → setup snapshot
   and produces non-empty role options;
3. partial inputs, wrong model digest, unsafe path chains and post-bind identity
   replacement fail before model-capable publication;
4. the metadata and RPC paths serialize byte-identical catalog semantics while
   retaining their independent disposable roots and lifecycles;
5. the invocation directory is removed after success, failure, timeout and
   cancellation, with no secret or bound path in errors/output;
6. existing no-catalog daemon behavior remains compatible and canonical
   collections remain JSON arrays.

Required GREEN evidence:

```text
focused piadapter/app/loomd tests
focused race repetitions
go test ./...
go test -race ./...
go vet ./...
go mod tidy -diff
go mod verify
gofmt and git diff --check
Swift debug, release and thread-sanitizer matrices
secret-negative and exact-owned-file scans
fresh independent Implementation Review
```

Deterministic tests may use only local fixtures and may not start installed Pi,
llama-server, OAuth, MiniMax, the native app, the resident daemon or any
network request.

## 6. One controlled W2 live closure

Implementation Review `PASS` permits one bounded closure lineage:

```text
p2a-w2-live-20260730-003
```

It uses:

- a fresh private `0700` attempt root and `0600` state database;
- an integrity-checked private copy of the exact closed post-diagnostic W2
  attempt-002 Journal baseline defined below, so the product can recover its
  existing Keychain reference without reading or re-entering the secret;
- newly built exact Candidate daemon and addressable native app bundle;
- the accepted Pi 0.82.1 and Node targets;
- the exact accepted llama-server and GGUF bytes;
- the exact accepted Codex native executable;
- one isolated socket, no LaunchAgent mutation and no resident-observer touch.

### 6.1 Exact inherited Journal baseline

The only allowed clone source is:

```text
/Users/lune/Library/Application Support/Loom/phase2a-w2-live-20260730-002/state/loom.db
```

The source is frozen at the post-diagnostic state:

```text
file mode                     = 0600 regular, no symlink
owner                         = uid 501
size                          = 24576 bytes
SHA-256                       = b0312739ba7b24961b9e8a698e4cbf436e9cace8758a1d2a058e050aec475e22
SQLite integrity_check        = ok
Event schema_version values   = exactly [1]
RuntimeInstanceDiscovered     = 1
ProviderCredentialConfigured  = 1
ProviderCredentialVerified    = 3
total Events                  = 5
every other Event type        = 0
```

The Runtime stream has sequence `1` only. The credential stream has sequence
`1` through `4`, contains one stable opaque credential reference across all
four Events, and that reference has the accepted `credential-ref-*` shape.
Review and evidence may assert only that shape, equality and length; they may
not print the reference value.

The source database remains read-only and byte-unchanged. Before copying,
preflight must reprove its path, clean/no-symlink chain, owner, mode, size,
SHA-256, SQLite integrity, Event set, stream sequences, schema versions and
credential-reference shape. Any mismatch stops before build, daemon or app.

The attempt-003 database is a new writable byte-for-byte copy under the fresh
`0700` attempt root. It must be a same-owner regular `0600` file with no
symlink, absent WAL/SHM residue, `integrity_check=ok`, and the exact source hash
before the new daemon first opens it. The source hash before and after copying
and the destination hash before daemon startup are recorded. The destination
may diverge only through subsequent authoritative attempt-003 Journal appends.

Result-Evidence Review must separate the five inherited attempt-002 baseline
Events from every Event appended by attempt-003. W2 acceptance may count only
Events after the cloned baseline as attempt-003 Provider-Test or Team Builder
success. The inherited Events remain invalid/diagnostic provenance and may be
used only to recover the existing credential reference and prior Runtime state;
they cannot be counted as new closure success.

The Product Owner's explicit source-provenance waiver applies to the existing
brokered MiniMax credential only. It does not rewrite attempt-002 history or
make the inherited diagnostic verification fact valid W2 evidence. The new
closure may use that credential only through the product Credential Broker and
OS Secret Store; the Controller may not read, export, enumerate, copy, print,
type, log or screenshot it.

The single native journey must prove:

1. Codex is observed as `available/native_auth` from the exact single stderr
   line without launching login when already connected;
2. MiniMax is recovered as configured and one explicit native `Test` produces
   one new closed verification fact;
3. Pi is online with the exact local model and Team Builder exposes compatible
   Main/SubAgent role options;
4. an ordinary user can complete the one-question-at-a-time Candidate,
   inspect compatibility/permissions/cost, and explicitly confirm exactly one
   TeamDefinition;
5. confirmation creates no TeamInstance, AgentInstance, WorkItem, Run, Grant,
   Evidence, dispatch or execution;
6. app restart reconstructs the same Provider/Runtime/saved-Team result from
   Journal/Projection;
7. socket/database/app/attempt permissions, Event counts, no-secret surfaces
   and process cleanup are exact.

No `codex login`, logout, browser OAuth, Team execution, Provider fallback,
credential replacement/revocation, llama-server inference, Pi RPC session,
push, merge, release or publication is authorized by this W2 closure.

## 7. Exit and status semantics

After live cleanup, a fresh independent Result-Evidence Reviewer must verify
the exact binaries, paths/modes, Event set, Provider/Runtime snapshots,
TeamDefinition-only authority, absence of secret leakage and absence of
attempt processes/socket.

P2A-W2 may be accepted only if deterministic Implementation Review and live
Result-Evidence Review both return `PASS`. The historical attempt-002 remains
invalid evidence; acceptance, if earned, belongs only to the reviewed
attempt-003 lineage under this amendment.

Even W2 acceptance does not itself satisfy the Phase 2A Codex browser-OAuth,
MiniMax execution or Pi execution canaries. Those final execution proofs remain
inside the future P2A-W3 controlled execution boundary. P2A-W3 stays locked
until W2 is accepted and recorded.
