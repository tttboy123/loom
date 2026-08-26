# P2D-HG1 Persistent Codex Segment and Cancellation Source Evidence

Status: `SOURCE + INSTALLED VERIFIED / HG1 ACCEPTED`

Date: 2026-08-25

Parent Goal: Phase 2D

Contract: `contracts/P2D-HG1-harness-gateway-segment-session.md`

## User-visible result

The current source keeps one Codex App Server Session for an immutable Loom
Conversation Segment. Later turns in that Segment reuse the same native thread.
The conversation composer exposes Stop response while work is active; stopping
targets the exact thread and Incident, retains the user message and cancelled
Attempt, and leaves a cleanly interrupted Codex Session available for retry.

## Authority and privacy

- Session identity freezes Harness and Backend versions, Conversation, Segment,
  Workspace digest, execution-binding digest, Provider Account, credential
  revision, model, reasoning effort, immutable Segment opening Capsule digest,
  current Attempt Capsule digest and policy digest.
- Cancellation carries only thread and Incident identifiers. It contains no
  Prompt, message content, Provider body, credential or Authorization header.
- A wrong Incident cannot cancel another response. Cancellation before Gateway
  activation and cancellation after activation both settle without silently
  changing the immutable Segment binding.
- Operational diagnostics retain the complete content-free Gateway event
  envelope: event time, schema and Harness/Backend versions, global sequence,
  Session/Harness/Backend/Conversation/Segment/Workspace/Response identities,
  safe result/error classification and Incident correlation. Stored records are
  revalidated against the authoritative Gateway event contract.

## Source boundaries

- `internal/harnessgateway`: registered Gateway lifecycle, exact frozen Session
  authority, per-Session serialization, Response cancellation, versioned
  content-free events and cleanup-before-reopen ordering.
- `internal/runtime/harnessadapter`: persistent Codex App Server Session and
  bounded `turn/interrupt` cleanup that permits thread reuse after success;
  Claude Code native session creation/resume with exact returned session ID.
- `cmd/loomd`: production Codex Segment Backend, active Response index and the
  authenticated private-UDS `chat_response_cancel` route.
- `internal/api` and `internal/localipc`: cancelled Attempt projection and
  strict metadata-only request/acknowledgement contracts.
- `apps/macos`: bounded cancel request, Store lifecycle and Stop response UI.

## Verification

- `go test -p 4 ./... -count=1` passed.
- `go test -race ./internal/runtime/harnessadapter ./internal/api ./internal/localipc ./internal/harnessgateway -count=1` passed.
- `go vet ./...` passed.
- The full macOS Swift suite passed 351 tests, with 2 conditional skips and 0
  failures.
- Focused tests passed for exact product cancellation, private-UDS independent
  send/cancel, native Codex interrupt and reuse, configured product Codex
  conversation, route manifest and Swift Store/UI cancellation behavior.
- `git diff --check` passed.

The subsequent lifecycle/backend hardening delta additionally passed:

- focused Gateway, Codex adapter, daemon and ADR-0022 first-loop tests;
- cross-layer race tests for Harness Adapter, Chat API, private UDS and Gateway;
- `go vet ./...` and `git diff --check`;
- the complete `internal/localipc` package serially in 385.697 seconds.

A repository-wide `go test -p 4 ./... -count=1` attempt passed every package
except `internal/localipc`, where concurrent Swift probe builds produced one
transient UDS failure and exhausted the package's ten-minute timeout. The failed
UDS test, strict Swift/Go setup probe and full package all passed when rerun
without that parallel build pressure. This is recorded as verification resource
contention, not as a green full-run claim for the latest delta.

The latest production-reachability and diagnostic-envelope delta additionally
verifies that:

- all five built-in Harness identities preserve exact Provider Account,
  credential revision, model and Context Capsule authority through registered
  Gateway Backends;
