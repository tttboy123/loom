# P7-HT2 source verification

Status: `ACCEPTED / BUILD 324 SOURCE AND INSTALLED LIVE GREEN / CLAUDE LIVE N/A`

Date: 2026-08-31

## Accepted source boundary

- The built-in Registry admits exactly 28 versioned tools: 13 bounded
  metadata reads and 15 mutation previews.
- Codex, OpenCode, Claude Code, Pi and Loom Native expose the same frozen
  Registry and preserve the same Proposal authority boundary.
- Proposal schema v2 binds Route, Workspace, Registry digest, Segment,
  Attempt, Incident, target content, expiry and proposal digest.
- Every terminal schema-v2 decision produces an immutable receipt binding the
  exact Proposal identity and digest, decision, decision time and Incident ID.
  Historical confirmed Proposals without a receipt remain readable but cannot
  execute.
- A Segment retains its frozen opening Execution Binding while each later Turn
  validates the current Attempt's Context Capsule digest. This keeps immutable
  Segment authority without rejecting legitimate second and later Turns.
- Legacy v1 Proposals remain readable and cancellable but cannot be confirmed
  or executed.
- The model has no confirm, cancel, execute, credential, policy, Journal,
  StateWriter or terminal Run/Attempt tool. Stop remains a direct local user
  control rather than a model-facing tool.
- The macOS client uses one strict Proposal review surface for alignment and 14
  generic action types, then opens the existing governed product review only
  after daemon confirmation.
- The Pi local Route pins the official llama.cpp release archive, derives and
  binds its complete Runtime tree, pins the Qwen model and revalidates every
  exact file identity immediately before process start. A changed sibling
  dylib, extra file, unsafe link/path, symlink or hard link fails closed.
- OpenAI-compatible and Anthropic-style tool loops repair malformed arguments
  within a strict bound, force a fresh selection after exhaustion and reject a
  Tool omitted from the current least-privilege round. MiniMax's documented
  ability to repeat a previously named Tool therefore cannot expand authority.
- Read-only Gateway calls receive one bounded retry for a transient
  `gateway_unavailable`; Proposal effects never auto-retry.
- Installed RoundTable action acceptance uses an explicit exact Mission/Team
  anchor and synthetic current Sessions. Concluded historical Sessions cannot
  satisfy Pause, Steer, Retry, Skip or Replace targeting.

## Verification

The final repository-wide Go run passed under default package concurrency:

```text
go test ./... -count=1
PASS
cmd/loomd                                       87.442s
internal/localipc                               61.643s
all packages green
```

Static and focused race gates passed:

```text
go vet ./...
PASS

go test -race ./internal/controltool ./internal/api ./internal/roundtable \
  ./internal/harnessgateway ./internal/provider \
  ./internal/runtime/harnessadapter
PASS

go test -race ./cmd/loomd -run '<Phase 7 production composition matrix>'
PASS

go test -race ./internal/app ./cmd/loomd -run '<Pi executable Route gate>'
PASS

go test -race ./internal/runtime/piadapter -run '<Pi local digest gate>'
PASS

go test -race ./internal/runtime/piadapter -run \
  'TestPiLocal(RuntimeArchive|ModelBindingInspector|ModelServer|ModelConfig)' \
  -count=1 -timeout=10m
PASS

go test -race ./cmd/loomd -run \
  'Test(ProductSharedLocalModel|ProductPiConversationDoesNotStartModelWithoutBoundPi|RunAcceptsLocalModelCatalogOnlyAsCompleteTuple|AppendLocalAppPiModelArgs)' \
  -count=1 -timeout=10m
PASS

go test -race ./internal/controltool ./internal/api ./cmd/loomd \
  -run 'Test(ProposalDecisionReceipt|Conversation(Control|Action|Governance|Mission)|ConfirmedGovernance|ProductChatControl|ProductOperationalDiagnosticsRecordsContentFreeControlDecision|LivePhase7ControlToolsE2E|SelectPhase7LiveProfiles)' \
  -count=1 -timeout=20m
PASS
```

The complete macOS source suite passed:

