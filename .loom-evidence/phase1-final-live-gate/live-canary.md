# Final Live Gate Controlled Canary

Date: 2026-07-27
Status: `FAIL — HUMAN_REQUIRED — NO SECOND REPLACEMENT`

Attempt inventory:

- original controlled canary: consumed, `FAIL`;
- explicitly authorized replacement canary: consumed, `FAIL`; and
- remaining live attempts: `0`.

The original failure record follows. The reviewed metadata repair and
replacement failure are recorded in `replacement-live-canary.md`.

## Authorized attempt

- Attempt count: `1`
- Approval decision: `approved`
- Execution authorization: `controlled local`
- Runtime: `runtime.pi.earendil-works.0.82.1`
- Adapter: `pi-cli`
- Resolved pre-live manifest: `PASS`
- Source provenance amendment: Contract Review `PASS`
- Model and llama.cpp binding: pre-live `PASS`

The exact opt-in test ran once:

```text
go test ./internal/app \
  -run '^TestFinalLiveGatePiRPCOfflineModel$' \
  -count=1 -v
```

Result:

```text
=== RUN   TestFinalLiveGatePiRPCOfflineModel
    final_live_gate_live_test.go:132: installed Pi Runtime discovery failed
--- FAIL: TestFinalLiveGatePiRPCOfflineModel (2.47s)
FAIL
```

## Exact failure boundary

The canary stopped in installed Pi metadata discovery before local
llama.cpp startup, Pi RPC mode, WorkItem/Run/Grant creation, model invocation,
Bridge Frames, Journal terminal facts, or Evidence acceptance.

Read-only diagnosis separated the exact metadata channels:

```text
version exit             = 0
version stdout bytes     = 7
version stderr bytes     = 0
version stdout           = 0.82.1
model-list exit          = 0
model-list stdout bytes  = 337
model-list stderr bytes  = 0
model-list physical lines = 3
```

The first model-list line exactly matched the frozen Pi 0.82.1 diagnostic.
The two documentation-path lines had the correct ordered `providers.md` and
`models.md` basenames and one common absolute parent, but each line began with
exactly two ASCII spaces.

No absolute diagnostic path is copied into this evidence.

The installed locked Pi source confirms that this is its actual 0.82.1
format:

```text
dist/core/auth-guidance.js:7  `  ${join(getDocsPath(), "providers.md")}`
dist/core/auth-guidance.js:8  `  ${join(getDocsPath(), "models.md")}`
```

The accepted compatibility contract and `parsePiModels` intentionally reject
leading/trailing boundary whitespace. Therefore discovery failed closed with
`ErrInvalidPiMetadataOutput`. The defect is a contract/product compatibility
miss: the reviewed parser modeled unindented paths while installed Pi 0.82.1
emits two-space-indented paths.

## Cleanup and non-effects

Control returned at `final_live_gate_live_test.go:132`. The
`StartPiLocalModelServer` call begins later at line 135, and Pi RPC adapter
construction/execution occurs later still. Therefore the failed control path
did not start llama-server or Pi RPC.

The Controller ran these read-only checks immediately after the failed `go
test` returned:

```text
lsof -nP -iTCP:18427 -sTCP:LISTEN
```

Result: no listener row.

```text
pgrep -alf '/llama-server|pi-coding-agent.*--mode rpc|runtime.pi.earendil-works.0.82.1'
```

Result: the only returned row was the invoking audit shell whose own command
line contained the search expression; no llama-server, Pi RPC child, or bound
Runtime process row was present.

The audit command output was obtained in the same Controller turn immediately
after failure but was not assigned a cryptographic or external timestamp. A
fresh independent evidence Reviewer later repeated the current listener/process
check and also found none. This evidence claims the observed same-turn and
current cleanup state, not an independently timestamped historical snapshot.

Together with the executed control-flow boundary:

- port `127.0.0.1:18427` had no listener;
- no llama-server or Pi RPC process remained;
- no model request or generated output occurred;
- no Run, Grant, Bridge Frame, Journal terminal, or Evidence was created;
- the resolved manifest remained a private `0600` sanitized file;
- the installed GGUF remained exact size `1117320768`, mode `0600`, and SHA-256
  `cc324af070c2ecbfd324a30884d2f951a7ff756aba85cb811a6ec436933bb046`;
- no credential, private diagnostic path, raw Grant, hidden reasoning, or model
  output entered repository evidence; and
- excluded user-owned worktree dirt remained unstaged.

## Stop condition

The frozen live contract says that any live failure stops immediately with no
retry, fallback, alternate model, alternate source, or second attempt. That
stop condition is active.

Any parser correction or second controlled canary now requires new explicit
user authorization for a bounded Final Live Gate Metadata Indentation Repair
and a one-time replacement canary. It must receive fresh Contract Review,
behavioral RED/GREEN, the applicable verification matrix, and fresh
Implementation Review before another live execution.

## Evidence Review 1

Fresh independent read-only evidence review returned:

- scope containment: `PASS`;
- root-cause accuracy: `PASS`;
- sanitization and no-retry boundary: `PASS`; and
- cleanup evidence accuracy: `FAIL`.

The Reviewer found no product, scope, privacy, or residual-process defect. Its
single finding was that the original cleanup bullets did not include the
immediate audit commands/results or disclose that no independent timestamp had
been captured. The bounded evidence repair above adds those exact facts and
does not run another canary, start a process, or change product code.

Fresh Evidence Review 2 is required.

## Evidence Review 2

Fresh independent read-only Evidence Review 2 confirmed:

- the repaired cleanup evidence closes Review 1's finding in substance;
- the failure boundary and two-space-indentation root cause are accurate;
- sanitization and the no-retry boundary remain intact; and
- evidence status consistency is `FAIL`.

The sole blocking finding was stale text in `contract.md`: it still said the
model was not installed and that one canary remained authorized, while the
source lock and this report correctly recorded installation and the consumed,
failed attempt.

Bounded Evidence Repair 2 changes governance evidence only. It synchronizes the
contract to `installed`, `attempt consumed`, `FAIL — HUMAN_REQUIRED — NO RETRY`
and clarifies that the earlier raw CLI process check did not prove production
parser acceptance. It also applies that distinction to the provenance and
private materialization status. It does not change product code, run a test,
start a process, or authorize repair/retry.

Fresh Evidence Review 3 is required.

## Evidence Review 3

Fresh independent read-only Evidence Review 3 returned `PASS` with no blocking
findings.

The Reviewer confirmed that Bounded Evidence Repair 2 closes the stale-status
finding and that all reviewed evidence now consistently records:

- the exact model is installed under the accepted provenance exception;
- the single live authorization was consumed;
- production Runtime discovery failed before model-server or Pi RPC startup;
- Pi 0.82.1's two-space-indented documentation paths are the bounded root
  cause;
- cleanup evidence is accurate within its disclosed timestamp limitation;
- sanitization and excluded-dirt scope remain intact; and
- no retry or replacement canary is authorized.

This evidence-review `PASS` validates the accuracy and containment of the
failure record. It does not change the live canary result or authorize another
attempt.

VERDICT: FAIL