- native-auth Claude Code is reachable through the production Conversation
  Profile, router, Gateway and responder path without a credential lease;
- an exact cancellation emits only the target Response cancellation outcome,
  while different Conversations continue to overlap;
- `go test -p 4 ./... -count=1 -timeout=15m` passes the complete repository,
  including `cmd/loomd` in 233.832 seconds and `internal/localipc` in 368.159
  seconds;
- the complete daemon test package, focused cross-layer race tests,
  `go vet ./...` and `git diff --check` pass.

The latest parallel closure additionally verifies that:

- Claude Code uses `--session-id` for the first same-Segment response and
  `--resume` for later responses while preserving the frozen Gateway Session;
- response authority schema v2 separates immutable Segment opening Capsule
  authority from each Attempt's current Capsule digest;
- a reviewed Codex-to-Loom-Native/DeepSeek transition followed by a second
  DeepSeek turn reuses the target Gateway Session while freezing an independent
  Attempt Capsule and binding digest;
- the affected Go suites, focused race tests, full vet and diff checks pass;
  the full macOS suite passes 355 tests with two conditional skips.

The reliability and installed-evidence preparation pass additionally verifies
that:

- Gateway event schema v3 preserves a non-secret Gateway Instance ID,
  Harness/Backend versions, Workspace and
  execution-binding digests, Provider Account, credential revision, model,
  reasoning effort, Segment opening Capsule, per-Response Attempt Capsule and
  governance-policy digest through daemon diagnostics and Swift's redacted
  diagnostic export;
- cancellation in the narrow window after Codex accepts `turn/start`, but
  before its response arrives, recovers the exact native turn ID, sends one
  bounded `turn/interrupt`, waits for `interrupted` and keeps the Session only
  when cleanup is complete;
- daemon startup durably reconciles orphaned plaintext and encrypted
  `dispatching` Attempts to `cancelled` before serving traffic, preserves the
  user message and Segment, and permits a later response without inventing an
  assistant reply;
- a real private-UDS acceptance crosses Local IPC, Chat API, production Gateway
  and Codex fixture, proving exact cancellation, unaffected peer progress and
  same-Session recovery;
- production composition diagnostics prove registered Gateway use for Codex,
  Claude Code, OpenCode, Pi and Loom Native without a direct responder bypass;
- the G7 source verifier accepts raw operational JSONL for local contract
  checking, while installed evidence accepts only an App diagnostic bundle
  whose embedded App/daemon build hashes match the exact installed bundle. It
  rejects cross-instance composition, duplicate sequences, incomplete
  Response lifecycles, authority drift, unknown fields and content-bearing
  fields, and produces bundle-hash-bound owner-only evidence. Historical v1/v2
  diagnostics remain readable but cannot satisfy the new G7 gate.

Verification for this pass:

- `scripts/test-phase2d-live-acceptance.sh`: PASS, including App bundle and raw
  trace fixtures plus negative privacy/authority cases.
- `go test -race ./internal/harnessgateway ./internal/runtime/harnessadapter ./internal/api ./cmd/loomd -count=1 -timeout=15m`: PASS.
- `go vet ./...`: PASS.
- Full `swift test`: 359 XCTest cases plus 20 Swift Testing cases, two
  conditional visual-export skips, zero failures.
- `go test -race ./internal/harnessgateway ./cmd/loomd -count=1 -timeout=15m`:
  PASS; `cmd/loomd` completed in 282.511 seconds.
- Controlled repository-wide
  `go test -p 4 ./... -count=1 -timeout=15m`: PASS, including `cmd/loomd` in
  403.236 seconds, `internal/localipc` in 673.519 seconds and
  `internal/runtime/piadapter` in 146.751 seconds.
- `git diff --check`: PASS.

No credential was read or modified. No external Provider request, packaging,
installation or installed-App acceptance was performed for this source slice.
Build 127 remains the installed predecessor.

## Remaining gate