```text
swift test --package-path apps/macos
496 XCTest cases, 2 conditional skips, 0 failures
21 strict Swift Testing contract cases, 0 failures
```

`git diff --check` passed.

The bounded installed live runner compiles, skips without explicit
authorization and passes its deterministic five-Harness selector contract:

```text
go test ./cmd/loomd -run \
  'Test(LivePhase7ControlToolsE2E|SelectPhase7LiveProfilesRequiresFiveDistinctExecutableHarnesses)$' \
  -count=1
PASS

go test ./cmd/loomd -count=1
PASS (247.200s)

go vet ./cmd/loomd
PASS
```

The runner's installed identity is fail closed on the exact build, strict code
signature, App executable SHA-256 and bundled daemon SHA-256. Its managed
restart loop tolerates the private Socket directory appearing asynchronously,
revalidates the same bundle identity after restart and rejects duplicate,
path-confused or pseudo-Socket daemon process matches. The new pure contracts
and focused race gate pass.

The no-model, no-restart identity preflight passed against installed Build 264:

```text
LOOM_PHASE7_IDENTITY_PREFLIGHT=1 \
LOOM_PHASE7_EXPECTED_BUILD=264 \
LOOM_PHASE7_EXPECTED_APP_SHA256=9db2b30f8091987ce5e9c5c7bdf7c0d9cae983263691aa8ebbbbc3308bedec75 \
LOOM_PHASE7_EXPECTED_DAEMON_SHA256=b841084fb63e04b635918004ac4e12beec46afc7da01288bfa330b11d7aa99a2 \
go test ./cmd/loomd -run '^TestPhase7InstalledIdentityPreflight$' -count=1 -v
PASS (build=264, daemon PID exact, providers=24, runtimes=7, profiles=7)
```

These commands do not count as installed live acceptance because the paid-call
and App-restart authorization gates remained unset.

## Build 313 installed live result

The Build 313 identity preflight passed with 24 Providers, seven Runtimes, six
executable Conversation Profiles and one explicit exact governance anchor. The
installed partial gate then made authorized real model calls and passed in
431.45 seconds for Codex, OpenCode, Pi and Loom Native. Across the complete
run, every one of the 28 Registry Tools was selected in an exact single-Tool
turn; confirmation, cancellation, expiry, replay rejection and encrypted App
restart restoration passed. The three synthetic RoundTable fixture streams
ended with `RoundtableConcluded`.

The formal five-Runtime gate failed closed before dispatch with:

```text
installed Phase 7 matrix missing executable claude-code profile
```

No credential was read or changed. The App and managed daemon remained running
after restart. Exact installed identities:

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `e2300168d2a0eaef7ae5f351f03c1cbf3e1409d82ec194166cf71988f0e08fa0` |
| `Contents/Library/Helpers/loomd` | `9c1b24a46136a65ef52e1150ed6ea5192ee4ac520f77befde84be848924c9964` |
| `Contents/Info.plist` | `bd4fbf6003842db4343833a1bcf51162c1446e5cf0deaf5432b01ce26906690e` |

Detailed installed evidence is
[`P7-HT2-installed-build313.md`](./P7-HT2-installed-build313.md).

## Build 316 installed hardening result

Build 316 preserves the Build 313 paid result while closing the installed
Claude recovery review findings. All production Setup snapshot writes now pass
through one admission function. A transient empty projection preserves the
last known-good Provider and Runtime inventory, publishes an actionable
`empty_setup` state and cannot become ready. A source contract rejects new
direct production assignments.

Claude native login now uses one App-generated Incident ID for the exact UDS
request, safe operational diagnostics, staged timeout and user-visible recovery
state. Local admission and transport failures keep that Incident; remote errors
retain the daemon Incident. The macOS sheet exposes an accessible Cancel action
while bounded polling is active. The process contract also covers Home and
temporary-directory identity replacement plus descendant process-group cleanup.

