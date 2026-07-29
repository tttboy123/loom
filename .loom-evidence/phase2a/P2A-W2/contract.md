# P2A-W2 Team Builder and Provider Onboarding Contract

**Date**: 2026-07-29
**Status**: FROZEN — fresh independent Contract Review PASS
**Parent**: frozen Phase 2A Local Product Experience Exit Contract
**Depends on**: ADR-0001, ADR-0002, ADR-0003, ADR-0004, ADR-0011,
P2A-W1 checkpoint commit `7c1c469c46c97eac0cab39a756b2d59867a49563`

## 1. Vertical capability

P2A-W2 closes one pre-execution product journey through the shared local
application boundary:

1. a user starts a Team Builder from blank, an accepted saved TeamDefinition,
   or a caller-supplied versioned template;
2. Loom asks exactly one bounded question at a time and keeps all intermediate
   state Candidate-only;
3. the user can answer, edit, validate, inspect exact bindings and compatibility,
   and explicitly confirm;
4. confirmation atomically saves one active TeamDefinition revision, but creates
   no TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence, dispatch, or
   Provider request;
5. saved definitions can be loaded, archived, and restored through the same
   application service and Event Journal authority;
6. the Runtime/Provider surface reports exact observed Runtime metadata and the
   truthful ADR-0004 authentication mode;
7. Codex native OAuth is observed only through the stable
   `codex login status` command and is never copied into Loom;
8. MiniMax credentials are stored only through the macOS Keychain-backed Secret
   Store, resolved by the Credential Broker for a bounded verification request,
   and represented in the Journal only by a non-secret opaque reference and safe
   status metadata.

The Bubble Tea client and native macOS app both call the same versioned daemon
IPC methods and application services. Neither client parses CLI output, reads
SQLite, stores authority, or creates a second catalog.

There is no P2A-W2a/W2b and no P2A-W4. Credential storage, Provider observation,
Team Builder, StateWriter, Projection, IPC, and UI work below are internal parts
of this one vertical WorkItem.

## 2. Baseline and preserved state

