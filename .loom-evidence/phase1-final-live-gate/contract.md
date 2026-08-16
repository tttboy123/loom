# Final Live Gate Compatibility Amendment

Status: REPLACEMENT CONTROLLED LIVE CANARY `FAIL` — `HUMAN_REQUIRED`, NO
SECOND REPLACEMENT.
Source provenance, atomic installation, pre-live binding, the metadata
indentation repair, and production metadata discovery passed. The single
authorized replacement advanced through local model-server startup into Team
execution, then failed closed in Pi RPC protocol validation because installed
Pi `0.82.1` represents the user message `content` as a text-block array while
the frozen adapter accepts only a string. No authorized assistant output was
accepted. The original attempt and its one replacement are both consumed; no
retry or further replacement is authorized.
FROZEN TECHNICAL CORRECTION 2, Product Repair 3 exhausted, Implementation
Review 4 `FAIL`, and explicit user authorization for one narrow Repair 4.
Contract Review 1 `FAIL`, bounded
contract Repair 1, Contract Review 2 `PASS`, behavioral RED, implementation
GREEN, Implementation Review 1 `FAIL`, bounded product Repair 1, and
Implementation Review 2 `FAIL`. The repeated Pi-schema failure received fresh
read-only problem analysis. Contract Reviews 3–5 returned `FAIL`; their
governance repairs and known product RED are frozen. Technical Correction 2
received fresh independent Contract Review 8 `PASS`; Product Repairs 2 and 3
were implemented and fully reverified. Implementation Review 4 nevertheless
reproduced a focused timeout/child-exit classification flake after the final
bounded repair.
Repair 4 owns only timeout/child-exit arbitration and the health-then-exit
fixture startup budget. Its full reverification and fresh independent
Implementation Review 5 passed and the reviewed local compatibility commit was
created at `7829423`. External materialization and the two subsequently
authorized controlled attempts are recorded in the final-live evidence.

- Amendment: `PHASE1-FINAL-LIVE-COMPAT-1`
- Risk: `HIGH`
- Baseline: `f51bfd4`
- Date: `2026-07-27`
- Authority: explicit user authorization on 2026-07-27, `TECH-PLAN.md`
  sections 6, 13–16, accepted ADR-0003/0007/0009, and the accepted Slice 5
  Exit Contract
- Architecture candidate: ADR-0010
- Runtime: reviewed `@earendil-works/pi-coding-agent@0.82.1`
- Scope: Pi 0.82.1 metadata compatibility, Pi RPC to `loom.bridge.v1`
  translation, and one controlled local offline-model final canary

This is a final human-gate compatibility amendment, not S5-W3 and not a new
product Slice. It does not reopen a completed Slice capability or weaken the
22-item engineering acceptance. It supplies the missing compatibility needed
to execute the already-authorized final installed-Runtime task.

No implementation file may be created until a fresh independent Contract
Reviewer returns `PASS`. No external binary/model asset may be downloaded,
extracted, or started, and no Provider/model request or live canary may execute,
until behavioral RED, GREEN, the full verification matrix, a fresh independent
Implementation Reviewer `PASS`, and the local compatibility commit are all
complete.

## Current evidence and exact defect

The reviewed installed Pi package is version 0.82.1 and its npm `gitHead`
matches the official repository tag. It was installed user-locally with
scripts disabled and its isolated dependency audit is clean after the recorded
`brace-expansion` 5.0.8 hardening.

Read-only preflight established:

1. `pi --version` returns `0.82.1`.
2. Pi's exact offline model-list command exits zero but emits:

   ```text
   No models available. Use /login to log into a provider via OAuth or API key. See:
   <absolute-install-root>/docs/providers.md
   <absolute-install-root>/docs/models.md
   ```

3. `parsePiModels` rejects that output as `ErrInvalidPiMetadataOutput`.
4. Pi 0.82.1 contains no `loom.bridge.v1` implementation.
5. Pi 0.82.1 documents strict LF-delimited JSONL RPC via `--mode rpc`.
6. The host has no `llama-server`, Ollama, or model asset in the controlled
   Loom root.