HG1 remains active until a newly packaged installed App passes G7's persistent
Session, Route Transition, concurrency and exact cancellation matrix. Claude
Code proves native session-identity reuse without claiming a persistent
process; OpenCode, Pi and Loom Native own real Loom Segment lifecycle without
claiming native process/thread persistence. Future native persistence is added
only where the runtime exposes a truthful stable resumable identity.

## 2026-08-25 parallel authority and evidence closure

Status: `SOURCE VERIFIED / INSTALLED OPEN`.

The trust-domain, full Context Capsule capacity and Route Transition paths were
re-reviewed in parallel with the five-Harness Gateway. This pass closes the
remaining source-evidence gaps:

- native-auth Codex, Claude Code, OpenCode and Pi preserve exact absent
  credential authority instead of synthesizing an account, revision or policy;
- every Gateway Session, Response, event and operational diagnostic freezes the
  Segment's optional Route Transition review digest, and substitution fails
  before a Backend call;
- the reviewed Loom Native/DeepSeek target Segment carries one exact 64-hex
  review digest across opening, response and diagnostic events, while an
  initial Codex Segment carries none;
- private-UDS cancellation is exercised before the native Codex turn ID becomes
  observable, with exact interrupt, durable cancellation, peer progress and
  same-Session reuse;
- two production Codex Segment Backends overlap across two Conversations but
  cannot overlap Responses inside one Session;
- Swift validates raw Gateway v3 keys before decoding, rejects nested
  content-bearing fields, preserves explicit native empty/zero authority and
  keeps historical v1/v2 records read-compatible;
- Gateway v3 JSON explicitly serializes native empty account, revision `0` and
  empty policy fields, so absence cannot be confused with decoder defaults;
- installed G7 evidence is accepted only from the canonical
  `$HOME/Applications/Loom.app` diagnostic bundle whose embedded App and daemon
  hashes match the bundle being stamped. Raw JSONL remains source evidence only.

Verification:

- focused Gateway/API/daemon tests and ten repeated IPC/concurrency runs: PASS;
- focused Gateway/daemon race tests: PASS;
- G7 source harness including negative identity, lifecycle, authority, Capsule,
  cancellation and privacy mutations: PASS;
- full Swift suite: 363 XCTest cases, two conditional skips, plus 20 Swift
  Testing cases; zero failures;
- controlled repository-wide Go run: every non-daemon package passed; the first
  run exposed one stale Claude native-auth fixture. After correcting that
  fixture, the complete `cmd/loomd` package passed twice, including the final
  diagnostic serialization change in 154.233 seconds;
- `go vet ./...` and `git diff --check`: PASS.

No credential, network, package, installation or installed-App action was
performed. Build 127 remains the installed predecessor and G7 remains open.

## 2026-08-25 trust, capacity and Route Transition closure

Status: `SOURCE VERIFIED / INSTALLED OPEN`.

Three independent agents re-audited trust-domain confirmation, complete Context
Capsule capacity handling and Route Transition acceptance. The trust path had
no remaining production bypass: the daemon compares complete frozen authority
and recomputes the canonical v3 review under the Conversation lock before any
message, Segment, Attempt, Capsule, lease or responder mutation.

The capacity audit found one omission-authority gap. Dispatch-safe rebuilding
could previously reconstruct disclosed items without carrying the original
omission manifest. It now preserves fixed policy/access omissions,
reconstructs retrievable budget omissions, retains the exact Capacity
Projection and TokenCounter identity, and includes all admitted plus omitted
contributions in the cumulative bound before tokenization. The regression is
RED without the fix and covers non-disclosure of unsafe prior-model output.

The Route Transition review found that Go source acceptance already proved a
second target turn, but the G7 trace verifier accepted only one DeepSeek target
response. G7 now requires two completed target responses in the same Session,
independent Attempt Capsule digests and one immutable non-secret Provider
Account, credential revision, model and Route Transition review digest. Its
summary exports those exact values. Negative fixtures remove the second turn
and substitute each authority field in one event; all fail closed.