The complete Go repository, `go vet`, four affected internal race packages and
the affected daemon race subset pass. A combined race command that included the
whole daemon package reached the repository's ten-minute timeout while race mode
was recompiling the strict Swift probe; it reported no race and is not rewritten
as a pass. The complete macOS suite, deterministic double-build, transactional
installer, dry run, real install, strict signing and candidate-to-installed
byte identity pass. The exact installed preflight reports 24 Providers, seven
Runtimes and six executable Profiles with an explicit Mission/Team anchor. It
fails closed on ambiguous automatic anchor selection and on a deliberately
incorrect daemon digest.

The installed 720px Runtime & Providers sheet retains complete inventory,
renders Claude as `Sign In Required`, keeps the `Sign In` action visible and
bounds long OpenCode model labels. Detailed evidence is
[`P7-HT2-installed-build316.md`](./P7-HT2-installed-build316.md).

## Build 317 installed hardening result

Build 317 preserves the Build 313 paid result and Build 316 Setup-admission
work while closing the remaining Claude cancellation race. Every Swift UDS
request now has an exact cancellation token bound to its live descriptor.
Cancellation shuts down that descriptor under a lock to interrupt blocked IO;
the worker remains the sole owner of final `close`.

The Store increments a Claude sign-in generation before cancelling the old
task, sends the daemon cancel request before awaiting that task and owns the
in-flight state until cancellation returns. Generation checks guard every
post-await publication, so stale polling cannot clear or overwrite the result.
The macOS row distinguishes waiting from cancelling and does not project that
activity onto another Runtime.

A concrete blocked private-UDS test returns `CancellationError` within one
second. A separate Store gate proves `claude_code_cancel` reaches the daemon
before a blocked Setup poll is released. The complete macOS suite passes 496
XCTest cases with two conditional skips and 21 strict Swift contracts.

The strict Go-to-Swift protocol probes remain real Swift binaries and retain
strict decoding, but now compile in debug mode for source tests. The daemon
probe improved from `631.46s` to `53.75s`, and the local IPC probe from
`192.25s` to `4.06s`. With that test-only change, `go test ./... -count=1`,
`go vet ./...`, four affected internal race packages and the focused daemon
race matrix pass under their default bounds. Release packaging still performs
two independent deterministic release builds.

Candidate build, transactional installer fixtures, dry run, real installation,
strict signing and candidate-to-installed byte identity pass. The exact
installed preflight reports 24 Providers, seven Runtimes and six executable
Profiles with the explicit Mission/Team anchor. A deliberately wrong daemon
digest fails before product state is read.

Three real 720px Runtime & Providers captures cover all seven Runtimes, CC
Switch imports and the full 24-Provider list without overlap. Claude remains
`Sign In Required`; no login, credential operation or paid Provider request was
performed. Detailed evidence is
[`P7-HT2-installed-build317.md`](./P7-HT2-installed-build317.md).

## Build 265 candidate

Build 265 adds no Provider request or credential operation. It closes the Pi
Runtime dependency authority gap before external assets are materialized:

- source-pinned llama.cpp b10107 archive SHA-256:
  `b9554ab4c9f6e91199f48387cb4ab27466fb1d724881f81463ef03f6370cfa32`;
- bounded tar/gzip parsing rejects traversal, absolute or escaping links,
  duplicate names, unsupported objects, decompression bounds and link cycles;
- the exact archive derives one deterministic tree digest;
- every local Runtime file and directory must match that tree, with no extras;
- archive, executable, dependencies and model reject symlinks, hard links,
  ownership/mode drift and file-identity replacement;
- App bootstrap and daemon parsing require private root, archive, executable
  and model as one complete tuple.

Packaging verification:

```text
scripts/test-build-loom-local-app.sh
PASS (two release builds, strict signature and byte-identical bundle)

scripts/test-install-loom-local-app.sh
PASS

scripts/install-loom-local-app.sh --dry-run \
  --app .../.dist-build265/Loom.app \
  --destination /Users/lune/Applications/Loom.app
PASS
```

The candidate bundle is Loom `0.5.6` Build 265:

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `2e86a99d068c969ab0c38dab3da378836881d62715b1a2dcfd90203ce47f64d0` |
| `Contents/Library/Helpers/loomd` | `d267ff498c3a7083e3ccf390314358457dad8f0e720f151edd30fe301eae537c` |
| `Contents/Info.plist` | `54160f6650542231785b26014f3d1c1ac560544effad621ce37426bad9f04a46` |