7. Installed Runtime discovery emits adapter type `pi-cli`; the historical
   direct-Bridge Pi fixture adapter returns `pi`.

No model invocation, credential access, resident daemon, or live canary was
performed while obtaining this evidence.

## Exact ownership

Governance candidate owned now:

- `.loom-evidence/phase1-final-live-gate/contract.md`
- `.loom-evidence/phase1-final-live-gate/source-lock.json`
- `.loom-evidence/phase1-final-live-gate/contract-review.md`
- `docs/adr/0010-pi-rpc-bridge-and-controlled-offline-model.md`
- the ADR-0008/0009/0010 index rows in `docs/adr/README.md`

Product and test ownership opens only after Contract Review PASS:

- `internal/runtime/pi_probe.go`
  - recognize only the exact bounded Pi 0.82.1 no-model diagnostic;
  - discard diagnostic paths and preserve every existing table parser rule.
- `internal/runtime/pi_probe_test.go`
  - exact compatibility fixture and negative matrix.
- `internal/runtime/piadapter/rpc_bridge_adapter.go`
  - new Pi RPC translation `supervisor.RuntimeAdapter`;
  - no change to `loom.bridge.v1` or an accepted authority.
- `internal/runtime/piadapter/rpc_bridge_adapter_test.go`
  - protocol, binding, streaming, cancellation, cleanup, and Supervisor tests.
- `internal/runtime/piadapter/local_model_server.go`
  - exact short-lived llama.cpp/model binding and loopback lifecycle.
- `internal/runtime/piadapter/local_model_server_test.go`
  - constructor, binding, health, timeout, cancellation, and cleanup tests.
- `internal/runtime/piadapter/local_model_process_unix.go`
- `internal/runtime/piadapter/local_model_process_other.go`
  - process-group containment and unsupported-platform stub only.
- `internal/app/final_live_gate_live_test.go`
  - opt-in external-package live harness;
  - skipped unless the exact gate environment is present;
  - no production authority or ordinary test execution.
- `.loom-evidence/phase1-final-live-gate/**`
  - RED/GREEN/matrix/review evidence;
  - resolved manifest and sanitized live evidence after their gates pass.

The existing `internal/runtime/piadapter/execution_adapter.go`,
`protocol/bridge/v1/**`, Supervisor, Grant, Journal, StateWriter, Projection,
Team scheduler/coordinator, Evidence authority, CLI, daemon, migrations,
`go.mod`, and `go.sum` remain closed.

`AGENTS.md`, `PROGRESS.md`, `.codex/**`, `.loom-drafts/**`, and
`.loom-evidence/phase1-slice3/POST-S3-W5-QUEUED-CONTRACT-INPUTS.md` remain
excluded and unstaged. `docs/CURRENT.md` is excluded because it contains
pre-existing user-owned changes; this amendment records status in its own
evidence tree and does not overwrite that dirt.

Any additional product file, dependency, executable, protocol field, writer,
authority, daemon, or persistent service requires a new reviewed amendment or
stops `HUMAN_REQUIRED`.

## Frozen architecture and dependency direction

Production direction is:

```text
internal/runtime/piadapter
  -> internal/supervisor
  -> protocol/bridge/v1

internal/runtime/pi_probe.go
  -> internal/runtime domain values
```

The opt-in live test may compose accepted public APIs from `internal/app`,
`internal/authorization`, `internal/evidence`, `internal/journal`,
`internal/projection`, `internal/runtime`, `internal/runtime/piadapter`,
`internal/supervisor`, `internal/teams`, `internal/work`, and
`protocol/bridge/v1`.

The Pi RPC translator is a Runtime Adapter only. It cannot:

- append Journal facts or write SQLite/Evidence directly;
- issue/revoke a Grant or accept a terminal;
- construct a WorkItem, Run, Team, approval, recovery directive, or Done fact;
- retry, fall back, schedule, or infer policy;
- publish a Frame before the existing Supervisor `FrameSink` accepts it; or
- make Pi RPC identity, session state, model output, or llama.cpp health an
  authority.

ADR-0010 returns to `proposed` for Technical Correction 2. A fresh Contract
Reviewer PASS plus the user's existing explicit authorization accepts the
corrected decision; the same Candidate then changes ADR-0010 and its index row
to `accepted` before product Repair 2. Repair 2 must first add regression tests
that fail against the current synthetic event mapping and inferred-root
implementation; the original behavioral RED remains preserved.

## Pi 0.82.1 metadata compatibility

`parsePiModels` continues to accept:

- the legacy exact single line `No models available.`; or
- exactly three physical LF-delimited lines, with at most one final LF:

  1. `No models available. Use /login to log into a provider via OAuth or API key. See:`
  2. one absolute clean UTF-8 path ending `/docs/providers.md`;
  3. one absolute clean UTF-8 path ending `/docs/models.md`.

The three physical lines may not have leading/trailing boundary whitespace.
No leading, interstitial, or appended blank/whitespace-only line is accepted.
The two documentation paths must share the same non-empty clean parent
directory, contain no control character, be at most 4096 bytes each, and appear
in that order. No fourth physical line is accepted. The paths are
validation-only and are never returned, logged, journaled, projected, or placed
in Evidence.

Prefix-only matches, relative paths, Windows drive syntax on the current
Unix-only runner, dot segments, duplicate/swapped basenames, different parents,
extra text, extra lines, NUL/control bytes, and output over existing limits
remain `ErrInvalidPiMetadataOutput`.

The tabular model inventory grammar, sorting, duplicate detection, typed errors,
and existing output bounds do not change.

## Exact Pi RPC Bridge Adapter API

The new file adds only:

```go
var (
    ErrInvalidPiRPCBridgeAdapter = errors.New("invalid Pi RPC bridge adapter")
    ErrPiRPCProtocol             = errors.New("Pi RPC protocol failed")
    ErrPiRPCOutputTooLarge       = errors.New("Pi RPC output too large")
    ErrPiRPCCleanup              = errors.New("Pi RPC cleanup failed")
)

type PiRPCBridgeAdapterConfig struct {
    Execution         PiExecutionAdapterConfig
    ProviderID        string
    ModelID           string
    BaseURL           string
    MaxAssistantBytes int
}

func NewPiRPCBridgeAdapter(
    PiRPCBridgeAdapterConfig,
) (supervisor.RuntimeAdapter, error)
```

`Execution.Arguments` must be empty because the adapter owns the exact Pi
invocation. All existing executable, search-path, Runtime ID, clock, random,
cancel-grace, private-directory, and binding checks are reused without
weakening. `ProviderID` is exactly `loom-local`; `ModelID` is exactly
`qwen2.5-coder-1.5b-instruct-q4-k-m`; `BaseURL` is an exact
`http://127.0.0.1:<port>/v1` URL with decimal port 1024–65535 and no user-info,
query, fragment, hostname alias, or trailing slash. `MaxAssistantBytes` is
1–65536 and is frozen to 16384 for the live gate.

The adapter returns:

```text
AdapterType()       = "pi-cli"
RuntimeInstanceID() = frozen installed Runtime ID
```

It does not modify the accepted direct-Bridge adapter.

## Closed dispatch payload

The adapter accepts only compact valid JSON with exactly:

```json
{
  "schema_version": 1,
  "kind": "pi_rpc_prompt",
  "prompt": "<bounded task>"
}
```

Duplicate or unknown keys, missing fields, non-canonical schema/kind, trailing
JSON, invalid UTF-8, NUL, leading slash command, and prompt outside 1–8192 bytes
fail `ErrPiRPCProtocol` before process start. Prompt controls may contain only
newline and tab. The exact Grant token and strings shaped like environment
assignment or credential material are rejected. The payload contains no model,
Provider URL, tool, session, retry, extension, or permission selector.