Production composition additionally proves Codex reachability through
`newProductConversationConstructionFactory` and the specialized
`backend.codex.app-server` Backend, completing equivalent factory evidence for
all five built-in Harness identities.

Verification:

- `go test -p 4 ./... -count=1 -timeout=15m`: PASS; `cmd/loomd` 209.050s,
  `internal/localipc` 335.647s and `internal/runtime/piadapter` 81.282s;
- focused Capsule/API/App/daemon tests and focused race tests: PASS;
- `scripts/test-phase2d-live-acceptance.sh`: PASS, including missing target turn
  and Provider Account/credential revision/model substitution negatives;
- full macOS suite: 364 XCTest cases, two conditional skips, plus 20 Swift
  Testing cases; zero failures;
- `go vet ./...` and `git diff --check`: PASS.

No credential, network, package, installation or installed-App action was
performed. Build 127 remains the installed predecessor and installed G7 remains
open.

## 2026-08-25 bounded Session opening completion audit

Status: `SOURCE VERIFIED / INSTALLED OPEN`.

A requirement-by-requirement ADR-0022 audit confirmed that production Chat API
construction receives only the unified Harness Gateway. The legacy Provider
router remains an internal governed Backend executor and is not injected as a
direct production responder. All five built-in Harnesses are registered through
the closed production registry.

The audit found one reliability gap: coalesced Session opening intentionally
outlived individual Response cancellation, but had no Gateway-owned deadline.
A Backend that never completed opening could retain a configured-Harness slot
until daemon shutdown. `harnessgateway.Config` now owns an independent Session
opening timeout; production uses 30 seconds. Deadline expiry cancels the exact
Backend opener, records terminal Session failure and releases the slot. If an
uncooperative Backend returns a Session after the deadline, Gateway rejects the
late success, completes exactly-once cleanup and waits for that cleanup before
allowing the same Session identity to reopen.

Additional direct contract tests prove that Configured Harness authority is
copied at registry construction and resolution, and that replaying a completed
Response ID is rejected before a second Backend call.

Focused verification:

- opening timeout, coalesced opening, close-deadline and slot-limit tests,
  repeated 20 times: PASS;
- the same lifecycle set under the race detector, repeated five times: PASS;
- full `internal/harnessgateway`: PASS;
- five-Harness production composition, ADR-0022 Segment switching and private
  IPC cancellation tests, repeated five times: PASS;
- focused Gateway/daemon race tests: PASS.

Full verification retained the failed-first history instead of replacing it
with only the final pass:

- the first repository-wide `go test -p 4 ./... -count=1 -timeout=15m` run
  passed every package except one transient `internal/localipc` UDS assertion
  under concurrent build pressure;
- the exact failed test then passed 20 repeated runs, and the complete
  `internal/localipc` package passed serially in 240.500 seconds;
- the controlled repository-wide
  `go test -p 2 ./... -count=1 -timeout=15m` rerun passed every package,
  including `cmd/loomd` in 199.682 seconds, `internal/localipc` in 270.873
  seconds and `internal/runtime/piadapter` in 69.219 seconds;
- `swift test --package-path apps/macos`: PASS, 364 XCTest cases with two
  conditional skips plus 20 Swift Testing cases, zero failures;
- `scripts/test-phase2d-live-acceptance.sh`: PASS;
- `go vet ./...` and `git diff --check`: PASS.

No credential, network, package, installation or installed-App action was
performed. Build 127 remains the installed predecessor and installed G7 remains
open.

## 2026-08-25 installed-evidence closure audit

Status: `SOURCE VERIFIED / INSTALLED OPEN`.

The final source audit found three ways the installed G7 boundary could be
weaker or harder to execute than the product contract:

- the runbook and wrapper usage still suggested raw daemon JSONL could create
  installed evidence even though the contract required the App export;