Installed Build 264 remained running throughout; Build 265 was neither
installed nor launched, so these results are candidate evidence only.

## Process-fixture stability investigation

Earlier repository-wide runs were not rewritten as passes. Under concurrent
package load they exposed test-fixture startup deadlines:

1. A process-group cancellation fixture failed before its child could publish
   a PID.
2. A JSONL fixture reached its session deadline before the shell produced its
   first line.
3. A Codex login fixture reached five seconds before its child wrote its PID.
4. A Pi Tool fixture reached two seconds before entering the governed Tool
   channel.

These tests verify identity, IO and cleanup, not repository scheduling latency.
Their fixture startup/session windows are now bounded at 15 seconds; their
cancellation and process-group cleanup deadlines remain short. The affected
Codex login and Pi Tool tests each passed 20 repeated runs, the earlier daemon
tests passed 20 repeated runs, the initial line test passed 50 repeated runs,
and all System Harness Session/Command tests passed 30 repeated runs before the
final repository-wide pass. Production Runtime and conversation deadlines were
not widened.

## Post-Build 317 final-gate hardening

This source-only slice closes three ways a final installed run could otherwise
overclaim readiness:

1. Claude Conversation Profile publication requires the shared canonical
   Runtime instance ID, the Claude adapter, online state, positive capacity,
   the exact model and `workspace_edit`. Foreign, zero-capacity, wrong-model or
   capability-incomplete Runtime projections remain non-executable even after
   native auth succeeds.
2. The Phase 7 runner validates the complete projection contract for Codex,
   OpenCode native and brokered Routes, Claude Code, Pi and Loom Native. Exact
   Profile ID, protocol, auth mode, Provider Account and credential revision
   must agree; a non-empty model alone is insufficient.
3. The installed privacy gate creates a deterministic content-only marker,
   rejects that marker in run identifiers, and performs a bounded owner-only
   scan of the exact private state and diagnostics roots after restart. Marker
   material is never included in the scanner error.

The concrete cancellation contract now starts the real Swift
`LocalIPCClient`, blocks `setup_snapshot` through a private Go UDS server and
the product daemon route, cancels the old live descriptor, and proves the same
Incident ID reaches `claude_code_cancel` before the blocked Setup handler is
released.

Focused RED tests first proved that all four inexact Runtime variants and all
five Claude Profile drifts were accepted by the previous implementation. They
then passed after the fail-closed gates were added. Verification completed with:

```text
go test ./... -count=1
PASS
cmd/loomd          239.215s
internal/localipc  231.805s
all packages green

go vet ./...
PASS

go test -race ./internal/app -run '<exact Claude Runtime publication>' -count=1
PASS

go test -race ./internal/runtime/harnessadapter \
  -run 'TestSystemClaudeCodeLoginControllerCancelKillsDescendantProcessGroup$' \
  -count=1
PASS

go test -race ./cmd/loomd -run \
  'Test(Phase7SwiftProbeCancelsBlockedSetupThroughProductDaemonRoute|SelectPhase7LiveProfilesRequiresFiveDistinctExecutableHarnesses|Phase7LiveProfileExecutableRequiresExactProjectionContract|ProductDaemonContainsExactPiMetadataTimeoutWithoutStoppingIPC)$' \
  -count=1
PASS (69.517s)

swift test --package-path apps/macos
496 XCTest cases, 2 conditional skips, 0 failures
21 strict Swift Testing contract cases, 0 failures

git diff --check
PASS
```

The first default-concurrency repository run is preserved as a failure: the Pi
metadata-isolation Setup read observed one transient local transport failure,
and the Claude process-group fixture did not publish its PID within five
seconds under concurrent Swift compilation. Both passed in isolation. The Pi
test now retries only `local product unavailable` or IPC timeout within a
bounded 30-second recovery window while continuing to fail on daemon exit and
business errors. The Claude fixture allows 30 seconds only for process startup;
its actual cancellation and descendant process-group cleanup deadline remains
five seconds. The second default-concurrency repository run passed completely.