## Exact isolated Pi invocation

For each request the adapter creates only these private paths under the
Supervisor-provided 0700 `HomePath`:

```text
.pi/agent/             mode 0700
.pi/sessions/          mode 0700
.pi/agent/models.json  mode 0600
```

`models.json` is compact JSON with one `loom-local` provider:

```json
{
  "providers": {
    "loom-local": {
      "baseUrl": "http://127.0.0.1:<port>/v1",
      "api": "openai-completions",
      "apiKey": "loom-local-offline",
      "compat": {
        "supportsDeveloperRole": false,
        "supportsReasoningEffort": false
      },
      "models": [
        {
          "id": "qwen2.5-coder-1.5b-instruct-q4-k-m",
          "name": "Loom Local Qwen 2.5 Coder 1.5B",
          "reasoning": false,
          "input": ["text"],
          "contextWindow": 4096,
          "maxTokens": 256,
          "cost": {
            "input": 0,
            "output": 0,
            "cacheRead": 0,
            "cacheWrite": 0
          }
        }
      ]
    }
  }
}
```

`loom-local-offline` is a documented non-secret placeholder and is never used
as a Loom credential.

The executable receives exactly:

```text
--mode rpc
--offline
--no-approve
--no-session
--no-tools
--no-extensions
--no-skills
--no-prompt-templates
--no-themes
--no-context-files
--provider loom-local
--model loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m
--thinking off
--system-prompt <frozen bounded no-tool final-gate system prompt>
```

The system prompt is a source constant at most 1024 bytes. It says to answer
only the supplied bounded task, use no tools, expose no hidden reasoning, avoid
credentials/paths, and return concise plain text.

The clean environment contains only:

```text
HOME, TMPDIR, PATH, LANG=C.UTF-8, LC_ALL=C.UTF-8,
PI_CODING_AGENT_DIR, PI_CODING_AGENT_SESSION_DIR,
PI_OFFLINE=1, PI_SKIP_VERSION_CHECK=1, PI_TELEMETRY=0,
NO_COLOR=1, TERM=dumb, NO_PROXY=127.0.0.1, no_proxy=127.0.0.1
```

`LOOM_AGENT_GRANT`, Provider credentials, inherited proxy variables, user HOME,
shell configuration, and arbitrary environment variables are absent.

## RPC-to-Bridge state machine

The adapter writes one LF-delimited command:

```json
{"id":"<dispatch-message-id>","type":"prompt","message":"<prompt>"}
```

It uses strict LF framing, rejects CR and Unicode line separators as record
delimiters, limits a Pi line to 1 MiB, total stdout to 8 MiB, stderr to 256 KiB,
at most 1024 Pi records, at most 1024 constructed Bridge Frames, and assistant
text to the configured 16 KiB.

The only accepted Pi records for the one-prompt/no-tool gate are:

- one successful correlated `response` for command `prompt`;
- `agent_start`;
- one `turn_start` and one `turn_end`;
- exact user `message_start`/`message_end` carrying the dispatch prompt;
- one assistant `message_start`, zero or more bounded assistant
  `message_update` records, and one assistant `message_end`;
- only `message_update.assistantMessageEvent` values `text_start`,
  `text_delta`, and `text_end`;
- `agent_end` with `willRetry=false`; and
- one final `agent_settled`.

The correlated successful prompt response constructs the exact Bridge Ack for
the dispatch. Locked Pi 0.82.1 `pi-agent-core` maps the Provider `start` event to
top-level assistant `message_start`, forwards only text/thinking/tool provider
updates as top-level `message_update`, and consumes Provider `done`/`error`
into top-level assistant `message_end`; RPC mode then forwards those
AgentSession events unchanged. Therefore invented nested `start`, `done`, or
`error` updates are rejected.