- the App diagnostic export retained only the last 100 operational events, so
  ordinary diagnostics could displace the Session opening events needed by a
  complete matrix;
- the verifier paired Codex and DeepSeek Segments by identity without proving
  the two source turns completed before the target Session opened, and persisted
  evidence later rechecked only a small subset of the generated summary.

Installed evidence now accepts only a complete, exact
`LocalDiagnosticBundlePreview` with matching App/daemon hashes. Top-level and
nested binary, console, Provider, Profile and Agent metadata use closed shapes;
raw JSONL, missing fields and unknown metadata cannot be stamped as installed
G7. The App retains a bounded 512-event privacy-safe export window within the
existing four-megabyte harness limit.

The G7 summary now freezes completed Codex sequences, the target Session opening
sequence and completion time. A valid transition requires at least two source
completions before target opening. The persisted gate reader revalidates the
complete versioned route, overlap and cancellation summary, so a previous weak
summary cannot remain accepted merely because it contains `matrix: pass`.

RED-first evidence:

- nested unknown binary/Provider metadata and an incomplete App export shape
  were accepted before the closed bundle validator;
- a 150-event Swift diagnostic fixture exported only 100 events before the
  bounded-window change;
- a target Session reordered ahead of the two source completions satisfied the
  previous route relation;
- a persisted summary without source completion sequences satisfied the old
  gate-document check.

Verification:

- `swift test --package-path apps/macos`: PASS, 365 XCTest cases with two
  conditional skips plus 20 Swift Testing cases, zero failures;
- `scripts/test-phase2d-live-acceptance.sh`, repeated 20 times: PASS;
- `go vet ./...` and `git diff --check`: PASS.

No credential, network, package, installation or installed-App action was
performed. Build 127 remains the installed predecessor and installed G7 remains
open.

## 2026-08-25 Build 128 installation preflight

Status: `SOURCE CANDIDATE / NOT PACKAGED / INSTALLED OPEN`.

The source App version advances from installed Build 127 to Loom `0.5.3` Build
128. The installed-live runbook previously instructed operators to use
`/Applications/Loom.app`, while the installer requires an owner-controlled
parent and G7 binds only `$HOME/Applications/Loom.app`. Following the old
instruction would therefore fail installation or produce a bundle that could
never satisfy G7.

The runbook now uses the canonical user App path exclusively. The source harness
fails if the instruction regresses to `/Applications` or if the HG1 candidate
does not advance beyond Build 127. The fixture passes without building,
installing, launching or reading any credential. A read-only check confirms the
current installed bundle remains Build 127 with its previously recorded App and
daemon hashes.

## 2026-08-25 Build 136 installed G7 acceptance

Status: `INSTALLED PASS / HG1 ACCEPTED`.

The canonical user installation at `$HOME/Applications/Loom.app` is Loom
`0.5.3` Build 136. Its App and bundled daemon signatures are valid. The App
exported a complete privacy-safe diagnostic bundle and the closed G7 verifier
accepted the installed matrix:

- two Codex responses complete in one immutable source Segment Session before
  the reviewed Loom Native/DeepSeek target Segment opens;
- two DeepSeek responses reuse the exact target Segment Session while freezing
  distinct Attempt Capsule digests and unchanged Provider Account, credential
  revision, model and Route Transition review authority;
- a Response in another Conversation overlaps the cancelled Response;
- cancellation binds the exact Incident, Response and Attempt, and the same
  Session completes a later Response after cancellation;
- App export and machine verification use the production Codex backend
  `backend.codex.app-server`, freeze the Profile ID and reject the historical
  alias or any authority drift.

The installed bundle, verifier summary and owner-only evidence document contain
no credential, credential reference, Prompt, transcript, Provider response or
Workspace path. Verification and hashes are recorded in
`P2D-HG1-build136-installed-g7.md`.