No App package, installation, browser login, credential operation, network
Provider request or paid model call was performed in this source slice.

## Build 318 installed hardening result

The source slice above is now installed as Loom `0.5.6` Build 318. Two clean
release builds were byte-identical, strict signing passed, the transactional
installer fixture and dry run passed, and candidate bytes exactly match the
installed bundle. Build 317 is retained as rollback.

| File | SHA-256 |
| --- | --- |
| `Contents/MacOS/LoomLocalApp` | `6fa0dd327b8d4dceb2ed1a60aa321d04243248dfb8c8ad7af0c8f155b7a3a779` |
| `Contents/Library/Helpers/loomd` | `c7583c8a3884728c9f7bc758a002fe666e3b77d51aed18e66612f9f63cc52ce4` |
| `Contents/Info.plist` | `c4974be62f47c925c14ec2a9c6199f5127d6d83bbeb7b09d5750530b0a06ba13` |

The cold-started App and managed daemon run as PIDs `414` and `416`. The exact
identity preflight passes with 24 Providers, seven Runtimes, six executable
Profiles, the explicit Mission/Team anchor and seven RoundTable records. Claude
Runtime is online with capacity three; no Claude Profile is published before
user authentication. Detailed evidence is
[`P7-HT2-installed-build318.md`](./P7-HT2-installed-build318.md).

## Visual acceptance

The exact Mission target, alignment sources, context mode, status, confirmation
and cancellation actions remain visible without overlap or horizontal clipping
at regular and accessibility sizes.

| Artifact | SHA-256 |
| --- | --- |
| `P7-HT2-action-proposal-v2-mission-360.png` | `f77b3e92380f370640ce7375544bcb5ff7b6bdfbb0226deff38ca2fb3c30e7fa` |
| `P7-HT2-action-proposal-v2-mission-560.png` | `c851fd4ca5ddb9baa202407522d7af599e73a494120b0651df00c21998f93f79` |
| `P7-HT2-alignment-proposal-360.png` | `e0103e791a581f61019728b9a42ff645726d49786d85f26fde005d076e978f4f` |
| `P7-HT2-alignment-proposal-560.png` | `4b4b0bf99c1b90b40a8bc0cf8b9d9f16276ad237288c12634085415e6095ed46` |
| `action-proposal-receipt-360.png` | `e32269d016995396f0b4f4d0e04a4cc5061d856fa495808b57dd4a8b5e6e7b82` |
| `action-proposal-receipt-560.png` | `5ff7a2aa84614a1f27e01f26c7a66e2ebd5a7be50c2013421725c0c4678f9601` |
| `P7-HT2-installed-build317-runtime-provider.png` | `7d666339b63a9bebb5e1e7d57137cde2c7eac76a72b7f7797927315819b04487` |
| `P7-HT2-installed-build317-providers.png` | `9f45b9b3b8cece167649f0a5688523692ace77fe667a0cd66562bf091ad39b6c` |
| `P7-HT2-installed-build317-providers-bottom.png` | `4b0ebaafb3b0db8830e1b909084aed5007baadb5c96981a58213a428415a8b40` |

## Historical Build 318 pre-closure state

At that checkpoint, installed Build 318 added exact Runtime/Profile admission
on top of Build 317's
cancellation-safe user-facing Claude native-auth recovery, Build 316's unified
Setup admission and dynamic no-restart Profile publication. Its exact bundle
identity, private UDS inventory and complete macOS regression pass; Build 317's
720px Runtime visual remains representative because no UI production source
changed. Detailed evidence is
[`P7-HT2-installed-build318.md`](./P7-HT2-installed-build318.md). Build 317,
Build 316 and Build 315 are preserved as preceding recovery checkpoints. Build
313 remains the last paid four-Runtime matrix and is not rewritten as a Build
318 result.