The accepted assistant lifecycle is one initial empty assistant
`message_start`, followed by one text block (`text_start`, one or more
`text_delta`, `text_end`), followed by one final assistant `message_end`.
`contentIndex` is zero throughout. Every update's top-level `message` must be
semantically identical to its nested `partial`; role/API/provider/model,
timestamp/response identity, usage shape, and accumulated text must remain
consistent with the locked transcript. `text_end.content` and final message
content must equal the concatenated deltas byte-for-byte. The final message must
have exact `stopReason:"stop"` and no error/diagnostic field. `length`,
`toolUse`, `aborted`, `error`, a missing reason, or any other reason fails
closed.

Each non-empty valid text delta is split on UTF-8 boundaries into at most
2048-byte payloads:

```json
{"delta":"<text>"}
```

Each becomes a Bridge Event and is passed synchronously to the existing
`FrameSink`. Text start/end lifecycle does not duplicate content. At settled,
the adapter constructs one Evidence Frame containing only:

```json
{
  "kind": "assistant_text_digest",
  "sha256": "<lowercase digest>",
  "bytes": 123
}
```

and then one Bridge Result:

```json
{"status":"succeeded","reason":""}
```

Every Bridge Frame uses the exact Dispatch correlation, WorkItem, Run,
generation, Runtime, and sender binding; monotonically increasing sequence; a
new canonical UUID; and the injected UTC clock. Every Frame crosses
`request.FrameSink.AcceptFrame` before entering `AdapterResult`.

Empty assistant text, missing/duplicate/out-of-order assistant start/end,
non-`stop` completion, mismatched `text_end`, Pi error content, unsuccessful or
uncorrelated response, `willRetry=true`, missing `agent_settled`, duplicate or
unknown JSON keys, unknown/out-of-order records, multiple prompts, `bash`,
tool-call/tool-execution, extension UI, extension error, queue, retry,
compaction, image, thinking/reasoning publication, oversized data, or any
Grant-token occurrence fails closed. No partial `AdapterResult` is returned.

On context cancellation the adapter sends one correlated Pi `abort`, accepts
only its bounded successful response, closes stdin, and terminates the whole
process group within the accepted 1–5 second grace. Protocol or process failure
also closes stdin and terminates the group. Cleanup errors join the primary
typed error. No Pi session is resumed or persisted.

## Controlled local model server

The source lock is `.loom-evidence/phase1-final-live-gate/source-lock.json`.
Before the local compatibility commit, only read-only verification of these
official metadata and documentation sources is allowed. After Contract Review
PASS, behavioral RED/GREEN, the full verification matrix, fresh independent
Implementation Review PASS, and the local compatibility commit, external
materialization may download:

- official llama.cpp release `b10107`, commit
  `c0bc8591e8815c63cb01dd3f051a8b0df02501c9`, macOS arm64 asset SHA-256
  `b9554ab4c9f6e91199f48387cb4ab27466fb1d724881f81463ef03f6370cfa32`;
- official Qwen model revision
  `2ab9f8f42af02fc212effaef7c4850c885e965f4`, file
  `qwen2.5-coder-1.5b-instruct-q4_k_m.gguf`, SHA-256
  `cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`,
  Apache-2.0.

The pinned llama.cpp `b10107` server reference is SHA-256
`5032018d3c81b226378bce313c0050dfea4fde8407a436cfeb57f036622cca22`.
It explicitly documents every frozen server flag below, including single-model
`--model`, loopback host/port, offline mode, disabled UI/Agent/MCP proxy,
bounded context/output/parallelism, and cleanup-related timeouts. This gate
deliberately uses llama.cpp single-model OpenAI-compatible mode with Pi's static
private `models.json`; it does not use Pi's `/llama` multi-model router or
runtime model download path.

Downloads go only to the user-selected private Loom root, through temporary
files, with digest verification before atomic rename. A mismatch is quarantined
and stops. No installer script, Homebrew mutation, PATH change, credential, or
runtime auto-download is allowed.

The local server API adds:

```go
var (
    ErrInvalidPiLocalModel       = errors.New("invalid Pi local model")
    ErrPiLocalModelBindingChanged = errors.New("Pi local model binding changed")
    ErrPiLocalModelProcess       = errors.New("Pi local model process failed")
    ErrPiLocalModelHealth        = errors.New("Pi local model health failed")
    ErrPiLocalModelCleanup       = errors.New("Pi local model cleanup failed")
)

type PiLocalModelServerConfig struct {
    PrivateRoot   string
    ExecutablePath string
    ModelPath      string
    Host           string
    Port           int
    StartupTimeout time.Duration
    CancelGrace    time.Duration
}

type PiLocalModelServer interface {
    BaseURL() string
    Close(context.Context) error
}

type PiLocalModelServerBinding struct {
    ExecutableSHA256 string
    ModelSHA256      string
}

func InspectPiLocalModelServerBinding(
    PiLocalModelServerConfig,
) (PiLocalModelServerBinding, error)

func StartPiLocalModelServer(
    context.Context,
    PiLocalModelServerConfig,
) (PiLocalModelServer, error)
```

The live values are the explicit selected 0700 `PrivateRoot`, host `127.0.0.1`,
port `18427`, startup timeout 60 seconds, and cancel grace 3 seconds.
`InspectPiLocalModelServerBinding` is read-only and returns only the validated
lowercase digests; it starts no process and creates no file. `Start` uses the
same internal binding primitive and additionally proves the port is not already
listening.

The private root and both leaves are absolute and clean. Lexical `Rel` and
resolved-path checks must keep each leaf below the exact root. Every root/leaf
path component is walked without following the component being inspected:
pre-root ancestors may not be group/other-writable, while the private root and
every descendant directory must be current-user-owned, non-symlink directories
with exact mode 0700. The llama.cpp executable leaf must be a
current-user-owned regular non-symlink file with exact mode 0700. The model,
resolved manifest, and every other non-executable private file must be
current-user-owned regular non-symlink files with exact mode 0600. Leaf opens
use `O_NOFOLLOW` on Unix;
the opened descriptor is `fstat`-matched to the pathname snapshot and hashed.
Device, inode, UID, mode, size, and digest for the directory chain and leaves
are revalidated immediately before start. The model is at most 4 GiB and
exactly matches the source-lock digest.

The final OS process launch and llama.cpp model handoff necessarily reopen
pathname strings after the last validation. This amendment does not claim a
perfect descriptor-bound `fexecve`/model API that Go/llama.cpp does not expose.
The residual same-user micro-race is bounded by a current-user-only 0700 tree,
two full-chain validations, no resident service, and immediate process-group
cleanup. Any pre-start drift fails closed.

The server receives exactly:

```text
--model <bound-private-model-path>
--alias qwen2.5-coder-1.5b-instruct-q4-k-m
--host 127.0.0.1
--port 18427
--ctx-size 4096
--parallel 1
--threads 4
--n-predict 256
--gpu-layers all
--offline
--no-webui
--no-agent
--no-ui-mcp-proxy
--no-context-shift
--no-cache-prompt
--sse-ping-interval -1
--timeout 30
```

It receives a clean environment with only private HOME/TMPDIR, PATH, locale,
`NO_PROXY=127.0.0.1`, and no credential/Hugging Face/proxy variables. It must
reach exact `GET /health` status 200 with `{"status":"ok"}` before returning.
Other status/body, early exit, timeout, binding change, or port conflict stops
and cleans the process group. `Close` is idempotent, bounded, and proves the
listener and process group are gone. The server never becomes a daemon.

## Resolved final live gate

After implementation Review PASS, the opt-in live test may run once with:

```text
installed_runtime_instance_id = runtime.pi.earendil-works.0.82.1
private_source_root            = user-selected private Loom final-gate root
approval_decision              = approved
live_execution_authorization   = local execution authorized
```

