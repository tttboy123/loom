# P2A-W2 Deterministic Verification

**Date**: 2026-07-29
**Status**: PASS
**Frozen contract SHA-256**:
`f2e7a4d27866da4ca5f08623db6d1082587f8fd4e986f3c39f1d839dd0609a55`

## Candidate outcome

The deterministic Candidate closes the single frozen pre-execution vertical:

- blank, saved-Team, and template Candidate Builder flows;
- exactly one question at a time, edit, validate, exact preview, stale-view
  rejection, explicit confirmation, and no execution fact;
- first-version TeamDefinition save plus archive/restore through Journal CAS;
- rebuildable saved-Team and non-secret Provider credential projections;
- Codex `login status` observation with bounded output, executable identity
  checks, and process-group cancellation;
- fixed-origin, redirects-disabled, non-generative MiniMax verification;
- macOS Keychain-backed Credential Broker configure/verify/replace/revoke with
  rollback and opaque-reference-only Journal metadata;
- shared daemon IPC used by both Bubble Tea and the strict native Swift client;
- saved/template selection and archive/restore without user-entered internal
  identifiers.

Provider-only setup remains visible when no compatible Runtime exists, while
Team Builder fails closed as incompatible.

## Verification matrix

The final Candidate passed:

```text
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
git diff --check
gofmt -l <all owned Go files>                    # empty
go mod tidy -diff                                # empty
go mod verify                                    # all modules verified
CGO_ENABLED=0 go test ./internal/credentials -count=1
CGO_ENABLED=0 go test ./internal/credentials ./internal/provider -run '^$'
go test ./internal/credentials \
  -run TestKeychainStoreConfigurationIsFixedAndBounded -count=1
go test ./internal/provider -count=20
swift test --package-path apps/macos
swift build -c release --package-path apps/macos
swift test --sanitize=thread --package-path apps/macos
```

The Swift debug and thread-sanitizer matrices each passed 27 XCTest cases with
one expected visual-export skip, plus all three Swift Testing cases. The release
app build passed.

The complete Go and repository-race matrices passed every package, including
the real Go UDS fixtures and the compiled strict Swift probes.

## Regression found during verification

A real `SystemCodexStatusRunner` fixture showed that embedding `bytes.Buffer`
also exposed `ReadFrom`. `os/exec` could therefore bypass the Candidate's
custom `Write` limit: a 64-byte bound accepted 256 bytes.

The Candidate now contains the buffer instead of embedding it, so all process
output passes through the bounded `Write`. Repeated real-process tests prove:

- output over the exact limit is rejected;
- cancellation kills the spawned process group;
- replacement of the executable after launch is rejected by post-run identity
  revalidation.

The Provider package passed this matrix for 20 consecutive runs after repair.

## Security and authority evidence

- Credential references must match the generated
  `credential-ref-<opaque ASCII>` grammar in Application Service, Broker,
  StateWriter, and Projection. A non-opaque application command is rejected
  before the credential mutator is invoked.
- Keychain service identity is fixed to Loom's reviewed service name.
- saved-Team reconstruction matches model, Runtime, Skill revision/digest,
  permission, and resource bindings exactly.
- catalog validation rejects model drift, Runtime-incompatible Skills,
  duplicate references, and invented references.
- nil JSON collections remain canonical empty arrays; the Swift decoder remains
  strict.
- confirmation and archive/restore write only the frozen TeamDefinition facts.
- no TeamInstance, AgentInstance, WorkItem, Run, Grant, Evidence, dispatch, or
  Provider-generation Event is produced by W2 tests.
- candidate source/evidence scans found no key-shaped or private-key material.
- the previously pasted MiniMax value was neither read nor used.

## Deterministic-only boundary

No network request, real Provider verification, installed Keychain mutation,
real Codex process, resident daemon mutation, installed-app action, Runtime
activation, live client action, staging, or commit occurred.

Fresh independent Implementation Review is the next gate. A separately frozen
live manifest remains mandatory after that PASS.

## Pre-review audit repair and rerun

The first attempted independent review was invalidated because its agent
modified the Candidate instead of remaining read-only. The two bounded changes
were retained only after Controller inspection:

- application credential mutations now reject a non-opaque reference before
  invoking the Credential Broker;
- saved-Team loading has an explicit regression proving exact binding drift
  fails closed.

After these changes, the focused W2 packages, complete repository tests, and
complete repository race tests passed again. The complete matrices that require
Unix sockets, Swift compiler caches, and controlled home-directory fixtures were
run outside the restricted sandbox with an isolated `/private/tmp` Go cache.
The earlier sandbox-only failures were permission denials and are not counted as
product failures. Vet, module, Swift debug/release/thread-sanitizer, format,
scope, and secret-negative results remain current because their relevant source
surfaces did not change, or were rerun after the change.

That modifying agent's result is not an Implementation Review. A new,
genuinely read-only independent Reviewer subsequently returned `PASS` with no
P0, P1, or P2 findings and explicitly confirmed that it modified no files,
staged nothing, committed nothing, and ran no process or live action. The
separate W2 live-manifest freeze is now the current gate.