On 2026-08-30 the user stated that Claude Code is unavailable and explicitly
waived its installed login and paid live call. Claude source parity, strict
Profile publication, cancellation, privacy and failure-isolation coverage stay
mandatory and green. The available-Runtime gate fails closed if an executable
Claude Profile appears and therefore cannot turn the waiver into a silent
capability downgrade. Build 324 later completed the remaining available-Runtime
and privacy gates.

## Build 319 Codex authentication correction

The authorized Build 318 rerun reached the installed managed daemon but failed
on the first Codex turn. A direct bounded native invocation proved the local
refresh token was invalid even though the previous `codex login status` observer
returned success. Build 319 replaces that observer with the exact attested
Codex App Server and an active `account/read` request using `refreshToken=true`.
Only `authenticated` and `requiresOpenaiAuth` booleans leave the probe boundary;
account identity, email, raw protocol output and Provider error text are
discarded and bounded buffers are zeroized.

The reconnect connector invalidates cached observations before and after the
official login launch. A bound App Server `error` notification with closed
`codexErrorInfo=unauthorized` becomes `provider_auth` with a reconnect action;
thread or turn substitution remains a protocol failure. Usage-window and
temporary-service classes remain fixed-enum, privacy-safe diagnostics. Focused
Provider, Harness and daemon tests were green at this checkpoint; the complete
repository, race, Swift, deterministic package and installed gates were closed
by Build 324.

## Build 324 source and installed closure

Build 324 corrects the final protocol and privacy defects without weakening
Harness identity or credential boundaries:

- Codex native auth interprets official App Server `account/read` semantics:
  an authenticated account with `requiresOpenaiAuth=true` is available; an
  unauthenticated account remains unavailable; unsupported shapes fail closed.
- Codex and OpenCode subprocesses receive a canonical, owner-only per-call
  temporary directory. Success, child failure and cancellation remove that
  tree. Constructor migration validates ownership, type, link count and
  identity before repairing legacy scratch trees to directories `0700` and
  files `0600`.
- The privacy acceptance marker uses a content-only shape that does not resemble
  a credential, while the existing credential-shaped rejection remains intact.

The final source gate passed:

```text
go test ./... -count=1
PASS
cmd/loomd                                  93.573s
internal/localipc                          79.796s
internal/provider                          27.793s
internal/runtime/harnessadapter            22.969s
internal/runtime/piadapter                 74.731s
all packages green

go vet ./...
PASS

go test -race -p=1 ./internal/provider -count=1
PASS

swift test --package-path apps/macos
496 XCTest cases, 2 conditional skips, 0 failures
21 strict Swift Testing contract cases, 0 failures

scripts/test-build-loom-local-app.sh
PASS (two independent deterministic release builds)

scripts/test-install-loom-local-app.sh
PASS (transactional replacement and rollback fixtures)
```

The exact Build 324 candidate passed strict signing and installer dry-run,
matched the installed bundle byte for byte, and retained Build 323 as the
rollback bundle. A cold-start and post-matrix restart both used the bundled
managed daemon. The final preflight reported 24 Providers, seven Runtimes, six
executable Profiles, the exact Mission/Team governance anchor and seven
RoundTable navigation records.

Two bounded Codex live probes passed before the full matrix: an ordinary text
reply in 8.10 seconds and a Mission Proposal/cancel lifecycle in 11.33 seconds.
The explicitly authorized available-Runtime matrix then passed in 407.66
seconds for Codex, OpenCode/DeepSeek, Pi and Loom Native/MiniMax. It exercised
all 28 Registry Tools, confirmation, cancellation, digest-bound expiry, replay
rejection, exact RoundTable action targets, managed restart, restored decision
authority and the final bounded plaintext/privacy scan. Claude remained
discovered with no executable Profile and was recorded as N/A under the user's
explicit live waiver. The acceptance harness did not inspect or print
credentials and did not change or migrate them; normal Provider calls used the
daemon's existing short-lived credential leases.

Detailed installed evidence is
[`P7-HT2-installed-build324.md`](./P7-HT2-installed-build324.md). The preceding
failed privacy gate remains in
[`P7-HT2-installed-build323-failed-privacy.md`](./P7-HT2-installed-build323-failed-privacy.md).