- Repository and Git top-level:
  `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- Branch: `codex/loom-platform-slice2`
- Baseline commit: `7c1c469c46c97eac0cab39a756b2d59867a49563`
- P2A-W1 governed live result remains:
  `FAIL - ROLLBACK_INCOMPLETE - HUMAN_REQUIRED - NO RETRY`.
- P2A-W1's live allowance remains `0`; W2 does not reinterpret, retry, or replace
  that canary.
- The existing Runtime observer LaunchAgent, installed app, sockets, model,
  Runtime state, and Phase 1 evidence remain unchanged until a separately frozen
  W2 post-Implementation-Review live gate.
- Pre-existing shared-worktree modifications and untracked files remain
  user-owned and excluded from every W2 Candidate and commit.
- The previously pasted MiniMax secret is prohibited input. W2 may use only a
  fresh user-entered secret through the product UI at a live gate or an already
  present Keychain item selected by its non-secret reference.

## 3. Exact owned files

### Existing product and authority files

W2 may modify only:

1. `cmd/loom/tui.go`
2. `cmd/loomd/run.go`
3. `cmd/loomd/run_test.go`
4. `cmd/loomd/product_daemon.go`
5. `cmd/loomd/product_daemon_test.go`
6. `internal/localipc/protocol.go`
7. `internal/localipc/protocol_test.go`
8. `internal/localipc/swift_contract_test.go`
9. `internal/projection/projection.go`
10. `internal/projection/projection_test.go`
11. `internal/projection/global_read_view.go`
12. `internal/projection/global_read_view_test.go`
13. `internal/tui/model.go`
14. `internal/tui/model_test.go`
15. `internal/tui/program.go`
16. `internal/tui/program_test.go`
17. `apps/macos/Sources/LoomLocalAppCore/LocalIPCClient.swift`
18. `apps/macos/Sources/LoomLocalAppCore/LocalProductModels.swift`
19. `apps/macos/Sources/LoomLocalAppCore/LocalProductStore.swift`
20. `apps/macos/Sources/LoomLocalAppCore/SafeText.swift`
21. `apps/macos/Sources/LoomLocalAppUI/ContentView.swift`
22. `apps/macos/Tests/LoomLocalAppTests/LocalIPCClientTests.swift`
23. `apps/macos/Tests/LoomLocalAppTests/LocalProductModelsTests.swift`
24. `apps/macos/Tests/LoomLocalAppTests/LocalProductStoreTests.swift`
25. `apps/macos/Tests/LoomLocalAppTests/SafeTextTests.swift`
26. `apps/macos/Tests/LoomLocalAppTests/LocalProductExperienceViewTests.swift`
27. `docs/CURRENT.md`

### New vertical implementation files

W2 may create only:

1. `internal/credentials/credential_broker.go`
2. `internal/credentials/credential_broker_test.go`
3. `internal/credentials/keychain_darwin.go`
4. `internal/credentials/keychain_darwin_test.go`
5. `internal/credentials/keychain_unsupported.go`
6. `internal/credentials/keychain_unsupported_test.go`
7. `internal/provider/codex_native_auth.go`
8. `internal/provider/codex_native_auth_test.go`
9. `internal/provider/minimax_verifier.go`
10. `internal/provider/minimax_verifier_test.go`
11. `internal/state/local_product_setup_writer.go`
12. `internal/state/local_product_setup_writer_test.go`
13. `internal/projection/local_product_setup.go`
14. `internal/projection/local_product_setup_test.go`
15. `internal/app/local_product_setup.go`
16. `internal/app/local_product_setup_test.go`
17. `internal/api/local_product_setup.go`
18. `internal/api/local_product_setup_test.go`
19. `apps/macos/Sources/LoomLocalAppCore/LocalProductSetupModels.swift`
20. `apps/macos/Tests/LoomLocalAppTests/LocalProductSetupModelsTests.swift`

### Evidence

W2 may create files only under:

```text
.loom-evidence/phase2a/P2A-W2/
```

No other file is owned. In particular, W2 does not own Journal schema or
migrations, Runtime discovery adapters, Agent/Runtime domain contracts,
Scheduler, Supervisor, WorkPackage/Run authority, Grant/Evidence authority,
Phase 1 evidence, W1 immutable live evidence, installed LaunchAgent state, or
release/publish surfaces.

An unexpected need to modify an unowned product file stops for one reviewed
amendment to this same W2 contract. It never creates W4 or a thin WorkItem.

## 4. Dependency and platform lock

- W2 adds no Go, Swift, JavaScript, or system-package dependency.
- The macOS Secret Store uses Security.framework generic-password APIs through
  a Darwin+cgo implementation.
- The production macOS build must link Security and CoreFoundation.
- `!darwin` and `!cgo` builds are fail-closed and return
  `credential_store_unavailable`; they do not fall back to a file, environment
  variable, process argument, prompt, or SQLite.
- W2 may not invoke `/usr/bin/security` because a secret must never appear in
  process arguments, shell text, environment variables, or captured output.
- Existing Go 1.22 and Swift package language floors remain unchanged.

## 5. Trust boundaries and secret-negative invariants

The W2 security model has six explicit boundaries:

```text
masked local client input
  -> private versioned UDS
  -> typed setup application service
  -> Credential Broker
  -> OS Keychain
  -> fixed MiniMax HTTPS origin
```

Codex native auth follows a separate observation path:

```text
typed setup application service
  -> fixed reviewed Codex executable
  -> exact `login status`
  -> closed non-secret status mapping