It first calls the read-only shared local-model binding inspector and then
creates a sanitized resolved manifest under this amendment evidence tree before
metadata discovery, model startup, or Provider invocation. The historical
S5-W2 manifest remains unchanged. The resolved manifest binds:

- Runtime adapter type `pi-cli`;
- exact Pi source lock plus the inspector-validated
  llama.cpp/model digests and explicit private-root containment;
- one node, one attempt, one Main Agent, no SubAgent;
- one no-tool prompt asking for a concise plain-text correction to a bounded
  in-memory Go snippet;
- no file write, shell, network, credential, publication, external action, or
  daemon;
- 120-second Run deadline and 180-second whole-canary deadline.

The opt-in test must prove:

1. installed Pi metadata discovery returns the exact Runtime ID online without
   leaking diagnostic paths;
2. llama.cpp is loopback-only, healthy, digest-bound, and non-resident;
3. the discovered `pi-cli` Runtime, RuntimeProfile, RPC Adapter, claimed Run,
   generation, and AgentGrant bind exactly;
4. prompt response becomes one authorized Bridge Ack;
5. at least one authorized tentative text Event is captured before terminal;
6. one digest-only Evidence Frame and one successful Result reconcile exactly;
7. Supervisor commits one terminal Run, revokes one Grant, and preserves private
   attempt evidence without duplicate Run/Grant/Evidence;
8. no workspace/source file changes, credential material, raw Grant, hidden
   reasoning, tool output, absolute private path, or per-token Journal fact
   appears;
9. cancel/timeout cleanup is separately exercised hermetically; the live success
   path leaves no Pi or llama.cpp child/listener; and
10. all private directories are exact mode 0700, the llama.cpp executable is
    exact mode 0700, the model/manifest/other non-executable private files are
    exact mode 0600, and the repository worktree differs only by the frozen
    Candidate plus pre-existing user dirt.

Model output is untrusted and tentative until the existing terminal/Evidence
path accepts it. Content quality beyond valid bounded non-empty text remains a
final human review item. The assistant cannot supply `final_review_signoff`.

Any live failure stops immediately, preserves sanitized evidence, records
`VERDICT: FAIL`, and does not retry or fall back.

## Mandatory behavioral RED

Before implementation, tests are added and:

```text
go test ./internal/runtime ./internal/runtime/piadapter \
  -run 'Pi0821|PiRPCBridge|PiLocalModel' -count=1
```

must fail behaviorally on the current product code. A compile failure alone is
insufficient; temporary test-local seams may expose at least:

- rejection of the exact valid Pi 0.82.1 no-model diagnostic;
- inability to translate a valid correlated Pi RPC transcript;
- acceptance of an unknown/tool/retry/hidden-reasoning/oversized transcript; or
- failure to contain a fake local model process.

The opt-in live test must remain skipped throughout RED and normal GREEN.

## Mandatory verification

After minimal GREEN:

```text
go test ./internal/runtime ./internal/runtime/piadapter -count=1
go test ./internal/supervisor ./internal/app ./internal/api \
  ./internal/authorization ./internal/evidence ./internal/journal \
  ./internal/projection ./internal/teams ./internal/work -count=1
go test -race -count=10 ./internal/runtime ./internal/runtime/piadapter
go test ./...
go test -race ./...
go vet ./...
gofmt -d internal/runtime/pi_probe.go internal/runtime/pi_probe_test.go \
  internal/runtime/piadapter/rpc_bridge_adapter.go \
  internal/runtime/piadapter/rpc_bridge_adapter_test.go \
  internal/runtime/piadapter/local_model_server.go \
  internal/runtime/piadapter/local_model_server_test.go \
  internal/runtime/piadapter/local_model_process_unix.go \
  internal/runtime/piadapter/local_model_process_other.go \
  internal/app/final_live_gate_live_test.go
git diff --check
GOOS=windows GOARCH=amd64 go test -exec=true \
  ./internal/runtime ./internal/runtime/piadapter
go mod verify
```

Additional audits:

- contract-to-test traceability for every numbered guarantee;
- 100 prompt/RPC chunk-boundary permutations and fuzz/no-panic decoding;
- exact `errors.Is` matrix;
- duplicate-key, CR/U+2028/U+2029, order, correlation, binding, sequence,
  unknown-event, tool, retry, compaction, extension-UI, error, and bound cases;
- process-group cleanup with child/grandchild fixtures;
- parent-symlink, public/intermediate-mode, lexical/realpath escape,
  owner, descriptor/path mismatch, and pre-start full-chain drift fixtures;
- no inherited environment, raw Grant, credential, private path, hidden
  reasoning, or tool output;
- no write to Journal/Evidence outside accepted Supervisor authority;
- exact source-lock JSON and ADR/link checks;
- no change outside exact ownership and no staging of excluded user dirt.

A fresh independent Implementation Reviewer must return `PASS`, and Controller
must create the local compatibility commit, before external materialization or
live execution. Normal repository tests never set the live gate environment and
must report the opt-in test as skipped, not passed live.

## Commit and final-signoff boundary

After all hermetic checks and Implementation Review PASS, Controller may create
one local atomic compatibility commit containing only the exact owned Candidate
and reviewed evidence. No push, merge, rebase, reset, force, or release is
authorized.

Only after that commit may external llama.cpp/model materialization occur.
After digest verification and offline preflight, the one live canary may run.
Live evidence is recorded in the private root first; only a
sanitized report, hashes, counts, exit statuses, and `VERDICT` may be copied
into this repository. A second local evidence-only commit requires exact scope
review and must not include model output, private paths, credentials, raw Grant,
or hidden reasoning.

Phase 1 remains `READY_FOR_FINAL_USER_SIGNOFF` after a live PASS. Only the user
can provide `final_review_signoff`; the assistant must not infer or fabricate it.

## Source provenance appendix

`PHASE1-FINAL-LIVE-SOURCE-PROVENANCE-1` is a governance-only amendment at
baseline `7829423`. The user explicitly accepted that the exact staged GGUF was
transported through `hf-mirror.com` after direct official access failed.

The mirror is an untrusted byte transport only. It receives no repository,
revision, Runtime, Provider, policy, fallback, retry, or execution authority.
The staged URL used `resolve/main`; this contract therefore does not claim that
the mirror proves the frozen revision ancestry. Artifact acceptance remains
bound exclusively to the previously reviewed exact filename, byte size, GGUF
v3 header, and full SHA-256 source lock.

The exact amendment, ownership, pre-install evidence, atomic same-filesystem
rename, post-install verification, and unchanged live-gate order are frozen in
`.loom-evidence/phase1-final-live-gate/source-provenance-amendment.md`.
Fresh independent Contract Review returned `PASS`. Atomic installation,
post-install verification, the shared inspector, sanitized pre-live manifest,
and raw installed-Pi metadata process preflight passed. That raw command check
did not prove acceptance by the production metadata parser. The one authorized
canary was subsequently consumed and failed closed in production Runtime
discovery on the two-space-indented Pi 0.82.1 documentation paths. No live
authorization remains; repair and a replacement canary are `HUMAN_REQUIRED`.

## Explicit exclusions

This amendment does not add or authorize:

- S5-W3, Phase 2/3 work, or another thin WorkItem;
- a new Bridge version, Journal, StateWriter, Projection, scheduler,
  coordinator, Grant, Evidence, approval, recovery, verification, acceptance,
  or completion authority;
- Pi tool use, file editing, bash, extension, Skill, template, context-file,
  session resume/checkpoint, Provider fallback, hidden retry, or Autopilot;
- model/runtime auto-download during execution;
- remote Provider/model traffic, credentials, paid work, publication, Web/TUI,
  callback, notification, resident daemon/model service, or external effect;
- mutation of the real repository by the model;
- user final signature; or
- push, merge, rebase, reset, force, or release.

END OF FROZEN REPAIR CANDIDATE