```

Binding rules:

- Provider secrets are accepted only as bounded UTF-8 request fields and are
  copied no more than required for Keychain or one HTTPS Authorization header.
- Mutable secret buffers are zeroed on every return path where Go can control
  the bytes. No API promises impossible zeroization of immutable runtime strings.
- A secret, prefix, suffix, length-derived fingerprint, hash, or digest must not
  enter source, Event payloads, Projection, logs, errors, stdout/stderr,
  Evidence, AgentDefinition, TeamDefinition, Team Draft preview, screenshots,
  request IDs, metrics, or test golden files.
- Keychain items use a fixed Loom service name, a normalized Provider account,
  `kSecClassGenericPassword`, `kSecAttrSynchronizable=false`, and an
  after-first-unlock-this-device-only accessibility class.
- Journal state stores only a generated opaque credential reference, Provider
  ID, truthful auth mode, revision, safe status, safe reason code, and UTC
  timestamps.
- The Broker resolves by opaque reference and Provider ID; mismatches,
  unavailable store, denied access, revoked reference, stale revision, timeout,
  TLS failure, unexpected origin, and Provider rejection fail closed.
- MiniMax verification is limited to a fixed HTTPS origin and bounded request,
  response, timeout, and redirects-disabled client. No user-controlled URL,
  proxy endpoint, model prompt, chat completion, billable generation, or retry
  loop is permitted in W2.
- Provider response bodies and raw Codex output never cross IPC. Only closed
  status/reason enums do.
- Codex observation never reads `auth.json`, Keychain items, access tokens,
  refresh tokens, API keys, browser cookies, or process environments.

## 6. Team Builder session

`internal/app.LocalProductSetupService` owns bounded in-memory Builder sessions.
A session is Candidate state, not authority, and contains:

- generated Draft ID and monotonically increasing revision;
- source kind `blank`, `saved_team`, or `template`;
- exact catalog/view version;
- at most one current question;
- answers and edits from a closed field set;
- selected AgentDefinition versions, Runtime profile and instance IDs, model
  IDs, Skill revision+digest, permissions, bounded resource pointers, requested
  concurrency, and maximum budget credits;
- an immutable StructuredTeamDraft binding digest once all required fields are
  valid;
- a compatibility/estimated-cost preview.

Rules:

- exactly one unresolved question is returned;
- answers bind Draft ID, expected revision, question ID, and catalog version;
- stale revision/catalog/view commands conflict without mutation;
- unknown fields, invented references, duplicate references, unbounded text,
  invalid UTF-8, unsupported source, and invalid choice fail closed;
- blank source asks the closed required question sequence;
- saved source is reconstructed only from an active saved TeamDefinition in the
  current GlobalReadView;
- template source requires an injected exact template ID, version, and digest;
- template and saved sources remain Drafts and cannot bypass confirmation;
- edit reopens exactly one closed field and preserves the same Draft identity;
- service restart may discard unconfirmed sessions; it must not synthesize a
  saved or confirmed Team;
- no model or Provider is called to generate questions or answers in W2.

The service composes the accepted `teams.BuildTeamDraftCatalog`,
`teams.BuildTeamDraftContent`, `teams.NewStructuredTeamDraft`,
`teams.AnswerStructuredTeamDraft`, `teams.EditStructuredTeamDraft`,
`teams.CheckStructuredTeamDraftAcceptable`, and
`teams.BuildTeamDefinition` boundaries. It does not fork their validation.

## 7. Exact preview

Every proposed Builder session returns copied, bounded DTOs with:

- Agent ID, version, scope, display name, role and responsibility;
- Runtime profile ID, Runtime instance ID, adapter, executable version, status,
  capacity, exact observed capabilities, selected model ID, and source probe;
- truthful auth mode: `brokered`, `provider_ephemeral`, or `native_auth`;
- Skill ID, revision, digest, source scope, risk, and Runtime compatibility;
- permissions and bounded resource pointer IDs only, never resource contents;
- per-role compatibility result and closed reason;
- requested concurrency, maximum budget credits, and an explicitly labeled
  non-authoritative estimated maximum cost;
- Draft ID/revision/catalog/content/binding digests;
- whether explicit confirmation is currently allowed.

Nil collections serialize as JSON empty arrays. Ordering is canonical and every
returned slice is a deep copy.

## 8. Confirmation and saved TeamDefinition authority

Confirmation requires:

- exact Draft ID, revision, catalog/view version, binding digest, and explicit
  `confirm=true`;
- a current `CheckStructuredTeamDraftAcceptable` result;
- no compatibility gap;
- exact source Definition/version/digest when derived from a saved Team;
- caller-supplied TeamDefinition ID and scope identity that pass existing domain
  validation.

The StateWriter appends one deterministic `TeamDefinitionSaved` Event to:

```text
team-definition/<team_definition_id>
```

The payload contains only the exact validated TeamDefinition snapshot and source
Draft digests. The Event and stream head are committed through
`journal.AppendBatchIfStreamHeads`. Concurrent confirmations with the same
expected stream head produce one winner and one conflict. No hidden retry is
allowed.

Archive and restore append `TeamDefinitionArchived` and
`TeamDefinitionRestored` to the same stream under CAS. Archive/restore never
changes the definition digest, creates a new definition version, or starts
execution.

## 9. Provider onboarding authority

The Credential Broker exposes typed configure, verify, replace, status, and
revoke operations for the fixed Provider ID `minimax`.

Credential metadata is committed to:

```text
provider-credential/minimax
```

using only:

- `ProviderCredentialConfigured`
- `ProviderCredentialVerified`
- `ProviderCredentialRevoked`

All events use `AppendBatchIfStreamHeads`. The writer validates exact Provider,
auth mode, opaque reference, expected revision/head, safe status and reason, and
UTC timestamps. It rejects any payload field that resembles secret material.

Cross-boundary transaction rules:

- configure: write Keychain item, then append metadata; delete the just-written
  item if append fails;
- replace: read the prior secret into bounded memory, replace Keychain bytes,
  append metadata, and restore prior bytes if append fails;
- verify: resolve exact reference, perform one bounded verifier request, then
  append only the safe result; Provider rejection does not disclose its body;
- revoke: read prior secret, delete Keychain item, append revoked metadata, and
  restore prior bytes if append fails;
- rollback failure is returned as `credential_rollback_failed`, keeps secrets
  out of diagnostics, and stops further mutation.

There is no automatic retry, background Provider polling, Provider fallback,
secret import from environment, or Runtime activation.

## 10. Projection and read view

The existing rebuildable Projection recognizes only the W2 event types above.
It exposes copied:

- latest TeamDefinition revision/status/source digests;
- latest Provider credential metadata;
- no secret, secret digest, Keychain locator internals, or raw Event payload.

Projection rebuild remains all-or-nothing; failure preserves the prior immutable
GlobalReadView. The GlobalReadView version remains the canonical
stream-head/event-ID digest. No table, database, checkpoint, queue, scheduler,
cache authority, or client authority is added.

## 11. Codex native-auth observation

The production observer:

- resolves only a caller-configured absolute regular executable whose containing
  directory and file identities are revalidated before launch;
- invokes exactly `codex login status`;
- uses a fixed minimal environment, bounded output, process-group cancellation,
  and a five-second timeout;
- maps only reviewed complete-line outputs to `available`, `not_logged_in`,
  `unsupported`, `timeout`, or `unavailable`;
- reports `native_auth` and never calls login/logout or reads credential storage.

The local product may select Codex native auth only while the current observation
is `available`; an observation is not a persisted credential and grants no Run.

## 12. Local IPC

W2 reuses protocol version 1 and adds typed methods only:

- `setup_snapshot`
- `builder_start`
- `builder_answer`
- `builder_edit`
- `builder_validate`
- `builder_confirm`
- `team_archive`
- `team_restore`
- `credential_configure`
- `credential_verify`
- `credential_replace`
- `credential_revoke`

Every request and result uses strict unknown-field rejection, bounded lengths,
closed enums, request deadlines, current-peer UID authorization, and canonical
JSON collections. Mutations are never replayed automatically by client
reconnect. Safe errors are limited to:

- `invalid_request`
- `not_found`
- `conflict`
- `incompatible`
- `denied`
- `credential_unavailable`
- `credential_rejected`
- `credential_rollback_failed`
- `timeout`
- `state_unavailable`
- `busy`
- `internal`

Error messages contain no upstream text, filesystem path, executable output,
secret reference internals, or credential material.

## 13. TUI and native app behavior

Both clients add one Team Builder/Provider setup journey:

- ordinary users select visible saved Teams/templates/Runtimes/Providers and do
  not enter Team IDs, Runtime IDs, model IDs, cursors, SQLite paths, Provider
  environment variables, or service-manager commands;
- one question is visible at a time with Back/Edit/Cancel semantics;
- confirmation is a distinct final action showing the exact preview;
- MiniMax entry is masked, never echoed in summaries, never copied to clipboard,
  and cleared after each request;
- configure/test/replace/revoke results show only the safe status;
- Codex OAuth is shown as native auth and Pi as local Runtime metadata;
- loading, empty, unavailable, denied, incompatible, conflict, partial, offline,
  stale, and fatal states are explicit;
- keyboard focus, resize, accessibility labels, and cancellation remain usable;
- clients retain no authoritative result after a daemon conflict or restart and
  refresh from Projection before another mutation.

No client creates a TeamInstance or starts a Run.

## 14. Mandatory RED

Before product implementation, tests must fail for missing W2 behavior and prove:

1. blank/saved/template once-per-question flow and stale revision conflict;
2. edit/validate/exact preview, including Skill revision/digest, permissions,
   resources, auth mode, capabilities, model and estimated cost;
3. explicit confirmation is required and pre-confirmation creates no
   TeamInstance/Run/Grant/Evidence/execution fact;
4. confirmation CAS has one winner and one conflict;
5. saved definition load/archive/restore and Projection rebuild/restart;
6. Codex observer exact command, status mapping, timeout, output bound, identity
   revalidation, and no credential-file access;
7. Keychain/store configure/verify/replace/revoke, unavailable, denied, rollback,
   stale reference/revision, and concurrent CAS behavior;
8. secret-negative scans over request handling, Event bytes, Projection,
   application/TUI/native state snapshots, errors, logs, stdout/stderr, and
   screenshots/fixtures;
9. real `LocalProductSetupService -> production handler -> Go UDS client`
   fixture for Builder and confirmation;
10. real Go UDS server -> strict compiled Swift client fixture for setup
    snapshot and one Candidate-only Builder step;
11. Bubble Tea no-terminal fixture for one Candidate-only Builder step.

RED must fail because the frozen W2 symbols/behavior are absent, not because of
syntax, dependency, fixture, network, Keychain prompt, or environment failure.

## 15. Deterministic verification

Before Implementation Review:

- focused tests for all owned Go packages;
- focused race tests for state/app/API/IPC/TUI and Credential Broker;
- complete `go test ./...`;
- complete `go test -race ./...`;
- `go vet ./...`;
- `gofmt -l` over owned Go files;
- offline `go mod tidy -diff` and `go mod verify`;
- Darwin production Keychain compilation and unsupported-platform compile-only
  gates;
- `swift test`, release build, and thread-sanitizer tests for the macOS app;
- exact source/diff/owned-file and pre-existing-dirt preservation checks;
- no secret-like value in source, Git diff, staged bytes, fixtures, evidence,
  Event payloads, product output, or screenshots;
- fixture E2E showing both clients use daemon IPC and never CLI/SQLite;
- Journal/Projection rebuild and fail-preserve tests;
- no TeamInstance/Run/Grant/Evidence/dispatch Event before or after W2
  confirmation;
- no network, installed Keychain mutation, launchctl, resident daemon mutation,
  Provider billing, Runtime activation, or live client action during
  deterministic verification.

## 16. Review and live gate

Sequence:

```text
Contract Review PASS
-> mandatory RED
-> implementation + deterministic verification
-> fresh independent Implementation Review PASS
-> separately frozen W2 live gate
-> controlled live canary
-> fresh Result-Evidence Review
-> local atomic W2 commit
```

The deterministic Candidate uses fake Secret Store, fake Provider verifier,
fixture Codex executable, isolated SQLite, temporary UDS, Bubble Tea messages,
and compiled Swift fixture only.

No installed Keychain item, real Codex process, real MiniMax request, resident
daemon, installed app, or Computer Use action is permitted before
Implementation Review PASS and a frozen live manifest.

The W2 live gate must:

- use a fresh `0700` root, `0600` files, private UDS, isolated non-production
  TeamDefinition IDs, and an exact manifest/source lock;
- observe the already installed Codex login only through `codex login status`;
- use either a fresh secret entered through the product at the gate or a
  pre-existing Keychain item selected by opaque reference;
- never use the earlier chat-pasted MiniMax secret;
- perform at most one non-generative MiniMax credential verification request;
- create and confirm one isolated TeamDefinition but no TeamInstance or Run;
- prove rollback/restoration and remove the canary TeamDefinition/Keychain item
  where the frozen manifest requires it;
- preserve all prior Phase 1 and W1 evidence and resident state.

If no authorized fresh/already-stored MiniMax credential exists, deterministic
W2 may be Implementation-Review PASS, but live W2 remains `HUMAN_REQUIRED`.
That condition cannot be bypassed by reading chat history, shell history,
environment variables, config files, or Provider caches.

## 17. Exit

P2A-W2 is accepted only when:

- all mandatory behavior and security tests pass;
- Contract and Implementation Review are fresh independent `PASS`;
- the controlled W2 live result and fresh Result-Evidence Review pass;
- a normal user completes Provider status/onboarding and Team confirmation
  without a terminal or internal identifier;
- no secret disclosure, duplicate authority, hidden retry, execution fact, or
  owned-file violation exists;
- W2 is committed atomically while all excluded dirt remains untouched.

Only then may P2A-W3 be frozen. P2A-W3 remains locked before this exit and no
P2A-W4 exists.
